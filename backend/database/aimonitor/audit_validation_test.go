package aimonitor

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestAuditInvalidWritesLeavePricingAndSettingsUntouched(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	before, _ := s.ListPricing()
	settings, _ := s.Settings()
	for _, value := range []float64{-1, math.NaN(), math.Inf(1)} {
		good := 1.0
		rule := PricingRule{Model: "fixture", Provider: "fixture", BillingMode: "api", Source: "operator supplied", Version: "manual", ProcessingMode: "manual", EffectiveAt: float64(now.Unix()), InputPerMillion: &value, OutputPerMillion: &good}
		if _, err := s.AppendPricing(rule, now); err == nil {
			t.Fatalf("accepted rate %v", value)
		}
	}
	invalid := settings
	invalid.CustomModels = []string{"bad model /"}
	if err := s.SetSettings(invalid); err == nil {
		t.Fatal("accepted invalid identifier")
	}
	after, _ := s.ListPricing()
	saved, _ := s.Settings()
	if len(before) != len(after) || len(saved.CustomModels) != len(settings.CustomModels) {
		t.Fatal("rejected writes changed persisted data")
	}
}
func TestOverviewCancellationReleasesSnapshot(t *testing.T) {
	s := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewStore(s.DB.WithContext(ctx)).Overview(Query{Preset: "24H", Timezone: "UTC"}, time.Now()); err == nil {
		t.Fatal("cancelled overview succeeded")
	}
	if _, err := s.Overview(Query{Preset: "24H", Timezone: "UTC"}, time.Now()); err != nil {
		t.Fatal(err)
	}
}
