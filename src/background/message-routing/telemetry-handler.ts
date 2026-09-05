/**
 * Purpose: Own routing for telemetry events emitted by content scripts.
 */
import type { ChromeMessageSender } from '../../types/runtime/chrome.js'
import type { WireEnhancedAction as EnhancedAction, WireEnvironmentPin } from '../../types/wire/wire-enhanced-action.js'
import type { LogEntry } from '../../types/capture/telemetry.js'
import type { WireNetworkBody as NetworkBodyPayload } from '../../types/wire/wire-network.js'
import type { WirePerformanceSnapshot as PerformanceSnapshot } from '../../types/wire/wire-performance-snapshot.js'
import type { WireWebSocketEvent as WebSocketEvent } from '../../types/wire/wire-websocket-event.js'
import { KABOOM_LOG_PREFIX } from '../../lib/brand.js'
import type { MessageHandlerOwner } from './types.js'
import type { CaptureDiagnosticMessage } from '../../types/runtime/telemetry-messages.js'

export interface TelemetryHandlerDependencies {
  addLog: (entry: LogEntry) => void
  addWebSocket: (event: WebSocketEvent) => void
  addEnhancedAction: (action: EnhancedAction) => void
  /**
   * The environment pin in force for the tab that produced an action, or undefined when the
   * tab is not pinned. Read per action rather than per session because a navigation clears
   * CDP overrides: a session-level stamp would claim a pin that lapsed halfway through.
   */
  environmentPinFor: (tabId: number | undefined) => WireEnvironmentPin | undefined
  addNetworkBody: (body: NetworkBodyPayload) => void
  addPerformance: (snapshot: PerformanceSnapshot) => void
  handleLog: (payload: LogEntry, sender: ChromeMessageSender, tabId?: number) => Promise<void>
  isNetworkBodyCaptureDisabled: () => boolean
  debugLog: (category: string, message: string, data?: unknown) => void
  addDiagnostic: (payload: CaptureDiagnosticMessage['payload']) => void
}

export function createTelemetryMessageHandler(deps: TelemetryHandlerDependencies): MessageHandlerOwner {
  return {
    feature: 'telemetry',
    handle(message, sender) {
      switch (message.type) {
        case 'capture_diagnostic':
          deps.addDiagnostic(message.payload)
          return false
        case 'ws_event':
          deps.addWebSocket(message.payload)
          return false
        case 'enhanced_action': {
          // The page cannot know what the extension pinned over CDP, so the stamp happens
          // here — in the context that applied it — rather than being inferred later.
          const environment = deps.environmentPinFor(message.payload.tab_id ?? message.tabId)
          deps.addEnhancedAction(environment ? { ...message.payload, environment } : message.payload)
          return false
        }
        case 'performance_snapshot':
          deps.addPerformance(message.payload)
          return false
        case 'network_body':
          if (deps.isNetworkBodyCaptureDisabled()) {
            deps.debugLog('capture', 'Network body dropped: capture disabled')
            return false
          }
          deps.addNetworkBody({ ...message.payload, tab_id: message.payload.tab_id ?? message.tabId })
          return false
        case 'log':
          deps.handleLog(message.payload, sender, message.tabId).catch((error) => {
            console.error(`${KABOOM_LOG_PREFIX} Failed to handle log message:`, error)
          })
          return false
        default:
          return undefined
      }
    }
  }
}
