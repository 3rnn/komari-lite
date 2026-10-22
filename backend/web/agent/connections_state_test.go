package agent

import (
	v1 "github.com/komari-monitor/komari/protocol/v1"
	"testing"
	"time"
)

func TestSingleClientStateIsIsolated(t *testing.T) {
	id := "single-state-test"
	defer DeleteConnectedClients(id)
	RecordReport(v1.Report{UUID: id, UpdatedAt: time.Now(), Uptime: 42})
	report := GetLatestReportByUUID(id)
	if report == nil || report.Uptime != 42 {
		t.Fatalf("report = %#v", report)
	}
	report.Uptime = 99
	if GetLatestReportByUUID(id).Uptime != 42 {
		t.Fatal("getter exposed mutable runtime report")
	}
	if GetLatestReportByUUID("missing-state-test") != nil {
		t.Fatal("missing report exists")
	}
	KeepAlivePresence(id, 1, time.Second)
	if !IsClientOnline(id) {
		t.Fatal("active presence not online")
	}
	KeepAlivePresence(id, 1, -time.Second)
	if IsClientOnline(id) {
		t.Fatal("expired presence still online")
	}
}
