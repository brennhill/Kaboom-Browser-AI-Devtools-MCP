/**
 * Purpose: Creates concrete batcher instances for each telemetry data type (logs, WebSocket, actions, network bodies, performance).
 * Why: Isolates batcher wiring from business logic in index.ts to keep module initialization explicit.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import { createBatcherWithCircuitBreaker } from './batchers.js';
import { updateBadge, sendLogsToServer, sendWSEventsToServer, sendEnhancedActionsToServer, sendNetworkBodiesToServer, sendPerformanceSnapshotsToServer } from './server.js';
import { checkContextAnnotations } from '../caches/snapshots.js';
import { errorMessage } from '../../lib/error-utils.js';
import { reportStateRecovery, resolveStateRecovery } from '../runtime-state/state-recovery.js';
// =============================================================================
// CONNECTION STATUS WRAPPER
// =============================================================================
function withConnectionStatus(deps, sendFn, onSuccess) {
    return async (entries) => {
        try {
            const result = await sendFn(entries);
            deps.setConnectionStatus({ connected: true });
            if (onSuccess)
                onSuccess(entries, result);
            updateBadge(deps.getConnectionStatus());
            return result;
        }
        catch (err) {
            deps.setConnectionStatus({ connected: false });
            updateBadge(deps.getConnectionStatus());
            deps.debugLog('error', 'Telemetry batch delivery failed', {
                correlation_id: `telemetry_batch_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
                error: errorMessage(err)
            });
            throw err;
        }
    };
}
function pressureLifecycle(deps, stream) {
    const name = `telemetry_${stream}_pressure`;
    return {
        onPressure: (event) => {
            deps.debugLog('error', 'Telemetry buffer pressure dropped entries', {
                stream,
                reason: event.reason,
                dropped_count: event.dropped,
                total_dropped: event.total_dropped,
                pending_count: event.pending,
                capacity: event.capacity
            });
            reportStateRecovery({
                name,
                detail: `${stream} telemetry exceeded its local delivery buffer; ${event.dropped} entr${event.dropped === 1 ? 'y was' : 'ies were'} dropped.`,
                fix: 'Restore the daemon connection and inspect local extension diagnostics.',
                correlation_id: name,
                expected_next_transition: 'buffer_delivery_recovered',
                recovery_attempt: event.total_dropped,
                recovery_outcome: 'fallback'
            });
        },
        onPressureRecovered: () => resolveStateRecovery(name)
    };
}
/**
 * Create all batcher instances wired to the shared circuit breaker.
 * Called once from index.ts during module initialization.
 */
export function createBatcherInstances(deps, sharedCircuitBreaker) {
    const logBatcherWithCB = createBatcherWithCircuitBreaker(withConnectionStatus(deps, (entries) => {
        checkContextAnnotations(entries);
        return sendLogsToServer(deps.getServerUrl(), entries, deps.debugLog);
    }, (entries, result) => {
        const typedResult = result;
        const status = deps.getConnectionStatus();
        deps.setConnectionStatus({
            entries: typedResult.entries || status.entries + entries.length,
            errorCount: status.errorCount + entries.filter((e) => e.level === 'error').length
        });
    }), { sharedCircuitBreaker, ...pressureLifecycle(deps, 'console') });
    const wsBatcherWithCB = createBatcherWithCircuitBreaker(withConnectionStatus(deps, (events) => sendWSEventsToServer(deps.getServerUrl(), events, deps.debugLog)), { debounceMs: 200, maxBatchSize: 100, sharedCircuitBreaker, ...pressureLifecycle(deps, 'websocket') });
    const enhancedActionBatcherWithCB = createBatcherWithCircuitBreaker(withConnectionStatus(deps, (actions) => sendEnhancedActionsToServer(deps.getServerUrl(), actions, deps.debugLog)), { debounceMs: 200, maxBatchSize: 50, sharedCircuitBreaker, ...pressureLifecycle(deps, 'action') });
    const networkBodyBatcherWithCB = createBatcherWithCircuitBreaker(withConnectionStatus(deps, (bodies) => sendNetworkBodiesToServer(deps.getServerUrl(), bodies, deps.debugLog)), { debounceMs: 200, maxBatchSize: 50, sharedCircuitBreaker, ...pressureLifecycle(deps, 'network_body') });
    const perfBatcherWithCB = createBatcherWithCircuitBreaker(withConnectionStatus(deps, (snapshots) => sendPerformanceSnapshotsToServer(deps.getServerUrl(), snapshots, deps.debugLog)), { debounceMs: 500, maxBatchSize: 10, sharedCircuitBreaker, ...pressureLifecycle(deps, 'performance') });
    return {
        logBatcherWithCB,
        logBatcher: logBatcherWithCB.batcher,
        wsBatcherWithCB,
        wsBatcher: wsBatcherWithCB.batcher,
        enhancedActionBatcherWithCB,
        enhancedActionBatcher: enhancedActionBatcherWithCB.batcher,
        networkBodyBatcherWithCB,
        networkBodyBatcher: networkBodyBatcherWithCB.batcher,
        perfBatcherWithCB,
        perfBatcher: perfBatcherWithCB.batcher
    };
}
//# sourceMappingURL=batcher-instances.js.map