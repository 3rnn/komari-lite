package aimonitor

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPricingEffectiveTimeAcceptsAdminISO8601(t *testing.T) {
	var rule PricingRule
	raw := []byte(`{"provider":"openai","model":"exact","billing_mode":"api","input_per_million":1,"output_per_million":2,"source":"operator override","version":"v1","processing_mode":"reference","effective_at":"2026-05-28T12:00:00Z"}`)
	if err := DecodeStrict(raw, &rule); err != nil {
		t.Fatalf("frontend timestamp rejected: %v", err)
	}
	if rule.EffectiveAt <= 0 {
		t.Fatal("effective time not parsed")
	}
	encoded, err := json.Marshal(rule)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	json.Unmarshal(encoded, &body)
	if _, ok := body["effective_at"].(string); !ok {
		t.Fatal("pricing effective time must round-trip as RFC3339")
	}
}
func TestCatalogRemainsInformationalUntilExplicitExactSelection(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	var catalog Catalog
	if err := json.Unmarshal(ReferenceCatalog(), &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.AutomaticActivation || len(catalog.Entries) != 28 {
		t.Fatalf("catalog: %+v", catalog)
	}
	var count int64
	s.DB.Model(&PricingRule{}).Count(&count)
	if count != 0 {
		t.Fatal("catalog automatically activated")
	}
	entry := catalog.Entries[0]
	rule, err := s.AppendPricing(PricingRule{Provider: entry.Provider, Model: "local-exact-alias", ReferenceModel: entry.Model, ProcessingMode: entry.ProcessingMode, ReferenceTier: entry.ContextTier, BillingMode: "operator-selected", EffectiveAt: float64(now.Unix())}, now)
	if err != nil || rule.InputPerMillion == nil || rule.Source != "official_docs_snapshot" {
		t.Fatalf("selection: %+v %v", rule, err)
	}
	if _, err := s.AppendPricing(PricingRule{Provider: entry.Provider, Model: "local-exact-alias", ReferenceModel: entry.Model, ProcessingMode: "inferred", ReferenceTier: entry.ContextTier, EffectiveAt: float64(now.Unix())}, now); err == nil {
		t.Fatal("guessed mode accepted")
	}
}
