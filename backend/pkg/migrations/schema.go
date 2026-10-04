package migrations

import (
	"fmt"

	"github.com/komari-monitor/komari/database/models"
	appconfig "github.com/komari-monitor/komari/pkg/config"
	"gorm.io/gorm"
)

// ExpectedSchemaVersion is the latest main SQLite schema understood by this binary.
// Add a new, immutable step for every subsequent schema change; never edit an
// already released step. The metrics database has an independent schema version.
const ExpectedSchemaVersion = 2

// SchemaStep applies one schema revision in a transaction with its ledger update.
type SchemaStep struct {
	Version int
	Name    string
	Apply   func(*gorm.DB) error
}

var mainSchemaSteps = []SchemaStep{
	{Version: 1, Name: "baseline", Apply: migrateMainBaseline},
	{Version: 2, Name: "client public IP addresses", Apply: migrateClientPublicIPAddresses},
}

func migrateClientPublicIPAddresses(db *gorm.DB) error {
	if db.Migrator().HasColumn(&models.Client{}, "ip_addresses") {
		return nil
	}
	return db.Migrator().AddColumn(&models.Client{}, "IPAddresses")
}

// SchemaVersion reads the main SQLite database's persistent schema ledger.
func SchemaVersion(db *gorm.DB) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("schema database is nil")
	}
	var version int
	if err := db.Raw("PRAGMA user_version").Scan(&version).Error; err != nil {
		return 0, fmt.Errorf("read main schema version: %w", err)
	}
	return version, nil
}

// CheckSchemaVersion refuses databases created by a newer binary before any
// compatibility migrations or application writes occur.
func CheckSchemaVersion(db *gorm.DB) error {
	version, err := SchemaVersion(db)
	if err != nil {
		return err
	}
	if version > ExpectedSchemaVersion {
		return fmt.Errorf("main schema version %d is newer than supported version %d", version, ExpectedSchemaVersion)
	}
	return nil
}

// RunVersioned migrates the main SQLite schema once per revision. A failed
// step rolls back both its DDL/data and the schema ledger, and is retriable.
func RunVersioned(db *gorm.DB) error {
	if err := CheckSchemaVersion(db); err != nil {
		return err
	}
	return runSchemaSteps(db, mainSchemaSteps)
}

func runSchemaSteps(db *gorm.DB, steps []SchemaStep) error {
	version, err := SchemaVersion(db)
	if err != nil {
		return err
	}
	for _, step := range steps {
		if step.Version <= version {
			continue
		}
		if step.Version != version+1 {
			return fmt.Errorf("main schema migration gap: current=%d next=%d", version, step.Version)
		}
		err := db.Transaction(func(tx *gorm.DB) error {
			// Recheck under SQLite's writer transaction so a concurrent migrator
			// cannot apply the same version twice.
			current, err := SchemaVersion(tx)
			if err != nil {
				return err
			}
			if current != version {
				return fmt.Errorf("main schema changed during migration: %d != %d", current, version)
			}
			if err := step.Apply(tx); err != nil {
				return err
			}
			return tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", step.Version)).Error
		})
		if err != nil {
			return fmt.Errorf("main schema migration %d (%s): %w", step.Version, step.Name, err)
		}
		version = step.Version
	}
	return nil
}

func migrateMainBaseline(db *gorm.DB) error {
	// Compatibility migrations still run separately before the baseline; do not
	// change their ordering or their legacy-data conversion semantics here.
	if err := db.AutoMigrate(&appconfig.ConfigItem{},
		&models.User{}, &models.Client{}, &models.ClientDeploymentProfile{},
		&models.Log{}, &models.LoadNotification{}, &models.LoadNotificationState{},
		&models.MetricCleanupJob{}, &models.OfflineNotification{},
		&models.TrafficReportNotification{}, &models.TrafficDailyLedger{},
		&models.TrafficCalibrationAdjustment{}, &models.PingTask{},
		&models.PingLossNotification{}, &models.OidcProvider{},
		&models.MessageSenderProvider{}, &models.ThemeConfiguration{},
		&models.Session{},
	); err != nil {
		return fmt.Errorf("migrate baseline tables: %w", err)
	}
	if err := MigrateTrafficResetDayFromTags(db); err != nil {
		return fmt.Errorf("migrate traffic reset days: %w", err)
	}
	return nil
}
