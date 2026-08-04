/**
 * Purpose: Canonical background-bound runtime contracts for captured telemetry and local diagnostics.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */

import type { LogEntry } from '../../types/capture/telemetry.js'
import type { WireWebSocketEvent as WebSocketEvent } from '../../types/wire/wire-websocket-event.js'
import type { WireNetworkBody as NetworkBodyPayload } from '../../types/wire/wire-network.js'
import type { WireEnhancedAction as EnhancedAction } from '../../types/wire/wire-enhanced-action.js'
import type { WirePerformanceSnapshot as PerformanceSnapshot } from '../../types/wire/wire-performance-snapshot.js'

export interface WsEventMessage {
  readonly type: 'ws_event'
  readonly payload: WebSocketEvent
  readonly tabId?: number
}

export interface EnhancedActionMessage {
  readonly type: 'enhanced_action'
  readonly payload: EnhancedAction
  readonly tabId?: number
}

export interface NetworkBodyMessage {
  readonly type: 'network_body'
  readonly payload: NetworkBodyPayload
  readonly tabId?: number
}

export interface PerformanceSnapshotMessage {
  readonly type: 'performance_snapshot'
  readonly payload: PerformanceSnapshot
  readonly tabId?: number
}

export interface CaptureDiagnosticMessage {
  readonly type: 'capture_diagnostic'
  readonly payload: {
    readonly category: string
    readonly message: string
    readonly error_type: string
  }
  readonly tabId?: number
}

export interface LogMessage {
  readonly type: 'log'
  readonly payload: LogEntry
  readonly tabId?: number
}
