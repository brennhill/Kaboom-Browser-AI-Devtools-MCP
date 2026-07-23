/**
 * Purpose: HTTP functions for sending telemetry data (logs, WebSocket events, network bodies, actions, performance) to the Kaboom MCP server.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
/**
 * @fileoverview Server Communication - HTTP functions for sending data to
 * the Kaboom server.
 */
import type { LogEntry, WebSocketEvent, NetworkBodyPayload, EnhancedAction, PerformanceSnapshot, ConnectionStatus } from '../types/index.js';
/**
 * Server health response
 */
export interface ServerHealthResponse {
    connected: boolean;
    error?: string;
    version?: string;
    availableVersion?: string;
    capture?: {
        available?: boolean;
        pilot_enabled?: boolean;
        pilot_state?: string;
        extension_connected?: boolean;
        extension_last_seen?: string;
        extension_client_id?: string;
        security_mode?: string;
        production_parity?: boolean;
        insecure_rewrites?: number;
    };
    logs?: {
        logFile?: string;
        logFileSize?: number;
        entries?: number;
        maxEntries?: number;
    };
}
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
/**
 * Send status ping to server
 */
export declare function sendStatusPing(serverUrl: string, statusMessage: {
    type: string;
    tracking_enabled: boolean;
    tracked_tab_id: number | null;
    tracked_tab_url: string | null;
    message: string;
    extension_connected: boolean;
    timestamp: string;
}, diagnosticLogFn?: (message: string) => void): Promise<void>;
//# sourceMappingURL=server.d.ts.map