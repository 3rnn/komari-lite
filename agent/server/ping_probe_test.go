package server

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestHTTPPingProbeUsesOneLocalRequest(t *testing.T) {
	requests := 0
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer endpoint.Close()
	if value := probeLatency(context.Background(), "http", endpoint.URL); value < 1 {
		t.Fatalf("HTTP latency = %d, want at least 1 ms", value)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want one", requests)
	}
}

func TestTCPPingProbeReportsLossOnClosedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	if value := probeLatency(context.Background(), "tcp", address); value != -1 {
		t.Fatalf("unreachable TCP latency = %d, want -1", value)
	}
}

func TestICMPPingProbeOnLoopback(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("ICMP raw sockets require root on this runner")
	}
	if value := probeLatency(context.Background(), "icmp", "127.0.0.1"); value < 1 {
		t.Fatalf("loopback ICMP latency = %d, want at least 1 ms", value)
	}
}
