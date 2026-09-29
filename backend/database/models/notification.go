package models

import (
	"crypto/sha256"
	"fmt"
	"math"
	"strings"
	"time"
)

// Notification defines notification-related database models
type OfflineNotification struct {
	Client     string `json:"client" gorm:"type:varchar(36);not null;index;unique;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:client;references:UUID"`
	ClientInfo Client `json:"client_info,omitempty" gorm:"foreignKey:Client;references:UUID"`
	Enable     bool   `json:"enable" gorm:"type:boolean;default:false"`
	//Cooldown int `json:"cooldown" gorm:"type:int;not null;default:1800"` // Cooldown time (seconds), default 30 minutes
	GracePeriod  int        `json:"grace_period" gorm:"type:int;not null;default:180"` // Grace period (seconds), default 3 minutes
	LastNotified *time.Time `json:"last_notified"`                                     // Last notification time
}

// LoadNotification defines load notification rules based on the ratio of resource occupation to target time
type LoadNotification struct {
	Id           uint        `json:"id,omitempty" gorm:"primaryKey;autoIncrement"`
	Name         string      `json:"name" gorm:"type:varchar(255)"`
	Clients      StringArray `json:"clients" gorm:"type:longtext"`
	DefaultOn    bool        `json:"default_on" gorm:"column:all_clients;not null;default:false"` // Whether newly added servers automatically enable this alarm; existing servers are not affected by this field
	Metric       string      `json:"metric" gorm:"type:varchar(50);not null;default:'cpu'"`       // Monitor indicators such as cpu, ram, load
	Threshold    float32     `json:"threshold" gorm:"type:decimal(5,2);not null;default:80.00"`   // threshold percentage
	Ratio        float32     `json:"ratio" gorm:"type:decimal(5,2);not null;default:0.80"`        // Time to reach target ratio
	Interval     int         `json:"interval" gorm:"type:int;not null;default:15"`                // Monitoring interval (minutes)
	LastNotified *time.Time  `json:"last_notified"`                                               // Last notification time
}

// LoadNotificationRuleFingerprint identifies the fields that define one load
// alert incident. Presentation and assignment changes deliberately do not
// alter it, while a metric, threshold, ratio, or interval change starts a new
// incident and must not inherit the previous silence state.
func LoadNotificationRuleFingerprint(rule LoadNotification) string {
	payload := fmt.Sprintf("%s:%08x:%08x:%d",
		strings.ToLower(strings.TrimSpace(rule.Metric)),
		math.Float32bits(rule.Threshold), math.Float32bits(rule.Ratio), rule.Interval)
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("%x", sum)
}

// LoadNotificationState stores the latest evaluation and silence preference
// for one load notification rule and one assigned client.
type LoadNotificationState struct {
	NotificationID  uint             `json:"notification_id" gorm:"primaryKey;not null;index"`
	Notification    LoadNotification `json:"notification,omitempty" gorm:"foreignKey:NotificationID;references:Id;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	Client          string           `json:"client" gorm:"type:varchar(36);primaryKey;not null;index"`
	ClientInfo      Client           `json:"client_info,omitempty" gorm:"foreignKey:Client;references:UUID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	RuleFingerprint string           `json:"rule_fingerprint" gorm:"type:varchar(64);not null;default:''"`
	AlertActive     bool             `json:"alert_active" gorm:"type:boolean;not null;default:false;index"`
	ActiveSince     *time.Time       `json:"active_since"`
	LastEvaluatedAt time.Time        `json:"last_evaluated_at" gorm:"not null;index"`
	LatestValue     float64          `json:"latest_value" gorm:"not null;default:0"`
	MatchedSamples  int              `json:"matched_samples" gorm:"not null;default:0"`
	TotalSamples    int              `json:"total_samples" gorm:"not null;default:0"`
	LastNotified    *time.Time       `json:"last_notified"`
	RecoveryPending bool             `json:"recovery_pending" gorm:"type:boolean;not null;default:false;index"`
	SilencedUntil   *time.Time       `json:"silenced_until"`
	SilencedForever bool             `json:"silenced_forever" gorm:"type:boolean;not null;default:false"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

// TrafficReportNotification defines the database model for traffic scheduled reporting
type TrafficReportNotification struct {
	Client         string `json:"client" gorm:"type:varchar(36);not null;index;unique;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:client;references:UUID"`
	ClientInfo     Client `json:"client_info,omitempty" gorm:"foreignKey:Client;references:UUID"`
	Enable         bool   `json:"enable" gorm:"type:boolean;default:false"`
	Daily          bool   `json:"daily" gorm:"type:boolean;default:false"`           // daily newspaper
	Weekly         bool   `json:"weekly" gorm:"type:boolean;default:false"`          // weekly report
	Monthly        bool   `json:"monthly" gorm:"type:boolean;default:false"`         // monthly report
	IncludeTraffic bool   `json:"include_traffic" gorm:"type:boolean;default:true"`  // Upstream/Downstream traffic
	IncludeBilling bool   `json:"include_billing" gorm:"type:boolean;default:false"` // Traffic calculated according to server billing rules
}

// TrafficDailyLedger stores exact report traffic for one Beijing calendar day.
// The daily ledger is intentionally separate from the general metric store so
// weekly and monthly reports do not require long retention for four metrics.
type TrafficDailyLedger struct {
	Client     string    `json:"client" gorm:"type:varchar(36);primaryKey;not null"`
	ClientInfo Client    `json:"-" gorm:"foreignKey:Client;references:UUID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	Day        string    `json:"day" gorm:"type:varchar(10);primaryKey;not null"`
	UpBytes    int64     `json:"up_bytes" gorm:"type:bigint;not null;default:0"`
	DownBytes  int64     `json:"down_bytes" gorm:"type:bigint;not null;default:0"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TrafficCalibrationAdjustment stores one auditable traffic correction
// allocated to a Beijing calendar day. Raw Agent metrics remain unchanged.
type TrafficCalibrationAdjustment struct {
	ID            uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	CalibrationID string    `json:"calibration_id" gorm:"type:varchar(32);not null;index;uniqueIndex:idx_traffic_calibration_day"`
	Client        string    `json:"client" gorm:"type:varchar(36);not null;index;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:Client;references:UUID"`
	ClientInfo    Client    `json:"-" gorm:"foreignKey:Client;references:UUID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	Cycle         string    `json:"cycle" gorm:"type:varchar(10);not null;index"`
	Day           string    `json:"day" gorm:"type:varchar(10);not null;index;uniqueIndex:idx_traffic_calibration_day"`
	UpDelta       int64     `json:"up_delta" gorm:"type:bigint;not null;default:0"`
	DownDelta     int64     `json:"down_delta" gorm:"type:bigint;not null;default:0"`
	TargetUp      int64     `json:"target_up" gorm:"type:bigint;not null;default:0"`
	TargetDown    int64     `json:"target_down" gorm:"type:bigint;not null;default:0"`
	Operator      string    `json:"operator,omitempty" gorm:"type:varchar(36);not null;default:''"`
	CreatedAt     time.Time `json:"created_at" gorm:"index"`
}

// PingLossNotification defines packet-loss alerts for one client and ping task.
type PingLossNotification struct {
	Id              uint       `json:"id,omitempty" gorm:"primaryKey;autoIncrement"`
	Client          string     `json:"client" gorm:"type:varchar(36);not null;uniqueIndex:idx_ping_loss_notification_target"`
	ClientInfo      Client     `json:"client_info,omitempty" gorm:"foreignKey:Client;references:UUID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	TaskId          uint       `json:"task_id" gorm:"not null;uniqueIndex:idx_ping_loss_notification_target"`
	Task            PingTask   `json:"task,omitempty" gorm:"foreignKey:TaskId;references:Id;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	Enable          bool       `json:"enable" gorm:"type:boolean;default:false"`
	WindowSeconds   int        `json:"window_seconds" gorm:"type:int;not null;default:60"`
	LossThreshold   float64    `json:"loss_threshold" gorm:"type:decimal(5,2);not null;default:5.00"`
	MinimumSamples  int        `json:"minimum_samples" gorm:"type:int;not null;default:1"`
	CooldownSeconds int        `json:"cooldown_seconds" gorm:"type:int;not null;default:300"`
	LastNotified    *time.Time `json:"last_notified"`
	AlertActive     bool       `json:"alert_active" gorm:"type:boolean;not null;default:false"`
}
