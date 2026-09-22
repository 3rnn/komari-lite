package router

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterExposesOnlyThemeSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine)

	routes := make(map[string]bool)
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	if !routes["POST /api/admin/theme/settings"] {
		t.Fatal("theme settings route is missing")
	}
	for _, route := range []string{
		"GET /api/admin/theme/list",
		"POST /api/admin/theme/delete",
		"GET /api/admin/theme/set",
		"POST /api/admin/theme/update",
		"POST /api/admin/theme/import",
	} {
		if routes[route] {
			t.Fatalf("removed theme-management route is still registered: %s", route)
		}
	}
}
