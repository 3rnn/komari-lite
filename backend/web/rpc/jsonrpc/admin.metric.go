package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/database/metricstore"
	"github.com/komari-monitor/komari/pkg/metric"
	"github.com/komari-monitor/komari/pkg/rpc"
)

// admin.metric.go
// Metrics database migration related RPC methods (admin namespace).
//
// Previously, these methods migrated the legacy records/ping tables in komari.db to the metric store.
// That one-time migration now happens automatically at startup (see metricstore.RunStartupMigration),
// without WebUI involvement. These methods instead migrate between metrics storage backends:
// they move history from default SQLite (./data/metrics.db) to administrator-configured MySQL/PostgreSQL.
//
// Typical order of use (WebUI):
//  1. admin:editSettings sets metric_db_dsn to MySQL/PostgreSQL (with a connection test
//     and hot reload; the active store switches to the initially empty remote database).
//  2. admin:startMetricMigration moves historical SQLite metrics data to the remote database.
//  3. Poll admin:getMetricMigrationStatus for progress.
//  4. Optionally call admin:cancelMetricMigration; idempotent writes allow safe retry.

func init() {
	reg("listMetricDefinitions", adminListMetricDefinitions, "List metric definitions and retention policies")
	reg("updateMetricDefinition", adminUpdateMetricDefinition, "Update a metric definition")
	reg("getMetricMigrationStatus", adminGetMetricMigrationStatus, "Get metrics store migration status (SQLite -> MySQL/PostgreSQL)")
	reg("startMetricMigration", adminStartMetricMigration, "Start migrating metrics data from source SQLite to the current MySQL/PostgreSQL target")
	reg("cancelMetricMigration", adminCancelMetricMigration, "Cancel the currently running metrics store migration")
}

type metricDefinitionResponse struct {
	Name          string            `json:"name"`
	Description   any               `json:"description,omitempty"`
	Type          string            `json:"type"`
	Unit          string            `json:"unit,omitempty"`
	RetentionDays int               `json:"retention_days"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

func metricDescriptionValue(raw string) any {
	desc := strings.TrimSpace(raw)
	if desc == "" {
		return ""
	}
	var dict map[string]string
	if err := json.Unmarshal([]byte(desc), &dict); err == nil && len(dict) > 0 {
		return dict
	}
	return raw
}

func adminListMetricDefinitions(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	store := metricstore.GetStore()
	if store == nil {
		return nil, rpc.MakeError(rpc.InternalError, "metric store not initialized", nil)
	}
	defs, err := store.ListMetrics(ctx)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to list metric definitions: "+err.Error(), nil)
	}
	out := make([]metricDefinitionResponse, 0, len(defs))
	for _, def := range defs {
		out = append(out, metricDefinitionResponse{
			Name:          def.Name,
			Description:   metricDescriptionValue(def.Description),
			Type:          string(def.Type),
			Unit:          def.Unit,
			RetentionDays: def.RetentionDays,
			Metadata:      def.Metadata,
			CreatedAt:     def.CreatedAt,
			UpdatedAt:     def.UpdatedAt,
		})
	}
	return out, nil
}

func adminUpdateMetricDefinition(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		Name          string `json:"name"`
		RetentionDays int    `json:"retention_days"`
	}
	if err := req.BindParams(&params); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid request body: "+err.Error(), nil)
	}
	params.Name = strings.TrimSpace(params.Name)
	if params.Name == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "name is required", nil)
	}
	if params.RetentionDays < 0 {
		return nil, rpc.MakeError(rpc.InvalidParams, "retention_days must be a non-negative integer", nil)
	}
	store := metricstore.GetStore()
	if store == nil {
		return nil, rpc.MakeError(rpc.InternalError, "metric store not initialized", nil)
	}
	def, err := store.UpdateMetricRetention(ctx, params.Name, params.RetentionDays)
	if errors.Is(err, metric.ErrNotFound) {
		return nil, rpc.MakeError(rpc.InvalidParams, "metric not found: "+params.Name, nil)
	}
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to update metric definition: "+err.Error(), nil)
	}
	if params.RetentionDays == 0 {
		metricstore.DeleteMetricDataAsync(params.Name)
	}

	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "update metric definition: "+params.Name, "info")

	return metricDefinitionResponse{
		Name:          def.Name,
		Description:   metricDescriptionValue(def.Description),
		Type:          string(def.Type),
		Unit:          def.Unit,
		RetentionDays: def.RetentionDays,
		Metadata:      def.Metadata,
		CreatedAt:     def.CreatedAt,
		UpdatedAt:     def.UpdatedAt,
	}, nil
}

// adminGetMetricMigrationStatus returns a snapshot of the current store-to-store migration progress.
//
// Response fields:
//   - status:          idle | running | completed | failed | canceled
//   - is_running: whether a migration is in progress
//   - source_driver: source database driver (e.g., sqlite)
//   - source_dsn: source database DSN (redacted)
//   - target_driver: target database driver (e.g., mysql/postgresql)
//   - target_dsn: target database DSN (redacted)
//   - total_metrics: total number of metric definitions
//   - metrics_done: number of metrics completed
//   - current_metric: The name of the metric currently being moved
//   - migrated_points: number of migrated sample points
//   - start_time / end_time / error
func adminGetMetricMigrationStatus(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	p := metricstore.GetStoreMigrationProgress()
	status := p.Status
	if status == "" {
		status = "idle"
	}
	return map[string]any{
		"status":          status,
		"is_running":      metricstore.IsStoreMigrationRunning(),
		"source_driver":   p.SourceDriver,
		"source_dsn":      p.SourceDSN,
		"target_driver":   p.TargetDriver,
		"target_dsn":      p.TargetDSN,
		"total_metrics":   p.TotalMetrics,
		"metrics_done":    p.MetricsDone,
		"current_metric":  p.CurrentMetric,
		"migrated_points": p.MigratedPoints,
		"start_time":      p.StartTime,
		"end_time":        p.EndTime,
		"error":           p.Error,
	}, nil
}

// adminStartMetricMigration starts a store-to-store migration.
//
// Parameters (all optional):
//   - source_driver: source database driver; inferred from source_dsn if empty.
//   - source_dsn: source database DSN; if empty, use the previous saved metrics target,
//     falling back to default SQLite (./data/metrics.db).
//
// The target is the active metric store after editSettings switches and hot-reloads it.
func adminStartMetricMigration(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		SourceDriver string `json:"source_driver"`
		SourceDSN    string `json:"source_dsn"`
	}
	req.BindParams(&params)

	if err := metricstore.StartStoreMigration(strings.TrimSpace(params.SourceDriver), strings.TrimSpace(params.SourceDSN)); err != nil {
		return nil, rpc.MakeError(rpc.InvalidRequest, err.Error(), nil)
	}

	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "start metrics store migration", "info")

	return map[string]any{
		"status":  "started",
		"message": "Metrics store migration started in background",
	}, nil
}

// adminCancelMetricMigration Cancels a running store-to-store migration.
// Since the write is idempotent upsert, it can be safely re-initiated after cancellation without duplicate data.
func adminCancelMetricMigration(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	if err := metricstore.CancelStoreMigration(); err != nil {
		return nil, rpc.MakeError(rpc.InvalidRequest, err.Error(), nil)
	}

	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "cancel metrics store migration", "warn")

	return map[string]any{
		"status":  "canceled",
		"message": "Metrics store migration canceled. It can be safely restarted.",
	}, nil
}
