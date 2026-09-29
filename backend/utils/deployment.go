package utils

import (
	"os"
	"runtime"
	"strings"
)

// Deployment form tag: only used to truthfully report the operating mode to the panel/node and does not participate in any update logic.
const (
	DeploymentDocker  = "docker"
	DeploymentLinux   = "linux"
	DeploymentWindows = "windows"
	DeploymentUnknown = "unknown"
)

// DeploymentType returns the deployment form of the current process (container/Linux binary/Windows/unknown).
func DeploymentType() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("KOMARI_DEPLOYMENT"))) {
	case DeploymentDocker:
		return DeploymentDocker
	case DeploymentLinux, "binary":
		return DeploymentLinux
	case DeploymentWindows:
		return DeploymentWindows
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return DeploymentDocker
	}
	if runtime.GOOS == "linux" {
		return DeploymentLinux
	}
	if runtime.GOOS == "windows" {
		return DeploymentWindows
	}
	return DeploymentUnknown
}
