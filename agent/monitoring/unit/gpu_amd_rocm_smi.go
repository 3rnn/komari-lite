package monitoring

// Modified from https://github.com/influxdata/telegraf/blob/master/plugins/inputs/amd_rocm_smi/amd_rocm_smi.go
// Original License: MIT

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type ROCmSMI struct {
	BinPath string
	data    []byte
}

// AMDGPUInfo holds detailed AMD GPU information.
type AMDGPUInfo struct {
	Name        string  // GPU model.
	MemoryTotal uint64  // Total VRAM (bytes).
	MemoryUsed  uint64  // Used VRAM (bytes).
	Utilization float64 // GPU utilization (0-100).
	Temperature uint64  // Temperature (Celsius).
}

// ROCmSMI JSON response structure.
type ROCmResponse map[string]ROCmGPUInfo

type ROCmGPUInfo struct {
	CardSeries          string `json:"Card series"`
	GPUUsage            string `json:"GPU use (%)"`
	VRAMTotalMemory     string `json:"VRAM Total Memory (B)"`
	VRAMTotalUsedMemory string `json:"VRAM Total Used Memory (B)"`
	TemperatureJunction string `json:"Temperature (Sensor junction) (C)"`
}

func (rsmi *ROCmSMI) GatherModel() ([]string, error) {
	return rsmi.gatherModel()
}

func (rsmi *ROCmSMI) GatherUsage() ([]float64, error) {
	return rsmi.gatherUsage()
}

// GatherDetailedInfo retrieves detailed GPU information.
func (rsmi *ROCmSMI) GatherDetailedInfo() ([]AMDGPUInfo, error) {
	return rsmi.gatherDetailedInfo()
}

func (rsmi *ROCmSMI) Start() error {
	if _, err := os.Stat(rsmi.BinPath); os.IsNotExist(err) {
		binPath, err := exec.LookPath("rocm-smi")
		if err != nil {
			return errors.New("rocm-smi tool not found")
		}
		rsmi.BinPath = binPath
	}

	rsmi.data = rsmi.pollROCmSMI()
	return nil
}

func (rsmi *ROCmSMI) pollROCmSMI() []byte {
	cmd := exec.Command(rsmi.BinPath, "--showallinfo", "--json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}
	return output
}

func (rsmi *ROCmSMI) gatherModel() ([]string, error) {
	var data map[string]interface{}
	var models []string

	if err := json.Unmarshal(rsmi.data, &data); err != nil {
		return nil, err
	}

	// Parse GPU models from JSON.
	for key, value := range data {
		if strings.HasPrefix(key, "card") {
			if cardData, ok := value.(map[string]interface{}); ok {
				if name, exists := cardData["Card series"]; exists {
					if nameStr, ok := name.(string); ok && nameStr != "" {
						models = append(models, nameStr)
					}
				}
			}
		}
	}

	return models, nil
}

func (rsmi *ROCmSMI) gatherUsage() ([]float64, error) {
	var data map[string]interface{}
	var usageList []float64

	if err := json.Unmarshal(rsmi.data, &data); err != nil {
		return nil, err
	}

	// Parse GPU utilization from JSON.
	for key, value := range data {
		if strings.HasPrefix(key, "card") {
			if cardData, ok := value.(map[string]interface{}); ok {
				usage := 0.0
				if utilizationData, exists := cardData["GPU use (%)"]; exists {
					if utilizationStr, ok := utilizationData.(string); ok {
						if parsed, err := parseAMDPercentage(utilizationStr); err == nil {
							usage = parsed
						}
					}
				}
				usageList = append(usageList, usage)
			}
		}
	}

	return usageList, nil
}

func (rsmi *ROCmSMI) gatherDetailedInfo() ([]AMDGPUInfo, error) {
	if rsmi.data == nil {
		return nil, errors.New("no data available")
	}

	var data map[string]interface{}
	var gpuInfos []AMDGPUInfo

	if err := json.Unmarshal(rsmi.data, &data); err != nil {
		return nil, err
	}

	// Parse each GPU device.
	for key, value := range data {
		if strings.HasPrefix(key, "card") {
			if cardData, ok := value.(map[string]interface{}); ok {
				gpuInfo := AMDGPUInfo{}

				// Get GPU name.
				if name, exists := cardData["Card series"]; exists {
					if nameStr, ok := name.(string); ok {
						gpuInfo.Name = nameStr
					}
				}

				// Get utilization.
				if utilizationData, exists := cardData["GPU use (%)"]; exists {
					if utilizationStr, ok := utilizationData.(string); ok {
						if usage, err := parseAMDPercentage(utilizationStr); err == nil {
							gpuInfo.Utilization = usage
						}
					}
				}

				// Get VRAM information.
				if memUsedData, exists := cardData["VRAM Total Used Memory (B)"]; exists {
					if memUsedStr, ok := memUsedData.(string); ok {
						if memUsed, err := parseAMDMemoryBytes(memUsedStr); err == nil {
							gpuInfo.MemoryUsed = memUsed
						}
					}
				}

				if memTotalData, exists := cardData["VRAM Total Memory (B)"]; exists {
					if memTotalStr, ok := memTotalData.(string); ok {
						if memTotal, err := parseAMDMemoryBytes(memTotalStr); err == nil {
							gpuInfo.MemoryTotal = memTotal
						}
					}
				}

				// Get temperature.
				if tempData, exists := cardData["Temperature (Sensor junction) (C)"]; exists {
					if tempStr, ok := tempData.(string); ok {
						if temp, err := parseAMDTemperature(tempStr); err == nil {
							gpuInfo.Temperature = temp
						}
					}
				}

				gpuInfos = append(gpuInfos, gpuInfo)
			}
		}
	}

	return gpuInfos, nil
}

// Parse an AMD percentage (e.g. "25" -> 25.0).
func parseAMDPercentage(value string) (float64, error) {
	cleaned := strings.TrimSpace(value)
	cleaned = strings.TrimSuffix(cleaned, "%")
	cleaned = strings.TrimSpace(cleaned)

	if cleaned == "" {
		return 0.0, nil
	}

	result, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0.0, err
	}

	return result, nil
}

// Parse AMD VRAM bytes (e.g. "1073741824" -> 1073741824 bytes).
func parseAMDMemoryBytes(value string) (uint64, error) {
	cleaned := strings.TrimSpace(value)

	if cleaned == "" {
		return 0, nil
	}

	bytes, err := strconv.ParseUint(cleaned, 10, 64)
	if err != nil {
		return 0, err
	}

	// Return the byte count directly.
	return bytes, nil
}

// Parse an AMD temperature (e.g. "65" -> 65).
func parseAMDTemperature(value string) (uint64, error) {
	cleaned := strings.TrimSpace(value)
	cleaned = strings.TrimSuffix(cleaned, "C")
	cleaned = strings.TrimSpace(cleaned)

	if cleaned == "" {
		return 0, nil
	}

	result, err := strconv.ParseUint(cleaned, 10, 64)
	if err != nil {
		return 0, err
	}

	return result, nil
}
