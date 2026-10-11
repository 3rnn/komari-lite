package aimonitor

import (
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type Sample struct {
	SampleID               string   `json:"sample_id" gorm:"size:36"`
	StartedAt              float64  `json:"started_at"`
	EndedAt                float64  `json:"ended_at" gorm:"index;index:ai_sample_node_end,priority:2"`
	Model                  string   `json:"model" gorm:"size:128"`
	Provider               string   `json:"provider" gorm:"size:64"`
	BillingMode            string   `json:"billing_mode" gorm:"size:64"`
	Task                   string   `json:"task" gorm:"size:128"`
	Calls                  int64    `json:"calls"`
	InputTokens            int64    `json:"input_tokens"`
	OutputTokens           int64    `json:"output_tokens"`
	CacheReadTokens        int64    `json:"cache_read_tokens"`
	CacheWriteTokens       int64    `json:"cache_write_tokens"`
	ReasoningTokens        int64    `json:"reasoning_tokens"`
	HermesEstimatedCostUSD *float64 `json:"hermes_estimated_cost_usd,omitempty"`
	RecordedActualCostUSD  *float64 `json:"recorded_actual_cost_usd,omitempty"`
	CostStatus             string   `json:"cost_status,omitempty" gorm:"size:64"`
	CostSource             string   `json:"cost_source,omitempty" gorm:"size:128"`
	Coverage               *bool    `json:"coverage,omitempty"`
	ResetCount             int64    `json:"reset_count,omitempty"`
}

func (s Sample) TotalTokens() int64 {
	return s.InputTokens + s.OutputTokens + s.CacheReadTokens + s.CacheWriteTokens
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
type Quota struct {
	AccountKey     string        `json:"account_key,omitempty"`
	Provider       string        `json:"provider"`
	Plan           string        `json:"plan"`
	Status         string        `json:"status"`
	ObservedAt     float64       `json:"observed_at"`
	LastSuccessAt  *float64      `json:"last_success_at,omitempty"`
	Windows        []QuotaWindow `json:"windows"`
	ResetBankCount *float64      `json:"reset_bank_count,omitempty"`
}
type QuotaWindow struct {
	Label         string   `json:"label"`
	UsedPercent   *float64 `json:"used_percent"`
	ResetsAt      *string  `json:"resets_at,omitempty"`
	WindowSeconds *float64 `json:"window_seconds,omitempty"`
}
type PricingRule struct {
	ReferenceModel       string    `json:"reference_model,omitempty" gorm:"size:128"`
	ID                   uint64    `json:"id" gorm:"primaryKey"`
	Provider             string    `json:"provider" gorm:"size:64;index"`
	Model                string    `json:"model" gorm:"size:128;index"`
	BillingMode          string    `json:"billing_mode" gorm:"size:64"`
	InputPerMillion      *float64  `json:"input_per_million"`
	OutputPerMillion     *float64  `json:"output_per_million"`
	CacheReadPerMillion  *float64  `json:"cache_read_per_million"`
	CacheWritePerMillion *float64  `json:"cache_write_per_million"`
	Source               string    `json:"source" gorm:"size:128"`
	SourceURL            string    `json:"source_url" gorm:"size:2048"`
	Version              string    `json:"version" gorm:"size:128"`
	ProcessingMode       string    `json:"processing_mode" gorm:"size:128"`
	ReferenceTier        string    `json:"reference_tier" gorm:"size:128"`
	EffectiveAt          float64   `json:"effective_at"`
	CreatedAt            time.Time `json:"created_at"`
}

func (PricingRule) TableName() string { return "ai_pricing_rules" }

type StoredSample struct {
	ID                   uint64 `gorm:"primaryKey"`
	Node                 string `gorm:"size:36;index:ai_sample_node_end,priority:1"`
	Sample               `gorm:"embedded"`
	PriceID              uint64
	PriceSnapshot        string `gorm:"type:text"`
	APIEquivalentCostUSD *float64
	IngestedAt           float64
}

func (StoredSample) TableName() string { return "ai_samples" }

type Dedup struct {
	Node     string  `gorm:"size:36;primaryKey"`
	SampleID string  `gorm:"size:36;primaryKey"`
	EndedAt  float64 `gorm:"index"`
}

func (Dedup) TableName() string { return "ai_dedup" }

type Schema struct {
	ID      int `gorm:"primaryKey"`
	Version int
}

func (Schema) TableName() string { return "ai_schema" }

type Store struct{ DB *gorm.DB }

func NewStore(db *gorm.DB) *Store { return &Store{DB: db} }

// AI tables have an independent additive schema ledger. Do not modify the
// main database user_version or any Hermes storage.
func Migrate(db *gorm.DB) error {
	if db.Migrator().HasTable(&Schema{}) {
		var row Schema
		if err := db.First(&row, "id = ?", 1).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if row.Version > 2 {
			return fmt.Errorf("AI schema is newer than supported version 2")
		}
		if row.Version == 2 {
			return ensureAutomaticTerraPricing(db)
		}
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&Schema{}, &PricingRule{}, &StoredSample{}, &Dedup{}, &NodeState{}, &AccountQuota{}, &SettingsRow{}, &Rollup{}, &Dimension{}); err != nil {
			return err
		}
		return tx.Save(&Schema{ID: 1, Version: 2}).Error
	})
	if err != nil {
		return err
	}
	return ensureAutomaticTerraPricing(db)
}
func ensureAutomaticTerraPricing(db *gorm.DB) error {
	input, cacheRead, cacheWrite, output := 2.0, .2, 2.5, 12.0
	return db.Transaction(func(tx *gorm.DB) error {
		for _, billingMode := range []string{"subscription_included", "unknown"} {
			var count int64
			if err := tx.Model(&PricingRule{}).Where("provider = ? AND model = ? AND billing_mode = ?", "openai-codex", "gpt-5.6-terra", billingMode).Count(&count).Error; err != nil {
				return err
			}
			if count != 0 {
				continue
			}
			rule := PricingRule{ReferenceModel: "gpt-5.6-terra", Provider: "openai-codex", Model: "gpt-5.6-terra", BillingMode: billingMode, InputPerMillion: &input, CacheReadPerMillion: &cacheRead, CacheWritePerMillion: &cacheWrite, OutputPerMillion: &output, Source: "official_docs_snapshot", SourceURL: "https://developers.openai.com/api/docs/pricing", Version: "openai-pricing-2026-10-11-gpt-5.6-terra", ProcessingMode: "standard", ReferenceTier: "short", EffectiveAt: 0, CreatedAt: time.Now().UTC()}
			if err := tx.Create(&rule).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *Store) AppendPricing(rule PricingRule, now time.Time) (PricingRule, error) {
	var selectionErr error
	rule, selectionErr = resolveSelection(rule)
	if selectionErr != nil {
		return PricingRule{}, selectionErr
	}
	if err := rule.Validate(now); err != nil {
		return PricingRule{}, err
	}
	rule.ID = 0
	rule.CreatedAt = now.UTC()
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&PricingRule{}).Count(&count).Error; err != nil {
			return err
		}
		if count >= MaxPricingRules {
			return fmt.Errorf("AI pricing rule limit reached")
		}
		return tx.Create(&rule).Error
	})
	return rule, err
}
func price(sample Sample, rule PricingRule) *float64 {
	total := 0.0
	for _, pair := range []struct {
		tokens int64
		rate   *float64
	}{{sample.InputTokens, rule.InputPerMillion}, {sample.OutputTokens, rule.OutputPerMillion}, {sample.CacheReadTokens, rule.CacheReadPerMillion}, {sample.CacheWriteTokens, rule.CacheWritePerMillion}} {
		if pair.tokens == 0 {
			continue
		}
		if pair.rate == nil {
			return nil
		}
		total += float64(pair.tokens) * *pair.rate / 1e6
	}
	return &total
}
func (s *Store) Ingest(node string, r Report, now time.Time) ([]string, error) {
	if err := r.Validate(now); err != nil {
		return nil, err
	}
	accepted := make([]string, 0, len(r.Samples))
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := saveState(tx, node, r, now); err != nil {
			return err
		}
		for _, sample := range r.Samples {
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&Dedup{Node: node, SampleID: sample.SampleID, EndedAt: sample.EndedAt})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				if err := recordDimension(tx, node, sample, now); err != nil {
					return err
				}
				row := StoredSample{Node: node, Sample: sample, IngestedAt: float64(now.Unix())}
				var rule PricingRule
				err := tx.Where("provider = ? AND model = ? AND billing_mode = ? AND effective_at <= ?", sample.Provider, sample.Model, sample.BillingMode, sample.EndedAt).Order("effective_at DESC, id DESC").First(&rule).Error
				if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				if err == nil {
					row.PriceID = rule.ID
					raw, _ := json.Marshal(rule)
					row.PriceSnapshot = string(raw)
					row.APIEquivalentCostUSD = price(sample, rule)
				}
				if err := tx.Create(&row).Error; err != nil {
					return err
				}
			}
			accepted = append(accepted, sample.SampleID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return accepted, nil
}
