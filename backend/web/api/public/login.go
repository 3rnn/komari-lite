package public

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/utils"
	"github.com/komari-monitor/komari/web/api"
	"github.com/komari-monitor/komari/web/security"

	"github.com/gin-gonic/gin"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TwoFa    string `json:"2fa_code"`
}

const sessionCookieMaxAge = 2592000

func setSessionCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "session_token",
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   utils.GetScheme(c) == "https",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func Login(c *gin.Context) {
	// Throttle brute-force guessing on FAILED attempts only; see lite_rate.go.
	ip := clientKey(c)
	if !loginFails.allow(ip) {
		c.Header("Retry-After", "60")
		api.RespondError(c, http.StatusTooManyRequests, "Too many failed login attempts, try again later")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16384)
	DisablePasswordLogin, _ := config.GetAs[bool](config.DisablePasswordLoginKey, false)
	if DisablePasswordLogin {
		api.RespondError(c, http.StatusForbidden, "Password login is disabled")
		return
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		api.RespondError(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	var data LoginRequest
	err = json.Unmarshal(bodyBytes, &data)
	if err != nil {
		api.RespondError(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	if data.Username == "" || data.Password == "" {
		api.RespondError(c, http.StatusBadRequest, "Invalid request body: Username and password are required")
		return
	}

	uuid, success := accounts.CheckPassword(data.Username, data.Password)
	if !success {
		loginFails.fail(ip)
		api.RespondError(c, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	// 2FA
	user, _ := accounts.GetUserByUUID(uuid)
	if user.TwoFactor != "" { // 2FA turned on
		if data.TwoFa == "" {
			loginFails.fail(ip)
			api.RespondError(c, http.StatusUnauthorized, "2FA code is required")
			return
		}
		if ok, err := accounts.Verify2Fa(uuid, data.TwoFa); err != nil || !ok {
			loginFails.fail(ip)
			api.RespondError(c, http.StatusUnauthorized, "Invalid 2FA code")
			return
		}
	}
	// Create session
	session, err := accounts.CreateSession(uuid, sessionCookieMaxAge, c.Request.UserAgent(), c.ClientIP(), "password")
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "Failed to create session: "+err.Error())
		return
	}
	setSessionCookie(c, session, sessionCookieMaxAge)
	auditlog.Log(c.ClientIP(), uuid, "logged in (password)", "login")
	// The HttpOnly cookie is the only delivery channel for the bearer token.
	api.RespondSuccess(c, nil)
}
func PostLogout(c *gin.Context) {
	// Logout is a cookie-authenticated state change: CORS allowlists, API keys,
	// and a missing Origin must not authorize a cross-site POST.
	origin := c.GetHeader("Origin")
	parsed, err := url.Parse(origin)
	if origin == "" || err != nil || parsed.Host == "" || parsed.Scheme != utils.GetScheme(c) ||
		parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" ||
		parsed.Opaque != "" || strings.TrimSpace(origin) != origin ||
		!security.OriginMatchesRequest(origin, c.Request) {
		api.RespondError(c, http.StatusForbidden, "Origin not allowed")
		return
	}
	session, err := c.Cookie("session_token")
	if err != nil || session == "" {
		api.RespondError(c, http.StatusUnauthorized, "Unauthorized.")
		return
	}
	if _, err := accounts.GetSession(session); err != nil {
		api.RespondError(c, http.StatusUnauthorized, "Unauthorized.")
		return
	}
	if err := accounts.DeleteSession(session); err != nil {
		api.RespondError(c, http.StatusInternalServerError, "Failed to delete session")
		return
	}
	setSessionCookie(c, "", -1)
	auditlog.Log(c.ClientIP(), "", "logged out", "logout")
	api.RespondSuccess(c, nil)
}

func Logout(c *gin.Context) {
	session, _ := c.Cookie("session_token")
	accounts.DeleteSession(session)
	setSessionCookie(c, "", -1)
	auditlog.Log(c.ClientIP(), "", "logged out", "logout")
	c.Redirect(302, "/")
}
