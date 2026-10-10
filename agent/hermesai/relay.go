// Package hermesai relays only the restricted helper's aggregate spool.
// It never opens Hermes databases, OAuth stores or invokes a Hermes process.
package hermesai

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/nuomiiiii/lite-agent/requestheaders"
)

const DefaultSpool = "/var/lib/komari-hermes/spool"
const MaxPayload = 512 * 1024

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type Sample struct {
	SampleID               string   `json:"sample_id"`
	StartedAt              float64  `json:"started_at"`
	EndedAt                float64  `json:"ended_at"`
	Model                  string   `json:"model"`
	Provider               string   `json:"provider"`
	BillingMode            string   `json:"billing_mode"`
	Task                   string   `json:"task"`
	Calls                  uint64   `json:"calls"`
	InputTokens            uint64   `json:"input_tokens"`
	OutputTokens           uint64   `json:"output_tokens"`
	CacheReadTokens        uint64   `json:"cache_read_tokens"`
	CacheWriteTokens       uint64   `json:"cache_write_tokens"`
	ReasoningTokens        uint64   `json:"reasoning_tokens"`
	HermesEstimatedCostUSD *float64 `json:"hermes_estimated_cost_usd,omitempty"`
	RecordedActualCostUSD  *float64 `json:"recorded_actual_cost_usd,omitempty"`
	CostStatus             string   `json:"cost_status,omitempty"`
	CostSource             string   `json:"cost_source,omitempty"`
	Coverage               *bool    `json:"coverage,omitempty"`
	ResetCount             uint64   `json:"reset_count,omitempty"`
}
type QuotaWindow struct {
	Label         string   `json:"label"`
	UsedPercent   *float64 `json:"used_percent"`
	ResetsAt      string   `json:"resets_at,omitempty"`
	WindowSeconds *float64 `json:"window_seconds,omitempty"`
}
type Quota struct {
	AccountKey     string        `json:"account_key,omitempty"`
	Provider       string        `json:"provider"`
	Plan           string        `json:"plan,omitempty"`
	Status         string        `json:"status"`
	ObservedAt     float64       `json:"observed_at"`
	LastSuccessAt  *float64      `json:"last_success_at,omitempty"`
	Windows        []QuotaWindow `json:"windows"`
	ResetBankCount *float64      `json:"reset_bank_count,omitempty"`
}
type Delivery struct {
	LastAttemptAt       float64 `json:"last_attempt_at"`
	LastSuccessAt       float64 `json:"last_success_at,omitempty"`
	ConsecutiveFailures int     `json:"consecutive_failures"`
	LastErrorClass      string  `json:"last_error_class"`
}
type Report struct {
	Version                 int       `json:"version"`
	CollectorID             string    `json:"collector_id"`
	Enabled                 bool      `json:"enabled"`
	Status                  string    `json:"status"`
	ObservedAt              float64   `json:"observed_at"`
	LastSuccessAt           *float64  `json:"last_success_at,omitempty"`
	SamplingIntervalSeconds float64   `json:"sampling_interval_seconds"`
	CurrentModel            string    `json:"current_model,omitempty"`
	Samples                 []Sample  `json:"samples"`
	Quota                   *Quota    `json:"quota,omitempty"`
	Delivery                *Delivery `json:"delivery,omitempty"`
	PendingSampleCount      int       `json:"pending_sample_count,omitempty"`
	OldestPendingEndedAt    *float64  `json:"oldest_pending_ended_at,omitempty"`
}

func readDelivery(spool string) (Delivery, error) {
	var delivery Delivery
	root, err := os.OpenRoot(spool)
	if err != nil {
		return delivery, err
	}
	defer root.Close()
	info, err := root.Lstat("delivery.json")
	if os.IsNotExist(err) {
		return delivery, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return delivery, errors.New("invalid delivery state")
	}
	f, err := root.Open("delivery.json")
	if err != nil {
		return delivery, err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 4097))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&delivery); err != nil {
		return delivery, errors.New("invalid delivery state")
	}
	if delivery.ConsecutiveFailures < 0 || delivery.ConsecutiveFailures > 4096 || (delivery.LastErrorClass != "" && delivery.LastErrorClass != "none" && delivery.LastErrorClass != "network" && delivery.LastErrorClass != "timeout" && delivery.LastErrorClass != "http_4xx" && delivery.LastErrorClass != "http_5xx" && delivery.LastErrorClass != "invalid_ack" && delivery.LastErrorClass != "spool_io") {
		return Delivery{}, errors.New("invalid delivery state")
	}
	return delivery, nil
}
func persistDelivery(spool string, delivery Delivery) error {
	data, err := json.Marshal(delivery)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(spool)
	if err != nil {
		return err
	}
	defer root.Close()
	return writeWithinRoot(root, "delivery.json", data)
}
func deliveryClass(err error) string {
	if err == nil {
		return "none"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	text := err.Error()
	if strings.HasPrefix(text, "AI report HTTP 4") {
		return "http_4xx"
	}
	if strings.HasPrefix(text, "AI report HTTP 5") {
		return "http_5xx"
	}
	if strings.Contains(text, "ACK") {
		return "invalid_ack"
	}
	if strings.Contains(text, "persist") || strings.Contains(text, "helper") || strings.Contains(text, "spool") {
		return "spool_io"
	}
	return "network"
}

func readReport(spool string) (Report, error) {
	var report Report
	root, err := os.OpenRoot(spool)
	if err != nil {
		return report, errors.New("Hermes helper not provisioned")
	}
	defer root.Close()
	info, err := root.Lstat("report.json")
	if err != nil || !info.Mode().IsRegular() {
		return report, errors.New("invalid helper report")
	}
	f, err := root.Open("report.json")
	if err != nil {
		return report, errors.New("Hermes helper not provisioned")
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > MaxPayload {
		return report, errors.New("invalid helper report")
	}
	decoder := json.NewDecoder(io.LimitReader(f, MaxPayload+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return report, errors.New("invalid helper report schema")
	}
	var extra interface{}
	if decoder.Decode(&extra) != io.EOF {
		return report, errors.New("trailing helper report data")
	}
	if report.Version != 1 || !uuidPattern.MatchString(report.CollectorID) || len(report.Samples) > 256 || math.IsNaN(report.ObservedAt) || math.IsInf(report.ObservedAt, 0) || report.SamplingIntervalSeconds < 1 || report.SamplingIntervalSeconds > 60 {
		return report, errors.New("invalid helper report limits")
	}
	for _, s := range report.Samples {
		if !uuidPattern.MatchString(s.SampleID) {
			return report, errors.New("invalid sample ID")
		}
	}
	return report, nil
}

// SetEnabled changes only a boolean control, never source paths. True requires a
// fresh helper report. Disabled, unprovisioned installations remain untouched.
func SetEnabled(spool string, enabled bool) error {
	if runtime.GOOS != "linux" {
		if enabled {
			return errors.New("Hermes AI monitoring unsupported on this platform")
		}
		return nil
	}
	if enabled {
		report, err := readReport(spool)
		if err != nil {
			return err
		}
		if report.Status != "disabled" && report.Status != "no_usage" && report.Status != "collecting" {
			return errors.New("Hermes helper source unavailable")
		}
		age := float64(time.Now().UnixNano())/1e9 - report.ObservedAt
		if age < -5 || age > 90 {
			return errors.New("Hermes helper unavailable or stale")
		}
	} else if _, err := os.Stat(spool); os.IsNotExist(err) {
		return nil
	}
	payload, _ := json.Marshal(struct {
		Enabled bool `json:"enabled"`
	}{enabled})
	root, err := os.OpenRoot(spool)
	if err != nil {
		return errors.New("helper spool unavailable")
	}
	defer root.Close()
	return writeWithinRoot(root, "control.json", payload)
}

func SetProtocolVersion(spool string, version int) error {
	if version != 2 { return errors.New("unsupported Hermes report protocol") }
	payload, _ := json.Marshal(struct { Version int `json:"version"` }{version})
	root, err := os.OpenRoot(spool)
	if err != nil { return errors.New("helper spool unavailable") }
	defer root.Close()
	return writeWithinRoot(root, "protocol.json", payload)
}

// writeWithinRoot is atomic and durable without ever following a path outside
// the spool. A helper owner cannot redirect privileged Agent writes via symlinks.
func writeWithinRoot(root *os.Root, name string, data []byte) error {
	temp := ".pending-" + rand.Text()
	f, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer root.Remove(temp)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = root.Rename(temp, name); err != nil {
		return err
	}
	d, err := root.Open(".")
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func persistACK(spool, id string) error {
	if !uuidPattern.MatchString(id) {
		return errors.New("invalid ACK ID")
	}
	root, err := os.OpenRoot(spool)
	if err != nil {
		return err
	}
	defer root.Close()
	ack, err := root.OpenRoot("acks")
	if err != nil {
		return err
	}
	defer ack.Close()
	return writeWithinRoot(ack, id, []byte{})
}

type Relay struct {
	Spool                string
	Endpoint             string
	Token                string
	CFAccessClientID     string
	CFAccessClientSecret string
	Client               *http.Client
}

// Once deletes nothing: it persists individual ACK markers for offered IDs only.
// The helper atomically removes accepted samples from its durable private outbox.
func (r Relay) Once(ctx context.Context, enabled bool) (err error) {
	if !enabled {
		return nil
	}
	report, err := readReport(r.Spool)
	if err != nil {
		return err
	}
	previous, _ := readDelivery(r.Spool)
	defer func() {
		now := float64(time.Now().UnixNano()) / 1e9
		outcome := Delivery{LastAttemptAt: now, LastSuccessAt: previous.LastSuccessAt, ConsecutiveFailures: previous.ConsecutiveFailures, LastErrorClass: deliveryClass(err)}
		if err == nil {
			outcome.LastSuccessAt = now
			outcome.ConsecutiveFailures = 0
		} else {
			outcome.ConsecutiveFailures++
		}
		if persistErr := persistDelivery(r.Spool, outcome); persistErr != nil && err == nil {
			err = errors.New("persist AI delivery state failed")
		}
	}()
	age := float64(time.Now().UnixNano())/1e9 - report.ObservedAt
	if age > 90 || age < -5 {
		report.Status = "stale"
		report.Quota = nil
	}
	payload, err := json.Marshal(report)
	if err != nil {
		return errors.New("invalid helper metrics")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.Endpoint+"/api/clients/ai/report", bytes.NewReader(payload))
	if err != nil {
		return errors.New("invalid AI endpoint")
	}
	request.Header.Set("Content-Type", "application/json")
	requestheaders.ApplyAgentAuthentication(request.Header, r.Token, r.CFAccessClientID, r.CFAccessClientSecret)
	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	// Never forward the node credential across a redirect, including same-origin.
	local := *client
	local.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := local.Do(request)
	if err != nil {
		return errors.New("AI report unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("AI report HTTP %d", response.StatusCode)
	}
	var ack struct {
		AcceptedSampleIDs []string `json:"accepted_sample_ids"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&ack); err != nil || len(ack.AcceptedSampleIDs) > 256 {
		return errors.New("invalid AI report ACK")
	}
	offered := make(map[string]bool, len(report.Samples))
	for _, sample := range report.Samples {
		offered[sample.SampleID] = true
	}
	for _, id := range ack.AcceptedSampleIDs {
		if !offered[id] || !uuidPattern.MatchString(id) {
			continue
		}
		if err := persistACK(r.Spool, id); err != nil {
			return errors.New("persist AI ACK failed")
		}
	}
	return nil
}

// Run is independent of heartbeat/reconnect work and intentionally never exits
// because of collector/network failures. Operators see delivery state via backend.
func (r Relay) Run(ctx context.Context, enabled func() bool) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		_ = r.Once(ctx, enabled())
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
