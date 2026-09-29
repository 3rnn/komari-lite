package dnsresolver

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	pkg_flags "github.com/nuomiiiii/lite-agent/cmd/flags"
)

var flags = pkg_flags.GlobalConfig
var (
	DNSServers = []string{
		"[2606:4700:4700::1111]:53", // Cloudflare IPv6
		"[2606:4700:4700::1001]:53", // Cloudflare IPv6 fallback.
		"[2001:4860:4860::8888]:53", // Google IPv6
		"[2001:4860:4860::8844]:53", // Google IPv6 fallback.

		"114.114.114.114:53", // 114DNS, mainland China.
		"1.1.1.1:53",         // Cloudflare IPv4
		"8.8.8.8:53",         // Google IPv4
		"8.8.4.4:53",         // Google IPv4 fallback.
		"223.5.5.5:53",       // Alibaba DNS, mainland China.
		"119.29.29.29:53",    // DNSPod, mainland China.
	}

	// CustomDNSServer is a custom DNS server set via CLI flags.
	CustomDNSServer string

	preferV4Once sync.Once
	hasIPv4      bool
	httpClientMu sync.Mutex
	httpClients  = make(map[httpClientKey]*http.Client)
)

type httpClientKey struct {
	timeout          time.Duration
	ignoreUnsafeCert bool
	preferIPVersion  string
}

// SetCustomDNSServer sets the custom DNS server.
func SetCustomDNSServer(dnsServer string) {
	if dnsServer == "" {
		return
	}
	CustomDNSServer = normalizeDNSServer(dnsServer)
}

// normalizeDNSServer normalizes a DNS server to host:port:
// Bracket IPv6 addresses and add port :53 if missing.
// Add port :53 to IPv4 addresses or domain names if missing.
func normalizeDNSServer(s string) string {
	s = strings.TrimSpace(s)
	// Already [ipv6]:port or host:port.
	if (strings.HasPrefix(s, "[") && strings.Contains(s, "]:")) || (strings.Count(s, ":") == 1 && !strings.Contains(s, "]")) {
		return s
	}
	// Bare IPv6 (without brackets or port).
	if strings.Count(s, ":") >= 2 && !strings.Contains(s, "]") {
		return "[" + s + "]:53"
	}
	// Otherwise add port 53 if absent.
	if !strings.Contains(s, ":") {
		return s + ":53"
	}
	return s
}

// getCurrentDNSServer returns the currently configured DNS server.
func getCurrentDNSServer() string {
	if CustomDNSServer != "" {
		return CustomDNSServer
	}
	// Without a custom DNS server, return empty to use the system resolver.
	return ""
}

// GetCustomResolver returns a resolver:
// With custom DNS, use it and fall back to the built-in server list on failure.
// Without custom DNS, use the system resolver (not the built-in list).
func GetCustomResolver() *net.Resolver {
	// No custom DNS: use the system resolver directly.
	if getCurrentDNSServer() == "" {
		return net.DefaultResolver
	}

	// Custom DNS: build a resolver using that server.
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 10 * time.Second}

			// Try the custom DNS server first.
			dnsServer := getCurrentDNSServer()
			if dnsServer != "" {
				if conn, err := d.DialContext(ctx, "udp", dnsServer); err == nil {
					return conn, nil
				}
			}
			log.Printf("Custom DNS server %s is unreachable, trying fallback servers", dnsServer)
			// If unavailable, try the built-in list as a fallback.
			for _, server := range DNSServers {
				if server == dnsServer {
					continue
				}
				if conn, err := d.DialContext(ctx, "udp", server); err == nil {
					return conn, nil
				}
			}

			return nil, fmt.Errorf("no available DNS server")
		},
	}
}

// buildTransport builds an HTTP transport with custom DNS/dialing and optional TLS configuration.
func buildTransport(timeout time.Duration, tlsConfig *tls.Config) *http.Transport {
	return buildTransportWithPreference(timeout, tlsConfig, "")
}

func buildTransportWithPreference(timeout time.Duration, tlsConfig *tls.Config, preferIPVersion string) *http.Transport {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	customResolver := GetCustomResolver()
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := customResolver.LookupHost(ctx, host)
			if err != nil {
				return nil, err
			}
			sortIPsByPreference(ips, preferIPVersion)
			for _, ip := range ips {
				dialer := &net.Dialer{
					Timeout:   timeout,
					KeepAlive: 30 * time.Second,
					DualStack: true,
				}
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
				if err == nil {
					return conn, nil
				}
			}
			return nil, fmt.Errorf("failed to dial to any of the resolved IPs")
		},
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   4,
		MaxConnsPerHost:       8,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       tlsConfig,
		ForceAttemptHTTP2:     true,
	}
}

func GetHTTPClient(timeout time.Duration) *http.Client {
	return getHTTPClient(timeout, "")
}

// GetHTTPClientWithPreference returns an HTTP client with custom DNS and IP-version ordering.
// With preferIPVersion "4" or "6", prefer that family; otherwise use automatic selection.
func GetHTTPClientWithPreference(timeout time.Duration, preferIPVersion string) *http.Client {
	return getHTTPClient(timeout, normalizeIPVersionPreference(preferIPVersion))
}

func getHTTPClient(timeout time.Duration, preferIPVersion string) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	key := httpClientKey{timeout: timeout, ignoreUnsafeCert: flags.IgnoreUnsafeCert, preferIPVersion: preferIPVersion}
	httpClientMu.Lock()
	defer httpClientMu.Unlock()
	if client := httpClients[key]; client != nil {
		return client
	}
	client := &http.Client{
		Transport: buildTransportWithPreference(timeout, &tls.Config{
			InsecureSkipVerify: flags.IgnoreUnsafeCert,
		}, preferIPVersion),
		Timeout: timeout,
	}
	httpClients[key] = client
	return client
}

// GetNetDialer returns a network dialer using the custom DNS resolver.
func GetNetDialer(timeout time.Duration) *net.Dialer {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
		Resolver:  GetCustomResolver(),
	}
}

// GetDialContext returns a custom DialContext:
// Resolve hostnames with the custom resolver.
// Choose IPv4 or IPv6 priority based on the local network.
// Try each IP until a connection succeeds or all fail.
func GetDialContext(timeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return GetDialContextWithPreference(timeout, "")
}

// GetDialContextWithPreference returns a DialContext with explicit IPv4/IPv6 preference.
// With preferIPVersion "4" or "6", prefer that family; otherwise use automatic selection.
func GetDialContextWithPreference(timeout time.Duration, preferIPVersion string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	resolver := GetCustomResolver()
	preferIPVersion = normalizeIPVersionPreference(preferIPVersion)

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}

		// Use a timed child context for DNS to avoid exhausting the overall dial timeout.
		lookupCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		ips, err := resolver.LookupHost(lookupCtx, host)
		if err != nil {
			return nil, err
		}

		sortIPsByPreference(ips, preferIPVersion)

		// Try each IP in turn.
		for _, ip := range ips {
			d := &net.Dialer{
				Timeout:   timeout,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}
			c, err := d.DialContext(ctx, network, net.JoinHostPort(ip, port))
			if err == nil {
				return c, nil
			}
		}
		return nil, fmt.Errorf("failed to dial to any of the resolved IPs")
	}
}

func normalizeIPVersionPreference(preferIPVersion string) string {
	if preferIPVersion == "4" || preferIPVersion == "6" {
		return preferIPVersion
	}
	return ""
}

func sortIPsByPreference(ips []string, preferIPVersion string) {
	preferIPVersion = normalizeIPVersionPreference(preferIPVersion)
	if preferIPVersion == "" {
		// Order addresses dynamically based on local IPv4 availability.
		if preferIPv4First() {
			preferIPVersion = "4"
		} else {
			preferIPVersion = "6"
		}
	}

	preferred := make([]string, 0, len(ips))
	others := make([]string, 0, len(ips))
	for _, ip := range ips {
		parsedIP := net.ParseIP(ip)
		isIPv4 := parsedIP != nil && parsedIP.To4() != nil
		isIPv6 := parsedIP != nil && parsedIP.To4() == nil
		if (preferIPVersion == "4" && isIPv4) || (preferIPVersion == "6" && isIPv6) {
			preferred = append(preferred, ip)
		} else {
			others = append(others, ip)
		}
	}
	n := copy(ips, preferred)
	copy(ips[n:], others)
}

// preferIPv4First checks for usable local IPv4; otherwise dial IPv6 first.
func preferIPv4First() bool {
	preferV4Once.Do(func() {
		ifaces, _ := net.Interfaces()
		for _, iface := range ifaces {
			if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
				continue
			}
			addrs, _ := iface.Addrs()
			for _, a := range addrs {
				var ip net.IP
				switch v := a.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip == nil || ip.IsLoopback() {
					continue
				}
				if ip.To4() != nil {
					hasIPv4 = true
					return
				}
			}
		}
		hasIPv4 = false
	})
	return hasIPv4
}
