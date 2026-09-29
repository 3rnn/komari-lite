package notifier

import (
	"fmt"
	logger "github.com/komari-monitor/komari/utils/log"
	"sync"
	"time"

	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	messageevent "github.com/komari-monitor/komari/database/models/messageEvent"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/utils/messageSender"
	"github.com/komari-monitor/komari/utils/renewal"
)

// notificationState saves the notification status of a single client.
// By embedding a mutex lock in the structure, fine-grained locking of each client is achieved, which is more efficient than global locks.
type notificationState struct {
	mu                  sync.Mutex // Mutex lock to protect the client state
	pendingOfflineSince time.Time  // The time the client was offline. A value of zero indicates that the client is online or has sent an offline notification.
	isFirstConnection   bool       // Mark whether it is the first online connection.
	isConnExist         bool       // Mark whether there is a connection
	connectionID        int64      // Connection ID, used to distinguish different connection sessions and prevent race conditions
}

// clientStates uses sync.Map to implement concurrent access to client state.
// Mapping relationship: clientID (string) -> *notificationState
var clientStates sync.Map

func ForgetClient(clientID string) {
	value, exists := clientStates.LoadAndDelete(clientID)
	if !exists {
		return
	}
	state := value.(*notificationState)
	state.mu.Lock()
	state.pendingOfflineSince = time.Time{}
	state.isConnExist = false
	state.connectionID++
	state.mu.Unlock()
}

// getNotificationConfig Gets the notification configuration of the specified client.
// Returns the configuration object and a boolean indicating whether notifications are enabled globally and for this client.
func getNotificationConfig(clientID string) (*models.OfflineNotification, bool) {
	conf, err := config.GetAs[bool](config.NotificationEnabledKey, false)
	if err != nil || !conf {
		return nil, false
	}

	notiConf := models.OfflineNotification{Client: clientID}
	db := dbcore.GetDBInstance()
	if err := db.Model(&models.OfflineNotification{}).Where("client = ?", clientID).FirstOrCreate(&notiConf).Error; err != nil {
		logger.Errorf("notifier", "Failed to get or create offline notification config for client %s: %v", clientID, err)
		return nil, false
	}

	return &notiConf, notiConf.Enable
}

// getOrInitState gets the client state from sync.Map. If it does not exist, create it and store it.
func getOrInitState(clientID string) *notificationState {
	// Atomicly load or store this client's state.
	val, _ := clientStates.LoadOrStore(clientID, &notificationState{isFirstConnection: true})
	return val.(*notificationState)
}

func beginOfflineGrace(state *notificationState, endedConnectionID int64, now time.Time) bool {
	state.mu.Lock()
	defer state.mu.Unlock()

	// Only the first offline event for the current connection is accepted. Delay events for old connections must be ignored.
	if !state.pendingOfflineSince.IsZero() || state.connectionID != endedConnectionID {
		return false
	}
	state.pendingOfflineSince = now
	return true
}

func recordOnlineConnection(state *notificationState, connectionID int64) (notifyOnline, duplicate bool) {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.connectionID = connectionID
	if state.isFirstConnection {
		state.isFirstConnection = false
		state.pendingOfflineSince = time.Time{}
		state.isConnExist = true
		return false, false
	}

	wasPending := !state.pendingOfflineSince.IsZero()
	state.pendingOfflineSince = time.Time{}
	if wasPending {
		return false, false
	}

	if state.isConnExist {
		return false, true
	}
	state.isConnExist = true
	return true, false
}

// OfflineNotification Sends a client offline notification when notifications are enabled and not sent within the grace period.
func OfflineNotification(clientID string, endedConnectionID int64) {
	client, err := clients.GetClientByUUID(clientID)
	if err != nil {
		return
	}

	notiConf, enabled := getNotificationConfig(clientID)
	if !enabled {
		return
	}

	gracePeriod := time.Duration(notiConf.GracePeriod) * time.Second
	if gracePeriod <= 0 {
		gracePeriod = 5 * time.Minute // Default grace period
	}

	now := time.Now().UTC()
	state := getOrInitState(clientID)
	if !beginOfflineGrace(state, endedConnectionID, now) {
		return
	}

	// Create a new coroutine and wait for the grace period to determine whether notification needs to be sent.
	go func(startTime time.Time, expectedConnectionID int64) {
		time.Sleep(gracePeriod)

		state.mu.Lock()
		defer state.mu.Unlock()

		// Check whether the offline status is still the status when this coroutine was started.
		// If it is zero, it means the client has reconnected.
		// Is the current connectionID still the same ID when we triggered offline. If not, it means that the client has reconnected and this offline notification has expired.
		if state.pendingOfflineSince.IsZero() || state.connectionID != expectedConnectionID {
			logger.Infof("notifier", "%s is reconnected new connID: %d, old connID: %d", clientID, state.connectionID, expectedConnectionID)
			return
		}

		// Notification is about to be sent, reset pending notification status.
		// One more boolean is needed because pendingOfflineSince is modified after offline sleep, which may lead to incorrect online judgment.
		state.pendingOfflineSince = time.Time{}
		state.isConnExist = false

		// Send notification
		message := fmt.Sprintf("🔴%s is offline", client.Name)
		go func(msg string) {
			if err := messageSender.SendEvent(models.EventMessage{
				Event:   messageevent.Offline,
				Clients: []models.Client{client},
				Time:    time.Now().UTC(),
				//Message: msg,
				Emoji: "🔴",
			}); err != nil {
				logger.ErrorArgs("notifier", "Failed to send offline notification:", err)
			}
		}(message)

		// Update the last notification time in the database
		db := dbcore.GetDBInstance()
		if err := db.Model(&models.OfflineNotification{}).Where("client = ?", clientID).Update("last_notified", now.UTC()).Error; err != nil {
			logger.Errorf("notifier", "Failed to update last_notified for client %s: %v", clientID, err)
		}
	}(now, endedConnectionID)
}

// OnlineNotification Sends client online notification when notifications are enabled.
func OnlineNotification(clientID string, connectionID int64) {
	client, err := clients.GetClientByUUID(clientID)
	if err != nil {
		return
	}
	// Detect renewal when going online
	renewal.CheckAndAutoRenewal(client)

	// The connection state must be maintained at all times. The notification switch only controls message sending, not the current connection ID.
	// Missing, otherwise the first disconnection after enabling notification when the node is online will be misjudged as an old connection event.
	state := getOrInitState(clientID)
	notifyOnline, duplicate := recordOnlineConnection(state, connectionID)

	_, enabled := getNotificationConfig(clientID)
	if !enabled {
		return
	}
	if duplicate {
		logger.Infof("notifier", "%s has connection exist: %d", clientID, connectionID)
		return
	}
	if !notifyOnline {
		return
	}

	// Rule 4: The client has been offline for long enough and has been notified (or has not yet been offline). Now it comes back online and an online notification is sent.
	message := fmt.Sprintf("🟢%s is online", client.Name)
	go func(msg string) {
		if err := messageSender.SendEvent(models.EventMessage{
			Event:   messageevent.Online,
			Clients: []models.Client{client},
			Time:    time.Now().UTC(),
			//Message: msg,
			Emoji: "🟢",
		}); err != nil {
			logger.ErrorArgs("notifier", "Failed to send online notification:", err)
		}
	}(message)
}
