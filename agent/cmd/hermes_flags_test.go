package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestHermesStartupFailureDoesNotStopOrdinaryAgent(t *testing.T) {
	if os.Getenv("KOMARI_TEST_HERMES_STARTUP") == "1" {
		RootCmd.SetArgs([]string{"--token", "fixture", "--endpoint", "https://127.0.0.1:1", "--custom-ipv4", "192.0.2.1", "--custom-ipv6", "2001:db8::1", "--enable-hermes-ai", "--hermes-ai-spool", os.Getenv("KOMARI_TEST_SPOOL")})
		if err := RootCmd.Execute(); err != nil {
			t.Fatal(err)
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHermesStartupFailureDoesNotStopOrdinaryAgent$")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "KOMARI_TEST_HERMES_STARTUP=1", "KOMARI_TEST_SPOOL="+filepath.Join(t.TempDir(), "absent"), "AGENT_CONFIG_FILE=")
	output, err := cmd.CombinedOutput()
	if ctx.Err() == nil {
		t.Fatalf("ordinary Agent exited because optional helper failed: %v\n%s", err, output)
	}
}

func TestHermesOptionalFlagsDefaultDisabledAndPathsStayLocal(t *testing.T) {
	flag := RootCmd.PersistentFlags().Lookup("enable-hermes-ai")
	if flag == nil {
		t.Fatal("missing optional Hermes flag")
	}
	if flag.DefValue != "false" {
		t.Fatal("Hermes must default disabled")
	}
	spool := RootCmd.PersistentFlags().Lookup("hermes-ai-spool")
	if spool == nil {
		t.Fatal("missing operator-local spool flag")
	}
}
