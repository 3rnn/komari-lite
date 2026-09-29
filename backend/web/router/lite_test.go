package router

import (
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// Disabled or deleted REST surfaces must answer 404 for everyone, including
// administrators. The four features removed outright (remote management, remote
// exec, remote terminal, terminal settings) now have no route at all; the rest
// are still present in the code but blocked by the monitor-build middleware.
func TestLiteDisabledRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r)
	for _, p := range []string{
		// Removed from code
		"/api/clients/terminal",
		"/api/clients/remote",
		"/api/clients/task/result",
		"/api/admin/exec",
		"/api/admin/task/all",
		"/api/admin/task/019f/result",
		"/api/admin/task/client/node-a",
		"/api/admin/settings/xtermjs",
		"/api/admin/client/node-a/terminal",
		"/api/admin/client/remote/session",
		"/api/admin/client/remote",
		"/terminal",
		"/admin/terminal",
		"/admin/exec",
		// Theme Market (removed with Theme Market feature)
		"/api/admin/theme/market/sources",
		"/api/admin/theme/market/catalog",
		"/api/admin/theme/market/preview",
		"/api/admin/theme/market/install",
		// Disabled only when running
		"/api/admin/clipboard",
		"/api/admin/cloudflared",
		"/api/admin/self-update",
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", p, nil))
		if w.Code != 404 {
			t.Errorf("%s: want 404 got %d", p, w.Code)
		}
	}
}

// Lite middleware may only block removed paths. Blocking a registered route with 404 leaves the panel
// with a visible menu but broken endpoints (return routes were previously blocked, causing full-page 404s).
// Test only the lite middleware, not handlers that depend on the database and runtime state.
func TestLiteFiltersNeverBlockRegisteredRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registered := gin.New()
	Register(registered)
	filter := gin.New()
	filter.Use(liteRoutes)
	filter.Any("/*path", func(c *gin.Context) { c.String(200, "reached") })
	for _, route := range registered.Routes() {
		requestPath := fillRouteParams(route.Path)
		if staticAssetExts[strings.ToLower(path.Ext(requestPath))] {
			continue
		}
		if litePolicyDisabledRoutes[requestPath] {
			continue
		}
		w := httptest.NewRecorder()
		filter.ServeHTTP(w, httptest.NewRequest(route.Method, requestPath, nil))
		if w.Code == 404 {
			t.Errorf("%s %s is still registered but the monitor-build filter answers 404", route.Method, route.Path)
		}
	}
}

// litePolicyDisabledRoutes is an explicit exception: OAuth/OIDC login is part of the login subsystem
// and disabled by lite policy, not treated as a removed feature (its frontend entry point is also gone).
var litePolicyDisabledRoutes = map[string]bool{
	"/api/oauth":               true,
	"/api/oauth_callback":      true,
	"/api/admin/oauth2/bind":   true,
	"/api/admin/oauth2/unbind": true,
	"/api/admin/settings/oidc": true,
}

// fillRouteParams swaps gin's routing parameter placeholder for the requestable literal.
func fillRouteParams(routePath string) string {
	segments := strings.Split(routePath, "/")
	for i, segment := range segments {
		switch {
		case strings.HasPrefix(segment, ":"):
			segments[i] = "sample"
		case strings.HasPrefix(segment, "*"):
			segments[i] = "sample"
		}
	}
	return strings.Join(segments, "/")
}

// Static front-end assets must keep working even when their file name contains a
// disabled feature keyword (e.g. chunk-terminal-<hash>.js), otherwise the
// dashboard shell would break. The middleware is exercised behind a catch-all
// route so a pass-through is observable.
func TestLiteStaticAssetsNotBlocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(liteRoutes)
	r.GET("/*asset", func(c *gin.Context) { c.String(200, "served") })
	for _, p := range []string{
		"/system-assets/chunk-terminal-vy5ANa4d.js",
		"/system-assets/chunk-theme_managed-BCc8BonO.js",
		"/system-assets/chunk-exec-deadbeef.css",
		"/assets/chunk-docker-abc123.js",
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", p, nil))
		if w.Code == 404 {
			t.Errorf("%s must not be 404 (got %d)", p, w.Code)
		}
	}
	// The same keyword in a real API path must still be blocked.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/clients/terminal", nil))
	if w.Code != 404 {
		t.Errorf("disabled API path must be 404, got %d", w.Code)
	}
}

// Monitoring paths that merely contain a keyword as a substring must stay reachable.
func TestLiteMonitoringPathsNotBlocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r)
	for _, p := range []string{"/ping"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", p, nil))
		if w.Code == 404 {
			t.Errorf("%s must not be 404 (got %d)", p, w.Code)
		}
	}
}
