package migrations

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/komari-monitor/komari/database/models"
	appconfig "github.com/komari-monitor/komari/pkg/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func schemaDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "main.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestLegacyConfigReadErrorNeverDropsOriginalTable(t *testing.T) {
	db := schemaDB(t)
	if err := db.Exec("CREATE TABLE configs (sitename TEXT NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO configs (sitename) VALUES (?)", "preserve-me").Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateLegacyConfigToItems(db); err == nil {
		t.Fatal("expected a read error for legacy config without id")
	}
	var values []string
	if err := db.Raw("SELECT sitename FROM configs").Scan(&values).Error; err != nil || len(values) != 1 || values[0] != "preserve-me" {
		t.Fatalf("legacy config was lost: values=%v err=%v", values, err)
	}
}

func TestVersionedBaselineFreshAndRepeated(t *testing.T) {
	db := schemaDB(t)
	if err := RunVersioned(db); err != nil {
		t.Fatal(err)
	}
	version, err := SchemaVersion(db)
	if err != nil || version != ExpectedSchemaVersion {
		t.Fatalf("schema version=%d err=%v", version, err)
	}
	for _, model := range []any{&models.Client{}, &models.Session{}, &models.TrafficDailyLedger{}} {
		if !db.Migrator().HasTable(model) {
			t.Fatalf("missing table %T", model)
		}
	}
	if err := db.Create(&models.User{Username: "preserve-me"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := RunVersioned(db); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&models.User{}).Where("username = ?", "preserve-me").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("user count=%d err=%v", count, err)
	}
}

func TestVersionedBaselineLegacyPreservesDataAndConfiguration(t *testing.T) {
	db := schemaDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.Client{}, &appconfig.ConfigItem{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&appconfig.ConfigItem{Key: "custom_setting", Value: "\"keep\""}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.User{Username: "legacy"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := RunVersioned(db); err != nil {
		t.Fatal(err)
	}
	var setting appconfig.ConfigItem
	if err := db.Where("key = ?", "custom_setting").First(&setting).Error; err != nil || setting.Value != "\"keep\"" {
		t.Fatalf("setting=%+v err=%v", setting, err)
	}
	var count int64
	if err := db.Model(&models.User{}).Where("username = ?", "legacy").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("legacy count=%d err=%v", count, err)
	}
}

func TestVersionedFailureDoesNotRecordVersionAndCanRetry(t *testing.T) {
	db := schemaDB(t)
	if err := db.Exec("CREATE VIEW sessions AS SELECT 1 AS id").Error; err != nil {
		t.Fatal(err)
	}
	if err := RunVersioned(db); err == nil {
		t.Fatal("expected session table migration failure")
	}
	version, err := SchemaVersion(db)
	if err != nil || version != 0 {
		t.Fatalf("failed migration marked version=%d err=%v", version, err)
	}
	if db.Migrator().HasTable(&models.User{}) {
		t.Fatal("baseline changes survived failed transaction")
	}
	if err := db.Exec("DROP VIEW sessions").Error; err != nil {
		t.Fatal(err)
	}
	if err := RunVersioned(db); err != nil {
		t.Fatalf("retry migration: %v", err)
	}
}

func TestVersionedRejectsFutureVersionWithoutWrites(t *testing.T) {
	db := schemaDB(t)
	if err := db.Exec("PRAGMA user_version = 99").Error; err != nil {
		t.Fatal(err)
	}
	if err := RunVersioned(db); err == nil {
		t.Fatal("expected future schema refusal")
	}
	if db.Migrator().HasTable(&models.User{}) {
		t.Fatal("future schema modified")
	}
	version, err := SchemaVersion(db)
	if err != nil || version != 99 {
		t.Fatalf("version=%d err=%v", version, err)
	}
}

func TestVersionedStepRollbackAndRetry(t *testing.T) {
	db := schemaDB(t)
	failed := errors.New("injected failure")
	steps := []SchemaStep{{Version: 1, Name: "first", Apply: func(tx *gorm.DB) error { return tx.Exec("CREATE TABLE example (id INTEGER)").Error }}, {Version: 2, Name: "second", Apply: func(tx *gorm.DB) error {
		if err := tx.Exec("CREATE TABLE next (id INTEGER)").Error; err != nil {
			return err
		}
		return failed
	}}}
	if err := runSchemaSteps(db, steps); !errors.Is(err, failed) {
		t.Fatalf("error=%v", err)
	}
	version, err := SchemaVersion(db)
	if err != nil || version != 1 {
		t.Fatalf("version=%d err=%v", version, err)
	}
	if !db.Migrator().HasTable("example") || db.Migrator().HasTable("next") {
		t.Fatal("step rollback damaged committed step")
	}
	steps[1].Apply = func(tx *gorm.DB) error { return tx.Exec("CREATE TABLE next (id INTEGER)").Error }
	if err := runSchemaSteps(db, steps); err != nil {
		t.Fatal(err)
	}
	version, err = SchemaVersion(db)
	if err != nil || version != 2 {
		t.Fatalf("retry version=%d err=%v", version, err)
	}
}
