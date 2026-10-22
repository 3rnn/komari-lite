package jsonrpc

import (
	"context"
	"testing"

	"github.com/komari-monitor/komari/pkg/rpc"
)

func TestDispatchRejectsSuccessAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	method := "public:securityLateSuccess"
	registerSecurityMethod(t, method, func(context.Context, *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
		cancel()
		return "must not be returned as success", nil
	})
	response := dispatchWithSensitive(ctx, nil, &rpc.ContextMeta{Principal: rpc.NewAnonymousPrincipal()}, rpc.NewRequest(1, method, nil))
	if response.Error == nil || response.Error.Code != rpc.InternalError {
		t.Fatal("cancelled work returned a successful result")
	}
}
