package config

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

func TestBindDbDoesNotMigrateConfigTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "main.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	BindDb(db)
	if db.Migrator().HasTable(&ConfigItem{}) {
		t.Fatal("BindDb created unversioned schema")
	}
	// Keep SetDb's standalone-test compatibility until its other callers are migrated.
	SetDb(db)
	if !db.Migrator().HasTable(&ConfigItem{}) {
		t.Fatal("SetDb no longer creates standalone config table")
	}
}
