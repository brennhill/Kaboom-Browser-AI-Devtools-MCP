/**
 * Purpose: Owns background telemetry batchers, log ingestion, automatic screenshots, and log clearing.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import type { LogEntry } from '../../types/capture/telemetry.js';
import type { ChromeMessageSender } from '../../types/runtime/chrome.js';
export declare const sharedServerCircuitBreaker: import("../sync/circuit-breaker.js").CircuitBreaker;
export declare const logBatcher: import("../sync/batchers.js").Batcher<LogEntry>;
export declare const wsBatcher: import("../sync/batchers.js").Batcher<import("../../types/wire/wire-websocket-event.js").WireWebSocketEvent>;
export declare const enhancedActionBatcher: import("../sync/batchers.js").Batcher<import("../../types/wire/wire-enhanced-action.js").WireEnhancedAction>;
export declare const networkBodyBatcher: import("../sync/batchers.js").Batcher<import("../../types/wire/wire-network.js").WireNetworkBody>;
export declare const perfBatcher: import("../sync/batchers.js").Batcher<import("../../types/wire/wire-performance-snapshot.js").WirePerformanceSnapshot>;
export declare function handleLogMessage(payload: LogEntry, sender: ChromeMessageSender, tabId?: number): Promise<void>;
export declare function handleClearLogs(): Promise<{
    success: boolean;
    error?: string;
}>;
//# sourceMappingURL=stream-runtime.d.ts.map