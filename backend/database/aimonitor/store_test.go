package aimonitor

import (
	"encoding/json"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"path/filepath"
	"testing"
	"time"
)

func fixture(t *testing.T) *Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "ai.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if err = Migrate(db); err != nil {
		t.Fatal(err)
	}
	return NewStore(db)
}
func validReport(now time.Time) Report {
	return Report{Version: 1, CollectorID: "550e8400-e29b-41d4-a716-446655440000", Enabled: true, Status: "collecting", ObservedAt: float64(now.Unix()), SamplingIntervalSeconds: 10, Samples: []Sample{{SampleID: "550e8400-e29b-41d4-a716-446655440001", StartedAt: float64(now.Unix() - 10), EndedAt: float64(now.Unix()), Model: "gpt-explicit", Provider: "openai", BillingMode: "reference", Task: "chat", Calls: 2, InputTokens: 100, OutputTokens: 20, CacheReadTokens: 40, ReasoningTokens: 10}}}
}
func TestValidationRejectsUnsafeReportsAndPricing(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	r := validReport(now)
	r.Samples[0].ReasoningTokens = 21
	if _, err := s.Ingest("node", r, now); err == nil {
		t.Fatal("reasoning cannot exceed output")
	}
	r = validReport(now)
	r.Samples[0].EndedAt = float64(now.Unix() + 121)
	if _, err := s.Ingest("node", r, now); err == nil {
		t.Fatal("future samples accepted")
	}
	r = validReport(now)
	r.Samples[0].InputTokens = -1
	if _, err := s.Ingest("node", r, now); err == nil {
		t.Fatal("negative counter accepted")
	}
	if _, err := s.AppendPricing(PricingRule{Provider: "openai", Model: "gpt-explicit"}, now); err == nil {
		t.Fatal("ambiguous pricing accepted")
	}
}
func TestStrictDecodeRejectsSensitiveAndDuplicateFields(t *testing.T) {
	for _, raw := range []string{`{"version":1,"session_id":"secret"}`, `{"version":1,"version":1}`, `{"version":1} {"version":1}`} {
		var r Report
		if err := DecodeStrict([]byte(raw), &r); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
func TestStrictReportRequiresSampleCountersAndExactFieldNames(t *testing.T) {
	r := validReport(time.Unix(1780000000, 0))
	raw, _ := json.Marshal(r)
	var body map[string]any
	json.Unmarshal(raw, &body)
	sample := body["samples"].([]any)[0].(map[string]any)
	sample["input_tokens"] = nil
	payload, _ := json.Marshal(body)
	var decoded Report
	if err := DecodeStrict(payload, &decoded); err == nil {
		t.Fatal("null required counter accepted as zero")
	}
	sample["input_tokens"] = 100
	body["Current_Model"] = "exact"
	payload, _ = json.Marshal(body)
	if err := DecodeStrict(payload, &decoded); err == nil {
		t.Fatal("case-insensitive undeclared field accepted")
	}
}
func TestNodeRateLimitAndQuotaMonotonicDedup(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	r := validReport(now)
	percent := 40.0
	r.Quota = &Quota{AccountKey: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Provider: "openai", Plan: "Plus", Status: "available", ObservedAt: float64(now.Unix()), LastSuccessAt: floatPtr(float64(now.Unix())), Windows: []QuotaWindow{{Label: "Session", UsedPercent: &percent}}}
	if _, err := s.Ingest("node-a", r, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Ingest("node-a", r, now.Add(time.Second)); err != ErrRateLimited {
		t.Fatalf("rate limit: %v", err)
	}
	if _, err := s.Ingest("node-b", r, now); err != nil {
		t.Fatal(err)
	}
	var quotas []AccountQuota
	s.DB.Find(&quotas)
	if len(quotas) != 1 {
		t.Fatalf("same account duplicated: %d", len(quotas))
	}
	r.Quota.ObservedAt -= 10
	r.Quota.Windows[0].UsedPercent = floatPtr(1)
	if _, err := s.Ingest("node-a", r, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	quotas = nil
	s.DB.Find(&quotas)
	var q Quota
	json.Unmarshal([]byte(quotas[0].Payload), &q)
	if *q.Windows[0].UsedPercent != 40 {
		t.Fatal("older quota overwrote latest")
	}
}
func TestNewerReportWithoutMatchedQuotaClearsOldAccountAssociation(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	r := validReport(now)
	r.Quota = &Quota{AccountKey: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Provider: "openai", Plan: "plus", Status: "available", ObservedAt: float64(now.Unix()), Windows: []QuotaWindow{}}
	if _, err := s.Ingest("node", r, now); err != nil {
		t.Fatal(err)
	}
	r.ObservedAt++
	r.Quota = nil
	if _, err := s.Ingest("node", r, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	view, err := s.Overview(Query{Preset: "24H", Timezone: "UTC"}, now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Accounts) != 0 {
		t.Fatalf("old account remained visible: %+v", view.Accounts)
	}
}
func TestRetainedDedupRowsDoNotBlockAtomicRelayBatch(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	rows := make([]Dedup, 0, 100000)
	for range 100000 {
		rows = append(rows, Dedup{Node: "node", SampleID: uuid.NewString(), EndedAt: float64(now.Add(-time.Hour).Unix())})
	}
	if err := s.DB.CreateInBatches(rows, 1000).Error; err != nil {
		t.Fatal(err)
	}
	r := validReport(now)
	r.Samples = append(r.Samples, Sample{SampleID: uuid.NewString(), StartedAt: float64(now.Unix() - 10), EndedAt: float64(now.Unix()), Model: "gpt-explicit", Provider: "openai", BillingMode: "reference", Task: "chat"})
	ack, err := s.Ingest("node", r, now)
	if err != nil {
		t.Fatalf("retained dedup rows blocked atomic relay batch: %v", err)
	}
	if len(ack) != 2 {
		t.Fatalf("accepted %d samples, want full batch", len(ack))
	}
}
func floatPtr(v float64) *float64 { return &v }
func TestIngestDeduplicatesAndFreezesExplicitPrice(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	r := validReport(now)
	rate := 1.0
	rule := PricingRule{Provider: "openai", Model: "gpt-explicit", BillingMode: "reference", InputPerMillion: &rate, OutputPerMillion: &rate, CacheReadPerMillion: &rate, Source: "operator override", Version: "v1", ProcessingMode: "explicit reference", EffectiveAt: float64(now.Unix() - 100)}
	if _, err := s.AppendPricing(rule, now); err != nil {
		t.Fatal(err)
	}
	ack, err := s.Ingest("node", r, now)
	if err != nil || len(ack) != 1 {
		t.Fatalf("ingest: %v %v", ack, err)
	}
	rate2 := 100.0
	rule.InputPerMillion = &rate2
	rule.Version = "v2"
	if _, err = s.AppendPricing(rule, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Ingest("node", r, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	var rows []StoredSample
	if err = s.DB.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("dedup rows=%d", len(rows))
	}
	if rows[0].APIEquivalentCostUSD == nil || *rows[0].APIEquivalentCostUSD != 0.00016 {
		t.Fatalf("frozen cost: %+v", rows[0])
	}
	if rows[0].TotalTokens() != 160 {
		t.Fatalf("reasoning or cache double counted: %d", rows[0].TotalTokens())
	}
}

func TestPricingUsesLatestApplicableEffectiveTimestamp(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	r := validReport(now)
	oldRate, newRate := 1.0, 3.0
	// Append the newer rule first, then backfill an older effective rule. Rule
	// insertion order must not rewrite the price that applied at sample time.
	for _, rule := range []PricingRule{
		{Provider: "openai", Model: "gpt-explicit", BillingMode: "reference", InputPerMillion: &newRate, OutputPerMillion: &newRate, CacheReadPerMillion: &newRate, Source: "official", Version: "new", ProcessingMode: "standard", EffectiveAt: float64(now.Unix() - 20)},
		{Provider: "openai", Model: "gpt-explicit", BillingMode: "reference", InputPerMillion: &oldRate, OutputPerMillion: &oldRate, CacheReadPerMillion: &oldRate, Source: "official", Version: "old", ProcessingMode: "standard", EffectiveAt: float64(now.Unix() - 100)},
	} {
		if _, err := s.AppendPricing(rule, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Ingest("node-effective", r, now); err != nil {
		t.Fatal(err)
	}
	var row StoredSample
	if err := s.DB.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	var snapshot PricingRule
	if err := json.Unmarshal([]byte(row.PriceSnapshot), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Version != "new" {
		t.Fatalf("selected %q, want newest effective pricing", snapshot.Version)
	}
}

func TestQuotaRejectsRawAccountIdentifiers(t *testing.T) {
	now := time.Unix(1780000000, 0)
	base := Quota{Provider: "openai-codex", Status: "available", ObservedAt: float64(now.Unix()), Windows: []QuotaWindow{}}
	for _, invalid := range []string{"operator@example.test", "account-alias", "eyJhbGciOiJIUzI1NiJ9.payload.signature"} {
		q := base
		q.AccountKey = invalid
		if err := q.Validate(now); err == nil {
			t.Fatalf("accepted raw account identifier %q", invalid)
		}
	}
	q := base
	q.AccountKey = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := q.Validate(now); err != nil {
		t.Fatalf("opaque SHA-256 account key rejected: %v", err)
	}
}
