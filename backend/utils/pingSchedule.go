package utils

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/corn"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

// PingTaskManager manages timers and tasks
type PingTaskManager struct {
	mu    sync.RWMutex
	tasks map[int][]models.PingTask
}

func IsPingTaskAssigned(taskID uint, clientUUID string) bool {
	if taskID == 0 || clientUUID == "" {
		return false
	}
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	for _, tasks := range manager.tasks {
		for _, task := range tasks {
			if task.Id == taskID && task.AppliesToClient(clientUUID) {
				return true
			}
		}
	}
	return false
}

var manager = &PingTaskManager{
	tasks: make(map[int][]models.PingTask),
}

// Reload reload schedule
func (m *PingTaskManager) Reload(pingTasks []models.PingTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	corn.RemovePrefix("ping:")
	m.tasks = make(map[int][]models.PingTask)

	// Group tasks by Interval
	taskGroups := make(map[int][]models.PingTask)
	for _, task := range pingTasks {
		if task.Interval <= 0 {
			continue
		}
		taskGroups[task.Interval] = append(taskGroups[task.Interval], task)
	}

	// Create coroutine for each unique Interval
	for interval, tasks := range taskGroups {
		interval := interval
		tasks := append([]models.PingTask(nil), tasks...)
		m.tasks[interval] = tasks
		if err := corn.AddContextFunc(fmt.Sprintf("ping:%d", interval), corn.Every(time.Duration(interval)*time.Second), false, func(ctx context.Context) {
			for _, task := range tasks {
				go executePingTask(ctx, task)
			}
		}); err != nil {
			return err
		}
	}
	return nil
}

// executePingTask executes a single PingTask
func executePingTask(ctx context.Context, task models.PingTask) {
	var message struct {
		TaskID  uint   `json:"ping_task_id"`
		Message string `json:"message"`
		Type    string `json:"ping_type"`
		Target  string `json:"ping_target"`
	}

	message.Message = "ping"
	message.TaskID = task.Id
	message.Type = task.Type
	message.Target = task.Target

	for _, clientUUID := range targetPingClientUUIDs(task) {
		select {
		case <-ctx.Done():
			// Context was canceled, stop sending pings.
			return
		default:
			// Context is still active, continue.
		}

		agent_runtime.DispatchPing(clientUUID, message, v2.PingParams{TaskID: task.Id, Type: task.Type, Target: task.Target})
	}
}

// targetPingClientUUIDs calculates the list of online servers that need to be delivered for this schedule based on the task configuration.
func targetPingClientUUIDs(task models.PingTask) []string {
	return task.Clients
}

// ReloadPingSchedule loads or reloads the schedule
func ReloadPingSchedule(pingTasks []models.PingTask) error {
	return manager.Reload(pingTasks)
}
