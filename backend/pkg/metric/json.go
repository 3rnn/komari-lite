package metric

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// encodeMap encodes a string map as JSON.
//
// encodeMap encodes the string map into a JSON string, and nil will be treated as an empty object.
func encodeMap(m map[string]string) (string, error) {
	if m == nil {
		m = map[string]string{}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// decodeMap decodes a scanned JSON value into a string map.
//
// decodeMap restores the JSON values scanned from the database into a string map.
func decodeMap(v any) (map[string]string, error) {
	switch x := v.(type) {
	case nil:
		return map[string]string{}, nil
	case string:
		return decodeMapString(x)
	case []byte:
		return decodeMapString(string(x))
	default:
		return nil, fmt.Errorf("unsupported json value type %T", v)
	}
}

// decodeMapString decodes a JSON object string into a string map.
//
// decodeMapString Decodes a JSON string into a string map.
func decodeMapString(s string) (map[string]string, error) {
	if s == "" {
		return map[string]string{}, nil
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]string{}
	}
	return out, nil
}

// canonicalTags returns a deterministic JSON encoding of a tag map. encodeMap
// already relies on encoding/json, which sorts map keys, so equal tag maps
// always produce identical bytes — the property tagsFingerprint depends on.
//
// canonicalTags returns the deterministic JSON encoding of the tag map; the same tag set will always get
// Same byte sequence, tagsFingerprint relies on this property.
func canonicalTags(m map[string]string) (string, error) {
	return encodeMap(m)
}

// tagsFingerprint returns a stable hex fingerprint of a tag set, used as the
// rollups table's tags_hash key column. Equal tag maps (regardless of Go map
// iteration order) hash identically, so each distinct tag combination becomes
// its own rollup series; an empty/nil tag map hashes to the fingerprint of "{}".
//
// tagsFingerprint generates a stable hexadecimal fingerprint for a collection of tags, used as a base for the rollups table
// tags_hash key column. The same label map (regardless of the Go map iteration order) will get the same hash,
// So each different tag combination will be its own rollup sequence; an empty or nil tag map will get "{}"
// of fingerprints.
func tagsFingerprint(m map[string]string) (hash string, canonical string, err error) {
	canonical, err = canonicalTags(m)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:]), canonical, nil
}
