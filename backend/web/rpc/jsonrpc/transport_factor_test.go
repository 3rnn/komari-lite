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
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/web/api"
	"github.com/pquerna/otp/totp"
)

func TestWebSocketRequiresMessageProofAfterFactorEnabled(t *testing.T) {
	method := "admin:securityFactorProbe"
	registerSecurityMethod(t, method, func(context.Context, *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) { return true, nil })
	rpc.MarkSensitive(method)
	user, err := accounts.CreateAccount(t.Name(), "test-only-password")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { accounts.DeleteAccountByUsername(user.Username) })
	session, err := accounts.CreateSession(user.UUID, 3600, "", "", "password")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { accounts.DeleteSession(session) })
	secret, _, err := accounts.Generate2Fa()
	if err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(api.IdentityMiddleware(), func(c *gin.Context) {
		c.Set("sensitive_2fa_verified", true) // A handshake-only snapshot is not message proof.
	})
	router.GET("/api/rpc2", OnRpcRequest)
	server := httptest.NewServer(router)
	defer server.Close()
	headers := http.Header{"Origin": {server.URL}, "Cookie": {(&http.Cookie{Name: "session_token", Value: session}).String()}, "X-2FA-Code": {code}}
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/rpc2", headers)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	call := func(params any) rpc.JsonRpcResponse {
		t.Helper()
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteJSON(rpc.NewRequest(1, method, params)); err != nil {
			t.Fatal(err)
		}
		var response rpc.JsonRpcResponse
		if err := conn.ReadJSON(&response); err != nil {
			t.Fatal(err)
		}
		return response
	}
	if call(nil).Error != nil {
		t.Fatal("account without factor denied")
	}
	if err := accounts.Enable2Fa(user.UUID, secret); err != nil {
		t.Fatal(err)
	}
	if response := call(nil); response.Error == nil || response.Error.Code != rpc.PermissionDenied {
		t.Fatal("handshake proof or old factor state bypassed message 2FA")
	}
	if call(map[string]any{"2fa_code": code}).Error != nil {
		t.Fatal("current message proof denied")
	}
}
