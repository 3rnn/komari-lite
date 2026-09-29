package jsonrpc

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/pkg/rpc"
)

// bridge.go
// The declarative route bridge binds gin routes directly to RPC2 methods without handwritten handlers.
// It assembles gin request parameters, calls RPC, and renders the original HTTP/JSON response contract.

// renderKind determines how successful RPC results become HTTP responses.
type renderKind int

const (
	// renderStandard: {"status": "success", "message":<msg>, "data":<result>} (api.Respond convention)
	renderStandard renderKind = iota
	// renderFlat: spread the result map into the top level and add {"status":"success"}.
	renderFlat
	// renderRaw: directly output result as body without any wrapping
	renderRaw
)

type bindConfig struct {
	render      renderKind
	successMsg  string
	pathParams  []string // Path parameter name to merge into parameter object
	queryParams []string // Query parameter name merged into parameter object
}

// BindOption configures Bind.
type BindOption func(*bindConfig)

// WithFlat spreads the result map into the top-level response (e.g., {status, uuid, token}).
func WithFlat() BindOption { return func(c *bindConfig) { c.render = renderFlat } }

// WithRaw sends the result directly without wrapping (for raw agent JSON endpoints).
func WithRaw() BindOption { return func(c *bindConfig) { c.render = renderRaw } }

// WithMessage includes a fixed message in a standard success response.
func WithMessage(msg string) BindOption {
	return func(c *bindConfig) { c.successMsg = msg }
}

// WithPath declares the path parameter (gin c.Param) to merge into the parameter object.
func WithPath(names ...string) BindOption {
	return func(c *bindConfig) { c.pathParams = append(c.pathParams, names...) }
}

// WithQuery declares the query parameters (gin c.Query) to be merged into the parameter object.
func WithQuery(names ...string) BindOption {
	return func(c *bindConfig) { c.queryParams = append(c.queryParams, names...) }
}

// Bind returns a gin.HandlerFunc that forwards the request to the specified RPC method.
func Bind(method string, opts ...BindOption) gin.HandlerFunc {
	cfg := &bindConfig{render: renderStandard}
	for _, o := range opts {
		o(cfg)
	}
	return func(c *gin.Context) {
		params, ok := assembleParams(c, cfg)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid or missing request body"})
			return
		}
		resp := CallFromGin(c, method, params)
		renderResponse(c, cfg, resp)
	}
}

// assembleParams combines body, path, and query values into RPC parameters.
// JSON objects merge with path/query values; arrays pass through unchanged when there are no path/query values.
// ok is false when a nonempty request body contains malformed JSON.
func assembleParams(c *gin.Context, cfg *bindConfig) (any, bool) {
	var bodyVal any
	if c.Request.Body != nil {
		if raw, err := io.ReadAll(c.Request.Body); err == nil && len(raw) > 0 {
			if err := json.Unmarshal(raw, &bodyVal); err != nil {
				return nil, false
			}
		}
	}

	// Pass an array body through unchanged when there are no additional parameters.
	if arr, ok := bodyVal.([]any); ok && len(cfg.pathParams) == 0 && len(cfg.queryParams) == 0 {
		return arr, true
	}

	obj := map[string]any{}
	if m, ok := bodyVal.(map[string]any); ok {
		for k, v := range m {
			obj[k] = v
		}
	}
	for _, name := range cfg.pathParams {
		if v := c.Param(name); v != "" {
			obj[name] = v
		}
	}
	for _, name := range cfg.queryParams {
		if v := c.Query(name); v != "" {
			obj[name] = v
		}
	}
	return obj, true
}

// rpcErrorHTTPStatus maps JSON-RPC error codes to HTTP status codes.
func rpcErrorHTTPStatus(code int) int {
	switch code {
	case rpc.InvalidParams, rpc.InvalidRequest, rpc.ParseError:
		return http.StatusBadRequest
	case rpc.PermissionDenied, rpc.Unauthenticated:
		return http.StatusUnauthorized
	case rpc.NotFound:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func renderResponse(c *gin.Context, cfg *bindConfig, resp *rpc.JsonRpcResponse) {
	if resp.Error != nil {
		// Unified error shape: {status: "error", message} —— same as api.RespondError.
		c.JSON(rpcErrorHTTPStatus(resp.Error.Code), gin.H{"status": "error", "message": resp.Error.Message})
		return
	}
	switch cfg.render {
	case renderRaw:
		c.JSON(http.StatusOK, resp.Result)
	case renderFlat:
		out := gin.H{"status": "success"}
		if m, ok := resp.Result.(map[string]any); ok {
			for k, v := range m {
				out[k] = v
			}
		}
		c.JSON(http.StatusOK, out)
	default: // renderStandard
		// Consistent with api.Response: this field is omitted when data is nil (omitempty semantics).
		out := gin.H{"status": "success", "message": cfg.successMsg}
		if resp.Result != nil {
			out["data"] = resp.Result
		}
		c.JSON(http.StatusOK, out)
	}
}
