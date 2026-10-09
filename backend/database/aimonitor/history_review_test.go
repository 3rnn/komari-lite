package aimonitor

import (
	"testing"
	"time"
)

func TestLiveDayTotalsSurviveEarlyDetailCompaction(t *testing.T) {
	s := fixture(t)
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	if err := s.SetSettings(Settings{Timezone: "UTC", RetentionDays: 30, DetailedRetentionHours: 1}); err != nil {
		t.Fatal(err)
	}
	collected := now.Add(-3 * time.Hour)
	r := validReport(collected)
	rate := 1.0
	_, err := s.AppendPricing(PricingRule{Provider: "openai", Model: "gpt-explicit", BillingMode: "reference", InputPerMillion: &rate, OutputPerMillion: &rate, CacheReadPerMillion: &rate, Source: "test override", Version: "v1", ProcessingMode: "explicit", EffectiveAt: float64(collected.Add(-time.Hour).Unix())}, collected)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Ingest("node", r, collected); err != nil {
		t.Fatal(err)
	}
	q := Query{From: float64(now.Add(-24 * time.Hour).Unix()), To: float64(now.Unix()), Timezone: "UTC"}
	before, err := s.Overview(q, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Maintain(now); err != nil {
		t.Fatal(err)
	}
	after, err := s.Overview(q, now)
	if err != nil {
		t.Fatal(err)
	}
	if after.Live.Calls24h != before.Live.Calls24h || after.Live.TodayAPIEquivalentCostUSD == nil || *after.Live.TodayAPIEquivalentCostUSD != *before.Live.TodayAPIEquivalentCostUSD {
		t.Fatalf("compaction changed day totals: before=%+v after=%+v", before.Live, after.Live)
	}
}
