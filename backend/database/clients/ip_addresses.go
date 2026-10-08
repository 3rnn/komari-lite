package clients

import (
	"encoding/json"
	"fmt"
	"net/netip"

	"github.com/komari-monitor/komari/database/models"
)

const maxReportedIPAddresses = 1024
const maxReportedIPAddressesJSON = 512 * 1024

var nonPublicReportedIPv4 = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
}

// IsReportedPublicIP reports whether ip is a globally routable address that an
// Agent may include in its public address inventory.
func IsReportedPublicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return false
	}
	if ip.Is4() {
		for _, prefix := range nonPublicReportedIPv4 {
			if prefix.Contains(ip) {
				return false
			}
		}
		return true
	}
	return netip.MustParsePrefix("2000::/3").Contains(ip) && !netip.MustParsePrefix("2001:db8::/32").Contains(ip)
}

// normalizeClientIPAddresses accepts an optional Agent list and writes the
// canonical JSON driver value. Legacy reports that update scalar addresses
// clear a formerly reported list rather than leaving stale identities behind.
func normalizeClientIPAddresses(update map[string]interface{}) error {
	reported, exists := update["ip_addresses"]
	if !exists {
		_, v4 := update["ipv4"]
		_, v6 := update["ipv6"]
		if v4 || v6 {
			update["ip_addresses"] = models.IPAddresses{}
		}
		return nil
	}
	if slice, ok := reported.([]interface{}); ok && len(slice) > maxReportedIPAddresses {
		return fmt.Errorf("too many reported public addresses")
	}
	encoded, err := json.Marshal(reported)
	if err != nil || len(encoded) > maxReportedIPAddressesJSON {
		return fmt.Errorf("invalid or oversized public address list")
	}
	var addresses models.IPAddresses
	if err := json.Unmarshal(encoded, &addresses); err != nil || addresses == nil || len(addresses) > maxReportedIPAddresses {
		return fmt.Errorf("invalid public address list")
	}
	seen := make(map[netip.Addr]bool, len(addresses))
	for index := range addresses {
		entry := &addresses[index]
		ip, err := netip.ParseAddr(entry.Address)
		if err != nil || !IsReportedPublicIP(ip) || seen[ip.Unmap()] || len(entry.Interface) > 128 || len(entry.Source) > 32 {
			return fmt.Errorf("invalid public address entry")
		}
		ip = ip.Unmap()
		seen[ip] = true
		family, primaryKey := "ipv6", "ipv6"
		if ip.Is4() {
			family, primaryKey = "ipv4", "ipv4"
		}
		if entry.Family != family {
			return fmt.Errorf("public address family mismatch")
		}
		if entry.Primary {
			primary, ok := update[primaryKey].(string)
			if !ok {
				return fmt.Errorf("primary public address is missing its legacy field")
			}
			parsed, err := netip.ParseAddr(primary)
			if err != nil || parsed.Unmap() != ip {
				return fmt.Errorf("primary public address differs from legacy field")
			}
		}
		entry.Address = ip.String()
	}
	update["ip_addresses"] = addresses
	return nil
}
