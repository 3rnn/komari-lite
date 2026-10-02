package public

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
)

func TestLogin(t *testing.T) {
	// Set Test Mode
	gin.SetMode(gin.TestMode)
	accounts.CreateAccount("testuser", "correctpassword")
	tests := []struct {
		name           string
		requestBody    LoginRequest
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "successful login",
			requestBody: LoginRequest{
				Username: "testuser",
				Password: "correctpassword",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "invalid request body",
			requestBody: LoginRequest{
				Username: "",
				Password: "",
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"status":  "error",
				"message": "Invalid request body: Username and password are required",
			},
		},
		{
			name: "invalid credentials",
			requestBody: LoginRequest{
				Username: "wronguser",
				Password: "wrongpassword",
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody: map[string]interface{}{
				"status":  "error",
				"message": "Invalid credentials",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test route
			router := gin.New()
			router.POST("/login", Login)

			// Create test request
			jsonBody, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/login", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			// Create a response recorder.
			w := httptest.NewRecorder()

			// Execute the request.
			router.ServeHTTP(w, req)

			// Assert the status code.
			assert.Equal(t, tt.expectedStatus, w.Code)

			// Parse the response body.
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			// Assert the response body.
			if tt.expectedStatus == http.StatusOK {
				// Successful login delivers its bearer token only in the HttpOnly cookie.
				assert.Equal(t, "success", response["status"])
				assert.Equal(t, "", response["message"])
				assert.NotContains(t, response, "data")
				cookies := w.Result().Cookies()
				if assert.Len(t, cookies, 1) {
					cookie := cookies[0]
					assert.Equal(t, "session_token", cookie.Name)
					assert.True(t, cookie.HttpOnly)
					assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
					assert.Equal(t, "/", cookie.Path)
					assert.Equal(t, sessionCookieMaxAge, cookie.MaxAge)
					assert.NotEmpty(t, cookie.Value)
					assert.NotContains(t, w.Body.String(), cookie.Value)
					_, err := accounts.GetSession(cookie.Value)
					assert.NoError(t, err)
				}
			} else {
				assert.Equal(t, tt.expectedBody, response)
			}
		})
	}
	// Clear test data
	accounts.DeleteAccountByUsername("testuser")
	accounts.DeleteAllSessions()
}

func TestPostLogoutDeletesSessionAndClearsCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token, err := accounts.CreateSession("logout-user", sessionCookieMaxAge, "", "", "password")
	if err != nil {
		t.Fatal(err)
	}
	defer accounts.DeleteSession(token)

	router := gin.New()
	router.POST("/api/logout", PostLogout)
	request := httptest.NewRequest(http.MethodPost, "https://monitor.example/api/logout", nil)
	request.Header.Set("Origin", "https://monitor.example")
	request.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"status":"success","message":""}`, response.Body.String())
	if cookies := response.Result().Cookies(); assert.Len(t, cookies, 1) {
		assert.Equal(t, "session_token", cookies[0].Name)
		assert.Equal(t, "", cookies[0].Value)
		assert.True(t, cookies[0].MaxAge < 0)
		assert.Equal(t, "/", cookies[0].Path)
		assert.True(t, cookies[0].HttpOnly)
	}
	_, err = accounts.GetSession(token)
	assert.Error(t, err, "logged-out cookie must not authenticate again")
}

func TestPostLogoutRejectsUntrustedOriginsWithoutDeletingSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name, origin, remoteAddr, forwardedHost, authorization string
	}{
		{name: "missing origin"},
		{name: "cross origin", origin: "https://evil.example"},
		{name: "scheme mismatch", origin: "http://monitor.example"},
		{name: "malformed origin", origin: "https://monitor.example/path"},
		{name: "spoofed forwarded host", origin: "https://evil.example", remoteAddr: "203.0.113.8:4555", forwardedHost: "evil.example"},
		{name: "authorization is not a bypass", origin: "https://evil.example", authorization: "Bearer untrusted"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token, err := accounts.CreateSession("logout-user", sessionCookieMaxAge, "", "", "password")
			if err != nil {
				t.Fatal(err)
			}
			defer accounts.DeleteSession(token)
			router := gin.New()
			router.POST("/api/logout", PostLogout)
			request := httptest.NewRequest(http.MethodPost, "https://monitor.example/api/logout", nil)
			if tc.origin != "" {
				request.Header.Set("Origin", tc.origin)
			}
			if tc.remoteAddr != "" {
				request.RemoteAddr = tc.remoteAddr
			}
			request.Header.Set("X-Forwarded-Host", tc.forwardedHost)
			request.Header.Set("Authorization", tc.authorization)
			request.AddCookie(&http.Cookie{Name: "session_token", Value: token})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			assert.Equal(t, http.StatusForbidden, response.Code)
			assert.Empty(t, response.Header().Values("Set-Cookie"))
			_, err = accounts.GetSession(token)
			assert.NoError(t, err, "rejected POST must leave the session intact")
		})
	}
}

func TestPostLogoutRequiresLiveSessionCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/logout", PostLogout)
	for _, tc := range []struct{ name, token string }{
		{name: "missing cookie"},
		{name: "invalid cookie", token: "not-a-session"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "https://monitor.example/api/logout", nil)
			request.Header.Set("Origin", "https://monitor.example")
			if tc.token != "" {
				request.AddCookie(&http.Cookie{Name: "session_token", Value: tc.token})
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			assert.Equal(t, http.StatusUnauthorized, response.Code)
			assert.Empty(t, response.Header().Values("Set-Cookie"))
		})
	}
}

func TestLegacyGetLogoutStillRedirects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token, err := accounts.CreateSession("logout-user", sessionCookieMaxAge, "", "", "password")
	if err != nil {
		t.Fatal(err)
	}
	defer accounts.DeleteSession(token)
	router := gin.New()
	router.GET("/api/logout", Logout)
	request := httptest.NewRequest(http.MethodGet, "https://monitor.example/api/logout", nil)
	request.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assert.Equal(t, http.StatusFound, response.Code)
	assert.Equal(t, "/", response.Header().Get("Location"))
	_, err = accounts.GetSession(token)
	assert.Error(t, err)
}

func TestPostLogoutBehindTrustedReverseProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token, err := accounts.CreateSession("logout-user", sessionCookieMaxAge, "", "", "password")
	if err != nil {
		t.Fatal(err)
	}
	defer accounts.DeleteSession(token)
	router := gin.New()
	router.POST("/api/logout", PostLogout)
	request := httptest.NewRequest(http.MethodPost, "http://internal:8080/api/logout", nil)
	request.RemoteAddr = "127.0.0.1:44000"
	request.Header.Set("Origin", "https://monitor.example")
	request.Header.Set("X-Forwarded-Host", "monitor.example")
	request.Header.Set("X-Forwarded-Proto", "https")
	request.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assert.Equal(t, http.StatusOK, response.Code)
	_, err = accounts.GetSession(token)
	assert.Error(t, err)
}

func TestSessionCookieSecureFollowsRequestScheme(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		requestURL string
		remoteAddr string
		forwarded  string
		wantSecure bool
	}{
		{
			name:       "http",
			requestURL: "http://example.test/login",
		},
		{
			name:       "https",
			requestURL: "https://example.test/login",
			wantSecure: true,
		},
		{
			name:       "trusted reverse proxy",
			requestURL: "http://example.test/login",
			remoteAddr: "127.0.0.1:43000",
			forwarded:  "https",
			wantSecure: true,
		},
		{
			name:       "untrusted spoofed reverse proxy",
			requestURL: "http://example.test/login",
			remoteAddr: "203.0.113.10:43000",
			forwarded:  "https",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, tt.requestURL, nil)
			if tt.remoteAddr != "" {
				c.Request.RemoteAddr = tt.remoteAddr
			}
			if tt.forwarded != "" {
				c.Request.Header.Set("X-Forwarded-Proto", tt.forwarded)
			}

			setSessionCookie(c, "test-session", sessionCookieMaxAge)

			setCookie := strings.Join(w.Header().Values("Set-Cookie"), "\n")
			if tt.wantSecure {
				assert.Contains(t, setCookie, "Secure")
			} else {
				assert.NotContains(t, setCookie, "Secure")
			}
		})
	}
}

// The login page relies on these two messages to show the 2FA code prompt:
// Changing them would break the frontend’s 2FA detection and leave users stuck.
// Keep the server contract: require a 2FA code and accept only a valid one.
func TestLoginEnforcesTwoFactorCode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user, err := accounts.CreateAccount("twofactoruser", "correctpassword")
	assert.NoError(t, err)
	secret, _, err := accounts.Generate2Fa()
	assert.NoError(t, err)
	assert.NoError(t, accounts.Enable2Fa(user.UUID, secret))

	post := func(body LoginRequest) (int, map[string]interface{}) {
		router := gin.New()
		router.POST("/login", Login)
		raw, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/login", bytes.NewBuffer(raw))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		var parsed map[string]interface{}
		_ = json.Unmarshal(recorder.Body.Bytes(), &parsed)
		return recorder.Code, parsed
	}

	// Password alone must trigger the 2FA code prompt.
	status, payload := post(LoginRequest{Username: "twofactoruser", Password: "correctpassword"})
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "2FA code is required", payload["message"])

	// An invalid 2FA code must show a code error, not a password error.
	status, payload = post(LoginRequest{Username: "twofactoruser", Password: "correctpassword", TwoFa: "000000"})
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "Invalid 2FA code", payload["message"])

	// A valid code completes login.
	code, err := totp.GenerateCode(secret, time.Now())
	assert.NoError(t, err)
	status, _ = post(LoginRequest{Username: "twofactoruser", Password: "correctpassword", TwoFa: code})
	assert.Equal(t, http.StatusOK, status)
}
