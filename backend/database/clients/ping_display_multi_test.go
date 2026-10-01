package clients

import (
	"encoding/json"
	"errors"
	"github.com/komari-monitor/komari/database/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"sync"
	"testing"
)

func multiDisplayDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "multi.db")+"?_txlock=immediate&_busy_timeout=5000&_journal_mode=WAL"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Client{}, &models.PingTask{}))
	require.NoError(t, db.Create(&models.Client{UUID: "a", Token: "a"}).Error)
	require.NoError(t, db.Create(&models.PingTask{Id: 1, Clients: models.StringArray{"a"}}).Error)
	require.NoError(t, db.Create(&models.PingTask{Id: 2, Clients: models.StringArray{"a"}}).Error)
	require.NoError(t, db.Create(&models.PingTask{Id: 3, Clients: models.StringArray{"b"}}).Error)
	return db
}
func TestMultiDisplaySelectionValidatesAndPersists(t *testing.T) {
	db := multiDisplayDB(t)
	require.NoError(t, setClientDisplayPingTasks(db, "a", []uint{2, 1}))
	var node models.Client
	require.NoError(t, db.First(&node, "uuid = ?", "a").Error)
	require.Equal(t, []uint{2, 1}, []uint(node.DisplayPingTaskIDs))
	require.Equal(t, uint(2), node.DisplayPingTaskID)
	b, err := json.Marshal(node)
	require.NoError(t, err)
	require.Contains(t, string(b), `"display_ping_task_ids":[2,1]`)
	for _, ids := range [][]uint{{0}, {1, 1}, {1, 3}, {1, 999}} {
		require.Error(t, setClientDisplayPingTasks(db, "a", ids))
		require.NoError(t, db.First(&node, "uuid = ?", "a").Error)
		require.Equal(t, []uint{2, 1}, []uint(node.DisplayPingTaskIDs))
	}
	require.True(t, errors.Is(setClientDisplayPingTasks(db, "a", []uint{3}), ErrDisplayPingTaskNotAssigned))
	require.NoError(t, setClientDisplayPingTask(db, "a", 1))
	require.NoError(t, db.First(&node, "uuid = ?", "a").Error)
	require.Equal(t, []uint{1}, []uint(node.DisplayPingTaskIDs))
	require.NoError(t, setClientDisplayPingTasks(db, "a", []uint{}))
	require.NoError(t, db.First(&node, "uuid = ?", "a").Error)
	require.Empty(t, node.DisplayPingTaskIDs)
	require.Zero(t, node.DisplayPingTaskID)
}
func TestLegacyDisplayFallbackAndMigration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "old.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE clients (uuid TEXT PRIMARY KEY, token TEXT, display_ping_task_id INTEGER NOT NULL DEFAULT 0)").Error)
	require.NoError(t, db.Exec("INSERT INTO clients (uuid, token, display_ping_task_id) VALUES ('old','token',7)").Error)
	require.NoError(t, db.AutoMigrate(&models.Client{}))
	var node models.Client
	require.NoError(t, db.First(&node, "uuid = ?", "old").Error)
	require.Equal(t, uint(7), node.DisplayPingTaskID)
	require.Empty(t, node.DisplayPingTaskIDs)
	legacyJSON, err := json.Marshal(node)
	require.NoError(t, err)
	require.Contains(t, string(legacyJSON), `"display_ping_task_ids":[]`)
	require.NoError(t, db.Create(&models.Client{UUID: "new", Token: "new"}).Error)
	node = models.Client{}
	require.NoError(t, db.First(&node, "uuid = ?", "new").Error)
	require.Empty(t, node.DisplayPingTaskIDs)
}
func TestMultiSelectionAndUnassignmentSerialize(t *testing.T) {
	db := multiDisplayDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(2)
	for i := 0; i < 15; i++ {
		require.NoError(t, db.Model(&models.PingTask{}).Where("id = 1").Update("clients", models.StringArray{"a"}).Error)
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			if e := setClientDisplayPingTasks(db, "a", []uint{1, 2}); e != nil && !errors.Is(e, ErrDisplayPingTaskNotAssigned) {
				errs <- e
			}
		}()
		go func() {
			defer wg.Done()
			errs <- db.Transaction(func(tx *gorm.DB) error {
				if e := tx.Model(&models.PingTask{}).Where("id = 1").Update("clients", models.StringArray{}).Error; e != nil {
					return e
				}
				return RemoveDisplayPingTasksForClients(tx, []string{"a"}, 1)
			})
		}()
		wg.Wait()
		close(errs)
		for e := range errs {
			require.NoError(t, e)
		}
		var node models.Client
		require.NoError(t, db.First(&node, "uuid = ?", "a").Error)
		require.NotContains(t, []uint(node.DisplayPingTaskIDs), uint(1))
		require.NotEqual(t, uint(1), node.DisplayPingTaskID)
	}
}
