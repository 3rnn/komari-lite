package cmd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/pkg/migrations"
	"github.com/komari-monitor/komari/utils"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
)

var schemaVersionJSON bool
var healthJSON bool

var SchemaVersionCmd = &cobra.Command{
	Use: "schema-version", Short: "Print the expected main SQLite schema version", Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if schemaVersionJSON {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]int{"schema_version": migrations.ExpectedSchemaVersion})
		}
		_, err := fmt.Fprintln(cmd.OutOrStdout(), migrations.ExpectedSchemaVersion)
		return err
	},
}

var HealthCmd = &cobra.Command{
	Use: "health", Short: "Check the main SQLite database without modifying it", Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		version, err := inspectMainDatabase(flags.DatabaseFile)
		result := struct {
			OK                    bool   `json:"ok"`
			SchemaVersion         int    `json:"schema_version"`
			ExpectedSchemaVersion int    `json:"expected_schema_version"`
			Version               string `json:"version"`
			Hash                  string `json:"hash"`
		}{err == nil, version, migrations.ExpectedSchemaVersion, utils.CurrentVersion, utils.VersionHash}
		if healthJSON {
			if encodeErr := json.NewEncoder(cmd.OutOrStdout()).Encode(result); encodeErr != nil {
				return encodeErr
			}
		} else if err == nil {
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "healthy (main schema %d)\n", version)
		}
		return err
	},
}

// inspectMainDatabase does not call Initialize or run migrations. Opening in
// SQLite read-only mode and checking the on-disk header prevents accidentally
// creating an empty database when the configured path is missing.
func inspectMainDatabase(path string) (int, error) {
	if flags.NormalizeDatabaseType(flags.DatabaseType) != flags.DatabaseTypeSQLite {
		return 0, fmt.Errorf("unsupported database type %q", flags.DatabaseType)
	}
	if path == "" {
		path = "./data/komari.db"
	}
	if strings.HasPrefix(path, "file:") || path == ":memory:" || strings.ContainsAny(path, "?\x00") {
		return 0, fmt.Errorf("health requires a SQLite database file path")
	}
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open main database: %w", err)
	}
	header := make([]byte, 16)
	_, err = io.ReadFull(file, header)
	closeErr := file.Close()
	if err != nil {
		return 0, fmt.Errorf("read main database header: %w", err)
	}
	if closeErr != nil {
		return 0, closeErr
	}
	if string(header) != "SQLite format 3\x00" {
		return 0, fmt.Errorf("invalid SQLite database header")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return 0, err
	}
	uri := url.URL{Scheme: "file", Path: abs}
	dsn := uri.String() + "?mode=ro&_query_only=1&_busy_timeout=5000"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return 0, fmt.Errorf("open SQLite read-only: %w", err)
	}
	defer db.Close()
	var integrity string
	if err := db.QueryRow("PRAGMA quick_check").Scan(&integrity); err != nil {
		return 0, fmt.Errorf("SQLite integrity check: %w", err)
	}
	if integrity != "ok" {
		return 0, fmt.Errorf("SQLite integrity check: %s", integrity)
	}
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("read main schema version: %w", err)
	}
	if version != migrations.ExpectedSchemaVersion {
		return version, fmt.Errorf("main schema version %d is incompatible with expected %d", version, migrations.ExpectedSchemaVersion)
	}
	for _, table := range []string{"configs", "users", "clients", "sessions"} {
		var found string
		if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&found); err != nil {
			return version, fmt.Errorf("required main table %s unavailable: %w", table, err)
		}
	}
	return version, nil
}

func init() {
	SchemaVersionCmd.Flags().BoolVar(&schemaVersionJSON, "json", false, "print machine-readable JSON")
	HealthCmd.Flags().BoolVar(&healthJSON, "json", false, "print machine-readable JSON")
	RootCmd.AddCommand(SchemaVersionCmd, HealthCmd)
}
