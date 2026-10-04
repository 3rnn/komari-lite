package client

import (
	"net"
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

func TestV2BasicInfoFillsRegionFromGeoIP(t *testing.T) {
	flags.DatabaseType = "sqlite"
	flags.DatabaseFile = "file:v2_basic_info_geoip?mode=memory&cache=shared"

	db := dbcore.GetDBInstance()
	if err := config.Set(config.GeoIpEnabledKey, true); err != nil {
		t.Fatalf("enable geoip: %v", err)
	}

	oldProvider := geoip.CurrentProvider
	geoip.CurrentProvider = staticGeoIPProvider{name: t.Name(), iso: "SG"}
	t.Cleanup(func() {
		geoip.CurrentProvider = oldProvider
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
