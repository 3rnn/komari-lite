package clients

import (
	"errors"
	"fmt"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/database/pingdisplay"
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
	if taskID == 0 {
		return setClientDisplayPingTasks(db, uuid, nil)
	}
	return setClientDisplayPingTasks(db, uuid, []uint{taskID})
}

// SetClientDisplayPingTasks saves ordered assigned tasks for public display.
// An empty selection restores theme defaults without changing probe assignments.
func SetClientDisplayPingTasks(uuid string, taskIDs []uint) error {
	return setClientDisplayPingTasks(dbcore.GetDBInstance(), uuid, taskIDs)
}

func setClientDisplayPingTasks(db *gorm.DB, uuid string, taskIDs []uint) error {
	if uuid == "" {
		return fmt.Errorf("client UUID is required")
	}
	seen := make(map[uint]struct{}, len(taskIDs))
	for _, id := range taskIDs {
		if id == 0 {
			return fmt.Errorf("ping task ID must be nonzero")
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("duplicate ping task ID: %d", id)
		}
		seen[id] = struct{}{}
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var client models.Client
		if err := tx.Select("uuid").First(&client, "uuid = ?", uuid).Error; err != nil {
			return err
		}
		for _, taskID := range taskIDs {
			var task models.PingTask
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&task, "id = ?", taskID).Error; err != nil {
				return err
			}
			if !task.AppliesToClient(uuid) {
				return ErrDisplayPingTaskNotAssigned
			}
		}
		var first uint
		if len(taskIDs) > 0 {
			first = taskIDs[0]
		}
		return tx.Model(&models.Client{}).Where("uuid = ?", uuid).
			Updates(map[string]interface{}{"display_ping_task_id": first, "display_ping_task_ids": models.UintArray(taskIDs)}).Error
	})
}

// RemoveDisplayPingTasksForClients removes one unassigned display selection.
func RemoveDisplayPingTasksForClients(tx *gorm.DB, uuids []string, taskID uint) error {
	return pingdisplay.RemoveDisplayPingTasksForClients(tx, uuids, taskID)
}
