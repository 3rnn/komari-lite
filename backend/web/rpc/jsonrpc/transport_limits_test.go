package jsonrpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/web/api"
)

func TestWebSocketRevalidatesTemporaryShare(t *testing.T) {
	method := "public:securityShareProbe"
	registerSecurityMethod(t, method, func(context.Context, *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) { return true, nil })
	oldPrivate, _ := config.GetAs[bool](config.PrivateSiteKey, false)
	oldShare, _ := config.GetAs[string]("tempory_share_token", "")
	oldExpiry, _ := config.GetAs[int64]("tempory_share_token_expire_at", 0)
	t.Cleanup(func() {
		config.Set(config.PrivateSiteKey, oldPrivate)
		config.Set("tempory_share_token", oldShare)
		config.Set("tempory_share_token_expire_at", oldExpiry)
	})
	for key, value := range map[string]any{
		config.PrivateSiteKey:           true,
		"tempory_share_token":           "test-only-share",
		"tempory_share_token_expire_at": time.Now().Add(time.Hour).Unix(),
	} {
		if err := config.Set(key, value); err != nil {
			t.Fatal(err)
		}
	}
	router := gin.New()
	router.Use(api.IdentityMiddleware())
	router.GET("/api/rpc2", OnRpcRequest)
	server := httptest.NewServer(router)
	defer server.Close()
	headers := http.Header{"Origin": {server.URL}, "Cookie": {"temp_key=test-only-share"}}
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/rpc2", headers)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	call := func() rpc.JsonRpcResponse {
		t.Helper()
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteJSON(rpc.NewRequest(1, method, nil)); err != nil {
			t.Fatal(err)
		}
		var response rpc.JsonRpcResponse
		if err := conn.ReadJSON(&response); err != nil {
			t.Fatal(err)
		}
		return response
	}
	if call().Error != nil {
		t.Fatal("live temporary share denied")
	}
	if err := config.Set("tempory_share_token", "test-only-rotated-share"); err != nil {
		t.Fatal(err)
	}
	if response := call(); response.Error == nil || response.Error.Code != rpc.PermissionDenied {
		t.Fatal("rotated temporary share retained access")
	}
}

func TestRPCTransportsRejectOversizedRequests(t *testing.T) {
	calls := 0
	method := "public:securitySizeProbe"
	registerSecurityMethod(t, method, func(context.Context, *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) { calls++; return true, nil })
	router := gin.New()
	router.Any("/api/rpc2", OnRpcRequest)
	body := `{"jsonrpc":"2.0","id":1,"method":"` + method + `","params":{"padding":"` + strings.Repeat("x", maxRPCRequestBytes) + `"}}`
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/rpc2", strings.NewReader(body)))
	if response.Code != http.StatusBadRequest || calls != 0 {
		t.Fatal("oversized POST dispatched a method")
	}
	server := httptest.NewServer(router)
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/rpc2", http.Header{"Origin": {server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	// The peer can close while WriteMessage is still sending the oversized frame.
	_ = conn.WriteMessage(websocket.TextMessage, []byte(body))
	_, _, err = conn.ReadMessage()
	if !websocket.IsCloseError(err, websocket.CloseMessageTooBig) {
		t.Fatalf("oversized frame was not closed with 1009: %v", err)
	}
	if calls != 0 {
		t.Fatal("oversized WebSocket request dispatched a method")
	}
}
