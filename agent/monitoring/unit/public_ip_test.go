package monitoring

import (
	"fmt"
	"net"
	"testing"
)

func TestBuildPublicIPAddressesBoundsInventoryWithoutDroppingPrimary(t *testing.T) {
	var interfaces []interfaceIPAddress
	for i := 1; i <= 1100; i++ {
		interfaces = append(interfaces, interfaceIPAddress{Address: fmt.Sprintf("2001:4860::%x", i), Interface: "eth0"})
	}
	got := buildPublicIPAddresses("8.8.8.8", "", interfaces)
	if len(got) != maxPublicIPAddresses || got[0].Address != "8.8.8.8" || !got[0].Primary {
		t.Fatalf("bounded inventory lost primary or exceeded backend limit: len=%d", len(got))
	}
}

func TestBuildPublicIPAddressesDoesNotDuplicateMismatchedLegacyFamily(t *testing.T) {
	got := buildPublicIPAddresses("8.8.8.8", "8.8.8.8", nil)
	if len(got) != 1 || got[0].Family != "ipv4" {
		t.Fatalf("misconfigured legacy family duplicated identity: %+v", got)
	}
}

func TestPublicInventoryScansPublicLoopbackAssignmentsForRoutedNodes(t *testing.T) {
	if !scanAssignedAddresses(net.FlagUp | net.FlagLoopback) {
		t.Fatal("public /32 and /128 addresses can be assigned to an up loopback interface")
	}
	if scanAssignedAddresses(0) {
		t.Fatal("down interfaces should not advertise unusable addresses")
	}
	got := buildPublicIPAddresses("", "", []interfaceIPAddress{{Address: "9.9.9.9", Interface: "lo"}})
	if len(got) != 1 || got[0].Address != "9.9.9.9" || got[0].Interface != "lo" {
		t.Fatalf("routed loopback address missing: %+v", got)
	}
}

func TestBuildPublicIPAddressesKeepsPrimaryAndAllDistinctPublicInterfaceAddresses(t *testing.T) {
	got := buildPublicIPAddresses("8.8.8.8", "2001:4860:4860::8888", []interfaceIPAddress{
		{Address: "9.9.9.9", Interface: "eth1"},
		{Address: "8.8.8.8", Interface: "eth0"},
		{Address: "1.1.1.1", Interface: "eth0"},
		{Address: "1.1.1.1", Interface: "eth1"},
		{Address: "2001:4860:4860::8844", Interface: "eth1"},
		{Address: "2001:4860:4860::8888", Interface: "eth0"},
		{Address: "10.0.0.5", Interface: "eth0"},
		{Address: "100.64.1.2", Interface: "eth0"},
		{Address: "203.0.113.10", Interface: "eth0"},
		{Address: "fe80::abcd", Interface: "eth0"},
		{Address: "fd00::abcd", Interface: "eth0"},
		{Address: "2001:db8::1", Interface: "eth0"},
	})
	want := []PublicIPAddress{
		{Address: "8.8.8.8", Family: "ipv4", Interface: "eth0", Source: "interface", Primary: true},
		{Address: "2001:4860:4860::8888", Family: "ipv6", Interface: "eth0", Source: "interface", Primary: true},
		{Address: "1.1.1.1", Family: "ipv4", Interface: "eth0", Source: "interface"},
		{Address: "9.9.9.9", Family: "ipv4", Interface: "eth1", Source: "interface"},
		{Address: "2001:4860:4860::8844", Family: "ipv6", Interface: "eth1", Source: "interface"},
	}
	if len(got) != len(want) {
		t.Fatalf("addresses = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("address %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestBuildPublicIPAddressesSupportsSingleStackAndEgressOnlyPrimary(t *testing.T) {
	got := buildPublicIPAddresses("", "2001:4860:4860::8888", nil)
	if len(got) != 1 || got[0] != (PublicIPAddress{Address: "2001:4860:4860::8888", Family: "ipv6", Source: "reported", Primary: true}) {
		t.Fatalf("IPv6-only addresses = %+v", got)
	}
	got = buildPublicIPAddresses("8.8.8.8", "", []interfaceIPAddress{{Address: "9.9.9.9", Interface: "eth1"}})
	if len(got) != 2 || got[0].Family != "ipv4" || got[1].Address != "9.9.9.9" {
		t.Fatalf("IPv4-only addresses = %+v", got)
	}
}
