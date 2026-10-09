package aimonitor

import (
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
