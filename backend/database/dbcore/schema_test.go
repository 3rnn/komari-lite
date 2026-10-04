package dbcore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/pkg/migrations"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func isolateMainStartup(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("data", 0700); err != nil {
		t.Fatal(err)
	}
	oldDB, oldErr, oldRestore, oldFlag, oldType, oldVersion, oldExisted := instance, initErr, pendingRestore, flags.DatabaseFile, flags.DatabaseType, versionID, dbFileExistedAtStartup
	instance = nil
	initErr = nil
	pendingRestore = nil
	flags.DatabaseFile = "./data/komari.db"
	flags.DatabaseType = "sqlite"
	versionID = ""
	dbFileExistedAtStartup = false
	t.Cleanup(func() {
		if instance != nil {
			if sqlDB, err := instance.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		instance, initErr, pendingRestore, flags.DatabaseFile, flags.DatabaseType, versionID, dbFileExistedAtStartup = oldDB, oldErr, oldRestore, oldFlag, oldType, oldVersion, oldExisted
		_ = os.Chdir(cwd)
	})
	return filepath.Join(dir, "data", "komari.db")
}

func TestStartupVersionedSchemaFresh(t *testing.T) {
	isolateMainStartup(t)
	if err := doInitialize(); err != nil {
		t.Fatal(err)
	}
	version, err := migrations.SchemaVersion(instance)
	if err != nil || version != migrations.ExpectedSchemaVersion {
		t.Fatalf("version=%d err=%v", version, err)
	}
}

func TestStartupRejectsFutureSchemaBeforeCompatibilityWrites(t *testing.T) {
	path := isolateMainStartup(t)
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("PRAGMA user_version=99").Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()
	if err := doInitialize(); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("expected future version refusal, got %v", err)
	}
	db, err = gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { sqlDB, _ := db.DB(); _ = sqlDB.Close() }()
	if db.Migrator().HasTable("configs") {
		t.Fatal("compatibility migrations modified future database")
	}
}

func TestStartupSchemaFailurePropagatesAndRetrySucceeds(t *testing.T) {
	path := isolateMainStartup(t)
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE VIEW sessions AS SELECT 1 AS id").Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()
	if err := doInitialize(); err == nil {
		t.Fatal("session migration failure ignored")
	}
	db, err = gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	version, err := migrations.SchemaVersion(db)
	if err != nil || version != 0 {
		t.Fatalf("failed schema marked version=%d err=%v", version, err)
	}
	if err := db.Exec("DROP VIEW sessions").Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, _ = db.DB()
	_ = sqlDB.Close()
	if err := doInitialize(); err != nil {
		t.Fatalf("retry: %v", err)
	}
	version, err = migrations.SchemaVersion(instance)
	if err != nil || version != migrations.ExpectedSchemaVersion {
		t.Fatalf("retry version=%d err=%v", version, err)
	}
}
