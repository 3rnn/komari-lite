package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
)

// An empty local policy preserves LAN/private monitoring. An explicitly set
// policy allows only these CIDRs, independent of server-dispatched targets.
func pingAllowedIPs(ctx context.Context, host string) ([]net.IPAddr, error) {
	policy := strings.TrimSpace(os.Getenv("KOMARI_AGENT_PING_ALLOW_CIDRS"))
	var prefixes []netip.Prefix
	if policy != "" {
		for _, raw := range strings.Split(policy, ",") {
			prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
			if err != nil {
				return nil, errors.New("invalid ping egress allowlist")
			}
			prefixes = append(prefixes, prefix)
		}
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	allowed := make([]net.IPAddr, 0, len(ips))
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip.IP)
		if !ok {
			continue
		}
		addr = addr.Unmap()
		permit := policy == ""
		for _, p := range prefixes {
			if p.Contains(addr) {
				permit = true
				break
			}
		}
		if permit {
			allowed = append(allowed, ip)
		}
	}
	if len(allowed) == 0 {
		return nil, errors.New("ping destination denied")
	}
	return allowed, nil
}

func dialPing(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := pingAllowedIPs(ctx, host)
	if err != nil {
		return nil, err
	}
	var last error
	for _, ip := range ips {
		conn, err := (&net.Dialer{Timeout: pingTimeout}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		last = err
	}
	return nil, last
}

// Legacy launches retain their environment proxy. A local CIDR policy requires
// direct probes so the proxy cannot resolve or reach a denied destination.
func pingProxy(request *http.Request) (*url.URL, error) {
	if strings.TrimSpace(os.Getenv("KOMARI_AGENT_PING_ALLOW_CIDRS")) != "" {
		return nil, nil
	}
	return http.ProxyFromEnvironment(request)
}
