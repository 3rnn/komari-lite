package aimonitor

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// Pricing administration accepts either Unix seconds or RFC3339. Reads use
// RFC3339 to match existing administration forms; storage uses Unix seconds.
func (r *PricingRule) UnmarshalJSON(raw []byte) error {
	type plain PricingRule
	var props map[string]json.RawMessage
	if err := json.Unmarshal(raw, &props); err != nil {
		return err
	}
	value, ok := props["effective_at"]
	if !ok {
		return fmt.Errorf("effective_at is required")
	}
	var seconds float64
	if len(value) > 0 && value[0] == '"' {
		var iso string
		if err := json.Unmarshal(value, &iso); err != nil {
			return err
		}
		at, err := time.Parse(time.RFC3339Nano, iso)
		if err != nil {
			return fmt.Errorf("effective_at must be RFC3339 or Unix seconds")
		}
		seconds = float64(at.Unix()) + float64(at.Nanosecond())/1e9
	} else {
		if string(value) == "null" {
			return fmt.Errorf("effective_at cannot be null")
		}
		if err := json.Unmarshal(value, &seconds); err != nil {
			return err
		}
	}
	numeric, err := json.Marshal(seconds)
	if err != nil {
		return err
	}
	props["effective_at"] = numeric
	normalized, err := json.Marshal(props)
	if err != nil {
		return err
	}
	return json.Unmarshal(normalized, (*plain)(r))
}
func (r PricingRule) MarshalJSON() ([]byte, error) {
	type plain PricingRule
	if !finite(r.EffectiveAt) {
		return nil, fmt.Errorf("non-finite effective time")
	}
	seconds, fraction := math.Modf(r.EffectiveAt)
	return json.Marshal(struct {
		plain
		EffectiveAt string `json:"effective_at"`
	}{plain: plain(r), EffectiveAt: time.Unix(int64(seconds), int64(fraction*1e9)).UTC().Format(time.RFC3339Nano)})
}
