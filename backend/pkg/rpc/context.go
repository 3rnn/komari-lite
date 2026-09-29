package rpc

// context.go
// Defines contextual metainformation that can be appended to an RPC execution. Used to obtain the authentication/subject information of the current request in the handler.
// Note: To avoid circular dependencies, only models.User is referenced here (database/models does not reference this package).

import (
	"context"

	"github.com/komari-monitor/komari/database/models"
)

// ContextMeta holds authentication/identity metadata available for an RPC call.
// In order to facilitate expansion, the fields remain redundant: both the structure and the corresponding UUID/Token are saved.
// If you need to add fields (such as IP, UserAgent, TraceID, etc.) in the future, you can directly extend this structure.
type ContextMeta struct {
	// Principal calls the principal (identity + ability). The authoritative source of the new identity model, unified by the identity recognition layer.
	// The Permission field is reserved for backward compatibility and is equivalent to Principal.PrimaryRole().
	Principal *Principal

	// Permission current permission group guest/client/admin
	Permission string
	// User logged in administrator user (only admin session exists)
	User *models.User
	// UserUUID facilitates quick judgment without dereferencing
	UserUUID string
	// ClientToken Private key/authentication token from the client (only client links may exist)
	ClientToken string
	// ClientUUID The client UUID parsed (if the token is legal)
	ClientUUID string
	// SessionToken The session token of the current administrator session (only exists for the admin session and is used to distinguish the current session and other scenarios)
	SessionToken string
	// RemoteIP request source IP (optional)
	RemoteIP string
	// UserAgent requests UA (optional)
	UserAgent string
	// TempShareValid Whether the temporary share access permission is valid (based on temp_key cookie verification, filled by the transport layer)
	TempShareValid bool
}

// Use private types as keys to avoid external conflicts
type ctxMetaKey struct{}

// NewContextWithMeta writes meta to context
func NewContextWithMeta(parent context.Context, meta *ContextMeta) context.Context {
	if parent == nil {
		parent = context.Background()
	}
	if meta == nil {
		return parent
	}
	return context.WithValue(parent, ctxMetaKey{}, meta)
}

// MetaFromContext reads meta; returns nil if it does not exist
func MetaFromContext(ctx context.Context) *ContextMeta {
	if ctx == nil {
		return nil
	}
	if v := ctx.Value(ctxMetaKey{}); v != nil {
		if m, ok := v.(*ContextMeta); ok {
			return m
		}
	}
	return nil
}
