package metricstore

import (
	"context"
	"errors"
	"fmt"
	logger "github.com/komari-monitor/komari/utils/log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/metric"
)

var (
	store             *metric.Store
	storeFingerprint  string
	storeMu           sync.RWMutex
	storeInitMu       sync.Mutex
	storeOperations   = newStoreOperationGate()
	compactOperations = newStoreOperationGate()
	compactAt         int
)

var ErrCompactInProgress = errors.New("metric store compact already in progress")

const (
	// DefaultRollupRawRetention keeps a short hot raw window; older samples are
	// served from rollups after compaction.
	DefaultRollupRawRetention   = 15 * time.Minute
	DefaultRollupFinestTier     = time.Minute
	externalStoreInitTimeout    = 30 * time.Second
	checkpointRetryTimeout      = time.Second
	backgroundCheckpointTimeout = 10 * time.Second
	metricWALCheckpointLimit    = 64 * 1024 * 1024
)

// MetricStoreConfig saves the metric store configuration.
//
// Note: metric store is now always enabled (the old metric_store_enabled switch is deprecated).
// SQLite (./data/metrics.db) is used by default when not explicitly configured.
type MetricStoreConfig struct {
	Driver       string `json:"metric_db_driver" default:"sqlite"`         // Database type: sqlite, mysql, postgresql
	DSN          string `json:"metric_db_dsn" default:"./data/metrics.db"` // Database connection string
	TablePrefix  string `json:"metric_table_prefix" default:"metric_"`     // table name prefix
	MaxOpenConns int    `json:"metric_max_open_conns" default:"25"`        // Maximum number of connections
	MaxIdleConns int    `json:"metric_max_idle_conns" default:"5"`         // Maximum number of idle connections
}

// MetricStoreConfigKeys configuration keys
//
// MetricStoreEnabledKey Deprecated: metric store is always enabled, kept constant only to clean up old configuration.
const (
	MetricStoreEnabledKey = "metric_store_enabled" // Deprecated: metric store is always enabled
	MetricDBDriverKey     = "metric_db_driver"
	MetricDBDSNKey        = "metric_db_dsn"
	// MetricDownsamplingEnabledKey is retained only to normalize databases
	// created by releases that allowed downsampling to be disabled.
	MetricDownsamplingEnabledKey = "metric_downsampling_enabled"
	MetricTablePrefixKey         = "metric_table_prefix"
	MetricMaxOpenConnsKey        = "metric_max_open_conns"
	MetricMaxIdleConnsKey        = "metric_max_idle_conns"
	// MigrationTargetKey records the target fingerprint (driver+dsn) of the last successful startup migration.
	// When the target changes (such as switching from SQLite to MySQL/PostgreSQL), starting a migration will
	// Re-execute and move the data from the previous target database to the new target metrics database.
	MigrationTargetKey = "metric_migration_target"
)

// buildMetricConfig constructs the underlying metric.Config from MetricStoreConfig.
// autoMigrate controls whether to automatically create a table when Open: true during formal initialization/hot reloading.
// It is false when only doing connection testing (the schema is not written to avoid side effects on the target database).
func buildMetricConfig(cfg *MetricStoreConfig, autoMigrate bool) (metric.Config, error) {
	driver := ResolveDriverFromConfig(cfg.Driver, cfg.DSN)

	tablePrefix := cfg.TablePrefix
	if tablePrefix == "" {
		tablePrefix = "metric_"
	}
	opts := []metric.Option{
		metric.WithTablePrefix(tablePrefix),
		metric.WithAutoMigrate(autoMigrate),
	}
	resources := detectSQLiteResourceProfile()
	opts = append(opts, metric.WithHeavyReadConcurrency(resources.HeavyReadConcurrent))
	opts = append(opts, metric.WithRollupPolicy(defaultRollupPolicy()))

	switch driver {
	case metric.DriverSQLite:
		dsn := cfg.DSN
		if dsn == "" || dsn == "./data/metrics.db" {
			// Note: cache=shared is intentionally not used. SQLite shared cache mode uses table-level locks,
			// When one connection holds a read lock and another connection attempts to write, it will return immediately.
			// SQLITE_LOCKED ("database table is locked"), and busy_timeout
			// The table-level lock on the shared cache is invalid, and an error will inevitably be reported during migration when it is concurrent with front-end query/real-time writing.
			// _txlock=immediate allows the write lock to be acquired at the beginning of the write transaction to avoid lock upgrade deadlock.
			dsn = "file:./data/metrics.db?mode=rwc&_txlock=immediate"
		} else {
			// When users customize DSN, strip cache=shared to avoid the above table-level locking problem.
			dsn = stripSharedCache(dsn)
		}
		// SQLite serialized writes: fixed single write connection to avoid "database is locked" race,
		// At the same time, enable an independent WAL read-only connection pool to improve front-end query concurrency (writes still use a single main connection).
		// cfg.MaxOpenConns/MaxIdleConns is deliberately ignored here - multi-write connections for SQLite
		// It will only introduce lock contention rather than improve throughput.
		opts = append(opts,
			metric.WithMaxOpenConns(1),
			metric.WithMaxIdleConns(1),
			metric.WithConnMaxLifetime(0),
			metric.WithSQLiteCacheSizeKB(resources.WriterCacheKB),
			metric.WithSQLiteReadCacheSizeKB(resources.ReaderCacheKB),
			metric.WithSQLiteMMapSize(resources.MMapBytes),
			metric.WithSQLiteReadPool(resources.ReadPoolSize),
			metric.WithSQLiteHeavyReadConcurrency(resources.HeavyReadConcurrent),
		)
		return metric.SQLite(dsn, opts...), nil
	case metric.DriverMySQL:
		opts = append(opts,
			metric.WithMaxOpenConns(cfg.MaxOpenConns),
			metric.WithMaxIdleConns(cfg.MaxIdleConns),
		)
		return metric.MySQL(cfg.DSN, opts...), nil
	case metric.DriverPostgreSQL:
		opts = append(opts,
			metric.WithMaxOpenConns(cfg.MaxOpenConns),
			metric.WithMaxIdleConns(cfg.MaxIdleConns),
		)
		return metric.PostgreSQL(cfg.DSN, opts...), nil
	default:
		return metric.Config{}, fmt.Errorf("unsupported metric database driver: %s", cfg.Driver)
	}
}

func defaultRollupPolicy() metric.RollupPolicy {
	return metric.RollupPolicy{
		RawRetention: DefaultRollupRawRetention,
		Tiers: []metric.RollupTier{
			{Interval: DefaultRollupFinestTier, Retention: 48 * time.Hour},
			{Interval: 5 * time.Minute, Retention: 14 * 24 * time.Hour},
			{Interval: time.Hour, Retention: 14 * 24 * time.Hour},
		},
	}
}

// ResolveDriverFromConfig automatically infers the metrics database type based on DSN; when DSN is unreliable
// Fallback to the driver in the old configuration when identifying, to be compatible with existing configurations and non-conventional DSNs.
func ResolveDriverFromConfig(configuredDriver, dsn string) metric.Driver {
	if driver, ok := InferDriverFromDSN(dsn); ok {
		return driver
	}

	switch driver := metric.Driver(strings.ToLower(strings.TrimSpace(configuredDriver))); driver {
	case metric.DriverSQLite, metric.DriverMySQL, metric.DriverPostgreSQL:
		return driver
	default:
		return metric.DriverSQLite
	}
}

// InferDriverFromDSN tries to infer the database type based on common DSN formats.
// Returning ok=false indicates that the format is not clear enough and the caller should use the existing configuration as a fallback.
func InferDriverFromDSN(dsn string) (metric.Driver, bool) {
	raw := strings.TrimSpace(dsn)
	if raw == "" {
		return metric.DriverSQLite, true
	}
	lower := strings.ToLower(raw)

	// PostgreSQL URL DSN: postgres://... or postgresql://...
	if strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://") {
		return metric.DriverPostgreSQL, true
	}

	// SQLite common file/memory DSNs.
	if raw == ":memory:" || strings.HasPrefix(lower, "file:") || strings.HasPrefix(lower, "sqlite://") || strings.HasPrefix(lower, "sqlite3://") {
		return metric.DriverSQLite, true
	}

	// MySQL URL (although the go-sql-driver/mysql native DSN is not usually a URL, it is used here to give
	// Type inference; connection tests still verify that the DSN is acceptable to the driver).
	if strings.HasPrefix(lower, "mysql://") {
		return metric.DriverMySQL, true
	}

	// PostgreSQL Keyword/Value DSN: host=... user=... dbname=...
	if looksLikePostgreSQLKeyValueDSN(lower) {
		return metric.DriverPostgreSQL, true
	}

	// go-sql-driver/mysql DSN: user:pass@tcp(host:3306)/db, user@unix(...)/db, user:pass@/db, etc.
	if looksLikeMySQLDSN(lower) {
		return metric.DriverMySQL, true
	}

	// SQLite path: ./data/metrics.db, /var/lib/metrics.sqlite3, metrics.sqlite, etc.
	if looksLikeSQLitePath(lower) {
		return metric.DriverSQLite, true
	}

	return "", false
}

func looksLikePostgreSQLKeyValueDSN(lower string) bool {
	if !strings.Contains(lower, "=") || strings.Contains(lower, "://") {
		return false
	}
	keys := []string{"host=", "user=", "password=", "dbname=", "port=", "sslmode="}
	matched := 0
	for _, key := range keys {
		if strings.Contains(lower, key) {
			matched++
		}
	}
	// dbname= is basically a strong feature of PostgreSQL libpq DSN; otherwise matches at least two common keys.
	return strings.Contains(lower, "dbname=") || matched >= 2
}

func looksLikeMySQLDSN(lower string) bool {
	if strings.Contains(lower, "://") || strings.Contains(lower, " ") {
		return false
	}
	if strings.Contains(lower, "@tcp(") || strings.Contains(lower, "@unix(") || strings.Contains(lower, "@/") {
		return true
	}
	// Although user:pass@host/db, user@host/db are not recommended formats, they are obviously biased towards MySQL.
	return strings.Contains(lower, "@") && strings.Contains(lower, "/")
}

func looksLikeSQLitePath(lower string) bool {
	path := lower
	if idx := strings.IndexAny(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	return strings.HasSuffix(path, ".db") || strings.HasSuffix(path, ".sqlite") || strings.HasSuffix(path, ".sqlite3")
}

// stripSharedCache Remove cache=shared parameter from SQLite DSN to avoid problems in shared cache mode
// Table-level lock (SQLITE_LOCKED "database table is locked"). Other parameters remain unchanged.
func stripSharedCache(dsn string) string {
	if !strings.Contains(dsn, "cache=shared") {
		return dsn
	}
	idx := strings.Index(dsn, "?")
	if idx < 0 {
		return dsn
	}
	base := dsn[:idx]
	query := dsn[idx+1:]
	parts := strings.Split(query, "&")
	kept := parts[:0]
	for _, p := range parts {
		if p == "cache=shared" {
			continue
		}
		kept = append(kept, p)
	}
	if len(kept) == 0 {
		return base
	}
	return base + "?" + strings.Join(kept, "&")
}

// openStore opens the metric store as configured and creates the metric definition.
func openStore(ctx context.Context, cfg *MetricStoreConfig) (*metric.Store, error) {
	return openStoreWithDefaultRetention(ctx, cfg, defaultBuiltinMetricRetentionDays)
}

func openStoreWithDefaultRetention(ctx context.Context, cfg *MetricStoreConfig, defaultRetentionDays int) (*metric.Store, error) {
	return openStoreWithDefaultRetentionAndProgress(ctx, cfg, defaultRetentionDays, nil)
}

func openStoreWithDefaultRetentionAndProgress(ctx context.Context, cfg *MetricStoreConfig, defaultRetentionDays int, progress metric.MigrationProgressFunc) (*metric.Store, error) {
	metricCfg, err := buildMetricConfig(cfg, true)
	if err != nil {
		return nil, err
	}
	metricCfg.MigrationProgress = progress

	s, err := metric.Open(ctx, metricCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to open metric store: %w", err)
	}

	if err := createMetricDefinitionsWithDefaultRetention(ctx, s, defaultRetentionDays); err != nil {
		s.Close()
		return nil, fmt.Errorf("failed to create metric definitions: %w", err)
	}

	return s, nil
}

// OpenStore opens an isolated metric store using the supplied configuration.
// It is used by the pre-start upgrade flow before the process-wide store is
// initialized. The caller owns the returned store and must close it.
func OpenStore(ctx context.Context, cfg *MetricStoreConfig) (*metric.Store, error) {
	return openStore(ctx, cfg)
}

// OpenStoreForMigration opens an isolated target and uses the legacy data span
// as the initial retention for definitions that do not exist yet. Existing
// definitions keep their configured retention, including an explicit zero.
func OpenStoreForMigration(ctx context.Context, cfg *MetricStoreConfig, legacyRetentionDays int) (*metric.Store, error) {
	return OpenStoreForMigrationWithProgress(ctx, cfg, legacyRetentionDays, nil)
}

// OpenStoreForMigrationWithProgress is OpenStoreForMigration with observable
// SQLite schema migration progress.
func OpenStoreForMigrationWithProgress(ctx context.Context, cfg *MetricStoreConfig, legacyRetentionDays int, progress metric.MigrationProgressFunc) (*metric.Store, error) {
	if legacyRetentionDays < defaultBuiltinMetricRetentionDays {
		legacyRetentionDays = defaultBuiltinMetricRetentionDays
	}
	return openStoreWithDefaultRetentionAndProgress(ctx, cfg, legacyRetentionDays, progress)
}

// InspectSQLiteStorageMigration checks whether the configured metric store
// needs a potentially long local SQLite schema migration. It is read-only.
func InspectSQLiteStorageMigration(ctx context.Context) (metric.SQLiteMigrationSummary, error) {
	cfg, err := config.GetManyAs[MetricStoreConfig]()
	if err != nil {
		return metric.SQLiteMigrationSummary{}, fmt.Errorf("failed to load metric store config: %w", err)
	}
	metricCfg, err := buildMetricConfig(cfg, false)
	if err != nil {
		return metric.SQLiteMigrationSummary{}, err
	}
	return metric.InspectSQLiteMigration(ctx, metricCfg)
}

// OpenConfiguredStoreForMigration runs the configured store's automatic
// schema migration without activating it as the process-wide store.
func OpenConfiguredStoreForMigration(ctx context.Context, progress metric.MigrationProgressFunc) (*metric.Store, error) {
	cfg, err := config.GetManyAs[MetricStoreConfig]()
	if err != nil {
		return nil, fmt.Errorf("failed to load metric store config: %w", err)
	}
	return openStoreWithDefaultRetentionAndProgress(ctx, cfg, defaultBuiltinMetricRetentionDays, progress)
}

// TestConnection attempts to connect to the metrics database using the given configuration (does not affect the currently running store).
// Only open the connection and ping, do not perform automatic table creation, and close the connection immediately after the connection is successful. Returns a readable error on failure.
func TestConnection(ctx context.Context, cfg *MetricStoreConfig) error {
	metricCfg, err := buildMetricConfig(cfg, false)
	if err != nil {
		return err
	}

	s, err := metric.Open(ctx, metricCfg)
	if err != nil {
		return err
	}
	defer s.Close()

	return s.Ping(ctx)
}

// InitializeStore Initializes the metric store (called at startup, can be retried after failure or shutdown).
func InitializeStore() error {
	storeInitMu.Lock()
	defer storeInitMu.Unlock()

	storeMu.RLock()
	initialized := store != nil
	storeMu.RUnlock()
	if initialized {
		return nil
	}

	cfg, err := config.GetManyAs[MetricStoreConfig]()
	if err != nil {
		return fmt.Errorf("failed to load metric store config: %w", err)
	}

	// SQLite V3/V4 migration can legitimately take longer than a connection
	// timeout. Remote stores retain a bounded startup deadline.
	ctx, cancel := startupStoreContext(cfg)
	defer cancel()

	s, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}

	storeMu.Lock()
	store = s
	storeFingerprint = targetFingerprint(cfg)
	storeMu.Unlock()
	resetRuntimeStatus(s.Driver())
	clearStoreClosing()

	logger.Infof("metricstore", "Metric store initialized successfully (driver=%s)", ResolveDriverFromConfig(cfg.Driver, cfg.DSN))
	return nil
}

func startupStoreContext(cfg *MetricStoreConfig) (context.Context, context.CancelFunc) {
	if ResolveDriverFromConfig(cfg.Driver, cfg.DSN) == metric.DriverSQLite {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), externalStoreInitTimeout)
}

// Reload hot-reloads the metric store according to the latest configuration without restarting the process.
// Metric store is always enabled: open and create tables with new configuration (the connection has been Pinged internally to verify the connection),
// After success, replace the running store and finally close the old instance. Any failure will leave the old store unchanged.
//
// Note: Reload only switches running connections and will not restore historical data in old targets (such as SQLite).
// Move to a new target (such as MySQL/PostgreSQL). Cross-database data migration is started by migration
// (RunStartupMigration) is automatically completed by target fingerprint on next startup.
func Reload(ctx context.Context) error {
	if err := storeOperations.Acquire(ctx); err != nil {
		return fmt.Errorf("wait for metric store operations before reload: %w", err)
	}
	defer storeOperations.Release()
	if isStoreClosing() {
		return ErrStoreBusy
	}

	cfg, err := config.GetManyAs[MetricStoreConfig]()
	if err != nil {
		return fmt.Errorf("failed to load metric store config: %w", err)
	}

	// Open and create the table with the new configuration (the connection has been pinged internally).
	s, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}

	storeMu.Lock()
	old := store
	store = s
	storeFingerprint = targetFingerprint(cfg)
	storeMu.Unlock()
	resetRuntimeStatus(s.Driver())

	if old != nil {
		if cerr := old.Close(); cerr != nil {
			logger.Errorf("metricstore", "Failed to close previous metric store on reload: %v", cerr)
		}
	}

	logger.Infof("metricstore", "Metric store reloaded successfully (driver=%s)", ResolveDriverFromConfig(cfg.Driver, cfg.DSN))
	return nil
}

// GetStore gets the metric store instance (returns nil if not enabled)
func GetStore() *metric.Store {
	storeMu.RLock()
	defer storeMu.RUnlock()
	return store
}

// RetentionSummary is the compatibility view of all persisted metric policies.
type RetentionSummary struct {
	AllPositive bool
	MaxDays     int
}

// GetRetentionSummary aggregates the active store's metric definitions. An
// empty definition set is not considered record-enabled.
func GetRetentionSummary(ctx context.Context) (RetentionSummary, error) {
	s := GetStore()
	if s == nil {
		return RetentionSummary{}, fmt.Errorf("metric store not initialized")
	}
	defs, err := s.ListMetrics(ctx)
	if err != nil {
		return RetentionSummary{}, err
	}
	return summarizeRetentionDefinitions(defs), nil
}

func summarizeRetentionDefinitions(defs []metric.Definition) RetentionSummary {
	if len(defs) == 0 {
		return RetentionSummary{}
	}
	summary := RetentionSummary{AllPositive: true}
	for _, def := range defs {
		if def.RetentionDays <= 0 {
			summary.AllPositive = false
		}
		if def.RetentionDays > summary.MaxDays {
			summary.MaxDays = def.RetentionDays
		}
	}
	return summary
}

func Compact(ctx context.Context, now time.Time) (int, error) {
	if !compactOperations.TryAcquire() {
		return 0, ErrCompactInProgress
	}
	defer compactOperations.Release()
	if err := storeOperations.AcquireShared(ctx); err != nil {
		return 0, fmt.Errorf("wait for metric store operation before compaction: %w", err)
	}
	defer storeOperations.ReleaseShared()

	storeMu.RLock()
	defer storeMu.RUnlock()
	activeStore := store
	if activeStore == nil {
		return 0, fmt.Errorf("metric store not initialized")
	}

	defs, err := activeStore.ListMetrics(ctx)
	if err != nil {
		return 0, err
	}
	defs = compactableMetricDefinitions(activeStore, defs)
	if len(defs) == 0 {
		compactAt = 0
		return 0, nil
	}
	if compactAt >= len(defs) {
		compactAt = 0
	}

	total := 0
	start := compactAt
	failedAt := -1
	var compactErrors []error
	for i := 0; i < len(defs); i++ {
		idx := (start + i) % len(defs)
		metricName := defs[idx].Name
		n, err := activeStore.CompactMetric(ctx, metricName, now)
		if metric.IsDigestHandoffDeferred(err) {
			handleDigestHandoffDeferred(metricName, err, time.Now().UTC())
			continue
		}
		if err != nil {
			if failedAt < 0 {
				failedAt = idx
			}
			compactErrors = append(compactErrors, fmt.Errorf("compact metric %q: %w", metricName, err))
			continue
		}
		clearDigestHandoffDeferred(metricName)
		total += n
	}
	if err := finishCompactCycle(ctx, activeStore, now, true); err != nil {
		compactErrors = append(compactErrors, err)
	}
	if failedAt >= 0 {
		compactAt = failedAt
	} else {
		compactAt = start
	}
	return total, errors.Join(compactErrors...)
}

func handleDigestHandoffDeferred(metricName string, err error, at time.Time) {
	reason := digestHandoffDeferredReason(err)
	recordDigestHandoffDeferred(metricName, reason, at)
	logger.Infof("metricstore", "Digest handoff deferred; raw data retained and will be retried during a later compaction cycle: metric=%q; reason=%s; detail=%v", metricName, reason, err)
}

func digestHandoffDeferredReason(err error) string {
	detail := err.Error()
	coordinate := ""
	if index := strings.Index(detail, "series="); index >= 0 {
		coordinate = detail[index:]
	} else if index := strings.Index(detail, "bucket="); index >= 0 {
		coordinate = detail[index:]
	}
	if index := strings.IndexAny(coordinate, "\r\n"); index >= 0 {
		coordinate = coordinate[:index]
	}

	reason := "Fine-grained and coarse-grained digest verification has not passed yet"
	if strings.Contains(detail, "finer digest missing") {
		reason = "Fine-grained digest is not yet complete"
	}
	if coordinate != "" {
		reason += "(" + coordinate + ")"
	}
	return reason + "; raw data retained and will be retried during a later compaction cycle"
}

// CompactStep compacts one metric and advances the rotating cursor. Cleanup
// and the SQLite WAL checkpoint run only after the cursor completes a cycle,
// keeping scheduled maintenance work short on low-performance single-core
// servers. A failed metric still advances so it cannot block the other metrics.
func CompactStep(ctx context.Context, now time.Time) (written int, cycleCompleted bool, err error) {
	if !compactOperations.TryAcquire() {
		return 0, false, ErrCompactInProgress
	}
	defer compactOperations.Release()
	if err := storeOperations.AcquireShared(ctx); err != nil {
		return 0, false, fmt.Errorf("wait for metric store operation before compaction: %w", err)
	}
	defer storeOperations.ReleaseShared()

	storeMu.RLock()
	defer storeMu.RUnlock()
	activeStore := store
	if activeStore == nil {
		return 0, false, fmt.Errorf("metric store not initialized")
	}
	checkpointRetried := retryMetricWALCheckpoint(ctx, activeStore, time.Now().UTC())

	defs, err := activeStore.ListMetrics(ctx)
	if err != nil {
		return 0, false, err
	}
	defs = compactableMetricDefinitions(activeStore, defs)
	if len(defs) == 0 {
		compactAt = 0
		cycleErr := finishCompactCycle(ctx, activeStore, now, !checkpointRetried)
		finishEmptyCompactCycle(activeStore.Driver(), cycleErr, time.Now().UTC())
		return 0, true, cycleErr
	}
	if compactAt < 0 || compactAt >= len(defs) {
		compactAt = 0
	}

	idx := compactAt
	compactAt = (compactAt + 1) % len(defs)
	cycleCompleted = compactAt == 0
	beginCompactStep(activeStore.Driver(), defs[idx].Name, idx, len(defs), time.Now().UTC())
	metricName := defs[idx].Name
	written, compactErr := activeStore.CompactMetric(ctx, metricName, now)
	if metric.IsDigestHandoffDeferred(compactErr) {
		handleDigestHandoffDeferred(metricName, compactErr, time.Now().UTC())
		compactErr = nil
	} else if compactErr == nil {
		clearDigestHandoffDeferred(metricName)
	} else {
		compactErr = fmt.Errorf("compact metric %q: %w", metricName, compactErr)
	}
	if !cycleCompleted {
		if !checkpointRetried {
			checkpointLargeMetricWAL(ctx, activeStore)
		}
		finishCompactStep(written, false, compactErr, time.Now().UTC())
		return written, false, compactErr
	}
	cycleErr := errors.Join(compactErr, finishCompactCycle(ctx, activeStore, now, !checkpointRetried))
	finishCompactStep(written, true, cycleErr, time.Now().UTC())
	return written, true, cycleErr
}

func compactableMetricDefinitions(activeStore *metric.Store, definitions []metric.Definition) []metric.Definition {
	filtered := definitions[:0]
	for _, definition := range definitions {
		if activeStore.IsVirtualMetric(definition.Name) {
			continue
		}
		filtered = append(filtered, definition)
	}
	return filtered
}

func checkpointLargeMetricWAL(ctx context.Context, activeStore *metric.Store) {
	if checkpointIsPending() {
		return
	}
	checkpointed, err := checkpointMetricWALAbove(ctx, activeStore, metricWALCheckpointLimit)
	if err != nil && !checkpointed {
		logger.Warnf("metricstore", "Failed to inspect metric WAL before threshold checkpoint: %v", err)
		return
	}
	if !checkpointed {
		return
	}
	recordCheckpointResult(activeStore.Driver(), err, time.Now().UTC())
	if err != nil {
		logger.Warnf("metricstore", "Failed to truncate oversized metric WAL: %v", err)
	}
}

func checkpointMetricWALAbove(ctx context.Context, activeStore *metric.Store, limit int64) (bool, error) {
	if activeStore.Driver() != metric.DriverSQLite || limit <= 0 {
		return false, nil
	}
	files, err := activeStore.SQLiteFiles(ctx)
	if err != nil || files.WAL < limit {
		return false, err
	}
	checkpointCtx, cancel := context.WithTimeout(ctx, backgroundCheckpointTimeout)
	defer cancel()
	return true, activeStore.CheckpointWAL(checkpointCtx)
}

func metricWALCheckpointTimeout(size int64) time.Duration {
	if size >= metricWALCheckpointLimit {
		return backgroundCheckpointTimeout
	}
	return checkpointRetryTimeout
}

func finishCompactCycle(ctx context.Context, activeStore *metric.Store, now time.Time, allowCheckpoint bool) error {
	var compactErrors []error
	if _, err := activeStore.CleanupExpired(ctx, now); err != nil {
		compactErrors = append(compactErrors, fmt.Errorf("clean up expired raw metrics: %w", err))
	}
	if activeStore.Driver() == metric.DriverSQLite && allowCheckpoint && !checkpointIsPending() {
		files, err := activeStore.SQLiteFiles(ctx)
		if err != nil {
			logger.Warnf("metricstore", "Failed to inspect metric WAL after compaction: %v", err)
			return errors.Join(compactErrors...)
		}
		if files.WAL == 0 {
			return errors.Join(compactErrors...)
		}
		checkpointTimeout := metricWALCheckpointTimeout(files.WAL)
		checkpointCtx, cancel := context.WithTimeout(ctx, checkpointTimeout)
		checkpointErr := activeStore.CheckpointWAL(checkpointCtx)
		cancel()
		recordCheckpointResult(activeStore.Driver(), checkpointErr, time.Now().UTC())
		if checkpointErr != nil {
			logger.Warnf("metricstore", "Failed to truncate metric WAL after compaction; deferred retry will keep the WAL intact: %v", checkpointErr)
		}
	}
	return errors.Join(compactErrors...)
}

func retryMetricWALCheckpoint(ctx context.Context, activeStore *metric.Store, at time.Time) bool {
	pending, quickDue, fullDue := checkpointRetryState(at)
	if !pending {
		return false
	}
	if activeStore.Driver() != metric.DriverSQLite {
		clearCheckpointForExternal(activeStore.Driver())
		return true
	}
	if !quickDue {
		return false
	}

	retryTimeout := checkpointRetryTimeout
	fullRetry := false
	if fullDue {
		if files, err := activeStore.SQLiteFiles(ctx); err == nil && files.WAL >= metricWALCheckpointLimit {
			retryTimeout = backgroundCheckpointTimeout
			fullRetry = true
		}
	}
	retryCtx, cancel := context.WithTimeout(ctx, retryTimeout)
	err := activeStore.CheckpointWAL(retryCtx)
	cancel()
	recordCheckpointResult(activeStore.Driver(), err, at)
	if err != nil && fullRetry {
		deferFullCheckpointRetry(at)
	}
	return true
}

// CloseStoreContext stops the asynchronous store migration before taking the
// store write lock, so shutdown cannot wait forever on the migration's lease.
func CloseStoreContext(ctx context.Context) error {
	if err := stopStoreMigrationForClose(ctx); err != nil {
		clearStoreClosing()
		return err
	}
	if err := storeOperations.Acquire(ctx); err != nil {
		clearStoreClosing()
		return fmt.Errorf("wait for metric store operations before close: %w", err)
	}
	defer storeOperations.Release()

	storeMu.Lock()
	defer storeMu.Unlock()

	if store != nil {
		err := store.Close()
		store = nil
		storeFingerprint = ""
		resetRuntimeStatus("")
		return err
	}
	storeFingerprint = ""
	resetRuntimeStatus("")
	return nil
}

const defaultBuiltinMetricRetentionDays = 1

// createMetricDefinitions creates built-in definitions with explicit policies.
func createMetricDefinitions(ctx context.Context, s *metric.Store) error {
	return createMetricDefinitionsWithDefaultRetention(ctx, s, defaultBuiltinMetricRetentionDays)
}

// EnsureBuiltinMetricDefinitions registers definitions for the server's
// built-in report and ping writers before a standalone Store receives points.
func EnsureBuiltinMetricDefinitions(ctx context.Context, s *metric.Store) error {
	return createMetricDefinitions(ctx, s)
}

func createMetricDefinitionsWithDefaultRetention(ctx context.Context, s *metric.Store, defaultRetentionDays int) error {
	if defaultRetentionDays < defaultBuiltinMetricRetentionDays {
		defaultRetentionDays = defaultBuiltinMetricRetentionDays
	}
	definitions := []metric.Definition{
		{Name: MetricCPU, Type: metric.TypeGauge, Unit: "%", Description: "CPU usage percentage", RetentionDays: defaultRetentionDays},
		{Name: MetricGPU, Type: metric.TypeGauge, Unit: "%", Description: "GPU usage percentage", RetentionDays: defaultRetentionDays},
		{Name: MetricGPUDeviceUsage, Type: metric.TypeGauge, Unit: "%", Description: "Per-device GPU utilization", RetentionDays: defaultRetentionDays},
		{Name: MetricGPUMem, Type: metric.TypeGauge, Unit: "bytes", Description: "GPU memory used", RetentionDays: defaultRetentionDays},
		{Name: MetricGPUMemTotal, Type: metric.TypeGauge, Unit: "bytes", Description: "GPU memory total", RetentionDays: defaultRetentionDays},
		{Name: MetricGPUTemp, Type: metric.TypeGauge, Unit: "°C", Description: "GPU temperature", RetentionDays: defaultRetentionDays},
		{Name: MetricRAM, Type: metric.TypeGauge, Unit: "bytes", Description: "RAM used", RetentionDays: defaultRetentionDays},
		{Name: MetricSwap, Type: metric.TypeGauge, Unit: "bytes", Description: "Swap used", RetentionDays: defaultRetentionDays},
		{Name: MetricLoad, Type: metric.TypeGauge, Unit: "", Description: "System load average", RetentionDays: defaultRetentionDays},
		{Name: MetricDisk, Type: metric.TypeGauge, Unit: "bytes", Description: "Disk used", RetentionDays: defaultRetentionDays},
		{Name: MetricNetIn, Type: metric.TypeGauge, Unit: "bytes/s", Description: "Network in rate", RetentionDays: defaultRetentionDays},
		{Name: MetricNetOut, Type: metric.TypeGauge, Unit: "bytes/s", Description: "Network out rate", RetentionDays: defaultRetentionDays},
		{Name: MetricNetTotalUp, Type: metric.TypeCounter, Unit: "bytes", Description: "Network total upload", RetentionDays: defaultRetentionDays},
		{Name: MetricNetTotalDown, Type: metric.TypeCounter, Unit: "bytes", Description: "Network total download", RetentionDays: defaultRetentionDays},
		{Name: MetricTrafficUp, Type: metric.TypeGauge, Unit: "bytes", Description: "Traffic upload delta", RetentionDays: defaultRetentionDays},
		{Name: MetricTrafficDown, Type: metric.TypeGauge, Unit: "bytes", Description: "Traffic download delta", RetentionDays: defaultRetentionDays},
		{Name: MetricProcess, Type: metric.TypeGauge, Unit: "count", Description: "Process count", RetentionDays: defaultRetentionDays},
		{Name: MetricConnections, Type: metric.TypeGauge, Unit: "count", Description: "TCP connections", RetentionDays: defaultRetentionDays},
		{Name: MetricConnectionsUDP, Type: metric.TypeGauge, Unit: "count", Description: "UDP connections", RetentionDays: defaultRetentionDays},
		{Name: MetricPingLatency, Type: metric.TypeGauge, Unit: "ms", Description: "Ping latency", RetentionDays: defaultRetentionDays},
		{Name: MetricPingLoss, Type: metric.TypeGauge, Unit: "ratio", Description: "Ping packet loss indicator", RetentionDays: defaultRetentionDays},
	}

	for _, def := range definitions {
		existing, err := s.GetMetric(ctx, def.Name)
		if err != nil && !errors.Is(err, metric.ErrNotFound) {
			return fmt.Errorf("failed to get metric %s: %w", def.Name, err)
		}
		if err == nil {
			if existing.RetentionDays == 0 {
				if _, err := s.SetMetricRetention(ctx, def.Name, 0); err != nil {
					return fmt.Errorf("failed to preserve disabled metric %s: %w", def.Name, err)
				}
				continue
			}
			def.RetentionDays = existing.RetentionDays
		}
		if err := s.UpsertMetric(ctx, def); err != nil {
			return fmt.Errorf("failed to create metric %s: %w", def.Name, err)
		}
	}
	return nil
}

// WritePingRecord writes ping records to metric store
func WritePingRecord(ctx context.Context, rec models.PingRecord) error {
	assignment := PingAssignment{Client: rec.Client, TaskID: rec.TaskId}
	if EntityWritesBlocked(rec.Client) || PingTaskWritesBlocked(rec.TaskId) || PingAssignmentWritesBlocked(assignment) {
		return ErrMetricWriteBlocked
	}
	if err := storeOperations.AcquireShared(ctx); err != nil {
		return fmt.Errorf("wait for metric store operation before writing ping record: %w", err)
	}
	defer storeOperations.ReleaseShared()
	if EntityWritesBlocked(rec.Client) || PingTaskWritesBlocked(rec.TaskId) || PingAssignmentWritesBlocked(assignment) {
		return ErrMetricWriteBlocked
	}
	s := GetStore()
	if s == nil {
		return fmt.Errorf("metric store not enabled")
	}

	ts := rec.Time
	entityID := rec.Client
	tags := map[string]string{
		"task_id": fmt.Sprintf("%d", rec.TaskId),
	}

	points := []metric.Point{
		{
			MetricName: MetricPingLatency,
			EntityID:   entityID,
			Timestamp:  ts,
			Value:      float64(rec.Value),
			Tags:       tags,
		},
	}
	if s.Driver() != metric.DriverSQLite {
		loss := 0.0
		if rec.Value < 0 {
			loss = 1
		}
		points = append(points, metric.Point{
			MetricName: MetricPingLoss,
			EntityID:   entityID,
			Timestamp:  ts,
			Value:      loss,
			Tags:       tags,
		})
	}

	return s.WriteBatch(ctx, points)
}

// GetRecordsByClientAndTime queries records from metric store and reconstructs them into models.Record
func GetRecordsByClientAndTime(ctx context.Context, clientUUID string, start, end time.Time) ([]models.Record, error) {
	return GetRecordsByClientAndTimeForLoadType(ctx, clientUUID, start, end, "all")
}

// GetRecordsByClientAndTimeForLoadType reads only the metric families needed
// by a projected legacy record response. Fields outside that family remain at
// their zero value, preserving the existing response shape.
func GetRecordsByClientAndTimeForLoadType(ctx context.Context, clientUUID string, start, end time.Time, loadType string) ([]models.Record, error) {
	return GetRecordsByClientAndTimeForLoadTypeMaxPoints(ctx, clientUUID, start, end, loadType, 500)
}

// GetRecordsByClientAndTimeForLoadTypeMaxPoints bounds the reconstructed
// timeline before legacy response objects are allocated. The storage query can
// therefore select a coarser materialized tier for large report windows.
func GetRecordsByClientAndTimeForLoadTypeMaxPoints(ctx context.Context, clientUUID string, start, end time.Time, loadType string, maxPoints int) ([]models.Record, error) {
	s := GetStore()
	if s == nil {
		return nil, fmt.Errorf("metric store not enabled")
	}

	return getRecordsByClientAndTimeFromSeries(ctx, s, clientUUID, start, end, recordMetricNamesForLoadType(loadType), recordClientMaxPoints(maxPoints))
}

// GetTrafficRecordsByClientAndTime reconstructs only the four traffic series
// needed by report accounting. Avoiding unrelated system metrics keeps ledger
// settlement inexpensive on low-resource servers.
func GetTrafficRecordsByClientAndTime(ctx context.Context, clientUUID string, start, end time.Time) ([]models.Record, error) {
	s := GetStore()
	if s == nil {
		return nil, fmt.Errorf("metric store not enabled")
	}

	return getRecordsByClientAndTimeFromSeries(ctx, s, clientUUID, start, end, []string{
		MetricNetTotalUp,
		MetricNetTotalDown,
		MetricTrafficUp,
		MetricTrafficDown,
	}, 500)
}

// GetTrafficRecordsByClientsAndTime reads the dashboard traffic window with a
// fixed number of metric scans while retaining the client dimension. It also
// returns the closest counter baseline before start for every requested client.
type DashboardTrafficRecord struct {
	Client         string
	Time           time.Time
	NetTotalUp     int64
	NetTotalDown   int64
	TrafficUp      int64
	TrafficDown    int64
	TrafficUpSet   bool
	TrafficDownSet bool
}

func GetTrafficRecordsByClientsAndTime(ctx context.Context, clientUUIDs []string, start, end time.Time) ([]DashboardTrafficRecord, map[string]DashboardTrafficRecord, error) {
	s := GetStore()
	if s == nil {
		return nil, nil, fmt.Errorf("metric store not enabled")
	}
	if end.Before(start) {
		return nil, nil, fmt.Errorf("traffic metric range end precedes start")
	}

	requested := make(map[string]struct{}, len(clientUUIDs))
	for _, clientUUID := range clientUUIDs {
		if clientUUID != "" {
			requested[clientUUID] = struct{}{}
		}
	}
	if len(requested) == 0 {
		return []DashboardTrafficRecord{}, map[string]DashboardTrafficRecord{}, nil
	}

	now := time.Now().UTC()
	interval := recordSeriesInterval(s, start, end, now, 500)
	recordMap := make(map[recordSeriesKey]DashboardTrafficRecord, len(requested)*recordClientMaxPoints(500))
	trafficMetrics := []string{MetricNetTotalUp, MetricNetTotalDown, MetricTrafficUp, MetricTrafficDown}
	queries := make([]metric.AggregateQuery, 0, len(trafficMetrics))
	for _, metricName := range trafficMetrics {
		queries = append(queries, metric.AggregateQuery{
			Query: metric.Query{
				MetricName: metricName,
				Start:      start,
				End:        end,
				Order:      metric.OrderAsc,
			},
			Aggregation:    recordMetricAggregation(metricName),
			Interval:       interval,
			PreserveSeries: true,
			OmitTags:       true,
		})
	}
	series, err := s.DashboardSeriesBatch(ctx, queries, now)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query dashboard traffic metrics: %w", err)
	}
	for index, points := range series {
		metricName := trafficMetrics[index]
		for _, point := range points {
			if _, ok := requested[point.EntityID]; !ok {
				continue
			}
			key := recordSeriesKey{client: point.EntityID, ts: point.Bucket.Unix()}
			record, exists := recordMap[key]
			if !exists {
				record = DashboardTrafficRecord{Client: point.EntityID, Time: point.Bucket.UTC()}
			}
			applyDashboardTrafficMetricValue(&record, metricName, point.Value)
			recordMap[key] = record
		}
	}

	baselines := make(map[string]DashboardTrafficRecord, len(requested))
	baselineStart := start.Add(-interval)
	baselineEnd := start.Add(-time.Nanosecond)
	baselineMetrics := []string{MetricNetTotalUp, MetricNetTotalDown}
	baselineQueries := make([]metric.AggregateQuery, 0, len(baselineMetrics))
	for _, metricName := range baselineMetrics {
		baselineQueries = append(baselineQueries, metric.AggregateQuery{
			Query: metric.Query{
				MetricName: metricName,
				Start:      baselineStart,
				End:        baselineEnd,
				Order:      metric.OrderAsc,
			},
			Aggregation:    metric.AggLast,
			Interval:       interval,
			PreserveSeries: true,
			OmitTags:       true,
		})
	}
	baselineSeries, err := s.DashboardSeriesBatch(ctx, baselineQueries, now)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query dashboard traffic baselines: %w", err)
	}
	for index, points := range baselineSeries {
		metricName := baselineMetrics[index]
		for _, point := range points {
			if _, ok := requested[point.EntityID]; !ok {
				continue
			}
			baseline := baselines[point.EntityID]
			baseline.Client = point.EntityID
			if point.Bucket.After(baseline.Time) {
				baseline.Time = point.Bucket.UTC()
			}
			applyDashboardTrafficMetricValue(&baseline, metricName, point.Value)
			baselines[point.EntityID] = baseline
		}
	}

	missing := make([]string, 0)
	for clientUUID := range requested {
		if _, ok := baselines[clientUUID]; !ok {
			missing = append(missing, clientUUID)
		}
	}
	if len(missing) > 0 {
		fallback, err := GetLatestTrafficBefore(ctx, missing, start)
		if err != nil {
			return nil, nil, err
		}
		for clientUUID, baseline := range fallback {
			baselines[clientUUID] = DashboardTrafficRecord{
				Client: clientUUID, Time: baseline.Time,
				NetTotalUp: baseline.NetTotalUp, NetTotalDown: baseline.NetTotalDown,
			}
		}
	}

	records := make([]DashboardTrafficRecord, 0, len(recordMap))
	for _, record := range recordMap {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Client != records[j].Client {
			return records[i].Client < records[j].Client
		}
		return records[i].Time.Before(records[j].Time)
	})
	return records, baselines, nil
}

func applyDashboardTrafficMetricValue(record *DashboardTrafficRecord, metricName string, value float64) {
	switch metricName {
	case MetricNetTotalUp:
		record.NetTotalUp = int64(value)
	case MetricNetTotalDown:
		record.NetTotalDown = int64(value)
	case MetricTrafficUp:
		record.TrafficUp = int64(value)
		record.TrafficUpSet = true
	case MetricTrafficDown:
		record.TrafficDown = int64(value)
		record.TrafficDownSet = true
	}
}

// GetRecordsByTime queries the records of all clients within the time range from the metric store
func GetRecordsByTime(ctx context.Context, start, end time.Time) ([]models.Record, error) {
	return GetRecordsByTimeForLoadType(ctx, start, end, "all")
}

// GetRecordsByTimeForLoadType is the all-client counterpart of
// GetRecordsByClientAndTimeForLoadType.
func GetRecordsByTimeForLoadType(ctx context.Context, start, end time.Time, loadType string) ([]models.Record, error) {
	return GetRecordsByTimeForLoadTypeMaxPoints(ctx, start, end, loadType, -1)
}

// GetRecordsByTimeForLoadTypeMaxPoints applies a global response budget before
// reconstructing per-node records. A two-times oversampling margin preserves
// proportional sampling when nodes have uneven reporting cadence while still
// avoiding the old 500-points-per-node temporary result on large installations.
func GetRecordsByTimeForLoadTypeMaxPoints(ctx context.Context, start, end time.Time, loadType string, maxPoints int) ([]models.Record, error) {
	s := GetStore()
	if s == nil {
		return nil, fmt.Errorf("metric store not enabled")
	}
	metricNames := recordMetricNamesForLoadType(loadType)

	interval := recordSeriesInterval(s, start, end, time.Now().UTC(), 500)
	entityIDs, err := listRecordEntityIDs(ctx, s, start, end, interval, metricNames)
	if err != nil {
		return nil, err
	}
	perEntityMaxPoints := recordPerEntityMaxPoints(maxPoints, len(entityIDs))
	var records []models.Record
	for _, entityID := range entityIDs {
		items, err := getRecordsByClientAndTimeFromSeries(ctx, s, entityID, start, end, metricNames, perEntityMaxPoints)
		if err != nil {
			return nil, err
		}
		records = append(records, items...)
	}
	sortRecords(records)
	return records, nil
}

func recordMetricNamesForLoadType(loadType string) []string {
	switch strings.ToLower(strings.TrimSpace(loadType)) {
	case "cpu":
		return []string{MetricCPU}
	case "gpu":
		return []string{MetricGPU}
	case "ram":
		return []string{MetricRAM}
	case "swap":
		return []string{MetricSwap}
	case "load":
		return []string{MetricLoad}
	case "temp":
		// Temperature was never persisted as an entity-level metric. Retain the
		// old response cadence while avoiding a full 15-series scan.
		return []string{MetricCPU}
	case "disk":
		return []string{MetricDisk}
	case "net_in", "netin":
		return []string{MetricNetIn}
	case "net_out", "netout":
		return []string{MetricNetOut}
	case "network":
		return []string{MetricNetIn, MetricNetOut, MetricNetTotalUp, MetricNetTotalDown}
	case "process":
		return []string{MetricProcess}
	case "connections":
		return []string{MetricConnections, MetricConnectionsUDP}
	default:
		return loadRecordMetricNames
	}
}

type recordSeriesKey struct {
	client string
	ts     int64
}

func getRecordsByClientAndTimeFromSeries(ctx context.Context, s *metric.Store, clientUUID string, start, end time.Time, metricNames []string, maxPoints int) ([]models.Record, error) {
	now := time.Now().UTC()
	interval := recordSeriesInterval(s, start, end, now, maxPoints)
	recordMap := make(map[recordSeriesKey]*models.Record)
	queries := make([]metric.AggregateQuery, 0, len(metricNames))
	for _, metricName := range metricNames {
		queries = append(queries, metric.AggregateQuery{
			Query: metric.Query{
				MetricName: metricName,
				EntityID:   clientUUID,
				Start:      start,
				End:        end,
				Order:      metric.OrderAsc,
			},
			Aggregation: recordMetricAggregation(metricName),
			Interval:    interval,
		})
	}
	series, err := s.SeriesBatch(ctx, queries, now)
	if err != nil {
		return nil, fmt.Errorf("failed to query record metric batch: %w", err)
	}
	for index, points := range series {
		metricName := metricNames[index]
		for _, point := range points {
			entityID := point.EntityID
			if entityID == "" {
				entityID = clientUUID
			}
			key := recordSeriesKey{client: entityID, ts: point.Bucket.Unix()}
			if recordMap[key] == nil {
				recordMap[key] = &models.Record{
					Client: entityID,
					Time:   point.Bucket.UTC(),
				}
			}
			applyRecordMetricValue(recordMap[key], metricName, point.Value)
		}
	}

	records := make([]models.Record, 0, len(recordMap))
	for _, rec := range recordMap {
		records = append(records, *rec)
	}
	sortRecords(records)
	return records, nil
}

func recordMetricAggregation(metricName string) metric.Aggregation {
	switch metricName {
	case MetricTrafficUp, MetricTrafficDown:
		return metric.AggSum
	case MetricNetTotalUp, MetricNetTotalDown:
		return metric.AggLast
	default:
		return metric.AggAvg
	}
}

func recordSeriesInterval(s *metric.Store, start, end, now time.Time, maxPoints int) time.Duration {
	interval := recordDownsampleInterval(end.Sub(start), maxPoints)
	return s.CompatibleSeriesInterval(start, now, interval)
}

func recordDownsampleInterval(rangeDuration time.Duration, maxPoints int) time.Duration {
	if maxPoints <= 0 {
		maxPoints = 500
	}
	nanos := rangeDuration.Nanoseconds()
	if nanos <= 0 {
		return time.Second
	}
	interval := time.Duration((nanos + int64(maxPoints) - 1) / int64(maxPoints))
	if interval < time.Second {
		return time.Second
	}
	return metric.FloorStandardInterval(interval)
}

func recordPerEntityMaxPoints(maxPoints, entityCount int) int {
	if maxPoints <= 0 || entityCount <= 0 {
		return 500
	}
	quotient := maxPoints / entityCount
	if quotient >= 250 {
		return 500
	}
	points := quotient * 2
	remainder := maxPoints % entityCount
	if remainder > 0 {
		points++
		if remainder > entityCount-remainder {
			points++
		}
	}
	if points < 16 {
		return 16
	}
	if points > 500 {
		return 500
	}
	return points
}

func recordClientMaxPoints(maxPoints int) int {
	if maxPoints <= 0 || maxPoints > 500 {
		return 500
	}
	return maxPoints
}

func listRecordEntityIDs(ctx context.Context, s *metric.Store, start, end time.Time, interval time.Duration, metricNames []string) ([]string, error) {
	seen := make(map[string]struct{})
	for _, metricName := range metricNames {
		ids, err := s.EntityIDs(ctx, metric.Query{
			MetricName: metricName,
			Start:      start.Add(-interval),
			End:        end,
		})
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			seen[id] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

func applyRecordMetricValue(rec *models.Record, metricName string, value float64) {
	switch metricName {
	case MetricCPU:
		rec.Cpu = float32(value)
	case MetricGPU:
		rec.Gpu = float32(value)
	case MetricRAM:
		rec.Ram = int64(value)
	case MetricSwap:
		rec.Swap = int64(value)
	case MetricLoad:
		rec.Load = float32(value)
	case MetricDisk:
		rec.Disk = int64(value)
	case MetricNetIn:
		rec.NetIn = int64(value)
	case MetricNetOut:
		rec.NetOut = int64(value)
	case MetricNetTotalUp:
		rec.NetTotalUp = int64(value)
	case MetricNetTotalDown:
		rec.NetTotalDown = int64(value)
	case MetricTrafficUp:
		rec.TrafficUp = int64(value)
		rec.TrafficUpSet = true
	case MetricTrafficDown:
		rec.TrafficDown = int64(value)
		rec.TrafficDownSet = true
	case MetricProcess:
		rec.Process = int(value)
	case MetricConnections:
		rec.Connections = int(value)
	case MetricConnectionsUDP:
		rec.ConnectionsUdp = int(value)
	}
}

func sortRecords(records []models.Record) {
	sort.Slice(records, func(i, j int) bool {
		if records[i].Client != records[j].Client {
			return records[i].Client < records[j].Client
		}
		return records[i].Time.Before(records[j].Time)
	})
}

// GetGPURecordsByClientAndTime queries GPU records from metric store
func GetGPURecordsByClientAndTime(ctx context.Context, clientUUID string, start, end time.Time) ([]models.GPURecord, error) {
	s := GetStore()
	if s == nil {
		return nil, fmt.Errorf("metric store not enabled")
	}

	// Query GPU related metrics (use independent metric gpu.device.usage for each device utilization)
	gpuMetrics := []string{MetricGPUDeviceUsage, MetricGPUMem, MetricGPUMemTotal, MetricGPUTemp}

	// Organize data by device index and time
	type gpuKey struct {
		entityID    string
		deviceIndex int
		timestamp   int64
	}
	recordMap := make(map[gpuKey]*models.GPURecord)
	now := time.Now().UTC()
	interval := pingQueryInterval(end.Sub(start), 4000)
	interval = s.CompatibleSeriesInterval(start, now, interval)

	for _, metricName := range gpuMetrics {
		points, err := s.Series(ctx, metric.AggregateQuery{
			Query: metric.Query{
				MetricName: metricName,
				EntityID:   clientUUID,
				Start:      start,
				End:        end,
				Order:      metric.OrderAsc,
			},
			Aggregation:    metric.AggAvg,
			Interval:       interval,
			PreserveSeries: true,
		}, now)
		if err != nil {
			continue // GPU data may not exist
		}

		for _, p := range points {
			entityID := p.EntityID
			if entityID == "" {
				entityID = clientUUID
			}
			deviceIndex := 0
			deviceName := ""
			if idx, ok := p.Tags["device_index"]; ok {
				fmt.Sscanf(idx, "%d", &deviceIndex)
			}
			if name, ok := p.Tags["device_name"]; ok {
				deviceName = name
			}

			key := gpuKey{entityID: entityID, deviceIndex: deviceIndex, timestamp: p.Bucket.Unix()}
			if recordMap[key] == nil {
				recordMap[key] = &models.GPURecord{
					Client:      entityID,
					Time:        p.Bucket.UTC(),
					DeviceIndex: deviceIndex,
					DeviceName:  deviceName,
				}
			}
			rec := recordMap[key]
			if rec.DeviceName == "" && deviceName != "" {
				rec.DeviceName = deviceName
			}

			switch metricName {
			case MetricGPUDeviceUsage:
				rec.Utilization = float32(p.Value)
			case MetricGPUMem:
				rec.MemUsed = int64(p.Value)
			case MetricGPUMemTotal:
				rec.MemTotal = int64(p.Value)
			case MetricGPUTemp:
				rec.Temperature = int(p.Value)
			}
		}
	}

	// Convert to slice
	records := make([]models.GPURecord, 0, len(recordMap))
	for _, rec := range recordMap {
		records = append(records, *rec)
	}
	sort.Slice(records, func(i, j int) bool {
		if !records[i].Time.Equal(records[j].Time) {
			return records[i].Time.Before(records[j].Time)
		}
		if records[i].Client != records[j].Client {
			return records[i].Client < records[j].Client
		}
		return records[i].DeviceIndex < records[j].DeviceIndex
	})

	return records, nil
}

// GetPingRecords queries the ping records compatible with the old interface from the metric store.
//
// The old interface used to read the origin point of ping_records directly. With metric rollup enabled, older
// The raw points will be pushed into rollup and deleted, so here using Series goes with queryMetrics
// Same raw/rollup mixed read path, and preserves task_id tags.
func GetPingRecords(ctx context.Context, clientUUID string, taskID int, start, end time.Time) ([]models.PingRecord, error) {
	s := GetStore()
	if s == nil {
		return nil, fmt.Errorf("metric store not enabled")
	}

	query := metric.Query{
		MetricName: MetricPingLatency,
		Start:      start,
		End:        end,
		Order:      metric.OrderAsc,
	}

	if clientUUID != "" {
		query.EntityID = clientUUID
	}

	if taskID >= 0 {
		query.Tags = map[string]string{"task_id": fmt.Sprintf("%d", taskID)}
	}

	interval := pingQueryInterval(end.Sub(start), 4000)
	interval = s.CompatibleSeriesInterval(start, time.Now().UTC(), interval)
	points, err := s.Series(ctx, metric.AggregateQuery{
		Query:          query,
		Aggregation:    metric.AggLast,
		Interval:       interval,
		PreserveSeries: true,
	}, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	records := make([]models.PingRecord, 0, len(points))
	for _, p := range points {
		taskIDVal := uint(0)
		if tid, ok := p.Tags["task_id"]; ok {
			var t uint64
			fmt.Sscanf(tid, "%d", &t)
			taskIDVal = uint(t)
		}

		records = append(records, models.PingRecord{
			Client: p.EntityID,
			TaskId: taskIDVal,
			Time:   p.Bucket.UTC(),
			Value:  int(p.Value),
		})
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Time.After(records[j].Time)
	})

	return records, nil
}

func pingQueryInterval(rangeDuration time.Duration, maxPoints int) time.Duration {
	if maxPoints <= 0 {
		maxPoints = 4000
	}
	if rangeDuration <= 0 {
		return time.Second
	}
	interval := time.Duration((rangeDuration.Nanoseconds() + int64(maxPoints) - 1) / int64(maxPoints))
	if interval < time.Second {
		return time.Second
	}
	return metric.FloorStandardInterval(interval)
}

// farFuture returns a time in the future far enough to clear all data for an metric with DeleteBefore semantics.
func farFuture() time.Time {
	return time.Now().UTC().Add(24 * 365 * time.Hour)
}

// DeleteAllRecords Delete all load/system class records (retaining metric definitions, excluding pings).
func DeleteAllRecords(ctx context.Context) error {
	s := GetStore()
	if s == nil {
		return fmt.Errorf("metric store not enabled")
	}

	for _, metricName := range recordMetricNames {
		if _, err := s.DeleteBefore(ctx, metricName, farFuture()); err != nil {
			logger.Errorf("metricstore", "Failed to delete metric %s: %v", metricName, err)
		}
	}
	clearReportTrafficStates()

	return nil
}

// DeleteAllPingRecords Delete all ping records (leaving metric definitions).
func DeleteAllPingRecords(ctx context.Context) error {
	s := GetStore()
	if s == nil {
		return fmt.Errorf("metric store not enabled")
	}
	for _, metricName := range pingMetricNames {
		if _, err := s.DeleteBefore(ctx, metricName, farFuture()); err != nil {
			return fmt.Errorf("failed to delete ping records: %w", err)
		}
	}
	return nil
}

// PingAssignment identifies one client's history for one ping task.
type PingAssignment struct {
	Client string
	TaskID uint
}

// DeletePingRecordsByAssignments deletes only the requested client/task series.
func DeletePingRecordsByAssignments(ctx context.Context, assignments []PingAssignment) error {
	if len(assignments) == 0 {
		return nil
	}
	unique := make(map[PingAssignment]struct{}, len(assignments))
	for _, assignment := range assignments {
		if assignment.Client == "" || assignment.TaskID == 0 {
			return fmt.Errorf("ping assignment client and task ID are required")
		}
		unique[assignment] = struct{}{}
	}
	if err := storeOperations.Acquire(ctx); err != nil {
		return fmt.Errorf("wait for metric store operations before deleting ping assignments: %w", err)
	}
	defer storeOperations.Release()
	storeMu.RLock()
	defer storeMu.RUnlock()
	s := store
	if s == nil {
		return fmt.Errorf("metric store not enabled")
	}
	for assignment := range unique {
		if err := deletePingAssignmentRecords(ctx, s, assignment.Client, fmt.Sprintf("%d", assignment.TaskID)); err != nil {
			return fmt.Errorf("failed to delete ping records for client %s task %d: %w", assignment.Client, assignment.TaskID, err)
		}
	}
	return nil
}

func deletePingAssignmentRecords(ctx context.Context, s *metric.Store, client, taskTag string) error {
	for _, metricName := range pingMetricNames {
		if _, err := s.DeleteSeries(ctx, metric.Query{
			MetricName: metricName,
			EntityID:   client,
			Tags:       map[string]string{"task_id": taskTag},
		}); err != nil {
			return err
		}
	}
	return nil
}

// DeletePingRecordsByTask deletes all ping records of the specified task (task_id).
func DeletePingRecordsByTask(ctx context.Context, taskIDs []uint) error {
	if err := storeOperations.Acquire(ctx); err != nil {
		return fmt.Errorf("wait for metric store operations before deleting ping tasks: %w", err)
	}
	defer storeOperations.Release()
	storeMu.RLock()
	defer storeMu.RUnlock()
	s := store
	if s == nil {
		return fmt.Errorf("metric store not enabled")
	}
	for _, id := range taskIDs {
		for _, metricName := range pingMetricNames {
			if _, err := s.DeleteSeries(ctx, metric.Query{
				MetricName: metricName,
				Tags:       map[string]string{"task_id": fmt.Sprintf("%d", id)},
			}); err != nil {
				return fmt.Errorf("failed to delete ping records for task %d: %w", id, err)
			}
		}
	}
	return nil
}

// DeleteEntity deletes the historical data of the specified agent under all metrics.
func DeleteEntity(ctx context.Context, entityID string) error {
	if err := storeOperations.Acquire(ctx); err != nil {
		return fmt.Errorf("wait for metric store operations before deleting entity: %w", err)
	}
	defer storeOperations.Release()
	storeMu.RLock()
	defer storeMu.RUnlock()
	s := store
	if s == nil {
		return fmt.Errorf("metric store not enabled")
	}
	if _, err := s.DeleteEntity(ctx, entityID); err != nil {
		return fmt.Errorf("failed to delete metric records for entity %s: %w", entityID, err)
	}
	deleteReportTrafficState(entityID)
	return nil
}

// DeleteEntityAsync clears one agent's metric history without delaying the
// client deletion response.
func DeleteEntityAsync(entityID string) {
	go func() {
		if err := DeleteEntity(context.Background(), entityID); err != nil {
			logger.Errorf("metricstore", "Failed to delete metric records for entity %s: %v", entityID, err)
		}
	}()
}

// DeleteMetricDataAsync clears disabled metric history without delaying an
// admin retention-policy update response.
func DeleteMetricDataAsync(metricName string) {
	go func() {
		s := GetStore()
		if s == nil {
			logger.Errorf("metricstore", "Failed to delete disabled metric %s: metric store not enabled", metricName)
			return
		}
		if _, err := s.DeleteMetricDataIfDisabled(context.Background(), metricName); err != nil {
			logger.Errorf("metricstore", "Failed to delete disabled metric %s: %v", metricName, err)
		}
	}()
}
