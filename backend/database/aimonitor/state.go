package aimonitor

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

var ErrRateLimited = errors.New("AI report rate limit exceeded")
var ErrQueryLimit = errors.New("AI query work limit exceeded; narrow the range or filters")

type NodeState struct {
	Node                    string `gorm:"size:36;primaryKey"`
	CollectorID             string `gorm:"size:36"`
	Enabled                 bool
	Status                  string `gorm:"size:32"`
	CurrentModel            string `gorm:"size:128"`
	ObservedAt              float64
	LastSuccessAt           *float64
	LastReportAt            float64 `gorm:"index"`
	SamplingIntervalSeconds float64
	QuotaProvider           string `gorm:"size:64"`
	QuotaAccount            string `gorm:"size:128"`
}

func (NodeState) TableName() string { return "ai_node_states" }

type AccountQuota struct {
	Provider   string `gorm:"size:64;primaryKey"`
	AccountKey string `gorm:"size:128;primaryKey"`
	ObservedAt float64
	UpdatedAt  float64 `gorm:"index"`
	Payload    string  `gorm:"type:text"`
}

func (AccountQuota) TableName() string { return "ai_account_quotas" }
func saveState(tx *gorm.DB, node string, r Report, now time.Time) error {
	current := float64(now.UnixNano()) / 1e9
	var previous NodeState
	err := tx.First(&previous, "node = ?", node).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		var count int64
		if err := tx.Model(&NodeState{}).Count(&count).Error; err != nil {
			return err
		}
		if count >= MaxOverviewNodes {
			return errors.New("AI tracked-node capacity reached")
		}
	}
	if err == nil && current-previous.LastReportAt < 2 {
		return ErrRateLimited
	}
	// Late spool deliveries cannot regress health or quota associations.
	state := previous
	state.Node = node
	state.LastReportAt = current
	if r.ObservedAt >= previous.ObservedAt {
		state.CollectorID = r.CollectorID
		state.Enabled = r.Enabled
		state.Status = r.Status
		state.CurrentModel = r.CurrentModel
		state.ObservedAt = r.ObservedAt
		state.SamplingIntervalSeconds = r.SamplingIntervalSeconds
		if r.LastSuccessAt != nil && (state.LastSuccessAt == nil || *r.LastSuccessAt > *state.LastSuccessAt) {
			state.LastSuccessAt = r.LastSuccessAt
		}
		if r.Quota != nil {
			state.QuotaProvider = r.Quota.Provider
			state.QuotaAccount = r.Quota.AccountKey
		}
	}
	if err := tx.Save(&state).Error; err != nil {
		return err
	}
	if r.Quota == nil || r.Quota.AccountKey == "" {
		return nil
	}
	q := *r.Quota
	var old AccountQuota
	err = tx.First(&old, "provider = ? AND account_key = ?", q.Provider, q.AccountKey).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		var count int64
		if err := tx.Model(&AccountQuota{}).Count(&count).Error; err != nil {
			return err
		}
		if count >= 1024 {
			return errors.New("AI quota account capacity reached")
		}
	}
	if err == nil {
		if q.ObservedAt < old.ObservedAt {
			return nil
		}
		var oldQ Quota
		if err := json.Unmarshal([]byte(old.Payload), &oldQ); err != nil {
			return err
		}
		if oldQ.LastSuccessAt != nil && (q.LastSuccessAt == nil || *q.LastSuccessAt < *oldQ.LastSuccessAt) {
			q.LastSuccessAt = oldQ.LastSuccessAt
		}
	}
	raw, err := json.Marshal(q)
	if err != nil {
		return err
	}
	row := AccountQuota{Provider: q.Provider, AccountKey: q.AccountKey, ObservedAt: q.ObservedAt, UpdatedAt: current, Payload: string(raw)}
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "provider"}, {Name: "account_key"}}, DoUpdates: clause.AssignmentColumns([]string{"observed_at", "updated_at", "payload"})}).Create(&row).Error
}
