package server

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"testing"

	pkg_flags "github.com/nuomiiiii/lite-agent/cmd/flags"
)

func TestV2PullCapabilitiesExposeMonitoringOnly(t *testing.T) {
	required := []string{"ping", "message", "event", "config"}
	forbidden := []string{"terminal", "remote", "files", "exec", "mcp_full", "route"}

	caps, versions := currentV2PullCapabilities()
	advertised := make(map[string]bool, len(caps))
	for _, capability := range caps {
		advertised[capability] = true
	}

	for _, want := range required {
		if !advertised[want] {
			t.Fatalf("pull capabilities missing %q: %v", want, caps)
		}
	}
	for _, unwanted := range forbidden {
		if advertised[unwanted] {
			t.Fatalf("pull capabilities must not advertise removed capability %q: %v", unwanted, caps)
		}
	}
	if len(versions) != 0 {
		t.Fatalf("pull capability versions = %v, want none", versions)
	}
}

func TestV2PullPayloadKeepsRemovedCapabilitiesAbsent(t *testing.T) {
	original := pkg_flags.GlobalConfig.RemoteControlEnabled
	t.Cleanup(func() { pkg_flags.GlobalConfig.RemoteControlEnabled = original })

	// The legacy flag is still parsed for compatibility with installed nodes, but
	// it must never bring remote file, exec, terminal or MCP capabilities back.
	pkg_flags.GlobalConfig.RemoteControlEnabled = true

	payload := v2PullPayload(nil)
	if !bytes.Contains(payload, []byte(`"method":"agent.pull"`)) {
		t.Fatalf("pull payload missing agent.pull: %s", payload)
	}
	for _, unwanted := range []string{"mcp_full", `"exec"`, `"remote"`, `"files"`, `"terminal"`, `"route"`} {
		if bytes.Contains(payload, []byte(unwanted)) {
			t.Fatalf("pull payload must not advertise %s: %s", unwanted, payload)
		}
	}
}

func TestHandleWebSocketAdvertisesPullAfterConnect(t *testing.T) {
	source, err := os.ReadFile("websocket.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(source, []byte("advertiseV2PullCapabilities(conn)")) {
		t.Fatal("websocket connect must advertise pull capabilities so the panel can record the Agent's monitoring abilities")
	}
	if !bytes.Contains(source, []byte("if message.Method == \"\"")) {
		t.Fatal("websocket read loop must ignore pull RPC responses without a method")
	}
}

func TestV2PullAdvertisesNoRemoteCapability(t *testing.T) {
	original := pkg_flags.GlobalConfig.RemoteControlEnabled
	t.Cleanup(func() { pkg_flags.GlobalConfig.RemoteControlEnabled = original })

	for _, enabled := range []bool{false, true} {
		pkg_flags.GlobalConfig.RemoteControlEnabled = enabled
		caps, versions := currentV2PullCapabilities()
		for _, capability := range caps {
			if capability == "mcp_full" {
				t.Fatalf("remote_control_enabled=%v must not advertise mcp_full: %v", enabled, caps)
			}
		}
		if _, ok := versions["mcp_full"]; ok {
			t.Fatalf("remote_control_enabled=%v must not advertise mcp_full version", enabled)
		}
	}
}

func TestRunV2PullLoopHasNoErrCh(t *testing.T) {
	fn := reflect.TypeOf(runV2PullLoop)
	if fn.NumIn() != 1 {
		t.Fatalf("runV2PullLoop should only take context, got %d params", fn.NumIn())
	}
	if fn.In(0) != reflect.TypeOf((*context.Context)(nil)).Elem() {
		t.Fatalf("runV2PullLoop argument = %s, want context.Context", fn)
	}
}

func TestRemoteWebSocketReadLimitIs2MiB(t *testing.T) {
	if remoteWebSocketReadLimit != 2<<20 {
		t.Fatalf("remote WebSocket read limit = %d, want 2MiB", remoteWebSocketReadLimit)
	}
}
