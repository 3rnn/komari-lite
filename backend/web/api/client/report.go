package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	logger "github.com/komari-monitor/komari/utils/log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	v1 "github.com/komari-monitor/komari/protocol/v1"
	"github.com/komari-monitor/komari/utils/notifier"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
	"github.com/komari-monitor/komari/web/api"
	"github.com/komari-monitor/komari/web/connection"
)

const (
	postPresenceTTL = 35 * time.Second
)

// postPresenceEntry tracks one client's HTTP POST reporting session.
type postPresenceEntry struct {
	connID     int64
	timer      *time.Timer
	generation uint64 // Each Reset increment is used to determine whether the old callback is expired in the callback
}

var (
	postPresenceMu     sync.Mutex
	postPresenceStates = make(map[string]*postPresenceEntry)
)

// refreshPostPresence tracks online/offline status for HTTP POST reporters.
// Each POST refreshes the TTL timer; expiry triggers an offline notification.
func refreshPostPresence(uuid string) {
	postPresenceMu.Lock()
	defer postPresenceMu.Unlock()

	if entry, exists := postPresenceStates[uuid]; exists {
		// Already online: increment generation to invalidate any old callbacks still running.
		entry.generation++
		entry.timer.Stop()
		// Recreate AfterFunc so the closure captures the new generation.
		gen := entry.generation
		entry.timer = time.AfterFunc(postPresenceTTL, func() {
			postPresenceExpired(uuid, entry.connID, gen)
		})
		agent_runtime.KeepAlivePresence(uuid, entry.connID, postPresenceTTL)
		return
	}

	// New POST session: generate connID, mark online, and start the timeout timer.
	connID := time.Now().UnixNano()
	agent_runtime.KeepAlivePresence(uuid, connID, postPresenceTTL)
	agent_runtime.SetClientProtocolVersion(uuid, 1)
	go notifier.OnlineNotification(uuid, connID)

	defaultGeneration := uint64(0)

	entry := &postPresenceEntry{
		connID:     connID,
		generation: defaultGeneration,
	}

	entry.timer = time.AfterFunc(postPresenceTTL, func() {
		postPresenceExpired(uuid, connID, defaultGeneration)
	})

	postPresenceStates[uuid] = entry
}

// postPresenceExpired runs when the timer expires.
// Clean up only when connID and generation still match the current entry,
// preventing timer.Reset races from removing active sessions.
func postPresenceExpired(uuid string, connID int64, gen uint64) {
	postPresenceMu.Lock()
	e, ok := postPresenceStates[uuid]
	if !ok || e.connID != connID || e.generation != gen {
		postPresenceMu.Unlock()
		return
	}
	delete(postPresenceStates, uuid)
	postPresenceMu.Unlock()

	agent_runtime.SetPresence(uuid, connID, false)
	notifier.OfflineNotification(uuid, connID)
}

func UploadReport(c *gin.Context) {
	bodyBytes, err := readBoundedReport(c.Request.Body)
	if err != nil {
		logger.ErrorArgs("client-api", "Failed to read request body:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var data map[string]interface{}
	err = json.Unmarshal(bodyBytes, &data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	// Save report to database
	var report v1.Report
	err = json.Unmarshal(bodyBytes, &report)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	// Use only the authenticated token for node identity; never trust a reported UUID.
	uuid, ok := clientUUIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	// Persist the POST report, update runtime state, and refresh presence.
	if err := ingestReport(uuid, report, 1, true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("%v", err)})
		return
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Restore the body for further use
	c.JSON(200, gin.H{"status": "success"})
}

func WebSocketReport(c *gin.Context) {
	// Upgrade to WebSocket.
	if !api.IsWebSocketUpgrade(c) {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Require WebSocket upgrade"})
		return
	}
	// Upgrade the HTTP connection to a WebSocket connection
	unsafeConn, err := api.UpgradeWebSocket(c, api.AllowAgentWebSocket)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Failed to upgrade to WebSocket." + err.Error()})
		return
	}
	conn := connection.NewSafeConn(unsafeConn)
	defer conn.Close()
	attachAgentWebSocketKeepalive(conn)

	_, message, err := conn.ReadMessage()
	if err != nil {
		logger.ErrorArgs("client-api", "Error reading message:", err)
		return
	}

	uuid, ok := clientUUIDFromContext(c)
	if !ok {
		conn.WriteJSON(gin.H{"status": "error", "error": "Invalid token"})
		return
	}

	// Accept new connections and handle old ones
	if oldConn, exists := agent_runtime.GetConnectedClients()[uuid]; exists {
		logger.Infof("client-api", "Client %s is reconnecting. Closing the old connection.", uuid)

		// Force-close the previous connection so its ReadMessage() loop exits.
		go oldConn.Close()
	}
	agent_runtime.SetConnectedClients(uuid, conn)
	logger.Infof("client-api", "Client %s is reconnect success, connID: %d", uuid, conn.ID)
	go notifier.OnlineNotification(uuid, conn.ID)
	defer func() {
		agent_runtime.DeleteClientConditionally(uuid, conn)
		notifier.OfflineNotification(uuid, conn.ID)
	}()

	// Process the first message received on the WebSocket connection.
	processMessage(conn, message, uuid)

	for {
		refreshAgentWebSocketReadDeadline(conn)

		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Errorf("client-api", "Client %s connection error: %v", uuid, err)
			}
			break // Any read errors (including timeouts) mean that the connection has been disconnected, exiting the loop
		}
		processMessage(conn, message, uuid)
	}
}

// Extract message processing into a reusable function.
func processMessage(conn *connection.SafeConn, message []byte, uuid string) {
	type MessageType struct {
		Type string `json:"type"`
	}
	var msgType MessageType
	err := json.Unmarshal(message, &msgType)
	if err != nil {
		conn.WriteJSON(gin.H{"status": "error", "error": "Invalid JSON"})
		return
	}

	switch msgType.Type {
	case "", "report":
		report := v1.Report{}
		err = json.Unmarshal(message, &report)
		if err != nil {
			conn.WriteJSON(gin.H{"status": "error", "error": "Invalid report format"})
			return
		}
		// WebSocket connections manage their own presence; do not refresh POST presence.
		if err := ingestReport(uuid, report, 1, false); err != nil {
			conn.WriteJSON(gin.H{"status": "error", "error": fmt.Sprintf("%v", err)})
			return
		}
	case "ping_result":
		var reqBody struct {
			PingTaskID uint   `json:"task_id"`
			PingResult int    `json:"value"`
			PingType   string `json:"ping_type"`
		}
		err = json.Unmarshal(message, &reqBody)
		if err != nil {
			conn.WriteJSON(gin.H{"status": "error", "error": "Invalid ping result format"})
			return
		}
		if err := ingestPingResult(uuid, reqBody.PingTaskID, reqBody.PingResult); err != nil {
			conn.WriteJSON(gin.H{"status": "error", "error": "Failed to save ping result"})
		}
	default:
		logger.Warnf("client-api", "Unknown message type: %s", msgType.Type)
		conn.WriteJSON(gin.H{"status": "error", "error": "Unknown message type"})
	}
}
