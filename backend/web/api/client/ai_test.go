package client

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/aimonitor"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/web/api"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestAIReportRequiresAgentIdentityAndExplicitProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "fixture.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err = aimonitor.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.Client{}, &models.ClientDeploymentProfile{}); err != nil {
		t.Fatal(err)
	}
	now := float64(time.Now().Unix())
	r := aimonitor.Report{Version: 1, CollectorID: "550e8400-e29b-41d4-a716-446655440000", Enabled: true, Status: "no_usage", ObservedAt: now, SamplingIntervalSeconds: 10, Samples: []aimonitor.Sample{}}
	raw, _ := json.Marshal(r)
	request := func(principal *rpc.Principal, payload []byte) *httptest.ResponseRecorder {
		engine := gin.New()
		engine.Use(func(c *gin.Context) { api.SetPrincipal(c, principal) })
		engine.POST("/report", AIReportHandler(aimonitor.NewStore(db)))
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/report", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w, req)
		return w
	}
	for _, principal := range []*rpc.Principal{nil, rpc.NewAnonymousPrincipal(), rpc.NewAPIKeyPrincipal(), rpc.NewUserPrincipal("admin")} {
		if w := request(principal, raw); w.Code != http.StatusUnauthorized {
			t.Fatalf("non-Agent accepted: %d %s", w.Code, w.Body.String())
		}
	}
	principal := rpc.NewAgentPrincipal("node-a")
	if w := request(principal, raw); w.Code != http.StatusForbidden {
		t.Fatalf("silent enablement accepted: %d", w.Code)
	}
	quotaOnly := r
	quotaOnly.Enabled = false
	quotaOnly.Status = "disabled"
	quotaOnly.Quota = &aimonitor.Quota{AccountKey: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Provider: "openai", Status: "available", ObservedAt: now, Windows: []aimonitor.QuotaWindow{}}
	quotaRaw, _ := json.Marshal(quotaOnly)
	if w := request(principal, quotaRaw); w.Code != http.StatusForbidden {
		t.Fatalf("disabled profile accepted quota collection: %d %s", w.Code, w.Body.String())
	}
	db.Create(&models.ClientDeploymentProfile{Client: "node-a", Config: `{"enable_hermes_ai":true}`})
	zeroInterval := r
	zeroInterval.SamplingIntervalSeconds = 0
	zeroRaw, _ := json.Marshal(zeroInterval)
	if w := request(principal, zeroRaw); w.Code != http.StatusBadRequest {
		t.Fatalf("zero sampling interval accepted: %d %s", w.Code, w.Body.String())
	}
	if w := request(principal, raw); w.Code != 200 {
		t.Fatalf("valid report: %d %s", w.Code, w.Body.String())
	}
	if w := request(principal, raw); w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") != "2" {
		t.Fatalf("node report rate limit missing: %d %s", w.Code, w.Body.String())
	}
	if w := request(principal, bytes.Repeat([]byte(" "), aimonitor.MaxPayloadBytes+1)); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("unbounded report body accepted: %d", w.Code)
	}
	if w := request(principal, []byte(`{"version":1,"node":"forged","session_id":"private"}`)); w.Code != 400 {
		t.Fatalf("sensitive fields accepted: %d", w.Code)
	}
	var states []aimonitor.NodeState
	db.Find(&states)
	if len(states) != 1 || states[0].Node != "node-a" {
		t.Fatalf("identity not authentication-derived: %+v", states)
	}
}
