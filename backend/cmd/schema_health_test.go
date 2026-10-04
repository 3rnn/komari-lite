package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/pkg/migrations"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func runReadOnlyCommand(t *testing.T, name, path string) (map[string]any, error) {
	t.Helper()
	previous := flags.DatabaseFile
	previousType := flags.DatabaseType
	t.Cleanup(func() { flags.DatabaseFile = previous; flags.DatabaseType = previousType })
	flags.DatabaseFile = path
	flags.DatabaseType = "sqlite"
	var out bytes.Buffer
	command := RootCmd
	command.SetOut(&out)
	t.Cleanup(func() { command.SetOut(nil) })
	switch name {
	case "health":
		healthJSON = true
		return decodeCommand(t, &out, HealthCmd.RunE(HealthCmd, nil))
	case "schema-version":
		schemaVersionJSON = true
		return decodeCommand(t, &out, SchemaVersionCmd.RunE(SchemaVersionCmd, nil))
	default:
		t.Fatalf("unexpected command %q", name)
	}
	return nil, nil
}

func decodeCommand(t *testing.T, out *bytes.Buffer, err error) (map[string]any, error) {
	t.Helper()
	if out.Len() == 0 {
		return nil, err
	}
	var value map[string]any
	if jsonErr := json.Unmarshal(out.Bytes(), &value); jsonErr != nil {
		t.Fatalf("invalid JSON %q: %v", out.String(), jsonErr)
	}
	return value, err
}

func TestSchemaVersionJSONDoesNotOpenDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.db")
	value, err := runReadOnlyCommand(t, "schema-version", path)
	if err != nil || value["schema_version"] != float64(migrations.ExpectedSchemaVersion) {
		t.Fatalf("version=%v err=%v", value, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("command created database: %v", err)
	}
}

func TestHealthReadOnlyAndVersioned(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.RunVersioned(db); err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	value, err := runReadOnlyCommand(t, "health", path)
	if err != nil || value["ok"] != true || value["schema_version"] != float64(migrations.ExpectedSchemaVersion) || value["expected_schema_version"] != float64(migrations.ExpectedSchemaVersion) {
		t.Fatalf("health=%v err=%v", value, err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("health mutated database")
	}
}

func TestHealthRejectsMissingCorruptLegacyAndFutureWithoutCreatingFiles(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.db")
	if _, err := runReadOnlyCommand(t, "health", missing); err == nil {
		t.Fatal("missing database accepted")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("missing database created: %v", err)
	}
	corrupt := filepath.Join(dir, "corrupt.db")
	if err := os.WriteFile(corrupt, []byte("not sqlite"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := runReadOnlyCommand(t, "health", corrupt); err == nil {
		t.Fatal("corrupt database accepted")
	}
	legacy := filepath.Join(dir, "legacy.db")
	db, err := gorm.Open(sqlite.Open(legacy), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE configs (key TEXT PRIMARY KEY, value TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()
	if _, err := runReadOnlyCommand(t, "health", legacy); err == nil {
		t.Fatal("unversioned database accepted")
	}
	future := filepath.Join(dir, "future.db")
	db, err = gorm.Open(sqlite.Open(future), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.RunVersioned(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("PRAGMA user_version=99").Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, _ = db.DB()
	_ = sqlDB.Close()
	if _, err := runReadOnlyCommand(t, "health", future); err == nil {
		t.Fatal("future database accepted")
	}
}
