package admin

import (
	"strings"
	"testing"

	"github.com/komari-monitor/komari/web/public"
)

func TestThemeSettingsAreScopedToFixedGlass(t *testing.T) {
	if public.DefaultTheme != "Glass" {
		t.Fatalf("fixed default theme = %q, want Glass", public.DefaultTheme)
	}
}

func TestThemePingSettingsValidateToggleAndPreferredTask(t *testing.T) {
	tests := []struct {
		name  string
		data  map[string]any
		valid bool
	}{
		{"legacy settings", map[string]any{"showCarrierPing": false}, true},
		{"automatic selection", map[string]any{"showCarrierPing": true, "preferredPingTaskId": float64(0)}, true},
		{"assigned task", map[string]any{"preferredPingTaskId": float64(3)}, true},
		{"invalid toggle", map[string]any{"showCarrierPing": "true"}, false},
		{"nonintegral ID", map[string]any{"preferredPingTaskId": float64(1.5)}, false},
		{"negative ID", map[string]any{"preferredPingTaskId": float64(-1)}, false},
		{"missing task", map[string]any{"preferredPingTaskId": float64(4)}, false},
		{"string ID", map[string]any{"preferredPingTaskId": "3"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePingThemeSettings(tt.data, func(id uint) bool { return id == 3 })
			if (err == nil) != tt.valid {
				t.Fatalf("error=%v, expected valid=%v", err, tt.valid)
			}
			if err != nil && !strings.Contains(err.Error(), "ping") {
				t.Fatalf("unhelpful error: %v", err)
			}
		})
	}
}
