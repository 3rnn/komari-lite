package metricstore

import (
	"fmt"
	logger "github.com/komari-monitor/komari/utils/log"
	"strings"

	"github.com/komari-monitor/komari/pkg/config"
)

// migration.go
//
// Metrics data migration during startup phase. This only handles the data transfer when switching the metrics storage backend.
// (e.g. default SQLite ./data/metrics.db switches to MySQL/PostgreSQL). oldkomari.db
// Monitoring table import is a one-time migration, see pkg/migrations.RunMetricStoreMigrations.

// targetFingerprint returns the fingerprint of the current metrics target database (driver + normalized DSN),
// Used to determine whether the metrics storage backend has changed (such as switching from SQLite to MySQL/PostgreSQL).
func targetFingerprint(cfg *MetricStoreConfig) string {
	driver := ResolveDriverFromConfig(cfg.Driver, cfg.DSN)
	dsn := strings.TrimSpace(cfg.DSN)
	return fmt.Sprintf("%s|%s", driver, dsn)
}

// RunStartupMigration checks whether the metrics storage backend has changed when the service starts.
// If there is a change, all data in the previous metrics target database will be moved to the current target.
//
// Determination based on MigrationTargetKey (recording "the complete metrics target fingerprint where the data currently resides"):
//   - saved == current: The data is already in the current target and does not need to be processed.
//   - saved == "": No historical target record. The metrics data of old snapshots are fixed in the default SQLite
//     (./data/metrics.db); if the current target is not the default SQLite (for example, it has been switched to MySQL/PostgreSQL),
//     Move historical data that may exist in the default SQLite, otherwise only register fingerprints.
//   - saved != current: The backend has been switched, and the data of the previous target (saved) is moved to the current target.
//
// The transfer is written with upsert, which is idempotent. It can be safely restarted after interruption. Any failure returns an error,
// The caller should let startup fail and print an explicit error.
func RunStartupMigration() error {
	s := GetStore()
	if s == nil {
		return fmt.Errorf("metric store not initialized")
	}

	cfg, err := config.GetManyAs[MetricStoreConfig]()
	if err != nil {
		return fmt.Errorf("failed to load metric store config: %w", err)
	}

	current := targetFingerprint(cfg)
	saved, _ := config.GetAs[string](MigrationTargetKey, "")

	// The data is completely at the current destination: no moving required.
	if saved == current {
		return nil
	}

	// Previous goal: Give priority to using saved fingerprints; when there is no record, use the default SQLite inference based on the old snapshot.
	prev := saved
	if prev == "" {
		prev = defaultSQLiteFingerprint()
	}

	if prev != current {
		if err := migrateFromPreviousStore(prev, cfg, s); err != nil {
			return err
		}
	}

	if err := config.Set(MigrationTargetKey, current); err != nil {
		logger.Errorf("metricstore", "Failed to persist migration target fingerprint: %v", err)
	}
	return nil
}
