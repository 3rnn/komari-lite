package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	v1 "github.com/komari-monitor/komari/protocol/v1"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

func TestClientStatusRefreshesSessionAndVisibility(t *testing.T) {
	flags.DatabaseType = flags.DatabaseTypeSQLite
	flags.DatabaseFile = "file:client_status_auth?mode=memory&cache=shared"
	db := dbcore.GetDBInstance()
	gin.SetMode(gin.TestMode)
	publicNode := models.Client{UUID: "ws-status-public", Token: "test-only-public-node"}
	if err := db.Create(&publicNode).Error; err != nil {
		t.Fatal(err)
	}
	agent_runtime.RecordReport(v1.Report{UUID: publicNode.UUID, UpdatedAt: time.Now(), Uptime: 24})
	agent_runtime.KeepAlivePresence(publicNode.UUID, 1, time.Hour)
	t.Cleanup(func() {
		db.Where("uuid = ?", publicNode.UUID).Delete(&models.Client{})
		agent_runtime.DeleteConnectedClients(publicNode.UUID)
	})
	for _, kind := range []string{"revoked", "expired", "deleted account", "already expired", "newly hidden", "hidden lookup failure"} {
		t.Run(kind, func(t *testing.T) {
			user := models.User{UUID: "ws-auth-" + kind, Username: "ws-auth-" + kind}
			if err := db.Create(&user).Error; err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { db.Where("uuid = ?", user.UUID).Delete(&models.User{}) })
			token := "test-only-status-" + kind
			session := models.Session{UUID: user.UUID, Session: token, Expires: time.Now().Add(time.Hour)}
			if kind == "already expired" {
				session.Expires = time.Now().Add(-time.Hour)
			}
			if err := db.Create(&session).Error; err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { accounts.DeleteSession(token) })
			node := models.Client{UUID: "ws-hidden-" + kind, Token: "test-only-node-" + kind, Hidden: kind != "newly hidden"}
			if err := db.Create(&node).Error; err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				db.Where("uuid = ?", node.UUID).Delete(&models.Client{})
				agent_runtime.DeleteConnectedClients(node.UUID)
			})
			agent_runtime.RecordReport(v1.Report{UUID: node.UUID, UpdatedAt: time.Now(), Uptime: 42})
			agent_runtime.KeepAlivePresence(node.UUID, 1, time.Hour)
			router := gin.New()
			// Exercise expiry in the handler itself, not middleware cleanup.
			if kind != "already expired" {
				router.Use(IdentityMiddleware())
			}
			router.GET("/ws", GetClients)
			server := httptest.NewServer(router)
			defer server.Close()
			headers := http.Header{"Origin": {server.URL}}
			if kind != "newly hidden" && kind != "hidden lookup failure" {
				headers.Set("Cookie", (&http.Cookie{Name: "session_token", Value: token}).String())
			}
			conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws", headers)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			lookupFailed := false
			call := func(message string) bool {
				t.Helper()
				conn.SetReadDeadline(time.Now().Add(3 * time.Second))
				if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
					t.Fatal(err)
				}
				var response struct {
					Status string `json:"status"`
					Data   struct {
						Online []string             `json:"online"`
						Data   map[string]v1.Report `json:"data"`
					} `json:"data"`
				}
				if err := conn.ReadJSON(&response); err != nil {
					t.Fatal(err)
				}
				if !lookupFailed {
					if response.Status != "success" {
						t.Fatal("valid status request rejected")
					}
					if message == "get" {
						if _, ok := response.Data.Data[publicNode.UUID]; !ok {
							t.Fatal("public status disappeared after authorization change")
						}
					}
				} else if response.Status != "error" {
					t.Fatal("failed visibility lookup did not fail closed")
				}
				_, visible := response.Data.Data[node.UUID]
				for _, id := range response.Data.Online {
					if id == node.UUID && !visible {
						t.Fatal("online list exposed a filtered node")
					}
				}
				return visible
			}
			visible := call("get")
			if kind == "already expired" {
				if visible {
					t.Fatal("expired session exposed hidden status")
				}
				return
			}
			if kind == "hidden lookup failure" {
				if visible {
					t.Fatal("anonymous initial hidden status exposed")
				}
				// A failed visibility query must not turn an empty deny list into access.
				if err := db.Exec("ALTER TABLE clients RENAME COLUMN hidden TO hidden_unavailable").Error; err != nil {
					t.Fatal(err)
				}
				lookupFailed = true
				defer db.Exec("ALTER TABLE clients RENAME COLUMN hidden_unavailable TO hidden")
			} else {
				if !visible {
					t.Fatal("live session/public node denied")
				}
				switch kind {
				case "revoked":
					err = accounts.DeleteSession(token)
				case "expired":
					err = db.Model(&models.Session{}).Where("session = ?", token).Update("expires", time.Now().Add(-time.Minute)).Error
				case "deleted account":
					err = db.Where("uuid = ?", user.UUID).Delete(&models.User{}).Error
				case "newly hidden":
					err = db.Model(&models.Client{}).Where("uuid = ?", node.UUID).Update("hidden", true).Error
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if call("get " + node.UUID) {
				t.Fatal("stale authorization exposed hidden status")
			}
			if call("get") {
				t.Fatal("stale authorization exposed hidden status in full snapshot")
			}
		})
	}
}
