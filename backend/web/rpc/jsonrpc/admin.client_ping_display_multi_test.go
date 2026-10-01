package jsonrpc

import (
	"context"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPublicMultiDisplayWriteRequiresExplicitArray(t *testing.T) {
	for _, params := range []map[string]any{
		{"uuid": "node-a"},
		{"uuid": "node-a", "task_ids": nil},
		{"uuid": "node-a", "task_ids": "1"},
		{"uuid": "node-a", "task_ids": []int{-1}},
		{"task_ids": []uint{}},
	} {
		req := rpc.NewRequest(1, "admin:setClientDisplayPingTasks", params)
		_, rpcErr := adminSetClientDisplayPingTasks(context.Background(), req)
		require.NotNil(t, rpcErr)
		require.Equal(t, rpc.InvalidParams, rpcErr.Code)
	}
}
