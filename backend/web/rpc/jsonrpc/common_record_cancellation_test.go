package jsonrpc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/metricstore"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/rpc"
	"gorm.io/gorm"
)

func TestPingHistoryHonorsCancelledContext(t *testing.T) {
	dbcore.GetDBInstance()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ctx = rpc.NewContextWithMeta(ctx, &rpc.ContextMeta{Principal: rpc.NewAPIKeyPrincipal()})
	_, err := getRecords(ctx, rpc.NewRequest(1, "common:getRecords", map[string]any{"type": "ping"}))
	if err == nil || err.Data != context.Canceled.Error() {
		t.Fatalf("cancelled ping history ignored cancellation: %v", err)
	}
}

func TestPingHistoryTaskQueryUsesRequestContext(t *testing.T) {
	db := dbcore.GetDBInstance()
	oldDSN, _ := config.GetAs[string](metricstore.MetricDBDSNKey, "./data/metrics.db")
	if metricstore.GetStore() != nil {
		t.Fatal("unexpected existing test metric store")
	}
	if err := config.Set(metricstore.MetricDBDSNKey, filepath.Join(t.TempDir(), "metrics.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		metricstore.CloseStoreContext(context.Background())
		config.Set(metricstore.MetricDBDSNKey, oldDSN)
	})
	if err := metricstore.InitializeStore(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = rpc.NewContextWithMeta(ctx, &rpc.ContextMeta{Principal: rpc.NewAPIKeyPrincipal()})
	observed := false
	name := "test:cancel-ping-task-query"
	if err := db.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "ping_tasks" {
			observed = true
			if tx.Statement.Context != ctx {
				t.Error("ping task lookup discarded request context")
			}
			cancel()
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer db.Callback().Query().Remove(name)
	_, err := getRecords(ctx, rpc.NewRequest(1, "common:getRecords", map[string]any{"type": "ping"}))
	if !observed {
		t.Fatal("task query was not exercised")
	}
	if err == nil || err.Data != context.Canceled.Error() {
		t.Fatalf("ping history returned success after query cancellation: %v", err)
	}
}
