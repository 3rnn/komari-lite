package utils

import (
	"os"
	"runtime"
	"strings"
)

// 部署形态标记：仅用于向面板/节点如实报告运行方式，不参与任何更新逻辑。
const (
	DeploymentDocker  = "docker"
	DeploymentLinux   = "linux"
	DeploymentWindows = "windows"
	DeploymentUnknown = "unknown"
)

// DeploymentType 返回当前进程的部署形态（容器 / Linux 二进制 / Windows / 未知）。
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
