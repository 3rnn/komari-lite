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
	// 设置测试模式
	gin.SetMode(gin.TestMode)
	accounts.CreateAccount("testuser", "correctpassword")
	tests := []struct {
		name           string
		requestBody    LoginRequest
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "成功登录",
			requestBody: LoginRequest{
				Username: "testuser",
				Password: "correctpassword",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "无效的请求体",
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
			name: "错误的凭据",
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
			// 创建测试路由
			router := gin.New()
			router.POST("/login", Login)

			// 创建测试请求
			jsonBody, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/login", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			// 创建响应记录器
			w := httptest.NewRecorder()

			// 执行请求
			router.ServeHTTP(w, req)

			// 断言状态码
			assert.Equal(t, tt.expectedStatus, w.Code)

			// 解析响应体
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			// 断言响应体
			if tt.expectedStatus == http.StatusOK {
				// 对于成功的情况，我们只检查响应结构，不检查具体的 session token
				assert.Equal(t, "success", response["status"])
				assert.Equal(t, "", response["message"])
				data, ok := response["data"].(map[string]interface{})
				assert.True(t, ok)
				setCookie, ok := data["set-cookie"].(map[string]interface{})
				assert.True(t, ok)
				assert.NotEmpty(t, setCookie["session_token"])
				assert.Contains(t, strings.Join(w.Header().Values("Set-Cookie"), "\n"), "session_token=")
			} else {
				assert.Equal(t, tt.expectedBody, response)
			}
		})
	}
	// 清除测试数据
	accounts.DeleteAccountByUsername("testuser")
	accounts.DeleteAllSessions()
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

// 登录页靠这两条文案切换「输入动态口令」界面：
// 文案一改，前端就认不出「需要 2FA」，用户会卡在登录页。
// 这里把服务端契约钉死：开了 2FA 就必须带口令，且只接受有效口令。
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

	// 只给账号密码 → 必须要求动态口令（前端据此弹出输入框）
	status, payload := post(LoginRequest{Username: "twofactoruser", Password: "correctpassword"})
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "2FA code is required", payload["message"])

	// 口令不合法 → 明确的口令错误文案（前端据此显示口令提示而非密码错误）
	status, payload = post(LoginRequest{Username: "twofactoruser", Password: "correctpassword", TwoFa: "000000"})
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "Invalid 2FA code", payload["message"])

	// 有效口令 → 登录成功
	code, err := totp.GenerateCode(secret, time.Now())
	assert.NoError(t, err)
	status, _ = post(LoginRequest{Username: "twofactoruser", Password: "correctpassword", TwoFa: code})
	assert.Equal(t, http.StatusOK, status)
}
