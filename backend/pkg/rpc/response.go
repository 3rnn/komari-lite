package rpc

// JsonRpcResponse JSON-RPC 2.0 response
// Contains result when successful and error when failed; they are mutually exclusive.
// In case of Notification the server will not send any response.
type JsonRpcResponse struct {
	Version string        `json:"jsonrpc"`
	ID      any           `json:"id,omitempty"`
	Result  any           `json:"result,omitempty"`
	Error   *JsonRpcError `json:"error,omitempty"`
}

// SuccessResponse constructs a successful response
func SuccessResponse(id any, result any) *JsonRpcResponse {
	return &JsonRpcResponse{Version: RPC_VERSION, ID: id, Result: result}
}

// ErrorResponse constructs failure response
func ErrorResponse(id any, code int, msg string, data any) *JsonRpcResponse {
	return &JsonRpcResponse{Version: RPC_VERSION, ID: id, Error: &JsonRpcError{Code: code, Message: msg, Data: data}}
}

// InternalErrorResponse unified internal error
func InternalErrorResponse(id any, err error) *JsonRpcResponse {
	msg := "internal error"
	if err != nil {
		msg = err.Error()
	}
	return ErrorResponse(id, InternalError, msg, nil)
}
