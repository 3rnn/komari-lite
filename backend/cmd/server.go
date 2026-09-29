package cmd

import (
	logger "github.com/komari-monitor/komari/utils/log"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/spf13/cobra"
)

var ServerCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the server",
	Long:  `Start the server`,
	Run: func(cmd *cobra.Command, args []string) {
		RunServer()
	},
}

func init() {
	// Bind locally by default. Public deployments should use a reverse proxy and
	// pass an explicit --listen only when they have a deliberate exposure plan.
	listenAddr := GetEnv("KOMARI_LISTEN", "127.0.0.1:25774")
	ServerCmd.PersistentFlags().StringVarP(&flags.Listen, "listen", "l", listenAddr, "Listen address [env: KOMARI_LISTEN]")
	RootCmd.AddCommand(ServerCmd)
}

// RunServer starts the server through explicit lifecycle stages.
//
// See App (cmd/app.go) for each stage's responsibility and order. This function only coordinates them:
// an initialization failure aborts startup rather than serving requests from a partially initialized process.
func RunServer() {
	app := NewApp()
	if err := app.Bootstrap(); err != nil {
		_ = app.Shutdown()
		logger.Fatalf("server", "server startup failed at %q: %v", "bootstrap", err)
	}

	installRequired, err := app.InstallRequired()
	if err != nil {
		_ = app.Shutdown()
		logger.Fatalf("server", "server startup failed at %q: %v", "detect-first-run-install", err)
	}
	if installRequired {
		completed, err := app.RunInstallGuide()
		if err != nil {
			_ = app.Shutdown()
			logger.Fatalf("server", "server startup failed at %q: %v", "run-first-run-install", err)
		}
		if !completed {
			return
		}
	}

	required, summary, err := app.LegacyUpgradeRequired()
	if err != nil {
		_ = app.Shutdown()
		logger.Fatalf("server", "server startup failed at %q: %v", "detect-1.2.7-upgrade", err)
	}
	if required {
		completed, err := app.RunLegacyUpgrade(summary)
		if err != nil {
			_ = app.Shutdown()
			logger.Fatalf("server", "server startup failed at %q: %v", "run-1.2.7-upgrade", err)
		}
		if !completed {
			return
		}
	}

	storageUpgradeRequired, storageSummary, err := app.MetricStorageUpgradeRequired()
	if err != nil {
		_ = app.Shutdown()
		logger.Fatalf("server", "server startup failed at %q: %v", "detect-metric-storage-upgrade", err)
	}
	if storageUpgradeRequired {
		completed, err := app.RunMetricStorageUpgrade(storageSummary)
		if err != nil {
			_ = app.Shutdown()
			logger.Fatalf("server", "server startup failed at %q: %v", "run-metric-storage-upgrade", err)
		}
		if !completed {
			return
		}
	}

	// Initialization: do not serve requests after any stage fails.
	type stage struct {
		name string
		fn   func() error
	}
	stages := []stage{
		{"init-stores", app.InitStores},
		{"init-providers", app.InitProviders},
		{"build-router", app.BuildRouter},
	}
	for _, s := range stages {
		if err := s.fn(); err != nil {
			// Release registered resources where possible before exiting.
			_ = app.Shutdown()
			logger.Fatalf("server", "server startup failed at %q: %v", s.name, err)
		}
	}

	if err := app.Run(); err != nil {
		logger.Fatalf("server", "server exited with error: %v", err)
	}
}
