package aimonitor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

const MaxDimensionsPerNode = 128
const MaxDedupPerNode = 100000
const MaxPricingRules = 2048
const MaxResponseGroups = 512
const MaxSafeJSONInteger int64 = 9007199254740991

type Dimension struct {
	Node     string  `gorm:"size:36;primaryKey"`
	Key      string  `gorm:"size:64;primaryKey"`
	LastSeen float64 `gorm:"index"`
}

func (Dimension) TableName() string { return "ai_dimensions" }
func recordDimension(tx *gorm.DB, node string, s Sample, now time.Time) error {
	raw := s.Provider + "\x00" + s.Model + "\x00" + s.BillingMode + "\x00" + s.Task
	digest := sha256.Sum256([]byte(raw))
	key := hex.EncodeToString(digest[:])
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&Dimension{Node: node, Key: key, LastSeen: float64(now.Unix())})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		var count int64
		if err := tx.Model(&Dimension{}).Where("node = ?", node).Count(&count).Error; err != nil {
			return err
		}
		if count > MaxDimensionsPerNode {
			return fmt.Errorf("AI dimension limit is %d per node", MaxDimensionsPerNode)
		}
	}
	return tx.Model(&Dimension{}).Where("node = ? AND key = ?", node, key).Update("last_seen", float64(now.Unix())).Error
}
func safeAdd(a, b Totals) bool {
	for _, pair := range [][2]int64{{a.Calls, b.Calls}, {a.TotalTokenCount, b.TotalTokenCount}, {a.UnpricedCalls, b.UnpricedCalls}, {a.PricedCalls, b.PricedCalls}} {
		if pair[0] < 0 || pair[1] < 0 || pair[0] > MaxSafeJSONInteger-pair[1] {
			return false
		}
	}
	return true
}
