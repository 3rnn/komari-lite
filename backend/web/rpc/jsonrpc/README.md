# RPC2 Internal Architecture

This directory contains the server-side implementation of Komari's frontend-facing RPC (JSON-RPC 2.0). All external interfaces ultimately use one pipeline:

```
External interfaces (REST / WebSocket / internal calls)
        │  Construct RPC request
        ▼
   Dispatch (private-site check → namespace authorization)
        │
        ▼
   rpc.CallWithContext → registered RPC handler (business logic)
```

## Layers

| File | Responsibility |
| --- | --- |
| `pkg/rpc/*` | Protocol core: request/response types, registry, invocation (`Call`/`Invoke`), and permissions (`permission.go`) |
| `transport.go` | HTTP POST and WebSocket transport for `/api/rpc2`, identity resolution, and `CallFromGin` |
| `dispatch.go` | Unified `Dispatch` entry point and `OnInternalRequest` for internal calls |
| `register.go` | Registration helpers `Register` / `RegisterWithGroupAndMeta` |
| `common*.go` | Business methods in the `common` namespace |
| `admin.*.go` | Business methods in the `admin` namespace |

## Namespaces and Permissions (Declarative ACL with Wildcards)

Method names use `namespace:method`. Names without `:` belong to the default `common` namespace; internal methods use the `rpc.` prefix.

Permissions use a **declarative ACL**: rules of the form `(pattern, minRole)`. Patterns support `*` wildcards and match the complete method name.
When multiple rules match, the most specific wins: exact match > wildcard with a longer literal prefix > global `*`.
If specificity is equal, the stricter (higher) role wins.

Role levels: `guest (0) < client (1) < admin (2)`.

Built-in defaults (see `init` in `pkg/rpc/permission.go`):

| Rule pattern | Required role |
| --- | --- |
| `*` (fallback) | admin |
| `common:*` / `guest:*` / `rpc.*` / `rpc:*` | guest |
| `client:*` | client |
| `admin:*` | admin |

Declare custom permissions (for plugins):

```go
rpc.Allow("plugin:*", rpc.RoleClient)         // Entire namespace
rpc.Allow("plugin:publicStat", rpc.RoleGuest) // A more specific method rule may relax access
rpc.RegisterNamespace("plugin", rpc.RoleAdmin) // Equivalent to Allow("plugin:*", admin)
```

Because specificity decides precedence, `plugin:*`=admin and `plugin:publicStat`=guest can coexist: guests can call `publicStat`,
while the remaining `plugin:*` methods still require admin privileges.

## Registering an RPC Method

```go
func init() {
    jsonrpc.RegisterWithGroupAndMeta("addClient", rpc.RoleAdmin, adminAddClient,
        &rpc.MethodMeta{
            Name:    "admin:addClient",
            Summary: "Create a new client",
            Params:  []rpc.ParamMeta{{Name: "name", Type: "string"}},
            Returns: "{ uuid, token }",
        })
}

func adminAddClient(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
    var params struct{ Name string `json:"name"` }
    req.BindParams(&params)
    // ... business logic
    return result, nil
}
```

- A handler returns `(result, nil)` on success or `(nil, *rpc.JsonRpcError)` on failure.
- `rpc.MetaFromContext(ctx)` provides the caller's identity (role, user UUID, client token, source IP, etc.).
- `req.BindParams(&struct)` binds named object or positional array parameters.

## Calling RPC from a Traditional REST Handler

During migration, gin handlers can use `jsonrpc.CallFromGin` to reuse the same authorization and audit context:

```go
func GetClient(c *gin.Context) {
    resp := jsonRpc.CallFromGin(c, "admin:getClient", map[string]any{"uuid": c.Param("uuid")})
    if resp.Error != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": resp.Error.Message})
        return
    }
    c.JSON(http.StatusOK, resp.Result)
}
```

The handler becomes a thin adapter: parse gin parameters → call RPC → map the response back to the original REST JSON shape, preserving the frontend contract.

## Plugin Extension Points (Infrastructure in Place; Plugins Not Yet Implemented)

`pkg/rpc` already provides APIs for future plugin runtimes:

- `rpc.Register(method, handler)` / `rpc.MustRegister` — register a method (`rpc.` is reserved; duplicate registration fails).
- `rpc.Unregister(method) bool` — unregister a method and remove its metadata (for plugin unloading); reserved prefixes cannot be unregistered.
- `rpc.Allow(pattern, minRole)` — declare a wildcard-capable ACL rule.
- `rpc.RegisterNamespace(namespace, requiredRole)` — declare the required role for an entire namespace.

Plugins can invoke existing methods through `rpc.Invoke`, register their own namespaced methods, and declare permissions with `Allow`, reusing the same authorization and dispatch pipeline.

## Migrated Interfaces (REST → RPC2)

Nearly all JSON interfaces are now RPC2 methods, directly attached to router paths through the declarative `Bind` bridge.
There is no longer a per-resource gin handler layer for these methods.

| Namespace | Methods (files) |
| --- | --- |
| `admin` | Client CRUD and deployment; ping tasks; sessions/settings/weights; notifications (load/offline/traffic); providers (messageSender/oidc); dashboard (settings/alerts/cache/latency/packet loss); metrics; database; preferences; system (logs/tests) |
| `public` | getMe, getNodesInformation, getPublicSettings, getVersion, getClientRecentRecords, getRecordsByUUID, getPingRecords, getPublicPingTasks, recordVisitorEvent |
| `client` | getPingTasks, uploadPingResult, taskResult |

### Declarative Route Bridge `Bind`

`web/rpc/jsonrpc/bridge.go` provides:

```go
r.GET("/api/admin/client/:uuid", jsonRpc.Bind("admin:getClient", jsonRpc.WithPath("uuid"), jsonRpc.WithRaw()))
```

- Parameter assembly: merge a JSON body (object/array), `WithPath(...)` path parameters, and `WithQuery(...)` query parameters into RPC parameters.
- Response renderers (preserving the API contract):
  - Default `renderStandard` → `{status:"success", message, data}` (omit `data` when empty, matching `api.Response`).
  - `WithFlat()` → spread the result map into the top level plus `{status:"success"}` (addClient/getClientToken/getSessions/provider set).
  - `WithRaw()` → return the result directly (agent raw JSON / me / listClients / getClient).
  - `WithMessage(msg)` → include a fixed success message (xtermjs save).
- Errors use `{status:"error", message}`, with JSON-RPC errors mapped to HTTP status codes.

### Interfaces Remaining as REST (Not Through the RPC Bridge)

Binary, streaming, redirect, and special-authorization interfaces remain in `web/api/admin` (2fa/theme settings/backup/update/oauth/https/traffic calibration/download),
`web/api/public` (login/logout/oauth/mjpeg, agent distribution and installers), and `web/api/client` (report WS+POST, v2 RPC, uploadBasicInfo, runtime config, AutoDiscovery registration).

The core agent v1/v2 report-ingestion logic is now shared in `web/api/client/ingest.go`.
