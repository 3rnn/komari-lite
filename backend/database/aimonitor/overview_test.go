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

func TestTodayTokensAreUnknownWhenCollectedCoverageHasAGap(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	r := validReport(now)
	covered := false
	r.Samples[0].Coverage = &covered
	r.Samples[0].ResetCount = 1
	if _, err := s.Ingest("node", r, now); err != nil {
		t.Fatal(err)
	}
	view, err := s.Overview(Query{Preset: "Today", Timezone: "UTC"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if view.Live.TodayCoverage != "unknown" {
		t.Fatalf("today coverage = %q, want unknown", view.Live.TodayCoverage)
	}
	if view.Live.TodayTokens != 160 {
		t.Fatalf("uncovered collected tokens = %d, want retained partial value", view.Live.TodayTokens)
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

func TestOverviewPresetUsesServerAuthoritativeThirtyDayBoundary(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0).UTC()
	for preset, duration := range map[string]time.Duration{"24H": 24 * time.Hour, "7D": 7 * 24 * time.Hour, "30D": 30 * 24 * time.Hour} {
		view, err := s.Overview(Query{Preset: preset, From: float64(now.Add(-duration - time.Minute).Unix()), To: float64(now.Add(-time.Minute).Unix()), Timezone: "UTC"}, now)
		if err != nil {
			t.Fatalf("server-authoritative %s preset rejected after client delay: %v", preset, err)
		}
		if view.Range.From != float64(now.Add(-duration).Unix()) || view.Range.To != float64(now.Unix()) {
			t.Fatalf("%s preset range = %#v, want exact server trailing duration", preset, view.Range)
		}
	}
	if _, err := s.Overview(Query{From: float64(now.Add(-30*24*time.Hour - time.Second).Unix()), To: float64(now.Unix()), Timezone: "UTC"}, now); err == nil {
		t.Fatal("custom range beyond strict retention was accepted")
	}
	if _, err := s.Overview(Query{From: float64(now.Add(-time.Hour).Unix()), To: float64(now.Add(121 * time.Second).Unix()), Timezone: "UTC"}, now); err == nil {
		t.Fatal("custom range beyond allowed future skew was accepted")
	}
}

func TestOverviewTodayUsesSelectedTimezoneCalendarBoundary(t *testing.T) {
	s := fixture(t)
	for _, tc := range []struct {
		name, timezone string
		now            time.Time
		wantFrom       time.Time
	}{
		{"utc", "UTC", time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC), time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)},
		{"utc-midnight", "UTC", time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)},
		{"shanghai", "Asia/Shanghai", time.Date(2026, 3, 8, 1, 0, 0, 0, time.UTC), time.Date(2026, 3, 7, 16, 0, 0, 0, time.UTC)},
		{"dst", "America/New_York", time.Date(2026, 3, 8, 16, 0, 0, 0, time.UTC), time.Date(2026, 3, 8, 5, 0, 0, 0, time.UTC)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			view, err := s.Overview(Query{Preset: "Today", Timezone: tc.timezone}, tc.now)
			if err != nil {
				t.Fatal(err)
			}
			if view.Range.From != float64(tc.wantFrom.Unix()) || view.Range.To != float64(tc.now.Unix()) {
				t.Fatalf("today range = %#v, want %v..%v", view.Range, tc.wantFrom, tc.now)
			}
		})
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
	view, err := s.Overview(Query{Preset: "24H", From: float64(now.Add(-48 * time.Hour).Unix()), To: float64(now.Add(-time.Hour).Unix()), Timezone: "UTC"}, now)
	if err != nil {
		t.Fatalf("large overview returned %v", err)
	}
	if view.PricingCoverage.Calls != int64(len(rows)) || view.PricingCoverage.TotalTokens() != int64(len(rows))*10 {
		t.Fatalf("large aggregate totals = %+v", view.PricingCoverage)
	}
	if len(view.Models) != 2 || len(view.Tasks) != 2 || len(view.Trends) == 0 || len(view.Daily) != 1 {
		t.Fatalf("large aggregate groups = models:%d tasks:%d trends:%d daily:%d", len(view.Models), len(view.Tasks), len(view.Trends), len(view.Daily))
	}
	week, err := s.Overview(Query{Preset: "7D", Timezone: "UTC"}, now)
	if err != nil || week.PricingCoverage.Calls != int64(len(rows)) || len(week.Trends) == 0 {
		t.Fatalf("7-day aggregate = %#v, %v", week.PricingCoverage, err)
	}
	month, err := s.Overview(Query{Preset: "30D", Timezone: "Asia/Shanghai"}, now)
	if err != nil || month.PricingCoverage.Calls != int64(len(rows)) || len(month.Trends) == 0 {
		t.Fatalf("30-day aggregate = %#v, %v", month.PricingCoverage, err)
	}
	today, err := s.Overview(Query{Preset: "Today", Timezone: "UTC"}, now)
	if err != nil || today.PricingCoverage.Calls != int64(len(rows)) || today.Live.TodayTokens != int64(len(rows))*10 {
		t.Fatalf("today aggregate = %#v, %v", today.PricingCoverage, err)
	}
	t.Logf("aggregated %d samples in %s", len(rows), time.Since(started))
}
