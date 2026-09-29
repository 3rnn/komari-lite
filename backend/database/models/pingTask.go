package models

import "time"

type PingRecord struct {
	Client     string    `json:"client" gorm:"type:varchar(36);not null;index"`
	ClientInfo Client    `json:"client_info" gorm:"foreignKey:Client;references:UUID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	TaskId     uint      `json:"task_id" gorm:"not null;index"`
	Task       PingTask  `json:"task" gorm:"foreignKey:TaskId;references:Id;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;"`
	Time       time.Time `json:"time" gorm:"index;not null"`
	Value      int       `json:"value" gorm:"type:int;not null"` // Ping value in milliseconds
}

// PingTask represents a delay monitoring task configuration.
type PingTask struct {
	Id        uint        `json:"id,omitempty" gorm:"primaryKey;autoIncrement"`
	Weight    int         `json:"weight" gorm:"type:int;not null;default:0;index"`
	Name      string      `json:"name" gorm:"type:varchar(255);not null;index"`
	Clients   StringArray `json:"clients" gorm:"type:longtext"`
	DefaultOn bool        `json:"default_on" gorm:"column:all_clients;not null;default:false"` // Whether newly added servers automatically turn on this monitoring; existing servers are not affected by this field
	Type      string      `json:"type" gorm:"type:varchar(12);not null;default:'icmp'"`        // icmp tcp http
	Target    string      `json:"target" gorm:"type:varchar(255);not null"`                    // Ping target address
	Interval  int         `json:"interval" gorm:"type:int;not null;default:60"`                // Interval time
}

// AppliesToClient determines whether the current PingTask is applicable to the specified server.
func (task PingTask) AppliesToClient(uuid string) bool {
	if uuid == "" {
		return false
	}
	for _, client := range task.Clients {
		if client == uuid {
			return true
		}
	}
	return false
}
