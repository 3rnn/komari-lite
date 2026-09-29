/**
 * Standard JSON-RPC 2.0 type definitions
 * Based on https://www.jsonrpc.org/specification
 */

/**
 * JSON-RPC 2.0 request object
 */
export interface JSONRPC2Request<T = any> {
  /** JSON-RPC version; must be "2.0" */
  jsonrpc: "2.0";
  /** Method to call */
  method: string;
  /** Optional method parameters */
  params?: T;
  /** Request ID; absent for notifications */
  id?: string | number | null;
}

/**
 * Successful JSON-RPC 2.0 response object
 */
export interface JSONRPC2SuccessResponse<T = any> {
  /** JSON-RPC version; must be "2.0" */
  jsonrpc: "2.0";
  /** Result of the call */
  result: T;
  /** Request ID */
  id: string | number | null;
}

/**
 * JSON-RPC 2.0 error object
 */
export interface JSONRPC2Error {
  /** Error code */
  code: number;
  /** Error message */
  message: string;
  /** Optional error details */
  data?: any;
}

/**
 * Failed JSON-RPC 2.0 response object
 */
export interface JSONRPC2ErrorResponse {
  /** JSON-RPC version; must be "2.0" */
  jsonrpc: "2.0";
  /** Error information */
  error: JSONRPC2Error;
  /** Request ID */
  id: string | number | null;
}

/**
 * JSON-RPC 2.0 response union
 */
export type JSONRPC2Response<T = any> = JSONRPC2SuccessResponse<T> | JSONRPC2ErrorResponse;

/**
 * JSON-RPC 2.0 batch request
 */
export type JSONRPC2BatchRequest = JSONRPC2Request[];

/**
 * JSON-RPC 2.0 batch response
 */
export type JSONRPC2BatchResponse = JSONRPC2Response[];

/**
 * Predefined error codes
 */
export const JSONRPC2ErrorCode = {
  /** Parse error: server received invalid JSON */
  PARSE_ERROR: -32700,
  /** Invalid request: JSON is not a valid request object */
  INVALID_REQUEST: -32600,
  /** Method not found: method does not exist or is unavailable */
  METHOD_NOT_FOUND: -32601,
  /** Invalid params: method parameters are invalid */
  INVALID_PARAMS: -32602,
  /** Internal JSON-RPC error */
  INTERNAL_ERROR: -32603,
} as const;

export type JSONRPC2ErrorCodeType = typeof JSONRPC2ErrorCode[keyof typeof JSONRPC2ErrorCode];

/**
 * RPC connection state
 */
export const RPC2ConnectionState = {
  DISCONNECTED: "disconnected",
  CONNECTING: "connecting", 
  CONNECTED: "connected",
  RECONNECTING: "reconnecting",
  ERROR: "error",
} as const;

export type RPC2ConnectionStateType = typeof RPC2ConnectionState[keyof typeof RPC2ConnectionState];

/**
 * RPC connection options
 */
export interface RPC2ConnectionOptions {
  /** Connect automatically */
  autoConnect?: boolean;
  /** Reconnect automatically */
  autoReconnect?: boolean;
  /** Reconnect interval in milliseconds */
  reconnectInterval?: number;
  /** Maximum reconnect attempts */
  maxReconnectAttempts?: number;
  /** Request timeout in milliseconds */
  requestTimeout?: number;
  /** Enable heartbeat */
  enableHeartbeat?: boolean;
  /** Heartbeat interval in milliseconds */
  heartbeatInterval?: number;
  /** Custom headers (POST requests only) */
  headers?: Record<string, string>;
}

/**
 * RPC call options
 */
export interface RPC2CallOptions {
  /** Request timeout in milliseconds */
  timeout?: number;
  /** Notification request (no response expected) */
  notification?: boolean;
}

/**
 * Event listener types
 */
export interface RPC2EventListeners {
  onConnect?: () => void;
  onDisconnect?: () => void;
  onError?: (error: Error) => void;
  onReconnecting?: (attempt: number) => void;
  onMessage?: (data: any) => void;
}