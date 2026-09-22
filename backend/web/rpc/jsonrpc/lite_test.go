package jsonrpc

import (
	"context"
	"strings"
	"testing"

	"github.com/komari-monitor/komari/pkg/rpc"
)

// 精简版已把「远程管理 / 远程执行 / 远程终端 / 终端设置」从代码里删除，
// 而不是仅在运行时拒绝：注册表中不应再存在这些方法。
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

// 分发层的黑名单只能拦截「已经删除」的方法。任何仍在注册表中的方法被拒绝，
// 都意味着面板存在一个永远不可用的功能。
func TestLiteDenylistNeverBlocksRegisteredMethods(t *testing.T) {
	registered := rpc.ListMethods()
	if len(registered) == 0 {
		t.Fatal("method registry is empty; the guard would pass vacuously")
	}
	// 明确保留的例外：OAuth/OIDC 登录配置属于登录子系统，精简版按策略关闭
	//（前端入口已一并移除），不作为「已删除功能」处理。
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

// 即使有人手写请求，分发层也必须拒绝这些方法。
func TestLiteBlockedRPC(t *testing.T) {
	for _, m := range []string{"admin:exec", "admin:getXtermjsSettings", "admin:getTasks", "admin:startCloudflared", "admin:listClipboard"} {
		r := Dispatch(context.Background(), &rpc.ContextMeta{Permission: "admin"}, &rpc.JsonRpcRequest{Method: m})
		if r.Error == nil || r.Error.Code != -32601 {
			t.Errorf("%s must be unavailable, got %+v", m, r.Error)
		}
	}
}
