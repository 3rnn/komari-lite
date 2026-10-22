package jsonrpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/web/api"
)

func registerSecurityMethod(t *testing.T, name string, handler rpc.Handler) {
	t.Helper()
	dbcore.GetDBInstance()
	gin.SetMode(gin.TestMode)
	if err := rpc.Register(name, handler); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rpc.Unregister(name) })
}

func TestWebSocketRevalidatesRevokedCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	method := "admin:securityProbe"
	registerSecurityMethod(t, method, func(context.Context, *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) { return "allowed", nil })
	user, err := accounts.CreateAccount("rpc-security-user", "test-only-password")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { accounts.DeleteAccountByUsername(user.Username) })
	for _, kind := range []string{"deleted session", "expired session", "deleted account", "rotated API key"} {
		t.Run(kind, func(t *testing.T) {
			headers := http.Header{}
			var token string
			if kind == "rotated API key" {
				if err := config.Set(config.ApiKeyKey, "test-only-key-123456"); err != nil {
					t.Fatal(err)
				}
				headers.Set("Authorization", "Bearer test-only-key-123456")
				t.Cleanup(func() { config.Set(config.ApiKeyKey, "") })
			} else {
				token, err = accounts.CreateSession(user.UUID, 3600, "", "", "password")
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { accounts.DeleteSession(token) })
				headers.Set("Cookie", (&http.Cookie{Name: "session_token", Value: token}).String())
			}
			router := gin.New()
			router.Use(api.IdentityMiddleware()) // Deliberately installs the stale handshake snapshot.
			router.GET("/api/rpc2", OnRpcRequest)
			server := httptest.NewServer(router)
			defer server.Close()
			headers.Set("Origin", server.URL)
			conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/rpc2", headers)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			call := func() rpc.JsonRpcResponse {
				t.Helper()
				conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				if err := conn.WriteJSON(rpc.JsonRpcRequest{Version: "2.0", ID: 1, Method: method}); err != nil {
					t.Fatal(err)
				}
				var response rpc.JsonRpcResponse
				if err := conn.ReadJSON(&response); err != nil {
					t.Fatal(err)
				}
				return response
			}
			if response := call(); response.Error != nil {
				t.Fatal("live credential rejected")
			}
			switch kind {
			case "deleted session":
				err = accounts.DeleteSession(token)
			case "expired session":
				err = dbcore.GetDBInstance().Model(&models.Session{}).Where("session = ?", token).Update("expires", time.Now().Add(-time.Minute)).Error
			case "deleted account":
				err = dbcore.GetDBInstance().Where("uuid = ?", user.UUID).Delete(&models.User{}).Error
				t.Cleanup(func() { dbcore.GetDBInstance().Create(&user) })
			case "rotated API key":
				err = config.Set(config.ApiKeyKey, "test-only-rotated-key")
			}
			if err != nil {
				t.Fatal(err)
			}
			if response := call(); response.Error == nil || response.Error.Code != rpc.PermissionDenied {
				t.Fatal("revoked credential retained privileged WebSocket access")
			}
		})
	}
}

func TestWebSocketBoundsInvalidRequestResponse(t *testing.T) {
	dbcore.GetDBInstance()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/rpc2", OnRpcRequest)
	server := httptest.NewServer(router)
	defer server.Close()
	headers := http.Header{"Origin": []string{server.URL}}
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/rpc2", headers)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"jsonrpc":"invalid","id":"`+strings.Repeat("<", 800000)+`","method":"public:getVersion"}`)); err != nil {
		t.Fatal(err)
	}
	_, encoded, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > 4<<20 {
		t.Fatalf("invalid-request response exceeded budget: %d", len(encoded))
	}
	if !json.Valid(encoded) {
		t.Fatal("invalid JSON response")
	}
}

func TestRPCPostBoundsResponseAndStopsBatchWork(t *testing.T) {
	calls := 0
	method := "public:securityLargeResult"
	registerSecurityMethod(t, method, func(context.Context, *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
		calls++
		return strings.Repeat("x", 3<<20), nil
	})
	body := `[{"jsonrpc":"2.0","id":1,"method":"` + method + `"},{"jsonrpc":"2.0","id":2,"method":"` + method + `"},{"jsonrpc":"2.0","id":3,"method":"` + method + `"}]`
	router := gin.New()
	router.POST("/api/rpc2", OnRpcRequest)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/rpc2", strings.NewReader(body)))
	if response.Body.Len() > 4<<20 {
		t.Fatalf("response exceeded 4 MiB: %d", response.Body.Len())
	}
	if calls > 2 {
		t.Fatalf("continued dispatch after response budget exhausted: %d", calls)
	}
	if !json.Valid(response.Body.Bytes()) {
		t.Fatal("budget response is not valid JSON")
	}
}

func TestRPCPostStopsCancelledBatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	method := "public:securityCancel"
	registerSecurityMethod(t, method, func(context.Context, *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
		calls++
		cancel()
		return "done", nil
	})
	body := `[{"jsonrpc":"2.0","id":1,"method":"` + method + `"},{"jsonrpc":"2.0","id":2,"method":"` + method + `"}]`
	router := gin.New()
	router.POST("/api/rpc2", OnRpcRequest)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/rpc2", strings.NewReader(body)).WithContext(ctx)
	router.ServeHTTP(response, request)
	if calls != 1 {
		t.Fatalf("cancelled batch executed %d calls", calls)
	}
}
