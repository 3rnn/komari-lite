package aimonitor

import (
	"testing"
	"time"
)

func TestRetentionRollupPreservesFrozenTotalsAndRetryDedup(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	r := validReport(now)
	if _, err := s.Ingest("node", r, now); err != nil {
		t.Fatal(err)
	}
	later := now.Add(25 * time.Hour)
	if err := s.Maintain(later); err != nil {
		t.Fatal(err)
	}
	var count int64
	s.DB.Model(&StoredSample{}).Count(&count)
	if count != 0 {
		t.Fatalf("detail not retired: %d", count)
	}
	var rows []Rollup
	s.DB.Find(&rows)
	if len(rows) != 1 || rows[0].Calls != 2 || rows[0].TotalTokens() != 160 {
		t.Fatalf("bad rollup: %+v", rows)
	}
	if _, err := s.Ingest("node", r, later); err != nil {
		t.Fatal(err)
	}
	s.DB.Model(&StoredSample{}).Count(&count)
	if count != 0 {
		t.Fatal("retry replayed compacted sample")
	}
	if err := s.Maintain(now.Add(31 * 24 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	s.DB.Model(&Rollup{}).Count(&count)
	if count != 0 {
		t.Fatal("rollups not expired")
	}
	s.DB.Model(&Dedup{}).Count(&count)
	if count != 0 {
		t.Fatal("dedup not expired")
	}
}
func TestSettingsBoundsAndIndependentSchema(t *testing.T) {
	s := fixture(t)
	settings, err := s.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.RetentionDays != 30 || settings.DetailedRetentionHours != 24 {
		t.Fatalf("defaults: %+v", settings)
	}
	settings.RetentionDays = 31
	if err := s.SetSettings(settings); err == nil {
		t.Fatal("unbounded retention accepted")
	}
	settings.RetentionDays = 7
	settings.Timezone = "Not/AZone"
	if err := s.SetSettings(settings); err == nil {
		t.Fatal("invalid timezone accepted")
	}
	if err := s.DB.Model(&Schema{}).Where("id = 1").Update("version", 3).Error; err != nil {
		t.Fatal(err)
	}
	if err := Migrate(s.DB); err == nil {
		t.Fatal("future AI schema accepted")
	}
}

func TestDedupRetentionIsIndependentOfConfiguredHistoryRetention(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	if err := s.SetSettings(Settings{DetailedRetentionHours: 1, RetentionDays: 1, Timezone: "UTC"}); err != nil {
		t.Fatal(err)
	}
	kept := Dedup{Node: "node", SampleID: "00000000-0000-4000-8000-000000000001", EndedAt: float64(now.Add(-DedupRetention + time.Hour).Unix())}
	expired := Dedup{Node: "node", SampleID: "00000000-0000-4000-8000-000000000002", EndedAt: float64(now.Add(-DedupRetention - time.Hour).Unix())}
	if err := s.DB.Create(&[]Dedup{kept, expired}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.Maintain(now); err != nil {
		t.Fatal(err)
	}
	var rows []Dedup
	if err := s.DB.Order("sample_id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].SampleID != kept.SampleID {
		t.Fatalf("dedup retention removed replay protection early: %#v", rows)
	}
}
