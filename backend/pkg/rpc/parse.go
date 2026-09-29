package rpc

import (
	"bytes"
	"encoding/json"
)

// ParseRequest Parses a single JSON-RPC request. Return request and error (parsing level).
func ParseRequest(data []byte) (*JsonRpcRequest, *JsonRpcError) {
	requests, err := ParseRequests(data)
	if err != nil {
		return nil, err
	}
	if len(requests) == 0 {
		return nil, &JsonRpcError{Code: InvalidRequest, Message: "no requests found"}
	}
	return requests[0], nil
}

// ParseRequests Parses single or batch JSON-RPC requests. Return request slices and errors (parsing level),
// If the array is a batch of empty arrays, an InvalidRequest error will be returned (protocol requirement).
func ParseRequests(data []byte) ([]*JsonRpcRequest, *JsonRpcError) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, &JsonRpcError{Code: ParseError, Message: "empty body"}
	}
	first := data[0]
	if first == '{' { // single
		var r JsonRpcRequest
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, &JsonRpcError{Code: ParseError, Message: "invalid json", Data: err.Error()}
		}
		if e := r.Validate(); e != nil {
			return nil, e
		}
		return []*JsonRpcRequest{&r}, nil
	}
	if first == '[' { // batch
		var arr []JsonRpcRequest
		if err := json.Unmarshal(data, &arr); err != nil {
			return nil, &JsonRpcError{Code: ParseError, Message: "invalid json", Data: err.Error()}
		}
		if len(arr) == 0 {
			return nil, &JsonRpcError{Code: InvalidRequest, Message: "empty batch"}
		}
		res := make([]*JsonRpcRequest, 0, len(arr))
		for i := range arr {
			rr := arr[i]
			if e := rr.Validate(); e != nil {
				return nil, e
			}
			res = append(res, &rr)
		}
		return res, nil
	}
	return nil, &JsonRpcError{Code: ParseError, Message: "invalid json: not object/array"}
}
