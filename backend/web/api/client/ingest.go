package client

import (
	"context"
	"time"

	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/metricstore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/database/tasks"
	v1 "github.com/komari-monitor/komari/protocol/v1"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

// ingest.go
// Shared transport-independent handling of agent reports from v1 (REST/WS) and v2 (JSON-RPC).
// Both protocols call these functions after parsing to persist reports and update runtime state.

// ingestReport stores a load report and refreshes runtime state.
// protocolVersion identifies the report protocol (1 or 2) for runtime capability tracking.
// When markPresence is true, refresh HTTP POST presence; WebSocket connections manage their own presence and pass false.
func ingestReport(uuid string, report v1.Report, protocolVersion int, markPresence bool) error {
	report.UUID = uuid
	report.UpdatedAt = time.Now().UTC()
	if err := clients.ReportVerify(report); err != nil {
		return err
	}
	savedReport, err := metricstore.WriteReport(context.Background(), report)
	if err != nil {
		return err
	}
	agent_runtime.RecordReport(savedReport)
	agent_runtime.SetClientProtocolVersion(uuid, protocolVersion)
	if markPresence {
		refreshPostPresence(uuid)
	}
	return nil
}

// ingestBasicInfo stores client details, using fallbackIP when a report omits the IP.
func ingestBasicInfo(uuid string, info map[string]interface{}, fallbackIP string) error {
	if info == nil {
		info = map[string]interface{}{}
	}
	return saveClientBasicInfo(info, uuid, fallbackIP)
}

// ingestPingResult stores a ping probe result.
func ingestPingResult(uuid string, taskID uint, value int) error {
	return tasks.SavePingRecord(models.PingRecord{
		Client: uuid,
		TaskId: taskID,
		Value:  value,
		Time:   time.Now().UTC(),
	})
}
