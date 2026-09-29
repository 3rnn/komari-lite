# Visitor Audit RPC Guide

This document explains how to call the `public:recordVisitorEvent` RPC2 method. Themes can use it to write visitor activity and frontend actions to the Komari backend audit log.

## Overview

- RPC method: `public:recordVisitorEvent`
- RPC path: `/api/rpc2`
- HTTP method: `POST`
- Storage: existing audit log table `models.Log`
- Admin access: existing `admin:getLogs` method
- Log type: `msg_type = "visitor"`
- Default: disabled; set `visitor_audit_enabled = true` to enable it

The method does not trust an IP supplied by the frontend. The backend records the source IP and User-Agent from the request context.

## Permissions

`public:recordVisitorEvent` belongs to the `public` namespace and can be called by guests.

It is also on the login-page allowlist in private-site mode so pre-login and public-facing activity can be recorded.

This is intentional: when the feature is disabled, the method remains callable but writes nothing to the database.

## Enablement

Anonymous log writes are disabled by default for both existing and new installations. An administrator can use
`admin:editSettings` to set `visitor_audit_enabled` to `true`. The public setting returned by
`public:getPublicSettings` includes this field so themes can decide whether to send events.

When disabled, the method returns `status = "disabled"`. When enabled, an in-memory token bucket limits each source IP:
30 requests per minute on average, with a burst of 10. Exceeding the limit returns `status = "rate_limited"` without writing to the database.

## Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `event` | `string` | Yes | Event name, such as `page_view`, `node_open`, or `search` |
| `action` | `string` | No | Alias for `event` |
| `operation` | `string` | No | Alias for `event` |
| `path` | `string` | No | Frontend path, such as `/` or `/instance/<uuid>` |
| `route` | `string` | No | Frontend route name, such as `home` or `instance-detail` |
| `target` | `string` | No | Action target, such as a node UUID, tool key, or button key |
| `detail` | `object` | No | Additional metadata; size-limited |

Supply any one of `event`, `action`, or `operation`, in this precedence order:

```text
event > action > operation
```

## Information Recorded Automatically by the Backend

| Field | Source |
| --- | --- |
| IP | `rpc.ContextMeta.RemoteIP`, from server-side `c.ClientIP()` |
| User-Agent | `rpc.ContextMeta.UserAgent`, from the request header |
| User UUID | `rpc.ContextMeta.UserUUID` when logged in; empty for guests |
| Time | Generated when the backend writes the audit log entry |

## Field Limits

String fields are counted in Unicode characters; `detail` and the final message are measured in serialized bytes.

| Field | Maximum length |
| --- | ---: |
| `event` | 64 |
| `path` | 512 |
| `route` | 128 |
| `target` | 128 |
| User-Agent | 512 |
| `detail` JSON | 2048 |
| Final log message | 4096 |

If `detail` exceeds its limit, it is replaced with a truncation marker:

```json
{
  "truncated": true,
  "size": 4096
}
```

## Event Name Normalization

The server normalizes event names:

- Convert to lowercase
- Replace spaces with `_`
- Keep only letters, digits, `_`, `-`, `:`, and `.`
- Limit length to 64

For example:

```text
Page View -> page_view
node:open.detail -> node:open.detail
```

## Request Example: Page View

```bash
curl -X POST http://localhost:25774/api/rpc2 \
  -H 'Content-Type: application/json' \
  --data '{
    "jsonrpc": "2.0",
    "method": "public:recordVisitorEvent",
    "params": {
      "event": "page_view",
      "path": "/",
      "route": "home",
      "detail": {
        "theme": "glassmorphism"
      }
    },
    "id": 1
  }'
```

A successful write returns:

```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "success"
  },
  "id": 1
}
```

`disabled` and `rate_limited` are also normal responses, not JSON-RPC errors; theme reporting should not interrupt normal operation.

## Request Example: Opening Node Details

```json
{
  "jsonrpc": "2.0",
  "method": "public:recordVisitorEvent",
  "params": {
    "event": "node_open",
    "path": "/instance/6f0b-example",
    "route": "instance-detail",
    "target": "6f0b-example",
    "detail": {
      "source": "node_card"
    }
  },
  "id": 2
}
```

## Request Example: Search

```json
{
  "jsonrpc": "2.0",
  "method": "public:recordVisitorEvent",
  "params": {
    "event": "search",
    "path": "/",
    "route": "home",
    "detail": {
      "keyword_length": 6,
      "result_count": 3
    }
  },
  "id": 3
}
```

Avoid recording full search terms, which may expose sensitive information in the audit log.

## Frontend Wrapper Example

```ts
interface VisitorAuditEvent {
  event?: string
  action?: string
  operation?: string
  path?: string
  route?: string
  target?: string
  detail?: Record<string, unknown>
}

let auditRequestId = 0

export async function recordVisitorEvent(event: VisitorAuditEvent): Promise<void> {
  try {
    await fetch('/api/rpc2', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include',
      body: JSON.stringify({
        jsonrpc: '2.0',
        method: 'public:recordVisitorEvent',
        params: event,
        id: ++auditRequestId,
      }),
    })
  }
  catch {
    // Audit reporting must not interrupt normal browsing
  }
}
```

## Vue Router Page-Visit Example

```ts
router.afterEach((to) => {
  void recordVisitorEvent({
    event: 'page_view',
    path: to.fullPath,
    route: String(to.name ?? ''),
    detail: {
      params: Object.keys(to.params),
      query_keys: Object.keys(to.query),
    },
  })
})
```

Record query keys rather than full query values to avoid exposing tokens or other sensitive parameters.

## Recommended Event Names

| Scenario | event |
| --- | --- |
| Page view | `page_view` |
| Open node details | `node_open` |
| Search | `search` |
| Change group | `group_change` |
| Change view mode | `view_mode_change` |
| Click admin entry | `admin_entry_click` |
| Open advanced tools | `home_tool_open` |
| Refresh audit log | `audit_refresh` |
| Change audit log page | `audit_page_change` |
| Export JSON | `export_json` |
| Export CSV | `export_csv` |
| Start WebRTC self-check | `webrtc_check_start` |
| Finish WebRTC self-check | `webrtc_check_done` |

## Audit Log Storage Format

Writes to the existing `models.Log` table:

| Field | Value |
| --- | --- |
| `ip` | Source IP as seen by the server |
| `uuid` | Logged-in user's UUID; empty for guests |
| `msg_type` | `visitor` |
| `message` | `visitor event: {...}` |
| `time` | Time written by the backend |

Example `message`:

```text
visitor event: {"event":"page_view","path":"/","route":"home","user_agent":"Mozilla/5.0 ...","detail":{"theme":"glassmorphism"}}
```

## Reading Logs as an Admin

Use the existing method:

```text
admin:getLogs
```

Example request:

```json
{
  "jsonrpc": "2.0",
  "method": "admin:getLogs",
  "params": {
    "limit": "100",
    "page": "1",
    "msg_type": "visitor"
  },
  "id": 10
}
```

`msg_type` is an optional exact-match filter. With `visitor`, counting and pagination filter in SQL,
so the frontend does not need to fetch every log entry before filtering.

## Security Considerations

### Do Not Supply an IP from the Frontend

Frontend-supplied IPs are untrusted. This method uses the source IP seen by the server.

### Do Not Record Sensitive Values

Avoid recording:

- Passwords
- token
- cookie
- Full URL query strings
- Full search terms
- Exported content
- WebSSH command contents
- Clipboard contents

Record only:

- Action type
- Route
- Target ID
- Result count
- Boolean state
- Non-sensitive summary

### Reporting Must Not Block Normal Operation

Frontend calls should use:

```ts
void recordVisitorEvent(...)
```

Reporting failures must not affect the page.
