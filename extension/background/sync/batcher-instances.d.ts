/**
 * Purpose: Creates concrete batcher instances for each telemetry data type (logs, WebSocket, actions, network bodies, performance).
 * Why: Isolates batcher wiring from business logic in index.ts to keep module initialization explicit.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import type { ConnectionStatus } from '../../types/runtime/state.js';
import type { LogEntry } from '../../types/capture/telemetry.js';
import type { WireWebSocketEvent as WebSocketEvent } from '../../types/wire/wire-websocket-event.js';
import type { WireNetworkBody as NetworkBodyPayload } from '../../types/wire/wire-network.js';
import type { WireEnhancedAction as EnhancedAction } from '../../types/wire/wire-enhanced-action.js';
import type { WirePerformanceSnapshot as PerformanceSnapshot } from '../../types/wire/wire-performance-snapshot.js';
import { type BatcherWithCircuitBreaker, type Batcher } from './batchers.js';
import type { CircuitBreaker } from './circuit-breaker.js';
/** Mutable connection status passed in from the state owner */
export type ConnectionStatusRef = Pick<ConnectionStatus, keyof ConnectionStatus>;
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