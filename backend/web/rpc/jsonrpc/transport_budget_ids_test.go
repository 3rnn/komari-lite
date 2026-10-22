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
	"github.com/komari-monitor/komari/pkg/rpc"
)

func TestRPCBudgetErrorsPreserveKnownIDs(t *testing.T) {
	for _, kind := range []string{"response", "cancelled", "encoding"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			method := "public:budgetIDs" + kind
			registerSecurityMethod(t, method, func(context.Context, *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
				calls++
				if kind == "cancelled" {
					cancel()
					return true, nil
				}
				if kind == "encoding" {
					return make(chan int), nil
				}
				return strings.Repeat("x", 3<<20), nil
			})
			router := gin.New()
			router.POST("/rpc", OnRpcRequest)
			body := `[{"jsonrpc":"2.0","id":"first","method":"` + method + `"},{"jsonrpc":"2.0","id":2,"method":"` + method + `"},{"jsonrpc":"2.0","id":"third","method":"` + method + `"}]`
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(body)).WithContext(ctx))
			var responses []rpc.JsonRpcResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &responses); err != nil {
				t.Fatal(err)
			}
			if len(responses) != 3 {
				t.Fatalf("pending calls did not receive correlated errors: got %d responses", len(responses))
			}
			for i, want := range []any{"first", float64(2), "third"} {
				if responses[i].ID != want {
					t.Fatalf("response %d ID = %v, want %v", i, responses[i].ID, want)
				}
			}
			if responses[1].Error == nil || responses[2].Error == nil {
				t.Fatal("budget exhausted calls missing errors")
			}
			if recorder.Body.Len() > maxRPCResponseBytes {
				t.Fatal("response exceeded budget")
			}
			if ((kind == "cancelled" || kind == "encoding") && calls != 1) || (kind == "response" && calls != 2) {
				t.Fatalf("unexpected dispatch count %d", calls)
			}
		})
	}
}

func TestRPCPostBudgetErrorsBoundEscapedIDs(t *testing.T) {
	method := "public:budgetEscapedIDs"
	registerSecurityMethod(t, method, func(context.Context, *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
		t.Fatal("cancelled batch dispatched work")
		return nil, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	body := `[{"jsonrpc":"2.0","id":"` + strings.Repeat("<", 800000) + `","method":"` + method + `"}`
	for i := 1; i < rpc.MaxBatchRequests; i++ {
		body += `,{"jsonrpc":"2.0","id":"known-id","method":"` + method + `"}`
	}
	body += `]`
	router := gin.New()
	router.POST("/rpc", OnRpcRequest)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(body)).WithContext(ctx))
	if recorder.Body.Len() > maxRPCResponseBytes {
		t.Fatal("escaped error IDs exceeded response budget")
	}
	var responses []rpc.JsonRpcResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &responses); err != nil {
		t.Fatal(err)
	}
	if len(responses) != rpc.MaxBatchRequests || responses[0].ID != nil {
		t.Fatal("amplifying ID did not fall back safely")
	}
	for _, response := range responses[1:] {
		if response.ID != "known-id" || response.Error == nil {
			t.Fatal("ordinary IDs lost correlation alongside oversized ID")
		}
	}
}

func TestWebSocketBudgetErrorPreservesKnownID(t *testing.T) {
	method := "public:budgetWebSocketID"
	registerSecurityMethod(t, method, func(context.Context, *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
		return strings.Repeat("x", maxRPCResponseBytes), nil
	})
	router := gin.New()
	router.GET("/rpc", OnRpcRequest)
	server := httptest.NewServer(router)
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/rpc", http.Header{"Origin": {server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteJSON(rpc.NewRequest("known-id", method, nil)); err != nil {
		t.Fatal(err)
	}
	var response rpc.JsonRpcResponse
	if err := conn.ReadJSON(&response); err != nil {
		t.Fatal(err)
	}
	if response.Error == nil || response.ID != "known-id" {
		t.Fatalf("budget error lost known ID: %#v", response)
	}
}
