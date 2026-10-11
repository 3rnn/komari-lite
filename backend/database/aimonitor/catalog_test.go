package aimonitor

import (
	"encoding/json"
	"testing"
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
	var catalog Catalog
	if err := json.Unmarshal(ReferenceCatalog(), &catalog); err != nil {
		t.Fatal(err)
	}
	if !catalog.AutomaticActivation || len(catalog.Entries) != 1 {
		t.Fatalf("catalog: %+v", catalog)
	}
	entry := catalog.Entries[0]
	if entry.Provider != "openai" || entry.Model != "gpt-5.6-terra" || entry.ProcessingMode != "standard" || entry.ContextTier != "short" || entry.InputPerMillion == nil || *entry.InputPerMillion != "2.00" || entry.CacheReadPerMillion == nil || *entry.CacheReadPerMillion != "0.20" || entry.CacheWritePerMillion == nil || *entry.CacheWritePerMillion != "2.50" || entry.OutputPerMillion == nil || *entry.OutputPerMillion != "12.00" {
		t.Fatalf("unexpected automatic gpt-5.6-terra SKU: %+v", entry)
	}
	var rules []PricingRule
	if err := s.DB.Order("billing_mode").Find(&rules).Error; err != nil || len(rules) != 2 {
		t.Fatalf("automatic defaults: %+v %v", rules, err)
	}
	for _, rule := range rules {
		if rule.Provider != "openai-codex" || rule.Model != "gpt-5.6-terra" || (rule.BillingMode != "subscription_included" && rule.BillingMode != "unknown") || rule.InputPerMillion == nil || *rule.InputPerMillion != 2 || rule.CacheReadPerMillion == nil || *rule.CacheReadPerMillion != .2 || rule.CacheWritePerMillion == nil || *rule.CacheWritePerMillion != 2.5 || rule.OutputPerMillion == nil || *rule.OutputPerMillion != 12 || rule.Source != "official_docs_snapshot" {
			t.Fatalf("unexpected automatic rule: %+v", rule)
		}
	}
	if err := Migrate(s.DB); err != nil {
		t.Fatal(err)
	}
	var count int64
	s.DB.Model(&PricingRule{}).Count(&count)
	if count != 2 {
		t.Fatalf("automatic defaults duplicated: %d", count)
	}
}
