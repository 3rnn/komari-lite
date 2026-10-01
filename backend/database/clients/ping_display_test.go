package clients

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/komari-monitor/komari/database/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSetClientDisplayPingTaskRequiresAssignedTask(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "ping-display.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Client{}, &models.PingTask{}))
	require.NoError(t, db.Create(&models.Client{UUID: "node-a", Token: "token-a"}).Error)
	require.NoError(t, db.Create(&models.PingTask{Id: 7, Name: "CT", Type: "icmp", Target: "1.1.1.1", Clients: models.StringArray{"node-a"}}).Error)
	require.NoError(t, db.Create(&models.PingTask{Id: 8, Name: "CU", Type: "icmp", Target: "8.8.8.8", Clients: models.StringArray{"node-b"}}).Error)
	seenTaskRead, lockedTaskRead := false, false
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:display_task_lock", func(tx *gorm.DB) {
		if tx.Statement.Table == "ping_tasks" {
			seenTaskRead = true
			_, lockedTaskRead = tx.Statement.Clauses["FOR"]
		}
	}))

	require.NoError(t, setClientDisplayPingTask(db, "node-a", 7))
	require.True(t, seenTaskRead, "selection must read the assigned task")
	require.True(t, lockedTaskRead, "selection must lock the task until the client write commits")
	var node models.Client
	require.NoError(t, db.First(&node, "uuid = ?", "node-a").Error)
	require.Equal(t, uint(7), node.DisplayPingTaskID)

	require.True(t, errors.Is(setClientDisplayPingTask(db, "node-a", 8), ErrDisplayPingTaskNotAssigned))
	require.Error(t, setClientDisplayPingTask(db, "node-a", 999))
	require.Error(t, setClientDisplayPingTask(db, "missing-node", 0))
	require.NoError(t, db.First(&node, "uuid = ?", "node-a").Error)
	require.Equal(t, uint(7), node.DisplayPingTaskID)

	require.NoError(t, setClientDisplayPingTask(db, "node-a", 0))
	require.NoError(t, db.First(&node, "uuid = ?", "node-a").Error)
	require.Zero(t, node.DisplayPingTaskID)
}

func TestAgentCannotOverwritePublicPingSelection(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "ping-display-agent.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Client{}))
	require.NoError(t, db.Create(&models.Client{UUID: "node-a", Token: "token-a", DisplayPingTaskID: 7}).Error)
	require.NoError(t, saveClientInfo(db, map[string]interface{}{
		"uuid": "node-a", "os": "Linux", "display_ping_task_id": 99,
	}))
	var node models.Client
	require.NoError(t, db.First(&node, "uuid = ?", "node-a").Error)
	require.Equal(t, uint(7), node.DisplayPingTaskID)
	require.Equal(t, "Linux", node.OS)
}

func TestSelectionAndUnassignmentNeverLeaveStaleDisplayUnderSQLite(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "race.db") + "?_txlock=immediate&_busy_timeout=5000&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1) // Match the production SQLite pool.
	require.NoError(t, db.AutoMigrate(&models.Client{}, &models.PingTask{}))
	require.NoError(t, db.Create(&models.Client{UUID: "node-a", Token: "token-a"}).Error)
	require.NoError(t, db.Create(&models.PingTask{Id: 7, Name: "CT", Clients: models.StringArray{"node-a"}}).Error)

	for i := 0; i < 30; i++ {
		require.NoError(t, db.Model(&models.PingTask{}).Where("id = 7").Update("clients", models.StringArray{"node-a"}).Error)
		require.NoError(t, db.Model(&models.Client{}).Where("uuid = 'node-a'").Update("display_ping_task_id", 0).Error)
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := setClientDisplayPingTask(db, "node-a", 7); err != nil && !errors.Is(err, ErrDisplayPingTaskNotAssigned) {
				errs <- err
			}
		}()
		go func() {
			defer wg.Done()
			errs <- db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Model(&models.PingTask{}).Where("id = 7").Update("clients", models.StringArray{}).Error; err != nil {
					return err
				}
				return tx.Model(&models.Client{}).Where("uuid = 'node-a' AND display_ping_task_id = 7").Update("display_ping_task_id", 0).Error
			})
		}()
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		var client models.Client
		require.NoError(t, db.First(&client, "uuid = 'node-a'").Error)
		require.Zero(t, client.DisplayPingTaskID)
	}
}

func TestExistingClientMigratesToAutomaticPublicDisplay(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "legacy.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE clients (uuid TEXT PRIMARY KEY, token TEXT)").Error)
	require.NoError(t, db.Exec("INSERT INTO clients (uuid, token) VALUES (?, ?)", "legacy-node", "legacy-token").Error)
	require.NoError(t, db.AutoMigrate(&models.Client{}))
	var node models.Client
	require.NoError(t, db.First(&node, "uuid = ?", "legacy-node").Error)
	require.Zero(t, node.DisplayPingTaskID)
	require.NoError(t, db.Create(&models.Client{UUID: "new-node", Token: "new-token"}).Error)
	node = models.Client{}
	require.NoError(t, db.First(&node, "uuid = ?", "new-node").Error)
	require.Zero(t, node.DisplayPingTaskID)
}
