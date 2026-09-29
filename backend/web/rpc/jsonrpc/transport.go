package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/web/api"
	"github.com/komari-monitor/komari/web/connection"
)

// OnRpcRequest handles /api/rpc2: GET upgrades to WebSocket; POST accepts single or batched JSON-RPC requests.
func OnRpcRequest(c *gin.Context) {
	// GET -> WebSocket
	if c.Request.Method == http.MethodGet {
		serveWebSocket(c)
		return
	}

	if c.Request.Method != http.MethodPost {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
		return
	}
	servePost(c)
}

// CallFromGin lets legacy gin handlers and the route bridge invoke RPC methods.
// Reuse the principal identified by IdentityMiddleware, or fall back to IdentifyPrincipal.
func CallFromGin(c *gin.Context, method string, params any) *rpc.JsonRpcResponse {
	meta := buildContextMeta(c)
	req := &rpc.JsonRpcRequest{Version: rpc.RPC_VERSION, Method: method, Params: params}
	return dispatchWithSensitive(c.Request.Context(), c, meta, req)
}

// dispatchWithSensitive applies additional 2FA checks before dispatch across every transport.
// Callers of authorized sensitive methods must satisfy sensitive-operation 2FA.
// Check the Principal (API keys and accounts without 2FA bypass this check); Dispatch remains the authoritative authorization boundary.
//
// Extract a 2FA code per RPC request, first from params (2fa_code/two_factor_code/otp).
// On long-lived WebSockets, each sensitive message needs a fresh TOTP code rather than reusing an expired handshake code.
// Otherwise, fall back to the X-2FA-Code/X-Two-Factor-Code header or query parameter (REST/direct calls).
//
// Skip checks already completed by RequireSensitive2FA (sensitive_2fa_verified).
func dispatchWithSensitive(ctx context.Context, c *gin.Context, meta *rpc.ContextMeta, req *rpc.JsonRpcRequest) *rpc.JsonRpcResponse {
	if meta != nil && meta.Principal != nil && (c == nil || !c.GetBool("sensitive_2fa_verified")) &&
		rpc.IsSensitive(req.Method) && rpc.CheckPrincipal(meta.Principal, req.Method) {
		code := extractRequestTwoFACode(req)
		if code == "" && c != nil {
			code = headerOrQueryTwoFACode(c)
		}
		if err := api.VerifySensitive2FACore(meta.Principal.UserUUID, code, meta.Principal.IsAPIKey); err != nil {
			return rpc.ErrorResponse(req.ID, rpc.PermissionDenied, err.Error(), nil)
		}
	}
	return Dispatch(ctx, meta, req)
}

// extractRequestTwoFACode reads the 2FA code from a single RPC request's named parameters.
// Only object (map) params are supported; check 2fa_code, two_factor_code, then otp.
func extractRequestTwoFACode(req *rpc.JsonRpcRequest) string {
	for _, key := range []string{"2fa_code", "two_factor_code", "otp"} {
		if v, ok := rpc.GetParamAs[string](req, key); ok && v != "" {
			return v
		}
	}
	return ""
}

// headerOrQueryTwoFACode falls back to headers or query parameters without consuming the body.
func headerOrQueryTwoFACode(c *gin.Context) string {
	if code := c.GetHeader("X-2FA-Code"); code != "" {
		return code
	}
	if code := c.GetHeader("X-Two-Factor-Code"); code != "" {
		return code
	}
	for _, key := range []string{"2fa_code", "two_factor_code", "otp"} {
		if code := c.Query(key); code != "" {
			return code
		}
	}
	return ""
}

func serveWebSocket(c *gin.Context) {
	_conn, err := api.UpgradeWebSocket(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Failed to upgrade to WebSocket." + err.Error()})
		return
	}
	conn := connection.NewSafeConn(_conn)
	defer conn.Close()

	meta := buildContextMeta(c)
	for {
		var req rpc.JsonRpcRequest
		if err := conn.ReadJSON(&req); err != nil {
			var se *json.SyntaxError
			var ute *json.UnmarshalTypeError
			if errors.As(err, &se) || errors.As(err, &ute) {
				conn.WriteJSON(rpc.ErrorResponse(nil, rpc.InvalidRequest, "bad request: "+err.Error(), nil))
				continue
			}
			// Others treated as connection/IO errors, ending the loop
			break
		}
		if jerr := req.Validate(); jerr != nil {
			conn.WriteJSON(jerr.ResponseWithID(req.ID))
			continue
		}
		// SafeConn serializes writes under a lock, preventing reordered responses and races.
		conn.WriteJSON(dispatchWithSensitive(context.Background(), c, meta, &req))
	}
}

func servePost(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, rpc.ErrorResponse(nil, rpc.ParseError, "read body error", err.Error()))
		return
	}
	requests, jerr := rpc.ParseRequests(body)
	if jerr != nil {
		c.JSON(http.StatusBadRequest, jerr.Response())
		return
	}
	meta := buildContextMeta(c)

	responses := make([]*rpc.JsonRpcResponse, 0, len(requests))
	for _, rreq := range requests {
		responses = append(responses, dispatchWithSensitive(c.Request.Context(), c, meta, rreq))
	}
	// Single direct object, batch array (compliant with JSON-RPC 2.0).
	if len(responses) == 1 {
		c.JSON(http.StatusOK, responses[0])
	} else {
		c.JSON(http.StatusOK, responses)
	}
}

// buildContextMeta builds *rpc.ContextMeta from gin.Context.
// Reuse the principal from IdentityMiddleware (api.GetPrincipal); otherwise call
// api.IdentifyPrincipal. Populate Principal, legacy Permission, User, UUIDs, token, and related fields.
func buildContextMeta(c *gin.Context) *rpc.ContextMeta {
	// Prefer the middleware's principal; fall back to identification for direct /api/rpc2 requests.
	p := api.GetPrincipal(c)
	if p == nil {
		p = api.IdentifyPrincipal(c)
	}

	meta := &rpc.ContextMeta{
		Principal:  p,
		Permission: p.PrimaryRole(), // Compatible with existing handlers and Dispatch
		RemoteIP:   c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
	}

	// Populate fields according to principal type.
	switch p.Type {
	case rpc.PrincipalUser:
		meta.UserUUID = p.UserUUID
		if session, err := c.Cookie("session_token"); err == nil && session != "" {
			meta.SessionToken = session
			if user, err := accounts.GetUserBySession(session); err == nil {
				meta.User = &user
			}
		}
	case rpc.PrincipalAgent:
		meta.ClientUUID = p.ClientUUID
		// Try to extract the client token (for scenarios where some handlers require the original token).
		// Prefer ?Authorization=<token>, then the Bearer header.
		if token := c.Query("Authorization"); token != "" {
			meta.ClientToken = token
		} else if auth := c.GetHeader("Authorization"); auth != "" && len(auth) > len("Bearer ") {
			meta.ClientToken = auth[len("Bearer "):]
		}
	}

	// Temporary share access.
	meta.TempShareValid = hasTempShareAccess(c)
	return meta
}

// hasTempShareAccess checks whether temp_key grants temporary share access.
func hasTempShareAccess(c *gin.Context) bool {
	tempKey, err := c.Cookie("temp_key")
	if err != nil || tempKey == "" {
		return false
	}
	expireAt, err := config.GetAs[int64]("tempory_share_token_expire_at", 0)
	if err != nil {
		return false
	}
	allowKey, err := config.GetAs[string]("tempory_share_token", "")
	if err != nil || allowKey == "" || tempKey != allowKey {
		return false
	}
	return expireAt >= time.Now().Unix()
}
