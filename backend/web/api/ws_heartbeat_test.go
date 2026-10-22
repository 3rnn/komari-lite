package api

import (
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStatusHeartbeatKeepsResponsiveIdleBrowserAndExpiresSilentPeer(t *testing.T) {
	for _, responsive := range []bool{true, false} {
		name := "silent"
		if responsive {
			name = "responsive"
		}
		t.Run(name, func(t *testing.T) {
			exited := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()
				defer close(exited)
				stop := startClientStatusHeartbeat(conn, 150*time.Millisecond, 30*time.Millisecond, 50*time.Millisecond)
				defer stop()
				for {
					if _, _, err := conn.ReadMessage(); err != nil {
						return
					}
				}
			}))
			defer server.Close()
			conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if responsive {
				go func() {
					for {
						if _, _, err := conn.ReadMessage(); err != nil {
							return
						}
					}
				}()
			}
			if responsive {
				select {
				case <-exited:
					t.Fatal("responsive idle browser expired")
				case <-time.After(400 * time.Millisecond):
				}
			} else {
				select {
				case <-exited:
				case <-time.After(time.Second):
					t.Fatal("silent peer did not expire")
				}
			}
		})
	}
}
