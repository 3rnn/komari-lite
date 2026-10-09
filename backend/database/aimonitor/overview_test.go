package aimonitor

import (
	"fmt"
	"testing"
	"time"
)

func TestOverviewUsesCollectedDeltasAndConservativeRates(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	r := validReport(now)
	if _, err := s.Ingest("node", r, now); err != nil {
		t.Fatal(err)
	}
	view, err := s.Overview(Query{From: float64(now.Unix() - 3600), To: float64(now.Unix()), Timezone: "Asia/Shanghai"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Models) != 1 || view.Models[0].TotalTokens() != 160 || view.Models[0].UnpricedCalls != 2 || len(view.Daily) != 1 || len(view.Tasks) != 1 {
		t.Fatalf("wrong totals: %+v", view)
	}
	if view.Live.TPS != nil {
		t.Fatal("unsupported coverage invented rate")
	}
	if view.Live.TodayTokens != 160 {
		t.Fatalf("today tokens = %d, want 160", view.Live.TodayTokens)
	}
	// A single covered interval is not a whole-minute coverage proof.
	covered := true
	r.Samples[0].Coverage = &covered
	r.Samples[0].SampleID = "550e8400-e29b-41d4-a716-446655440002"
	r.Samples[0].StartedAt = float64(now.Unix() - 60)
	if _, err := s.Ingest("node", r, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	view, err = s.Overview(Query{From: float64(now.Unix() - 3600), To: float64(now.Unix()), Timezone: "UTC"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if view.Live.TPS != nil {
		t.Fatal("overlapping uncovered sample must invalidate rates")
	}
}
func TestCoveredRangeSupportsSampledAggregateAverages(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	r := validReport(now)
	covered := true
	r.Samples[0].Coverage = &covered
	r.Samples[0].StartedAt = float64(now.Unix() - 60)
	if _, err := s.Ingest("node", r, now); err != nil {
		t.Fatal(err)
	}
	view, err := s.Overview(Query{From: float64(now.Unix() - 60), To: float64(now.Unix()), Timezone: "UTC"}, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, rate := range []*float64{view.Models[0].AverageTPS, view.Tasks[0].AverageTPS, view.Daily[0].AverageTPS} {
		if rate == nil || *rate != 160.0/60 {
			t.Fatalf("supported sampled average omitted: %v", rate)
		}
	}
}
func TestOverviewCompleteMinuteAndModelFilters(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	r := validReport(now)
	covered := true
	r.Samples[0].Coverage = &covered
	r.Samples[0].StartedAt = float64(now.Unix() - 60)
	if _, err := s.Ingest("node", r, now); err != nil {
		t.Fatal(err)
	}
	view, err := s.Overview(Query{From: float64(now.Unix() - 3600), To: float64(now.Unix()), Timezone: "UTC"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if view.Live.TPS != nil || view.Live.TPM == nil || *view.Live.TPM != 160 {
		t.Fatalf("minute counters must not interpolate a ten-second rate: %+v", view.Live)
	}
	view, err = s.Overview(Query{From: float64(now.Unix() - 3600), To: float64(now.Unix()), Timezone: "UTC", Model: "unknown"}, now)
	if err != nil || len(view.Models) != 0 {
		t.Fatalf("filter: %v %+v", err, view.Models)
	}
	if _, err = s.Overview(Query{From: 0, To: float64(now.Unix()), Timezone: "UTC"}, now); err == nil {
		t.Fatal("unbounded query accepted")
	}
}

func TestOverviewAggregatesMoreThanMaxRawSamplesInDatabase(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0).UTC()
	rows := make([]StoredSample, 0, MaxQueryRows*2+1)
	priced := 0.0
	for i := 0; i < MaxQueryRows*2+1; i++ {
		ended := float64(now.Unix()) - float64(i%86400)/2
		row := StoredSample{
			Node:   []string{"node-a", "node-b"}[i%2],
			Sample: Sample{SampleID: fmt.Sprintf("sample-%d", i), StartedAt: ended - .5, EndedAt: ended, Model: []string{"model-a", "model-b"}[i%2], Provider: "fixture", BillingMode: "mode", Task: []string{"task-a", "task-b"}[i%2], Calls: 1, InputTokens: 7, OutputTokens: 3},
		}
		if i%2 == 0 {
			row.APIEquivalentCostUSD = &priced
		}
		rows = append(rows, row)
	}
	if err := s.DB.CreateInBatches(rows, 1000).Error; err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	view, err := s.Overview(Query{From: float64(now.Add(-24 * time.Hour).Unix()), To: float64(now.Unix()), Timezone: "UTC"}, now)
	if err != nil {
		t.Fatalf("large overview returned %v", err)
	}
	if view.PricingCoverage.Calls != int64(len(rows)) || view.PricingCoverage.TotalTokens() != int64(len(rows))*10 {
		t.Fatalf("large aggregate totals = %+v", view.PricingCoverage)
	}
	if len(view.Models) != 2 || len(view.Tasks) != 2 || len(view.Trends) == 0 || len(view.Daily) != 1 {
		t.Fatalf("large aggregate groups = models:%d tasks:%d trends:%d daily:%d", len(view.Models), len(view.Tasks), len(view.Trends), len(view.Daily))
	}
	month, err := s.Overview(Query{From: float64(now.Add(-30 * 24 * time.Hour).Unix()), To: float64(now.Unix()), Timezone: "Asia/Shanghai"}, now)
	if err != nil || month.PricingCoverage.Calls != int64(len(rows)) || len(month.Trends) == 0 {
		t.Fatalf("30-day aggregate = %#v, %v", month.PricingCoverage, err)
	}
	t.Logf("aggregated %d samples in %s", len(rows), time.Since(started))
}
