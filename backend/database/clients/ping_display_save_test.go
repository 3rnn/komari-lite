package clients

import (
	"github.com/komari-monitor/komari/database/models"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGeneralClientSaveCannotChangeDisplaySelections(t *testing.T) {
	db := multiDisplayDB(t)
	require.NoError(t, setClientDisplayPingTasks(db, "a", []uint{1, 2}))
	err := saveClient(db, map[string]interface{}{"uuid": "a", "name": "change", "display_ping_task_ids": []uint{3}})
	require.Error(t, err)
	var node models.Client
	require.NoError(t, db.First(&node, "uuid = ?", "a").Error)
	require.Equal(t, models.UintArray{1, 2}, node.DisplayPingTaskIDs)
}

func TestGeneralClientSaveRejectsCaseInsensitiveDisplayColumns(t *testing.T) {
	for _, tc := range []struct {
		field string
		value interface{}
	}{
		{"DISPLAY_PING_TASK_IDS", models.UintArray{3}},
		{"dIsPlAy_PiNg_TaSk_Id", uint(3)},
	} {
		t.Run(tc.field, func(t *testing.T) {
			db := multiDisplayDB(t)
			require.NoError(t, setClientDisplayPingTasks(db, "a", []uint{1, 2}))
			err := saveClient(db, map[string]interface{}{"uuid": "a", "name": "change", tc.field: tc.value})
			require.Error(t, err)
			var node models.Client
			require.NoError(t, db.First(&node, "uuid = ?", "a").Error)
			require.Equal(t, models.UintArray{1, 2}, node.DisplayPingTaskIDs)
			require.Equal(t, uint(1), node.DisplayPingTaskID)
		})
	}
}
