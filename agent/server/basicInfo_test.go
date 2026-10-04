package server

import (
	"testing"

	monitoring "github.com/nuomiiiii/lite-agent/monitoring/unit"
)

func TestBasicInfoReportsStructuredPublicAddressesWithoutReplacingPrimaryFields(t *testing.T) {
	old4, old6, oldNIC := flags.CustomIpv4, flags.CustomIpv6, flags.GetIpAddrFromNic
	t.Cleanup(func() {
		flags.CustomIpv4, flags.CustomIpv6, flags.GetIpAddrFromNic = old4, old6, oldNIC
	})
	flags.CustomIpv4, flags.CustomIpv6, flags.GetIpAddrFromNic = "8.8.8.8", "2001:4860:4860::8888", false
	data := buildBasicInfoMap()
	if data["ipv4"] != "8.8.8.8" || data["ipv6"] != "2001:4860:4860::8888" {
		t.Fatalf("existing primary fields changed: ipv4=%v ipv6=%v", data["ipv4"], data["ipv6"])
	}
	addresses, ok := data["ip_addresses"].([]monitoring.PublicIPAddress)
	if !ok || len(addresses) < 2 || !addresses[0].Primary || !addresses[1].Primary {
		t.Fatalf("structured public addresses missing primary identities: %#v", data["ip_addresses"])
	}
}

func TestBasicInfoCarriesOnlyFieldsThePanelKnows(t *testing.T) {
	data := buildBasicInfoMap()

	if data["remote_protocol"] != 2 {
		t.Fatalf("remote_protocol = %#v, want 2", data["remote_protocol"])
	}

	// The panel updates the clients table using keys reported by the Agent. An extra
	// unknown column causes a "no such column" error and drops the entire basic-info update.
	// Without remote control, this lite Agent must never report remote_control_enabled.
	for _, forbidden := range []string{
		"remote_control_enabled",
		"remote_control_protected",
		"mcp_full",
		"mcp_full_version",
		"mcp_enabled",
	} {
		if _, ok := data[forbidden]; ok {
			t.Fatalf("basic info must not include %s; the panel has no such column and would reject the whole update", forbidden)
		}
	}
}
