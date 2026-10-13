package aimonitor

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDailyCoverageUsesOwnIntervalsNotRangeStatus(t *testing.T) {
	q := Query{From: 0, To: 172800, Timezone: "UTC"}
	days := []Day{{Date: "1970-01-01"}, {Date: "1970-01-02"}}
	trends := []Trend{}
	for at := 0.; at < q.To; at += 3600 {
		coverage := "covered"
		if at == 90000 {
			coverage = "missing"
		}
		trends = append(trends, Trend{Timestamp: at, Coverage: coverage})
	}
	applyDailyCoverage(days, trends, q, 3600, time.UTC)
	if days[0].Coverage != "covered" || days[1].Coverage != "partial" {
		t.Fatalf("coverage: %+v", days)
	}
	applyDailyCoverage(days, nil, q, 3600, time.UTC)
	if days[0].Coverage != "unknown" {
		t.Fatal("missing evidence must stay unknown")
	}
}
func TestRangeEvidenceSerializesUnknownWithoutInventedReasons(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	view, err := s.Overview(Query{From: float64(now.Unix() - 60), To: float64(now.Unix()), Timezone: "UTC"}, now)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"coverage_reason":"no_interval_evidence"`) {
		t.Fatalf("missing evidence reason: %s", encoded)
	}
}
