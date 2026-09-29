package rpc

// sensitive.go
// Sensitive Operations Registration Form. Marked methods (such as admin:exec) still require additional
// Sensitive operation two-step verification (sensitive 2FA). Registration has nothing to do with transmission and is determined uniformly by RPC boundaries.
// Ensure that all call entries behave consistently.

import "sync"

var (
	muSensitive      sync.RWMutex
	sensitiveMethods = map[string]bool{}
)

func init() {
	// Keep this list beside the transport-level guard so every HTTP and
	// WebSocket JSON-RPC entry point enforces the same second factor.
	for _, method := range []string{
		"admin:rotateClientToken",
		"admin:editSettings",
		"admin:updateAccountPreferences",
		"admin:deleteAllSessions",
		"admin:removeClient",
		"admin:clearRecords",
		"admin:clearAllRecords",
	} {
		MarkSensitive(method)
	}
}

// MarkSensitive marks a method as a sensitive operation. method is the complete method name (such as "admin:exec").
// Repeat markers are idempotent. Provide method registry declaration.
func MarkSensitive(method string) {
	muSensitive.Lock()
	defer muSensitive.Unlock()
	sensitiveMethods[method] = true
}

// IsSensitive determines whether the method is marked as a sensitive operation.
func IsSensitive(method string) bool {
	muSensitive.RLock()
	defer muSensitive.RUnlock()
	return sensitiveMethods[method]
}
