package geoip

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/komari-monitor/komari/pkg/config"
	logger "github.com/komari-monitor/komari/utils/log"
	"github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

var providerMu sync.RWMutex
var currentProvider GeoIPService
var geoCache *cache.Cache
var geoLookupGroup singleflight.Group

const geoIPFailureCacheTTL = 5 * time.Minute

type geoCacheEntry struct {
	info    *GeoInfo
	errText string
}

type GeoInfo struct {
	ISOCode string
	Name    string
}

func init() {
	currentProvider = &EmptyProvider{}
	geoCache = cache.New(48*time.Hour, time.Hour)
}

// The GeoIPService interface defines the core method for obtaining geographical location information.
// Any type that implements this interface can serve as a geolocation service provider.
type GeoIPService interface {
	Name() string
	GetGeoInfo(ip net.IP) (*GeoInfo, error)
	UpdateDatabase() error
	Close() error
}

// CurrentProvider returns a stable provider snapshot for an individual lookup.
func CurrentProvider() GeoIPService {
	providerMu.RLock()
	defer providerMu.RUnlock()
	return currentProvider
}

// SetCurrentProvider atomically replaces the provider and clears values that
// were produced by its predecessor. It is safe to call while requests are
// performing lookups.
func SetCurrentProvider(provider GeoIPService) {
	if provider == nil {
		provider = &EmptyProvider{}
	}
	providerMu.Lock()
	old := currentProvider
	currentProvider = provider
	providerMu.Unlock()
	geoCache.Flush()
	if old != nil && old != provider {
		if err := old.Close(); err != nil {
			logger.Warn("geoip", "failed to close replaced GeoIP provider", "error", err)
		}
	}
}

func GetRegionUnicodeEmoji(isoCode string) string {
	if len(isoCode) != 2 {
		return ""
	}
	isoCode = strings.ToUpper(isoCode)
	if !unicode.IsLetter(rune(isoCode[0])) || !unicode.IsLetter(rune(isoCode[1])) {
		return ""
	}
	rune1 := rune(0x1F1E6 + (rune(isoCode[0]) - 'A'))
	rune2 := rune(0x1F1E6 + (rune(isoCode[1]) - 'A'))
	return string(rune1) + string(rune2)
}

func InitGeoIp() {
	conf, err := config.GetMany(map[string]any{
		config.GeoIpEnabledKey:  true,
		config.GeoIpProviderKey: "ipinfo",
	})
	if err != nil {
		panic("Failed to get configuration for GeoIP: " + err.Error())
	}
	if !conf[config.GeoIpEnabledKey].(bool) {
		SetCurrentProvider(&EmptyProvider{})
		return
	}

	var provider GeoIPService
	switch conf[config.GeoIpProviderKey].(string) {
	case "mmdb":
		provider, err = NewMaxMindGeoIPService()
	case "ip-api":
		provider, err = NewIPAPIService()
	case "geojs":
		provider, err = NewGeoJSService()
	case "ipinfo":
		provider, err = NewIPInfoService()
	default:
		SetCurrentProvider(&EmptyProvider{})
		return
	}
	if err != nil || provider == nil {
		logger.Error("geoip", "failed to initialize GeoIP provider; using EmptyProvider", "provider", conf[config.GeoIpProviderKey], "error", err)
		SetCurrentProvider(&EmptyProvider{})
		return
	}
	SetCurrentProvider(provider)
	logger.Info("geoip", "using GeoIP provider", "provider", provider.Name())
}

// Shutdown closes the resources held by the current GeoIP provider (such as mmdb file handle). Called by the shutdown process.
func Shutdown() error {
	provider := CurrentProvider()
	if provider == nil {
		return nil
	}
	return provider.Close()
}

func GetGeoInfo(ip net.IP) (*GeoInfo, error) {
	provider := CurrentProvider()
	if provider == nil {
		return nil, fmt.Errorf("GeoIP provider is unavailable")
	}
	cacheKey := provider.Name() + ":" + ip.String()
	if cached, found := geoCache.Get(cacheKey); found {
		return unpackGeoCacheEntry(cached.(geoCacheEntry))
	}

	result, err, _ := geoLookupGroup.Do(cacheKey, func() (any, error) {
		if cached, found := geoCache.Get(cacheKey); found {
			return cached.(geoCacheEntry), nil
		}
		info, lookupErr := provider.GetGeoInfo(ip)
		entry := geoCacheEntry{info: info}
		if lookupErr != nil || info == nil {
			if lookupErr != nil {
				entry.errText = lookupErr.Error()
			} else {
				entry.errText = "provider returned no result"
			}
			geoCache.Set(cacheKey, entry, geoIPFailureCacheTTL)
			return entry, nil
		}
		geoCache.Set(cacheKey, entry, cache.DefaultExpiration)
		return entry, nil
	})
	if err != nil {
		return nil, err
	}
	return unpackGeoCacheEntry(result.(geoCacheEntry))
}

func unpackGeoCacheEntry(entry geoCacheEntry) (*GeoInfo, error) {
	if entry.errText != "" {
		return nil, fmt.Errorf("GeoIP lookup failed: %s", entry.errText)
	}
	return entry.info, nil
}

func UpdateDatabase() error {
	err := CurrentProvider().UpdateDatabase()
	if err == nil {
		geoCache.Flush()
		logger.Info("geoip", "cache cleared due to database update")
	}
	return err
}
