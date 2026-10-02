package router

import (
	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/web/api"
	"github.com/komari-monitor/komari/web/api/admin"
	"github.com/komari-monitor/komari/web/api/client"
	public_api "github.com/komari-monitor/komari/web/api/public"
	installweb "github.com/komari-monitor/komari/web/install"
	"github.com/komari-monitor/komari/web/public"
	jsonRpc "github.com/komari-monitor/komari/web/rpc/jsonrpc"
)

// Register binds all HTTP, WebSocket, JSON-RPC and static frontend routes.
//
// JSON endpoints bind directly to RPC2 methods through the declarative jsonRpc.Bind bridge.
// Only binary, streaming, redirect, and special-authorization endpoints retain REST handlers.
func Register(r *gin.Engine) {
	r.Use(liteRoutes)
	r.Any("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	registerPublicRoutes(r)
	registerAgentRoutes(r)
	registerAdminRoutes(r)

	public.Static(r.Group("/"), func(handlers ...gin.HandlerFunc) {
		r.NoRoute(handlers...)
	})
}

// registerPublicRoutes sets up public routes, binding JSON reads to public:* RPC methods.
func registerPublicRoutes(r *gin.Engine) {
	installweb.RegisterCompleted(r)

	// Agent installation scripts and binaries are distributed through the panel; the target machine no longer has direct access to GitHub.
	r.GET("/agent/install.sh", func(c *gin.Context) { public_api.ServeAgentInstaller(c.Writer, c.Request) })
	r.GET("/agent/install.ps1", func(c *gin.Context) { public_api.ServeAgentInstaller(c.Writer, c.Request) })
	r.GET("/agent/download/:artifact", func(c *gin.Context) { public_api.ServeAgentDownload(c.Writer, c.Request) })

	// Keep REST handlers for non-JSON or special workflows.
	r.POST("/api/login", public_api.Login)
	r.GET("/api/logout", public_api.Logout)
	r.POST("/api/logout", public_api.PostLogout)
	r.GET("/api/oauth", public_api.OAuth)
	r.GET("/api/oauth_callback", public_api.OAuthCallback)
	r.GET("/api/mjpeg_live", public_api.MjpegLiveHandler)
	// /api/clients is a WebSocket endpoint: clients send "get" or "get <uuid>" for online nodes and latest reports.
	// It is not JSON-RPC, so keep its WebSocket handler.
	r.GET("/api/clients", api.GetClients)

	// Bind JSON endpoints to RPC2.
	r.GET("/api/me", jsonRpc.Bind("public:getMe", jsonRpc.WithRaw()))
	r.GET("/api/nodes", jsonRpc.Bind("public:getNodesInformation"))
	r.GET("/api/public", jsonRpc.Bind("public:getPublicSettings"))
	r.GET("/api/version", jsonRpc.Bind("public:getVersion"))
	r.GET("/api/recent/:uuid", jsonRpc.Bind("public:getClientRecentRecords", jsonRpc.WithPath("uuid")))
	r.GET("/api/records/load", jsonRpc.Bind("public:getRecordsByUUID", jsonRpc.WithQuery("uuid", "load_type", "hours")))
	r.GET("/api/records/ping", jsonRpc.Bind("public:getPingRecords", jsonRpc.WithQuery("uuid", "task_id", "hours")))
	r.GET("/api/task/ping", jsonRpc.Bind("public:getPublicPingTasks"))

	// Direct JSON-RPC entry point.
	r.GET("/api/rpc2", jsonRpc.OnRpcRequest)
	r.POST("/api/rpc2", jsonRpc.OnRpcRequest)
}

// registerAgentRoutes sets up agent report and polling routes.
func registerAgentRoutes(r *gin.Engine) {
	// AutoDiscovery registration uses a separate Authorization key authentication, preserving the rest handler.
	r.POST("/api/clients/register", client.RegisterClient)

	tokenAuthorized := r.Group("/api/clients", api.RequireRole(api.RoleAdmin, api.RoleClient))
	{
		// Keep REST handlers for reports over WebSocket, raw streams, and legacy protocols.
		tokenAuthorized.GET("/report", client.WebSocketReport)
		tokenAuthorized.POST("/uploadBasicInfo", client.UploadBasicInfo)
		tokenAuthorized.POST("/report", client.UploadReport)
		tokenAuthorized.GET("/v2/rpc", client.WebSocketV2RPC)
		tokenAuthorized.POST("/v2/rpc", client.UploadV2RPC)

		// Bind JSON endpoints to RPC2 (client:* namespace).
		tokenAuthorized.GET("/ping/tasks", jsonRpc.Bind("client:getPingTasks", jsonRpc.WithRaw()))
		tokenAuthorized.POST("/ping/result", jsonRpc.Bind("client:uploadPingResult", jsonRpc.WithRaw()))
	}
}

// registerAdminRoutes binds admin JSON endpoints to admin:* RPC methods; binary/streaming endpoints remain REST.
func registerAdminRoutes(r *gin.Engine) {
	g := r.Group("/api/admin", api.RequireRole(api.RoleAdmin))
	g.GET("/dashboard", jsonRpc.Bind("admin:getDashboard", jsonRpc.WithQuery("sections", "limit"), jsonRpc.WithRaw()))
	g.GET("/dashboard/charts", jsonRpc.Bind("admin:getDashboardCharts", jsonRpc.WithQuery("sections", "limit"), jsonRpc.WithRaw()))
	g.GET("/dashboard/alerts", jsonRpc.Bind("admin:getDashboardAlertItems", jsonRpc.WithQuery("kind"), jsonRpc.WithRaw()))

	// --- Keep REST handlers for binary, streaming, and redirect endpoints ---
	g.GET("/download/backup", admin.DownloadBackup)
	uploadHandler := admin.NewArchiveUploadHandler()
	uploadGroup := g.Group("/upload")
	{
		uploadGroup.POST("/init", uploadHandler.Init)
		uploadGroup.POST("/chunk", uploadHandler.Chunk)
		uploadGroup.POST("/merge", uploadHandler.Merge)
		uploadGroup.POST("/cancel", uploadHandler.Cancel)
	}
	g.GET("/test/geoip", jsonRpc.Bind("admin:testGeoip", jsonRpc.WithQuery("ip")))
	g.POST("/test/sendMessage", jsonRpc.Bind("admin:testSendMessage"))
	g.POST("/update/mmdb", admin.UpdateMmdbGeoIP)
	g.POST("/update/user", admin.UpdateUser)
	g.PUT("/update/favicon", admin.UploadFavicon)
	g.POST("/update/favicon", admin.DeleteFavicon)
	g.GET("/settings/https", admin.GetHTTPSSettings)
	g.POST("/settings/https", admin.UpdateHTTPSSettings)
	g.POST("/settings/https/reload", admin.ReloadHTTPSCertificate)

	// Lite uses the fixed local Glass theme and exposes only display settings, not theme installation, import, switching, deletion, or upstream updates.
	theme := g.Group("/theme")
	{
		theme.POST("/settings", admin.UpdateThemeSettings)
	}

	// 2FA contains QR code PNG/sensitive operation, keep rest handler.
	twoFactor := g.Group("/2fa")
	{
		twoFactor.GET("/generate", admin.Generate2FA)
		twoFactor.POST("/enable", admin.Enable2FA)
		twoFactor.POST("/disable", api.RequireSensitive2FA(), admin.Disable2FA)
	}

	// OAuth2 account binding uses a redirect, so keep its REST handler.
	oauth2 := g.Group("/oauth2")
	{
		oauth2.GET("/bind", admin.BindingExternalAccount)
		oauth2.POST("/unbind", admin.UnbindExternalAccount)
	}

	// --- Bind all remaining JSON endpoints to RPC2 ---

	// settings
	settings := g.Group("/settings")
	{
		settings.GET("/", jsonRpc.Bind("admin:getSettings"))
		settings.POST("/", jsonRpc.Bind("admin:editSettings"))
		settings.GET("/dashboard", jsonRpc.Bind("admin:getDashboardSettings"))
		settings.POST("/dashboard", jsonRpc.Bind("admin:setDashboardSettings", jsonRpc.WithMessage("settings saved")))
		settings.POST("/oidc", jsonRpc.Bind("admin:setOidcProvider"))
		settings.GET("/oidc", jsonRpc.Bind("admin:getOidcProvider", jsonRpc.WithQuery("provider")))
		settings.POST("/message-sender", jsonRpc.Bind("admin:setMessageSenderProvider"))
		settings.GET("/message-sender", jsonRpc.Bind("admin:getMessageSenderProvider", jsonRpc.WithQuery("provider")))
	}

	// database storage inspection and maintenance
	databaseGroup := g.Group("/database")
	{
		databaseGroup.GET("/size", jsonRpc.Bind("admin:getDatabaseSize"))
		databaseGroup.POST("/vacuum", jsonRpc.Bind("admin:vacuumDatabase"))
	}

	// clients
	clientGroup := g.Group("/client")
	{
		clientGroup.POST("/add", jsonRpc.Bind("admin:addClient", jsonRpc.WithFlat()))
		clientGroup.GET("/list", jsonRpc.Bind("admin:listClients", jsonRpc.WithRaw()))
		clientGroup.GET("/:uuid", jsonRpc.Bind("admin:getClient", jsonRpc.WithPath("uuid"), jsonRpc.WithRaw()))
		clientGroup.POST("/:uuid/edit", jsonRpc.Bind("admin:editClient", jsonRpc.WithPath("uuid")))
		clientGroup.POST("/:uuid/display-ping-task", jsonRpc.Bind("admin:setClientDisplayPingTask", jsonRpc.WithPath("uuid")))
		clientGroup.POST("/:uuid/display-ping-tasks", jsonRpc.Bind("admin:setClientDisplayPingTasks", jsonRpc.WithPath("uuid")))
		clientGroup.POST("/:uuid/remove", jsonRpc.Bind("admin:removeClient", jsonRpc.WithPath("uuid")))
		clientGroup.GET("/:uuid/token", jsonRpc.Bind("admin:getClientToken", jsonRpc.WithPath("uuid"), jsonRpc.WithFlat()))
		clientGroup.GET("/:uuid/deployment-profile", jsonRpc.Bind("admin:getClientDeploymentProfile", jsonRpc.WithPath("uuid"), jsonRpc.WithRaw()))
		clientGroup.POST("/:uuid/deployment-profile", jsonRpc.Bind("admin:saveClientDeploymentProfile", jsonRpc.WithPath("uuid"), jsonRpc.WithRaw()))
		clientGroup.GET("/:uuid/traffic-calibration", admin.GetTrafficCalibration)
		clientGroup.POST("/:uuid/traffic-calibration", admin.UpdateTrafficCalibration)
		clientGroup.POST("/token/rotate", api.RequireSensitive2FA(), jsonRpc.Bind("admin:rotateClientToken"))
		clientGroup.POST("/order", jsonRpc.Bind("admin:orderClients"))
	}

	// records
	record := g.Group("/record")
	{
		record.POST("/clear", jsonRpc.Bind("admin:clearRecords"))
		record.POST("/clear/all", jsonRpc.Bind("admin:clearAllRecords"))
	}

	// sessions
	session := g.Group("/session")
	{
		session.GET("/get", jsonRpc.Bind("admin:getSessions", jsonRpc.WithFlat()))
		session.POST("/remove", jsonRpc.Bind("admin:deleteSession"))
		session.POST("/remove/all", jsonRpc.Bind("admin:deleteAllSessions"))
	}

	g.GET("/logs", jsonRpc.Bind("admin:getLogs", jsonRpc.WithQuery("limit", "page")))

	// notifications
	notificationGroup := g.Group("/notification")
	{
		notificationGroup.GET("/offline", jsonRpc.Bind("admin:listOfflineNotifications"))
		notificationGroup.POST("/offline/edit", jsonRpc.Bind("admin:editOfflineNotification"))
		notificationGroup.POST("/offline/enable", jsonRpc.Bind("admin:enableOfflineNotification"))
		notificationGroup.POST("/offline/disable", jsonRpc.Bind("admin:disableOfflineNotification"))
		notificationGroup.GET("/offline/default", jsonRpc.Bind("admin:getOfflineNotificationDefault"))
		notificationGroup.POST("/offline/default", jsonRpc.Bind("admin:setOfflineNotificationDefault"))
		loadAlert := notificationGroup.Group("/load")
		{
			loadAlert.GET("/", jsonRpc.Bind("admin:getAllLoadNotifications"))
			loadAlert.POST("/add", jsonRpc.Bind("admin:addLoadNotification"))
			loadAlert.POST("/delete", jsonRpc.Bind("admin:deleteLoadNotification"))
			loadAlert.POST("/edit", jsonRpc.Bind("admin:editLoadNotification"))
			loadAlert.GET("/current", jsonRpc.Bind("admin:listCurrentLoadAlerts"))
			loadAlert.POST("/silence", jsonRpc.Bind("admin:setLoadAlertSilence"))
		}
		trafficReport := notificationGroup.Group("/traffic-report")
		{
			trafficReport.GET("/", jsonRpc.Bind("admin:listTrafficReportNotifications"))
			trafficReport.POST("/edit", jsonRpc.Bind("admin:editTrafficReportNotifications"))
			trafficReport.POST("/enable", jsonRpc.Bind("admin:enableTrafficReportNotifications"))
			trafficReport.POST("/disable", jsonRpc.Bind("admin:disableTrafficReportNotifications"))
			trafficReport.GET("/default", jsonRpc.Bind("admin:getTrafficReportDefault"))
			trafficReport.POST("/default", jsonRpc.Bind("admin:setTrafficReportDefault"))
			trafficReport.POST("/send-daily", jsonRpc.Bind("admin:sendDailyTrafficReport"))
		}
		pingLoss := notificationGroup.Group("/ping-loss")
		{
			pingLoss.GET("/", jsonRpc.Bind("admin:listPingLossNotifications"))
			pingLoss.POST("/add", jsonRpc.Bind("admin:addPingLossNotification"))
			pingLoss.POST("/edit", jsonRpc.Bind("admin:editPingLossNotifications"))
			pingLoss.POST("/batch", jsonRpc.Bind("admin:upsertPingLossNotifications"))
			pingLoss.POST("/delete", jsonRpc.Bind("admin:deletePingLossNotifications"))
			pingLoss.GET("/default", jsonRpc.Bind("admin:getPingLossNotificationDefault"))
			pingLoss.POST("/default", jsonRpc.Bind("admin:setPingLossNotificationDefault"))
		}
	}

	// ping tasks
	pingTask := g.Group("/ping")
	{
		pingTask.GET("/", jsonRpc.Bind("admin:getAllPingTasks"))
		pingTask.POST("/add", jsonRpc.Bind("admin:addPingTask"))
		pingTask.POST("/delete", jsonRpc.Bind("admin:deletePingTask"))
		pingTask.POST("/edit", jsonRpc.Bind("admin:editPingTask"))
		pingTask.POST("/order", jsonRpc.Bind("admin:orderPingTask"))
	}

}
