package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/pquerna/otp/totp"
)

func TestGenerate2FAReplacementAcceptsHeaderProofWithoutChangingFactor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user, err := accounts.CreateAccount(t.Name(), "test-only-password")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { accounts.DeleteAccountByUsername(user.Username) })
	session, err := accounts.CreateSession(user.UUID, 3600, "", "", "password")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { accounts.DeleteSession(session) })
	secret, _, err := accounts.Generate2Fa()
	if err != nil {
		t.Fatal(err)
	}
	if err := accounts.Enable2Fa(user.UUID, secret); err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, proof := range []string{"", "not-a-code", code} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("uuid", user.UUID)
		c.Request = httptest.NewRequest(http.MethodGet, "/generate", nil)
		c.Request.AddCookie(&http.Cookie{Name: "session_token", Value: session})
		c.Request.Header.Set("X-2FA-Code", proof)
		Generate2FA(c)
		if proof != code {
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("unproved generation status = %d", w.Code)
			}
		} else {
			if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/png" {
				t.Fatalf("proved generation status = %d", w.Code)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("QR response is cacheable")
			}
			if len(w.Result().Cookies()) != 0 {
				t.Fatal("generation exposed a pending secret in a cookie")
			}
		}
		current, err := accounts.GetUserByUUID(user.UUID)
		if err != nil {
			t.Fatal(err)
		}
		if current.TwoFactor != secret {
			t.Fatal("QR generation changed the live factor")
		}
	}
}
