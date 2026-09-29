package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/database/trafficledger"
	"github.com/komari-monitor/komari/protocol/v1"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

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

	// Initialize user information
	var (
		isLogin    = false
		hiddenMap  = map[string]bool{}
		session, _ = c.Cookie("session_token")
	)

	// Login status check
	_, err = accounts.GetUserBySession(session)
	if err == nil {
		isLogin = true
	}

	// Hidden information only needs to be filtered when not logged in
	if !isLogin {
		var hiddenClients []models.Client
		db := dbcore.GetDBInstance()
		_ = db.Select("uuid").Where("hidden = ?", true).Find(&hiddenClients).Error
		for _, cli := range hiddenClients {
			hiddenMap[cli.UUID] = true
		}
	}

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
		message := string(data)

		uuID := ""
		if message != "get" { // Not requesting all content
			if strings.HasPrefix(message, "get ") {
				uuID = strings.TrimSpace(strings.TrimPrefix(message, "get "))
			} else {
				conn.WriteJSON(gin.H{"status": "error", "error": "Invalid message"})
				continue
			}
		}

		// List of online client uuids (WebSocket vs. non-WebSocket)
		for _, key := range agent_runtime.GetAllOnlineUUIDs() {
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
		for key, report := range agent_runtime.GetLatestReport() {
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

		err = conn.WriteJSON(gin.H{"status": "success", "data": resp})
		if err != nil {
			return
		}
	}
}
