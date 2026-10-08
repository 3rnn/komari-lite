package client

import (
	"errors"
	"net"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/config"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	"github.com/komari-monitor/komari/utils/geoip"
)

type staticGeoIPProvider struct {
	name string
	iso  string
}

func (p staticGeoIPProvider) Name() string {
	return p.name
}

func (p staticGeoIPProvider) GetGeoInfo(ip net.IP) (*geoip.GeoInfo, error) {
	return &geoip.GeoInfo{ISOCode: p.iso, Name: p.iso}, nil
}

func (p staticGeoIPProvider) UpdateDatabase() error {
	return nil
}

func (p staticGeoIPProvider) Close() error {
	return nil
}

type recordingGeoIPProvider struct {
	name  string
	calls []string
	err   error
}

func (p *recordingGeoIPProvider) Name() string { return p.name }

func (p *recordingGeoIPProvider) GetGeoInfo(ip net.IP) (*geoip.GeoInfo, error) {
	p.calls = append(p.calls, ip.String())
	if p.err != nil {
		return nil, p.err
	}
	return &geoip.GeoInfo{ISOCode: "SG", Name: "Singapore"}, nil
}

func (p *recordingGeoIPProvider) UpdateDatabase() error { return nil }

func (p *recordingGeoIPProvider) Close() error { return nil }

func TestAppendClientRegionFromGeoIPSelectsOnlyReportedPublicAddresses(t *testing.T) {
	flags.DatabaseType = "sqlite"
	flags.DatabaseFile = "file:basic_info_geoip_selection?mode=memory&cache=shared"
	_ = dbcore.GetDBInstance()
	if err := config.Set(config.GeoIpEnabledKey, true); err != nil {
		t.Fatalf("enable geoip: %v", err)
	}

	oldProvider := geoip.CurrentProvider()
	t.Cleanup(func() { geoip.SetCurrentProvider(oldProvider) })

	tests := []struct {
		name      string
		info      map[string]interface{}
		wantCalls []string
	}{
		{
			name: "reported IPv4 before IPv6 and inventory",
			info: map[string]interface{}{
				"ipv4": "8.8.8.8", "ipv6": "2001:4860::1",
				"ip_addresses": []map[string]interface{}{{"address": "9.9.9.9", "family": "ipv4"}},
			},
			wantCalls: []string{"8.8.8.8"},
		},
		{
			name: "reported IPv6 before inventory",
			info: map[string]interface{}{
				"ipv6":         "2001:4860::1",
				"ip_addresses": []map[string]interface{}{{"address": "9.9.9.9", "family": "ipv4"}},
			},
			wantCalls: []string{"2001:4860::1"},
		},
		{
			name: "public inventory after missing scalar addresses",
			info: map[string]interface{}{
				"ip_addresses": []map[string]interface{}{
					{"address": "9.9.9.9", "family": "ipv4"},
					{"address": "2001:4860::2", "family": "ipv6"},
				},
			},
			wantCalls: []string{"9.9.9.9"},
		},
		{
			name: "private mapped documentation and loopback addresses are never looked up",
			info: map[string]interface{}{
				"ipv4": "10.0.0.1", "ipv6": "::ffff:8.8.8.8",
				"ip_addresses": []map[string]interface{}{
					{"address": "192.0.2.1", "family": "ipv4"},
					{"address": "2001:db8::1", "family": "ipv6"},
					{"address": "127.0.0.1", "family": "ipv4"},
				},
			},
			wantCalls: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &recordingGeoIPProvider{name: t.Name()}
			geoip.SetCurrentProvider(provider)
			appendClientRegionFromGeoIP(tt.info)
			if len(provider.calls) != len(tt.wantCalls) {
				t.Fatalf("GeoIP calls = %v, want %v", provider.calls, tt.wantCalls)
			}
			for i := range tt.wantCalls {
				if provider.calls[i] != tt.wantCalls[i] {
					t.Fatalf("GeoIP calls = %v, want %v", provider.calls, tt.wantCalls)
				}
			}
		})
	}
}

func TestAppendClientRegionFromGeoIPPreservesExistingRegionOnLookupFailure(t *testing.T) {
	flags.DatabaseType = "sqlite"
	flags.DatabaseFile = "file:basic_info_geoip_failure?mode=memory&cache=shared"
	_ = dbcore.GetDBInstance()
	if err := config.Set(config.GeoIpEnabledKey, true); err != nil {
		t.Fatalf("enable geoip: %v", err)
	}

	oldProvider := geoip.CurrentProvider()
	geoip.SetCurrentProvider(&recordingGeoIPProvider{name: t.Name(), err: errors.New("lookup failed")})
	t.Cleanup(func() { geoip.SetCurrentProvider(oldProvider) })

	info := map[string]interface{}{"ipv4": "8.8.8.8", "region": "manual-region"}
	appendClientRegionFromGeoIP(info)
	if got := info["region"]; got != "manual-region" {
		t.Fatalf("region changed after GeoIP failure: %v", got)
	}
}

func TestSaveClientBasicInfoPreservesExistingRegionOnLookupFailure(t *testing.T) {
	flags.DatabaseType = "sqlite"
	flags.DatabaseFile = "file:basic_info_geoip_saved_failure?mode=memory&cache=shared"
	db := dbcore.GetDBInstance()
	if err := config.Set(config.GeoIpEnabledKey, true); err != nil {
		t.Fatalf("enable geoip: %v", err)
	}

	oldProvider := geoip.CurrentProvider()
	geoip.SetCurrentProvider(&recordingGeoIPProvider{name: t.Name(), err: errors.New("lookup failed")})
	t.Cleanup(func() { geoip.SetCurrentProvider(oldProvider) })

	const clientUUID = "geoip-failure-preserves-region"
	if err := db.Create(&models.Client{UUID: clientUUID, Token: clientUUID, Region: "existing-region"}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := saveClientBasicInfo(map[string]interface{}{"ipv4": "8.8.8.8"}, clientUUID, ""); err != nil {
		t.Fatalf("save basic info: %v", err)
	}

	var got models.Client
	if err := db.First(&got, "uuid = ?", clientUUID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Region != "existing-region" {
		t.Fatalf("region changed after GeoIP failure: %q", got.Region)
	}
}

func TestSaveClientBasicInfoPreservesManualRegionOverride(t *testing.T) {
	flags.DatabaseType = "sqlite"
	flags.DatabaseFile = "file:basic_info_geoip_region_override?mode=memory&cache=shared"
	db := dbcore.GetDBInstance()
	if err := config.Set(config.GeoIpEnabledKey, true); err != nil {
		t.Fatalf("enable geoip: %v", err)
	}

	oldProvider := geoip.CurrentProvider()
	geoip.SetCurrentProvider(&recordingGeoIPProvider{name: t.Name()})
	t.Cleanup(func() { geoip.SetCurrentProvider(oldProvider) })

	const clientUUID = "geoip-manual-region"
	if err := db.Create(&models.Client{UUID: clientUUID, Token: clientUUID, Region: "🇺🇸", RegionOverride: "manual-region"}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := saveClientBasicInfo(map[string]interface{}{"ipv4": "8.8.8.8"}, clientUUID, ""); err != nil {
		t.Fatalf("save basic info: %v", err)
	}

	var got models.Client
	if err := db.First(&got, "uuid = ?", clientUUID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Region != geoip.GetRegionUnicodeEmoji("SG") || got.RegionOverride != "manual-region" {
		t.Fatalf("manual region override was changed by GeoIP update: %+v", got)
	}
}

func TestPeerIPFromRequestIgnoresForwardedHeaders(t *testing.T) {
	req := httptest.NewRequest("POST", "/uploadBasicInfo", nil)
	req.RemoteAddr = "203.0.113.10:443"
	req.Header.Set("X-Forwarded-For", "8.8.8.8")
	req.Header.Set("X-Real-IP", "9.9.9.9")
	if got := peerIPFromRequest(req); got != "203.0.113.10" {
		t.Fatalf("peer IP = %q, want direct remote address", got)
	}
}

func TestPublicGeoIPCandidateNormalizesMappedIPv4(t *testing.T) {
	if got := publicGeoIPCandidate("::ffff:8.8.8.8", "ipv4"); got == nil || got.String() != "8.8.8.8" {
		t.Fatalf("mapped IPv4 candidate = %v, want 8.8.8.8", got)
	}
	if got := publicGeoIPCandidate("::ffff:8.8.8.8", "ipv6"); got != nil {
		t.Fatalf("mapped IPv4 labelled ipv6 = %v, want rejection", got)
	}
}

func TestApplyFallbackClientIPReplacesInvalidPrimaryAndNormalizesMappedPeer(t *testing.T) {
	info := map[string]interface{}{"ipv4": "10.0.0.1"}
	applyFallbackClientIP(info, "8.8.8.8")
	if got := info["ipv4"]; got != "8.8.8.8" {
		t.Fatalf("invalid IPv4 hid public peer fallback: got %q", got)
	}

	mapped := map[string]interface{}{}
	applyFallbackClientIP(mapped, "::ffff:8.8.8.8")
	if got := mapped["ipv4"]; got != "8.8.8.8" {
		t.Fatalf("mapped peer fallback = %q, want canonical IPv4", got)
	}
}

func TestV2BasicInfoUsesOnlyPublicDirectPeerAsFallback(t *testing.T) {
	flags.DatabaseType = "sqlite"
	flags.DatabaseFile = "file:v2_basic_info_geoip_peer_fallback?mode=memory&cache=shared"
	db := dbcore.GetDBInstance()
	if err := config.Set(config.GeoIpEnabledKey, true); err != nil {
		t.Fatalf("enable geoip: %v", err)
	}

	oldProvider := geoip.CurrentProvider()
	provider := &recordingGeoIPProvider{name: t.Name()}
	geoip.SetCurrentProvider(provider)
	t.Cleanup(func() { geoip.SetCurrentProvider(oldProvider) })

	for _, clientUUID := range []string{"v2-peer-public", "v2-peer-proxy"} {
		if err := db.Create(&models.Client{UUID: clientUUID, Token: clientUUID}).Error; err != nil {
			t.Fatalf("create client %s: %v", clientUUID, err)
		}
	}

	request := v2.Request{JSONRPC: v2.Version, Method: v2.MethodAgentBasicInfo, Params: v2.BasicInfoParams{Info: map[string]interface{}{"version": "2.0"}}, ID: "basic-info-peer"}
	if resp := handleV2RPCWithPeer("v2-peer-public", request, false, "8.8.8.8"); resp.Error != nil {
		t.Fatalf("public peer fallback failed: %+v", resp.Error)
	}
	if resp := handleV2RPCWithPeer("v2-peer-proxy", request, false, "10.0.0.1"); resp.Error != nil {
		t.Fatalf("private proxy peer fallback failed: %+v", resp.Error)
	}

	var publicPeer, privatePeer models.Client
	if err := db.First(&publicPeer, "uuid = ?", "v2-peer-public").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&privatePeer, "uuid = ?", "v2-peer-proxy").Error; err != nil {
		t.Fatal(err)
	}
	if publicPeer.IPv4 != "8.8.8.8" || publicPeer.Region != geoip.GetRegionUnicodeEmoji("SG") {
		t.Fatalf("public peer fallback was not saved and geolocated: %+v", publicPeer)
	}
	if privatePeer.IPv4 != "" || privatePeer.IPv6 != "" || privatePeer.Region != "" {
		t.Fatalf("private proxy peer must not become an address or region: %+v", privatePeer)
	}
	if len(provider.calls) != 1 || provider.calls[0] != "8.8.8.8" {
		t.Fatalf("GeoIP calls = %v, want only public direct peer", provider.calls)
	}
}

func TestV2BasicInfoFillsRegionFromGeoIP(t *testing.T) {
	flags.DatabaseType = "sqlite"
	flags.DatabaseFile = "file:v2_basic_info_geoip?mode=memory&cache=shared"

	db := dbcore.GetDBInstance()
	if err := config.Set(config.GeoIpEnabledKey, true); err != nil {
		t.Fatalf("enable geoip: %v", err)
	}

	oldProvider := geoip.CurrentProvider()
	geoip.SetCurrentProvider(staticGeoIPProvider{name: t.Name(), iso: "SG"})
	t.Cleanup(func() {
		geoip.SetCurrentProvider(oldProvider)
	})

	clientUUID := "client-v2-geoip"
	now := time.Now().UTC()
	if err := db.Create(&models.Client{
		UUID:      clientUUID,
		Token:     "token-v2-geoip",
		Name:      "client_v2_geoip",
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}

	resp := handleV2RPC(clientUUID, v2.Request{
		JSONRPC: v2.Version,
		Method:  v2.MethodAgentBasicInfo,
		Params: map[string]interface{}{
			"info": map[string]interface{}{
				"ipv4":         "8.8.8.8",
				"month_rotate": 26,
			},
		},
		ID: "basic-info",
	}, false)
	if resp.Error != nil {
		t.Fatalf("v2 basic info failed: %+v", resp.Error)
	}

	var got models.Client
	if err := db.First(&got, "uuid = ?", clientUUID).Error; err != nil {
		t.Fatalf("load client: %v", err)
	}
	want := geoip.GetRegionUnicodeEmoji("SG")
	if got.Region != want {
		t.Fatalf("expected GeoIP region to be saved, got %q", got.Region)
	}
	if got.TrafficResetDay == nil || *got.TrafficResetDay != 26 {
		t.Fatalf("expected Agent reset day to be adopted, got %v", got.TrafficResetDay)
	}
	var result struct {
		Config *v2.ConfigParams `json:"config"`
	}
	if err := bindV2Params(resp.Result, &result); err != nil {
		t.Fatalf("bind response config: %v", err)
	}
	if result.Config == nil || result.Config.MonthRotate == nil || *result.Config.MonthRotate != 26 {
		t.Fatalf("expected response reset day 26, got %+v", result.Config)
	}
}

func TestV2BasicInfoStoresMultiplePublicAddresses(t *testing.T) {
	flags.DatabaseType = "sqlite"
	flags.DatabaseFile = "file:v2_basic_info_multiple_public_addresses?mode=memory&cache=shared"
	db := dbcore.GetDBInstance()
	const clientUUID = "client-v2-multiple-addresses"
	if err := db.Create(&models.Client{UUID: clientUUID, Token: "token-v2-multiple-addresses"}).Error; err != nil {
		t.Fatal(err)
	}
	response := handleV2RPC(clientUUID, v2.Request{
		JSONRPC: v2.Version,
		Method:  v2.MethodAgentBasicInfo,
		Params: v2.BasicInfoParams{Info: map[string]interface{}{
			"ipv4": "8.8.8.8", "ipv6": "2001:4860::1",
			"ip_addresses": []map[string]interface{}{
				{"address": "8.8.8.8", "family": "ipv4", "primary": true, "source": "reported"},
				{"address": "9.9.9.9", "family": "ipv4", "interface": "eth1", "source": "interface"},
				{"address": "2001:4860::1", "family": "ipv6", "primary": true, "source": "reported"},
				{"address": "2001:4860::2", "family": "ipv6", "interface": "eth2", "source": "interface"},
			},
		}},
		ID: "basic-info-multiple-addresses",
	}, false)
	if response.Error != nil {
		t.Fatalf("v2 basic info failed: %+v", response.Error)
	}
	var stored models.Client
	if err := db.First(&stored, "uuid = ?", clientUUID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.IPv4 != "8.8.8.8" || stored.IPv6 != "2001:4860::1" || len(stored.IPAddresses) != 4 || stored.IPAddresses[3].Address != "2001:4860::2" {
		t.Fatalf("basic info did not persist primary and extra identities: primary=%s/%s inventory=%+v", stored.IPv4, stored.IPv6, stored.IPAddresses)
	}
}

func TestV2BasicInfoSynchronizesCurrentAgentRuntimeConfig(t *testing.T) {
	flags.DatabaseType = "sqlite"
	flags.DatabaseFile = "file:v2_basic_info_runtime_config?mode=memory&cache=shared"
	db := dbcore.GetDBInstance()

	clientUUID := "client-v2-runtime-config"
	if err := db.Create(&models.Client{UUID: clientUUID, Token: "token-v2-runtime-config"}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	day, interval := 14, 9.0
	include, exclude, mounts := "eth0", "lo", "/;/srv"
	memoryCache, gpu := true, false
	resp := handleV2RPC(clientUUID, v2.Request{
		JSONRPC: v2.Version,
		Method:  v2.MethodAgentBasicInfo,
		Params: v2.BasicInfoParams{
			Info:     map[string]interface{}{"version": "2.2.0.0"},
			Platform: "linux",
			ConfigState: &v2.ConfigParams{
				MonthRotate:        &day,
				Interval:           &interval,
				IncludeNics:        &include,
				ExcludeNics:        &exclude,
				IncludeMountpoints: &mounts,
				MemoryIncludeCache: &memoryCache,
				EnableGPU:          &gpu,
			},
		},
		ID: "basic-info-runtime-config",
	}, false)
	if resp.Error != nil {
		t.Fatalf("v2 basic info failed: %+v", resp.Error)
	}
	profile, saved, err := clients.GetDeploymentProfile(clientUUID)
	if err != nil {
		t.Fatalf("load synchronized profile: %v", err)
	}
	if !saved || profile.MonthRotate != day || profile.Interval != interval ||
		profile.IncludeNics != include || profile.ExcludeNics != exclude ||
		profile.IncludeMountpoints != mounts || !profile.MemoryIncludeCache || profile.EnableGPU {
		t.Fatalf("unexpected synchronized profile: %+v", profile)
	}
}
