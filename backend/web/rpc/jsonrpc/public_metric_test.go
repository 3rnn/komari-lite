package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/komari-monitor/komari/database/metricstore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/metric"
	"github.com/komari-monitor/komari/pkg/rpc"
)

func TestPublicQueryMetricsRejectsOversizedRequestsBeforeStorage(t *testing.T) {
	end := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	start := end.Add(-time.Hour)
	keys := make([]string, 17)
	for i := range keys {
		keys[i] = fmt.Sprintf("metric.%d", i)
	}
	entities := make([]string, 129)
	for i := range entities {
		entities[i] = fmt.Sprintf("node-%d", i)
	}
	for _, tc := range []struct {
		name   string
		params map[string]any
	}{
		{"future range", map[string]any{"metric_key": "cpu.usage", "start": end.AddDate(10, 0, 0), "end": end.AddDate(10, 0, 0).Add(time.Hour)}},
		{"ancient range", map[string]any{"metric_key": "cpu.usage", "start": time.Date(1000, 1, 1, 0, 0, 0, 0, time.UTC), "end": time.Date(1000, 1, 1, 1, 0, 0, 0, time.UTC)}},
		{"range", map[string]any{"metric_key": "cpu.usage", "start": end.Add(-91 * 24 * time.Hour), "end": end}},
		{"metric count", map[string]any{"metric_keys": keys, "start": start, "end": end}},
		{"entity count", map[string]any{"metric_key": "cpu.usage", "entity_ids": entities, "start": start, "end": end}},
		{"series product", map[string]any{"metric_keys": keys[:16], "entity_ids": entities[:9], "start": start, "end": end}},
		{"points", map[string]any{"metric_key": "cpu.usage", "max_points": 1001, "start": start, "end": end}},
		{"per-metric points", map[string]any{"metric_key": "cpu.usage", "max_points_by_metric": map[string]int{"cpu.usage": 1001}, "start": start, "end": end}},
		{"raw series count", map[string]any{"metric_key": "cpu.usage", "entity_ids": entities[:9], "server_downsample": false, "start": start, "end": end}},
		{"raw range", map[string]any{"metric_key": "cpu.usage", "server_downsample": false, "start": end.Add(-5 * time.Hour), "end": end}},
		{"raw per-metric override", map[string]any{"metric_key": "cpu.usage", "server_downsample_by_metric": map[string]bool{"cpu.usage": false}, "start": end.Add(-5 * time.Hour), "end": end}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := publicQueryMetrics(context.Background(), &rpc.JsonRpcRequest{Params: tc.params})
			if err == nil || err.Code != rpc.InvalidParams {
				t.Fatalf("oversized request must be rejected as invalid params before storage, got %v", err)
			}
		})
	}
}

func TestPublicPingMetricStatsRejectsOversizedRequestsBeforeStorage(t *testing.T) {
	end := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	entities := make([]string, maxPublicMetricEntities+1)
	for i := range entities {
		entities[i] = fmt.Sprintf("node-%d", i)
	}
	for _, tc := range []struct {
		name   string
		params map[string]any
	}{
		{"range", map[string]any{"start": end.Add(-maxPublicMetricRange - time.Second), "end": end}},
		{"future", map[string]any{"start": end.AddDate(10, 0, 0), "end": end.AddDate(10, 0, 0).Add(time.Hour)}},
		{"ancient", map[string]any{"start": time.Date(1000, 1, 1, 0, 0, 0, 0, time.UTC), "end": time.Date(1000, 1, 1, 1, 0, 0, 0, time.UTC)}},
		{"hours", map[string]any{"hours": maxPublicMetricRange.Hours() + 1}},
		{"entities", map[string]any{"entity_ids": entities, "start": end.Add(-time.Hour), "end": end}},
		{"max points", map[string]any{"max_points": maxPublicMetricQueryPoints + 1, "start": end.Add(-time.Hour), "end": end}},
		{"downsample points", map[string]any{"downsample_points": maxPublicMetricQueryPoints + 1, "start": end.Add(-time.Hour), "end": end}},
		{"negative points", map[string]any{"max_points": -1, "start": end.Add(-time.Hour), "end": end}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := publicGetPingMetricStats(context.Background(), &rpc.JsonRpcRequest{Params: tc.params})
			if err == nil || err.Code != rpc.InvalidParams {
				t.Fatalf("oversized ping stats request must be rejected before storage, got %v", err)
			}
		})
	}
}

func TestPublicPingMetricStatsAcceptsNormalQueryLimits(t *testing.T) {
	end := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for _, points := range []int{0, defaultMetricQueryPoints, maxPublicMetricQueryPoints} {
		params := publicMetricQueryParams{MaxPoints: points}
		if err := validatePublicMetricQuery(params, []string{metricstore.MetricPingLatency}, []string{"visible-node"}, end.Add(-4*time.Hour), end); err != nil {
			t.Fatalf("normal ping query with max_points=%d rejected: %v", points, err)
		}
	}
}

func TestPublicRawMetricQueryRejectsExcessPoints(t *testing.T) {
	ctx := context.Background()
	store, err := metric.Open(ctx, metric.Config{Driver: metric.DriverSQLite, DSN: t.TempDir() + "/metrics.db", AutoMigrate: true})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.CreateMetric(ctx, metric.Definition{Name: "test.cpu", Type: metric.TypeGauge, RetentionDays: 7}); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	points := make([]metric.Point, maxPublicRawMetricPoints+1)
	for i := range points {
		points[i] = metric.Point{MetricName: "test.cpu", EntityID: "node-a", Timestamp: start.Add(time.Duration(i) * time.Second), Value: float64(i)}
	}
	if err := store.WriteBatch(ctx, points); err != nil {
		t.Fatal(err)
	}
	query := metric.Query{MetricName: "test.cpu", EntityID: "node-a", Start: start, End: start.Add(time.Duration(maxPublicRawMetricPoints-1) * time.Second), Order: metric.OrderAsc}
	got, err := queryPublicRawMetricSeries(ctx, store, []metric.Query{query})
	if err != nil || len(got) != 1 || len(got[0]) != maxPublicRawMetricPoints {
		t.Fatalf("raw query at point budget: count=%d, err=%v", len(got), err)
	}
	query.End = start.Add(time.Hour)
	if _, err := queryPublicRawMetricSeries(ctx, store, []metric.Query{query}); err == nil {
		t.Fatal("raw query must reject more than its point budget")
	}
}

func TestPublicRawMetricQueryRespectsConcurrencyGate(t *testing.T) {
	for i := 0; i < cap(publicRawMetricReadGate); i++ {
		publicRawMetricReadGate <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(publicRawMetricReadGate); i++ {
			<-publicRawMetricReadGate
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := queryPublicRawMetricSeries(ctx, nil, []metric.Query{{MetricName: "test.cpu"}}); err != context.Canceled {
		t.Fatalf("queued raw query must honor cancellation, got %v", err)
	}
}

func TestMetricQueryParamsRequireRFC3339Time(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{name: "RFC3339", value: "2026-07-17T09:30:00.123456789+08:00"},
		{name: "offset free", value: "2026-07-17 09:30:00.123456789", wantErr: true},
		{name: "Unix number", value: float64(1_752_720_600), wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := &rpc.JsonRpcRequest{Params: map[string]any{"start": test.value}}
			var params publicMetricQueryParams
			err := req.BindParams(&params)
			if (err != nil) != test.wantErr {
				t.Fatalf("BindParams() error = %v, wantErr %v", err, test.wantErr)
			}
			if err == nil {
				want := time.Date(2026, 7, 17, 1, 30, 0, 123456789, time.UTC)
				if params.Start == nil || !params.Start.Equal(want) {
					t.Fatalf("start = %v, want %s", params.Start, want)
				}
			}
		})
	}
}

func TestSplitPublicMetricSeriesKeepsTagSeries(t *testing.T) {
	baseTime := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	base := publicMetricSeries{
		MetricKey: "gpu.device.usage",
		EntityID:  "node-a",
		Points: []publicMetricPoint{
			{Time: baseTime, Value: publicMetricValue(10), Count: 2, Tags: map[string]string{"device_index": "0"}},
			{Time: baseTime, Value: publicMetricValue(80), Count: 2, Tags: map[string]string{"device_index": "1"}},
			{Time: baseTime.Add(time.Minute), Value: publicMetricValue(20), Count: 2, Tags: map[string]string{"device_index": "0"}},
		},
	}

	got := splitPublicMetricSeries(base)
	if len(got) != 2 {
		t.Fatalf("expected 2 tag series, got %d: %#v", len(got), got)
	}
	if got[0].Tags["device_index"] != "0" || got[0].Count != 2 {
		t.Fatalf("unexpected first series: %#v", got[0])
	}
	if got[1].Tags["device_index"] != "1" || got[1].Count != 1 {
		t.Fatalf("unexpected second series: %#v", got[1])
	}
	if got[0].Points[0].Tags["device_index"] != "0" || got[1].Points[0].Tags["device_index"] != "1" {
		t.Fatalf("point tags were not preserved: %#v", got)
	}
}

func TestPublicMetricJSONIncludesOnlyTags(t *testing.T) {
	pointTime := time.Date(2026, 6, 18, 0, 0, 0, 123456789, time.UTC)
	payload, err := json.Marshal(publicMetricSeries{
		MetricKey: "ping.loss",
		EntityID:  "node-a",
		Tags:      map[string]string{"task_id": "7"},
		Points: []publicMetricPoint{{
			Time:  pointTime,
			Value: publicMetricValue(0),
			Tags:  map[string]string{"task_id": "7"},
		}},
	})
	if err != nil {
		t.Fatalf("marshal series: %v", err)
	}
	text := string(payload)
	if !strings.Contains(text, `"tags":{"task_id":"7"}`) {
		t.Fatalf("series tags missing: %s", text)
	}
	if strings.Contains(text, `"tag":`) {
		t.Fatalf("legacy tag field should not be serialized: %s", text)
	}
	if !strings.Contains(text, `"time":"2026-06-18T00:00:00.123456789Z"`) {
		t.Fatalf("metric time is not UTC RFC3339Nano: %s", text)
	}
}

func TestAdaptiveFillPublicMetricSeriesUsesObservedInterval(t *testing.T) {
	base := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	series := publicMetricSeries{
		MetricKey:       "cpu.usage",
		EntityID:        "node-a",
		FillEmpty:       true,
		IntervalSeconds: 1,
		Tags:            map[string]string{"core": "0"},
	}
	for i := 0; i < 10; i++ {
		series.Points = append(series.Points, publicMetricPoint{
			Time:  base.Add(time.Duration(i) * time.Minute),
			Value: publicMetricValue(float64(i)),
		})
	}

	got := adaptiveFillPublicMetricSeries(series, base, base.Add(9*time.Minute))
	if got.IntervalSeconds != 60 {
		t.Fatalf("expected observed 60s interval, got %v", got.IntervalSeconds)
	}
	if len(got.Points) != 10 {
		t.Fatalf("regular sparse series should not gain null buckets, got %#v", got.Points)
	}
	for _, point := range got.Points {
		if point.Value == nil {
			t.Fatalf("regular sparse series gained a null point: %#v", got.Points)
		}
	}
}

func TestAdaptiveFillPublicMetricSeriesAddsCompactGapsAndBounds(t *testing.T) {
	base := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	tags := map[string]string{"device_index": "0"}
	series := publicMetricSeries{
		MetricKey:       "gpu.device.usage",
		EntityID:        "node-a",
		IntervalSeconds: 1,
		Tags:            tags,
		Points: []publicMetricPoint{
			{Time: base, Value: publicMetricValue(10)},
			{Time: base.Add(time.Minute), Value: publicMetricValue(20)},
			{Time: base.Add(2 * time.Minute), Value: publicMetricValue(30)},
			{Time: base.Add(4 * time.Minute), Value: publicMetricValue(40)},
		},
	}
	start := base.Add(-30 * time.Second)
	end := base.Add(4*time.Minute + 30*time.Second)
	got := adaptiveFillPublicMetricSeries(series, start, end)
	if got.IntervalSeconds != 60 {
		t.Fatalf("expected observed 60s interval, got %v", got.IntervalSeconds)
	}
	if len(got.Points) != 7 {
		t.Fatalf("expected four values, one gap and two bounds, got %#v", got.Points)
	}
	wantNullTimes := map[time.Time]bool{
		start:                     true,
		base.Add(3 * time.Minute): true,
		end:                       true,
	}
	for _, point := range got.Points {
		if point.Value != nil {
			continue
		}
		if !wantNullTimes[point.Time] {
			t.Fatalf("unexpected null point at %s: %#v", point.Time, got.Points)
		}
		if point.Tags["device_index"] != "0" {
			t.Fatalf("adaptive null point lost tags: %#v", point)
		}
		delete(wantNullTimes, point.Time)
	}
	if len(wantNullTimes) != 0 {
		t.Fatalf("missing expected null points: %#v", wantNullTimes)
	}
}

func TestPublicPingMetricFillEmptyMapsMinusOneToNull(t *testing.T) {
	for _, metricName := range []string{metricstore.MetricPingLatency, metricstore.MetricPingLoss} {
		if value := publicRawMetricValue(metricName, -1, true); value != nil {
			t.Fatalf("raw %s -1 should become null when fill_empty is enabled, got %v", metricName, *value)
		}
	}
	if value := publicRawMetricValue(metricstore.MetricPingLatency, -1, true); value != nil {
		t.Fatalf("downsampled ping -1 should become null when fill_empty is enabled, got %v", *value)
	}
}

func TestPublicPingMetricMinusOneIsPreservedWithoutFillEmpty(t *testing.T) {
	value := publicRawMetricValue(metricstore.MetricPingLatency, -1, false)
	if value == nil || *value != -1 {
		t.Fatalf("raw ping -1 should be preserved when fill_empty is disabled, got %v", value)
	}

	nonPing := publicRawMetricValue("temperature", -1, true)
	if nonPing == nil || *nonPing != -1 {
		t.Fatalf("negative values from non-ping metrics must be preserved, got %v", nonPing)
	}
}

func TestPublicPingStatsFromAggregateGroupsUsesTaskNamesAndLossMetric(t *testing.T) {
	base := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	taskMap := map[string]models.PingTask{
		"1": {Id: 1, Name: "Tokyo ICMP", Clients: models.StringArray{"node-a"}, Type: "icmp", Interval: 60},
	}
	groups := publicPingMetricAggregateGroups{
		Avg: map[string][]metric.AggregatePoint{
			"1": {
				{Bucket: base, Count: 2, Value: 20},
				{Bucket: base.Add(time.Minute), Count: 2, Value: 40},
			},
		},
		Min: map[string][]metric.AggregatePoint{
			"1": {{Bucket: base, Count: 4, Value: 12}},
		},
		Max: map[string][]metric.AggregatePoint{
			"1": {{Bucket: base, Count: 4, Value: 92}},
		},
		Last: map[string][]metric.AggregatePoint{
			"1": {{Bucket: base.Add(time.Minute), Count: 1, Value: 44}},
		},
		P50: map[string][]metric.AggregatePoint{
			"1": {{Bucket: base, Count: 4, Value: 30}},
		},
		P99: map[string][]metric.AggregatePoint{
			"1": {{Bucket: base, Count: 4, Value: 80}},
		},
		StdDev: map[string][]metric.AggregatePoint{
			"1": {{Bucket: base, Count: 4, Value: 8}},
		},
		Loss: map[string][]metric.AggregatePoint{
			"1": {{Bucket: base, Count: 4, Value: 0.25}},
		},
		LossAvailable: true,
	}

	stats := publicPingStatsFromAggregateGroups("node-a", groups, taskMap, nil)
	if len(stats) != 1 {
		t.Fatalf("expected one stat, got %#v", stats)
	}
	got := stats[0]
	if got.Name != "Tokyo ICMP" || got.Type != "icmp" || got.Interval != 60 {
		t.Fatalf("task metadata not applied: %#v", got)
	}
	if got.Total != 4 || got.Valid != 3 {
		t.Fatalf("unexpected totals: %#v", got)
	}
	if got.Loss != 25 || got.LossApproximate {
		t.Fatalf("loss should come from ping.loss metric: %#v", got)
	}
	if got.Min == nil || *got.Min != 12 || got.Max == nil || *got.Max != 92 || got.Avg == nil || *got.Avg != 30 {
		t.Fatalf("latency stats mismatch: %#v", got)
	}
	if math.Abs(got.P99P50Ratio-1.6666666666666667) > 0.000001 {
		t.Fatalf("unexpected volatility ratio: %#v", got)
	}
}

func TestPublicPingStatsFromAggregateGroupsExcludesUnassignedHistory(t *testing.T) {
	base := time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC)
	groups := publicPingMetricAggregateGroups{
		Avg: map[string][]metric.AggregatePoint{
			"1": {{Bucket: base, Count: 1, Value: 20}},
			"2": {{Bucket: base, Count: 1, Value: 30}},
			"3": {{Bucket: base, Count: 1, Value: 40}},
		},
	}
	taskMap := map[string]models.PingTask{
		"1": {Id: 1, Name: "removed", Clients: models.StringArray{"node-b"}},
		"2": {Id: 2, Name: "active", Clients: models.StringArray{"node-a"}},
	}

	stats := publicPingStatsFromAggregateGroups("node-a", groups, taskMap, nil)
	if len(stats) != 1 || stats[0].TaskID != "2" || stats[0].Name != "active" {
		t.Fatalf("only the currently assigned task should remain, got %#v", stats)
	}
}

func TestPublicPingMetricStatsIncludesZeroVolatility(t *testing.T) {
	payload, err := json.Marshal(publicPingMetricTaskStats{
		EntityID:    "node-a",
		TaskID:      "1",
		Total:       1,
		Valid:       1,
		P99P50Ratio: 0,
	})
	if err != nil {
		t.Fatalf("marshal ping stats: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal ping stats: %v", err)
	}
	if value, ok := decoded["p99_p50_ratio"]; !ok || value != float64(0) {
		t.Fatalf("zero volatility must be present, got %s", payload)
	}
}

func TestMetricDownsampleIntervalCeilsToStandardInterval(t *testing.T) {
	got := metricDownsampleInterval(30*24*time.Hour, 500)
	if got != 2*time.Hour {
		t.Fatalf("30d/500 should ceil to 2h, got %s", got)
	}

	got = metricDownsampleInterval(time.Hour, 500)
	if got != 10*time.Second {
		t.Fatalf("1h/500 should ceil to 10s, got %s", got)
	}

	got = metricDownsampleInterval(1000*24*time.Hour, 10)
	if got != 100*24*time.Hour {
		t.Fatalf("ranges beyond the standard table should ceil to whole days, got %s", got)
	}
}
