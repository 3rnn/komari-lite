package jsonrpc

import (
	"context"
	"time"

	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/database/tasks"
	"github.com/komari-monitor/komari/pkg/rpc"
)

// client.go
// Agent-oriented RPC2 methods (client namespace, client/admin callable).
// These methods rely on the meta.ClientUUID to identify the caller client.

func init() {
	regClient("getPingTasks", clientGetPingTasks, "Get ping tasks assigned to the calling client")
	regClient("uploadPingResult", clientUploadPingResult, "Upload a ping result")
}

func regClient(name string, h rpc.Handler, summary string) {
	RegisterWithGroupAndMeta(name, rpc.RoleClient, h, &rpc.MethodMeta{Name: "client:" + name, Summary: summary})
}

// callingClientUUID returns the UUID of the calling client.
func callingClientUUID(ctx context.Context) string {
	if meta := rpc.MetaFromContext(ctx); meta != nil {
		return meta.ClientUUID
	}
	return ""
}

func clientGetPingTasks(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	uuid := callingClientUUID(ctx)
	if uuid == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "client_uuid not found", nil)
	}
	return tasks.GetPingTasksByClient(uuid), nil
}

func clientUploadPingResult(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	uuid := callingClientUUID(ctx)
	if uuid == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "client_uuid not found", nil)
	}
	var params struct {
		TaskID   uint   `json:"task_id"`
		Value    int    `json:"value"`
		PingType string `json:"ping_type"`
	}
	if err := req.BindParams(&params); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid request: "+err.Error(), nil)
	}
	record := models.PingRecord{
		Client: uuid,
		TaskId: params.TaskID,
		Value:  params.Value,
		Time:   time.Now().UTC(),
	}
	if err := tasks.SavePingRecord(record); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to save ping result: "+err.Error(), nil)
	}
	return map[string]any{"status": "success"}, nil
}
