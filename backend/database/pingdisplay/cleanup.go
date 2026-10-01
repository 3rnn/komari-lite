package pingdisplay

import (
	"github.com/komari-monitor/komari/database/models"
	"gorm.io/gorm"
)

// RemoveDisplayPingTasksForClients drops only the unassigned task, preserving
// other selected tasks and the legacy scalar's first-selected compatibility.
// The caller must hold the same write transaction as the assignment change.
func RemoveDisplayPingTasksForClients(tx *gorm.DB, uuids []string, taskID uint) error {
	if len(uuids) == 0 {
		return nil
	}
	var nodes []models.Client
	if err := tx.Select("uuid", "display_ping_task_id", "display_ping_task_ids").Where("uuid IN ?", uuids).Find(&nodes).Error; err != nil {
		return err
	}
	for _, node := range nodes {
		remaining := make(models.UintArray, 0, len(node.DisplayPingTaskIDs))
		for _, id := range node.DisplayPingTaskIDs {
			if id != taskID {
				remaining = append(remaining, id)
			}
		}
		if len(remaining) == len(node.DisplayPingTaskIDs) && node.DisplayPingTaskID != taskID {
			continue
		}
		var first uint
		if len(remaining) > 0 {
			first = remaining[0]
		}
		if err := tx.Model(&models.Client{}).Where("uuid = ?", node.UUID).
			Updates(map[string]interface{}{"display_ping_task_id": first, "display_ping_task_ids": remaining}).Error; err != nil {
			return err
		}
	}
	return nil
}

// RemoveDisplayPingTasksForDeletedTasks removes deleted IDs from every client.
func RemoveDisplayPingTasksForDeletedTasks(tx *gorm.DB, ids []uint) error {
	var nodes []models.Client
	if err := tx.Select("uuid", "display_ping_task_id", "display_ping_task_ids").Find(&nodes).Error; err != nil {
		return err
	}
	removed := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		removed[id] = struct{}{}
	}
	for _, node := range nodes {
		remaining := make(models.UintArray, 0, len(node.DisplayPingTaskIDs))
		for _, id := range node.DisplayPingTaskIDs {
			if _, exists := removed[id]; !exists {
				remaining = append(remaining, id)
			}
		}
		_, scalarRemoved := removed[node.DisplayPingTaskID]
		if len(remaining) == len(node.DisplayPingTaskIDs) && !scalarRemoved {
			continue
		}
		var first uint
		if len(remaining) > 0 {
			first = remaining[0]
		}
		if err := tx.Model(&models.Client{}).Where("uuid = ?", node.UUID).
			Updates(map[string]interface{}{"display_ping_task_id": first, "display_ping_task_ids": remaining}).Error; err != nil {
			return err
		}
	}
	return nil
}
