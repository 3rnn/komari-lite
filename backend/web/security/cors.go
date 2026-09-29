package security

import (
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/pkg/config"
)

// CorsController stores the CORS middleware's hot-reloadable state.
//
// Rather than subscribing inside a middleware closure, keep current configuration in an explicit controller:
//   - the lifecycle's reload manager updates controller state externally;
//     Update centralizes changes instead of scattering subscriptions across middleware.
//   - Middleware() reads state and checks CORS without subscribing to configuration events.
type CorsController struct {
	mu             sync.RWMutex
	enabled        bool
	allowedOrigins string
}

// NewCorsController constructs a CORS controller using the initial configuration.
func NewCorsController(enabled bool, allowedOrigins string) *CorsController {
	return &CorsController{
		enabled:        enabled,
		allowedOrigins: allowedOrigins,
	}
}

// Update applies CORS configuration changes and reports whether any field changed.
func (ctrl *CorsController) Update(event config.ConfigEvent) bool {
	ctrl.mu.Lock()
	defer ctrl.mu.Unlock()
	changed := false
	if ok, t := config.IsChangedT[bool](event, config.CorsOriginCheckEnabledKey); ok {
		ctrl.enabled = t
		changed = true
	}
	if ok, t := config.IsChangedT[string](event, config.CorsAllowedOriginsKey); ok {
		ctrl.allowedOrigins = t
		changed = true
	}
	return changed
}

func (ctrl *CorsController) snapshot() (bool, string) {
	ctrl.mu.RLock()
	defer ctrl.mu.RUnlock()
	return ctrl.enabled, ctrl.allowedOrigins
}

// Middleware returns gin middleware that reads the current controller state.
func (ctrl *CorsController) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isAPIRequestPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		corsEnabled, corsAllowedOrigins := ctrl.snapshot()

		if !corsEnabled {
			c.Next()
			return
		}

		origin := c.GetHeader("Origin")
		allowOrigin := ""
		if origin != "" && (IsAPIKeyRequest(c.Request) ||
			OriginMatchesRequest(origin, c.Request) ||
			OriginInAllowlist(origin, corsAllowedOrigins)) {
			allowOrigin = origin
		}

		authorizationPreflight := origin != "" && allowOrigin == "" && IsAuthorizationPreflight(c.Request)
		if authorizationPreflight {
			allowOrigin = origin
		}

		if allowOrigin != "" {
			c.Header("Access-Control-Allow-Origin", allowOrigin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Length, Content-Type, Authorization, Accept, X-CSRF-Token, X-Requested-With, Set-Cookie, X-2FA-Code, X-Two-Factor-Code")
			c.Header("Access-Control-Expose-Headers", "Content-Length, Authorization, Set-Cookie")
			if !authorizationPreflight {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
			c.Header("Access-Control-Max-Age", "43200") // 12 hours
		}

		if c.Request.Method == http.MethodOptions {
			if allowOrigin != "" {
				c.AbortWithStatus(http.StatusNoContent)
			} else {
				c.AbortWithStatus(http.StatusForbidden)
			}
			return
		}

		if origin != "" && allowOrigin == "" {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Next()
	}
}

func isAPIRequestPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}
