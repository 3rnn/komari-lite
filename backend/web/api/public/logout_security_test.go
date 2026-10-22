package public

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
)

func TestLegacyGetLogoutRequiresConfirmationWithoutMutation(t *testing.T) {
	token, err := accounts.CreateSession("logout-confirmation-user", 3600, "", "", "password")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { accounts.DeleteSession(token) })
	router := gin.New()
	router.GET("/api/logout", Logout)
	router.POST("/api/logout", PostLogout)
	request := httptest.NewRequest(http.MethodGet, "https://monitor.example/api/logout", nil)
	request.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("confirmation status = %d", response.Code)
	}
	if _, err := accounts.GetSession(token); err != nil {
		t.Fatal("GET revoked a session")
	}
	if len(response.Header().Values("Set-Cookie")) != 0 {
		t.Fatal("GET changed session cookies")
	}
	if !strings.Contains(response.Body.String(), `method="post"`) || !strings.Contains(response.Body.String(), `action="/api/logout"`) {
		t.Fatal("missing explicit POST confirmation")
	}
	if strings.Contains(response.Body.String(), token) {
		t.Fatal("confirmation leaked bearer token")
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("confirmation is cacheable")
	}
	if !strings.Contains(response.Header().Get("Content-Security-Policy"), "form-action 'self'") || response.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("confirmation lacks same-origin form or framing protection")
	}
	for _, origin := range []string{"https://evil.example", "https://monitor.example"} {
		confirmation := httptest.NewRequest(http.MethodPost, "https://monitor.example/api/logout", nil)
		confirmation.Header.Set("Origin", origin)
		confirmation.AddCookie(&http.Cookie{Name: "session_token", Value: token})
		result := httptest.NewRecorder()
		router.ServeHTTP(result, confirmation)
		_, sessionErr := accounts.GetSession(token)
		if origin == "https://evil.example" {
			if result.Code != http.StatusForbidden || sessionErr != nil || len(result.Header().Values("Set-Cookie")) != 0 {
				t.Fatal("cross-origin confirmation mutated session state")
			}
		} else if result.Code != http.StatusOK || sessionErr == nil {
			t.Fatal("same-origin confirmation did not log out")
		}
	}
}
