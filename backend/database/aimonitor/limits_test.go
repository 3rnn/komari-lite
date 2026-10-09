package aimonitor

import (
	"fmt"
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestCardinalityLimitRollsBackWholeReport(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	r := validReport(now)
	base := r.Samples[0]
	r.Samples = nil
	for i := 0; i < 129; i++ {
		row := base
		row.SampleID = uuid.NewString()
		row.Model = fmt.Sprintf("model-%d", i)
		r.Samples = append(r.Samples, row)
	}
	if _, err := s.Ingest("node", r, now); err == nil {
		t.Fatal("unbounded dimensions accepted")
	}
	var count int64
	s.DB.Model(&StoredSample{}).Count(&count)
	if count != 0 {
		t.Fatal("failed report partially stored")
	}
}
func TestOverviewRejectsOverflowInsteadOfReturningNegativeTotals(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	hour := float64(now.Unix()/3600*3600 - 3600)
	rows := make([]Rollup, 1025)
	for i := range rows {
		rows[i] = Rollup{Node: "node", Hour: hour, Model: "exact", Provider: "fixture", Totals: Totals{TotalTokenCount: MaxSafeJSONInteger}}
	}
	if err := s.DB.CreateInBatches(rows, 128).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Overview(Query{From: hour - 3600, To: hour + 3600, Timezone: "UTC"}, now); err == nil {
		t.Fatal("overflowed totals accepted")
	}
}
func TestQuotaStorageHasHardCapacity(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	rows := make([]AccountQuota, 1024)
	for i := range rows {
		rows[i] = AccountQuota{Provider: "openai", AccountKey: fmt.Sprintf("account-%d", i), UpdatedAt: float64(now.Unix()), Payload: `{}`}
	}
	if err := s.DB.CreateInBatches(rows, 128).Error; err != nil {
		t.Fatal(err)
	}
	r := validReport(now)
	r.Quota = &Quota{Provider: "openai", AccountKey: "new-account", Status: "unavailable", ObservedAt: float64(now.Unix()), Windows: []QuotaWindow{}}
	if _, err := s.Ingest("node", r, now); err == nil {
		t.Fatal("quota cardinality unbounded")
	}
}
func TestOldLastSuccessDoesNotInvalidateCurrentHealthReport(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	r := validReport(now)
	r.Samples = nil
	r.Status = "database_unavailable"
	r.LastSuccessAt = floatPtr(float64(now.Add(-90 * 24 * time.Hour).Unix()))
	if _, err := s.Ingest("node", r, now); err != nil {
		t.Fatalf("old last-success invalidated current health: %v", err)
	}
}
func TestStorageIndexesAndNodeCapacityBoundMaintenanceWork(t *testing.T) {
	s := fixture(t)
	if !s.DB.Migrator().HasIndex(&StoredSample{}, "idx_ai_samples_ended_at") {
		t.Fatal("retention queries require an ended_at index")
	}
	states := make([]NodeState, 1000)
	for i := range states {
		states[i] = NodeState{Node: fmt.Sprintf("node-%d", i)}
	}
	if err := s.DB.CreateInBatches(states, 128).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1780000000, 0)
	if _, err := s.Ingest("over-capacity-node", validReport(now), now); err == nil {
		t.Fatal("AI tracked-node cardinality unbounded")
	}
}
func TestPricingAppendReturnsImmutableServerID(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	rate := 1.0
	rule, err := s.AppendPricing(PricingRule{Provider: "fixture", Model: "exact", InputPerMillion: &rate, OutputPerMillion: &rate, Source: "operator override", ProcessingMode: "explicit", Version: "1", EffectiveAt: float64(now.Unix())}, now)
	if err != nil || rule.ID == 0 {
		t.Fatalf("append did not return persisted identifier: %v %+v", err, rule)
	}
}
