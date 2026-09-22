package admin

import (
	"testing"

	"github.com/komari-monitor/komari/web/public"
)

func TestThemeSettingsAreScopedToFixedGlass(t *testing.T) {
	if public.DefaultTheme != "Glass" {
		t.Fatalf("fixed default theme = %q, want Glass", public.DefaultTheme)
	}
}
