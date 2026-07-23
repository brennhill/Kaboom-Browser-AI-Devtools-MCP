/**
 * Purpose: Creates concrete batcher instances for each telemetry data type (logs, WebSocket, actions, network bodies, performance).
 * Why: Isolates batcher wiring from business logic in index.ts to keep module initialization explicit.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import type { LogEntry, WebSocketEvent, NetworkBodyPayload, EnhancedAction, PerformanceSnapshot } from '../types/index.js';
import { type CircuitBreaker, type BatcherWithCircuitBreaker, type Batcher } from './communication.js';
/** Mutable connection status passed in from the state owner */
export interface ConnectionStatusRef {
    connected: boolean;
    entries: number;
    maxEntries: number;
    errorCount: number;
    logFile: string;
    logFileSize?: number;
    serverVersion?: string;
    extensionVersion?: string;
    versionMismatch?: boolean;
}
type DebugLogFn = (category: string, message: string, data?: unknown) => void;
/** Dependencies injected by index.ts to avoid circular imports */
export interface BatcherDeps {
    getServerUrl: () => string;
    getConnectionStatus: () => ConnectionStatusRef;
    setConnectionStatus: (patch: Partial<ConnectionStatusRef>) => void;
    debugLog: DebugLogFn;
}
export interface BatcherInstances {
    logBatcherWithCB: BatcherWithCircuitBreaker<LogEntry>;
    logBatcher: Batcher<LogEntry>;
    wsBatcherWithCB: BatcherWithCircuitBreaker<WebSocketEvent>;
    wsBatcher: Batcher<WebSocketEvent>;
    enhancedActionBatcherWithCB: BatcherWithCircuitBreaker<EnhancedAction>;
    enhancedActionBatcher: Batcher<EnhancedAction>;
    networkBodyBatcherWithCB: BatcherWithCircuitBreaker<NetworkBodyPayload>;
    networkBodyBatcher: Batcher<NetworkBodyPayload>;
    perfBatcherWithCB: BatcherWithCircuitBreaker<PerformanceSnapshot>;
    perfBatcher: Batcher<PerformanceSnapshot>;
}
/**
 * Create all batcher instances wired to the shared circuit breaker.
 * Called once from index.ts during module initialization.
 */
export declare function createBatcherInstances(deps: BatcherDeps, sharedCircuitBreaker: CircuitBreaker): BatcherInstances;
export {};
//# sourceMappingURL=batcher-instances.d.ts.map