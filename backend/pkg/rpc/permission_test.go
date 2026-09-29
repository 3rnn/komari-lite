package rpc

import (
	"context"
	"testing"
)

func TestNamespaceOf(t *testing.T) {
	cases := map[string]string{
		"getNodes":        DefaultNamespace,
		"admin:addClient": "admin",
		"client:report":   "client",
		"rpc.ping":        DefaultNamespace, // Without ":", fall into the default namespace
		":bare":           "",
	}
	for method, want := range cases {
		if got := NamespaceOf(method); got != want {
			t.Errorf("NamespaceOf(%q) = %q, want %q", method, got, want)
		}
	}
}

func TestCheckPermission(t *testing.T) {
	cases := []struct {
		group  string
		method string
		want   bool
	}{
		{RoleGuest, "common:getNodes", true},
		{RoleGuest, "getNodes", true},
		{RoleGuest, "admin:addClient", false},
		{RoleGuest, "client:report", false},
		{RoleClient, "client:report", true},
		{RoleClient, "admin:addClient", false},
		{RoleAdmin, "admin:addClient", true},
		{RoleAdmin, "client:report", false},
		{RoleAdmin, "common:getNodes", true},
		// Unknown namespaces require admin by default
		{RoleGuest, "plugin:foo", false},
		{RoleAdmin, "plugin:foo", true},
	}
	for _, c := range cases {
		if got := CheckPermission(c.group, c.method); got != c.want {
			t.Errorf("CheckPermission(%q, %q) = %v, want %v", c.group, c.method, got, c.want)
		}
	}
}

func TestRegisterNamespace(t *testing.T) {
	RegisterNamespace("myplugin", RoleClient)
	if !CheckPermission(RoleClient, "myplugin:doThing") {
		t.Error("client should be allowed in myplugin namespace after registration")
	}
	if CheckPermission(RoleGuest, "myplugin:doThing") {
		t.Error("guest should not be allowed in myplugin namespace")
	}
}

func TestAllowWildcardSpecificity(t *testing.T) {
	// The namespace defaults to admin, but more specific method-level rules can be relaxed.
	RegisterNamespace("acltest", RoleAdmin)
	Allow("acltest:public*", RoleGuest) // Prefix wildcard, more specific than "acltest:*"

	if !CheckPermission(RoleGuest, "acltest:publicInfo") {
		t.Error("guest should be allowed: acltest:public* is more specific than acltest:*")
	}
	if CheckPermission(RoleGuest, "acltest:secret") {
		t.Error("guest should be denied acltest:secret (falls back to acltest:* = admin)")
	}

	// Exact rules take precedence over any wildcards.
	Allow("acltest:secret", RoleClient)
	if !CheckPermission(RoleClient, "acltest:secret") {
		t.Error("client should be allowed by exact rule acltest:secret")
	}
	if CheckPermission(RoleGuest, "acltest:secret") {
		t.Error("guest still denied acltest:secret (exact rule requires client)")
	}
}

func TestAllowOverride(t *testing.T) {
	Allow("override:x", RoleAdmin)
	if CheckPermission(RoleGuest, "override:x") {
		t.Fatal("precondition: guest denied")
	}
	Allow("override:x", RoleGuest) // Cover the same pattern
	if !CheckPermission(RoleGuest, "override:x") {
		t.Error("override should relax override:x to guest")
	}
}

func TestInternalMethodsPermission(t *testing.T) {
	// Internal rpc.* methods are exposed to guests.
	if !CheckPermission(RoleGuest, "rpc.ping") {
		t.Error("guest should be allowed to call rpc.ping")
	}
	// Naked method names belong to common and are open to guests.
	if !CheckPermission(RoleGuest, "getNodes") {
		t.Error("guest should be allowed to call bare common method getNodes")
	}
}

func TestUnregister(t *testing.T) {
	method := "unreg.sample"
	MustRegister(method, func(ctx context.Context, req *JsonRpcRequest) (any, *JsonRpcError) {
		return "ok", nil
	})
	if getHandler(method) == nil {
		t.Fatal("method should be registered")
	}
	if !Unregister(method) {
		t.Fatal("Unregister should return true for existing method")
	}
	if getHandler(method) != nil {
		t.Fatal("method should be removed after Unregister")
	}
	// Repeated logout returns false
	if Unregister(method) {
		t.Fatal("Unregister should return false for missing method")
	}
	// Keep prefix to disable logout
	if Unregister("rpc.ping") {
		t.Fatal("Unregister must refuse rpc.* reserved methods")
	}
	if getHandler("rpc.ping") == nil {
		t.Fatal("rpc.ping must still be registered")
	}
}
