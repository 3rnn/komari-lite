package client

import (
	"encoding/json"
	"net"
	"net/http"
	"net/netip"

	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/utils/geoip"

	"github.com/gin-gonic/gin"
)

func saveClientBasicInfo(info map[string]interface{}, uuid string, fallbackIP string) error {
	monthRotate, reportsMonthRotate := info["month_rotate"]
	delete(info, "month_rotate")
	info["uuid"] = uuid
	applyFallbackClientIP(info, fallbackIP)
	appendClientRegionFromGeoIP(info)
	if err := clients.SaveClientInfo(info); err != nil {
		return err
	}
	if reportsMonthRotate {
		return clients.AdoptTrafficResetDay(uuid, monthRotate)
	}
	return nil
}

func applyFallbackClientIP(info map[string]interface{}, fallbackIP string) {
	if hasClientIP(info) {
		return
	}
	ip, err := netip.ParseAddr(fallbackIP)
	if err != nil {
		return
	}
	ip = ip.Unmap()
	if !clients.IsReportedPublicIP(ip) {
		return
	}
	if ip.Is4() {
		info["ipv4"] = ip.String()
		return
	}
	info["ipv6"] = ip.String()
}

func hasClientIP(info map[string]interface{}) bool {
	return normalizeReportedPrimary(info, "ipv4", true) || normalizeReportedPrimary(info, "ipv6", false)
}

// normalizeReportedPrimary retains only a canonical globally-routable Agent
// primary. Invalid scalar values must not block the direct-peer last-resort
// fallback, and IPv4-mapped IPv6 is stored as the IPv4 identity.
func normalizeReportedPrimary(info map[string]interface{}, key string, ipv4 bool) bool {
	raw, ok := info[key].(string)
	if !ok {
		return false
	}
	ip, err := netip.ParseAddr(raw)
	if err != nil {
		delete(info, key)
		return false
	}
	ip = ip.Unmap()
	if !clients.IsReportedPublicIP(ip) || ip.Is4() != ipv4 {
		delete(info, key)
		return false
	}
	info[key] = ip.String()
	return true
}

func appendClientRegionFromGeoIP(info map[string]interface{}) {
	cfg, err := config.GetAs[bool](config.GeoIpEnabledKey)
	if err != nil || !cfg {
		return
	}

	ip := geoIPCandidate(info)
	if ip == nil {
		return
	}
	record, err := geoip.GetGeoInfo(ip)
	if err != nil || record == nil {
		return
	}
	region := geoip.GetRegionUnicodeEmoji(record.ISOCode)
	if region != "" {
		info["region"] = region
	}
}

// geoIPCandidate selects one trusted, globally routable address from an Agent
// report. Legacy scalar addresses are the Agent's reported primary identities,
// followed by its validated public-address inventory.
func geoIPCandidate(info map[string]interface{}) net.IP {
	for _, candidate := range []struct {
		key    string
		family string
	}{{"ipv4", "ipv4"}, {"ipv6", "ipv6"}} {
		if raw, ok := info[candidate.key].(string); ok {
			if ip := publicGeoIPCandidate(raw, candidate.family); ip != nil {
				return ip
			}
		}
	}

	reported, ok := info["ip_addresses"]
	if !ok {
		return nil
	}
	encoded, err := json.Marshal(reported)
	if err != nil {
		return nil
	}
	var addresses models.IPAddresses
	if err := json.Unmarshal(encoded, &addresses); err != nil {
		return nil
	}
	for _, address := range addresses {
		if ip := publicGeoIPCandidate(address.Address, address.Family); ip != nil {
			return ip
		}
	}
	return nil
}

func publicGeoIPCandidate(raw, family string) net.IP {
	address, err := netip.ParseAddr(raw)
	if err != nil {
		return nil
	}
	address = address.Unmap()
	if !clients.IsReportedPublicIP(address) {
		return nil
	}
	if (family == "ipv4" && !address.Is4()) || (family == "ipv6" && !address.Is6()) || (family != "ipv4" && family != "ipv6") {
		return nil
	}
	return net.IP(address.AsSlice())
}

// peerIPFromRequest returns the directly connected peer only. Forwarded
// headers are intentionally ignored because they are not Agent-reported IPs.
func peerIPFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return ""
	}
	return host
}

func UploadBasicInfo(c *gin.Context) {
	var cbi = map[string]interface{}{}
	if err := c.ShouldBindJSON(&cbi); err != nil {
		c.JSON(400, gin.H{"status": "error", "error": "Invalid or missing data"})
		return
	}

	uuid, ok := clientUUIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "error": "Invalid token"})
		return
	}

	if err := ingestBasicInfo(uuid, cbi, peerIPFromRequest(c.Request)); err != nil {
		c.JSON(500, gin.H{"status": "error", "error": err})
		return
	}

	response := gin.H{"status": "success"}
	runtimeConfig, err := getClientRuntimeConfig(uuid)
	if err != nil {
		c.JSON(500, gin.H{"status": "error", "error": err.Error()})
		return
	}
	if runtimeConfig != nil {
		response["config"] = runtimeConfig
	} else {
		response["request_config_state"] = true
	}
	c.JSON(200, response)
}
