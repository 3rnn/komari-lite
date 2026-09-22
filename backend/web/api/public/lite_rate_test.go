package public

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func resetLoginFails() {
	loginFails.Lock()
	defer loginFails.Unlock()
	loginFails.start = loginFails.start.Add(-2 * loginWindowSize)
	loginFails.byIP = make(map[string]int)
	loginFails.total = 0
}

func postLogin(router *gin.Engine, body string, remoteAddr string) int {
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
	req.RemoteAddr = remoteAddr
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w.Code
}

// Failed attempts from one IP are capped; the limiter must eventually answer 429.
func TestLiteLoginRatePerIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetLoginFails()
	defer resetLoginFails()

	router := gin.New()
	router.POST("/api/login", Login)

	const addr = "203.0.113.7:40000"
	got429 := false
	for i := 0; i < loginMaxPerIP+3; i++ {
		if postLogin(router, `{"username":"nobody","password":"wrong"}`, addr) == 429 {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Fatalf("want rate limited after %d failed attempts from one IP", loginMaxPerIP)
	}
}

// A different IP must stay usable, and the global cap must trip once many IPs fail.
func TestLiteLoginRateGlobal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetLoginFails()
	defer resetLoginFails()

	router := gin.New()
	router.POST("/api/login", Login)

	if code := postLogin(router, `{"username":"nobody","password":"wrong"}`, "198.51.100.9:5000"); code != 401 {
		t.Fatalf("first failed attempt should be 401, got %d", code)
	}
	if code := postLogin(router, `{"username":"nobody","password":"wrong"}`, "198.51.100.10:5000"); code != 401 {
		t.Fatalf("second IP should still be allowed, got %d", code)
	}

	// Exhaust the global budget from many distinct source IPs (5 failures each).
	for i := 0; i < 30; i++ {
		addr := "198.51.100." + strconv.Itoa(10+i) + ":6000"
		for j := 0; j < loginMaxPerIP; j++ {
			postLogin(router, `{"username":"nobody","password":"wrong"}`, addr)
		}
	}
	if code := postLogin(router, `{"username":"nobody","password":"wrong"}`, "192.0.2.200:7000"); code != 429 {
		t.Fatalf("global cap should return 429 for a fresh IP, got %d", code)
	}
}

// A malformed body must not consume the failure budget: it is not credential guessing.
func TestLiteLoginBadBodyNotCounted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetLoginFails()
	defer resetLoginFails()

	router := gin.New()
	router.POST("/api/login", Login)

	for i := 0; i < loginMaxPerIP+3; i++ {
		if code := postLogin(router, `not-json`, "192.0.2.55:8000"); code == 429 {
			t.Fatalf("malformed bodies should not trip the credential limiter (attempt %d)", i+1)
		}
	}
}
