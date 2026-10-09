package dbcore

import (
	"github.com/komari-monitor/komari/database/aimonitor"
	"github.com/komari-monitor/komari/pkg/migrations"
	"testing"
)

func TestStartupCreatesIndependentAdditiveAITables(t *testing.T) {
	isolateMainStartup(t)
	if err := doInitialize(); err != nil {
		t.Fatal(err)
	}
	for _, model := range []any{&aimonitor.StoredSample{}, &aimonitor.PricingRule{}, &aimonitor.Rollup{}, &aimonitor.Dedup{}, &aimonitor.NodeState{}, &aimonitor.AccountQuota{}, &aimonitor.SettingsRow{}} {
		if !instance.Migrator().HasTable(model) {
			t.Fatalf("AI table not migrated: %T", model)
		}
	}
	version, err := migrations.SchemaVersion(instance)
	if err != nil || version != migrations.ExpectedSchemaVersion {
		t.Fatal("AI migration changed main schema revision")
	}
}
