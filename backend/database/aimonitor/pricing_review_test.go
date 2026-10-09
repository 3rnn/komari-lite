package aimonitor

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPricingExactSelectorsAndFrozenRollup(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	rate := 1.0
	rule, err := s.AppendPricing(PricingRule{Provider: "openai", Model: "gpt-explicit", BillingMode: "reference", InputPerMillion: &rate, OutputPerMillion: &rate, CacheReadPerMillion: &rate, Source: "explicit test override", Version: "v1", ProcessingMode: "standard", EffectiveAt: float64(now.Unix() - 100)}, now)
	if err != nil {
		t.Fatal(err)
	}
	r := validReport(now)
	for _, change := range []func(*Sample){
		func(s *Sample) { s.Model = "gpt-explicit-alias" },
		func(s *Sample) { s.Provider = "other-provider" },
		func(s *Sample) { s.BillingMode = "other-mode" },
		func(s *Sample) { s.CacheWriteTokens = 1 },
	} {
		sample := r.Samples[0]
		sample.SampleID = uuid.NewString()
		change(&sample)
		r.Samples = append(r.Samples, sample)
	}
	if _, err := s.Ingest("node", r, now); err != nil {
		t.Fatal(err)
	}
	var rows []StoredSample
	if err := s.DB.Order("id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("rows=%d", len(rows))
	}
	if rows[0].APIEquivalentCostUSD == nil || *rows[0].APIEquivalentCostUSD != 0.00016 {
		t.Fatalf("overlapping categories priced incorrectly: %+v", rows[0])
	}
	for _, row := range rows[1:] {
		if row.APIEquivalentCostUSD != nil {
			t.Fatalf("ambiguous/unsupported usage priced: %+v", row)
		}
	}
	if err := s.Maintain(now.Add(25 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	var roll Rollup
	if err := s.DB.Where("price_id = ? AND model = ? AND billing_mode = ?", rule.ID, rule.Model, rule.BillingMode).First(&roll).Error; err != nil {
		t.Fatal(err)
	}
	var snapshot PricingRule
	if err := json.Unmarshal([]byte(roll.PriceSnapshot), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Version != "v1" || snapshot.Source != rule.Source || roll.APIEquivalentCostUSD == nil || *roll.APIEquivalentCostUSD != 0.00016 || roll.UnpricedCalls != 2 {
		t.Fatalf("frozen price/cost/coverage lost during compaction: %+v", roll)
	}
}
