package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// IPAddress describes one public network identity reported by an Agent.
// Primary IPv4/IPv6 remain in the legacy scalar client columns for consumers
// that do not understand this optional, extensible collection.
type IPAddress struct {
	Address   string `json:"address"`
	Family    string `json:"family"`
	Interface string `json:"interface,omitempty"`
	Source    string `json:"source,omitempty"`
	Primary   bool   `json:"primary,omitempty"`
}

type IPAddresses []IPAddress

func (addresses *IPAddresses) Scan(value any) error {
	var raw []byte
	switch v := value.(type) {
	case nil:
		*addresses = IPAddresses{}
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("scan IP addresses: unsupported value type %T", value)
	}
	if len(raw) == 0 || string(raw) == "null" {
		*addresses = IPAddresses{}
		return nil
	}
	return json.Unmarshal(raw, addresses)
}

func (addresses IPAddresses) Value() (driver.Value, error) {
	if addresses == nil {
		addresses = IPAddresses{}
	}
	encoded, err := json.Marshal(addresses)
	return string(encoded), err
}
