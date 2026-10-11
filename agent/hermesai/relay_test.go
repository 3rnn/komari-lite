package hermesai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) (string, []byte) {
	t.Helper()
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "acks"), 0700)
	payload := []byte(`{"version":1,"collector_id":"11111111-1111-4111-8111-111111111111","enabled":true,"status":"collecting","observed_at":` + jsonTime() + `,"sampling_interval_seconds":10,"samples":[{"sample_id":"22222222-2222-4222-8222-222222222222","started_at":100,"ended_at":110,"model":"fixture","provider":"openai","billing_mode":"api","task":"main","calls":1,"input_tokens":50,"output_tokens":10,"cache_read_tokens":2,"cache_write_tokens":0,"reasoning_tokens":1}]}`)
	os.WriteFile(filepath.Join(dir, "report.json"), payload, 0600)
	return dir, payload
}
func jsonTime() string { b, _ := json.Marshal(float64(time.Now().Unix())); return string(b) }

func TestRelayReportsAtSixtySecondCadenceByDefault(t *testing.T) {
	if DefaultReportInterval != 60*time.Second {
		t.Fatalf("default report interval = %s, want 60s", DefaultReportInterval)
	}
}

func TestRelayRetriesIdenticalSamplesAndExplicitDurableACK(t *testing.T) {
	dir, _ := fixture(t)
	attempts := 0
	var first string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/clients/ai/report" || r.Header.Get("Authorization") != "Bearer node" {
			t.Errorf("unsafe request")
		}
		var report Report
		if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
			t.Fatal(err)
		}
		id := report.Samples[0].SampleID
		if attempts == 0 {
			first = id
			attempts++
			w.WriteHeader(503)
			return
		}
		if id != first {
			t.Error("retry changed immutable sample ID")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"accepted_sample_ids":["` + id + `","33333333-3333-4333-8333-333333333333"]}`))
	}))
	defer srv.Close()
	relay := Relay{Spool: dir, Endpoint: srv.URL, Token: "node", Client: srv.Client()}
	if err := relay.Once(context.Background(), true); err == nil {
		t.Fatal("expected failure")
	}
	if _, err := os.Stat(filepath.Join(dir, "acks", first)); !os.IsNotExist(err) {
		t.Fatal("failed request ACKed")
	}
	if err := relay.Once(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "acks", first)); err != nil {
		t.Fatal("ACK not persisted", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "acks", "33333333-3333-4333-8333-333333333333")); !os.IsNotExist(err) {
		t.Fatal("unoffered ID ACKed")
	}
}
func TestCoverageBooleanMatchesBackendContract(t *testing.T) {
	dir, b := fixture(t)
	b = []byte(strings.Replace(string(b), `"calls":1`, `"coverage":true,"calls":1`, 1))
	os.WriteFile(filepath.Join(dir, "report.json"), b, 0600)
	if _, err := readReport(dir); err != nil {
		t.Fatal("covered intervals rejected", err)
	}
}

func TestRelayPersistsSanitizedDeliveryFailureAndRecovery(t *testing.T) {
	dir, _ := fixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "token=secret endpoint=private", http.StatusServiceUnavailable) }))
	defer srv.Close()
	relay := Relay{Spool: dir, Endpoint: srv.URL, Token: "node", Client: srv.Client()}
	if err := relay.Once(context.Background(), true); err == nil { t.Fatal("expected delivery failure") }
	b, err := os.ReadFile(filepath.Join(dir, "delivery.json")); if err != nil { t.Fatal(err) }
	if strings.Contains(string(b), "secret") || strings.Contains(string(b), "private") { t.Fatal("delivery diagnostic leaked error body") }
	if !strings.Contains(string(b), `"last_error_class":"http_5xx"`) || !strings.Contains(string(b), `"consecutive_failures":1`) { t.Fatalf("unexpected diagnostic: %s", b) }
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"accepted_sample_ids":[]}`)) })
	if err := relay.Once(context.Background(), true); err != nil { t.Fatal(err) }
	b, err = os.ReadFile(filepath.Join(dir, "delivery.json")); if err != nil { t.Fatal(err) }
	if !strings.Contains(string(b), `"last_error_class":"none"`) || !strings.Contains(string(b), `"consecutive_failures":0`) { t.Fatalf("recovery diagnostic: %s", b) }
}

func TestDisabledDoesNotReadSpoolOrRequest(t *testing.T) {
	relay := Relay{Spool: filepath.Join(t.TempDir(), "does-not-exist")}
	if err := relay.Once(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(relay.Spool); !os.IsNotExist(err) {
		t.Fatal("disabled created spool")
	}
}
func TestSpoolSymlinksCannotEscapeOperatorLocalRoot(t *testing.T) {
	dir, _ := fixture(t)
	outside := t.TempDir()
	target := filepath.Join(outside, "control.json")
	os.WriteFile(target, []byte("unchanged"), 0600)
	os.Symlink(outside, filepath.Join(dir, "escape"))
	os.RemoveAll(filepath.Join(dir, "acks"))
	os.Symlink(outside, filepath.Join(dir, "acks"))
	if err := persistACK(dir, "22222222-2222-4222-8222-222222222222"); err == nil {
		t.Fatal("ACK escaped spool root")
	}
	b, _ := os.ReadFile(target)
	if string(b) != "unchanged" {
		t.Fatal("outside file changed")
	}
}

func TestDisabledRevokesPreinstalledControlWithoutReadingReport(t *testing.T) {
 dir := t.TempDir()
 outside := filepath.Join(t.TempDir(), "private")
 if err := os.WriteFile(outside, []byte("never read"), 0600); err != nil { t.Fatal(err) }
 if err := os.Symlink(outside, filepath.Join(dir, "report.json")); err != nil { t.Fatal(err) }
 if err := os.WriteFile(filepath.Join(dir, "control.json"), []byte(`{"enabled":true}`), 0600); err != nil { t.Fatal(err) }
 if err := SetEnabled(dir, false); err != nil { t.Fatal(err) }
 b, err := os.ReadFile(filepath.Join(dir, "control.json"))
 if err != nil || string(b) != `{"enabled":false}` { t.Fatalf("old helper was not disabled: %s %v", b, err) }
}

func TestControlRejectsUnavailableHelper(t *testing.T) {
	dir, b := fixture(t)
	b = []byte(stringReplace(string(b), "collecting", "database_unavailable"))
	os.WriteFile(filepath.Join(dir, "report.json"), b, 0600)
	if err := SetEnabled(dir, true); err == nil {
		t.Fatal("unusable helper acknowledged")
	}
}
func stringReplace(s, old, new string) string { return strings.ReplaceAll(s, old, new) }

func TestControlRequiresFreshProvisionedHelper(t *testing.T) {
	dir, payload := fixture(t)
	if err := SetEnabled(dir, true); err != nil {
		t.Fatal(err)
	}
	var control map[string]interface{}
	b, _ := os.ReadFile(filepath.Join(dir, "control.json"))
	json.Unmarshal(b, &control)
	if len(control) != 1 || control["enabled"] != true {
		t.Fatal(string(b))
	}
	os.WriteFile(filepath.Join(dir, "report.json"), append(payload[:len(payload)-1], []byte(`,"access_token":"secret"}`)...), 0600)
	if err := SetEnabled(dir, true); err == nil {
		t.Fatal("sensitive field accepted")
	}
	if err := SetEnabled(filepath.Join(t.TempDir(), "absent"), true); err == nil {
		t.Fatal("unprovisioned accepted")
	}
}
