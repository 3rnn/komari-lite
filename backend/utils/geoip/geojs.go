package geoip

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

// GeoJSService implements the GeoIPService interface using geojs.io services.
type GeoJSService struct {
	Client *http.Client
}

// geoJSResponse defines the structure of the JSON response returned by the geojs.io service.
// We only define the fields we need.
type geoJSResponse struct {
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	// Additional fields can be added as needed, for example:
	// City    string `json:"city"`
	// Region  string `json:"region"`
}

// NewGeoJSService Creates and returns a new instance of GeoJSService.
func NewGeoJSService() (*GeoJSService, error) {
	return &GeoJSService{
		Client: &http.Client{
			Timeout: 5 * time.Second, // Set a reasonable timeout
		},
	}, nil
}

// Name returns the name of the service.
func (s *GeoJSService) Name() string {
	return "geojs.io"
}

// GetGeoInfo uses the geojs.io service to retrieve geolocation information for a given IP address.
func (s *GeoJSService) GetGeoInfo(ip net.IP) (*GeoInfo, error) {
	// API endpoints for GeoJS
	apiURL := fmt.Sprintf("https://get.geojs.io/v1/ip/geo/%s.json", ip.String())

	resp, err := s.Client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get geo info from geojs.io: %w", err)
	}
	defer resp.Body.Close()

	// Check response status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geojs.io returned non-200 status code: %d", resp.StatusCode)
	}

	var apiResp geoJSResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode geojs.io response: %w", err)
	}

	// Check if the country code is empty as geojs may return 200 OK for invalid/private IP but with empty content
	if apiResp.CountryCode == "" {
		return nil, fmt.Errorf("geojs.io returned empty geo info for ip: %s", ip.String())
	}

	return &GeoInfo{
		ISOCode: apiResp.CountryCode,
		Name:    apiResp.Country,
	}, nil
}

// UpdateDatabase is a no-op for geojs.io since it is a web service.
func (s *GeoJSService) UpdateDatabase() error {
	return nil
}

// Close is a no-op for geojs.io.
func (s *GeoJSService) Close() error {
	return nil
}
