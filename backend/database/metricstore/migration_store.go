package metricstore

import (
	"context"
	"fmt"
	logger "github.com/komari-monitor/komari/utils/log"
	"strings"
	"time"

	"github.com/komari-monitor/komari/pkg/metric"
)

// storeMigrationWindow is the window size of time window query during store-to-store migration.
// The sample points are pulled in batches according to the time window to avoid reading the entire sequence into the memory at once.
const storeMigrationWindow = 6 * time.Hour

// configFromFingerprint rebuilds a MetricStoreConfig from the target fingerprint (driver|dsn),
// Used to open the "last metrics target database" in read-only mode. Table prefix, retention days, number of connections, etc.
// Keep the current configuration (these usually don't change when switching backends).
func configFromFingerprint(fingerprint string, base *MetricStoreConfig) (*MetricStoreConfig, error) {
	idx := strings.Index(fingerprint, "|")
	if idx < 0 {
		return nil, fmt.Errorf("invalid target fingerprint: %q", fingerprint)
	}
	driver := fingerprint[:idx]
	dsn := fingerprint[idx+1:]
	if strings.TrimSpace(driver) == "" {
		return nil, fmt.Errorf("empty driver in target fingerprint: %q", fingerprint)
	}
	return &MetricStoreConfig{
		Driver:       driver,
		DSN:          dsn,
		TablePrefix:  base.TablePrefix,
		MaxOpenConns: base.MaxOpenConns,
		MaxIdleConns: base.MaxIdleConns,
	}, nil
}

// openSourceStore opens an existing metrics target database as the source database for data transfer.
//
// Use autoMigrate=true: GORM's AutoMigrate only adds tables/columns/indexes and never deletes data.
// Therefore, it is harmless to the real old database idempotent; but when the source database file/table does not exist (for example, the old snapshot records completed
// (but metrics.db is missing), you can create an empty table so that subsequent ListMetrics returns an empty set instead of a report.
// "no such table", thereby identifying "no history to migrate" as a normal situation rather than an error.
func openSourceStore(ctx context.Context, cfg *MetricStoreConfig) (*metric.Store, error) {
	metricCfg, err := buildMetricConfig(cfg, true)
	if err != nil {
		return nil, err
	}
	return metric.Open(ctx, metricCfg)
}

// defaultSQLiteFingerprint Returns the target fingerprint for the default SQLite metrics database (./data/metrics.db).
// The metrics data of the old snapshot is fixed in this SQLite file, which is used to infer the previous source database when fingerprints are missing.
func defaultSQLiteFingerprint() string {
	return targetFingerprint(&MetricStoreConfig{Driver: "sqlite", DSN: "./data/metrics.db"})
}

// migrateFromPreviousStore opens the previous metrics target database specified by prevFingerprint as the source.
// Move all the metrics in it to the current target dst (for example, SQLite metrics.db → MySQL/PostgreSQL).
//
// The process is idempotent: dst is written with (metric, entity, tags, ts) upsert, and can be safely rerun after interruption.
func migrateFromPreviousStore(prevFingerprint string, cfg *MetricStoreConfig, dst *metric.Store) error {
	prevCfg, err := configFromFingerprint(prevFingerprint, cfg)
	if err != nil {
		return fmt.Errorf("parse previous metrics target %q: %w", prevFingerprint, err)
	}

	ctx := context.Background()
	src, err := openSourceStore(ctx, prevCfg)
	if err != nil {
		return fmt.Errorf("open previous metrics store (%s): %w", prevFingerprint, err)
	}
	defer src.Close()

	logger.Infof("metricstore", "Migrating metrics from previous store %s to current target...", prevFingerprint)
	total, err := MigrateBetweenStores(ctx, src, dst)
	if err != nil {
		return fmt.Errorf("store-to-store metrics migration failed: %w", err)
	}
	logger.Infof("metricstore", "Store-to-store metrics migration completed (%d points)", total)
	return nil
}

// storeMigrationObserver Receives progress callbacks during store-to-store migration.
//   - currentMetric: The name of the metric currently being moved.
//   - metricIndex: The serial number of this metric among all metrics (starting from 0), that is, the number of completed metrics.
//   - totalMetrics: Total number of metric definitions.
//   - addedPoints: The number of sample points newly written to the target database this time (used for external accumulation).
type storeMigrationObserver func(currentMetric string, metricIndex, totalMetrics int, addedPoints int64)

// MigrateBetweenStores moves all metric definitions and sample points of the src metric store to dst.
//
// For metrics backend switching (e.g. default SQLite metrics.db → MySQL/PostgreSQL):
//   - First create/update all metric definitions in dst;
//   - Then read the source database sample points in batches according to metrics and time windows and write them to dst.
//
// The sample points are written in dst with (metric_name, entity_id, tags, ts) upsert, and the window boundaries overlap.
// Idempotent, so the entire process is safe to retry. Returns the total number of sample points transported.
func MigrateBetweenStores(ctx context.Context, src, dst *metric.Store) (int64, error) {
	return migrateBetweenStores(ctx, src, dst, nil)
}

// migrateBetweenStores is an internal implementation of MigrateBetweenStores and additionally accepts an optional progress
// Observer observe (behaves exactly like the old version when nil). Start migration and remove nil, WebUI/API triggers
// The migration is passed in a callback to update the progress in real time.
func migrateBetweenStores(ctx context.Context, src, dst *metric.Store, observe storeMigrationObserver) (int64, error) {
	if src == nil || dst == nil {
		return 0, fmt.Errorf("source or destination metric store is nil")
	}

	defs, err := src.ListMetrics(ctx)
	if err != nil {
		return 0, fmt.Errorf("list source metrics: %w", err)
	}

	var total int64
	for i, def := range defs {
		if observe != nil {
			// Go to next metric: i metrics completed.
			observe(def.Name, i, len(defs), 0)
		}
		// The target database first establishes metric definitions to ensure that metrics written later exist.
		if err := dst.UpsertMetric(ctx, def); err != nil {
			return total, fmt.Errorf("upsert metric %q on target: %w", def.Name, err)
		}
		if def.RetentionDays == 0 {
			continue
		}

		earliest, latest, ok, err := metricTimeBounds(ctx, src, def.Name)
		if err != nil {
			return total, fmt.Errorf("resolve time bounds for metric %q: %w", def.Name, err)
		}
		if !ok {
			// This metric has only definition and no data, so skip it.
			continue
		}

		var migrated int64
		for windowStart := earliest; !windowStart.After(latest); windowStart = windowStart.Add(storeMigrationWindow) {
			select {
			case <-ctx.Done():
				return total, ctx.Err()
			default:
			}

			windowEnd := windowStart.Add(storeMigrationWindow)
			points, err := src.Query(ctx, metric.Query{
				MetricName: def.Name,
				Start:      windowStart,
				End:        windowEnd,
				Order:      metric.OrderAsc,
			})
			if err != nil {
				return total, fmt.Errorf("query metric %q window [%s,%s): %w", def.Name, windowStart, windowEnd, err)
			}
			if len(points) == 0 {
				continue
			}
			if err := dst.WriteBatch(ctx, points); err != nil {
				return total, fmt.Errorf("write metric %q batch to target: %w", def.Name, err)
			}
			migrated += int64(len(points))
			total += int64(len(points))
			if observe != nil {
				observe(def.Name, i, len(defs), int64(len(points)))
			}
		}
		if migrated > 0 {
			logger.Infof("metricstore", "[store-migration] metric %q: migrated %d points", def.Name, migrated)
		}
	}

	return total, nil
}

// metricTimeBounds returns the earliest/latest sampling time of a certain metric in src. ok=false means no data.
// Position the boundary by taking one sample point in ascending/descending order to avoid reading the entire sequence into memory.
func metricTimeBounds(ctx context.Context, src *metric.Store, name string) (time.Time, time.Time, bool, error) {
	wide := metric.Query{
		MetricName: name,
		Start:      time.Unix(0, 0),
		End:        time.Now().UTC().Add(24 * time.Hour),
	}

	asc := wide
	asc.Order = metric.OrderAsc
	asc.Limit = 1
	first, err := src.Query(ctx, asc)
	if err != nil {
		return time.Time{}, time.Time{}, false, err
	}
	if len(first) == 0 {
		return time.Time{}, time.Time{}, false, nil
	}

	desc := wide
	desc.Order = metric.OrderDesc
	desc.Limit = 1
	last, err := src.Query(ctx, desc)
	if err != nil {
		return time.Time{}, time.Time{}, false, err
	}
	if len(last) == 0 {
		return time.Time{}, time.Time{}, false, nil
	}

	return first[0].Timestamp, last[0].Timestamp, true, nil
}
