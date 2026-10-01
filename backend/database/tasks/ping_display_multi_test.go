package tasks

import (
	"github.com/komari-monitor/komari/database/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

func multiDisplayTaskDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "multi.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Client{}, &models.PingTask{}, &models.MetricCleanupJob{}, &models.PingLossNotification{}))
	require.NoError(t, db.Create(&models.Client{UUID: "a", Token: "a", DisplayPingTaskID: 1, DisplayPingTaskIDs: models.UintArray{1, 2}}).Error)
	require.NoError(t, db.Create([]models.PingTask{{Id: 1, Clients: models.StringArray{"a"}}, {Id: 2, Clients: models.StringArray{"a"}}}).Error)
	return db
}
func TestDeletingFirstSelectedTaskPreservesRemaining(t *testing.T) {
	db := multiDisplayTaskDB(t)
	require.NoError(t, deletePingTaskRows(db, []uint{1}))
	var node models.Client
	require.NoError(t, db.First(&node, "uuid = ?", "a").Error)
	require.Equal(t, models.UintArray{2}, node.DisplayPingTaskIDs)
	require.Equal(t, uint(2), node.DisplayPingTaskID)
}
func TestUnassigningFirstSelectedTaskPreservesRemaining(t *testing.T) {
	db := multiDisplayTaskDB(t)
	_, err := editPingTasks(db, []*models.PingTask{{Id: 1, Clients: models.StringArray{}}})
	require.NoError(t, err)
	var node models.Client
	require.NoError(t, db.First(&node, "uuid = ?", "a").Error)
	require.Equal(t, models.UintArray{2}, node.DisplayPingTaskIDs)
	require.Equal(t, uint(2), node.DisplayPingTaskID)
}
