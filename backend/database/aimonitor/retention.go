package aimonitor

import (
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"

	"regexp"
	"time"
)

var customModelIdentifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

type Settings struct {
	Timezone               string   `json:"timezone"`
	RetentionDays          int      `json:"retention_days"`
	DetailedRetentionHours int      `json:"detailed_retention_hours"`
	CustomModels           []string `json:"custom_models"`
}
type SettingsRow struct {
	ID      int    `gorm:"primaryKey"`
	Payload string `gorm:"type:text"`
}

func (SettingsRow) TableName() string { return "ai_settings" }
func (s *Store) Settings() (Settings, error) {
	defaults := Settings{Timezone: "UTC", RetentionDays: 30, DetailedRetentionHours: 24}
	var row SettingsRow
	err := s.DB.First(&row, "id = ?", 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return defaults, nil
	}
	if err != nil {
		return defaults, err
	}
	err = json.Unmarshal([]byte(row.Payload), &defaults)
	return defaults, err
}
func (s *Store) SetSettings(v Settings) error {
	if _, err := time.LoadLocation(v.Timezone); err != nil {
		return fmt.Errorf("unknown timezone")
	}
	if len(v.Timezone) > 64 || v.RetentionDays < 1 || v.RetentionDays > 30 || v.DetailedRetentionHours < 1 || v.DetailedRetentionHours > 24 || v.DetailedRetentionHours > v.RetentionDays*24 {
		return fmt.Errorf("retention must be 1..30 days with 1..24 detailed hours")
	}
	if len(v.CustomModels) > 64 {
		return fmt.Errorf("too many custom models")
	}
	seen := make(map[string]struct{}, len(v.CustomModels))
	for _, model := range v.CustomModels {
		if !customModelIdentifier.MatchString(model) {
			return fmt.Errorf("invalid custom model identifier")
		}
		if _, duplicate := seen[model]; duplicate {
			return fmt.Errorf("duplicate custom model identifier")
		}
		seen[model] = struct{}{}
	}
	raw, _ := json.Marshal(v)
	return s.DB.Save(&SettingsRow{ID: 1, Payload: string(raw)}).Error
}

type Totals struct {
	Calls                  int64    `json:"calls"`
	InputTokens            int64    `json:"input_tokens"`
	OutputTokens           int64    `json:"output_tokens"`
	CacheReadTokens        int64    `json:"cache_read_tokens"`
	CacheWriteTokens       int64    `json:"cache_write_tokens"`
	ReasoningTokens        int64    `json:"reasoning_tokens"`
	TotalTokenCount        int64    `json:"total_tokens"`
	APIEquivalentCostUSD   *float64 `json:"api_equivalent_cost_usd"`
	UnpricedCalls          int64    `json:"unpriced_calls"`
	PricedCalls            int64    `json:"priced_calls"`
	HermesEstimatedCostUSD *float64 `json:"hermes_estimated_cost_usd"`
	RecordedActualCostUSD  *float64 `json:"recorded_actual_cost_usd"`
}

func (t Totals) TotalTokens() int64 { return t.TotalTokenCount }
func addMoney(dst **float64, src *float64) {
	if src == nil {
		return
	}
	if *dst == nil {
		v := *src
		*dst = &v
	} else {
		**dst += *src
	}
}
func (t *Totals) addSample(s StoredSample) {
	t.Calls += s.Calls
	t.InputTokens += s.InputTokens
	t.OutputTokens += s.OutputTokens
	t.CacheReadTokens += s.CacheReadTokens
	t.CacheWriteTokens += s.CacheWriteTokens
	t.ReasoningTokens += s.ReasoningTokens
	t.TotalTokenCount += s.TotalTokens()
	if s.APIEquivalentCostUSD == nil {
		t.UnpricedCalls += s.Calls
	} else {
		t.PricedCalls += s.Calls
	}
	addMoney(&t.APIEquivalentCostUSD, s.APIEquivalentCostUSD)
	addMoney(&t.HermesEstimatedCostUSD, s.HermesEstimatedCostUSD)
	addMoney(&t.RecordedActualCostUSD, s.RecordedActualCostUSD)
}
func (t *Totals) add(v Totals) {
	t.Calls += v.Calls
	t.InputTokens += v.InputTokens
	t.OutputTokens += v.OutputTokens
	t.CacheReadTokens += v.CacheReadTokens
	t.CacheWriteTokens += v.CacheWriteTokens
	t.ReasoningTokens += v.ReasoningTokens
	t.TotalTokenCount += v.TotalTokenCount
	t.UnpricedCalls += v.UnpricedCalls
	t.PricedCalls += v.PricedCalls
	addMoney(&t.APIEquivalentCostUSD, v.APIEquivalentCostUSD)
	addMoney(&t.HermesEstimatedCostUSD, v.HermesEstimatedCostUSD)
	addMoney(&t.RecordedActualCostUSD, v.RecordedActualCostUSD)
}

type Rollup struct {
	ID            uint64  `gorm:"primaryKey"`
	Node          string  `gorm:"size:36;index"`
	Hour          float64 `gorm:"index"`
	Model         string  `gorm:"size:128"`
	Provider      string  `gorm:"size:64"`
	BillingMode   string  `gorm:"size:64"`
	Task          string  `gorm:"size:128"`
	PriceID       uint64
	PriceSnapshot string `gorm:"type:text"`
	Totals        `gorm:"embedded"`
}

func (Rollup) TableName() string { return "ai_hourly_rollups" }

const MaintenanceBatch = 512

// Maintain bounds every cleanup transaction. Retries are atomic: detailed rows
// retire only after their frozen counters and reference snapshots are preserved.
func (s *Store) Maintain(now time.Time) error {
	settings, err := s.Settings()
	if err != nil {
		return err
	}
	detailCutoff := float64(now.Unix() - int64(settings.DetailedRetentionHours)*3600)
	cutoff := float64(now.Unix() - int64(settings.RetentionDays)*86400)
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var rows []StoredSample
		if err := tx.Where("ended_at < ?", detailCutoff).Order("ended_at, id").Limit(MaintenanceBatch).Find(&rows).Error; err != nil {
			return err
		}
		ids := make([]uint64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
			if row.EndedAt < cutoff {
				continue
			}
			hour := float64(int64(row.EndedAt) / 3600 * 3600)
			var roll Rollup
			err := tx.Where("node = ? AND hour = ? AND model = ? AND provider = ? AND billing_mode = ? AND task = ? AND price_id = ?", row.Node, hour, row.Model, row.Provider, row.BillingMode, row.Task, row.PriceID).First(&roll).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if errors.Is(err, gorm.ErrRecordNotFound) {
				roll = Rollup{Node: row.Node, Hour: hour, Model: row.Model, Provider: row.Provider, BillingMode: row.BillingMode, Task: row.Task, PriceID: row.PriceID, PriceSnapshot: row.PriceSnapshot}
			}
			var incoming Totals
			incoming.addSample(row)
			if !safeAdd(roll.Totals, incoming) {
				return fmt.Errorf("AI rollup totals exceed safe integer range")
			}
			roll.Totals.add(incoming)
			if err := tx.Save(&roll).Error; err != nil {
				return err
			}
		}
		if len(ids) > 0 {
			if err := tx.Where("id IN ?", ids).Delete(&StoredSample{}).Error; err != nil {
				return err
			}
		}
		var expired []Rollup
		if err := tx.Select("id").Where("hour + 3600 < ?", cutoff).Limit(MaintenanceBatch).Find(&expired).Error; err != nil {
			return err
		}
		ids = ids[:0]
		for _, v := range expired {
			ids = append(ids, v.ID)
		}
		if len(ids) > 0 {
			if err := tx.Where("id IN ?", ids).Delete(&Rollup{}).Error; err != nil {
				return err
			}
		}
		var ledger []Dedup
		if err := tx.Where("ended_at < ?", float64(now.Add(-DedupRetention).Unix())).Limit(MaintenanceBatch).Find(&ledger).Error; err != nil {
			return err
		}
		for _, v := range ledger {
			if err := tx.Delete(&v).Error; err != nil {
				return err
			}
		}
		var accounts []AccountQuota
		if err := tx.Where("updated_at < ?", cutoff).Limit(MaintenanceBatch).Find(&accounts).Error; err != nil {
			return err
		}
		for _, v := range accounts {
			if err := tx.Delete(&v).Error; err != nil {
				return err
			}
		}
		var states []NodeState
		if err := tx.Where("last_report_at < ?", cutoff).Limit(MaintenanceBatch).Find(&states).Error; err != nil {
			return err
		}
		for _, v := range states {
			if err := tx.Delete(&v).Error; err != nil {
				return err
			}
		}
		var dims []Dimension
		if err := tx.Where("last_seen < ?", float64(now.Unix()-30*86400)).Limit(MaintenanceBatch).Find(&dims).Error; err != nil {
			return err
		}
		for _, v := range dims {
			if err := tx.Delete(&v).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
