package rpc

import "fmt"

// JsonRpcError error object
// Code uses the reserved range of -32768 ~ -32000 according to the JSON-RPC specification. Other business customizations can use positive numbers or custom intervals.
// Message is a short description, and Data can carry extended information (structured or string).
type JsonRpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Predefined error codes JSON-RPC 2.0 standard
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// Komari defines error code
const (
	NotFound         = -32044
	AlreadyExists    = -32045
	PermissionDenied = -32041
	Unauthenticated  = -32040 // Not logged in/no identity
	Cancelled        = -32010 // Cancel proactively
	DeadlineExceeded = -32011 // timeout
	Aborted          = -32021 // Concurrency conflicts/transaction interruptions
	OutOfRange       = -32022 // Value/index out of bounds
	Unimplemented    = -32050 // not yet implemented
	Unavailable      = -32051 // Dependent services are temporarily unavailable
	DataLoss         = -32052 // Unrecoverable data loss
)

// MakeError conveniently creates error objects
func MakeError(code int, msg string, data any) *JsonRpcError {
	return &JsonRpcError{Code: code, Message: msg, Data: data}
}

func (e *JsonRpcError) Error() string {
	return fmt.Sprintf("JSON-RPC Error %d: %s", e.Code, e.Message)
}

func (e *JsonRpcError) Response() *JsonRpcResponse {
	return e.ResponseWithID(nil)
}
func (e *JsonRpcError) ResponseWithID(id any) *JsonRpcResponse {
	return &JsonRpcResponse{
		Version: RPC_VERSION,
		Error:   e,
		ID:      id,
	}
}
