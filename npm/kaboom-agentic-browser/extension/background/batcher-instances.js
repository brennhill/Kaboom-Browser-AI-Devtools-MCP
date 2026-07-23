/**
 * Purpose: Creates concrete batcher instances for each telemetry data type (logs, WebSocket, actions, network bodies, performance).
 * Why: Isolates batcher wiring from business logic in index.ts to keep module initialization explicit.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import { updateBadge, createBatcherWithCircuitBreaker, sendLogsToServer, sendWSEventsToServer, sendEnhancedActionsToServer, sendNetworkBodiesToServer, sendPerformanceSnapshotsToServer } from './communication.js';
import { checkContextAnnotations } from './state-manager.js';
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
            throw err;
        }
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
    }), { sharedCircuitBreaker });
    const wsBatcherWithCB = createBatcherWithCircuitBreaker(withConnectionStatus(deps, (events) => sendWSEventsToServer(deps.getServerUrl(), events, deps.debugLog)), { debounceMs: 200, maxBatchSize: 100, sharedCircuitBreaker });
    const enhancedActionBatcherWithCB = createBatcherWithCircuitBreaker(withConnectionStatus(deps, (actions) => sendEnhancedActionsToServer(deps.getServerUrl(), actions, deps.debugLog)), { debounceMs: 200, maxBatchSize: 50, sharedCircuitBreaker });
    const networkBodyBatcherWithCB = createBatcherWithCircuitBreaker(withConnectionStatus(deps, (bodies) => sendNetworkBodiesToServer(deps.getServerUrl(), bodies, deps.debugLog)), { debounceMs: 200, maxBatchSize: 50, sharedCircuitBreaker });
    const perfBatcherWithCB = createBatcherWithCircuitBreaker(withConnectionStatus(deps, (snapshots) => sendPerformanceSnapshotsToServer(deps.getServerUrl(), snapshots, deps.debugLog)), { debounceMs: 500, maxBatchSize: 10, sharedCircuitBreaker });
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