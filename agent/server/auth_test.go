package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func configureAccessTest(t *testing.T, endpoint, clientID, clientSecret string) {
	t.Helper()
	originalEndpoint := flags.Endpoint
	originalToken := flags.Token
	originalID := flags.CFAccessClientID
	originalSecret := flags.CFAccessClientSecret
	originalCompression := flags.DisableCompression
	originalRetries := flags.MaxRetries
	flags.Endpoint = endpoint
	flags.Token = "agent-token"
	flags.CFAccessClientID = clientID
	flags.CFAccessClientSecret = clientSecret
	flags.DisableCompression = true
	flags.MaxRetries = 0
	t.Cleanup(func() {
		flags.Endpoint = originalEndpoint
		flags.Token = originalToken
		flags.CFAccessClientID = originalID
		flags.CFAccessClientSecret = originalSecret
		flags.DisableCompression = originalCompression
		flags.MaxRetries = originalRetries
	})
}

func requireAccessHeaders(request *http.Request) bool {
	return request.Header.Get("Authorization") == "Bearer agent-token" &&
		request.Header.Get("CF-Access-Client-Id") == "access-id" &&
		request.Header.Get("CF-Access-Client-Secret") == "access-secret"
}

func TestAgentAuthorizationUsesHeader(t *testing.T) {
	configureAccessTest(t, "", "access-id", "access-secret")
	request, err := http.NewRequest(http.MethodGet, "https://example.com/api/clients/v2/rpc", nil)
	if err != nil {
		t.Fatal(err)
	}
	authorizeAgentRequest(request, "secret-token")
	if got := request.Header.Get("Authorization"); got != "Bearer secret-token" {
		t.Fatalf("unexpected Authorization header: %q", got)
	}
	if got := request.Header.Get("CF-Access-Client-Id"); got != "access-id" {
		t.Fatalf("unexpected Cloudflare Access client ID: %q", got)
	}
	if got := request.Header.Get("CF-Access-Client-Secret"); got != "access-secret" {
		t.Fatalf("unexpected Cloudflare Access client secret: %q", got)
	}
	if request.URL.RawQuery != "" {
		t.Fatalf("agent credential leaked into query: %q", request.URL.RawQuery)
	}
}

func TestRemoteHeadersAreLiteOnly(t *testing.T) {
	configureAccessTest(t, "", "access-id", "access-secret")

	remoteHeaders := remoteAuthorizationHeader("agent-token", "remote-session", "remote-ticket")
	if remoteHeaders.Get("X-Lite-Remote-Session") != "remote-session" ||
		remoteHeaders.Get("X-Lite-Remote-Ticket") != "remote-ticket" ||
		!headersContainAccessCredentials(remoteHeaders) {
		t.Fatalf("remote headers are incomplete: %#v", remoteHeaders)
	}
	for _, forbidden := range []string{
		"X-Komari-Remote-Session",
		"X-Komari-Remote-Ticket",
		"X-Komari-Terminal-Session",
		"X-Lite-Terminal-Session",
	} {
		if got := remoteHeaders.Get(forbidden); got != "" {
			t.Fatalf("forbidden header %s = %q", forbidden, got)
		}
	}
}

func headersContainAccessCredentials(headers http.Header) bool {
	return headers.Get("Authorization") == "Bearer agent-token" &&
		headers.Get("CF-Access-Client-Id") == "access-id" &&
		headers.Get("CF-Access-Client-Secret") == "access-secret"
}

func TestCloudflareAccessProtectedWebSocket(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !requireAccessHeaders(request) {
			http.Error(writer, "Cloudflare Access denied", http.StatusForbidden)
			return
		}
		connection, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		_ = connection.Close()
	}))
	t.Cleanup(server.Close)

	websocketEndpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/clients/v2/rpc"
	configureAccessTest(t, server.URL, "", "")
	if connection, err := connectWebSocket(websocketEndpoint); err == nil {
		_ = connection.Close()
		t.Fatal("WebSocket unexpectedly passed Cloudflare Access without service-token headers")
	}

	flags.CFAccessClientID = "access-id"
	flags.CFAccessClientSecret = "access-secret"
	connection, err := connectWebSocket(websocketEndpoint)
	if err != nil {
		t.Fatalf("WebSocket with both authentication layers failed: %v", err)
	}
	_ = connection.Close()
}

func TestWebSocketEndpointDoesNotContainAgentToken(t *testing.T) {
	originalEndpoint, originalToken := flags.Endpoint, flags.Token
	flags.Endpoint, flags.Token = "https://example.com", "secret-token"
	t.Cleanup(func() {
		flags.Endpoint, flags.Token = originalEndpoint, originalToken
	})

	endpoint := buildWebSocketEndpoint()
	parsed, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.RawQuery != "" {
		t.Fatalf("websocket URL contains query data: %q", parsed.RawQuery)
	}
	if endpoint == "" || parsed.Host != "example.com" {
		t.Fatalf("unexpected websocket endpoint: %q", endpoint)
	}
	if !strings.HasSuffix(parsed.Path, "/api/clients/v2/rpc") {
		t.Fatalf("unexpected websocket path: %q", parsed.Path)
	}
}
