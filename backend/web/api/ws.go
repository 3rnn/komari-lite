package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/database/trafficledger"
	"github.com/komari-monitor/komari/protocol/v1"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

const clientStatusWriteTimeout = 10 * time.Second

// Browser WebSocket implementations answer control pings even when polling is paused.
func startClientStatusHeartbeat(conn *websocket.Conn, idleTimeout, pingInterval, writeTimeout time.Duration) func() {
	_ = conn.SetReadDeadline(time.Now().Add(idleTimeout))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(idleTimeout)) })
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeTimeout)); err != nil {
					_ = conn.Close()
					return
				}
			}
		}
	}()
	return func() { close(stop); <-done }
}

func GetClients(c *gin.Context) {
	// Upgrade to ws
	if !IsWebSocketUpgrade(c) {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Require WebSocket upgrade"})
		return
	}
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := UpgradeWebSocket(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Failed to upgrade to WebSocket." + err.Error()})
		return
	}
	defer conn.Close()
	stopHeartbeat := startClientStatusHeartbeat(conn, 90*time.Second, 30*time.Second, clientStatusWriteTimeout)
	defer stopHeartbeat()

	// Retain the original credential, never the handshake authorization snapshot.
	session, _ := c.Cookie("session_token")

	// Request
	for {
		var resp struct {
			Online []string             `json:"online"` // List of connected client uuids
			Data   map[string]v1.Report `json:"data"`   // Last reported data
		}

		resp.Online = []string{}
		resp.Data = map[string]v1.Report{}

		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		message := string(data)

		uuID := ""
		if message != "get" { // Not requesting all content
			if strings.HasPrefix(message, "get ") {
				uuID = strings.TrimSpace(strings.TrimPrefix(message, "get "))
			} else {
				_ = conn.SetWriteDeadline(time.Now().Add(clientStatusWriteTimeout))
				if err := conn.WriteJSON(gin.H{"status": "error", "error": "Invalid message"}); err != nil {
					return
				}
				continue
			}
		}

		// Resolve session expiry/revocation and account existence on every request.
		isLogin := false
		if session != "" {
			if uuid, err := accounts.GetSession(session); err == nil && uuid != "" {
				_, err = accounts.GetUserByUUID(uuid)
				isLogin = err == nil
			}
		}
		hiddenMap := map[string]bool{}
		if !isLogin {
			var hiddenClients []models.Client
			if err := dbcore.GetDBInstance().WithContext(c.Request.Context()).Select("uuid").Where("hidden = ?", true).Find(&hiddenClients).Error; err != nil {
				// A failed visibility lookup must not expose an unfiltered snapshot.
				_ = conn.SetWriteDeadline(time.Now().Add(clientStatusWriteTimeout))
				if err := conn.WriteJSON(gin.H{"status": "error", "error": "Failed to resolve client visibility"}); err != nil {
					return
				}
				continue
			}
			for _, cli := range hiddenClients {
				hiddenMap[cli.UUID] = true
			}
		}

		// List of online client uuids (WebSocket vs. non-WebSocket)
		online := []string{}
		reports := map[string]*v1.Report{}
		if uuID == "" {
			online = agent_runtime.GetAllOnlineUUIDs()
			reports = agent_runtime.GetLatestReport()
		} else {
			if agent_runtime.IsClientOnline(uuID) {
				online = append(online, uuID)
			}
			if report := agent_runtime.GetLatestReportByUUID(uuID); report != nil {
				reports[uuID] = report
			}
		}
		for _, key := range online {
			if !isLogin && hiddenMap[key] {
				continue
			}
			if uuID != "" && key != uuID {
				continue
			}
			resp.Online = append(resp.Online, key)
		}

		//Past Node Data Information
		calibrated, _ := trafficledger.CurrentCalibratedCycleUsages(
			c.Request.Context(), dbcore.GetDBInstance(), time.Now().UTC(),
		)
		for key, report := range reports {
			if !isLogin && hiddenMap[key] {
				continue
			}
			if uuID != "" && key != uuID {
				continue
			}

			report.UUID = "" // Do not expose uuid
			if report.CPU.Usage == 0 {
				report.CPU.Usage = 0.01
			}
			if usage, ok := calibrated[key]; ok {
				report.Network.TotalUp = usage.Up
				report.Network.TotalDown = usage.Down
			}
			resp.Data[key] = *report
		}

		_ = conn.SetWriteDeadline(time.Now().Add(clientStatusWriteTimeout))
		err = conn.WriteJSON(gin.H{"status": "success", "data": resp})
		if err != nil {
			return
		}
	}
}
