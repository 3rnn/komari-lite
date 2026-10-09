package clients

import (
	"encoding/json"
	"github.com/komari-monitor/komari/database/models"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

func TestHermesAIExplicitOptInAndLegacyAcknowledgmentGuard(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "profiles.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err = db.AutoMigrate(&models.Client{}, &models.ClientDeploymentProfile{}); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.Client{UUID: "ai-node", Token: "fixture-token"}).Error; err != nil {
		t.Fatal(err)
	}
	defaults := defaultDeploymentProfile(models.Client{})
	if defaults.EnableHermesAI {
		t.Fatal("AI silently enabled")
	}
	profile := DeploymentProfile{Platform: "linux", EnableHermesAI: true}
	_, state, _, err := saveDeploymentProfileForDispatch(db, "ai-node", profile)
	if err != nil {
		t.Fatal(err)
	}
	if value := profile.RuntimeConfig().EnableHermesAI; value == nil || !*value {
		t.Fatal("runtime config missing explicit enablement")
	}
	applied, err := completeDeploymentConfig(db, "ai-node", v2.ConfigResultParams{Revision: state.Revision, Status: "applied"})
	if err != nil || applied {
		t.Fatalf("old Agent falsely acknowledged AI: %v %v", applied, err)
	}
	// Old runtime state must not acknowledge enablement by omission.
	if _, err = adoptDeploymentRuntimeConfig(db, "ai-node", "linux", v2.ConfigParams{}); err != nil {
		t.Fatal(err)
	}
	var row models.ClientDeploymentProfile
	db.First(&row, "client = ?", "ai-node")
	if row.DeliveryStatus == "applied" {
		t.Fatal("legacy runtime state falsely applied AI")
	}
	enabled := true
	applied, err = completeDeploymentConfig(db, "ai-node", v2.ConfigResultParams{Revision: state.Revision, Status: "applied", EnableHermesAI: &enabled})
	if err != nil || !applied {
		t.Fatalf("capable Agent could not acknowledge AI: %v %v", applied, err)
	}
	var got DeploymentProfile
	json.Unmarshal([]byte(row.Config), &got)
	if !got.EnableHermesAI {
		t.Fatal("managed profile overwritten")
	}
}
