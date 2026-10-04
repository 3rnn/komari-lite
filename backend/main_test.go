package main

import (
	"encoding/json"
	"os/exec"
	"testing"
)

func TestSchemaVersionCLIStdoutIsOnlyJSON(t *testing.T) {
	output, err := exec.Command("go", "run", ".", "schema-version", "--json").Output()
	if err != nil {
		t.Fatalf("run schema-version: %v", err)
	}
	var probe map[string]int
	if err := json.Unmarshal(output, &probe); err != nil {
		t.Fatalf("stdout is not clean JSON: %q: %v", output, err)
	}
	if probe["schema_version"] != 1 {
		t.Fatalf("schema version=%v", probe)
	}
}
