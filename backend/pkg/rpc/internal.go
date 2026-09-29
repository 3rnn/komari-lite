package rpc

// internal.go
// Defines and registers internal methods that retain the prefix "rpc." and their metadata.

import (
	"context"
	"sort"
	"strings"
)

// registerInternal writes directly to the reserved prefix method.
// The call site is assumed to be under control (init phase), and duplicates are ignored to prevent panic.
func registerInternal(method string, h Handler) {
	if !strings.HasPrefix(method, "rpc.") {
		method = "rpc." + method
	}
	muHandlers.Lock()
	if _, exists := handlers[method]; !exists {
		handlers[method] = h
	}
	muHandlers.Unlock()
}

// listMethods returns the method list; rpc.* is eliminated when includeInternal=false
func listMethods(includeInternal bool) []string {
	all := ListMethods()
	out := make([]string, 0, len(all))
	for _, m := range all {
		if !includeInternal && strings.HasPrefix(m, "rpc.") {
			continue
		}
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

func init() {
	// rpc.methods -> list method names
	registerInternal("rpc.methods", func(ctx context.Context, req *JsonRpcRequest) (any, *JsonRpcError) {
		var params struct {
			ShowInternal bool `json:"internal"`
		}
		req.BindParams(&params)
		return listMethods(params.ShowInternal), nil
	})
	// rpc.version -> protocol version
	registerInternal("rpc.version", func(ctx context.Context, req *JsonRpcRequest) (any, *JsonRpcError) {
		return RPC_VERSION, nil
	})
	// rpc.ping -> health check
	registerInternal("rpc.ping", func(ctx context.Context, req *JsonRpcRequest) (any, *JsonRpcError) {
		return "pong", nil
	})
	// rpc.help -> method metadata or overview
	registerInternal("rpc.help", func(ctx context.Context, req *JsonRpcRequest) (any, *JsonRpcError) {
		var params struct {
			Method string `json:"method"`
		}
		req.BindParams(&params)
		if params.Method != "" {
			meta := getMetaUnsafe(params.Method)
			if meta == nil {
				return nil, MakeError(InvalidRequest, "method not found", nil)
			}
			return meta, nil
		}
		return listMetas(true), nil
	})

	// Metadata registration
	RegisterMeta("rpc.methods", &MethodMeta{
		Name:        "rpc.methods",
		Summary:     "List methods",
		Description: "Return the list of currently callable methods. By default, internal methods are not included. Pass internal=true to include them.",
		Params:      []ParamMeta{{Name: "internal", Type: "bool", Description: "Whether to include internal rpc.* methods"}},
		Returns:     "[]string",
	})
	RegisterMeta("rpc.version", &MethodMeta{Name: "rpc.version", Summary: "Return the RPC version", Returns: "string"})
	RegisterMeta("rpc.ping", &MethodMeta{Name: "rpc.ping", Summary: "Health check, returns pong", Returns: "string"})
	RegisterMeta("rpc.help", &MethodMeta{
		Name:        "rpc.help",
		Summary:     "Get method help",
		Description: "Returns detailed metadata for the specified method if given.",
		Params: []ParamMeta{
			{Name: "method", Type: "string", Description: "Target method name (mutually exclusive with list)"},
		},
		Returns: "MethodMeta",
	})
}
