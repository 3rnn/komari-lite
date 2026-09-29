package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/metricstore"
	"github.com/komari-monitor/komari/database/models"
	d_notification "github.com/komari-monitor/komari/database/notification"
	"github.com/komari-monitor/komari/database/tasks"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/corn"
	"github.com/komari-monitor/komari/pkg/metric"
	"github.com/komari-monitor/komari/pkg/migrations"
	"github.com/komari-monitor/komari/utils"
	"github.com/komari-monitor/komari/utils/geoip"
	"github.com/komari-monitor/komari/utils/httpsserver"
	logger "github.com/komari-monitor/komari/utils/log"
	"github.com/komari-monitor/komari/utils/messageSender"
	"github.com/komari-monitor/komari/utils/notifier"
	"github.com/komari-monitor/komari/web/api"
	installweb "github.com/komari-monitor/komari/web/install"
	"github.com/komari-monitor/komari/web/oauth"
	frontendpublic "github.com/komari-monitor/komari/web/public"
	"github.com/komari-monitor/komari/web/router"
	"github.com/komari-monitor/komari/web/security"
	storageupdateweb "github.com/komari-monitor/komari/web/storageupdate"
	upgradeweb "github.com/komari-monitor/komari/web/update"
	"github.com/komari-monitor/komari/web/upload"
	"gorm.io/gorm"
)

// cleanupFunc is a cleanup function executed during the shutdown phase.
type cleanupFunc struct {
	name string
	fn   func(ctx context.Context) error
}

// App explicitly models the startup lifecycle of the server.
//
// Previously, RunServer combined directory creation, database and metric-store initialization, GeoIP, scheduled jobs, notifications, OAuth,
// Gin middleware, routing, HTTP startup, and shutdown in one function.
// Its startup order was implicit, asynchronous initialization ran too early and swallowed errors, and shutdown was incomplete.
//
// App separates these responsibilities into ordered phases:
//
//	Bootstrap       infrastructure: directories, database, configuration snapshot
//	InitStores      storage: metric store
//	InitProviders   external providers: OAuth (synchronous, required by routes), GeoIP, messaging
//	StartBackground scheduled jobs
//	BuildRouter     Gin engine and routes
//	Run             start HTTP and block until a signal arrives
//	Shutdown        run registered cleanups in reverse order
//
// Each stage returns errors so the caller can abort startup. Resources register cleanup functions on a
// cleanup stack; shutdown releases them in last-in, first-out (LIFO) order.
type App struct {
	settings   *config.Settings
	engine     *gin.Engine
	server     *http.Server
	reload     *ReloadManager
	dbReady    bool
	oauthReady bool

	cleanups []cleanupFunc
}

// NewApp constructs an empty App; each stage performs its own initialization.
func NewApp() *App {
	return &App{
		reload: NewReloadManager(),
	}
}

// addCleanup registers a cleanup function for the shutdown phase (last in first out execution).
func (a *App) addCleanup(name string, fn func(ctx context.Context) error) {
	a.cleanups = append(a.cleanups, cleanupFunc{name: name, fn: fn})
}

// Bootstrap initializes the data directories, database, and configuration snapshot.
func (a *App) Bootstrap() error {
	if err := os.MkdirAll("./data/theme", os.ModePerm); err != nil {
		return fmt.Errorf("failed to create theme directory: %w", err)
	}

	// Set the version ID so dbcore can back up ./data automatically when upgrading.
	// This must happen before Initialize().
	dbcore.SetVersionID(utils.CurrentVersion + "-" + utils.VersionHash)

	// Initialize the database explicitly (returns an error instead of log.Fatal in the getter).
	if err := dbcore.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	a.dbReady = true
	a.addCleanup("database", func(context.Context) error {
		return dbcore.Close()
	})
	if err := frontendpublic.EnsureBundledThemes(); err != nil {
		return fmt.Errorf("failed to initialize public themes: %w", err)
	}

	gin.SetMode(gin.ReleaseMode)

	if err := normalizeMetricStorageSettings(); err != nil {
		return fmt.Errorf("failed to normalize metric storage settings: %w", err)
	}

	conf, err := config.GetManyAs[config.Settings]()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}
	a.settings = conf
	return nil
}

// normalizeMetricStorageSettings makes the active storage policy independent
// of settings saved by older upstream, stable, fix, or snapshot releases.
// Per-metric retention values are deliberately left untouched.
func normalizeMetricStorageSettings() error {
	return config.SetMany(map[string]any{
		config.LowResourceModeKey:                false,
		metricstore.MetricDownsamplingEnabledKey: true,
	})
}

// InitStores initializes the metric store and performs the metrics migration.
//
// metric store is now always enabled (the old metric_store_enabled switch is deprecated):
// Without explicit configuration, use SQLite (./data/metrics.db); otherwise use the configured MySQL/PostgreSQL database.
// Initialization errors abort startup; there is no silent fallback to the legacy records table.
//
// After initialization, run the one-time metric-store migration, then the startup migration. If the metrics
// backend changes (for example, from default SQLite to MySQL/PostgreSQL), migrate data from the previous
// metrics target to the current one. Migration failures abort startup with an explicit error.
func (a *App) InitStores() error {
	if err := metricstore.InitializeStore(); err != nil {
		auditlog.EventLog("error", fmt.Sprintf("Failed to initialize metric store: %v", err))
		return fmt.Errorf("failed to initialize metric store: %w", err)
	}
	a.addCleanup("metric-store", func(ctx context.Context) error {
		return metricstore.CloseStoreContext(ctx)
	})

	if err := migrations.RunMetricStoreMigrations(migrations.MetricContext{
		DB:    dbcore.GetDBInstance(),
		Store: metricstore.GetStore(),
	}); err != nil {
		auditlog.EventLog("error", fmt.Sprintf("Metric store one-shot migrations failed: %v", err))
		return fmt.Errorf("metric store one-shot migrations failed: %w", err)
	}

	// Migrate data from the previous metrics target when the storage backend changes; fail startup on error.
	if err := metricstore.RunStartupMigration(); err != nil {
		auditlog.EventLog("error", fmt.Sprintf("Metrics startup migration failed: %v", err))

		return fmt.Errorf("metrics startup migration failed: %w", err)
	}
	processPendingMetricCleanupAtStartup(dbcore.GetDBInstance())
	var registeredClients []models.Client
	if err := dbcore.GetDBInstance().Select("uuid").Find(&registeredClients).Error; err != nil {
		return fmt.Errorf("failed to list clients for metrics orphan cleanup: %w", err)
	}
	validEntities := make(map[string]struct{}, len(registeredClients))
	for _, client := range registeredClients {
		validEntities[client.UUID] = struct{}{}
	}
	var pingTasks []models.PingTask
	if err := dbcore.GetDBInstance().Select("id", "clients").Find(&pingTasks).Error; err != nil {
		return fmt.Errorf("failed to list ping tasks for metrics orphan cleanup: %w", err)
	}
	validPingAssignments := make(map[uint]map[string]struct{}, len(pingTasks))
	for _, task := range pingTasks {
		clients := make(map[string]struct{}, len(task.Clients))
		for _, client := range task.Clients {
			clients[client] = struct{}{}
		}
		validPingAssignments[task.Id] = clients
	}
	orphanResult, err := metricstore.CleanupOrphanedData(context.Background(), validEntities, validPingAssignments)
	if err != nil {
		return fmt.Errorf("failed to clean orphaned metrics: %w", err)
	}
	if orphanResult.Entities > 0 || orphanResult.PingTasks > 0 || orphanResult.PingAssignments > 0 {
		logger.Infof("metricstore", "Removed orphaned metric data (entities=%d ping_tasks=%d ping_assignments=%d)", orphanResult.Entities, orphanResult.PingTasks, orphanResult.PingAssignments)
	}
	metricstore.StartReportBatcher()
	a.addCleanup("metric-report-batcher", func(ctx context.Context) error {
		return metricstore.StopReportBatcher(ctx)
	})
	return nil
}

func processPendingMetricCleanupAtStartup(db *gorm.DB) {
	if err := metricstore.ProcessPendingCleanupJobs(context.Background(), db); err != nil {
		logger.Errorf("metricstore", "Pending metric cleanup failed during startup and will be retried in the background: %v", err)
	}
}

// CommitRestore marks a staged backup as fully usable only after the main
// database, metric store, providers, and router all initialized. Background
// tasks start afterwards so a candidate restore cannot emit notifications or
// perform scheduled writes before it becomes durable.
func (a *App) CommitRestore() error {
	if err := dbcore.CommitPendingRestore(); err != nil {
		return fmt.Errorf("commit verified backup restore: %w", err)
	}
	return nil
}

// InitProviders initializes external providers.
//
// OAuth must initialize synchronously before HTTP starts accepting requests; otherwise oauth.CurrentProvider()
// may be nil. GeoIP and messaging may initialize in the background.
func (a *App) InitProviders() error {
	a.initOAuth()

	// GeoIP: may involve downloading/loading mmdb and performing in the background to avoid slow startup.
	go geoip.InitGeoIp()
	a.addCleanup("geoip", func(context.Context) error {
		return geoip.Shutdown()
	})

	// Message sending provider.
	messageSender.Initialize()
	a.addCleanup("message-sender", func(context.Context) error {
		return messageSender.Shutdown()
	})

	return nil
}

// initOAuth initializes the provider once. The restricted upgrade server also
// needs OAuth login, so provider setup may happen before the normal provider
// stage without registering duplicate cleanup work.
func (a *App) initOAuth() {
	if a.oauthReady {
		return
	}
	if err := oauth.Initialize(); err != nil {
		// Keep password login available when an OAuth provider is misconfigured.
		logger.Errorf("server", "Failed to initialize OAuth provider: %v", err)
		auditlog.EventLog("error", fmt.Sprintf("Failed to initialize OAuth provider: %v", err))
	}
	a.oauthReady = true
	a.addCleanup("oauth", func(context.Context) error {
		return oauth.Shutdown()
	})
}

func (a *App) LegacyUpgradeRequired() (bool, migrations.LegacyMonitoringSummary, error) {
	return migrations.LegacyMonitoringMigrationRequired(dbcore.GetDBInstance())
}

func (a *App) MetricStorageUpgradeRequired() (bool, metric.SQLiteMigrationSummary, error) {
	summary, err := metricstore.InspectSQLiteStorageMigration(context.Background())
	return summary.Required, summary, err
}

// InstallRequired reports whether the instance still needs the first-run guide.
// A zero-user database is deliberately treated as incomplete so an interrupted
// guide can be resumed after a restart.
func (a *App) InstallRequired() (bool, error) {
	var count int64
	if err := dbcore.GetDBInstance().Model(&models.User{}).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

func firstRunInstallRedirect() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		if c.Request.Method == http.MethodGet &&
			!strings.HasPrefix(requestPath, "/api") &&
			requestPath != installweb.PagePath &&
			filepath.Ext(requestPath) == "" {
			c.Redirect(http.StatusTemporaryRedirect, installweb.PagePath)
			c.Abort()
			return
		}
		c.Next()
	}
}

// RunInstallGuide starts the restricted first-run HTTP server.
func (a *App) RunInstallGuide() (bool, error) {
	controller := installweb.NewController(dbcore.GetDBInstance())
	controller.Activate()
	defer controller.Deactivate()

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8<<20); c.Next() })
	r.Use(logger.GinLogger())
	r.Use(logger.GinRecovery())
	r.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.Header("Cache-Control", "no-store")
		}
		c.Next()
	})
	r.Use(firstRunInstallRedirect())
	controller.Register(r)
	frontendpublic.Static(r.Group("/"), func(handlers ...gin.HandlerFunc) {
		r.NoRoute(func(c *gin.Context) {
			requestPath := c.Request.URL.Path
			if strings.HasPrefix(requestPath, "/api") {
				api.RespondError(c, http.StatusNotFound, "Not found in install mode")
				return
			}
			for _, handler := range handlers {
				handler(c)
				if c.IsAborted() {
					return
				}
			}
		})
	})

	a.engine = r
	a.server = &http.Server{Addr: flags.Listen, Handler: r}
	serverErr := make(chan error, 1)
	logger.Infof("server", "First-run installation guide is available on %s", flags.Listen)
	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(quit)
	select {
	case err := <-serverErr:
		return false, fmt.Errorf("listen in install mode: %w", err)
	case <-quit:
		return false, a.Shutdown()
	case <-controller.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.server.Shutdown(ctx); err != nil {
			return false, fmt.Errorf("stop install server: %w", err)
		}
		a.server = nil
		a.engine = nil
		return true, nil
	}
}

// RunLegacyUpgrade starts a restricted HTTP server on the normal listen
// address. It returns only after the migration finishes and the restricted
// listener has released the port, or after shutdown/interruption.
func (a *App) RunLegacyUpgrade(summary migrations.LegacyMonitoringSummary) (bool, error) {
	a.initOAuth()
	controller := upgradeweb.NewController(dbcore.GetDBInstance(), summary)
	controller.Activate()
	defer controller.Deactivate()

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8<<20); c.Next() })
	r.Use(logger.GinLogger())
	r.Use(logger.GinRecovery())
	cors := security.NewCorsController(a.settings.CorsOriginCheckEnabled, a.settings.CorsAllowedOrigins)
	r.Use(cors.Middleware())
	r.Use(api.IdentityMiddleware())
	r.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.Header("Cache-Control", "no-store")
		}
		c.Next()
	})

	controller.Register(r)
	frontendpublic.Static(r.Group("/"), func(handlers ...gin.HandlerFunc) {
		r.NoRoute(func(c *gin.Context) {
			requestPath := c.Request.URL.Path
			if strings.HasPrefix(requestPath, "/api") {
				api.RespondError(c, http.StatusNotFound, "Not found in upgrade mode")
				return
			}
			if c.Request.Method == http.MethodGet && requestPath != upgradeweb.PagePath && filepath.Ext(requestPath) == "" {
				c.Redirect(http.StatusTemporaryRedirect, upgradeweb.PagePath)
				return
			}
			for _, handler := range handlers {
				handler(c)
				if c.IsAborted() {
					return
				}
			}
		})
	})

	a.engine = r
	a.server = &http.Server{Addr: flags.Listen, Handler: r}
	serverErr := make(chan error, 1)
	logger.Infof("server", "Legacy monitoring data requires the 1.2.7 upgrade wizard on %s", flags.Listen)
	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case err := <-serverErr:
		return false, fmt.Errorf("listen in upgrade mode: %w", err)
	case <-quit:
		return false, a.Shutdown()
	case <-controller.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.server.Shutdown(ctx); err != nil {
			return false, fmt.Errorf("stop upgrade server: %w", err)
		}
		a.server = nil
		a.engine = nil
		return true, nil
	}
}

// RunMetricStorageUpgrade exposes only authentication and SQLite migration
// progress. Normal application routes remain unavailable until validation
// succeeds and the restricted listener releases the port.
func (a *App) RunMetricStorageUpgrade(summary metric.SQLiteMigrationSummary) (bool, error) {
	a.initOAuth()
	controller := storageupdateweb.NewController(summary)
	controller.Activate()
	defer controller.Deactivate()

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8<<20); c.Next() })
	r.Use(logger.GinLogger())
	r.Use(logger.GinRecovery())
	cors := security.NewCorsController(a.settings.CorsOriginCheckEnabled, a.settings.CorsAllowedOrigins)
	r.Use(cors.Middleware())
	r.Use(api.IdentityMiddleware())
	r.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.Header("Cache-Control", "no-store")
		}
		c.Next()
	})

	controller.Register(r)
	frontendpublic.Static(r.Group("/"), func(handlers ...gin.HandlerFunc) {
		r.NoRoute(func(c *gin.Context) {
			requestPath := c.Request.URL.Path
			if strings.HasPrefix(requestPath, "/api") {
				api.RespondError(c, http.StatusNotFound, "Not found in metric storage upgrade mode")
				return
			}
			if c.Request.Method == http.MethodGet && requestPath != storageupdateweb.PagePath && filepath.Ext(requestPath) == "" {
				c.Redirect(http.StatusTemporaryRedirect, storageupdateweb.PagePath)
				return
			}
			for _, handler := range handlers {
				handler(c)
				if c.IsAborted() {
					return
				}
			}
		})
	})

	a.engine = r
	a.server = &http.Server{Addr: flags.Listen, Handler: r}
	serverErr := make(chan error, 1)
	logger.Infof("server", "Metric storage upgrade progress is available on %s", flags.Listen)
	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case err := <-serverErr:
		return false, fmt.Errorf("listen in metric storage upgrade mode: %w", err)
	case <-quit:
		return false, a.Shutdown()
	case <-controller.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.server.Shutdown(ctx); err != nil {
			return false, fmt.Errorf("stop metric storage upgrade server: %w", err)
		}
		a.server = nil
		a.engine = nil
		return true, nil
	}
}

// StartBackground starts scheduled jobs.
func (a *App) StartBackground() error {
	stopMetricCleanup := metricstore.StartPendingCleanupWorker(dbcore.GetDBInstance())
	a.addCleanup("metric-cleanup", func(context.Context) error {
		stopMetricCleanup()
		return nil
	})

	registerScheduledWork()
	a.addCleanup("scheduler", func(context.Context) error {
		corn.StopAll()
		return nil
	})

	return nil
}

// registerReloadHandlers registers previously scattered config.Subscribe to the reload manager.
func (a *App) registerReloadHandlers(cors *security.CorsController) {
	// Switch OAuth providers.
	a.reload.Register("oauth-provider", func(event config.ConfigEvent) {
		if ok, t := config.IsChangedT[string](event, config.OAuthProviderKey); ok {
			if t == "" || t == "none" {
				t = "github"
			}
			oidcProvider, err := database.GetOidcConfigByName(t)
			if err != nil {
				logger.Errorf("server", "Failed to get OIDC provider config: %v", err)
				return
			}
			logger.Infof("server", "Using %s as OIDC provider", oidcProvider.Name)
			if err := oauth.LoadProvider(oidcProvider.Name, oidcProvider.Addition); err != nil {
				auditlog.EventLog("error", fmt.Sprintf("Failed to load OIDC provider: %v", err))
			}
		}
	})

	// Switch GeoIP providers.
	a.reload.Register("geoip-provider", func(event config.ConfigEvent) {
		if event.IsChanged(config.GeoIpProviderKey) {
			go geoip.InitGeoIp()
		}
	})

	// Switch the message sender.
	a.reload.Register("message-sender", func(event config.ConfigEvent) {
		if event.IsChanged(config.NotificationMethodKey) {
			go messageSender.Initialize()
		}
	})

	// Update the traffic-report schedule (times are interpreted in Beijing time).
	a.reload.Register("traffic-report-schedule", func(event config.ConfigEvent) {
		if event.IsChanged(config.TrafficReportTimeKey) {
			if err := notifier.ReloadTrafficReportSchedule(); err != nil {
				logger.Errorf("server", "Failed to reload traffic report schedule: %v", err)
			}
		}
	})

	// Apply CORS configuration updates.
	a.reload.Register("cors", func(event config.ConfigEvent) {
		cors.Update(event)
	})
}

// BuildRouter builds the Gin engine, middleware, and all routes, and registers the hot reload processor.
func (a *App) BuildRouter() error {
	if err := upload.DefaultStore.CleanupAll(); err != nil {
		logger.Errorf("upload", "Failed to clean interrupted uploads: %v", err)
	}
	r := gin.New()
	// The native deployment is either directly exposed or reverse-proxied by a
	// local Caddy instance. Never trust client-provided forwarding headers.
	if err := r.SetTrustedProxies(nil); err != nil {
		return fmt.Errorf("disable untrusted proxy headers: %w", err)
	}
	r.Use(func(c *gin.Context) { c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8<<20); c.Next() })
	r.Use(logger.GinLogger())
	r.Use(logger.GinRecovery())

	cors := security.NewCorsController(a.settings.CorsOriginCheckEnabled, a.settings.CorsAllowedOrigins)
	r.Use(cors.Middleware())

	r.Use(api.IdentityMiddleware())
	r.Use(api.PrivateSiteMiddleware())

	r.Use(func(c *gin.Context) {
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			c.Header("Cache-Control", "no-store")
		}
		c.Next()
	})

	router.Register(r)

	// Register and start hot-reload subscriptions in one place.
	a.registerReloadHandlers(cors)
	a.reload.Start()

	a.engine = r
	return nil
}

// Run starts the HTTP service and blocks until an interrupt signal is received or the service exits abnormally.
func (a *App) Run() error {
	a.server = &http.Server{
		Addr:    flags.Listen,
		Handler: httpsserver.Default.HTTPRedirectHandler(a.engine),
	}
	listener, err := listenAndFinalizeStartup(
		flags.Listen,
		a.CommitRestore,
		a.StartBackground,
	)
	if err != nil {
		a.onFatal(err)
		return err
	}

	httpsSettings, err := httpsserver.LoadSettings()
	if err != nil {
		logger.Errorf("https", "Failed to load built-in HTTPS settings: %v", err)
	} else if err := httpsserver.Default.Start(a.engine, httpsSettings, flags.Listen); err != nil {
		// Keep HTTP available so an invalid certificate or occupied HTTPS port
		// can still be corrected from the admin page.
		logger.Errorf("https", "Built-in HTTPS did not start: %v", err)
	}
	a.addCleanup("https-server", func(ctx context.Context) error {
		return httpsserver.Default.Shutdown(ctx)
	})

	serverErr := make(chan error, 1)
	logger.Infof("server", "Starting server on %s ...", flags.Listen)
	go func() {
		if err := a.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		a.onFatal(err)
		return fmt.Errorf("listen: %w", err)
	case <-quit:
		return a.Shutdown()
	}
}

func listenAndFinalizeStartup(
	address string,
	commitRestore func() error,
	startBackground func() error,
) (net.Listener, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			_ = listener.Close()
		}
	}()
	if err := commitRestore(); err != nil {
		return nil, fmt.Errorf("commit verified backup restore: %w", err)
	}
	if err := startBackground(); err != nil {
		return nil, fmt.Errorf("start background work: %w", err)
	}
	closeOnError = false
	return listener, nil
}

// Shutdown gracefully stops new HTTP requests, then runs registered cleanups in reverse order.
func (a *App) Shutdown() error {
	if a.dbReady {
		auditlog.Log("", "", "server is shutting down", "info")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shut down HTTP first so no new requests are accepted.
	if a.server != nil {
		if err := a.server.Shutdown(ctx); err != nil {
			logger.Infof("server", "HTTP server forced to shutdown: %v", err)
		}
	}

	// Release resources in reverse order (last in first out).
	for i := len(a.cleanups) - 1; i >= 0; i-- {
		c := a.cleanups[i]
		if err := c.fn(ctx); err != nil {
			logger.Errorf("server", "cleanup %q failed: %v", c.name, err)
		}
	}
	return nil
}

// onFatal handles fatal HTTP errors and attempts to release registered resources.
func (a *App) onFatal(err error) {
	if a.dbReady {
		auditlog.Log("", "", "server encountered a fatal error: "+err.Error(), "error")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i := len(a.cleanups) - 1; i >= 0; i-- {
		c := a.cleanups[i]
		if cerr := c.fn(ctx); cerr != nil {
			logger.Errorf("server", "cleanup %q failed: %v", c.name, cerr)
		}
	}
}

// registerScheduledWork sets up scheduled jobs and initial synchronization.
func registerScheduledWork() {
	if err := tasks.ReloadPingSchedule(); err != nil {
		logger.ErrorArgs("server", "Failed to reload ping schedule:", err)
	}
	if err := d_notification.ReloadLoadNotificationSchedule(); err != nil {
		logger.ErrorArgs("server", "Failed to reload load notification schedule:", err)
	}
	// Finish the one-time traffic-ledger backfill before compaction starts so a
	// low-resource server never performs both history scans concurrently.
	if err := d_notification.EnsureTrafficReportMetricRetention(context.Background()); err != nil {
		logger.Errorf("server", "Failed to ensure traffic report metric retention: %v", err)
	}

	if err := corn.AddFunc("records:cleanup", "@every 30m", cleanupScheduledData); err != nil {
		logger.ErrorArgs("server", "Failed to add cleanup scheduled task:", err)
	}
	if err := corn.AddContextFunc("metrics:compact", "@every 10s", true, compactMetricStore); err != nil {
		logger.ErrorArgs("server", "Failed to add metric compact scheduled task:", err)
	}
	if err := corn.AddFunc("notifier:traffic", "@every 1m", notifier.CheckTraffic); err != nil {
		logger.ErrorArgs("server", "Failed to add traffic notification task:", err)
	}
	if err := corn.AddFunc("notifier:expire", "0 0 9 * * *", notifier.CheckExpireScheduledWork); err != nil {
		logger.ErrorArgs("server", "Failed to add expire notification scheduled task:", err)
	}

	notifier.InitTrafficReportSchedule()
	notifier.InitPingLossNotificationSchedule()
}

func cleanupScheduledData() {
	auditlog.RemoveOldLogs()
	accounts.RemoveExpiredSessions()
}

func compactMetricStore(ctx context.Context) {
	compactCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	written, cycleCompleted, err := metricstore.CompactStep(compactCtx, time.Now().UTC())
	if errors.Is(err, metricstore.ErrCompactInProgress) {
		return
	}
	metricCompactCycle.add(written, err)
	if !cycleCompleted {
		return
	}
	cycleWritten, cycleErr := metricCompactCycle.finish()
	if cycleErr != nil {
		logger.Errorf("server", "Metric store compact cycle finished after writing %d rollup buckets: %v", cycleWritten, cycleErr)
		return
	}
	if cycleWritten > 0 {
		logger.Infof("server", "Metric store compacted %d rollup buckets", cycleWritten)
	}
}

type metricCompactCycleState struct {
	sync.Mutex
	written int
	errors  []error
}

var metricCompactCycle metricCompactCycleState

func (s *metricCompactCycleState) add(written int, err error) {
	s.Lock()
	defer s.Unlock()
	s.written += written
	if err != nil {
		s.errors = append(s.errors, err)
	}
}

func (s *metricCompactCycleState) finish() (int, error) {
	s.Lock()
	defer s.Unlock()
	written := s.written
	err := errors.Join(s.errors...)
	s.written = 0
	s.errors = nil
	return written, err
}
