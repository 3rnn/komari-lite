package jsonrpc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/aimonitor"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/web/api"
)

// Exercise the HTTP bridge, query decoding, SQL aggregation and wire gap
// semantics, not a mocked overview or a status-code mapping in isolation.
func TestAIOverviewHTTPUnalignedFullDayAboveRawLimit(t *testing.T) {
	db := dbcore.GetDBInstance()
	node := uuid.NewString()
	end := time.Now().UTC().Truncate(time.Minute).Add(-time.Minute).Unix() + 31
	start := end - 86400
	covered := true
	rows := make([]aimonitor.StoredSample, 0, 86400)
	for i := int64(0); i < 86400; i++ {
		// Keep a real missing minute while retaining both partial edge buckets.
		if i >= 3600 && i < 3660 {
			continue
		}
		at := float64(start + i)
		rows = append(rows, aimonitor.StoredSample{Node: node, Sample: aimonitor.Sample{
			SampleID: fmt.Sprintf("%s-%d", node, i), StartedAt: at, EndedAt: at + 1,
			Model: "boundary-fixture", Provider: "fixture", BillingMode: "metered", Task: "chat",
			Calls: 1, InputTokens: 2, Coverage: &covered,
		}})
	}
	if err := db.CreateInBatches(rows, 500).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Where("node = ?", node).Delete(&aimonitor.StoredSample{}) })
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	user, err := accounts.CreateAccount("ai-boundary-"+uuid.NewString(), "test-only-password")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { accounts.DeleteAccountByUsername(user.Username) })
	token, err := accounts.CreateSession(user.UUID, 3600, "", "", "password")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { accounts.DeleteSession(token) })
	engine.Use(api.IdentityMiddleware())
	engine.GET("/api/admin/ai/overview", Bind("admin:getAIOverview", WithRaw(), WithQuery("preset", "from", "to", "timezone", "node", "model", "task")))
	// Identical frontend filter encoding with explicit stable boundaries for a
	// deterministic regression, followed by its literal default preset request.
	for _, query := range []string{
		fmt.Sprintf("from=%d&to=%d&timezone=Asia%%2FShanghai&node=%s&model=&task=", start, end, node),
		"preset=24H&timezone=Asia%2FShanghai&node=&model=boundary-fixture&task=",
		"preset=24H&timezone=Asia%2FShanghai&node=&model=&task=",
	} {
		w := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/api/admin/ai/overview?"+query, nil)
		request.AddCookie(&http.Cookie{Name: "session_token", Value: token})
		engine.ServeHTTP(w, request)
		if w.Code != 200 {
			t.Fatalf("GET overview?%s: HTTP %d %s", query, w.Code, w.Body.String())
		}
		var view struct {
			RangeUsage struct {
				CollectedCalls int64 `json:"collected_calls"`
			} `json:"range_usage"`
			Trends []struct {
				Timestamp float64 `json:"timestamp"`
				Coverage  string  `json:"coverage"`
				Calls     *int64  `json:"calls"`
			} `json:"trends"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil {
			t.Fatal(err)
		}
		if view.RangeUsage.CollectedCalls == 0 || len(view.Trends) < 1440 {
			t.Fatalf("missing overview data: %+v", view.RangeUsage)
		}
		if query[0:4] == "from" {
			if view.RangeUsage.CollectedCalls != int64(len(rows)) || len(view.Trends) != 1441 {
				t.Fatalf("lost edge data: calls=%d trends=%d", view.RangeUsage.CollectedCalls, len(view.Trends))
			}
		}
		gaps, numeric := 0, 0
		for _, point := range view.Trends {
			if point.Coverage != "covered" {
				gaps++
				if point.Calls != nil {
					t.Fatal("collection gap became numeric")
				}
			} else {
				numeric++
				if point.Calls == nil {
					t.Fatal("covered usage lost numeric calls")
				}
			}
		}
		if gaps == 0 || numeric == 0 {
			t.Fatalf("coverage semantics lost: gaps=%d numeric=%d", gaps, numeric)
		}
	}
}
