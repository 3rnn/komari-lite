package jsonrpc

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/metricstore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/database/records"
	"github.com/komari-monitor/komari/database/tasks"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/rpc"
)

// admin.misc.go
// Miscellaneous admin RPC2 methods: session management, settings, client-side sorting.

func parseUintKey(s string) (uint, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	return uint(v), err
}

func init() {
	RegisterWithGroupAndMeta("getSessions", rpc.RoleAdmin, adminGetSessions, &rpc.MethodMeta{
		Name:    "admin:getSessions",
		Summary: "List all login sessions",
		Returns: "{ current: string, data: Session[] } (session/current are non-bearer IDs)",
	})
	RegisterWithGroupAndMeta("deleteSession", rpc.RoleAdmin, adminDeleteSession, &rpc.MethodMeta{
		Name:    "admin:deleteSession",
		Summary: "Delete a session by non-bearer ID",
		Returns: "null",
	})
	RegisterWithGroupAndMeta("deleteAllSessions", rpc.RoleAdmin, adminDeleteAllSessions, &rpc.MethodMeta{
		Name:    "admin:deleteAllSessions",
		Summary: "Delete all sessions",
		Returns: "null",
	})
	RegisterWithGroupAndMeta("getSettings", rpc.RoleAdmin, adminGetSettings, &rpc.MethodMeta{
		Name:    "admin:getSettings",
		Summary: "Get all settings",
		Returns: "object",
	})
	RegisterWithGroupAndMeta("editSettings", rpc.RoleAdmin, adminEditSettings, &rpc.MethodMeta{
		Name:    "admin:editSettings",
		Summary: "Update settings (partial)",
		Returns: "null",
	})
	RegisterWithGroupAndMeta("clearAllRecords", rpc.RoleAdmin, adminClearAllRecords, &rpc.MethodMeta{
		Name:    "admin:clearAllRecords",
		Summary: "Delete all load and ping records",
		Returns: "null",
	})
	RegisterWithGroupAndMeta("orderClients", rpc.RoleAdmin, adminOrderClients, &rpc.MethodMeta{
		Name:    "admin:orderClients",
		Summary: "Reorder clients (map of uuid->weight)",
		Returns: "null",
	})
}

// sessionView explicitly allows only nonsecret fields into the browser response.
// Keep the `session` key for existing clients, but its value is no longer a token.
type sessionView struct {
	UUID            string    `json:"uuid"`
	Session         string    `json:"session"`
	UserAgent       string    `json:"user_agent"`
	IP              string    `json:"ip"`
	LoginMethod     string    `json:"login_method"`
	LatestOnline    time.Time `json:"latest_online"`
	LatestUserAgent string    `json:"latest_user_agent"`
	LatestIP        string    `json:"latest_ip"`
	Expires         time.Time `json:"expires"`
	CreatedAt       time.Time `json:"created_at"`
}

func publicSession(s models.Session) sessionView {
	return sessionView{s.UUID, accounts.SessionID(s.Session), s.UserAgent, s.Ip,
		s.LoginMethod, s.LatestOnline, s.LatestUserAgent, s.LatestIp, s.Expires, s.CreatedAt}
}

func adminGetSessions(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	ss, err := accounts.GetAllSessions()
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to retrieve sessions: "+err.Error(), nil)
	}
	current := ""
	if meta := rpc.MetaFromContext(ctx); meta != nil {
		if meta.SessionToken != "" {
			current = accounts.SessionID(meta.SessionToken)
		}
	}
	views := make([]sessionView, 0, len(ss))
	for _, s := range ss {
		views = append(views, publicSession(s))
	}
	return map[string]any{"current": current, "data": views}, nil
}

func adminDeleteSession(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		Session string `json:"session"`
	}
	req.BindParams(&params)
	if params.Session == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "session is required", nil)
	}
	if err := accounts.DeleteSessionByID(params.Session); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to delete session: "+err.Error(), nil)
	}
	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "delete session", "info")
	return nil, nil
}

func adminDeleteAllSessions(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	if err := accounts.DeleteAllSessions(); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to delete all sessions: "+err.Error(), nil)
	}
	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "delete all sessions", "warn")
	return nil, nil
}

func adminGetSettings(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	cst, err := config.GetAll()
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to get settings: "+err.Error(), nil)
	}
	delete(cst, config.CloudflareTunnelTokenKey)
	delete(cst, config.LowResourceModeKey)
	delete(cst, metricstore.MetricDownsamplingEnabledKey)
	return cst, nil
}

// metricStoreConfigKeys lists independent metrics-database settings that trigger a connection test and hot reload.
//
// Note: metric_store_enabled is deprecated (metric store is always enabled) and is no longer included in this collection.
var metricStoreConfigKeys = map[string]struct{}{
	metricstore.MetricDBDriverKey:     {},
	metricstore.MetricDBDSNKey:        {},
	metricstore.MetricTablePrefixKey:  {},
	metricstore.MetricMaxOpenConnsKey: {},
	metricstore.MetricMaxIdleConnsKey: {},
}

// metricKeysTouched reports whether a settings change affects metrics-database configuration.
func metricKeysTouched(cfg map[string]interface{}) bool {
	for key := range cfg {
		if _, ok := metricStoreConfigKeys[key]; ok {
			return true
		}
	}
	return false
}

func adminEditSettings(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	cfg := make(map[string]interface{})
	if err := req.BindParams(&cfg); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid or missing request body: "+err.Error(), nil)
	}
	if rawTime, ok := cfg[config.TrafficReportTimeKey]; ok {
		reportTime, ok := rawTime.(string)
		if !ok {
			return nil, rpc.MakeError(rpc.InvalidParams, "Traffic report time must be a string", nil)
		}
		normalized, err := config.NormalizeTrafficReportTime(reportTime)
		if err != nil {
			return nil, rpc.MakeError(rpc.InvalidParams, err.Error(), nil)
		}
		cfg[config.TrafficReportTimeKey] = normalized
	}
	if rawPageSize, ok := cfg[config.AdminDefaultPageSizeKey]; ok {
		pageSize, ok := normalizeAdminDefaultPageSize(rawPageSize)
		if !ok {
			return nil, rpc.MakeError(
				rpc.InvalidParams,
				"Admin default page size must be an integer between "+
					strconv.Itoa(config.AdminDefaultPageSizeMin)+" and "+
					strconv.Itoa(config.AdminDefaultPageSizeMax),
				nil,
			)
		}
		cfg[config.AdminDefaultPageSizeKey] = pageSize
	}
	// Ignore retired controls submitted by an older cached frontend. The
	// startup normalizer keeps their persisted compatibility values fixed.
	delete(cfg, config.LowResourceModeKey)
	delete(cfg, metricstore.MetricDownsamplingEnabledKey)

	// If metrics-database settings changed, test the merged current and proposed configuration before saving.
	// The metric store is always enabled, so test connectivity whenever a metrics setting is touched
	// to avoid persisting an invalid connection string.
	touchedMetric := metricKeysTouched(cfg)
	if touchedMetric {
		// The database type is no longer explicitly selected by the front-end, but is automatically inferred from the DSN and written back to the configuration.
		// Makes subsequent connection tests, hot reloads, and initializations all use the same driver.
		if v, ok := cfg[metricstore.MetricDBDSNKey]; ok {
			if dsn, ok := v.(string); ok {
				dsn = strings.TrimSpace(dsn)
				cfg[metricstore.MetricDBDSNKey] = dsn
				if driver, inferred := metricstore.InferDriverFromDSN(dsn); inferred {
					cfg[metricstore.MetricDBDriverKey] = string(driver)
				}
			}
		}

		merged, err := mergedMetricConfig(cfg)
		if err != nil {
			return nil, rpc.MakeError(rpc.InternalError, "Failed to resolve metric store config: "+err.Error(), nil)
		}
		testCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		if err := metricstore.TestConnection(testCtx, merged); err != nil {
			cancel()
			return nil, rpc.MakeError(rpc.InvalidParams,
				"Metrics database connection test failed: "+err.Error(), nil)
		}
		cancel()
	}

	if err := config.SetMany(cfg); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to update settings: "+err.Error(), nil)
	}
	// Settings are now saved; hot-reload the metric store without restarting. Connectivity was tested above.
	// Report any unexpected hot-reload failure to the user.
	if touchedMetric {
		reloadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		if err := metricstore.Reload(reloadCtx); err != nil {
			cancel()
			return nil, rpc.MakeError(rpc.InternalError,
				"Settings saved but metrics database hot reload failed: "+err.Error(), nil)
		}
		cancel()
	}

	message := "update settings: "
	for key := range cfg {
		message += key + ", "
	}
	if len(message) > 2 {
		message = message[:len(message)-2]
	}
	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, message, "info")
	return nil, nil
}

func normalizeAdminDefaultPageSize(raw any) (int, bool) {
	value, ok := raw.(float64)
	if !ok || math.IsNaN(value) || math.IsInf(value, 0) || math.Trunc(value) != value ||
		value < config.AdminDefaultPageSizeMin || value > config.AdminDefaultPageSizeMax {
		return 0, false
	}
	return int(value), true
}

// mergedMetricConfig combines persisted metric-store settings with the changes in this request.
// The result is the proposed configuration used for a connection test before saving.
func mergedMetricConfig(cfg map[string]interface{}) (*metricstore.MetricStoreConfig, error) {
	merged, err := config.GetManyAs[metricstore.MetricStoreConfig]()
	if err != nil {
		return nil, err
	}

	if v, ok := cfg[metricstore.MetricDBDriverKey]; ok {
		if s, ok := v.(string); ok {
			merged.Driver = s
		}
	}

	if v, ok := cfg[metricstore.MetricDBDSNKey]; ok {
		if s, ok := v.(string); ok {
			merged.DSN = s
		}
	}
	if v, ok := cfg[metricstore.MetricTablePrefixKey]; ok {
		if s, ok := v.(string); ok {
			merged.TablePrefix = s
		}
	}
	if v, ok := cfg[metricstore.MetricMaxOpenConnsKey]; ok {
		merged.MaxOpenConns = toInt(v, merged.MaxOpenConns)
	}
	if v, ok := cfg[metricstore.MetricMaxIdleConnsKey]; ok {
		merged.MaxIdleConns = toInt(v, merged.MaxIdleConns)
	}

	return merged, nil
}

func toBool(v any, fallback bool) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		if parsed, err := strconv.ParseBool(val); err == nil {
			return parsed
		}
	}
	return fallback
}

// toInt Converts any value decoded from JSON (usually float64 or string) to int, returning fallback on failure.
func toInt(v any, fallback int) int {

	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case int64:
		return int(val)
	case string:
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return fallback
}

func adminClearAllRecords(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {

	records.DeleteAll()
	tasks.DeleteAllPingRecords()
	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "clear all records", "info")
	return nil, nil
}

func adminOrderClients(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var order map[string]int
	if err := req.BindParams(&order); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid or missing request body: "+err.Error(), nil)
	}
	if err := clients.UpdateClientOrder(order); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to update client weight: "+err.Error(), nil)
	}
	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "order clients", "info")
	return nil, nil
}
