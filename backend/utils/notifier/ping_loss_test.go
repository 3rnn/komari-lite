package notifier

import (
	"strings"
	"testing"
	"time"

	"github.com/komari-monitor/komari/database/models"
	messageevent "github.com/komari-monitor/komari/database/models/messageEvent"
	"github.com/komari-monitor/komari/pkg/metric"
	"github.com/stretchr/testify/assert"
)

func TestPingLossStatsFromPoints(t *testing.T) {
	stats := pingLossStatsFromPoints([]metric.AggregatePoint{
		{Value: 0.1, Count: 20},
		{Value: 0.25, Count: 4},
	})
	assert.Equal(t, int64(24), stats.Total)
	assert.Equal(t, int64(3), stats.Lost)
	assert.InDelta(t, 12.5, stats.LossRate(), 0.001)
}

func TestEvaluatePingLossNotification(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	notification := models.PingLossNotification{
		Enable:          true,
		LossThreshold:   5,
		MinimumSamples:  10,
		CooldownSeconds: 300,
	}
	stats := pingLossStats{Total: 20, Lost: 2}
	assert.Equal(t, pingLossNotificationAlert, evaluatePingLossNotification(notification, stats, now))

	lastNotified := now.Add(-time.Minute)
	notification.LastNotified = &lastNotified
	assert.Equal(t, pingLossNotificationAlert, evaluatePingLossNotification(notification, stats, now), "a new alert must not be delayed by stale notification history")

	notification.AlertActive = true
	assert.Equal(t, pingLossNotificationNone, evaluatePingLossNotification(notification, stats, now))

	lastNotified = now.Add(-10 * time.Minute)
	assert.Equal(t, pingLossNotificationAlert, evaluatePingLossNotification(notification, stats, now))

	stats = pingLossStats{Total: 9, Lost: 9}
	assert.Equal(t, pingLossNotificationNone, evaluatePingLossNotification(notification, stats, now))

	stats = pingLossStats{Total: 20, Lost: 1}
	assert.Equal(t, pingLossNotificationRecovery, evaluatePingLossNotification(notification, stats, now))

	notification.AlertActive = false
	assert.Equal(t, pingLossNotificationNone, evaluatePingLossNotification(notification, stats, now))
}

func TestFormatPingLossMessageIdentifiesExactTask(t *testing.T) {
	notification := models.PingLossNotification{
		Client:          "node-a",
		ClientInfo:      models.Client{Name: "Tokyo node"},
		TaskId:          17,
		Task:            models.PingTask{Name: "Cloudflare DNS", Target: "1.1.1.1"},
		WindowSeconds:   60,
		LossThreshold:   5,
		MinimumSamples:  1,
		CooldownSeconds: 300,
	}
	message := formatPingLossMessage(notification, pingLossStats{Total: 20, Lost: 2}, pingLossNotificationAlert)
	for _, expected := range []string{"Ping monitoring alert", "Tokyo node", "Ping task: Cloudflare DNS", "1.1.1.1", "10.00%", "2/20"} {
		if !strings.Contains(message, expected) {
			t.Fatalf("message %q does not contain %q", message, expected)
		}
	}
	assert.NotContains(t, message, "(#17)")
	assert.Equal(t, "\u5ef6\u8fdf\u76d1\u6d4b\u544a\u8b66", messageevent.PingLoss)
}

func TestFormatPingLossRecoveryMessage(t *testing.T) {
	notification := models.PingLossNotification{
		Client:        "node-a",
		ClientInfo:    models.Client{Name: "Ningbo server"},
		TaskId:        117,
		Task:          models.PingTask{Name: "Ningbo Telecom", Target: "example.com"},
		WindowSeconds: 60,
		LossThreshold: 5,
	}
	message := formatPingLossMessage(notification, pingLossStats{Total: 20, Lost: 1}, pingLossNotificationRecovery)
	assert.Contains(t, message, "Ping monitoring recovered")
	assert.Contains(t, message, "Ping task: Ningbo Telecom")
	assert.NotContains(t, message, "#117")
}

func TestFormatPingLossWindow(t *testing.T) {
	assert.Equal(t, "1 minute", formatPingLossWindow(60))
	assert.Equal(t, "2 hours", formatPingLossWindow(7200))
	assert.Equal(t, "90 seconds", formatPingLossWindow(90))
}
