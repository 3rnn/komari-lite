package monitoring

import (
	"bufio"
	"os"
	"runtime"
	"strings"

	pkg_flags "github.com/nuomiiiii/lite-agent/cmd/flags"
	"github.com/shirou/gopsutil/v4/cpu"
)

var flags = pkg_flags.GlobalConfig

type CpuInfo struct {
	CPUName          string  `json:"cpu_name"`
	CPUArchitecture  string  `json:"cpu_architecture"`
	CPUCores         int     `json:"cpu_cores"`
	CPUPhysicalCores int     `json:"cpu_physical_cores"`
	CPUUsage         float64 `json:"cpu_usage"`
}

var cpuStaticCache ttlCache[CpuInfo]

func Cpu() CpuInfo {
	cpuinfo := CpuStaticInfo()

	percentages, err := cpu.Percent(0, false)
	if err == nil && len(percentages) > 0 {
		cpuinfo.CPUUsage = percentages[0]
	}

	return cpuinfo
}

func CpuStaticInfo() CpuInfo {
	return cpuStaticCache.get(0, readCPUStaticInfo)
}

func readCPUStaticInfo() CpuInfo {
	cpuinfo := CpuInfo{
		CPUName:          "Unknown",
		CPUArchitecture:  runtime.GOARCH,
		CPUCores:         1,
		CPUPhysicalCores: 0, // For compatibility with older Agents, 0 means unknown or unreported, not an actual core count.
		CPUUsage:         0.0,
	}

	// Prefer gopsutil for CPU information to avoid lscpu flooding lockdown logs on some kernels.
	info, err := cpu.Info()
	if err == nil && len(info) > 0 {
		cpuinfo.CPUName = strings.TrimSpace(info[0].ModelName)
		if cpuinfo.CPUName == "" {
			if info[0].VendorID != "" || info[0].Family != "" {
				cpuinfo.CPUName = strings.TrimSpace(info[0].VendorID + " " + info[0].Family)
			}
		}
	}

	if cpuinfo.CPUName == "Unknown" {
		name, err := readCPUNameFromProc()
		if err == nil && name != "" {
			cpuinfo.CPUName = strings.TrimSpace(name)
		}
	}

	cores, err := cpu.Counts(true)
	if err == nil && cores > 0 {
		cpuinfo.CPUCores = cores
	}

	physicalCores, err := cpu.Counts(false)
	if err == nil && physicalCores > 0 {
		cpuinfo.CPUPhysicalCores = physicalCores
	}

	return cpuinfo
}

// readCPUNameFromProc reads the CPU name from /proc/cpuinfo.
func readCPUNameFromProc() (string, error) {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Model\t") || strings.HasPrefix(line, "Hardware\t") || strings.HasPrefix(line, "Processor\t") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}

	return "", scanner.Err()
}
