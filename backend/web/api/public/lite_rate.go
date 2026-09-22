package public

import (
	"net"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// loginFailWindow throttles FAILED password login attempts only.
//
// Design notes:
//   - Successful logins never consume budget, so an administrator cannot lock
//     themselves out by logging in normally (upstream shipped no limiter at all).
//   - Two caps apply: per-client-IP (stops targeted guessing) and a global cap
//     (stops a botnet that rotates IPs from getting unlimited attempts).
//   - Fixed one-minute window; the per-IP map is replaced on rollover, so it
//     cannot grow without bound.
const (
	loginWindowSize = time.Minute
	loginMaxPerIP   = 5
	// Global cap is a resource guard, not a lockout: password hashing only runs
	// for existing usernames, so the real cost of a distributed flood is bounded.
	// It is deliberately far above per-IP so a botnet cannot lock the owner out.
	loginMaxGlobal = 60
)

type loginFailWindow struct {
	sync.Mutex
	start time.Time
	byIP  map[string]int
	total int
}

var loginFails = &loginFailWindow{byIP: make(map[string]int)}

func (w *loginFailWindow) rollLocked() {
	if w.start.IsZero() || time.Since(w.start) >= loginWindowSize {
		w.start = time.Now()
		w.byIP = make(map[string]int)
		w.total = 0
	}
}

func (w *loginFailWindow) allow(ip string) bool {
	w.Lock()
	defer w.Unlock()
	w.rollLocked()
	return w.byIP[ip] < loginMaxPerIP && w.total < loginMaxGlobal
}

func (w *loginFailWindow) fail(ip string) {
	w.Lock()
	defer w.Unlock()
	w.rollLocked()
	w.byIP[ip]++
	w.total++
}

// clientKey returns the client IP without a port, so the limiter cannot be
// bypassed by opening new TCP connections from the same host.
func clientKey(c *gin.Context) string {
	ip := c.ClientIP()
	if ip == "" {
		return "unknown"
	}
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	return ip
}
