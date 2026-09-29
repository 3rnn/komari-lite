package metricstore

import (
	"context"
	"fmt"
	logger "github.com/komari-monitor/komari/utils/log"
	"strings"
	"sync"
	"time"

	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/metric"
)

// store_migration.go
//
// "Storage backend migration" support for WebUI/API: put a metrics source database (default local SQLite
// ./data/metrics.db) to the currently running metrics target database.
// (Usually MySQL/PostgreSQL newly configured by the user and hot reloaded).
//
// Differences from RunStartupMigration:
//   - Start migrations are executed automatically and synchronously when the process starts (komari.db → current metrics target, or previous
//     metrics target → current target).
//   - The store migration here is manually triggered by the administrator "after switching databases" in the WebUI. It is executed asynchronously and can
//     Query progress/cancel. Data is written as upsert and can be safely re-executed.
//
// Typical process:
//  1. The administrator changes metric_db_dsn to MySQL/PostgreSQL in the settings, save → backend connection test
//     After hot reloading, the current store is switched to the remote end (the remote end is empty at this time).
//  2. The administrator calls startMetricMigration (without the source parameter), and the system displays the previous metrics
//     The target (default SQLite metrics.db) is used as the source to transfer historical data to the remote end.
//  3. After completion, register the target fingerprint and the transfer will not be repeated next time it is started.

// StoreMigrationProgress describes the real-time progress of a store-to-store migration.
type StoreMigrationProgress struct {
	Status         string    `json:"status"`          // idle, running, completed, failed, canceled
	SourceDriver   string    `json:"source_driver"`   // Source library driver
	SourceDSN      string    `json:"source_dsn"`      // Source library DSN (after desensitization)
	TargetDriver   string    `json:"target_driver"`   // Target library driver
	TargetDSN      string    `json:"target_dsn"`      // Target library DSN (after desensitization)
	TotalMetrics   int       `json:"total_metrics"`   // Total number of indicator definitions
	CurrentMetric  string    `json:"current_metric"`  // The name of the indicator currently being moved
	MetricsDone    int       `json:"metrics_done"`    // Number of indicators completed
	MigratedPoints int64     `json:"migrated_points"` // Number of sample points transported
	StartTime      time.Time `json:"start_time"`      // start time
	EndTime        time.Time `json:"end_time"`        // end time
	Error          string    `json:"error,omitempty"` // error message
}

var (
	storeMigMu       sync.Mutex
	storeMigProgress StoreMigrationProgress
	storeMigCancel   context.CancelFunc
	storeMigDone     chan struct{}
	storeClosing     bool
)

// IsStoreMigrationRunning reports whether a store-to-store migration is running.
func IsStoreMigrationRunning() bool {
	storeMigMu.Lock()
	defer storeMigMu.Unlock()
	return storeMigCancel != nil
}

// GetStoreMigrationProgress returns a snapshot of the current store-to-store migration progress.
func GetStoreMigrationProgress() StoreMigrationProgress {
	storeMigMu.Lock()
	defer storeMigMu.Unlock()
	return storeMigProgress
}

// ResolveStoreMigrationSourceFingerprint infers the source database fingerprint of this store migration.
//
// Priority:
//  1. Explicitly passed in driver+dsn (driver can be left blank and left to DSN inference).
//  2. The last saved metrics target fingerprint (MigrationTargetKey).
//  3. Default SQLite (./data/metrics.db).
func ResolveStoreMigrationSourceFingerprint(driver, dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn != "" {
		d := ResolveDriverFromConfig(driver, dsn)
		return fmt.Sprintf("%s|%s", d, dsn)
	}
	if saved, _ := config.GetAs[string](MigrationTargetKey, ""); saved != "" {
		return saved
	}
	return defaultSQLiteFingerprint()
}

// StartStoreMigration starts a store-to-store migration asynchronously: put the source database (by sourceDriver +
// sourceDSN (if left blank, it will be automatically inferred) and all metric data will be transferred to the currently running metrics target database.
//
// Returning an error means "failed to start" (for example, there is already a migration running, the source and target are the same, the target is not initialized, etc.);
// Errors during migration are exposed via GetStoreMigrationProgress().Status == "failed" and Error .
func StartStoreMigration(sourceDriver, sourceDSN string) error {
	cfg, err := config.GetManyAs[MetricStoreConfig]()
	if err != nil {
		return fmt.Errorf("failed to load metric store config: %w", err)
	}
	targetFP := targetFingerprint(cfg)
	sourceFP := ResolveStoreMigrationSourceFingerprint(sourceDriver, sourceDSN)

	if sourceFP == targetFP {
		return fmt.Errorf("source and target metrics database are the same; nothing to migrate")
	}

	srcCfg, err := configFromFingerprint(sourceFP, cfg)
	if err != nil {
		return fmt.Errorf("resolve source metrics store: %w", err)
	}
	if !storeOperations.TryAcquire() {
		return ErrStoreBusy
	}
	exclusiveLeaseHandedOff := false
	defer func() {
		if !exclusiveLeaseHandedOff {
			storeOperations.Release()
		}
	}()

	storeMu.RLock()
	dst := store
	activeFingerprint := storeFingerprint
	storeMu.RUnlock()
	if dst == nil {
		return ErrStoreNotInitialized
	}
	if activeFingerprint != targetFP {
		return fmt.Errorf("active metric store does not match the configured migration target; wait for hot reload and retry")
	}

	storeMigMu.Lock()
	if storeClosing {
		storeMigMu.Unlock()
		return ErrStoreBusy
	}
	if storeMigCancel != nil {
		storeMigMu.Unlock()
		return fmt.Errorf("a store migration is already in progress")
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	storeMigCancel = cancel
	storeMigDone = done
	storeMigProgress = StoreMigrationProgress{
		Status:       "running",
		SourceDriver: string(ResolveDriverFromConfig(srcCfg.Driver, srcCfg.DSN)),
		SourceDSN:    maskDSN(srcCfg.DSN),
		TargetDriver: string(ResolveDriverFromConfig(cfg.Driver, cfg.DSN)),
		TargetDSN:    maskDSN(cfg.DSN),
		StartTime:    time.Now().UTC(),
	}
	storeMigMu.Unlock()

	exclusiveLeaseHandedOff = true
	go runStoreMigration(ctx, cancel, done, srcCfg, cfg, dst, targetFP)
	return nil
}

// runStoreMigration performs the actual migration logic (in a separate goroutine).
func runStoreMigration(ctx context.Context, cancel context.CancelFunc, done chan struct{}, srcCfg *MetricStoreConfig, cfg *MetricStoreConfig, dst *metric.Store, targetFP string) {
	defer func() {
		cancel()
		storeOperations.Release()

		storeMigMu.Lock()
		storeMigCancel = nil
		storeMigDone = nil
		close(done)
		storeMigMu.Unlock()
	}()

	src, err := openSourceStore(ctx, srcCfg)
	if err != nil {
		finishStoreMigration("failed", fmt.Errorf("open source metrics store: %w", err))
		return
	}
	defer src.Close()

	observe := func(currentMetric string, metricIndex, totalMetrics int, addedPoints int64) {
		storeMigMu.Lock()
		storeMigProgress.CurrentMetric = currentMetric
		storeMigProgress.MetricsDone = metricIndex
		storeMigProgress.TotalMetrics = totalMetrics
		storeMigProgress.MigratedPoints += addedPoints
		storeMigMu.Unlock()
	}

	total, err := migrateBetweenStores(ctx, src, dst, observe)
	if err != nil {
		if ctx.Err() != nil {
			finishStoreMigration("canceled", nil)
			logger.Warnf("metricstore", "[store-migration] canceled after %d points", total)
			return
		}
		finishStoreMigration("failed", err)
		return
	}

	// Transfer successful: Register the current target fingerprint to avoid repeated transfer next time.
	if err := config.Set(MigrationTargetKey, targetFP); err != nil {
		logger.Errorf("metricstore", "[store-migration] failed to persist migration target fingerprint: %v", err)
	}
	finishStoreMigration("completed", nil)
	logger.Infof("metricstore", "[store-migration] completed via API (%d points) target_driver=%s", total, ResolveDriverFromConfig(cfg.Driver, cfg.DSN))
}

// finishStoreMigration unified finishing: set the final status, end time and error message.
func finishStoreMigration(status string, err error) {
	storeMigMu.Lock()
	defer storeMigMu.Unlock()
	storeMigProgress.Status = status
	storeMigProgress.EndTime = time.Now().UTC()
	if err != nil {
		storeMigProgress.Error = err.Error()
	}
	// When completed/cancelled, mark the current metric as fully processed to facilitate the return of the front-end progress bar.
	if status == "completed" && storeMigProgress.TotalMetrics > 0 {
		storeMigProgress.MetricsDone = storeMigProgress.TotalMetrics
	}
}

// CancelStoreMigration Requests to cancel a running store-to-store migration and wait for it to exit.
// Since writing is an idempotent upsert, it can be safely reinitiated after cancellation without generating duplicate data.
func CancelStoreMigration() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return stopStoreMigration(ctx, true)
}

func stopStoreMigration(ctx context.Context, requireRunning bool) error {
	storeMigMu.Lock()
	cancel := storeMigCancel
	done := storeMigDone
	storeMigMu.Unlock()

	if cancel == nil {
		if requireRunning {
			return fmt.Errorf("no store migration is currently running")
		}
		return nil
	}
	cancel()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for store migration to cancel: %w", ctx.Err())
	}
}

func stopStoreMigrationForClose(ctx context.Context) error {
	storeMigMu.Lock()
	storeClosing = true
	cancel := storeMigCancel
	done := storeMigDone
	storeMigMu.Unlock()

	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for store migration to cancel: %w", ctx.Err())
	}
}

func clearStoreClosing() {
	storeMigMu.Lock()
	storeClosing = false
	storeMigMu.Unlock()
}

func isStoreClosing() bool {
	storeMigMu.Lock()
	defer storeMigMu.Unlock()
	return storeClosing
}

// maskDSN performs coarse-grained desensitization on DSN to avoid leaking passwords in API responses/logs.
// Two common formats are processed: URL (scheme://user:pass@host/...) and key=value (password=...).
func maskDSN(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return ""
	}
	// key=value form: shield the password=... segment.
	if strings.Contains(dsn, "password=") {
		parts := strings.Fields(dsn)
		for i, p := range parts {
			if strings.HasPrefix(strings.ToLower(p), "password=") {
				parts[i] = "password=***"
			}
		}
		return strings.Join(parts, " ")
	}
	// URL / user:pass@host Form: Mask the password in user:pass@.
	if at := strings.LastIndex(dsn, "@"); at > 0 {
		head := dsn[:at]
		tail := dsn[at:]
		if colon := strings.LastIndex(head, ":"); colon >= 0 {
			// Keep scheme://user and block the password.
			return head[:colon] + ":***" + tail
		}
	}
	return dsn
}
