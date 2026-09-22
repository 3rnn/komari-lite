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
		// 已从代码中删除
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
		// 主题市场（随主题市场功能一并删除）
		"/api/admin/theme/market/sources",
		"/api/admin/theme/market/catalog",
		"/api/admin/theme/market/preview",
		"/api/admin/theme/market/install",
		// 仅运行时禁用
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

// 精简中间件只能拦截「已经删除」的路径。任何仍在注册中的路由被 404，都会让面板
// 出现「菜单在、接口全挂」的死功能（回程路由曾因被误拦而整页 404）。
// 这里只跑精简中间件本身，不执行真实 handler（它们依赖数据库与运行期状态）。
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

// litePolicyDisabledRoutes 是明确保留的例外：OAuth/OIDC 登录流程属于登录子系统
// 的一部分，精简版按策略关闭（前端入口已一并移除），而不是当作已删除功能处理。
var litePolicyDisabledRoutes = map[string]bool{
	"/api/oauth":               true,
	"/api/oauth_callback":      true,
	"/api/admin/oauth2/bind":   true,
	"/api/admin/oauth2/unbind": true,
	"/api/admin/settings/oidc": true,
}

// fillRouteParams 把 gin 的路由参数占位换成可请求的字面量。
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
