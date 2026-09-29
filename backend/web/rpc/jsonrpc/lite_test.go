package jsonrpc

import (
	"context"
	"strings"
	"testing"

	"github.com/komari-monitor/komari/pkg/rpc"
)

// Lite removes remote management, execution, terminal, and terminal settings from the code,
// not merely at runtime; their methods must not remain registered.
func TestLiteRemovedRPCsAreUnregistered(t *testing.T) {
	forbidden := []string{"exec", "terminal", "xtermjs", "remote", "taskresult", "gettasks"}
	registered := rpc.ListMethods()
	if len(registered) == 0 {
		t.Fatal("method registry is empty; the guard would pass vacuously")
	}
	for _, name := range registered {
		lower := strings.ToLower(name)
		for _, bad := range forbidden {
			if strings.Contains(lower, bad) {
				t.Errorf("method %q should have been removed from the monitoring build", name)
			}
		}
	}
}

// The dispatch denylist may only block removed methods. Blocking a registered method
// would leave a visible panel feature permanently unavailable.
func TestLiteDenylistNeverBlocksRegisteredMethods(t *testing.T) {
	registered := rpc.ListMethods()
	if len(registered) == 0 {
		t.Fatal("method registry is empty; the guard would pass vacuously")
	}
	// OAuth/OIDC login configuration is an explicit exception: lite policy disables this login subsystem
	// (and removes its frontend entry point), rather than treating it as deleted.
	policyDisabled := map[string]bool{
		"admin:getOidcProvider": true,
		"admin:setOidcProvider": true,
	}
	for _, name := range registered {
		if policyDisabled[name] {
			continue
		}
		resp := Dispatch(context.Background(), &rpc.ContextMeta{Permission: "admin"}, &rpc.JsonRpcRequest{Method: name})
		if resp.Error != nil && resp.Error.Code == -32601 {
			t.Errorf("method %q is still registered but the monitor-build denylist refuses it", name)
		}
	}
}

// Dispatch must reject these methods even for manually crafted requests.
func TestLiteBlockedRPC(t *testing.T) {
	for _, m := range []string{"admin:exec", "admin:getXtermjsSettings", "admin:getTasks", "admin:startCloudflared", "admin:listClipboard"} {
		r := Dispatch(context.Background(), &rpc.ContextMeta{Permission: "admin"}, &rpc.JsonRpcRequest{Method: m})
		if r.Error == nil || r.Error.Code != -32601 {
			t.Errorf("%s must be unavailable, got %+v", m, r.Error)
		}
	}
}
