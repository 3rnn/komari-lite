package server

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	v2 "github.com/nuomiiiii/lite-agent/protocol/v2"
	"github.com/nuomiiiii/lite-agent/ws"
)

func TestV2PingEventReportsTCPResult(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	go func() {
		connection, acceptErr := probe.Accept()
		if acceptErr == nil {
			connection.Close()
		}
	}()

	result := make(chan v2.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, upgradeErr := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if upgradeErr != nil {
			return
		}
		defer connection.Close()
		connection.SetReadDeadline(time.Now().Add(3 * time.Second))
		var message v2.Request
		if connection.ReadJSON(&message) == nil {
			result <- message
		}
	}))
	defer server.Close()
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if !processV2Event(ws.NewSafeConn(client), v2.MethodAgentPing, map[string]any{
		"ping_task_id": 7,
		"ping_type":    "tcp",
		"ping_target":  probe.Addr().String(),
	}, "") {
		t.Fatal("a valid TCP ping event must be handled")
	}
	select {
	case message := <-result:
		if message.Method != v2.MethodAgentPingResult {
			t.Fatalf("result method = %q", message.Method)
		}
		var params struct {
			TaskID uint `json:"task_id"`
			Value  int  `json:"value"`
		}
		if err := v2.BindParams(message.Params, &params); err != nil {
			t.Fatal(err)
		}
		if params.TaskID != 7 || params.Value < 0 {
			t.Fatalf("unexpected TCP result: task=%d, value=%d", params.TaskID, params.Value)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Agent did not send the ping result")
	}
}

func TestClosedWebSocketPingResultFallsBackToHTTP(t *testing.T) {
	probe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer probe.Close()

	received := make(chan int, 1)
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var message v2.Request
		if err := json.NewDecoder(r.Body).Decode(&message); err == nil && message.Method == v2.MethodAgentPingResult {
			var params struct {
				Value int `json:"value"`
			}
			if v2.BindParams(message.Params, &params) == nil {
				received <- params.Value
			}
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{}}`))
	}))
	defer rpc.Close()
	oldEndpoint, oldToken, oldCompression := flags.Endpoint, flags.Token, flags.DisableCompression
	flags.Endpoint, flags.Token, flags.DisableCompression = rpc.URL, "test-only", true
	defer func() { flags.Endpoint, flags.Token, flags.DisableCompression = oldEndpoint, oldToken, oldCompression }()

	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		connection.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, _, _ = connection.ReadMessage()
	}))
	defer wsServer.Close()
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(wsServer.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	client.Close() // Simulates the transport dropping after a task is dispatched.
	if !handlePingEvent(ws.NewSafeConn(client), map[string]any{
		"ping_task_id": 9, "ping_type": "http", "ping_target": probe.URL,
	}) {
		t.Fatal("valid ping event was not handled")
	}
	select {
	case value := <-received:
		if value < 0 {
			t.Fatalf("HTTP fallback reported lost ping: %d", value)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("WebSocket write failure must fall back to HTTP result reporting")
	}
}

func TestTimedOutHTTPProbeStillPostsLossResult(t *testing.T) {
	probe := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer probe.Close()
	received := make(chan int, 1)
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var message v2.Request
		if err := json.NewDecoder(r.Body).Decode(&message); err == nil && message.Method == v2.MethodAgentPingResult {
			var params struct {
				Value int `json:"value"`
			}
			if v2.BindParams(message.Params, &params) == nil {
				received <- params.Value
			}
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{}}`))
	}))
	defer rpc.Close()
	oldEndpoint, oldToken, oldCompression := flags.Endpoint, flags.Token, flags.DisableCompression
	flags.Endpoint, flags.Token, flags.DisableCompression = rpc.URL, "test-only", true
	defer func() { flags.Endpoint, flags.Token, flags.DisableCompression = oldEndpoint, oldToken, oldCompression }()
	if !handlePingEvent(nil, map[string]any{"ping_task_id": 8, "ping_type": "http", "ping_target": probe.URL}) {
		t.Fatal("HTTP ping event not handled")
	}
	select {
	case value := <-received:
		if value != -1 {
			t.Fatalf("timeout value = %d, want -1", value)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("timed-out probe must still report a loss through HTTP fallback")
	}
}
