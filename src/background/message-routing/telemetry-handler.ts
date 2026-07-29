/**
 * Purpose: Own routing for telemetry events emitted by content scripts.
 */
import type {
  ChromeMessageSender,
  EnhancedAction,
  LogEntry,
  NetworkBodyPayload,
  PerformanceSnapshot,
  WebSocketEvent
} from '../../types/index.js'
import { KABOOM_LOG_PREFIX } from '../../lib/brand.js'
import type { MessageHandlerOwner } from './types.js'

export interface TelemetryHandlerDependencies {
  addLog: (entry: LogEntry) => void
  addWebSocket: (event: WebSocketEvent) => void
  addEnhancedAction: (action: EnhancedAction) => void
  addNetworkBody: (body: NetworkBodyPayload) => void
  addPerformance: (snapshot: PerformanceSnapshot) => void
  handleLog: (payload: LogEntry, sender: ChromeMessageSender, tabId?: number) => Promise<void>
  isNetworkBodyCaptureDisabled: () => boolean
  debugLog: (category: string, message: string, data?: unknown) => void
}

export function createTelemetryMessageHandler(deps: TelemetryHandlerDependencies): MessageHandlerOwner {
  return {
    feature: 'telemetry',
    handle(message, sender) {
      switch (message.type) {
        case 'ws_event':
          deps.addWebSocket(message.payload)
          return false
        case 'enhanced_action':
          deps.addEnhancedAction(message.payload)
          return false
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
