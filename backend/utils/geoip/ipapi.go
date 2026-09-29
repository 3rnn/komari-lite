package geoip

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

// IPAPIService implements the GeoIPService interface using the ip-api.com service.
type IPAPIService struct {
	Client *http.Client
}

// ipAPIResponse defines the structure of the JSON response returned by the ip-api.com service.
type ipAPIResponse struct {
	Status      string  `json:"status"`
	Message     string  `json:"message"` // Appears when status is fail
	Country     string  `json:"country"`
	CountryCode string  `json:"countryCode"`
	Region      string  `json:"region"`
	RegionName  string  `json:"regionName"`
	City        string  `json:"city"`
	Zip         string  `json:"zip"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Timezone    string  `json:"timezone"`
	ISP         string  `json:"isp"`
	Org         string  `json:"org"`
	As          string  `json:"as"`
	Query       string  `json:"query"`
}

func (s *IPAPIService) Name() string {
	return "ip-api.com"
}

// NewIPAPIService Creates and returns a new instance of IPAPIService.
func NewIPAPIService() (*IPAPIService, error) {
	return &IPAPIService{
		Client: &http.Client{
			Timeout: 5 * time.Second, // Set request timeout
		},
	}, nil
}

// GetGeoInfo uses the ip-api.com service to retrieve geolocation information for a given IP address.
func (s *IPAPIService) GetGeoInfo(ip net.IP) (*GeoInfo, error) {
	// API URL, use the fields parameter to request only the required fields
	apiURL := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,message,country,countryCode", ip.String())

	resp, err := s.Client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get geo info from ip-api.com: %w", err)
	}
	defer resp.Body.Close()

	var apiResp ipAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode ip-api.com response: %w", err)
	}

	if apiResp.Status != "success" {
		return nil, fmt.Errorf("ip-api.com returned an error: %s", apiResp.Message)
	}

	return &GeoInfo{
		ISOCode: apiResp.CountryCode,
		Name:    apiResp.Country,
	}, nil
}

// UpdateDatabase is a no-op for ip-api.com since it is a web service.
func (s *IPAPIService) UpdateDatabase() error {
	// No action is required as the data is provided by an external service
	return nil
}

// Close is a no-op for ip-api.com since there are no persistent connections that need to be closed.
func (s *IPAPIService) Close() error {
	// No action required
	return nil
}
