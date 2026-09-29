package notifier

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/database/trafficledger"
	"github.com/komari-monitor/komari/pkg/config"
	logger "github.com/komari-monitor/komari/utils/log"
	"github.com/komari-monitor/komari/utils/messageSender"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
	cache "github.com/patrickmn/go-cache"
)

// trafficCache is used to record the threshold steps triggered by each client to avoid repeated reminders
// key: "traffic:"+clientUUID, value: trafficReminderState
var trafficCache = cache.New(30*24*time.Hour, time.Hour) // 30 days cache, 1 hour cleanup

type trafficUsageSnapshot struct {
	Used  int64
	Limit int64
	Type  string
}

type trafficReminderState struct {
	Step  int
	Limit int64
	Type  string
}

func currentTrafficUsage(client models.Client, up, down int64, now time.Time) trafficUsageSnapshot {
	limit, typeName := clients.EffectiveTrafficLimit(client, now)
	return trafficUsageSnapshot{
		Used:  computeUsedByType(typeName, up, down),
		Limit: limit,
		Type:  typeName,
	}
}

// CheckTraffic checks the traffic usage of each client and reminds you once when the threshold is reached and every +5%; an additional reminder when it reaches 100%
// Called once every minute by an external coroutine
func CheckTraffic() {
	// Get the latest reports and client configurations
	reports := agent_runtime.GetLatestReport()
	if len(reports) == 0 {
		return
	}
	cfg, err := config.GetAs[float64](config.TrafficLimitPercentageKey, 80.0)
	if err != nil {
		logger.Error("notifier", "failed to get traffic limit percentage", "error", err)
	}

	if cfg <= 0 {
		return
	}

	// Starting threshold: for example 80%, non-multiples of 5 are rounded up to the nearest multiple of 5, for example 83->85
	startThreshold := cfg
	if startThreshold < 0 {
		startThreshold = 0
	}
	baseStep := int(math.Ceil(startThreshold/5.0) * 5.0)
	if baseStep > 100 {
		baseStep = 100
	}

	allClients, err := clients.GetAllClientBasicInfo()
	if err != nil {
		return
	}

	now := time.Now().UTC()
	calibrated, err := trafficledger.CurrentCalibratedCycleUsages(context.Background(), dbcore.GetDBInstance(), now)
	if err != nil {
		logger.Error("notifier", "failed to read calibrated traffic usage", "error", err)
		calibrated = map[string]trafficledger.Usage{}
	}
	for _, c := range allClients {
		r, ok := reports[c.UUID]
		if !ok || r == nil {
			continue
		}

		up, down := r.Network.TotalUp, r.Network.TotalDown
		if value, ok := calibrated[c.UUID]; ok {
			up, down = value.Up, value.Down
		}
		usage := currentTrafficUsage(c, up, down, now)
		if usage.Limit <= 0 || usage.Used <= 0 {
			continue
		}

		pct := float64(usage.Used) / float64(usage.Limit) * 100.0
		key := "traffic:" + c.UUID
		last, _ := trafficCache.Get(key)
		state, _ := last.(trafficReminderState)
		if state.Limit != usage.Limit || state.Type != usage.Type {
			state = trafficReminderState{Limit: usage.Limit, Type: usage.Type}
			trafficCache.SetDefault(key, state)
		}
		if pct < startThreshold {
			continue
		}

		// Current threshold step (multiple of 5%)
		curStep := int(math.Floor(pct/5.0) * 5.0)
		if curStep < baseStep {
			curStep = baseStep
		}
		// if curStep > 100 {
		// 	curStep = 100
		// }

		// Fix: When it is detected that the current progress is less than the historical record, it means that the traffic has been reset and the baseline is reset to zero
		if curStep < state.Step {
			state.Step = 0
		}

		if curStep > state.Step { // Only remind once when entering a new step
			state.Step = curStep
			trafficCache.SetDefault(key, state)

			msg := fmt.Sprintf("used %d%% (%s / %s), type=%s", curStep, humanBytes(usage.Used), humanBytes(usage.Limit), usage.Type)
			// Send notification (NotificationEnabled is checked internally)
			_ = messageSender.SendEvent(models.EventMessage{
				Event:   "Traffic",
				Clients: []models.Client{c},
				Time:    time.Now().UTC(),
				Emoji:   "⚠️",
				Message: msg,
			})
		}
	}
}

func computeUsedByType(t string, up, down int64) int64 {
	return trafficledger.BillableUsage(t, up, down)
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	// KMGTPE
	prefixes := []string{"K", "M", "G", "T", "P", "E"}
	if exp >= len(prefixes) {
		exp = len(prefixes) - 1
	}
	return fmt.Sprintf("%.2f %sB", float64(b)/float64(div), prefixes[exp])
}
