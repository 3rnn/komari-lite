package cmd

import (
	"os"
	"path/filepath"
	"testing"

	pkg_flags "github.com/nuomiiiii/lite-agent/cmd/flags"
	"github.com/spf13/cobra"
)

func validRuntimeConfig() *pkg_flags.Config {
	return &pkg_flags.Config{
		Interval:           3,
		ReconnectInterval:  5,
		InfoReportInterval: 5,
		MaxRetries:         3,
		ProtocolVersion:    2,
		Endpoint:           "https://panel.example",
	}
}

func TestValidateRuntimeConfig(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*pkg_flags.Config)
		valid  bool
	}{
		{name: "defaults", valid: true},
		{name: "zero interval", mutate: func(c *pkg_flags.Config) { c.Interval = 0 }},
		{name: "zero reconnect interval", mutate: func(c *pkg_flags.Config) { c.ReconnectInterval = 0 }},
		{name: "zero info interval", mutate: func(c *pkg_flags.Config) { c.InfoReportInterval = 0 }},
		{name: "negative retries", mutate: func(c *pkg_flags.Config) { c.MaxRetries = -1 }},
		{name: "invalid month day", mutate: func(c *pkg_flags.Config) { c.MonthRotate = 32 }},
		{name: "invalid month time", mutate: func(c *pkg_flags.Config) { c.MonthRotateTime = "25:00:00" }},
		{name: "month time ok", mutate: func(c *pkg_flags.Config) { c.MonthRotateTime = "00:00:00" }, valid: true},
		{name: "invalid timezone", mutate: func(c *pkg_flags.Config) { c.MonthRotateTimezone = "Not/AZone" }},
		{name: "shanghai timezone", mutate: func(c *pkg_flags.Config) { c.MonthRotateTimezone = "Asia/Shanghai" }, valid: true},
		{name: "invalid protocol", mutate: func(c *pkg_flags.Config) { c.ProtocolVersion = 3 }},
		{name: "protocol v1 removed", mutate: func(c *pkg_flags.Config) { c.ProtocolVersion = 1 }},
		{name: "protocol 0 means 2", mutate: func(c *pkg_flags.Config) { c.ProtocolVersion = 0 }, valid: true},
		{name: "protocol 2 ok", mutate: func(c *pkg_flags.Config) { c.ProtocolVersion = 2 }, valid: true},
		{name: "ipv4 preferred", mutate: func(c *pkg_flags.Config) { c.PreferIPVersion = "4" }, valid: true},
		{name: "invalid preferred IP", mutate: func(c *pkg_flags.Config) { c.PreferIPVersion = "auto" }},
		{name: "Cloudflare Access credentials", mutate: func(c *pkg_flags.Config) {
			c.CFAccessClientID = "access-id"
			c.CFAccessClientSecret = "access-secret"
		}, valid: true},
		{name: "Cloudflare Access client ID only", mutate: func(c *pkg_flags.Config) { c.CFAccessClientID = "access-id" }},
		{name: "Cloudflare Access client secret only", mutate: func(c *pkg_flags.Config) { c.CFAccessClientSecret = "access-secret" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := validRuntimeConfig()
			if tt.mutate != nil {
				tt.mutate(config)
			}
			err := validateRuntimeConfig(config)
			if tt.valid && err != nil {
				t.Fatalf("validateRuntimeConfig() error = %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatal("validateRuntimeConfig() expected an error")
			}
		})
	}
}

func TestLoadEffectiveConfigPrecedence(t *testing.T) {
	tests := []struct {
		name      string
		fileValue bool
		envValue  string
		args      []string
		want      bool
	}{
		{
			name:      "explicit disable flag overrides config file",
			fileValue: false,
			args:      []string{"--disable-auto-update"},
			want:      true,
		},
		{
			name:      "config file keeps automatic updates enabled",
			fileValue: false,
			want:      false,
		},
		{
			name:      "environment enables automatic updates",
			fileValue: true,
			envValue:  "false",
			want:      false,
		},
		{
			name:      "explicit false flag overrides environment and config",
			fileValue: true,
			envValue:  "true",
			args:      []string{"--disable-auto-update=false"},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "agent.json")
			contents := []byte(`{"disable_auto_update":false}`)
			if tt.fileValue {
				contents = []byte(`{"disable_auto_update":true}`)
			}
			if err := os.WriteFile(configPath, contents, 0o600); err != nil {
				t.Fatal(err)
			}

			t.Setenv("AGENT_CONFIG_FILE", "")
			t.Setenv("AGENT_DISABLE_AUTO_UPDATE", tt.envValue)
			config := &pkg_flags.Config{ConfigFile: configPath}
			command := &cobra.Command{Use: "test"}
			command.Flags().BoolVar(&config.DisableAutoUpdate, "disable-auto-update", false, "")
			command.Flags().StringVar(&config.ConfigFile, "config", configPath, "")
			if err := command.ParseFlags(tt.args); err != nil {
				t.Fatal(err)
			}
			if err := loadEffectiveConfig(command, config); err != nil {
				t.Fatal(err)
			}
			if config.DisableAutoUpdate != tt.want {
				t.Fatalf("DisableAutoUpdate = %v, want %v", config.DisableAutoUpdate, tt.want)
			}
		})
	}
}
