package monitoring

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
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
		re := regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
		ipv4 := re.FindString(string(body))
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

		// Extract the IPv6 address from the response body using a regular expression.
		re := regexp.MustCompile(`(([0-9A-Fa-f]{1,4}:){7})([0-9A-Fa-f]{1,4})|(([0-9A-Fa-f]{1,4}:){1,6}:)(([0-9A-Fa-f]{1,4}:){0,4})([0-9A-Fa-f]{0,4})`)
		ipv6 := re.FindString(string(body))
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
	if flags.GetIpAddrFromNic {
		pair := nicIPCache.get(nicIPCacheTTL, lookupNICIP)
		if pair.v4 != "" || pair.v6 != "" {
			return pair.v4, pair.v6, nil
		}
	}

	if flags.CustomIpv4 != "" {
		ipv4 = flags.CustomIpv4
	}
	if flags.CustomIpv6 != "" {
		ipv6 = flags.CustomIpv6
	}
	if ipv4 != "" && ipv6 != "" {
		return ipv4, ipv6, nil
	}

	cached := lookupPublicIPCached()
	if ipv4 == "" {
		ipv4 = cached.v4
	}
	if ipv6 == "" {
		ipv6 = cached.v6
	}
	return ipv4, ipv6, nil
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
	if flags.CustomIpv4 == "" {
		pair.v4, err = GetIPv4Address()
		if err != nil {
			log.Printf("Get IPV4 Error: %v", err)
			pair.v4 = ""
		}
	}
	if flags.CustomIpv6 == "" {
		pair.v6, err = GetIPv6Address()
		if err != nil {
			log.Printf("Get IPV6 Error: %v", err)
			pair.v6 = ""
		}
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
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			// Get the IPv4 address.
			if ipv4 == "" && ip.To4() != nil {
				ipv4 = ip.String()
			}

			// Get the IPv6 address, excluding link-local addresses.
			if ipv6 == "" && ip.To4() == nil && !ip.IsLinkLocalUnicast() {
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
