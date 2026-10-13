package aimonitor

import (
	"encoding/json"
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

func TestDailyModelCardKeepsCanonicalTokensAndTruthfulPriceAndOutputRate(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0).UTC()
	covered, priced := true, 0.01
	rows := []StoredSample{
		{Node: "node", Sample: Sample{SampleID: "550e8400-e29b-41d4-a716-446655440301", StartedAt: float64(now.Add(-120 * time.Second).Unix()), EndedAt: float64(now.Add(-60 * time.Second).Unix()), Model: "compact", Provider: "fixture", BillingMode: "metered", Task: "chat", Calls: 1, InputTokens: 10, CacheReadTokens: 5, CacheWriteTokens: 5, OutputTokens: 40, ReasoningTokens: 40, Coverage: &covered}, APIEquivalentCostUSD: &priced},
		{Node: "node", Sample: Sample{SampleID: "550e8400-e29b-41d4-a716-446655440302", StartedAt: float64(now.Add(-60 * time.Second).Unix()), EndedAt: float64(now.Unix()), Model: "compact", Provider: "fixture", BillingMode: "metered", Task: "chat", Calls: 1, InputTokens: 10, CacheReadTokens: 5, CacheWriteTokens: 5, OutputTokens: 120, ReasoningTokens: 120, Coverage: &covered}},
	}
	if err := s.DB.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	view, err := s.Overview(Query{From: float64(now.Add(-120 * time.Second).Unix()), To: float64(now.Unix()), Timezone: "UTC"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Daily) != 1 || len(view.Daily[0].Models) != 1 {
		t.Fatalf("daily model groups = %+v", view.Daily)
	}
	model := view.Daily[0].Models[0]
	if model.TotalTokens() != 200 || model.ReasoningTokens != 160 {
		t.Fatalf("reasoning must remain an output subset, model = %+v", model)
	}
	if model.APIEquivalentCostUSD == nil || *model.APIEquivalentCostUSD != priced || model.UnpricedCalls != 1 || model.PricedCalls != 1 {
		t.Fatalf("price coverage must retain exact priced and unpriced calls, model = %+v", model)
	}
	if model.OutputTPS == nil || *model.OutputTPS != 160.0/120 {
		t.Fatalf("output rate = %v, want %v", model.OutputTPS, 160.0/120)
	}
}

func TestDailyModelCardLeavesOutputRateUnavailableWithoutCoverageProof(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0).UTC()
	row := StoredSample{Node: "node", Sample: Sample{SampleID: "550e8400-e29b-41d4-a716-446655440303", StartedAt: float64(now.Add(-60 * time.Second).Unix()), EndedAt: float64(now.Unix()), Model: "compact", Provider: "fixture", BillingMode: "metered", Task: "chat", Calls: 1, OutputTokens: 60}}
	if err := s.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	view, err := s.Overview(Query{From: float64(now.Add(-60 * time.Second).Unix()), To: float64(now.Unix()), Timezone: "UTC"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if got := view.Daily[0].Models[0].OutputTPS; got != nil {
		t.Fatalf("output rate without coverage proof = %v, want nil", *got)
	}
}

func TestDailyModelCardUsesObservedIntervalsForOutputRate(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0).UTC()
	covered := true
	rows := []StoredSample{
		{Node: "node", Sample: Sample{SampleID: "550e8400-e29b-41d4-a716-446655440304", StartedAt: float64(now.Add(-120 * time.Second).Unix()), EndedAt: float64(now.Add(-110 * time.Second).Unix()), Model: "compact", Provider: "fixture", BillingMode: "metered", Task: "chat", Calls: 1, OutputTokens: 100, Coverage: &covered}},
		{Node: "node", Sample: Sample{SampleID: "550e8400-e29b-41d4-a716-446655440305", StartedAt: float64(now.Add(-10 * time.Second).Unix()), EndedAt: float64(now.Unix()), Model: "compact", Provider: "fixture", BillingMode: "metered", Task: "chat", Calls: 1, OutputTokens: 200, Coverage: &covered}},
	}
	if err := s.DB.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	view, err := s.Overview(Query{From: float64(now.Add(-120 * time.Second).Unix()), To: float64(now.Unix()), Timezone: "UTC"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if got := view.Daily[0].Models[0].OutputTPS; got == nil || *got != 15 {
		t.Fatalf("observed output rate = %v, want 15", got)
	}
}

func TestOverviewRangeUsageUsesDatabaseAggregatesAndLabelsCoverage(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0).UTC()
	covered := true
	rows := make([]StoredSample, 0, 12)
	for i := 0; i < 12; i++ {
		end := float64(now.Add(time.Duration(i-11) * 10 * time.Second).Unix())
		rows = append(rows, StoredSample{Node: "node", Sample: Sample{SampleID: fmt.Sprintf("550e8400-e29b-41d4-a716-44665544%04d", i+10), StartedAt: end - 10, EndedAt: end, Model: "fixture", Provider: "fixture", Task: "chat", Calls: 2, InputTokens: 30, OutputTokens: 10, Coverage: &covered}})
	}
	if err := s.DB.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	view, err := s.Overview(Query{From: float64(now.Add(-2 * time.Minute).Unix()), To: float64(now.Unix()), Timezone: "UTC"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if view.RangeUsage.Coverage != "covered" || view.RangeUsage.CollectedTokens != 480 || view.RangeUsage.CollectedCalls != 24 || view.RangeUsage.AverageTokensPerCall == nil || *view.RangeUsage.AverageTokensPerCall != 20 || view.RangeUsage.AverageCallsPerHour != 720 {
		t.Fatalf("covered range usage = %+v", view.RangeUsage)
	}
	if len(view.Trends) == 0 || view.Trends[0].Coverage != "covered" {
		t.Fatalf("covered trend must be marked covered: %+v", view.Trends)
	}

	rows[0].ID = 0
	rows[0].Coverage = nil
	rows[0].SampleID = "550e8400-e29b-41d4-a716-446655449999"
	if err := s.DB.Create(&rows[0]).Error; err != nil {
		t.Fatal(err)
	}
	view, err = s.Overview(Query{From: float64(now.Add(-2 * time.Minute).Unix()), To: float64(now.Unix()), Timezone: "UTC"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if view.RangeUsage.Coverage != "partial" || view.RangeUsage.CollectedTokens != 520 || view.RangeUsage.CollectedCalls != 26 {
		t.Fatalf("partial range usage = %+v", view.RangeUsage)
	}
	if len(view.Trends) == 0 || view.Trends[0].Coverage != "partial" {
		t.Fatalf("partial trend must not be presented as verified: %+v", view.Trends)
	}
}

func TestOverviewTrendSeparatesVerifiedBucketsFromCollectionGaps(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000020, 0).UTC() // exact UTC minute boundary
	covered := true
	rows := []StoredSample{
		{Node: "node", Sample: Sample{SampleID: "550e8400-e29b-41d4-a716-446655440701", StartedAt: float64(now.Add(-3 * time.Minute).Unix()), EndedAt: float64(now.Add(-2 * time.Minute).Unix()), Model: "fixture", Provider: "fixture", Task: "chat", Calls: 1, InputTokens: 10, Coverage: &covered}},
		{Node: "node", Sample: Sample{SampleID: "550e8400-e29b-41d4-a716-446655440702", StartedAt: float64(now.Add(-time.Minute).Unix()), EndedAt: float64(now.Unix()), Model: "fixture", Provider: "fixture", Task: "chat", Calls: 2, InputTokens: 20, Coverage: &covered}},
	}
	if err := s.DB.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	view, err := s.Overview(Query{From: float64(now.Add(-3 * time.Minute).Unix()), To: float64(now.Unix()), Timezone: "UTC"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if view.RangeUsage.Coverage != "partial" || view.RangeUsage.CollectedCalls != 3 || view.RangeUsage.CollectedTokens != 30 {
		t.Fatalf("partial range must retain collected totals: %+v", view.RangeUsage)
	}
	if len(view.Trends) != 3 {
		t.Fatalf("trend bucket count = %d, want 3 with an explicit gap: %#v", len(view.Trends), view.Trends)
	}
	want := []string{"covered", "missing", "covered"}
	for i, coverage := range want {
		if view.Trends[i].Coverage != coverage {
			t.Fatalf("trend bucket %d coverage = %q, want %q: %#v", i, view.Trends[i].Coverage, coverage, view.Trends)
		}
	}
	if view.Trends[1].TotalTokens() != 0 || view.Trends[1].Calls != 0 {
		t.Fatalf("missing bucket must carry no fabricated usage: %+v", view.Trends[1])
	}
}

func TestOverviewTrendShowsLeadingAndTrailingCollectionGaps(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000020, 0).UTC()
	covered := true
	row := StoredSample{Node: "node", Sample: Sample{SampleID: "550e8400-e29b-41d4-a716-446655440703", StartedAt: float64(now.Add(-3 * time.Minute).Unix()), EndedAt: float64(now.Add(-2 * time.Minute).Unix()), Model: "fixture", Provider: "fixture", Task: "chat", Calls: 1, InputTokens: 10, Coverage: &covered}}
	if err := s.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	view, err := s.Overview(Query{From: float64(now.Add(-4 * time.Minute).Unix()), To: float64(now.Unix()), Timezone: "UTC"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Trends) != 4 {
		t.Fatalf("trend bucket count = %d, want full range including edge gaps: %#v", len(view.Trends), view.Trends)
	}
	want := []string{"missing", "covered", "missing", "missing"}
	for i, coverage := range want {
		if view.Trends[i].Coverage != coverage {
			t.Fatalf("trend bucket %d coverage = %q, want %q: %#v", i, view.Trends[i].Coverage, coverage, view.Trends)
		}
	}
}

func TestTrendJSONUsesNullForUnverifiedBucketsAndNumericZeroForCoveredBuckets(t *testing.T) {
	for _, tc := range []struct {
		name  string
		trend Trend
		want  any
	}{
		{name: "missing", trend: Trend{Timestamp: 1, Coverage: "missing"}, want: nil},
		{name: "partial", trend: Trend{Timestamp: 1, Coverage: "partial", Totals: Totals{Calls: 9, TotalTokenCount: 99}}, want: nil},
		{name: "covered_zero", trend: Trend{Timestamp: 1, Coverage: "covered"}, want: float64(0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(tc.trend)
			if err != nil {
				t.Fatal(err)
			}
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatal(err)
			}
			if got := payload["calls"]; got != tc.want {
				t.Fatalf("calls JSON = %#v, want %#v; body=%s", got, tc.want, body)
			}
			if got := payload["total_tokens"]; got != tc.want {
				t.Fatalf("total_tokens JSON = %#v, want %#v; body=%s", got, tc.want, body)
			}
		})
	}
}

func TestOverviewRangeUsageIsPartialForRollupsAndHasNoAverageWithoutCalls(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0).UTC()
	covered := true
	row := StoredSample{Node: "node", Sample: Sample{SampleID: "550e8400-e29b-41d4-a716-446655440099", StartedAt: float64(now.Add(-24 * time.Hour).Unix()), EndedAt: float64(now.Add(-24*time.Hour + 10*time.Second).Unix()), Model: "fixture", Provider: "fixture", Task: "idle", Coverage: &covered}}
	if err := s.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.Maintain(now); err != nil {
		t.Fatal(err)
	}
	view, err := s.Overview(Query{Preset: "7D", Timezone: "UTC"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if view.RangeUsage.Coverage != "partial" || view.RangeUsage.CollectedCalls != 0 || view.RangeUsage.AverageTokensPerCall != nil || view.RangeUsage.AverageCallsPerHour != 0 {
		t.Fatalf("rollup/idle range usage = %+v", view.RangeUsage)
	}
}

func TestOverviewRangeUsageSurvivesQuotaAccountTransition(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0).UTC()
	covered, percent := true, 20.0
	r := validReport(now)
	r.Samples[0].Coverage = &covered
	r.Quota = &Quota{AccountKey: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Provider: "openai", Plan: "Plus", Status: "available", ObservedAt: float64(now.Unix()), Windows: []QuotaWindow{{Label: "Session", UsedPercent: &percent}}}
	if _, err := s.Ingest("node", r, now); err != nil {
		t.Fatal(err)
	}
	then := now.Add(3 * time.Second)
	r.Samples[0].SampleID = "550e8400-e29b-41d4-a716-446655440098"
	r.Samples[0].StartedAt, r.Samples[0].EndedAt = float64(then.Add(-10*time.Second).Unix()), float64(then.Unix())
	r.ObservedAt = float64(then.Unix())
	r.Quota.AccountKey = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	r.Quota.ObservedAt = float64(then.Unix())
	if _, err := s.Ingest("node", r, then); err != nil {
		t.Fatal(err)
	}
	view, err := s.Overview(Query{From: float64(now.Add(-time.Hour).Unix()), To: float64(then.Unix()), Timezone: "UTC"}, then)
	if err != nil {
		t.Fatal(err)
	}
	if view.RangeUsage.CollectedCalls != 4 || len(view.Accounts) != 1 || view.Accounts[0].AccountKey != r.Quota.AccountKey || view.Accounts[0].Status != "available" {
		t.Fatalf("account transition lost range data or attribution: usage=%+v accounts=%+v", view.RangeUsage, view.Accounts)
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
	if view.PricingCoverage.Calls != int64(len(rows)) || view.PricingCoverage.TotalTokens() != int64(len(rows))*10 || view.RangeUsage.CollectedCalls != int64(len(rows)) || view.RangeUsage.Coverage != "partial" {
		t.Fatalf("large aggregate totals = %+v", view.PricingCoverage)
	}
	if len(view.Models) != 2 || len(view.Tasks) != 2 || len(view.Trends) == 0 || len(view.Daily) != 1 {
		t.Fatalf("large aggregate groups = models:%d tasks:%d trends:%d daily:%d", len(view.Models), len(view.Tasks), len(view.Trends), len(view.Daily))
	}
	week, err := s.Overview(Query{Preset: "7D", Timezone: "UTC"}, now)
	if err != nil || week.PricingCoverage.Calls != int64(len(rows)) || week.RangeUsage.CollectedCalls != int64(len(rows)) || week.RangeUsage.Coverage != "partial" || len(week.Trends) == 0 {
		t.Fatalf("7-day aggregate = %#v, %v", week.PricingCoverage, err)
	}
	month, err := s.Overview(Query{Preset: "30D", Timezone: "Asia/Shanghai"}, now)
	if err != nil || month.PricingCoverage.Calls != int64(len(rows)) || month.RangeUsage.CollectedCalls != int64(len(rows)) || month.RangeUsage.Coverage != "partial" || len(month.Trends) == 0 {
		t.Fatalf("30-day aggregate = %#v, %v", month.PricingCoverage, err)
	}
	today, err := s.Overview(Query{Preset: "Today", Timezone: "UTC"}, now)
	if err != nil || today.PricingCoverage.Calls != int64(len(rows)) || today.Live.TodayTokens != int64(len(rows))*10 {
		t.Fatalf("today aggregate = %#v, %v", today.PricingCoverage, err)
	}
	t.Logf("aggregated %d samples in %s", len(rows), time.Since(started))
}

func TestOverviewModelRegistryKeepsPriorityCustomAndHistoricalIdentifiers(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	r := validReport(now)
	r.Samples[0].SampleID = "550e8400-e29b-41d4-a716-446655440401"
	r.Samples[0].Model = "historic-custom"
	if _, err := s.Ingest("node", r, now); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSettings(Settings{Timezone: "UTC", RetentionDays: 30, DetailedRetentionHours: 24, CustomModels: []string{"operator-model"}}); err != nil {
		t.Fatal(err)
	}
	v, err := s.Overview(Query{From: float64(now.Add(-time.Hour).Unix()), To: float64(now.Unix()), Timezone: "UTC"}, now)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, model := range v.ModelRegistry {
		got[model] = true
	}
	for _, want := range []string{"gpt-6.1-sol", "gpt-6-sol", "gpt-5.6-terra", "historic-custom", "operator-model"} {
		if !got[want] {
			t.Fatalf("registry missing %s: %#v", want, v.ModelRegistry)
		}
	}
}
