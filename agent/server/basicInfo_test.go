package server

import (
	"testing"
)

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
