package admin

import (
	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/web/api"
)

// oauth.go
// External account binding/unbinding: The binding takes 302 redirects and remains a rest handler (does not take the RPC bridge).
// The get/set configured by the OIDC provider has been migrated to the RPC2 method (see web/rpc/jsonrpc/admin.provider.go).

func BindingExternalAccount(c *gin.Context) {
	session, _ := c.Cookie("session_token")
	user, err := accounts.GetUserBySession(session)
	if err != nil {
		api.RespondError(c, 500, "No user found: "+err.Error())
		return
	}
	c.SetCookie("binding_external_account", user.UUID, 3600, "/", "", false, true)
	c.Redirect(302, "/api/oauth")
}

func UnbindExternalAccount(c *gin.Context) {
	session, _ := c.Cookie("session_token")
	user, err := accounts.GetUserBySession(session)
	if err != nil {
		api.RespondError(c, 500, "No user found: "+err.Error())
		return
	}
	if err := accounts.UnbindExternalAccount(user.UUID); err != nil {
		api.RespondError(c, 500, "Failed to unbind external account: "+err.Error())
		return
	}
	api.RespondSuccess(c, nil)
}
