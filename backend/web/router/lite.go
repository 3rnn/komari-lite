package router

import (
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// liteDisabledSegments lists URL path segments that the pure-monitoring build
// refuses to serve, even for administrators: remote terminal/exec, clipboard,
// Docker management, self-update, Cloudflare tunnel, the external SSO login
// flows, and the removed return-route probe. It only covers surfaces with no route any more; every entry must stay
// consistent with the registered routes (see the guard tests).
var liteDisabledSegments = map[string]bool{
	"terminal":     true,
	"xtermjs":      true,
	"exec":         true,
	"clipboard":    true,
	"docker":       true,
	"remote":       true,
	"cloudflared":  true,
	"self-update":  true,
	"selfupdate":   true,
	"return-route": true,
	"oidc":         true,
	"oauth":        true,
}

// liteDisabledPrefixes covers API namespaces whose routes were deleted outright
// (remote exec tasks, theme market). Without this, an unknown path would fall
// through to the SPA fallback and answer 200 with HTML instead of a hard 404.
var liteDisabledPrefixes = []string{
	"/api/admin/task",
	"/api/clients/task",
	"/api/admin/theme/market",
	"/api/admin/theme/list",
	"/api/admin/theme/delete",
	"/api/admin/theme/set",
	"/api/admin/theme/update",
	"/api/admin/theme/import",
}

// staticAssetExts are never blocked: a front-end chunk may legitimately be named
// after a disabled feature (e.g. chunk-terminal-<hash>.js), and 404ing it would
// break the dashboard UI.
var staticAssetExts = map[string]bool{
	".js": true, ".mjs": true, ".css": true, ".map": true, ".json": true,
	".ico": true, ".png": true, ".svg": true, ".jpg": true, ".jpeg": true,
	".webp": true, ".gif": true, ".woff": true, ".woff2": true, ".ttf": true,
	".txt": true, ".wasm": true, ".html": true,
}

// liteRoutes must be registered before any route group so disabled features can
// never be reached through an unlisted alias.
func liteRoutes(c *gin.Context) {
	p := strings.ToLower(c.Request.URL.Path)
	if strings.HasPrefix(p, "/system-assets/") || strings.HasPrefix(p, "/assets/") {
		c.Next()
		return
	}
	if staticAssetExts[strings.ToLower(path.Ext(p))] {
		c.Next()
		return
	}
	for _, prefix := range liteDisabledPrefixes {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			c.AbortWithStatus(404)
			return
		}
	}
	for _, seg := range strings.Split(strings.Trim(p, "/"), "/") {
		if liteDisabledSegments[seg] {
			c.AbortWithStatus(404)
			return
		}
	}
	c.Next()
}
