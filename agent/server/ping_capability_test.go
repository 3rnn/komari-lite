package server

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestPrivilegedICMPOptInForCapabilityService(t *testing.T) {
	t.Setenv("KOMARI_AGENT_PRIVILEGED_ICMP", "1")
	if !shouldUsePrivilegedICMP(65534) {
		t.Fatal("CAP_NET_RAW service must use raw ICMP when explicitly configured")
	}
	t.Setenv("KOMARI_AGENT_PRIVILEGED_ICMP", "")
	if shouldUsePrivilegedICMP(65534) {
		t.Fatal("ordinary non-root launch must keep UDP ICMP")
	}
	if !shouldUsePrivilegedICMP(0) {
		t.Fatal("root compatibility")
	}
}

// Can also run as an unprivileged account with only ambient CAP_NET_RAW.
func TestCapabilityServiceICMPLoopback(t *testing.T) {
	if os.Getenv("KOMARI_TEST_CAPABILITY_SERVICE") != "1" {
		t.Skip("explicit isolated capability check")
	}
	if os.Geteuid() == 0 {
		t.Fatal("capability fixture must not be root")
	}
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"CapEff:\t0000000000002000", "CapBnd:\t0000000000002000", "NoNewPrivs:\t1"} {
		if !strings.Contains(string(status), line) {
			t.Fatalf("capability sandbox missing %s", line)
		}
	}
	if value := probeLatency(context.Background(), "icmp", "127.0.0.1"); value < 1 {
		t.Fatalf("restricted raw ICMP latency=%d", value)
	}
}
