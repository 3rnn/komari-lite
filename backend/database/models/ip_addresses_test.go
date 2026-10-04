package models

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestIPAddressesJSONDatabaseRoundTrip(t *testing.T) {
	original := IPAddresses{
		{Address: "8.8.8.8", Family: "ipv4", Interface: "eth0", Source: "interface", Primary: true},
		{Address: "2001:4860::1", Family: "ipv6", Interface: "eth1", Source: "interface"},
	}
	stored, err := original.Value()
	if err != nil {
		t.Fatal(err)
	}
	var restored IPAddresses
	if err := restored.Scan(stored); err != nil || !reflect.DeepEqual(restored, original) {
		t.Fatalf("roundtrip: restored=%+v err=%v", restored, err)
	}
	jsonBytes, err := json.Marshal(restored)
	if err != nil || !json.Valid(jsonBytes) || string(jsonBytes) == "null" {
		t.Fatalf("API JSON = %s, err=%v", jsonBytes, err)
	}
	var empty IPAddresses
	if err := empty.Scan(nil); err != nil || len(empty) != 0 {
		t.Fatalf("legacy null column = %+v, err=%v", empty, err)
	}
	value, err := empty.Value()
	if err != nil || value.(string) != "[]" {
		t.Fatalf("empty persisted value = %q, err=%v", value, err)
	}
}
