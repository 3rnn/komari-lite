package rpc

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
)

// JsonRpcRequest represents a JSON-RPC 2.0 request (single or an element in a batch)
// Note: In order to distinguish Notification (without ID), the ID is defined as any and marked with omitempty.
// You can use req.HasID() to determine whether it is a normal request (needs to be returned) or a notification.
type JsonRpcRequest struct {
	Version string `json:"jsonrpc"`          // Required === "2.0"
	Method  string `json:"method"`           // Method name; those prefixed with "rpc." are reserved internal methods
	Params  any    `json:"params,omitempty"` // Parameters (positional array or named object)
	ID      any    `json:"id,omitempty"`     // String / value / null; omitted for Notification
}

// NewRequest creates a normal request (with id)
func NewRequest(id any, method string, params any) *JsonRpcRequest {
	return &JsonRpcRequest{Version: RPC_VERSION, Method: method, Params: params, ID: id}
}

// NewNotification creates a Notification (no id, no return will be received)
func NewNotification(method string, params any) *JsonRpcRequest {
	return &JsonRpcRequest{Version: RPC_VERSION, Method: method, Params: params}
}

// HasID determines whether it contains id (Notification does not have id and does not need to be returned)
func (r *JsonRpcRequest) HasID() bool { return r != nil && r.ID != nil }

// Validate verifies the legality of the request format (does not verify whether the method exists)
func (r *JsonRpcRequest) Validate() *JsonRpcError {
	if r == nil {
		return &JsonRpcError{Code: InvalidRequest, Message: "invalid request: null"}
	}
	if r.Version != RPC_VERSION {
		return &JsonRpcError{Code: InvalidRequest, Message: "invalid jsonrpc version"}
	}
	if strings.TrimSpace(r.Method) == "" {
		return &JsonRpcError{Code: InvalidRequest, Message: "method required"}
	}
	return nil
}

// GetParams (compatible with old interface): Get parameters by name (only map Object case), if not, the target remains unchanged
func (r *JsonRpcRequest) GetParams(name string, target *any) {
	if r == nil || r.Params == nil || target == nil {
		return
	}
	if m, ok := r.Params.(map[string]any); ok {
		*target = m[name]
	}
}

// GetParamAs gets a named parameter and attempts to convert to type T
func GetParamAs[T any](req *JsonRpcRequest, name string) (val T, ok bool) {
	if req == nil || req.Params == nil {
		return
	}
	if m, isMap := req.Params.(map[string]any); isMap {
		raw, exists := m[name]
		if !exists {
			return
		}
		if v, good := raw.(T); good {
			return v, true
		}
		b, err := json.Marshal(raw)
		if err != nil {
			return
		}
		var t T
		if err = json.Unmarshal(b, &t); err != nil {
			return
		}
		return t, true
	}
	return
}

// GetPositionalParamAs gets the value of positional parameter idx as T
func GetPositionalParamAs[T any](req *JsonRpcRequest, idx int) (val T, ok bool) {
	if req == nil || req.Params == nil {
		return
	}
	if arr, isArr := req.Params.([]any); isArr {
		if idx < 0 || idx >= len(arr) {
			return
		}
		raw := arr[idx]
		if v, good := raw.(T); good {
			return v, true
		}
		b, err := json.Marshal(raw)
		if err != nil {
			return
		}
		var t T
		if err = json.Unmarshal(b, &t); err != nil {
			return
		}
		return t, true
	}
	return
}

// BindParams Binds Params to the given structure pointer.
// Support:
//  1. object(map) -> Deserialize by field name (standard encoding/json behavior, case insensitive)
//  2. array([]any) -> If target is a structure pointer, fill it in order according to the declaration order of exported fields;
//     Shorter arrays: remaining fields retain zero values; longer arrays: extra elements are ignored.
//     Non-structure pointers are returned to the original logical overall deserialization.
//  3. Single scalar -> If target is a structure pointer, assign it to the first exported field; otherwise, deserialize it as a whole.
//  4. Other types -> Direct overall deserialization.
func (r *JsonRpcRequest) BindParams(target any) error {
	if r == nil {
		return errors.New("nil request")
	}
	if target == nil || reflect.ValueOf(target).Kind() != reflect.Ptr {
		return errors.New("target must be pointer")
	}
	if r.Params == nil {
		return nil
	}
	switch p := r.Params.(type) {
	case map[string]any:
		b, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, target)
	case []any:
		// Special treatment: struct pointers are mapped in field order
		rv := reflect.ValueOf(target).Elem()
		if rv.Kind() == reflect.Struct {
			rt := rv.Type()
			ai := 0
			for i := 0; i < rt.NumField() && ai < len(p); i++ {
				f := rt.Field(i)
				if f.PkgPath != "" { // Non-exported fields are skipped (do not occupy space)
					continue
				}
				fv := rv.Field(i)
				raw := p[ai]
				ai++
				// Fast path: direct assignment
				if raw != nil {
					val := reflect.ValueOf(raw)
					if val.IsValid() {
						if val.Type().AssignableTo(fv.Type()) {
							fv.Set(val)
							continue
						}
						if val.Type().ConvertibleTo(fv.Type()) {
							fv.Set(val.Convert(fv.Type()))
							continue
						}
					}
				}
				// Fallback: do an exact conversion via JSON (processing numbers float64 -> int, etc.)
				b, err := json.Marshal(raw)
				if err != nil {
					return err
				}
				tmp := reflect.New(fv.Type())
				if err = json.Unmarshal(b, tmp.Interface()); err != nil {
					return err
				}
				fv.Set(tmp.Elem())
			}
			return nil
		}
		// In non-struct cases, return to overall decoding.
		b, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, target)
	default:
		// Single scalar to first exported field of structure
		rv := reflect.ValueOf(target).Elem()
		if rv.Kind() == reflect.Struct {
			rt := rv.Type()
			for i := 0; i < rt.NumField(); i++ {
				f := rt.Field(i)
				if f.PkgPath != "" { // non-export
					continue
				}
				fv := rv.Field(i)
				raw := p
				if raw != nil {
					val := reflect.ValueOf(raw)
					if val.IsValid() {
						if val.Type().AssignableTo(fv.Type()) {
							fv.Set(val)
							return nil
						}
						if val.Type().ConvertibleTo(fv.Type()) {
							fv.Set(val.Convert(fv.Type()))
							return nil
						}
					}
				}
				// fallback json conversion
				b, err := json.Marshal(raw)
				if err != nil {
					return err
				}
				tmp := reflect.New(fv.Type())
				if err = json.Unmarshal(b, tmp.Interface()); err != nil {
					return err
				}
				fv.Set(tmp.Elem())
				return nil
			}
			// No export fields
			return nil
		}
		b, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, target)
	}
}
