package httpsserver

import (
	"net/http"
	"time"
)

// NewHTTPServer bounds inbound headers and bodies, but deliberately leaves
// writes unbounded for MJPEG and other long-running responses. WebSocket
// upgrades clear HTTP deadlines and install their own heartbeat deadlines.
func NewHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 15 * time.Second,
		ReadTimeout:       5 * time.Minute,
		IdleTimeout:       90 * time.Second,
	}
}
