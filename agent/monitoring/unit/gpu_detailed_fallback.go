//go:build !linux

package monitoring

import (
	"errors"
)

// DetailedGPUInfo holds detailed GPU information.
type DetailedGPUInfo struct {
	Name        string  `json:"name"`         // GPU model.
	MemoryTotal uint64  `json:"memory_total"` // Total VRAM (bytes).
	MemoryUsed  uint64  `json:"memory_used"`  // Used VRAM (bytes).
	Utilization float64 `json:"utilization"`  // GPU utilization (0-100).
	Temperature uint64  `json:"temperature"`  // Temperature (Celsius).
}

// GetDetailedGPUHost returns GPU models (fallback implementation).
func GetDetailedGPUHost() ([]string, error) {
	return nil, errors.New("detailed GPU monitoring not supported on this platform")
}

// GetDetailedGPUState returns GPU utilization (fallback implementation).
func GetDetailedGPUState() ([]float64, error) {
	return nil, errors.New("detailed GPU monitoring not supported on this platform")
}

// GetDetailedGPUInfo returns detailed GPU information (fallback implementation).
func GetDetailedGPUInfo() ([]DetailedGPUInfo, error) {
	return nil, errors.New("detailed GPU monitoring not supported on this platform")
}
