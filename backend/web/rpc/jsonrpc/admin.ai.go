package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/komari-monitor/komari/database/aimonitor"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/pkg/rpc"
	"strconv"
	"sync"
	"time"
)

func init() {
	reg("getAIOverview", adminGetAIOverview, "Get private Hermes AI monitoring overview")
	reg("getAIPricing", adminGetAIPricing, "Get append-only AI reference pricing and informational catalog")
	reg("appendAIPricing", adminAppendAIPricing, "Append an explicitly selected AI reference pricing rule")
	reg("getAISettings", adminGetAISettings, "Get Hermes AI monitoring retention and timezone")
	reg("setAISettings", adminSetAISettings, "Set bounded Hermes AI monitoring retention and timezone")
}

type aiAdminLimit struct {
	Start time.Time
	Count int
}

var aiAdminLimits = struct {
	sync.Mutex
	Rows map[string]aiAdminLimit
}{Rows: map[string]aiAdminLimit{}}

func aiAdminAllowed(key string, now time.Time) bool {
	aiAdminLimits.Lock()
	defer aiAdminLimits.Unlock()
	for k, row := range aiAdminLimits.Rows {
		if now.Sub(row.Start) >= 10*time.Second {
			delete(aiAdminLimits.Rows, k)
		}
	}
	row := aiAdminLimits.Rows[key]
	if row.Count >= 5 {
		return false
	}
	if row.Count == 0 {
		if len(aiAdminLimits.Rows) >= 256 {
			return false
		}
		row.Start = now
	}
	row.Count++
	aiAdminLimits.Rows[key] = row
	return true
}
func withAIAdmin(ctx context.Context, fn func(*aimonitor.Store) (any, error)) (any, *rpc.JsonRpcError) {
	meta := rpc.MetaFromContext(ctx)
	if meta == nil || meta.Principal == nil || !meta.Principal.HasRole(rpc.RoleAdmin) || (meta.Principal.Type != rpc.PrincipalUser && meta.Principal.Type != rpc.PrincipalAPIKey) {
		return nil, rpc.MakeError(rpc.PermissionDenied, "Administrator authentication required", nil)
	}
	key := meta.Principal.UserUUID
	if meta.Principal.Type == rpc.PrincipalAPIKey {
		key = "api-key"
	}
	if !aiAdminAllowed(key, time.Now()) {
		return nil, rpc.MakeError(rpc.InvalidParams, "AI admin request rate limit exceeded", nil)
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	value, err := fn(aimonitor.NewStore(dbcore.GetDBInstance().WithContext(bounded)))
	if err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, err.Error(), nil)
	}
	return value, nil
}
func strictAIParams(req *rpc.JsonRpcRequest, dst any) error {
	if req == nil {
		return fmt.Errorf("request params required")
	}
	raw, err := json.Marshal(req.Params)
	if err != nil {
		return err
	}
	return aimonitor.DecodeStrict(raw, dst)
}
func adminGetAIOverview(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	return withAIAdmin(ctx, func(s *aimonitor.Store) (any, error) {
		var params map[string]any
		if err := req.BindParams(&params); err != nil {
			return nil, err
		}
		if params == nil {
			params = map[string]any{}
		}
		now := time.Now().UTC()
		for _, name := range []string{"from", "to"} {
			if value, ok := params[name].(string); ok {
				parsed, err := strconv.ParseFloat(value, 64)
				if err != nil {
					return nil, fmt.Errorf("invalid %s", name)
				}
				params[name] = parsed
			}
		}
		raw, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		var query aimonitor.Query
		if err = aimonitor.DecodeStrict(raw, &query); err != nil {
			return nil, err
		}
		if _, exists := params["to"]; !exists {
			query.To = float64(now.Unix())
		}
		if _, exists := params["from"]; !exists {
			query.From = query.To - 3600
		}
		if query.Timezone == "" {
			settings, err := s.Settings()
			if err != nil {
				return nil, err
			}
			query.Timezone = settings.Timezone
		}
		return s.Overview(query, now)
	})
}
func adminGetAIPricing(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	return withAIAdmin(ctx, func(s *aimonitor.Store) (any, error) {
		rules := []aimonitor.PricingRule{}
		if err := s.DB.Order("id DESC").Limit(2049).Find(&rules).Error; err != nil {
			return nil, err
		}
		if len(rules) > 2048 {
			return nil, aimonitor.ErrQueryLimit
		}
		return map[string]any{"rules": rules, "reference_catalog": aimonitor.ReferenceCatalog()}, nil
	})
}
func adminAppendAIPricing(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	return withAIAdmin(ctx, func(s *aimonitor.Store) (any, error) {
		var rule aimonitor.PricingRule
		if err := strictAIParams(req, &rule); err != nil {
			return nil, err
		}
		// Server identifiers/timestamps are immutable, not caller-controlled updates.
		if rule.ID != 0 || !rule.CreatedAt.IsZero() {
			return nil, fmt.Errorf("pricing IDs and creation timestamps are server-managed")
		}
		return s.AppendPricing(rule, time.Now().UTC())
	})
}
func adminGetAISettings(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	return withAIAdmin(ctx, func(s *aimonitor.Store) (any, error) { return s.Settings() })
}
func adminSetAISettings(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	return withAIAdmin(ctx, func(s *aimonitor.Store) (any, error) {
		var settings aimonitor.Settings
		if err := strictAIParams(req, &settings); err != nil {
			return nil, err
		}
		if err := s.SetSettings(settings); err != nil {
			return nil, err
		}
		return s.Settings()
	})
}
