/**
 * Purpose: Facade that re-exports communication primitives (circuit breaker, batchers, server HTTP) and provides log formatting and screenshot capture.
 * Why: Single import point for communication functions, avoiding scattered imports across consumers.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
/**
 * @fileoverview Communication - Facade that re-exports communication functions
 * from modular subcomponents: circuit-breaker.ts, batchers.ts, and server.ts
 */
export { createCircuitBreaker, type CircuitBreakerOptions, type CircuitBreaker } from './circuit-breaker.js';
export { createBatcherWithCircuitBreaker, createLogBatcher, RATE_LIMIT_CONFIG, type Batcher, type BatcherWithCircuitBreaker, type BatcherConfig, type LogBatcherOptions } from './batchers.js';
export { sendLogsToServer, sendWSEventsToServer, sendNetworkBodiesToServer, sendEnhancedActionsToServer, sendPerformanceSnapshotsToServer, checkServerHealth, updateBadge, sendStatusPing, type ServerHealthResponse } from './server.js';
import type { LogEntry } from '../types/index.js';
/**
 * Format a log entry with timestamp and truncation
 */
export declare function formatLogEntry(entry: LogEntry): LogEntry;
/**
 * Determine if a log should be captured based on level filter
 */
export declare function shouldCaptureLog(logLevel: string, filterLevel: string, logType?: string): boolean;
/**
 * Capture a screenshot of the visible tab area
 */
export declare function captureScreenshot(tabId: number, serverUrl: string, relatedErrorId: string | null, errorType: string | null, canTakeScreenshotFn: (tabId: number) => {
    allowed: boolean;
    reason?: string;
    nextAllowedIn?: number | null;
}, recordScreenshotFn: (tabId: number) => void, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<{
    success: boolean;
    entry?: LogEntry;
    error?: string;
    nextAllowedIn?: number | null;
}>;
//# sourceMappingURL=communication.d.ts.map