package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestSlimAgentStartupNeverHandsOffToLegacyLiteAgent(t *testing.T) {
	source, err := os.ReadFile("root.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if strings.Contains(text, "relocate.RelocateIfNeeded") {
		t.Fatal("monitoring-only Agent must not invoke legacy layout relocation or hand off to Lite-agent")
	}
	if strings.Contains(text, `"github.com/nuomiiiii/lite-agent/relocate"`) {
		t.Fatal("monitoring-only Agent must not import legacy relocation support")
	}
}
