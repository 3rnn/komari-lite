package trafficledger

import (
	"context"
	"errors"
	"github.com/komari-monitor/komari/database/metricstore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/config"
	"gorm.io/gorm"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCalibrationCacheCoalescesAndWaiterCanCancel(t *testing.T) {
	InvalidateCalibratedCycleCache()
	defer InvalidateCalibratedCycleCache()
	db := openLedgerTestDB(t, "cache-coalesce")
	entered := make(chan struct{}, 10)
	release := make(chan struct{})
	var calls atomic.Int32
	db.Callback().Row().Before("gorm:row").Register("block-calibration", func(tx *gorm.DB) { calls.Add(1); entered <- struct{}{}; <-release })
	done := make(chan error, 1)
	go func() { _, err := CurrentCalibratedCycleUsages(context.Background(), db, time.Now()); done <- err }()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	waiter := make(chan error, 1)
	go func() { _, err := CurrentCalibratedCycleUsages(ctx, db, time.Now()); waiter <- err }()
	cancel()
	select {
	case err := <-waiter:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("waiter = %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("canceled waiter blocked behind database loader")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("concurrent cache misses issued %d queries", calls.Load())
	}
}

func TestCalibrationCacheSharesLoadFailureAndAllowsRetry(t *testing.T) {
	InvalidateCalibratedCycleCache()
	defer InvalidateCalibratedCycleCache()
	db := openLedgerTestDB(t, "cache-failure")
	sentinel := errors.New("calibration database unavailable")
	entered := make(chan struct{}, 32)
	release := make(chan struct{})
	var calls atomic.Int32
	db.Callback().Row().Before("gorm:row").Register("fail-calibration", func(tx *gorm.DB) { calls.Add(1); entered <- struct{}{}; <-release; tx.AddError(sentinel) })
	results := make(chan error, 16)
	for i := 0; i < 16; i++ {
		go func() { _, err := CurrentCalibratedCycleUsages(context.Background(), db, time.Now()); results <- err }()
	}
	<-entered
	time.Sleep(30 * time.Millisecond)
	close(release)
	for i := 0; i < 16; i++ {
		if err := <-results; !errors.Is(err, sentinel) {
			t.Fatalf("load error = %v", err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("failed cache load was repeated %d times in one wave", calls.Load())
	}
	db.Callback().Row().Remove("fail-calibration")
	if _, err := CurrentCalibratedCycleUsages(context.Background(), db, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestEffectiveCycleUsageDoesNotReadCalibrationHistory(t *testing.T) {
	InvalidateCalibratedCycleCache()
	defer InvalidateCalibratedCycleCache()
	db := openLedgerTestDB(t, "cache-effective-only")
	config.SetDb(db)
	defer config.BindDb(nil)
	if err := config.SetMany(map[string]any{"metric_db_driver": "sqlite", "metric_db_dsn": filepath.Join(t.TempDir(), "metrics.db")}); err != nil {
		t.Fatal(err)
	}
	if err := metricstore.InitializeStore(); err != nil {
		t.Fatal(err)
	}
	defer metricstore.CloseStoreContext(context.Background())
	now := time.Now().UTC()
	resetDay := BeijingDay(now).Day()
	if err := db.Model(&models.Client{}).Where("uuid = ?", "client-a").Update("traffic_reset_day", resetDay).Error; err != nil {
		t.Fatal(err)
	}
	cycle := BeijingDay(now).Format(time.DateOnly)
	if err := db.Create(&models.TrafficCalibrationAdjustment{CalibrationID: "effective-only", Client: "client-a", Cycle: cycle, Day: cycle, UpDelta: 5, DownDelta: 7}).Error; err != nil {
		t.Fatal(err)
	}
	historyUnavailable := errors.New("history query must not run for effective usage")
	db.Callback().Row().Before("gorm:row").Register("reject-history", func(tx *gorm.DB) {
		if strings.Contains(strings.Join(tx.Statement.Selects, ","), "calibration_id") {
			tx.AddError(historyUnavailable)
		}
	})
	values, err := CurrentCalibratedCycleUsages(context.Background(), db, now)
	if err != nil {
		t.Fatal(err)
	}
	if values["client-a"] != (Usage{Up: 5, Down: 7}) {
		t.Fatalf("effective usage = %#v", values)
	}
	client := models.Client{UUID: "client-a", TrafficResetDay: &resetDay}
	if _, err := LoadCalibrationSnapshot(context.Background(), db, client, now); !errors.Is(err, historyUnavailable) {
		t.Fatalf("full snapshot must still read history: %v", err)
	}
}

func TestCalibrationInvalidationDuringLoadPreventsStalePublication(t *testing.T) {
	InvalidateCalibratedCycleCache()
	defer InvalidateCalibratedCycleCache()
	db := openLedgerTestDB(t, "cache-generation")
	var calls atomic.Int32
	db.Callback().Row().After("gorm:row").Register("invalidate-once", func(tx *gorm.DB) {
		if calls.Add(1) == 1 {
			InvalidateCalibratedCycleCache()
		}
	})
	if _, err := CurrentCalibratedCycleUsages(context.Background(), db, time.Now()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() < 2 {
		t.Fatal("invalidated in-flight result was published without reload")
	}
}
