package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/pquerna/otp/totp"
)

func TestEnable2FARejectsClientSuppliedSecret(t *testing.T) {
	flags.DatabaseFile = "file:admin_twofactor_security?mode=memory&cache=shared"
	gin.SetMode(gin.TestMode)
	user, err := accounts.CreateAccount("pending-factor-user", "test-only-password")
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
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Set("uuid", user.UUID)
	c.Request = httptest.NewRequest(http.MethodPost, "/enable?code="+code, nil)
	c.Request.AddCookie(&http.Cookie{Name: "session_token", Value: session})
	c.Request.AddCookie(&http.Cookie{Name: "2fa_secret", Value: secret})
	Enable2FA(c)
	if response.Code == http.StatusOK {
		t.Fatal("client-supplied enrollment secret was accepted")
	}
	stored, err := accounts.GetUserByUUID(user.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.TwoFactor != "" {
		t.Fatal("untrusted enrollment changed stored factor")
	}
}
