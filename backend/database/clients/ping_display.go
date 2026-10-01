package clients

import (
	"errors"
	"fmt"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrDisplayPingTaskNotAssigned = errors.New("selected ping task is not assigned to this server")

// SetClientDisplayPingTask chooses the one assigned task shown on a public node card.
// Zero restores the theme's default selection without changing Agent probe assignments.
func SetClientDisplayPingTask(uuid string, taskID uint) error {
	return setClientDisplayPingTask(dbcore.GetDBInstance(), uuid, taskID)
}

func setClientDisplayPingTask(db *gorm.DB, uuid string, taskID uint) error {
	if uuid == "" {
		return fmt.Errorf("client UUID is required")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var client models.Client
		if err := tx.Select("uuid").First(&client, "uuid = ?", uuid).Error; err != nil {
			return err
		}
		if taskID != 0 {
			var task models.PingTask
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&task, "id = ?", taskID).Error; err != nil {
				return err
			}
			if !task.AppliesToClient(uuid) {
				return ErrDisplayPingTaskNotAssigned
			}
		}
		return tx.Model(&models.Client{}).Where("uuid = ?", uuid).
			Update("display_ping_task_id", taskID).Error
	})
}
