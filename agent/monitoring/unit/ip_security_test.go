package monitoring

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type ipRoundTrip func(*http.Request) (*http.Response, error)

func (f ipRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPublicIPDiscoveryRejectsOversizedBodies(t *testing.T) {
	old := ipv4HTTPClient
	defer func() { ipv4HTTPClient = old }()
	ipv4HTTPClient = &http.Client{Transport: ipRoundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("1.2.3.4" + strings.Repeat(" ", 128*1024))), Header: make(http.Header)}, nil
	})}
	if got, _ := GetIPv4Address(); got != "" {
		t.Fatalf("accepted oversized public IP response: %s", got)
	}
}

func TestPublicIPDiscoveryUsesHTTPSAndOneTotalBudget(t *testing.T) {
	old4, old6 := ipv4HTTPClient, ipv6HTTPClient
	defer func() { ipv4HTTPClient = old4; ipv6HTTPClient = old6 }()
	var deadline time.Time
	client := &http.Client{Transport: ipRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme != "https" {
			t.Errorf("insecure provider: %s", r.URL)
		}
		got, ok := r.Context().Deadline()
		if !ok || time.Until(got) > 5*time.Second {
			t.Error("provider has no total deadline")
		}
		if deadline.IsZero() {
			deadline = got
		} else if !got.Equal(deadline) {
			t.Error("deadline resets for every provider")
		}
		return nil, errors.New("provider unavailable")
	})}
	ipv4HTTPClient, ipv6HTTPClient = client, client
	_, _ = GetIPv4Address()
	deadline = time.Time{}
	_, _ = GetIPv6Address()
}

func TestPublicIPDiscoveryRejectsErrorStatusAndInvalidAddress(t *testing.T) {
	old := ipv4HTTPClient
	defer func() { ipv4HTTPClient = old }()
	for _, test := range []struct {
		status int
		body   string
	}{{500, "1.2.3.4"}, {200, "999.999.999.999"}} {
		ipv4HTTPClient = &http.Client{Transport: ipRoundTrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body)), Header: make(http.Header)}, nil
		})}
		if got, _ := GetIPv4Address(); got != "" {
			t.Fatalf("accepted invalid provider result %q", got)
		}
	}
}
