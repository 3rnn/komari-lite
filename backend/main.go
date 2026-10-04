package main

import (
	"log/slog"
	"os"

	"github.com/komari-monitor/komari/cmd"
	"github.com/komari-monitor/komari/utils"
	logger "github.com/komari-monitor/komari/utils/log"
)

func main() {
	// Machine-readable probes must not carry the server startup log prefix.
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "schema-version" || os.Args[1] == "health") {
		cmd.Execute()
		return
	}
	if utils.VersionHash == "unknown" {
		logger.Setup(slog.LevelDebug)
	} else {
		logger.Setup(slog.LevelInfo)
	}

	logger.Infof("server", "Komari Monitor %s (%s)", utils.CurrentVersion, utils.VersionHash)

	cmd.Execute()
}
