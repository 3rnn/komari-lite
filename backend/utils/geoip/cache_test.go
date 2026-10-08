package geoip

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
)

type cacheTestProvider struct {
	name  string
	calls atomic.Int32
	err   error
}

func (p *cacheTestProvider) Name() string { return p.name }
func (p *cacheTestProvider) GetGeoInfo(net.IP) (*GeoInfo, error) {
	p.calls.Add(1)
	if p.err != nil {
		return nil, p.err
	}
	return &GeoInfo{ISOCode: "GB", Name: "United Kingdom"}, nil
}
func (p *cacheTestProvider) UpdateDatabase() error { return nil }
func (p *cacheTestProvider) Close() error          { return nil }

func TestGetGeoInfoCoalescesConcurrentLookups(t *testing.T) {
	provider := &cacheTestProvider{name: t.Name()}
	old := CurrentProvider()
	SetCurrentProvider(provider)
	t.Cleanup(func() { SetCurrentProvider(old) })

	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			info, err := GetGeoInfo(net.ParseIP("8.8.8.8"))
			if err != nil || info == nil || info.ISOCode != "GB" {
				t.Errorf("lookup = %#v, %v", info, err)
			}
		}()
	}
	wg.Wait()
	if got := provider.calls.Load(); got != 1 {
		t.Fatalf("provider calls = %d, want 1", got)
	}
}

func TestGetGeoInfoTemporarilyCachesFailures(t *testing.T) {
	provider := &cacheTestProvider{name: t.Name(), err: errors.New("temporary provider failure")}
	old := CurrentProvider()
	SetCurrentProvider(provider)
	t.Cleanup(func() { SetCurrentProvider(old) })

	for range 2 {
		if _, err := GetGeoInfo(net.ParseIP("9.9.9.9")); err == nil {
			t.Fatal("expected provider error")
		}
	}
	if got := provider.calls.Load(); got != 1 {
		t.Fatalf("provider calls = %d, want one cached failure", got)
	}
}
