package cmd

import (
	"context"
	"github.com/komari-monitor/komari/database/aimonitor"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
	"time"
)

func TestAIMaintenanceRunsOnIsolatedStore(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "ai.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err = aimonitor.Migrate(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	row := aimonitor.StoredSample{Node: "fixture", Sample: aimonitor.Sample{SampleID: "550e8400-e29b-41d4-a716-446655440001", StartedAt: float64(now.Add(-25*time.Hour).Unix() - 10), EndedAt: float64(now.Add(-25 * time.Hour).Unix()), Model: "exact", Provider: "fixture", Calls: 1}}
	if err = db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err = maintainAIStorage(context.Background(), aimonitor.NewStore(db), now); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&aimonitor.StoredSample{}).Count(&count)
	if count != 0 {
		t.Fatal("scheduled maintenance did not retire details")
	}
	db.Model(&aimonitor.Rollup{}).Count(&count)
	if count != 1 {
		t.Fatal("scheduled maintenance lost usage")
	}
}
