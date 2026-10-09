package api

// principal.go
// IdentifyPrincipal is the single entry point for identifying request principals, replacing duplicate logic in
// IdentityMiddleware / transport.detectPermissionGroup / transport.buildContextMeta
// those three locations. Priority: API key > user session > agent token > anonymous.

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/pkg/rpc"
)

// principalContextKey is the storage key for principal in gin.Context.
const principalContextKey = "principal"

// IdentifyPrincipal identifies the caller without changing state, so it can be called repeatedly.
// Identification priority is consistent with historical IdentityMiddleware: API Key > Session > Client Token > Anonymous.
func IdentifyPrincipal(c *gin.Context) *rpc.Principal {
	// 1. API Key(Authorization: Bearer <key>)
	if authorization := c.GetHeader("Authorization"); isApiKeyValid(authorization) {
		digest := sha256.Sum256([]byte(authorization))
		return rpc.NewAPIKeyPrincipalWithFingerprint(hex.EncodeToString(digest[:]))
	}

	// 2. Session (Admin User)
	if session, err := c.Cookie("session_token"); err == nil && session != "" {
		if uuid, err := accounts.GetSession(session); err == nil && uuid != "" {
			return rpc.NewUserPrincipal(uuid)
		}
	}

	// 3. Client Token (agent client)
	if token := extractClientToken(c); token != "" {
		if uuid, err := checkTokenAndGetUUID(token); err == nil && uuid != "" {
			return rpc.NewAgentPrincipal(uuid)
		}
	}

	// 4. Anonymous Visitors
	return rpc.NewAnonymousPrincipal()
}

// SetPrincipal stores an identified principal in gin.Context for downstream reuse (including RPC).
func SetPrincipal(c *gin.Context, p *rpc.Principal) {
	c.Set(principalContextKey, p)
}

// GetPrincipal returns the identified principal, or nil if identity middleware has not run.
func GetPrincipal(c *gin.Context) *rpc.Principal {
	if v, ok := c.Get(principalContextKey); ok {
		if p, ok := v.(*rpc.Principal); ok {
			return p
		}
	}
	return nil
}
