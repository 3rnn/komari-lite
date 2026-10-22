package admin

import (
	"image/png"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/web/api"
)

func Generate2FA(c *gin.Context) {
	uuid, _ := c.Get("uuid")
	userUUID, _ := uuid.(string)
	session, _ := c.Cookie("session_token")
	code := c.GetHeader("X-2FA-Code")
	if code == "" {
		code = c.GetHeader("X-Two-Factor-Code")
	}
	if code == "" {
		code = c.Query("2fa_code")
	}
	if code == "" {
		code = c.Query("code")
	}
	img, err := accounts.Begin2FAEnrollment(userUUID, session, code)
	if err != nil {
		api.RespondError(c, 401, "Failed to generate 2FA: "+err.Error())
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Type", "image/png")
	c.Writer.WriteHeader(200)
	png.Encode(c.Writer, img)
}

func Enable2FA(c *gin.Context) {
	uuid, _ := c.Get("uuid")
	userUUID, _ := uuid.(string)
	session, _ := c.Cookie("session_token")
	code := c.Query("code")
	if err := accounts.Complete2FAEnrollment(userUUID, session, code); err != nil {
		api.RespondError(c, 400, "Failed to enable 2FA: "+err.Error())
		return
	}
	c.SetCookie("2fa_secret", "", -1, "/", "", false, true)

	api.RespondSuccess(c, "2FA enabled successfully")
}

func Disable2FA(c *gin.Context) {
	uuid, _ := c.Get("uuid")
	err := accounts.Disable2Fa(uuid.(string))
	if err != nil {
		api.RespondError(c, 500, "Failed to disable 2FA: "+err.Error())
		return
	}
	api.RespondSuccess(c, "")
}
