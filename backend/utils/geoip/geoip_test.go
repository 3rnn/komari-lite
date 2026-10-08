package geoip_test

import (
	"net"
	"testing"

	"github.com/komari-monitor/komari/utils/geoip"
)

// Test the initialization and update functions of the GeoIP database.
func TestMmdb(t *testing.T) {
	provider, _ := geoip.NewMaxMindGeoIPService()
	geoip.SetCurrentProvider(provider)
	testIPAddr(t)
}
func TestIPAPI(t *testing.T) {
	provider, _ := geoip.NewIPAPIService()
	geoip.SetCurrentProvider(provider)
	testIPAddr(t)
}

func TestGeoJS(t *testing.T) {
	provider, _ := geoip.NewGeoJSService()
	geoip.SetCurrentProvider(provider)
	testIPAddr(t)
}

func TestIPInfo(t *testing.T) {
	provider, _ := geoip.NewIPInfoService()
	geoip.SetCurrentProvider(provider)
	testIPAddr(t)
}

func testIPAddr(t *testing.T) {
	t.Helper()
	for _, ipaddr := range []string{"8.8.8.8", "2001:4860:4860::8888"} {
		record, err := geoip.GetGeoInfo(net.ParseIP(ipaddr))
		if err != nil {
			t.Errorf("failed to get GeoIP info for %s: %v", ipaddr, err)
			continue
		}
		if record == nil || (record.ISOCode == "" && record.Name == "") {
			t.Errorf("country information is missing for %s", ipaddr)
		}
	}
}

func TestUnicodeEmoji(t *testing.T) {
	isoCode := "CN"
	emoji := geoip.GetRegionUnicodeEmoji(isoCode)
	if emoji != "🇨🇳" {
		t.Errorf("expected emoji for %s, got %s", isoCode, emoji)
	}
}
