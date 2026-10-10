package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/komari-monitor/komari/database/aimonitor"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/rpc"
	"testing"
	"time"
)

func TestAIAdminVerticalSliceSettingsPricingAndOverview(t *testing.T) {
	db := dbcore.GetDBInstance()
	store := aimonitor.NewStore(db)
	previous, err := store.Settings()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.SetSettings(previous) })
	ctx := rpc.NewContextWithMeta(context.Background(), &rpc.ContextMeta{Principal: rpc.NewUserPrincipal(uuid.NewString())})
	value, rpcErr := adminGetAISettings(ctx, rpc.NewRequest(1, "", nil))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if value.(aimonitor.Settings).RetentionDays < 1 {
		t.Fatal("settings response invalid")
	}
	_, rpcErr = adminSetAISettings(ctx, rpc.NewRequest(1, "", map[string]any{"timezone": "UTC", "retention_days": 7, "detailed_retention_hours": 24}))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	priceCtx := rpc.NewContextWithMeta(context.Background(), &rpc.ContextMeta{Principal: rpc.NewUserPrincipal(uuid.NewString())})
	_, rpcErr = adminAppendAIPricing(priceCtx, rpc.NewRequest(1, "", map[string]any{"provider": "fixture-admin-ai", "model": "exact", "billing_mode": "api", "input_per_million": 1, "output_per_million": 2, "source": "operator override", "version": "v1", "processing_mode": "explicit", "effective_at": time.Now().UTC().Format(time.RFC3339)}))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	t.Cleanup(func() { db.Where("provider = ?", "fixture-admin-ai").Delete(&aimonitor.PricingRule{}) })
	value, rpcErr = adminGetAIPricing(priceCtx, rpc.NewRequest(1, "", nil))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	raw, _ := json.Marshal(value)
	var pricing map[string]json.RawMessage
	json.Unmarshal(raw, &pricing)
	if pricing["reference_catalog"] == nil || pricing["rules"] == nil {
		t.Fatal("pricing wire shape incomplete")
	}
	node := uuid.NewString()
	db.Create(&models.Client{UUID: node, Name: "AI fixture", Token: uuid.NewString()})
	db.Create(&models.ClientDeploymentProfile{Client: node, Config: `{"enable_hermes_ai":true}`, DeliveryStatus: "saved"})
	t.Cleanup(func() {
		db.Where("client = ?", node).Delete(&models.ClientDeploymentProfile{})
		db.Where("uuid = ?", node).Delete(&models.Client{})
	})
	value, rpcErr = adminGetAIOverview(ctx, rpc.NewRequest(1, "", map[string]any{"node": node, "timezone": "UTC", "from": fmtTime(time.Now().Add(-time.Hour)), "to": fmtTime(time.Now())}))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	view := value.(aimonitor.Overview)
	if len(view.Nodes) != 1 || view.Nodes[0].Status != "unavailable" || view.Nodes[0].DeliveryStatus != "saved" {
		t.Fatalf("legacy Agent falsely collecting: %+v", view.Nodes)
	}
	_, rpcErr = adminGetAIOverview(ctx, rpc.NewRequest(1, "", map[string]any{"from": float64(time.Now().Add(-8 * 24 * time.Hour).Unix()), "to": float64(time.Now().Unix()), "timezone": "UTC"}))
	if rpcErr == nil {
		t.Fatal("query ignored configured retention")
	}
}
func fmtTime(t time.Time) string { return fmt.Sprintf("%d", t.Unix()) }
func TestAIAdminHandlersRejectDirectAndRoleOnlyBypass(t *testing.T) {
	handlers := []rpc.Handler{adminGetAIOverview, adminGetAIPricing, adminAppendAIPricing, adminGetAISettings, adminSetAISettings}
	for _, handler := range handlers {
		for _, meta := range []*rpc.ContextMeta{nil, {Permission: rpc.RoleAdmin}, {Principal: rpc.NewAnonymousPrincipal(), TempShareValid: true}, {Principal: rpc.NewAgentPrincipal("node")}} {
			ctx := rpc.NewContextWithMeta(context.Background(), meta)
			_, err := handler(ctx, rpc.NewRequest(1, "test", map[string]any{}))
			if err == nil || err.Code != rpc.PermissionDenied {
				t.Fatalf("AI handler lacks independent admin gate: %+v", err)
			}
		}
	}
}

func TestAIAdminRateLimitIsPerResource(t *testing.T) {
	aiAdminLimits.Lock()
	aiAdminLimits.Rows = map[string]aiAdminLimit{}
	aiAdminLimits.Unlock()
	now := time.Now()
	for i := 0; i < 5; i++ {
		if !aiAdminAllowed("admin", "overview", now) {
			t.Fatal("unexpected overview limit")
		}
	}
	if aiAdminAllowed("admin", "overview", now) {
		t.Fatal("overview limit not enforced")
	}
	if !aiAdminAllowed("admin", "settings", now) || !aiAdminAllowed("admin", "pricing", now) {
		t.Fatal("unrelated resource was limited")
	}
	if !aiAdminAllowed("api-key:two", "overview", now) {
		t.Fatal("separate API-key fingerprint was limited")
	}
	if !aiAdminAllowed("admin", "overview", now.Add(10*time.Second)) {
		t.Fatal("window did not expire")
	}
}
func TestAIQueryBudgetIsNotReportedAsRetryableRateLimit(t *testing.T) {
	if got := rpcErrorHTTPStatus(rpc.ResourceExhausted); got != 422 {
		t.Fatalf("resource exhausted status = %d, want 422", got)
	}
	if got := rpcErrorHTTPStatus(rpc.RateLimited); got != 429 {
		t.Fatalf("rate limited status = %d, want 429", got)
	}
}
