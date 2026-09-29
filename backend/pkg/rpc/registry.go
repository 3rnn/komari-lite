package rpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Handler method signature: return result (success) or *JsonRpcError (failure)
type Handler func(ctx context.Context, req *JsonRpcRequest) (any, *JsonRpcError)

var (
	muHandlers sync.RWMutex
	handlers   = map[string]Handler{}
)

// Register registration method. Repeated registration returns an error. Reserve the prefix "rpc." to disable external registration.
func Register(method string, h Handler) error {
	method = strings.TrimSpace(method)
	if method == "" {
		return errors.New("method empty")
	}
	if strings.HasPrefix(method, "rpc.") {
		return errors.New("method prefix 'rpc.' is reserved")
	}
	muHandlers.Lock()
	defer muHandlers.Unlock()
	if _, exists := handlers[method]; exists {
		return fmt.Errorf("method already registered: %s", method)
	}
	handlers[method] = h
	return nil
}

// MustRegister convenient registration (panic on error)
func MustRegister(method string, h Handler) {
	if err := Register(method, h); err != nil {
		panic(err)
	}
}

// Unregister Unregisters a registered method and cleans its metadata.
// Internal methods that retain the prefix "rpc." disable logout. Returns whether it exists and was removed.
// It is mainly used to dynamically remove the registration of plug-ins when they are uninstalled.
func Unregister(method string) bool {
	method = strings.TrimSpace(method)
	if method == "" || strings.HasPrefix(method, "rpc.") {
		return false
	}
	muHandlers.Lock()
	_, exists := handlers[method]
	if exists {
		delete(handlers, method)
	}
	muHandlers.Unlock()
	if exists {
		muMetas.Lock()
		delete(methodMetas, method)
		muMetas.Unlock()
	}
	return exists
}

// ListMethods lists currently registered method names (copies)
func ListMethods() []string {
	muHandlers.RLock()
	defer muHandlers.RUnlock()
	res := make([]string, 0, len(handlers))
	for k := range handlers {
		res = append(res, k)
	}
	return res
}
