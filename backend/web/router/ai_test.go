package router

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestAIRoutesRegisteredAndAdminReadsNeverPublic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine)
	routes := map[string]bool{}
	for _, r := range engine.Routes() {
		routes[r.Method+" "+r.Path] = true
	}
	for _, route := range []string{"POST /api/clients/ai/report", "GET /api/admin/ai/overview", "GET /api/admin/ai/settings", "POST /api/admin/ai/settings"} {
		if !routes[route] {
			t.Fatalf("missing %s", route)
		}
	}
	for _, path := range []string{"/api/admin/ai/overview", "/api/admin/ai/settings"} {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 401 {
			t.Fatalf("public bypass %s: %d", path, w.Code)
		}
	}
	for _, route := range []string{"GET /api/admin/ai/pricing", "POST /api/admin/ai/pricing"} {
		if routes[route] {
			t.Fatalf("manual pricing route remains available: %s", route)
		}
	}
}
