# RPC2 Client Guide

This RPC2 client implements JSON-RPC 2.0 and calls Komari's `/api/rpc2` over WebSocket or HTTP POST.

## Features

- ✅ JSON-RPC 2.0 support
- ✅ WebSocket and HTTP POST transports
- ✅ **Automatic WebSocket connection management**
- ✅ **Automatic heartbeat every 5 seconds**
- ✅ Automatic reconnection
- ✅ Request timeouts
- ✅ Batch requests
- ✅ Notification requests
- ✅ TypeScript types
- ✅ React Context integration

## Quick start

### 1. Basic usage

```typescript
import { RPC2Client } from './lib/rpc2';

// Create a client (automatic connection is enabled by default).
const client = new RPC2Client('/api/rpc2');

// Call directly without connecting manually; the client picks a transport.
const result = await client.call('getStatus');

// Force an HTTP call.
const httpResult = await client.callViaHTTP('getNodes', { active: true });

// Force a WebSocket call (connects automatically).
const wsResult = await client.callViaWebSocket('getNodes', { active: true });
```

### 2. React integration

```tsx
import { RPC2Provider, useRPC2Call } from './contexts/RPC2Context';

// Wrap the app root in the provider.
function App() {
  return (
    <RPC2Provider>
      <MyComponent />
    </RPC2Provider>
  );
}

// Use the hook from a component.
function MyComponent() {
  const { call } = useRPC2Call();
  
  const handleCall = async () => {
    try {
      const result = await call('getStatus');
      console.log(result);
    } catch (error) {
      console.error('Call failed:', error);
    }
  };
  
  return (
    <button onClick={handleCall} disabled={!isConnected}>
      Call RPC
    </button>
  );
}
```

## API reference

### RPC2Client class

#### Constructor
```typescript
new RPC2Client(baseUrl?: string, options?: RPC2ConnectionOptions)
```

#### Main methods

- `call(method, params?, options?)`: **Recommended**; automatically picks a transport.
- `callViaWebSocket(method, params?, options?)`: Force WebSocket.
- `callViaHTTP(method, params?, options?)`: Force HTTP.
- `batchCall(requests)`: Batch calls (HTTP only).
- `connect()`: Connect the WebSocket manually (usually unnecessary).
- `disconnect()`: Disconnect the WebSocket.

#### Configuration

```typescript
interface RPC2ConnectionOptions {
interface RPC2ConnectionOptions {
    autoConnect?: boolean;          // Connect automatically (default: true).
    autoReconnect?: boolean;        // Reconnect automatically (default: true).
    reconnectInterval?: number;     // Reconnect interval (default: 3000 ms).
    maxReconnectAttempts?: number;  // Retry limit (default: 5).
    requestTimeout?: number;        // Request timeout (default: 30000 ms).
    enableHeartbeat?: boolean;      // Enable heartbeats (default: true).
    heartbeatInterval?: number;     // Heartbeat interval (default: 5000 ms).
    headers?: Record<string, string>; // Custom request headers.
}
```

**Note**:
- WebSocket connects and stays connected automatically by default.
- The heartbeat runs every 5 seconds to keep the connection alive.

### React Hooks

#### useRPC2()
Returns RPC2 connection state and controls.

#### useRPC2Call()
Returns the RPC call method.

## Examples

### Basic calls
```typescript
// Get system status.
const status = await client.call('getStatus');

// Get nodes.
const nodes = await client.call('getNodes', { active: true });

// Update a node.
const result = await client.call('updateNode', {
  id: 1,
  name: 'new-name',
  weight: 100
});
```

### Batch calls
```typescript
const results = await client.batchCall([
  { method: 'getStatus' },
  { method: 'getNodes', params: { active: true } },
  { method: 'getVersion' }
]);
```

### Notification requests
```typescript
// Send a notification (no response expected).
await client.call('notifyUpdate', {
  timestamp: Date.now()
}, { notification: true });
```

### WebSocket connection management

**Automatic mode (recommended):**
```typescript
// Create a client with automatic connection management.
const client = new RPC2Client('/api/rpc2');

// Call directly without managing the connection.
const result = await client.call('getStatus');

// Add event listeners.
client.setEventListeners({
    onConnect: () => console.log('WebSocket connected'),
    onDisconnect: () => console.log('WebSocket disconnected'),
    onError: (error) => console.error('Connection error:', error),
    onReconnecting: (attempt) => console.log(`Retry ${attempt}`)
});
```

**Manual mode:**
```typescript
// Disable automatic connection.
const client = new RPC2Client('/api/rpc2', { autoConnect: false });

// Connect manually.
await client.connect();

// Disconnect manually.
client.disconnect();
```

## Error handling

The client handles these error cases automatically:

1. **Network errors**: Automatic reconnection.
2. **Request timeouts**: Configurable timeout.
3. **JSON-RPC errors**: Standard error codes.
4. **Disconnections**: Automatic reconnection and status notifications.

## Notes

1. **Automatic connection is enabled by default**: WebSocket connects and stays connected.
2. Batch calls only work over HTTP.
3. Notifications do not return responses.
4. Ensure the server supports JSON-RPC 2.0.
