package notifier

import (
	"context"
	"fmt"
	"strings"
	"time"

	clientdb "github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	messageevent "github.com/komari-monitor/komari/database/models/messageEvent"
	"github.com/komari-monitor/komari/database/trafficledger"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/corn"
	logger "github.com/komari-monitor/komari/utils/log"
	"github.com/komari-monitor/komari/utils/messageSender"
)

var beijingLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

type TrafficReportSendResult struct {
	Sent        bool `json:"sent"`
	ClientCount int  `json:"client_count"`
}

type trafficReportTarget struct {
	client       models.Client
	notification models.TrafficReportNotification
}

func (target trafficReportTarget) hasReportContent() bool {
	return target.notification.IncludeTraffic || (target.notification.IncludeBilling && target.client.Price > 0)
}

func trafficReportTargetsInClientOrder(notifications []models.TrafficReportNotification, clientList []models.Client) []trafficReportTarget {
	notificationsByClient := make(map[string]models.TrafficReportNotification, len(notifications))
	for _, notification := range notifications {
		notificationsByClient[notification.Client] = notification
	}

	targets := make([]trafficReportTarget, 0, len(notifications))
	for _, client := range clientList {
		notification, ok := notificationsByClient[client.UUID]
		if !ok {
			continue
		}
		targets = append(targets, trafficReportTarget{client: client, notification: notification})
	}
	return targets
}

// InitTrafficReportSchedule registers three scheduled tasks executed according to Beijing time: daily report, weekly report, and monthly report.
func InitTrafficReportSchedule() {
	if err := ReloadTrafficReportSchedule(); err != nil {
		logger.ErrorArgs("notifier", "Failed to register traffic report schedules:", err)
	}
	if err := corn.AddContextFunc("traffic-ledger-maintenance", "@every 1h", true, func(ctx context.Context) {
		if err := trafficledger.Maintain(ctx, dbcore.GetDBInstance(), time.Now().UTC()); err != nil {
			logger.Errorf("notifier", "Failed to maintain traffic ledger: %v", err)
		}
	}); err != nil {
		logger.ErrorArgs("notifier", "Failed to register traffic ledger maintenance:", err)
	}
}

// ReloadTrafficReportSchedule applies the configured HH:mm time without restarting Komari.
func ReloadTrafficReportSchedule() error {
	reportTime, err := config.GetAs[string](config.TrafficReportTimeKey, config.DefaultTrafficReportTime)
	if err != nil {
		return fmt.Errorf("load traffic report time: %w", err)
	}
	reportTime, err = config.NormalizeTrafficReportTime(reportTime)
	if err != nil {
		return err
	}
	parsed, _ := time.Parse("15:04", reportTime)
	minute, hour := parsed.Minute(), parsed.Hour()

	jobs := []struct {
		name string
		spec string
		run  func()
	}{
		{"traffic-report-daily", fmt.Sprintf("0 %d %d * * *", minute, hour), func() { runScheduledTrafficReport(true, false, false) }},
		{"traffic-report-weekly", fmt.Sprintf("0 %d %d * * 1", minute, hour), func() { runScheduledTrafficReport(false, true, false) }},
		{"traffic-report-monthly", fmt.Sprintf("0 %d %d 1 * *", minute, hour), func() { runScheduledTrafficReport(false, false, true) }},
	}
	for _, scheduledJob := range jobs {
		if err := corn.AddFuncInLocation(scheduledJob.name, scheduledJob.spec, beijingLocation, scheduledJob.run); err != nil {
			return err
		}
	}
	logger.Infof("notifier", "Traffic report schedules registered for %s Asia/Shanghai", reportTime)
	return nil
}

func runScheduledTrafficReport(daily, weekly, monthly bool) {
	if _, err := sendTrafficReport(daily, weekly, monthly, false); err != nil {
		logger.Errorf("notifier", "Failed to send scheduled traffic report: %v", err)
	}
}

// SendDailyTrafficReportNow sends traffic from 00:00 Beijing time through the click time.
func SendDailyTrafficReportNow() (TrafficReportSendResult, error) {
	return sendTrafficReport(true, false, false, true)
}

// sendTrafficReport aggregates all server traffic with the specified report type enabled and combines it into a notification to send.
func sendTrafficReport(daily, weekly, monthly, currentDaily bool) (TrafficReportSendResult, error) {
	result := TrafficReportSendResult{}
	// Check the global notification switch
	enabled, err := config.GetAs[bool](config.NotificationEnabledKey, false)
	if err != nil {
		return result, fmt.Errorf("load notification setting: %w", err)
	}
	if !enabled {
		return result, fmt.Errorf("notifications are disabled")
	}

	db := dbcore.GetDBInstance()
	now := time.Now().UTC()

	var eventType, label, suffix string

	switch {
	case daily:
		eventType = messageevent.DReport
		label = "daily"
		if currentDaily {
			suffix = "Today's traffic"
		} else {
			suffix = "Yesterday's traffic"
		}
	case weekly:
		eventType = messageevent.WReport
		label = "weekly"
		suffix = "Last week's traffic"
	case monthly:
		eventType = messageevent.MReport
		label = "monthly"
		suffix = "Last month's traffic"
	default:
		return result, fmt.Errorf("traffic report cadence is required")
	}
	start, end := previousTrafficReportRange(now, label)
	if currentDaily {
		start, end = currentDailyTrafficReportRange(now)
	}

	// Query all server configurations that enable this type of reporting
	var notifications []models.TrafficReportNotification
	query := db.Model(&models.TrafficReportNotification{}).Where("enable = ?", true)
	if daily {
		query = query.Where("daily = ?", true)
	} else if weekly {
		query = query.Where("weekly = ?", true)
	} else if monthly {
		query = query.Where("monthly = ?", true)
	}
	if err := query.Find(&notifications).Error; err != nil {
		return result, fmt.Errorf("query %s traffic report notifications: %w", label, err)
	}
	if len(notifications) == 0 {
		return result, nil
	}

	// Get client information
	clientUUIDs := make([]string, 0, len(notifications))
	for _, n := range notifications {
		clientUUIDs = append(clientUUIDs, n.Client)
	}
	clientList, err := clientdb.GetClientBasicInfoByUUIDs(clientUUIDs)
	if err != nil {
		return result, fmt.Errorf("query clients for %s traffic report: %w", label, err)
	}
	targets := trafficReportTargetsInClientOrder(notifications, clientList)
	ctx := context.Background()
	ledgerStart := trafficledger.BeijingDay(start)
	ledgerEnd := trafficledger.BeijingDay(end.Add(time.Nanosecond))
	if !currentDaily && len(targets) > 0 {
		targetIDs := make([]string, 0, len(targets))
		for _, target := range targets {
			if target.hasReportContent() {
				targetIDs = append(targetIDs, target.client.UUID)
			}
		}
		if len(targetIDs) > 0 {
			if err := trafficledger.EnsureRange(ctx, db, targetIDs, ledgerStart, ledgerEnd); err != nil {
				return result, fmt.Errorf("settle %s traffic ledger: %w", label, err)
			}
		}
	}

	// Count traffic and splice messages for each server
	var lines []string
	eventClients := make([]models.Client, 0, len(targets))
	var lastClientError error
	for _, target := range targets {
		if !target.hasReportContent() {
			continue
		}
		var usage trafficUsage
		var err error
		if currentDaily {
			usage, err = trafficledger.AdjustedMetricUsage(ctx, db, target.client.UUID, start, end)
		} else {
			usage, err = trafficledger.AdjustedLedgerUsage(ctx, db, target.client.UUID, ledgerStart, ledgerEnd)
		}
		if err != nil {
			logger.Errorf("notifier", "Failed to compute traffic for client %s (%s): %v", target.client.UUID, label, err)
			lastClientError = err
			continue
		}

		line := formatTrafficReportLine(target.client, suffix, usage, target.notification.IncludeTraffic, target.notification.IncludeBilling)
		if line == "" {
			continue
		}
		lines = append(lines, line)
		eventClients = append(eventClients, target.client)
	}

	if len(lines) == 0 {
		if lastClientError != nil {
			return result, fmt.Errorf("compute traffic report: %w", lastClientError)
		}
		return result, nil
	}

	message := strings.Join(lines, "\n")
	var emoji string
	switch {
	case daily:
		emoji = "📊"
	case weekly:
		emoji = "📈"
	case monthly:
		emoji = "📅"
	}

	if err := messageSender.SendEvent(models.EventMessage{
		Event:   eventType,
		Clients: eventClients,
		Time:    now,
		Emoji:   emoji,
		Message: message,
	}); err != nil {
		return result, fmt.Errorf("send %s traffic report: %w", label, err)
	}
	result.Sent = true
	result.ClientCount = len(eventClients)
	return result, nil
}

func currentDailyTrafficReportRange(now time.Time) (time.Time, time.Time) {
	beijingNow := now.In(beijingLocation)
	start := time.Date(beijingNow.Year(), beijingNow.Month(), beijingNow.Day(), 0, 0, 0, 0, beijingLocation)
	return start.UTC(), now.UTC()
}

func previousTrafficReportRange(now time.Time, period string) (time.Time, time.Time) {
	localNow := now.In(beijingLocation)
	var startLocal, endLocal time.Time

	switch period {
	case "daily":
		yesterday := localNow.AddDate(0, 0, -1)
		startLocal = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, beijingLocation)
		endLocal = startLocal.AddDate(0, 0, 1)
	case "weekly":
		weekday := int(localNow.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		lastMonday := localNow.AddDate(0, 0, -(weekday-1)-7)
		startLocal = time.Date(lastMonday.Year(), lastMonday.Month(), lastMonday.Day(), 0, 0, 0, 0, beijingLocation)
		endLocal = startLocal.AddDate(0, 0, 7)
	case "monthly":
		endLocal = time.Date(localNow.Year(), localNow.Month(), 1, 0, 0, 0, 0, beijingLocation)
		startLocal = endLocal.AddDate(0, -1, 0)
	default:
		return time.Time{}, time.Time{}
	}

	return startLocal.UTC(), endLocal.Add(-time.Nanosecond).UTC()
}

type trafficUsage = trafficledger.Usage

func formatTrafficReportLine(client models.Client, suffix string, usage trafficUsage, includeTraffic, includeBilling bool) string {
	name := strings.TrimSpace(client.Name)
	if name == "" {
		name = client.UUID
	}
	parts := make([]string, 0, 3)
	if includeTraffic {
		parts = append(parts, "Upload "+humanBytes(usage.Up), "Download "+humanBytes(usage.Down))
	}
	if includeBilling && client.Price > 0 {
		rule := strings.ToLower(strings.TrimSpace(client.TrafficLimitType))
		switch rule {
		case "up", "down", "sum", "min", "max":
		default:
			rule = "max"
		}
		used := computeUsedByType(rule, usage.Up, usage.Down)
		parts = append(parts, fmt.Sprintf("Billable traffic %s (%s)", humanBytes(used), rule))
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("%s %s:%s", name, suffix, strings.Join(parts, ","))
}

// getClientTrafficInRange queries the upstream and downstream traffic increments of a client within a specified time period.
//
// The historical monitoring data has been completely migrated to the metric store. Here, the records in the interval are read from the metric store and
// Accumulate the exact flow increment field to calculate usage; fall back to the accumulated flow difference when the increment is missing.
func getClientTrafficInRange(clientUUID string, start, end time.Time) (trafficUsage, error) {
	return trafficledger.MetricUsage(context.Background(), clientUUID, start, end)
}

type trafficDeltaRecord = trafficledger.DeltaRecord

func sumTrafficDeltas(records []trafficDeltaRecord, previous *trafficDeltaRecord) (int64, int64) {
	return trafficledger.SumTrafficDeltas(records, previous)
}

func trafficDeltaOrFallback(storedDelta int64, storedDeltaSet bool, currentTotal, previousTotal int64) int64 {
	return trafficledger.TrafficDeltaOrFallback(storedDelta, storedDeltaSet, currentTotal, previousTotal)
}
