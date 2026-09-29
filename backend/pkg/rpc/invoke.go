package rpc

import "context"

// Invoke convenience call: constructs a request and executes the registered method, returning result or *JsonRpcError.
// JsonRpcResponse is not generated.
//
// @ref Call
func Invoke(method string, params any) (any, *JsonRpcError) {
	req := &JsonRpcRequest{Version: RPC_VERSION, Method: method, Params: params}
	if e := req.Validate(); e != nil {
		return nil, e
	}
	muHandlers.RLock()
	h, ok := handlers[method]
	muHandlers.RUnlock()
	if !ok {
		return nil, &JsonRpcError{Code: MethodNotFound, Message: "method not found", Data: method}
	}
	return h(context.Background(), req)
}

// Call executes the method and returns the complete JSON-RPC Response directly.
// Suitable for external exposure: always return a structure (including errors).
// ctx: execution context; id: request id; method/params: method and parameters.
func Call(id any, method string, params any) *JsonRpcResponse {
	return CallWithContext(context.Background(), id, method, params)
}

func CallWithContext(ctx context.Context, id any, method string, params any) *JsonRpcResponse {
	if ctx == nil {
		ctx = context.Background()
	}
	req := &JsonRpcRequest{Version: RPC_VERSION, Method: method, Params: params, ID: id}
	if e := req.Validate(); e != nil {
		return ErrorResponse(id, e.Code, e.Message, e.Data)
	}
	muHandlers.RLock()
	h, ok := handlers[method]
	muHandlers.RUnlock()
	if !ok {
		return ErrorResponse(id, MethodNotFound, "method not found", method)
	}
	result, jerr := h(ctx, req)
	if jerr != nil {
		return ErrorResponse(id, jerr.Code, jerr.Message, jerr.Data)
	}
	return SuccessResponse(id, result)
}
