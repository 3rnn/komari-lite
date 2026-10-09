package server

import (
	"encoding/json"
	"testing"

	v2 "github.com/nuomiiiii/lite-agent/protocol/v2"
	"github.com/nuomiiiii/lite-agent/runtimeconfig"
)

func TestHermesSuccessConfigResultIncludesExplicitAppliedBoolean(t *testing.T) {
	previous := runtimeconfig.Snapshot()
	defer runtimeconfig.Initialize(previous)
	oldSpool := flags.HermesAISpool
	defer func() { flags.HermesAISpool = oldSpool }()
	flags.HermesAISpool = t.TempDir()
	pendingConfigResultMu.Lock()
	oldPending := pendingConfigResult
	pendingConfigResultMu.Unlock()
	defer func() { pendingConfigResultMu.Lock(); pendingConfigResult = oldPending; pendingConfigResultMu.Unlock() }()
	oldProcessed := processedConfigRevision.Load()
	oldApplied := appliedConfigRevision.Load()
	defer processedConfigRevision.Store(oldProcessed)
	defer appliedConfigRevision.Store(oldApplied)
	var config v2.ConfigParams
	json.Unmarshal([]byte(`{"revision":1000000,"enable_hermes_ai":false}`), &config)
	processRuntimeConfig(config, "fixture")
	b, _ := json.Marshal(snapshotPendingConfigResult())
	var result map[string]interface{}
	json.Unmarshal(b, &result)
	if result["status"] != "applied" || result["enable_hermes_ai"] != false {
		t.Fatal("missing explicit applied capability", string(b))
	}
}

func TestHermesRuntimeRejectsUnprovisionedAndPreservesHeartbeatState(t *testing.T) {
	previous := runtimeconfig.Snapshot()
	defer runtimeconfig.Initialize(previous)
	oldSpool := flags.HermesAISpool
	defer func() { flags.HermesAISpool = oldSpool }()
	flags.HermesAISpool = t.TempDir()
	var config v2.ConfigParams
	json.Unmarshal([]byte(`{"enable_hermes_ai":true,"interval":17}`), &config)
	if _, err := applyRuntimeConfig(config); err == nil {
		t.Fatal("true must fail when helper unprovisioned")
	}
	if runtimeconfig.Snapshot() != previous {
		t.Fatal("failure changed ordinary metrics")
	}
	result, _ := json.Marshal(currentRuntimeConfigParams())
	var state map[string]interface{}
	json.Unmarshal(result, &state)
	if state["enable_hermes_ai"] != false {
		t.Fatal("new Agent must explicitly advertise disabled", string(result))
	}
	json.Unmarshal([]byte(`{"enable_hermes_ai":false}`), &config)
	if _, err := applyRuntimeConfig(config); err != nil {
		t.Fatal(err)
	}
}
