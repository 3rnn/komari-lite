package jsonrpc

import (
	"context"
	"strings"

	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/rpc"
)

// privateSiteLoginWhitelist contains methods accessible anonymously in private-site mode.
// They provide the site settings, version, and login placeholder needed to render the login page.
// Other public:* methods, such as getNodesInformation, are blocked in private-site mode.
var privateSiteLoginWhitelist = map[string]bool{
	"public:getMe":              true,
	"public:getPublicSettings":  true,
	"public:getVersion":         true,
	"public:recordVisitorEvent": true,
}

// Dispatch is the shared entry point for all transports: private-site check → authorization → method call.
// ctx carries an optional cancel/timeout; meta is the caller identity metadata (Principal is the authoritative source).
// Always return a full JsonRpcResponse with errors.
func Dispatch(ctx context.Context, meta *rpc.ContextMeta, req *rpc.JsonRpcRequest) *rpc.JsonRpcResponse {
	for _, disabled := range []string{"exec", "clipboard", "terminal", "remote", "docker", "xtermjs", "cloudflared", "selfupdate", "oidc", "oauth"} {
		if strings.Contains(strings.ToLower(req.Method), disabled) {
			return rpc.ErrorResponse(req.ID, -32601, "Unavailable in monitoring build", nil)
		}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if meta == nil {
		meta = &rpc.ContextMeta{Principal: rpc.NewAnonymousPrincipal()}
	}
	// Keep Principal and Permission synchronized for backward compatibility.
	if meta.Principal == nil {
		if meta.Permission != "" {
			meta.Principal = rpc.PrincipalFromRole(meta.Permission)
		} else {
			meta.Principal = rpc.NewAnonymousPrincipal()
		}
	}
	if meta.Permission == "" {
		meta.Permission = meta.Principal.PrimaryRole()
	}

	// In private-site mode, reject unauthenticated guests except for login-page metadata (see issue #567).
	// Also allow guests with a valid temporary share (temp_key), so shared analysis links work in private-site mode;
	// CheckPrincipal still restricts guests to public:* methods, never admin methods.
	if meta.Principal.Type == rpc.PrincipalAnonymous && !privateSiteLoginWhitelist[req.Method] && !meta.TempShareValid {
		if privateSite, _ := config.GetAs[bool](config.PrivateSiteKey); privateSite {
			return rpc.ErrorResponse(req.ID, rpc.PermissionDenied, "Private site enabled, please login first", nil)
		}
	}

	// Check namespace permissions using the Principal's capability set (set membership).
	if !rpc.CheckPrincipal(meta.Principal, req.Method) {

		return rpc.ErrorResponse(req.ID, rpc.PermissionDenied, "Permission denied", nil)
	}

	return rpc.CallWithContext(rpc.NewContextWithMeta(ctx, meta), req.ID, req.Method, req.Params)
}

// OnInternalRequest invokes an RPC method internally with a permission group.
// group: caller role (guest/client/admin); method: "namespace:method"; params: arguments.
func OnInternalRequest(ctx context.Context, group string, method string, params interface{}) *rpc.JsonRpcResponse {
	meta := &rpc.ContextMeta{Permission: group}
	req := &rpc.JsonRpcRequest{Version: rpc.RPC_VERSION, Method: method, Params: params}
	return Dispatch(ctx, meta, req)
}
