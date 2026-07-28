/**
 * Purpose: Main background service worker hub -- owns debug logging, log handling, connection management, and batcher wiring.
 * Why: Central export point that delegates to specialized modules while owning cross-cutting concerns.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
/**
 * @fileoverview Main Background Service Worker — Business logic and export hub.
 * Mutable state lives in state.ts; this module owns debug logging, log handling,
 * connection management, and batcher wiring. Delegates batcher instance creation
 * to batcher-instances.ts and sync client lifecycle to sync-manager.ts.
 */
import type { LogEntry, ChromeMessageSender } from '../types/index.js';
import { handlePendingQuery as handlePendingQueryImpl, handlePilotCommand as handlePilotCommandImpl } from './pending-queries.js';
export { DEFAULT_SERVER_URL } from '../lib/constants.js';
export { DebugCategory } from './debug.js';
/**
 * Log a debug message (only when debug mode is enabled)
 */
export declare function debugLog(category: string, message: string, data?: unknown): void;
/**
 * Get all debug log entries
 */
export declare function getDebugLog(): import("../types/index.js").DebugLogEntry[];
/**
 * Clear debug log buffer
 */
export declare function clearDebugLog(): void;
/**
 * Export debug log as JSON string
 */
export declare function exportDebugLog(): string;
/**
 * Set debug mode enabled/disabled
 */
export declare function setDebugMode(enabled: boolean): void;
export declare const sharedServerCircuitBreaker: import("./sync/circuit-breaker.js").CircuitBreaker;
export declare const logBatcherWithCB: import("./sync/batchers.js").BatcherWithCircuitBreaker<LogEntry>;
export declare const logBatcher: import("./sync/batchers.js").Batcher<LogEntry>;
export declare const wsBatcher: import("./sync/batchers.js").Batcher<import("../types/index.js").WireWebSocketEvent>;
export declare const enhancedActionBatcherWithCB: import("./sync/batchers.js").BatcherWithCircuitBreaker<import("../types/index.js").WireEnhancedAction>;
export declare const enhancedActionBatcher: import("./sync/batchers.js").Batcher<import("../types/index.js").WireEnhancedAction>;
export declare const networkBodyBatcherWithCB: import("./sync/batchers.js").BatcherWithCircuitBreaker<import("../types/index.js").WireNetworkBody>;
export declare const networkBodyBatcher: import("./sync/batchers.js").Batcher<import("../types/index.js").WireNetworkBody>;
export declare const perfBatcherWithCB: import("./sync/batchers.js").BatcherWithCircuitBreaker<import("../types/index.js").WirePerformanceSnapshot>;
export declare const perfBatcher: import("./sync/batchers.js").Batcher<import("../types/index.js").WirePerformanceSnapshot>;
export declare function handleLogMessage(payload: LogEntry, sender: ChromeMessageSender, tabId?: number): Promise<void>;
export declare function handleClearLogs(): Promise<{
    success: boolean;
    error?: string;
}>;
/**
 * Check if a connection check is currently running (for testing)
 */
export declare function isConnectionCheckRunning(): boolean;
export declare function checkConnectionAndUpdate(): Promise<void>;
/**
 * Reset sync client connection (call when user enables pilot/tracking)
 */
export declare function resetSyncClientConnection(): void;
export declare const handlePendingQuery: typeof handlePendingQueryImpl;
export declare const handlePilotCommand: typeof handlePilotCommandImpl;
//# sourceMappingURL=index.d.ts.map