package utils

import (
	"net"
	"net/url"
	"strings"

	"golang.org/x/net/idna"
)

// ConvertIDNToASCII converts URLs with internationalized domain names (IDNs) to ASCII-compatible encoding (ACE).
// For example: "https://\u4e2d\u6587\u57df\u540d.com" -> "https://xn--fiq228c.com".
func ConvertIDNToASCII(urlStr string) (string, error) {
	// Parse the URL.
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return urlStr, err
	}

	hostname := parsedURL.Hostname()

	// IP addresses (IPv4 or IPv6) need no conversion.
	if net.ParseIP(hostname) != nil {
		return parsedURL.String(), nil
	}

	// Convert the hostname to Punycode.
	asciiHost, err := idna.ToASCII(hostname)
	if err != nil {
		return urlStr, err
	}

	// Preserve the port, if present.
	if parsedURL.Port() != "" {
		parsedURL.Host = asciiHost + ":" + parsedURL.Port()
	} else {
		parsedURL.Host = asciiHost
	}

	return parsedURL.String(), nil
}

// ConvertHostToASCII converts a hostname (possibly including a port) to ACE.
// For example: "\u4e2d\u6587\u57df\u540d.com:8080" -> "xn--fiq228c.com:8080".
func ConvertHostToASCII(host string) (string, error) {
	// Separate hostname and port.
	var hostname, port string

	// Handle bracketed IPv6 addresses such as [::1]:port.
	if strings.HasPrefix(host, "[") {
		if idx := strings.LastIndex(host, "]"); idx != -1 {
			hostname = host[1:idx] // Remove brackets.
			if len(host) > idx+1 {
				port = host[idx+1:] // Include the colon.
			}
		} else {
			hostname = host
		}
	} else if idx := strings.LastIndex(host, ":"); idx != -1 {
		// Check for IPv6 addresses (multiple colons).
		if strings.Count(host, ":") > 1 {
			// This may be an unbracketed IPv6 address.
			hostname = host
		} else {
			// IPv4:port format.
			hostname = host[:idx]
			port = host[idx:]
		}
	} else {
		hostname = host
	}

	// IP addresses need no conversion.
	if net.ParseIP(hostname) != nil {
		return host, nil
	}

	// Convert to ASCII.
	asciiHost, err := idna.ToASCII(hostname)
	if err != nil {
		return host, err
	}

	// Reassemble, retaining brackets from the original input.
	if strings.HasPrefix(host, "[") && port != "" {
		return "[" + asciiHost + "]" + port, nil
	}
	return asciiHost + port, nil
}
