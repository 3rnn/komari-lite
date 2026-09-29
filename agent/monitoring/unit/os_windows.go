//go:build windows
// +build windows

package monitoring

import (
	"strconv"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func OSName() string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return "Microsoft Windows"
	}
	defer key.Close()

	productName, _, err := key.GetStringValue("ProductName")
	if err != nil {
		return "Microsoft Windows"
	}

	// Preserve Server edition names.
	if strings.Contains(productName, "Server") {
		return productName
	}

	// If the registry already names Windows 11, return it as is.
	if strings.Contains(productName, "Windows 11") {
		return productName
	}

	// Windows 11 starts at build 22000. DisplayVersion can also be 21H2 on Windows 10, so it is not definitive.
	buildNumberStr, _, err := key.GetStringValue("CurrentBuild")
	if err == nil {
		if buildNumber, err2 := strconv.Atoi(buildNumberStr); err2 == nil && buildNumber >= 22000 {
			// Older registry fields may still say Windows 10; replace that prefix with Windows 11.
			if strings.HasPrefix(productName, "Windows 10 ") {
				edition := strings.TrimPrefix(productName, "Windows 10 ")
				return "Windows 11 " + edition
			}
			if productName == "Windows 10" { // Minimal registry data case.
				return "Windows 11"
			}
			// If the name lacks a Windows 10 prefix but build >= 22000, prefix Windows 11 to the remaining name.
			if !strings.Contains(productName, "Windows 11") {
				return strings.Replace(productName, "Windows 10", "Windows 11", 1)
			}
		}
	}

	return productName
}

// KernelVersion returns the kernel version on Windows systems (build number)
func KernelVersion() string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return "Unknown"
	}
	defer key.Close()

	// Get current build number
	buildNumber, _, err := key.GetStringValue("CurrentBuild")
	if err != nil {
		return "Unknown"
	}

	// Get UBR (Update Build Revision) if available
	ubr, _, err := key.GetIntegerValue("UBR")
	if err != nil {
		// UBR not available, just return build number
		return buildNumber
	}

	return buildNumber + "." + strconv.FormatUint(ubr, 10)
}
