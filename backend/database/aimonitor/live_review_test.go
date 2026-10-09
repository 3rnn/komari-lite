package aimonitor

import (
	"testing"
	"time"
)

func liveReviewRows(now time.Time) ([]StoredSample, []NodeState) {
	end := float64(now.Unix())
	covered := true
	rows := []StoredSample{}
	for i := 0; i < 6; i++ {
		tokens, calls := int64(10), int64(1)
		if i == 5 {
			tokens, calls = 100, 10
		}
		rows = append(rows, StoredSample{Node: "node", Sample: Sample{StartedAt: end - 60 + float64(i*10), EndedAt: end - 50 + float64(i*10), InputTokens: tokens, OutputTokens: tokens, CacheReadTokens: tokens, CacheWriteTokens: tokens, Calls: calls, Coverage: &covered}})
	}
	return rows, []NodeState{{Node: "node", Enabled: true, Status: "collecting", ObservedAt: end, SamplingIntervalSeconds: 10}}
}
func TestLiveChecksEverySampleAgeAgainstWallClock(t *testing.T) {
	now := time.Unix(1780000000, 0)
	rows, states := liveReviewRows(now.Add(-100 * time.Second))
	states[0].ObservedAt = float64(now.Unix())
	states = append(states, NodeState{Node: "other", Enabled: true, Status: "collecting", ObservedAt: float64(now.Unix()), SamplingIntervalSeconds: 10})
	other, _ := liveReviewRows(now.Add(-100 * time.Second))
	for _, row := range other {
		row.Node = "other"
		rows = append(rows, row)
	}
	for i := 0; i < 5; i++ {
		row := other[5]
		row.Node = "other"
		row.StartedAt += float64((i + 1) * 10)
		row.EndedAt += float64((i + 1) * 10)
		rows = append(rows, row)
	}
	// Iteration order must not make an old node fresh after the common endpoint moves.
	for i := 0; i < 128; i++ {
		got := computeLive(Live{Status: "unavailable"}, rows, states, nil, now)
		if got.TPS != nil || got.RPM != nil {
			t.Fatalf("stale sample accepted: %+v", got)
		}
	}
}

func TestLiveLatestTenSecondsDoesNotRequireMinuteCoverage(t *testing.T) {
	now := time.Unix(1780000000, 0)
	rows, states := liveReviewRows(now)
	got := computeLive(Live{Status: "unavailable"}, rows[5:], states, nil, now)
	if got.TPS == nil || *got.TPS != 40 || got.RPM != nil || got.TPM != nil {
		t.Fatalf("independent windows: %+v", got)
	}
}

func TestLiveCrossBoundaryWindowIsNotInterpolated(t *testing.T) {
	now := time.Unix(1780000000, 0)
	rows, states := liveReviewRows(now)
	rows[5].StartedAt -= 5
	got := computeLive(Live{Status: "unavailable"}, rows, states, nil, now)
	if got.TPS != nil || got.RPM == nil || *got.RPM != 15 {
		t.Fatalf("cross-boundary latest interval: %+v", got)
	}
}

func TestLiveUsesLatestTenSecondsSeparatelyFromRollingMinute(t *testing.T) {
	now := time.Unix(1780000000, 0)
	rows, states := liveReviewRows(now)
	got := computeLive(Live{Status: "unavailable"}, rows, states, nil, now.Add(30*time.Second))
	for name, pair := range map[string][2]*float64{"tps": {got.TPS, floatPtrValue(40)}, "input": {got.InputTPS, floatPtrValue(10)}, "output": {got.OutputTPS, floatPtrValue(10)}, "cache": {got.CacheReadTPS, floatPtrValue(10)}, "rps": {got.RPS, floatPtrValue(1)}, "rpm": {got.RPM, floatPtrValue(15)}, "tpm": {got.TPM, floatPtrValue(600)}} {
		if pair[0] == nil || *pair[0] != *pair[1] {
			t.Errorf("%s: got %v; expected %v", name, pair[0], *pair[1])
		}
	}
	if got.RollingCalls != 15 || got.RollingTokens != 600 {
		t.Fatalf("minute totals: %+v", got)
	}
}
