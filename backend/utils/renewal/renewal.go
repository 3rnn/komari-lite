package renewal

import (
	"fmt"
	"time"

	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/models"
	messageevent "github.com/komari-monitor/komari/database/models/messageEvent"
	"github.com/komari-monitor/komari/pkg/timeutil"
	"github.com/komari-monitor/komari/utils/messageSender"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

func CheckAndAutoRenewal(client models.Client) {
	// Automatic renewal check
	//type renewedClient struct {
	//	Name          string
	//	NewExpireTime time.Time
	//}
	//var renewedClients []renewedClient

	if !client.AutoRenewal {
		return
	}
	// No renewal if not online
	if _, ok := agent_runtime.GetConnectedClients()[client.UUID]; !ok {
		return
	}
	if client.ExpiredAt == nil {
		return
	}

	clientExpireTime := client.ExpiredAt.UTC()
	checkTime := time.Now().UTC()

	// If the expiration time is less than 0002, skip
	if clientExpireTime.Year() < 2 {
		return
	}

	// Check if it has expired or expired today
	if clientExpireTime.Before(checkTime) || timeutil.SameSystemDate(clientExpireTime, checkTime) {
		// Calculate the total number of days from the expiration date to the creation time to determine whether it is a long-term bill
		now := checkTime
		localNow := now.In(time.Local)
		hundredYearsFromNow := localNow.AddDate(100, 0, 0).UTC()

		// If the expiration time exceeds 100 years from the current time, it will be regarded as a long-term/one-time bill and will not be renewed.
		if clientExpireTime.After(hundredYearsFromNow) {
			return
		}

		// If there is a billing cycle and it is not 0, automatic renewal will be performed.
		if client.BillingCycle > 0 {
			// Calculate new expiration time based on billing cycle
			var newExpireTime time.Time
			billingCycle := client.BillingCycle

			// If the server's expiration time is too early, set it directly to the next expiration time calculated from the current time.
			baseTime := clientExpireTime.In(time.Local)
			if clientExpireTime.Before(localNow.AddDate(0, 0, -30).UTC()) { // Expiration date is more than 30 days ago
				baseTime = localNow
			}

			if billingCycle >= 27 && billingCycle <= 32 {
				// Monthly billing - add 1 month
				newExpireTime = baseTime.AddDate(0, 1, 0)
			} else if billingCycle >= 87 && billingCycle <= 95 {
				// Quarterly billing - add 3 months
				newExpireTime = baseTime.AddDate(0, 3, 0)
			} else if billingCycle >= 175 && billingCycle <= 185 {
				// Half-year billing - add 6 months
				newExpireTime = baseTime.AddDate(0, 6, 0)
			} else if billingCycle >= 360 && billingCycle <= 370 {
				// Annual billing - add 1 year
				newExpireTime = baseTime.AddDate(1, 0, 0)
			} else if billingCycle >= 720 && billingCycle <= 750 {
				// Two years billed - add 2 years
				newExpireTime = baseTime.AddDate(2, 0, 0)
			} else if billingCycle >= 1080 && billingCycle <= 1150 {
				// Three years billed - add 3 years
				newExpireTime = baseTime.AddDate(3, 0, 0)
			} else if billingCycle >= 1800 && billingCycle <= 1850 {
				// Five years billed - add 5 years
				newExpireTime = baseTime.AddDate(5, 0, 0)
			} else {
				// In other cases, directly add the number of days in the billing cycle
				newExpireTime = baseTime.AddDate(0, 0, billingCycle)
			}

			// Update client expiration time
			updates := map[string]interface{}{
				"uuid":       client.UUID,
				"expired_at": newExpireTime.UTC(),
			}

			err := clients.SaveClient(updates)
			if err != nil {
				auditlog.EventLog("renewal", fmt.Sprintf("Failed to renew client %s (%s): %v", client.Name, client.UUID, err))
				return
			}

			//renewedClients = append(renewedClients, renewedClient{
			//	Name:          client.Name,
			//	NewExpireTime: newExpireTime,
			//})

			auditlog.EventLog("renewal", fmt.Sprintf("Auto-renewed client: %s until %s",
				client.Name, timeutil.FormatSystemDate(newExpireTime)))

			messageSender.SendEvent(models.EventMessage{
				Event:   messageevent.Renew,
				Clients: []models.Client{client},
				Time:    time.Now().UTC(),
				Emoji:   "🔄",
				Message: fmt.Sprintf("• %s until %s\n", client.Name, timeutil.FormatSystemDate(newExpireTime)),
			})
		}
	}

	// Send renewal notice
	// if len(renewedClients) > 0 {
	// 	message := ""
	// 	for _, clientInfo := range renewedClients {
	// 		message += fmt.Sprintf("• %s until %s\n", clientInfo.Name, clientInfo.NewExpireTime.Format("2006-01-02"))
	// 	}
	// 	messageSender.SendEvent(models.EventMessage{
	// 		Event:   messageevent.Renew,
	// 		Clients: []models.Client{client},
	// 		Time:    time.Now(),
	// 		Emoji:   "🔄",
	// 		Message: message,
	// 	})
	// }
}
