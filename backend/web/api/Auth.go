package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/rpc"
	"gorm.io/gorm"
)

const (
	RoleAdmin  = "admin"
	RoleClient = "client"
	RoleGuest  = "guest"
)

// IdentityMiddleware identifies callers at the outermost layer of the route stack.
// It stores each requester's identity (Admin/Client/Guest) in the context.
// Identification delegates to IdentifyPrincipal and retains legacy c.Set keys (role/uuid/
// api_key/session/client_uuid) for existing handlers and middleware.
func IdentityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		p := IdentifyPrincipal(c)
		SetPrincipal(c, p)

		// Write compatible fields.
		c.Set("role", p.PrimaryRole())
		switch p.Type {
		case rpc.PrincipalAPIKey:
			// Preserve the raw API key and fixed placeholder UUID for legacy callers.
			apiKey := c.GetHeader("Authorization")
			c.Set("api_key", apiKey[len("Bearer "):])
			c.Set("uuid", "00000000-0000-0000-0000-000000000000")
		case rpc.PrincipalUser:
			if session, err := c.Cookie("session_token"); err == nil && session != "" {
				c.Set("session", session)
				accounts.UpdateLatest(session, c.Request.UserAgent(), c.ClientIP())
			}
			c.Set("uuid", p.UserUUID)
		case rpc.PrincipalAgent:
			c.Set("client_uuid", p.ClientUUID)
		}

		c.Next()
	}
}

// RequireRole only permits callers with one of the specified roles.
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		current := GetRole(c)
		for _, role := range allowedRoles {
			if current == role {
				c.Next()
				return
			}
		}
		RespondError(c, http.StatusUnauthorized, "Unauthorized.")
		c.Abort()
	}
}

// GetRole returns the current requester's role.
func GetRole(c *gin.Context) string {
	role, exists := c.Get("role")
	if !exists {
		return RoleGuest
	}
	if s, ok := role.(string); ok {
		return s
	}
	return RoleGuest
}

// --- Private Site Access Control ---

var publicPaths = []string{
	"/ping",
	"/api/public",
	"/api/login",
	"/api/me",
	"/api/oauth",
	"/api/oauth_callback",
	"/api/version",
	"/api/recent",
	"/api/admin",    // Handled by RequireRole
	"/api/clients/", // Handled by RequireRole
}

// PrivateSiteMiddleware restricts guest access in private-site mode.
// It uses the role set by IdentityMiddleware to block unauthenticated visitors.
func PrivateSiteMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Allow authenticated users.
		if GetRole(c) != RoleGuest {
			c.Next()
			return
		}

		path := c.Request.URL.Path

		// Allow public paths.
		for _, p := range publicPaths {
			if strings.HasPrefix(path, p) {
				c.Next()
				return
			}
		}

		// Allow non-API paths (including static assets).
		if !strings.HasPrefix(path, "/api") {
			c.Next()
			return
		}

		// Allow requests when private-site mode is off.
		privateSite, err := config.GetAs[bool](config.PrivateSiteKey, false)
		if err != nil {
			RespondError(c, http.StatusInternalServerError, "Failed to get configuration.")
			c.Abort()
			return
		}
		if !privateSite {
			c.Next()
			return
		}

		// Allow temporary access grants.
		if hasTempAccess(c) {
			c.Next()
			return
		}

		RespondError(c, http.StatusUnauthorized, "Private site is enabled, please login first.")
		c.Abort()
	}
}

func hasTempAccess(c *gin.Context) bool {
	tempKey, err := c.Cookie("temp_key")
	if err != nil {
		return false
	}
	expireAt, err := config.GetAs[int64]("tempory_share_token_expire_at", 0)
	if err != nil {
		return false
	}
	allowKey, err := config.GetAs[string]("tempory_share_token", "")
	if err != nil {
		return false
	}
	if allowKey == "" || tempKey != allowKey {
		return false
	}
	return expireAt >= time.Now().Unix()
}

func extractClientToken(c *gin.Context) string {
	token := c.Query("token")
	if token != "" {
		return token
	}
	authorization := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(authorization, "Bearer ") && !isApiKeyValid(authorization) {
		if token := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer ")); token != "" {
			return token
		}
	}
	// RPC2 agents supply their client token in ?Authorization=<token>.
	if token := c.Query("Authorization"); token != "" {
		return token
	}

	if c.Request.Method != http.MethodGet {
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			return ""
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		var bodyMap map[string]interface{}
		if len(bodyBytes) > 0 {
			if err := json.Unmarshal(bodyBytes, &bodyMap); err == nil {
				if tokenVal, exists := bodyMap["token"]; exists {
					if str, ok := tokenVal.(string); ok && str != "" {
						return str
					}
				}
			}
		}
	}

	return ""
}

func checkTokenAndGetUUID(token string) (string, error) {
	uuid, err := clients.GetClientUUIDByToken(token)

	if err == sql.ErrNoRows {
		return "", nil
	}
	if err == gorm.ErrRecordNotFound {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return uuid, nil
}

func isApiKeyValid(apiKey string) bool {
	apiKeyConfig, err := config.GetAs[string](config.ApiKeyKey, "")
	if err != nil {
		return false
	}

	if apiKeyConfig == "" || len(apiKeyConfig) < 12 {
		return false
	}
	return apiKey == "Bearer "+apiKeyConfig
}
