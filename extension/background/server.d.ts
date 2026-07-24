/**
 * Purpose: HTTP functions for sending telemetry data (logs, WebSocket events, network bodies, actions, performance) to the Kaboom MCP server.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
/**
 * @fileoverview Server Communication - HTTP functions for sending data to
 * the Kaboom server.
 */
import type { LogEntry, WebSocketEvent, NetworkBodyPayload, EnhancedAction, PerformanceSnapshot, ConnectionStatus } from '../types/index.js';
import type { components } from '../generated/openapi-types.js';
/**
 * Server health response — union of the generated spec shape and the two
 * extension-side fields (`connected`, `error`) that this wrapper adds on top
 * of the raw server response. Using the generated HealthResponse ensures that
 * field names (e.g., `available_version`) stay snake_case in lockstep with
 * what the Go daemon actually emits.
 */
export type ServerHealthResponse = components['schemas']['HealthResponse'] & {
    connected: boolean;
    error?: string;
};
/**
 * Get standard headers for API requests including version header
 */
export declare function getRequestHeaders(additionalHeaders?: Record<string, string>): Record<string, string>;
/**
 * Send log entries to the server
 */
export declare function sendLogsToServer(serverUrl: string, entries: LogEntry[], debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<{
    entries: number;
}>;
/**
 * Send WebSocket events to the server
 */
export declare function sendWSEventsToServer(serverUrl: string, events: WebSocketEvent[], debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<void>;
/**
 * Send network bodies to the server
 */
export declare function sendNetworkBodiesToServer(serverUrl: string, bodies: NetworkBodyPayload[], debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<void>;
/**
 * Send enhanced actions to server
 */
export declare function sendEnhancedActionsToServer(serverUrl: string, actions: EnhancedAction[], debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<void>;
/**
 * Send performance snapshots to server
 */
export declare function sendPerformanceSnapshotsToServer(serverUrl: string, snapshots: PerformanceSnapshot[], debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<void>;
/**
 * Check server health
 */
export declare function checkServerHealth(serverUrl: string): Promise<ServerHealthResponse>;
/**
 * Update extension badge.
 * Uses Promise.all to ensure both text and color are applied atomically
 * before the MV3 service worker can be suspended.
 */
export declare function updateBadge(status: ConnectionStatus): void;
//# sourceMappingURL=server.d.ts.map