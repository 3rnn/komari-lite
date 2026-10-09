package aimonitor

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"math"
	"net/url"
	"reflect"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const MaxPayloadBytes = 512 * 1024

// DecodeStrict rejects unknown fields, duplicate keys at every level, trailing
// documents and oversized payloads. Credentials and session data are not schema.
func DecodeStrict(raw []byte, dst any) error {
	if len(raw) > MaxPayloadBytes {
		return fmt.Errorf("AI payload is too large")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := checkJSON(d, 0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	if err := checkFieldNames(raw, reflect.TypeOf(dst)); err != nil {
		return err
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return fmt.Errorf("invalid AI JSON: %w", err)
	}
	if _, ok := dst.(*Report); ok {
		var props map[string]json.RawMessage
		_ = json.Unmarshal(raw, &props)
		for _, key := range []string{"version", "collector_id", "enabled", "status", "observed_at", "samples"} {
			v, exists := props[key]
			if !exists || bytes.Equal(v, []byte("null")) {
				return fmt.Errorf("missing %s", key)
			}
		}
		var samples []map[string]json.RawMessage
		if err := json.Unmarshal(props["samples"], &samples); err != nil {
			return err
		}
		for _, sample := range samples {
			for _, key := range []string{"sample_id", "started_at", "ended_at", "model", "provider", "billing_mode", "task", "calls", "input_tokens", "output_tokens", "cache_read_tokens", "cache_write_tokens", "reasoning_tokens"} {
				value, ok := sample[key]
				if !ok || bytes.Equal(value, []byte("null")) {
					return fmt.Errorf("missing sample %s", key)
				}
			}
		}
	}
	return nil
}
func checkFieldNames(raw json.RawMessage, t reflect.Type) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	raw = bytes.TrimSpace(raw)
	if t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		if len(raw) == 0 || raw[0] != '[' {
			return nil
		}
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return err
		}
		for _, item := range items {
			if err := checkFieldNames(item, t.Elem()); err != nil {
				return err
			}
		}
		return nil
	}
	if t.Kind() != reflect.Struct || len(raw) == 0 || raw[0] != '{' {
		return nil
	}
	fields := map[string]reflect.Type{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name != "" && name != "-" {
			fields[name] = field.Type
		}
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return err
	}
	for key, value := range values {
		field, ok := fields[key]
		if !ok {
			return fmt.Errorf("undeclared JSON field %s", key)
		}
		if err := checkFieldNames(value, field); err != nil {
			return err
		}
	}
	return nil
}
func checkJSON(d *json.Decoder, depth int) error {
	if depth > 16 {
		return fmt.Errorf("JSON nesting limit exceeded")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			t, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := t.(string)
			if !ok || seen[key] {
				return fmt.Errorf("duplicate or invalid JSON key")
			}
			seen[key] = true
			if err := checkJSON(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := checkJSON(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("invalid JSON delimiter")
	}
	_, err = d.Token()
	return err
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func text(v string, max int, required bool) bool {
	if (required && v == "") || len(v) > max || !utf8.ValidString(v) {
		return false
	}
	for _, r := range v {
		if unicode.IsControl(r) {
			return false
		}
	}
	return strings.TrimSpace(v) == v
}
func timestamp(v float64, now time.Time) bool {
	return finite(v) && v >= float64(now.Unix()-30*86400) && v <= float64(now.Unix()+120)
}
func optionalTime(v *float64, now time.Time) bool {
	return v == nil || (finite(*v) && *v >= 0 && *v <= float64(now.Unix()+120))
}
func counter(v int64) bool          { return v >= 0 && v <= 1_000_000_000_000 }
func optionalMoney(v *float64) bool { return v == nil || (finite(*v) && *v >= 0 && *v <= 1e9) }
func (r Report) Validate(now time.Time) error {
	if r.Version != 1 || uuid.Validate(r.CollectorID) != nil || !timestamp(r.ObservedAt, now) || !optionalTime(r.LastSuccessAt, now) || len(r.Samples) > 256 || !text(r.CurrentModel, 128, false) {
		return fmt.Errorf("invalid AI report metadata")
	}
	switch r.Status {
	case "collecting", "disabled", "hermes_not_detected", "database_unavailable", "no_usage", "stale":
	default:
		return fmt.Errorf("invalid AI collection status")
	}
	if !r.Enabled && r.Status != "disabled" {
		return fmt.Errorf("disabled collector must report disabled status")
	}
	if !finite(r.SamplingIntervalSeconds) || r.SamplingIntervalSeconds < 1 || r.SamplingIntervalSeconds > 3600 {
		return fmt.Errorf("sampling interval must be 1..3600 seconds")
	}
	for _, s := range r.Samples {
		if uuid.Validate(s.SampleID) != nil || !timestamp(s.StartedAt, now) || !timestamp(s.EndedAt, now) || s.EndedAt <= s.StartedAt || s.EndedAt-s.StartedAt > 3600 {
			return fmt.Errorf("invalid AI sample identity or interval")
		}
		if !text(s.Model, 128, true) || !text(s.Provider, 64, true) || !text(s.BillingMode, 64, false) || !text(s.Task, 128, false) || !text(s.CostStatus, 64, false) || !text(s.CostSource, 128, false) {
			return fmt.Errorf("invalid AI sample dimensions")
		}
		for _, v := range []int64{s.Calls, s.InputTokens, s.OutputTokens, s.CacheReadTokens, s.CacheWriteTokens, s.ReasoningTokens, s.ResetCount} {
			if !counter(v) {
				return fmt.Errorf("invalid AI counter")
			}
		}
		if s.ReasoningTokens > s.OutputTokens || !optionalMoney(s.HermesEstimatedCostUSD) || !optionalMoney(s.RecordedActualCostUSD) {
			return fmt.Errorf("invalid AI reasoning or cost")
		}
	}
	if r.Quota != nil {
		return r.Quota.Validate(now)
	}
	return nil
}
func (q Quota) Validate(now time.Time) error {
	if !text(q.AccountKey, 128, false) || !text(q.Provider, 64, true) || !text(q.Plan, 128, false) || !timestamp(q.ObservedAt, now) || !optionalTime(q.LastSuccessAt, now) || len(q.Windows) > 8 || !optionalMoney(q.ResetBankCount) {
		return fmt.Errorf("invalid quota metadata")
	}
	if q.AccountKey != "" {
		decoded, err := hex.DecodeString(q.AccountKey)
		if err != nil || len(decoded) != 32 || strings.ToLower(q.AccountKey) != q.AccountKey {
			return fmt.Errorf("quota account key must be a lowercase SHA-256 identifier")
		}
	}
	switch q.Status {
	case "available", "unavailable", "stale":
	default:
		return fmt.Errorf("invalid quota status")
	}
	if q.AccountKey == "" && q.Status == "available" {
		return fmt.Errorf("unidentifiable account cannot have available quota")
	}
	for _, w := range q.Windows {
		if !text(w.Label, 64, true) || (w.UsedPercent != nil && (!finite(*w.UsedPercent) || *w.UsedPercent < 0 || *w.UsedPercent > 100)) {
			return fmt.Errorf("invalid quota window")
		}
		if w.WindowSeconds != nil && (!finite(*w.WindowSeconds) || *w.WindowSeconds <= 0 || *w.WindowSeconds > 366*86400) {
			return fmt.Errorf("invalid quota duration")
		}
		if w.ResetsAt != nil {
			if _, err := time.Parse(time.RFC3339, *w.ResetsAt); err != nil {
				return fmt.Errorf("invalid quota reset timestamp")
			}
		}
	}
	return nil
}
func (r PricingRule) Validate(now time.Time) error {
	if !text(r.Provider, 64, true) || !text(r.Model, 128, true) || !text(r.BillingMode, 64, false) || !text(r.Source, 128, true) || !text(r.Version, 128, true) || !text(r.ProcessingMode, 128, true) || !text(r.ReferenceTier, 128, false) || !text(r.SourceURL, 2048, false) || !timestamp(r.EffectiveAt, now) {
		return fmt.Errorf("explicit pricing source, version and processing mode are required")
	}
	for _, v := range []*float64{r.InputPerMillion, r.OutputPerMillion, r.CacheReadPerMillion, r.CacheWritePerMillion} {
		if !optionalMoney(v) {
			return fmt.Errorf("invalid token price")
		}
	}
	if r.InputPerMillion == nil || r.OutputPerMillion == nil {
		return fmt.Errorf("input and output prices are required")
	}
	if r.SourceURL != "" {
		u, err := url.Parse(r.SourceURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return fmt.Errorf("pricing source URL must be HTTPS without credentials")
		}
	}
	return nil
}
