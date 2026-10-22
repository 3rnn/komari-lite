package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestPingProxyPolicyKeepsLegacyDefault(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://monitoring.example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("KOMARI_AGENT_PING_ALLOW_CIDRS", "")
	expected, err := http.ProxyFromEnvironment(req)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pingProxy(req)
	if err != nil {
		t.Fatal(err)
	}
	if (got == nil) != (expected == nil) || (got != nil && got.String() != expected.String()) {
		t.Fatal("default proxy behavior changed")
	}
	t.Setenv("KOMARI_AGENT_PING_ALLOW_CIDRS", "192.0.2.0/24")
	got, err = pingProxy(req)
	if err != nil || got != nil {
		t.Fatal("configured CIDR policy must not be bypassed through a proxy")
	}
}

func TestPingLocalEgressAllowlist(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(204) }))
	defer srv.Close()
	t.Setenv("KOMARI_AGENT_PING_ALLOW_CIDRS", "192.0.2.0/24")
	if got := probeLatency(context.Background(), "http", srv.URL); got != -1 {
		t.Fatalf("denied local probe=%d", got)
	}
	if hits.Load() != 0 {
		t.Fatal("policy-denied destination was contacted")
	}
	t.Setenv("KOMARI_AGENT_PING_ALLOW_CIDRS", "127.0.0.0/8,::1/128")
	if got := probeLatency(context.Background(), "http", srv.URL); got < 1 {
		t.Fatalf("explicit local monitoring=%d", got)
	}
	t.Setenv("KOMARI_AGENT_PING_ALLOW_CIDRS", "invalid")
	if got := probeLatency(context.Background(), "http", srv.URL); got != -1 {
		t.Fatal("invalid allowlist must fail closed")
	}
	t.Setenv("KOMARI_AGENT_PING_ALLOW_CIDRS", "")
	if got := probeLatency(context.Background(), "http", srv.URL); got < 1 {
		t.Fatal("default must retain private monitoring")
	}
}
