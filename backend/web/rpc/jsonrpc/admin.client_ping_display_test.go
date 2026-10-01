package jsonrpc

import (
	"context"
	"testing"

	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/stretchr/testify/require"
)

func TestGenericClientEditCannotBypassPingDisplayValidation(t *testing.T) {
	for _, field := range []string{"display_ping_task_id", "DisplayPingTaskID", "displayPingTaskId", "display_ping_task_ids", "DisplayPingTaskIDs", "displayPingTaskIds", "DISPLAY_PING_TASK_IDS", "dIsPlAy_PiNg_TaSk_Id"} {
		t.Run(field, func(t *testing.T) {
			req := rpc.NewRequest(1, "admin:editClient", map[string]any{"uuid": "node-a", field: 999})
			_, rpcErr := adminEditClient(context.Background(), req)
			require.NotNil(t, rpcErr)
			require.Equal(t, rpc.InvalidParams, rpcErr.Code)
		})
	}
}

func TestPublicDisplayWriteRequiresExplicitTaskID(t *testing.T) {
	for _, params := range []map[string]any{
		{"uuid": "node-a"},
		{"uuid": "node-a", "task_id": -1},
		{"task_id": 0},
	} {
		req := rpc.NewRequest(1, "admin:setClientDisplayPingTask", params)
		_, rpcErr := adminSetClientDisplayPingTask(context.Background(), req)
		require.NotNil(t, rpcErr)
		require.Equal(t, rpc.InvalidParams, rpcErr.Code)
	}
}
