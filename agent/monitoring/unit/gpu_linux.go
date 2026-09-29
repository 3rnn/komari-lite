//go:build linux
// +build linux

package monitoring

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func GpuName() string {
	if name := getFromLspci(); name != "None" {
		return name
	}

	if name := getFromSysfsDRM(); name != "None" {
		return name
	}
	return "None"
}

func getFromLspci() string {
	out, err := exec.Command("lspci").Output()
	if err != nil {
		return "None"
	}
	excludePatterns := []string{
		"^1111",                             // 1111 (rev 02)
		`(?i)^cirrus logic (cl[-\s]?)?gd 5`, // CL-GD series from the mid-1990s, now often found in VMs.
		"(?i)virtio",
		"(?i)vmware",
		`(?i)qxl`, // SPICE virtual graphics adapter.
		`(?i)hyper-v`,
	}

	lines := strings.Split(string(out), "\n")

	priorityVendors := []string{"nvidia", "amd", "radeon", "intel", "arc", "snap", "qualcomm", "snapdragon"}

	isExcludedGPUName := func(name string) bool {
		for _, pattern := range excludePatterns {
			if matched, _ := regexp.MatchString(pattern, name); matched {
				return true
			}
		}
		return false
	}

	var result []string
	appendName := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || isExcludedGPUName(name) {
			return
		}
		result = append(result, name)
	}

	extractName := func(line string) string {
		// Take the text after the final colon.
		idx := strings.LastIndex(line, ":")
		if idx == -1 || idx == len(line)-1 {
			return ""
		}
		name := strings.TrimSpace(line[idx+1:])

		// Strip a trailing (rev xx).
		if parenIdx := strings.LastIndex(name, "("); parenIdx != -1 {
			name = strings.TrimSpace(name[:parenIdx])
		}
		return name
	}

	// Find priorityVendors.
	for _, line := range lines {
		lower := strings.ToLower(line)

		// Confirm this is a display device, not an Intel NIC or Qualcomm Bluetooth adapter.
		if !strings.Contains(lower, "vga") && !strings.Contains(lower, "3d") && !strings.Contains(lower, "display") {
			continue
		}

		for _, vendor := range priorityVendors {
			if strings.Contains(lower, vendor) {
				name := extractName(line)
				appendName(name)
				break
			}
		}
	}

	if len(result) > 0 {
		return formatGPUNameList(result)
	}

	// Any VGA device not on the denylist.
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "vga") || strings.Contains(lower, "3d") || strings.Contains(lower, "display") {
			name := extractName(line)
			appendName(name)
		}
	}

	if len(result) > 0 {
		return formatGPUNameList(result)
	}

	return "None"

}

func getFromSysfsDRM() string {
	matches, _ := filepath.Glob("/sys/class/drm/card*")

	excludedDrivers := map[string]bool{
		"virtio-pci":  true,
		"virtio_gpu":  true,
		"bochs-drm":   true,
		"qxl":         true,
		"vmwgfx":      true,
		"cirrus":      true,
		"vboxvideo":   true,
		"hyperv_fb":   true,
		"simpledrm":   true,
		"simplefb":    true,
		"cirrus-qemu": true,
	}

	var result []string
	appendName := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || isExcludedSysfsGPUName(name) {
			return
		}
		result = append(result, name)
	}

	for _, path := range matches {
		if !isDRMCardPath(path) {
			continue
		}

		// Driver name.
		driverLink, err := os.Readlink(filepath.Join(path, "device", "driver"))
		if err != nil {
			continue
		}
		driverName := filepath.Base(driverLink)

		if excludedDrivers[driverName] {
			continue
		}

		// Extract the model from the device-tree compatible string.
		// /sys/class/drm/card0/device/of_node/compatible
		// "qcom,adreno-750.1\0qcom,adreno"
		exactModel := ""
		compatibleBytes, err := os.ReadFile(filepath.Join(path, "device", "of_node", "compatible"))
		if err == nil {
			exactModel = parseSocModel(driverName, compatibleBytes)
		}

		// Return a specific model if found.
		if exactModel != "" {
			appendName(exactModel)
			continue
		}

		// Map generic driver names.
		switch driverName {
		case "vc4", "vc4-drm":
			appendName("Broadcom VideoCore IV/VI (Raspberry Pi)")
		case "v3d", "v3d-drm":
			appendName("Broadcom V3D (Raspberry Pi 4/5)")
		case "msm", "msm_drm":
			appendName("Qualcomm Adreno (Unknown Model)")
		case "panfrost":
			appendName("ARM Mali (Panfrost)")
		case "lima":
			appendName("ARM Mali (Lima)")
		case "sun4i-drm", "sunxi-drm":
			appendName("Allwinner Display Engine")
		case "tegra":
			appendName("NVIDIA Tegra")
		case "ast": // LXC container exposing a physical GPU.
			appendName("ASPEED Technology, Inc. ASPEED Graphics Family")
		case "i915", "i915-drm":
			appendName("Intel Integrated Graphics")
		case "mgag200":
			appendName("Matrox G200 Series")
		default:
			if driverName != "" {
				appendName("Direct Render Manager " + driverName)
			}
		}
	}

	if len(result) > 0 {
		return formatGPUNameList(result)
	}

	// Development board model.
	modelData, err := os.ReadFile("/sys/firmware/devicetree/base/model")
	if err == nil {
		model := string(modelData)
		if strings.Contains(model, "Raspberry Pi") {
			return "Broadcom VideoCore (Integrated)"
		}
	}

	return "None"
}

func isDRMCardPath(path string) bool {
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "card") || len(base) == len("card") {
		return false
	}
	for _, char := range base[len("card"):] {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func isExcludedSysfsGPUName(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "virtio") ||
		strings.Contains(lower, "vmware") ||
		strings.Contains(lower, "qxl") ||
		strings.Contains(lower, "hyper-v") ||
		strings.Contains(lower, "cirrus")
}

// parseSocModel extracts a human-readable model from device-tree compatible strings.
func parseSocModel(driver string, rawBytes []byte) string {
	// The compatible file has multiple NUL-separated strings.
	content := string(bytes.ReplaceAll(rawBytes, []byte{0}, []byte(" ")))
	lower := strings.ToLower(content)

	// Qualcomm Adreno.
	if driver == "msm" || strings.Contains(lower, "adreno") {
		// "adreno-750", "adreno-660"
		re := regexp.MustCompile(`adreno[-_](\d+)`)
		matches := re.FindStringSubmatch(lower)
		if len(matches) > 1 {
			return "Qualcomm Adreno " + matches[1]
		}
		return "Qualcomm Adreno"
	}

	// ARM Mali (Rockchip/MediaTek/AmLogic)
	if driver == "panfrost" || driver == "lima" || strings.Contains(lower, "mali") {
		// "mali-g610", "mali-t860"
		re := regexp.MustCompile(`mali[-_]([a-z]\d+)`)
		matches := re.FindStringSubmatch(lower)
		if len(matches) > 1 {
			return "ARM Mali " + strings.ToUpper(matches[1]) // Mali G610
		}
		return "ARM Mali" // Generic name.
	}

	// Raspberry Pi VideoCore.
	if driver == "vc4" || driver == "vc4-drm" || driver == "v3d" {
		if strings.Contains(lower, "bcm2712") {
			return "Broadcom VideoCore VII (Pi 5)"
		}
		if strings.Contains(lower, "bcm2711") {
			return "Broadcom VideoCore VI (Pi 4)"
		}
		if strings.Contains(lower, "bcm2837") || strings.Contains(lower, "bcm2835") {
			return "Broadcom VideoCore IV"
		}
	}

	// Allwinner.
	// "allwinner,sun50i-h6-display-engine"
	if strings.Contains(lower, "allwinner") || strings.Contains(lower, "sun50i") || strings.Contains(lower, "sun8i") {
		re := regexp.MustCompile(`sun\d+i-([a-z0-9]+)`)
		matches := re.FindStringSubmatch(lower)
		if len(matches) > 1 {
			model := strings.ToUpper(matches[1])
			return "Allwinner " + model
		}
		return "Allwinner Display Engine"
	}

	// NVIDIA Tegra
	if driver == "tegra" {
		if strings.Contains(lower, "tegra194") {
			return "NVIDIA Tegra Xavier"
		}
		if strings.Contains(lower, "tegra234") {
			return "NVIDIA Orin"
		}
		if strings.Contains(lower, "tegra210") {
			return "NVIDIA Tegra X1"
		}
	}

	return ""
}
