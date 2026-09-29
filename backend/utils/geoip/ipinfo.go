package geoip

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

// IPInfoService implements the GeoIPService interface using the ipinfo.io service.
type IPInfoService struct {
	Client *http.Client
	// 1000 requests per day, limit shared by the owner of the IP address.
	// APIToken string
}

// ipInfoResponse defines the structure of the JSON response returned by the ipinfo.io service, including only fields available for free quota.
type ipInfoResponse struct {
	IP          string `json:"ip"`
	Hostname    string `json:"hostname"`
	City        string `json:"city"`
	Region      string `json:"region"`
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"` // ipinfo.io returns the ISO code of "country". In order to be consistent with GeoInfo, an additional CountryCode is added here.
	Loc         string `json:"loc"`         // Latitude,Longitude
	Org         string `json:"org"`
	Postal      string `json:"postal"`
	Timezone    string `json:"timezone"`
}

// NewIPInfoService Creates and returns a new instance of IPInfoService.
func NewIPInfoService() (*IPInfoService, error) {
	return &IPInfoService{
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}, nil
}

// Name returns the name of the service.
func (s *IPInfoService) Name() string {
	return "ipinfo.io"
}

// GetGeoInfo uses the ipinfo.io service to retrieve geolocation information for a given IP address.
// The free quota mainly provides country information.
func (s *IPInfoService) GetGeoInfo(ip net.IP) (*GeoInfo, error) {
	// IPinfo free quota does not require an API token to query basic IP information.
	// API URL: https://ipinfo.io/json (query your own IP) or https://ipinfo.io/YOUR_IP/json
	apiURL := fmt.Sprintf("https://ipinfo.io/%s/json", ip.String())

	resp, err := s.Client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get geo info from ipinfo.io: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ipinfo.io returned non-200 status: %d %s", resp.StatusCode, resp.Status)
	}

	var apiResp ipInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode ipinfo.io response: %w", err)
	}

	// The "country" field of IPinfo directly returns the ISO 2-letter code, such as "US", "CN"
	// We need the "country" field as ISOCode and try to get its corresponding country name.
	// The full country name is usually not provided directly in the IPinfo response, but we can map it via the CountryCode.
	// To simplify and conform to the GeoInfo structure, we directly use Country as the ISOCode and try to get the name from the CountryCode.
	// In fact, the 'country' field of IPinfo is the ISO 2-letter code.
	// If the full country name is required, a local ISO code to name mapping may be required.
	// For compatibility with the GetRegionUnicodeEmoji function, we use country directly as the ISOCode.
	return &GeoInfo{
		ISOCode: apiResp.Country, // The 'country' field of IPinfo is the ISO 2-letter code
		Name:    apiResp.Country, // Free quota usually only provides ISO encoding. ISO encoding is temporarily used as the name here.
	}, nil
}

// UpdateDatabase is a no-op for ipinfo.io since it is a web service.
func (s *IPInfoService) UpdateDatabase() error {
	// No action is required as the data is provided by an external service
	return nil
}

// Close is a no-op for ipinfo.io since there are no persistent connections that need to be closed.
func (s *IPInfoService) Close() error {
	// No action required
	return nil
}
