package monitoring

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/nuomiiiii/lite-agent/dnsresolver"
)

var (
	// Create HTTP clients for IPv4 and IPv6.
	ipv4HTTPClient = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				dialer := dnsresolver.GetNetDialer(15 * time.Second)
				return dialer.DialContext(ctx, "tcp4", addr) // Force IPv4 to avoid address-family issues.
			},
			DisableKeepAlives:     true,
			MaxIdleConns:          1,
			IdleConnTimeout:       5 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: 15 * time.Second,
	}
	ipv6HTTPClient = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				dialer := dnsresolver.GetNetDialer(15 * time.Second)
				return dialer.DialContext(ctx, "tcp6", addr) // Force IPv6 to avoid address-family issues.
			},
			DisableKeepAlives:     true,
			MaxIdleConns:          1,
			IdleConnTimeout:       5 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: 15 * time.Second,
	}
	userAgent = "curl/8.0.1"
)

func GetIPv4Address() (string, error) {

	webAPIs := []string{
		"https://www.visa.cn/cdn-cgi/trace",
		"https://www.qualcomm.cn/cdn-cgi/trace",
		"https://www.toutiao.com/stream/widget/local_weather/data/",
		"https://edge-ip.html.zone/geo",
		"https://vercel-ip.html.zone/geo",
		"http://ipv4.ip.sb",
		"https://api.ipify.org?format=json",
	}

	for _, api := range webAPIs {
		// get ipv4
		req, err := http.NewRequest("GET", api, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", userAgent)
		resp, err := ipv4HTTPClient.Do(req)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close() // Close the response immediately after reading to avoid blocking.
		if err != nil {
			continue
		}
		ipv4 := firstPublicAddress(string(body), true)
		if ipv4 != "" {
			log.Printf("Get IPV4 Success: %s", ipv4)
			return ipv4, nil
		}
	}
	return "", nil
}

func GetIPv6Address() (string, error) {

	webAPIs := []string{
		"https://v6.ip.zxinc.org/info.php?type=json",
		"https://api6.ipify.org?format=json",
		"https://ipv6.icanhazip.com",
		"http://api-ipv6.ip.sb/geoip",
	}

	for _, api := range webAPIs {
		// get ipv6
		req, err := http.NewRequest("GET", api, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", userAgent)
		resp, err := ipv6HTTPClient.Do(req)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close() // Close the response immediately after reading to avoid blocking.
		if err != nil {
			continue
		}

		ipv6 := firstPublicAddress(string(body), false)
		if ipv6 != "" {
			log.Printf("Get IPV6 Success:  %s", ipv6)
			return ipv6, nil
		}
	}
	return "", nil
}

type publicIPPair struct {
	v4 string
	v6 string
}

var (
	publicIPCache ttlCache[publicIPPair]
	nicIPCache    ttlCache[publicIPPair]
)

func GetIPAddress() (ipv4, ipv6 string, err error) {
	var nic publicIPPair
	if flags.GetIpAddrFromNic {
		nic = nicIPCache.get(nicIPCacheTTL, lookupNICIP)
		pair := selectPublicIPPair(true, flags.CustomIpv4, flags.CustomIpv6, nic, publicIPPair{})
		if pair.v4 != "" && pair.v6 != "" {
			return pair.v4, pair.v6, nil
		}
	} else {
		pair := selectPublicIPPair(false, flags.CustomIpv4, flags.CustomIpv6, publicIPPair{}, publicIPPair{})
		if pair.v4 != "" && pair.v6 != "" {
			return pair.v4, pair.v6, nil
		}
	}

	pair := selectPublicIPPair(flags.GetIpAddrFromNic, flags.CustomIpv4, flags.CustomIpv6, nic, lookupPublicIPCached())
	return pair.v4, pair.v6, nil
}

// parsePublicPrimaryAddress accepts exactly one canonical, globally routable
// address in the requested family. This keeps legacy primary fields safe while
// preserving their string wire format.
func parsePublicPrimaryAddress(value string, ipv4 bool) string {
	ip, ok := publicAddress(strings.TrimSpace(value))
	if !ok || ip.Is4() != ipv4 {
		return ""
	}
	return ip.String()
}

// firstPublicAddress extracts token-shaped candidates from provider responses,
// then validates every candidate with netip before returning it.
func firstPublicAddress(body string, ipv4 bool) string {
	for _, candidate := range strings.FieldsFunc(body, func(r rune) bool {
		return !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') || r == '.' || r == ':')
	}) {
		if ip := parsePublicPrimaryAddress(strings.Trim(candidate, ".:"), ipv4); ip != "" {
			return ip
		}
	}
	return ""
}

// selectPublicIPPair preserves NIC-mode priority for public NIC identities,
// custom priority otherwise, and fills only missing families from external
// discovery. Invalid, private, and non-routable candidates are never primary.
func selectPublicIPPair(nicMode bool, custom4, custom6 string, nic, discovered publicIPPair) publicIPPair {
	pair := publicIPPair{}
	if nicMode {
		pair.v4 = parsePublicPrimaryAddress(nic.v4, true)
		pair.v6 = parsePublicPrimaryAddress(nic.v6, false)
	} else {
		pair.v4 = parsePublicPrimaryAddress(custom4, true)
		pair.v6 = parsePublicPrimaryAddress(custom6, false)
	}
	if pair.v4 == "" {
		pair.v4 = parsePublicPrimaryAddress(discovered.v4, true)
	}
	if pair.v6 == "" {
		pair.v6 = parsePublicPrimaryAddress(discovered.v6, false)
	}
	return pair
}

func lookupNICIP() publicIPPair {
	allowNics, err := InterfaceList()
	if err != nil {
		log.Printf("Get Interface List Error: %v", err)
		return publicIPPair{}
	}
	v4, v6 := getIPFromInterfaces(allowNics)
	if v4 != "" || v6 != "" {
		log.Printf("Get IP from NIC - IPv4: %s, IPv6: %s", v4, v6)
	}
	return publicIPPair{v4: v4, v6: v6}
}

func lookupPublicIPCached() publicIPPair {
	publicIPCache.mu.Lock()
	if publicIPCache.ok && time.Since(publicIPCache.at) < publicIPCacheTTL && (publicIPCache.val.v4 != "" || publicIPCache.val.v6 != "") {
		pair := publicIPCache.val
		publicIPCache.mu.Unlock()
		return pair
	}
	publicIPCache.mu.Unlock()

	pair := lookupPublicIP()
	if pair.v4 == "" && pair.v6 == "" {
		return pair
	}
	publicIPCache.mu.Lock()
	publicIPCache.val = pair
	publicIPCache.at = time.Now()
	publicIPCache.ok = true
	publicIPCache.mu.Unlock()
	return pair
}

func lookupPublicIP() publicIPPair {
	var pair publicIPPair
	var err error
	pair.v4, err = GetIPv4Address()
	if err != nil {
		log.Printf("Get IPV4 Error: %v", err)
		pair.v4 = ""
	}
	pair.v6, err = GetIPv6Address()
	if err != nil {
		log.Printf("Get IPV6 Error: %v", err)
		pair.v6 = ""
	}
	return pair
}

// getIPFromInterfaces retrieves IPv4 and IPv6 addresses from selected interfaces.
func getIPFromInterfaces(nicNames []string) (ipv4, ipv6 string) {
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("Failed to get network interfaces: %v", err)
		return "", ""
	}
	for _, iface := range interfaces {
		// Check whether the interface is allowed.
		if !func(slice []string, item string) bool {
			for _, s := range slice {
				if s == item {
					return true
				}
			}
			return false
		}(nicNames, iface.Name) {
			continue
		}

		// Skip interfaces that are down.
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip netip.Addr
			var ok bool
			switch v := addr.(type) {
			case *net.IPNet:
				ip, ok = netip.AddrFromSlice(v.IP)
			case *net.IPAddr:
				ip, ok = netip.AddrFromSlice(v.IP)
			}

			if !ok {
				continue
			}
			ip, ok = publicNetipAddress(ip)
			if !ok {
				continue
			}

			// Get the IPv4 address.
			if ipv4 == "" && ip.Is4() {
				ipv4 = ip.String()
			}

			// Get the globally routable IPv6 address.
			if ipv6 == "" && !ip.Is4() {
				ipv6 = ip.String()
			}

			// Return early once both IPv4 and IPv6 addresses are found.
			if ipv4 != "" && ipv6 != "" {
				return ipv4, ipv6
			}
		}
	}

	return ipv4, ipv6
}
