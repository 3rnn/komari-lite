package monitoring

import "testing"

func TestParsePublicPrimaryAddressCanonicalizesAndRejectsNonPublicValues(t *testing.T) {
	cases := []struct {
		name  string
		value string
		ipv4  bool
		want  string
	}{
		{name: "public IPv4", value: "8.8.8.8", ipv4: true, want: "8.8.8.8"},
		{name: "public IPv6", value: "2001:4860:4860::8888", want: "2001:4860:4860::8888"},
		{name: "IPv4 family mismatch", value: "8.8.8.8", want: ""},
		{name: "private IPv4", value: "10.0.0.1", ipv4: true, want: ""},
		{name: "documentation IPv4", value: "203.0.113.9", ipv4: true, want: ""},
		{name: "documentation IPv6", value: "2001:db8::9", want: ""},
		{name: "invalid IPv4-shaped value", value: "999.1.1.1", ipv4: true, want: ""},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := parsePublicPrimaryAddress(test.value, test.ipv4); got != test.want {
				t.Fatalf("parsePublicPrimaryAddress(%q, %t) = %q, want %q", test.value, test.ipv4, got, test.want)
			}
		})
	}
}

func TestFirstPublicAddressUsesNetipValidatedCandidates(t *testing.T) {
	if got := firstPublicAddress("ip=999.1.1.1\nprivate=10.0.0.1\npublic=8.8.8.8", true); got != "8.8.8.8" {
		t.Fatalf("firstPublicAddress IPv4 = %q, want 8.8.8.8", got)
	}
	if got := firstPublicAddress(`{"ip":"2001:db8::9","fallback":"2001:4860:4860::8888"}`, false); got != "2001:4860:4860::8888" {
		t.Fatalf("firstPublicAddress IPv6 = %q, want 2001:4860:4860::8888", got)
	}
}

func TestSelectPublicIPPairFallsBackWhenNICHasNoPublicAddress(t *testing.T) {
	got := selectPublicIPPair(
		true,
		"10.0.0.1",
		"fd00::1",
		publicIPPair{v4: "192.168.1.2", v6: "fe80::1"},
		publicIPPair{v4: "8.8.8.8", v6: "2001:4860:4860::8888"},
	)
	want := publicIPPair{v4: "8.8.8.8", v6: "2001:4860:4860::8888"}
	if got != want {
		t.Fatalf("selectPublicIPPair() = %+v, want %+v", got, want)
	}
}

func TestGetIPAddressUsesExternalCacheWhenNICOnlyHasPrivateAddresses(t *testing.T) {
	original4, original6, originalNIC := flags.CustomIpv4, flags.CustomIpv6, flags.GetIpAddrFromNic
	t.Cleanup(func() {
		flags.CustomIpv4, flags.CustomIpv6, flags.GetIpAddrFromNic = original4, original6, originalNIC
		publicIPCache.reset()
		nicIPCache.reset()
	})

	flags.CustomIpv4, flags.CustomIpv6, flags.GetIpAddrFromNic = "10.0.0.1", "fd00::1", true
	nicIPCache.get(0, func() publicIPPair { return publicIPPair{v4: "192.168.1.2", v6: "fe80::1"} })
	publicIPCache.get(0, func() publicIPPair { return publicIPPair{v4: "8.8.8.8", v6: "2001:4860:4860::8888"} })

	ipv4, ipv6, err := GetIPAddress()
	if err != nil {
		t.Fatal(err)
	}
	if ipv4 != "8.8.8.8" || ipv6 != "2001:4860:4860::8888" {
		t.Fatalf("GetIPAddress() = %q, %q; want only externally discovered public addresses", ipv4, ipv6)
	}
}

func TestGetIPAddressKeepsValidCustomPrimaries(t *testing.T) {
	original4, original6, originalNIC := flags.CustomIpv4, flags.CustomIpv6, flags.GetIpAddrFromNic
	t.Cleanup(func() {
		flags.CustomIpv4, flags.CustomIpv6, flags.GetIpAddrFromNic = original4, original6, originalNIC
		publicIPCache.reset()
		nicIPCache.reset()
	})

	flags.CustomIpv4, flags.CustomIpv6, flags.GetIpAddrFromNic = "8.8.4.4", "2001:4860:4860::8844", false
	publicIPCache.reset()
	nicIPCache.reset()

	ipv4, ipv6, err := GetIPAddress()
	if err != nil {
		t.Fatal(err)
	}
	if ipv4 != "8.8.4.4" || ipv6 != "2001:4860:4860::8844" {
		t.Fatalf("GetIPAddress() = %q, %q; want valid custom primaries", ipv4, ipv6)
	}
}

func TestSelectPublicIPPairPrefersValidNICAndValidCustomValues(t *testing.T) {
	got := selectPublicIPPair(
		true,
		"8.8.4.4",
		"2001:4860:4860::8844",
		publicIPPair{v4: "1.1.1.1", v6: "fd00::1"},
		publicIPPair{v4: "9.9.9.9", v6: "2001:4860:4860::8888"},
	)
	want := publicIPPair{v4: "1.1.1.1", v6: "2001:4860:4860::8888"}
	if got != want {
		t.Fatalf("NIC values must win only for their public family: got %+v, want %+v", got, want)
	}

	got = selectPublicIPPair(
		false,
		"8.8.4.4",
		"2001:4860:4860::8844",
		publicIPPair{},
		publicIPPair{v4: "9.9.9.9", v6: "2001:4860:4860::8888"},
	)
	want = publicIPPair{v4: "8.8.4.4", v6: "2001:4860:4860::8844"}
	if got != want {
		t.Fatalf("valid custom values must be preserved: got %+v, want %+v", got, want)
	}
}

func TestCachedHostIdentityStable(t *testing.T) {
	osNameCache.reset()
	kernelCache.reset()
	if first, second := CachedOSName(), CachedOSName(); first == "" || first != second {
		t.Fatalf("CachedOSName %q vs %q", first, second)
	}
	if first, second := CachedKernelVersion(), CachedKernelVersion(); first != second {
		t.Fatalf("CachedKernelVersion %q vs %q", first, second)
	}
}
