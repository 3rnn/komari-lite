package geoip // Keep the same package name as geoip.go to indicate that they are part of the same package

import (
	"fmt"
	logger "github.com/komari-monitor/komari/utils/log"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath" // Added new import for processing file paths
	"sync"

	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/oschwald/maxminddb-golang"
)

// GeoIpUrl is the download address of the MaxMind database.
var GeoIpUrl = "https://raw.githubusercontent.com/Loyalsoldier/geoip/release/GeoLite2-Country.mmdb"

// GeoIpFilePath is the path where the MaxMind database is stored locally.
var GeoIpFilePath = "./data/GeoLite2-Country.mmdb"

// The GeoIpRecord structure defines the original structure of MaxMind database query results.
// It is specific to the MaxMind library and is used to parse data from .mmdb files.
type GeoIpRecord struct {
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
}

// MaxMindGeoIPService is a specific implementation of the GeoIPService interface.
// It uses MaxMind database as backend.
type MaxMindGeoIPService struct {
	// maxMindDBReader The MaxMind database reader instance held internally.
	maxMindDBReader *maxminddb.Reader
	// dbFilePath is the path to the MaxMind database file.
	dbFilePath string
	// mu is used to protect concurrent access to maxMindDBReader and ensure thread safety.
	mu sync.RWMutex
}

// Name returns the name of the service.
func (s *MaxMindGeoIPService) Name() string {
	return "MaxMind"
}

// NewMaxMindGeoIPService Creates and returns a MaxMindGeoIPService instance.
// It is responsible for initializing the service, including attempting to load or download the database.
func NewMaxMindGeoIPService() (*MaxMindGeoIPService, error) {
	dbFilePath := GeoIpFilePath
	service := &MaxMindGeoIPService{
		dbFilePath: dbFilePath,
	}

	// Make sure the data directory exists
	if err := os.MkdirAll(filepath.Dir(dbFilePath), os.ModePerm); err != nil {
		auditlog.Log("", "", "Failed to create data directory for MaxMind database: "+err.Error(), "error")
		return nil, fmt.Errorf("failed to create data directory for MaxMind database: %w", err)
	}

	// Check if the database file exists, if not try to download it
	if _, err := os.Stat(dbFilePath); os.IsNotExist(err) {
		if err := service.UpdateDatabase(); err != nil {
			auditlog.Log("", "", "Failed to download initial MaxMind database: "+err.Error(), "error")
			return nil, fmt.Errorf("failed to download initial MaxMind database: %w", err)
		}
	}

	// Initialize or reload the MaxMind database.
	if err := service.initialize(); err != nil {
		auditlog.Log("", "", "Failed to initialize MaxMind database: "+err.Error(), "error")
		return nil, fmt.Errorf("failed to initialize MaxMind database: %w", err)
	}
	return service, nil
}

// initialize Initializes or reloads the MaxMind database.
// This is an internal method called by NewMaxMindGeoIPService and UpdateDatabase.
// It closes the existing connection (if one exists) and reopens the database file.
func (s *MaxMindGeoIPService) initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If a database reader already exists, close it first.
	if s.maxMindDBReader != nil {
		s.maxMindDBReader.Close()
		s.maxMindDBReader = nil
	}

	// Try opening a new database file.
	reader, err := maxminddb.Open(s.dbFilePath)
	if err != nil {
		return fmt.Errorf("error opening MaxMind database at %s: %w", s.dbFilePath, err)
	}
	s.maxMindDBReader = reader
	return nil
}

// GetGeoInfo Gets MaxMind's geolocation information based on IP address.
// It queries the MaxMind database and converts its unique GeoIpRecord into a generic GeoInfo structure.
func (s *MaxMindGeoIPService) GetGeoInfo(ip net.IP) (*GeoInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.maxMindDBReader == nil {
		return nil, fmt.Errorf("MaxMind database is not initialized or failed to open")
	}
	if ip == nil {
		return nil, fmt.Errorf("IP address cannot be nil")
	}

	var record GeoIpRecord // Use the original GeoIpRecord structure to receive query results
	err := s.maxMindDBReader.Lookup(ip, &record)
	if err != nil {
		// Return errors, but avoid directly returning internal errors of the maxminddb library to provide more friendly information.
		return nil, fmt.Errorf("error looking up IP %s in MaxMind database: %w", ip.String(), err)
	}

	// Convert MaxMind-specific structures to generic GeoInfo structures
	geoInfo := &GeoInfo{
		ISOCode: record.Country.ISOCode,
		// Try to get the English country name, using the ISO code as a fallback if it doesn't exist
		Name: record.Country.Names["en"],
	}
	if geoInfo.Name == "" && geoInfo.ISOCode != "" {
		geoInfo.Name = geoInfo.ISOCode // If there is no English name, fall back to the ISO code
	}
	return geoInfo, nil
}

// UpdateDatabase implements the UpdateDatabase method of the GeoIPService interface.
// It downloads the latest GeoLite2-Country.mmdb file and reloads the database.
func (s *MaxMindGeoIPService) UpdateDatabase() error {
	s.mu.Lock() // Obtain a write lock to ensure mutual exclusivity of the update process

	resp, err := http.Get(GeoIpUrl) // GeoIpUrl is the predefined MaxMind database download address
	if err != nil {
		return fmt.Errorf("failed to initiate MaxMind database download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download MaxMind database: HTTP status %s", resp.Status)
	}

	// Make sure the data directory exists (NewMaxMindGeoIPService already handles this, but make sure again here in case of direct call)
	if err := os.MkdirAll(filepath.Dir(s.dbFilePath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create data directory for MaxMind database update: %w", err)
	}

	out, err := os.Create(s.dbFilePath) // Create or overwrite local database files
	if err != nil {
		return fmt.Errorf("failed to create MaxMind database file at %s: %w", s.dbFilePath, err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body) // Write the downloaded content to the file
	if err != nil {
		return fmt.Errorf("failed to write MaxMind database file: %w", err)
	}
	s.mu.Unlock() // The initialize method needs to be called after unlocking to avoid deadlock
	// Reload the database to use the newly downloaded file
	return s.initialize()
}

// Close implements the Close method of the GeoIPService interface.
// It closes the MaxMind database reader, releasing file handles and other resources.
func (s *MaxMindGeoIPService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.maxMindDBReader != nil {
		err := s.maxMindDBReader.Close()
		s.maxMindDBReader = nil // Clear reader instance
		if err != nil {
			return fmt.Errorf("error closing MaxMind database: %w", err)
		}
	}
	logger.InfoArgs("geoip", "MaxMind GeoIP service closed.")
	return nil
}
