package monitoring

import (
	"net"
	"net/netip"
	"sort"
)

// PublicIPAddress is an advertised network identity. Primary is the existing
// reported address for its family; Interface and Source describe the discovery
// method without assuming an address is usable for every destination.
type PublicIPAddress struct {
	Address   string `json:"address"`
	Family    string `json:"family"`
	Interface string `json:"interface,omitempty"`
	Source    string `json:"source,omitempty"`
	Primary   bool   `json:"primary,omitempty"`
}

type interfaceIPAddress struct {
	Address   string
	Interface string
}

const maxPublicIPAddresses = 1024

var nonPublicIPv4 = []netip.Prefix{
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

var globalIPv6 = netip.MustParsePrefix("2000::/3")
var documentationIPv6 = netip.MustParsePrefix("2001:db8::/32")

func publicAddress(value string) (netip.Addr, bool) {
	ip, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, false
	}
	return publicNetipAddress(ip)
}

func publicNetipAddress(ip netip.Addr) (netip.Addr, bool) {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return netip.Addr{}, false
	}
	if ip.Is4() {
		for _, excluded := range nonPublicIPv4 {
			if excluded.Contains(ip) {
				return netip.Addr{}, false
			}
		}
	} else if !globalIPv6.Contains(ip) || documentationIPv6.Contains(ip) {
		return netip.Addr{}, false
	}
	return ip, true
}

func buildPublicIPAddresses(primary4, primary6 string, interfaces []interfaceIPAddress) []PublicIPAddress {
	result := make([]PublicIPAddress, 0, len(interfaces)+2)
	seen := make(map[string]bool, len(interfaces)+2)
	primaryIndex := make(map[string]int, 2)
	for index, primary := range []string{primary4, primary6} {
		ip, ok := publicAddress(primary)
		if !ok || ip.Is4() != (index == 0) || seen[ip.String()] {
			continue
		}
		family := "ipv6"
		if ip.Is4() {
			family = "ipv4"
		}
		seen[ip.String()] = true
		primaryIndex[ip.String()] = len(result)
		result = append(result, PublicIPAddress{Address: ip.String(), Family: family, Source: "reported", Primary: true})
	}
	additional := make([]PublicIPAddress, 0, len(interfaces))
	for _, candidate := range interfaces {
		ip, ok := publicAddress(candidate.Address)
		if !ok {
			continue
		}
		key := ip.String()
		if index, exists := primaryIndex[key]; exists {
			// An on-interface address gives the reported primary a concrete identity.
			if result[index].Interface == "" {
				result[index].Interface = candidate.Interface
				result[index].Source = "interface"
			}
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		family := "ipv6"
		if ip.Is4() {
			family = "ipv4"
		}
		additional = append(additional, PublicIPAddress{Address: key, Family: family, Interface: candidate.Interface, Source: "interface"})
	}
	sort.Slice(additional, func(i, j int) bool {
		if additional[i].Family != additional[j].Family {
			return additional[i].Family < additional[j].Family
		}
		left, _ := netip.ParseAddr(additional[i].Address)
		right, _ := netip.ParseAddr(additional[j].Address)
		return left.Compare(right) < 0
	})
	result = append(result, additional...)
	if len(result) > maxPublicIPAddresses {
		result = result[:maxPublicIPAddresses]
	}
	return result
}

func scanAssignedAddresses(flags net.Flags) bool {
	// Routed public /32 and /128 identities are sometimes assigned to lo.
	// Reject only down interfaces; the address filter discards 127/8 and ::1.
	return flags&net.FlagUp != 0
}

// GetPublicIPAddresses enumerates assigned public addresses in addition to
// the existing primary/default egress identities. NAT mappings not assigned to
// an interface cannot be inferred beyond the primary reported egress IP.
func GetPublicIPAddresses(primary4, primary6 string) []PublicIPAddress {
	interfaces, err := net.Interfaces()
	if err != nil {
		return buildPublicIPAddresses(primary4, primary6, nil)
	}
	var candidates []interfaceIPAddress
	for _, iface := range interfaces {
		if !scanAssignedAddresses(iface.Flags) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			switch a := addr.(type) {
			case *net.IPNet:
				candidates = append(candidates, interfaceIPAddress{Address: a.IP.String(), Interface: iface.Name})
			case *net.IPAddr:
				candidates = append(candidates, interfaceIPAddress{Address: a.IP.String(), Interface: iface.Name})
			}
		}
	}
	return buildPublicIPAddresses(primary4, primary6, candidates)
}
