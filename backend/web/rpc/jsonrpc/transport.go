package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/web/api"
	"github.com/komari-monitor/komari/web/connection"
)

const (
	maxRPCRequestBytes  = 2 << 20
	maxRPCResponseBytes = 4 << 20
	rpcWorkBudget       = 5 * time.Second
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
// The dispatch boundary resolves credentials afresh rather than trusting middleware.
func CallFromGin(c *gin.Context, method string, params any) *rpc.JsonRpcResponse {
	req := &rpc.JsonRpcRequest{Version: rpc.RPC_VERSION, Method: method, Params: params}
	return dispatchWithSensitive(c.Request.Context(), c, nil, req)
}

// dispatchWithSensitive applies additional 2FA checks before dispatch across every transport.
// Callers of authorized sensitive methods must satisfy sensitive-operation 2FA.
// Check the current principal and factor state; Dispatch remains the
// authoritative namespace authorization boundary.
//
// Extract a 2FA code per RPC request, first from params (2fa_code/two_factor_code/otp).
// On long-lived WebSockets, each sensitive message needs a fresh TOTP code rather than reusing an expired handshake code.
// Otherwise, fall back to the X-2FA-Code/X-Two-Factor-Code header or query parameter (REST/direct calls).
//
// Skip checks already completed by RequireSensitive2FA (sensitive_2fa_verified).
func dispatchWithSensitive(ctx context.Context, c *gin.Context, meta *rpc.ContextMeta, req *rpc.JsonRpcRequest) *rpc.JsonRpcResponse {
	if err := ctx.Err(); err != nil {
		return rpc.ErrorResponse(req.ID, rpc.InternalError, "RPC work budget exhausted", nil)
	}
	// Middleware and WebSocket handshake identities are snapshots, never proof
	// that a credential is still valid for the next privileged operation.
	if c != nil {
		meta = buildContextMeta(c)
	}
	if meta != nil && meta.Principal != nil && (c == nil || !c.GetBool("sensitive_2fa_verified") || api.IsWebSocketUpgrade(c)) &&
		rpc.IsSensitive(req.Method) && rpc.CheckPrincipal(meta.Principal, req.Method) {
		code := extractRequestTwoFACode(req)
		if code == "" && c != nil && !api.IsWebSocketUpgrade(c) {
			code = headerOrQueryTwoFACode(c)
		}
		if err := api.VerifySensitive2FACore(meta.Principal.UserUUID, code, meta.Principal.IsAPIKey); err != nil {
			return rpc.ErrorResponse(req.ID, rpc.PermissionDenied, err.Error(), nil)
		}
	}
	response := Dispatch(ctx, meta, req)
	if ctx.Err() != nil {
		return rpc.ErrorResponse(req.ID, rpc.InternalError, "RPC work budget exhausted", nil)
	}
	return response
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

// budgetErrorJSON retains correlation when the encoded ID fits the available
// wire budget. IDs can expand under JSON escaping, so even errors need a cap.
func budgetErrorJSON(id any, message string, limit int) []byte {
	encoded, err := json.Marshal(rpc.ErrorResponse(id, rpc.InternalError, message, nil))
	if err != nil || len(encoded) > limit {
		encoded, _ = json.Marshal(rpc.ErrorResponse(nil, rpc.InternalError, message, nil))
	}
	return encoded
}

func writeRPCWebSocketResponse(conn *connection.SafeConn, response *rpc.JsonRpcResponse) error {
	encoded, err := json.Marshal(response)
	if err != nil || len(encoded) > maxRPCResponseBytes {
		encoded = budgetErrorJSON(response.ID, "RPC response budget exhausted", maxRPCResponseBytes)
	}
	return conn.WriteMessage(websocket.TextMessage, encoded)
}

func serveWebSocket(c *gin.Context) {
	_conn, err := api.UpgradeWebSocket(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Failed to upgrade to WebSocket." + err.Error()})
		return
	}
	_conn.SetReadLimit(maxRPCRequestBytes)
	conn := connection.NewSafeConn(_conn)
	defer conn.Close()

	for {
		var req rpc.JsonRpcRequest
		if err := conn.ReadJSON(&req); err != nil {
			var se *json.SyntaxError
			var ute *json.UnmarshalTypeError
			if errors.As(err, &se) || errors.As(err, &ute) {
				if err := writeRPCWebSocketResponse(conn, rpc.ErrorResponse(nil, rpc.InvalidRequest, "bad request: "+err.Error(), nil)); err != nil {
					return
				}
				continue
			}
			// Others treated as connection/IO errors, ending the loop
			break
		}
		if jerr := req.Validate(); jerr != nil {
			if err := writeRPCWebSocketResponse(conn, jerr.ResponseWithID(req.ID)); err != nil {
				return
			}
			continue
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), rpcWorkBudget)
		response := dispatchWithSensitive(ctx, c, nil, &req)
		cancel()
		if err := writeRPCWebSocketResponse(conn, response); err != nil {
			return
		}
	}
}

func servePost(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRPCRequestBytes)
	ctx, cancel := context.WithTimeout(c.Request.Context(), rpcWorkBudget)
	defer cancel()
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
	// Reserve a bounded, correlated error for every parsed call before spending
	// the response budget on results. Exhaustion stops work, not error replies.
	responses := make([]json.RawMessage, len(requests))
	errorLimit := (maxRPCResponseBytes - 2 - len(requests)) / len(requests)
	totalBytes := 2 + len(requests) // Array delimiters and commas (conservative).
	for i, request := range requests {
		responses[i] = budgetErrorJSON(request.ID, "RPC response budget exhausted", errorLimit)
		totalBytes += len(responses[i])
	}
	for i, rreq := range requests {
		if ctx.Err() != nil {
			for j := i; j < len(requests); j++ {
				responses[j] = budgetErrorJSON(requests[j].ID, "RPC work budget exhausted", errorLimit)
			}
			break
		}
		response := dispatchWithSensitive(ctx, c, nil, rreq)
		encoded, err := json.Marshal(response)
		if err != nil || totalBytes-len(responses[i])+len(encoded) > maxRPCResponseBytes {
			break
		}
		totalBytes += len(encoded) - len(responses[i])
		responses[i] = encoded
	}
	// Keep the legacy one-response shape for compatibility.
	if len(responses) == 1 && len(requests) == 1 {
		c.Data(http.StatusOK, "application/json; charset=utf-8", responses[0])
	} else {
		c.JSON(http.StatusOK, responses)
	}
}

// buildContextMeta builds fresh authentication metadata from the original
// credentials, including principal, legacy role, account and temporary share.
func buildContextMeta(c *gin.Context) *rpc.ContextMeta {
	// Always resolve the original credentials again, including current API-key
	// configuration and session expiry/revocation; ignore middleware snapshots.
	p := api.IdentifyPrincipal(c)
	if p.Type == rpc.PrincipalUser {
		if _, err := accounts.GetUserByUUID(p.UserUUID); err != nil {
			p = rpc.NewAnonymousPrincipal()
		}
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
