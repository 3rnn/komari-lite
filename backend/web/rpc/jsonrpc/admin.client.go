package jsonrpc

import (
	"context"
	"errors"
	"time"

	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/metricstore"
	d_notification "github.com/komari-monitor/komari/database/notification"
	"github.com/komari-monitor/komari/database/records"
	"github.com/komari-monitor/komari/pkg/rpc"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	logger "github.com/komari-monitor/komari/utils/log"
	"github.com/komari-monitor/komari/utils/notifier"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
	"gorm.io/gorm"
)

// admin.client.go
// Client RPC2 methods (admin namespace) contain the business logic formerly in web/api/admin/client.go,
// including audit logs and runtime effects. Traditional REST handlers invoke them through CallFromGin.

func init() {
	RegisterWithGroupAndMeta("addClient", rpc.RoleAdmin, adminAddClient, &rpc.MethodMeta{
		Name:    "admin:addClient",
		Summary: "Create a new client",
		Params: []rpc.ParamMeta{
			{Name: "name", Type: "string", Required: false, Description: "Optional client name"},
		},
		Returns: "{ uuid: string, token: string }",
	})
	RegisterWithGroupAndMeta("editClient", rpc.RoleAdmin, adminEditClient, &rpc.MethodMeta{
		Name:    "admin:editClient",
		Summary: "Edit a client (partial update)",
		Params: []rpc.ParamMeta{
			{Name: "uuid", Type: "string", Required: true, Description: "Client UUID"},
		},
		Returns: "null",
	})
	RegisterWithGroupAndMeta("setClientDisplayPingTask", rpc.RoleAdmin, adminSetClientDisplayPingTask, &rpc.MethodMeta{
		Name:    "admin:setClientDisplayPingTask",
		Summary: "Choose an assigned ping task for a server's public node card",
		Returns: "{ display_ping_task_id: uint }",
	})
	RegisterWithGroupAndMeta("removeClient", rpc.RoleAdmin, adminRemoveClient, &rpc.MethodMeta{
		Name:    "admin:removeClient",
		Summary: "Delete a client",
		Params: []rpc.ParamMeta{
			{Name: "uuid", Type: "string", Required: true, Description: "Client UUID"},
		},
		Returns: "null",
	})
	RegisterWithGroupAndMeta("getClient", rpc.RoleAdmin, adminGetClient, &rpc.MethodMeta{
		Name:    "admin:getClient",
		Summary: "Get a client by UUID",
		Params: []rpc.ParamMeta{
			{Name: "uuid", Type: "string", Required: true, Description: "Client UUID"},
		},
		Returns: "Client",
	})
	RegisterWithGroupAndMeta("listClients", rpc.RoleAdmin, adminListClients, &rpc.MethodMeta{
		Name:    "admin:listClients",
		Summary: "List all clients (basic info)",
		Returns: "Client[]",
	})
	RegisterWithGroupAndMeta("getClientToken", rpc.RoleAdmin, adminGetClientToken, &rpc.MethodMeta{
		Name:    "admin:getClientToken",
		Summary: "Get a client's token by UUID",
		Params: []rpc.ParamMeta{
			{Name: "uuid", Type: "string", Required: true, Description: "Client UUID"},
		},
		Returns: "{ token: string }",
	})
	RegisterWithGroupAndMeta("rotateClientToken", rpc.RoleAdmin, adminRotateClientToken, &rpc.MethodMeta{
		Name:    "admin:rotateClientToken",
		Summary: "Rotate a client token with a transition period",
		Params: []rpc.ParamMeta{
			{Name: "uuid", Type: "string", Required: true, Description: "Client UUID"},
		},
		Returns: "{ token: string, previous_token_expires_at: string }",
	})
	RegisterWithGroupAndMeta("clearRecords", rpc.RoleAdmin, adminClearRecords, &rpc.MethodMeta{
		Name:    "admin:clearRecords",
		Summary: "Delete all load records",
		Returns: "null",
	})
}

// auditActor extracts the actor UUID and source IP from the context for auditing.
func auditActor(ctx context.Context) (uuid, ip string) {
	if meta := rpc.MetaFromContext(ctx); meta != nil {
		uuid = meta.UserUUID
		ip = meta.RemoteIP
	}
	return uuid, ip
}

func adminAddClient(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		Name string `json:"name"`
	}
	req.BindParams(&params)

	var (
		uuid, token string
		err         error
	)
	if params.Name == "" {
		uuid, token, err = clients.CreateClient()
	} else {
		uuid, token, err = clients.CreateClientWithName(params.Name)
	}
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	if err := d_notification.AddDefaultOnClientUUID(uuid); err != nil {
		logger.ErrorArgs("clients", "Failed to apply default-on load notifications to new client:", err)
	}
	if params.Name != "" {
		actor, ip := auditActor(ctx)
		auditlog.Log(ip, actor, "create client:"+uuid, "info")
	}
	return map[string]any{"uuid": uuid, "token": token}, nil
}

func adminEditClient(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var update map[string]interface{}
	if err := req.BindParams(&update); err != nil || update == nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid params", nil)
	}
	uuid, _ := update["uuid"].(string)
	if uuid == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid or missing UUID", nil)
	}
	for _, key := range []string{"display_ping_task_id", "DisplayPingTaskID", "displayPingTaskId"} {
		if _, present := update[key]; present {
			return nil, rpc.MakeError(rpc.InvalidParams, "Use the public display task selector", nil)
		}
	}
	if err := clients.SaveClient(update); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	if _, changed := update["traffic_reset_day"]; changed {
		if clientInfo, err := clients.GetClientByUUID(uuid); err == nil && clientInfo.TrafficResetDay != nil {
			monthRotate := *clientInfo.TrafficResetDay
			agent_runtime.DispatchV2Event(uuid, v2.MethodAgentConfig, v2.ConfigParams{
				MonthRotate: &monthRotate,
			})
		}
	}
	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "edit client:"+uuid, "info")
	return nil, nil
}

func adminSetClientDisplayPingTask(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		UUID   string `json:"uuid"`
		TaskID *uint  `json:"task_id"`
	}
	if err := req.BindParams(&params); err != nil || params.UUID == "" || params.TaskID == nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Client UUID and task_id are required", nil)
	}
	if err := clients.SetClientDisplayPingTask(params.UUID, *params.TaskID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, rpc.MakeError(rpc.InvalidParams, "Server or ping task not found", nil)
		}
		if errors.Is(err, clients.ErrDisplayPingTaskNotAssigned) {
			return nil, rpc.MakeError(rpc.InvalidParams, err.Error(), nil)
		}
		return nil, rpc.MakeError(rpc.InternalError, "Failed to save public display task", nil)
	}
	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "set public ping task:"+params.UUID, "info")
	return map[string]any{"display_ping_task_id": *params.TaskID}, nil
}

func adminRemoveClient(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		UUID string `json:"uuid"`
	}
	req.BindParams(&params)
	if params.UUID == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid or missing UUID", nil)
	}
	metricstore.BlockEntityWrites(params.UUID)
	agent_runtime.DeleteConnectedClients(params.UUID)
	notifier.ForgetClient(params.UUID)
	if err := clients.DeleteClient(params.UUID); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to delete client: "+err.Error(), nil)
	}
	agent_runtime.DeleteConnectedClients(params.UUID)
	if err := d_notification.ReloadLoadNotificationSchedule(); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Client deleted but failed to reload load notification schedule: "+err.Error(), nil)
	}
	notifier.ForgetClient(params.UUID)
	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "delete client:"+params.UUID, "warn")
	return nil, nil
}

func adminGetClient(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		UUID string `json:"uuid"`
	}
	req.BindParams(&params)
	if params.UUID == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid or missing UUID", nil)
	}
	result, err := clients.GetClientByUUID(params.UUID)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	return result, nil
}

func adminListClients(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	cls, err := clients.GetAllClientBasicInfo()
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	return cls, nil
}

func adminGetClientToken(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		UUID string `json:"uuid"`
	}
	req.BindParams(&params)
	if params.UUID == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid or missing UUID", nil)
	}
	token, err := clients.GetClientTokenByUUID(params.UUID)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	return map[string]any{"token": token}, nil
}

func adminRotateClientToken(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		UUID string `json:"uuid"`
	}
	req.BindParams(&params)
	if params.UUID == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid or missing UUID", nil)
	}
	token, expiresAt, err := clients.RotateClientToken(params.UUID, 24*time.Hour)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "rotate client token:"+params.UUID, "warn")
	return map[string]any{"token": token, "previous_token_expires_at": expiresAt}, nil
}

func adminClearRecords(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	if err := records.DeleteAll(); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to delete Record"+err.Error(), nil)
	}
	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "clear records", "warn")
	return nil, nil
}
