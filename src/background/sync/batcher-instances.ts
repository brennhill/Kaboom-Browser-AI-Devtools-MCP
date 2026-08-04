/**
 * Purpose: Creates concrete batcher instances for each telemetry data type (logs, WebSocket, actions, network bodies, performance).
 * Why: Isolates batcher wiring from business logic in index.ts to keep module initialization explicit.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */

// batcher-instances.ts — Concrete batcher instances for each data type.
// Creates log, WebSocket, enhanced-action, network-body, and performance batchers,
// each wired to the shared circuit breaker and connection-status tracking.

import type { ConnectionStatus } from '../../types/runtime/state.js'
import type { LogEntry } from '../../types/capture/telemetry.js'
import type { WireWebSocketEvent as WebSocketEvent } from '../../types/wire/wire-websocket-event.js'
import type { WireNetworkBody as NetworkBodyPayload } from '../../types/wire/wire-network.js'
import type { WireEnhancedAction as EnhancedAction } from '../../types/wire/wire-enhanced-action.js'
import type { WirePerformanceSnapshot as PerformanceSnapshot } from '../../types/wire/wire-performance-snapshot.js'

import {
  createBatcherWithCircuitBreaker,
  type BatcherWithCircuitBreaker,
  type Batcher,
  type BatcherConfig
} from './batchers.js'
import type { CircuitBreaker } from './circuit-breaker.js'
import {
  updateBadge,
  sendLogsToServer,
  sendWSEventsToServer,
  sendEnhancedActionsToServer,
  sendNetworkBodiesToServer,
  sendPerformanceSnapshotsToServer
} from './server.js'
import { checkContextAnnotations } from '../caches/snapshots.js'
import { errorMessage } from '../../lib/error-utils.js'
import { reportStateRecovery, resolveStateRecovery } from '../runtime-state/state-recovery.js'

// =============================================================================
// TYPES
// =============================================================================

/** Mutable connection status passed in from the state owner */
export type ConnectionStatusRef = Pick<ConnectionStatus, keyof ConnectionStatus>

type DebugLogFn = (category: string, message: string, data?: unknown) => void

/** Dependencies injected by index.ts to avoid circular imports */
export interface BatcherDeps {
  getServerUrl: () => string
  getConnectionStatus: () => ConnectionStatusRef
  setConnectionStatus: (patch: Partial<ConnectionStatusRef>) => void
  debugLog: DebugLogFn
}

// =============================================================================
// CONNECTION STATUS WRAPPER
// =============================================================================

function withConnectionStatus<T>(
  deps: BatcherDeps,
  sendFn: (entries: T[]) => Promise<unknown>,
  onSuccess?: (entries: T[], result: unknown) => void
): (entries: T[]) => Promise<unknown> {
  return async (entries: T[]) => {
    try {
      const result = await sendFn(entries)
      deps.setConnectionStatus({ connected: true })
      if (onSuccess) onSuccess(entries, result)
      updateBadge(deps.getConnectionStatus())
      return result
    } catch (err) {
      deps.setConnectionStatus({ connected: false })
      updateBadge(deps.getConnectionStatus())
      deps.debugLog('error', 'Telemetry batch delivery failed', {
        correlation_id: `telemetry_batch_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
        error: errorMessage(err)
      })
      throw err
    }
  }
}

function pressureLifecycle(deps: BatcherDeps, stream: string): Pick<BatcherConfig, 'onPressure' | 'onPressureRecovered'> {
  const name = `telemetry_${stream}_pressure`
  return {
    onPressure: (event) => {
      deps.debugLog('error', 'Telemetry buffer pressure dropped entries', {
        stream,
        reason: event.reason,
        dropped_count: event.dropped,
        total_dropped: event.total_dropped,
        pending_count: event.pending,
        capacity: event.capacity
      })
      reportStateRecovery({
        name,
        detail: `${stream} telemetry exceeded its local delivery buffer; ${event.dropped} entr${event.dropped === 1 ? 'y was' : 'ies were'} dropped.`,
        fix: 'Restore the daemon connection and inspect local extension diagnostics.',
        correlation_id: name,
        expected_next_transition: 'buffer_delivery_recovered',
        recovery_attempt: event.total_dropped,
        recovery_outcome: 'fallback'
      })
    },
    onPressureRecovered: () => resolveStateRecovery(name)
  }
}

// =============================================================================
// FACTORY
// =============================================================================

export interface BatcherInstances {
  logBatcherWithCB: BatcherWithCircuitBreaker<LogEntry>
  logBatcher: Batcher<LogEntry>
  wsBatcherWithCB: BatcherWithCircuitBreaker<WebSocketEvent>
  wsBatcher: Batcher<WebSocketEvent>
  enhancedActionBatcherWithCB: BatcherWithCircuitBreaker<EnhancedAction>
  enhancedActionBatcher: Batcher<EnhancedAction>
  networkBodyBatcherWithCB: BatcherWithCircuitBreaker<NetworkBodyPayload>
  networkBodyBatcher: Batcher<NetworkBodyPayload>
  perfBatcherWithCB: BatcherWithCircuitBreaker<PerformanceSnapshot>
  perfBatcher: Batcher<PerformanceSnapshot>
}

/**
 * Create all batcher instances wired to the shared circuit breaker.
 * Called once from index.ts during module initialization.
 */
export function createBatcherInstances(deps: BatcherDeps, sharedCircuitBreaker: CircuitBreaker): BatcherInstances {
  const logBatcherWithCB = createBatcherWithCircuitBreaker<LogEntry>(
    withConnectionStatus(
      deps,
      (entries) => {
        checkContextAnnotations(entries)
        return sendLogsToServer(deps.getServerUrl(), entries, deps.debugLog)
      },
      (entries, result) => {
        const typedResult = result as { entries?: number }
        const status = deps.getConnectionStatus()
        deps.setConnectionStatus({
          entries: typedResult.entries || status.entries + entries.length,
          errorCount: status.errorCount + entries.filter((e) => e.level === 'error').length
        })
      }
    ),
    { sharedCircuitBreaker, ...pressureLifecycle(deps, 'console') }
  )

  const wsBatcherWithCB = createBatcherWithCircuitBreaker<WebSocketEvent>(
    withConnectionStatus(deps, (events) => sendWSEventsToServer(deps.getServerUrl(), events, deps.debugLog)),
    { debounceMs: 200, maxBatchSize: 100, sharedCircuitBreaker, ...pressureLifecycle(deps, 'websocket') }
  )

  const enhancedActionBatcherWithCB = createBatcherWithCircuitBreaker<EnhancedAction>(
    withConnectionStatus(deps, (actions) => sendEnhancedActionsToServer(deps.getServerUrl(), actions, deps.debugLog)),
    { debounceMs: 200, maxBatchSize: 50, sharedCircuitBreaker, ...pressureLifecycle(deps, 'action') }
  )

  const networkBodyBatcherWithCB = createBatcherWithCircuitBreaker<NetworkBodyPayload>(
    withConnectionStatus(deps, (bodies) => sendNetworkBodiesToServer(deps.getServerUrl(), bodies, deps.debugLog)),
    { debounceMs: 200, maxBatchSize: 50, sharedCircuitBreaker, ...pressureLifecycle(deps, 'network_body') }
  )

  const perfBatcherWithCB = createBatcherWithCircuitBreaker<PerformanceSnapshot>(
    withConnectionStatus(deps, (snapshots) =>
      sendPerformanceSnapshotsToServer(deps.getServerUrl(), snapshots, deps.debugLog)
    ),
    { debounceMs: 500, maxBatchSize: 10, sharedCircuitBreaker, ...pressureLifecycle(deps, 'performance') }
  )

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
  }
}
