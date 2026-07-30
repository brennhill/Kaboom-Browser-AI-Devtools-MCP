/**
 * Purpose: Own status, log-clearing, and debug diagnostic messages.
 */
import type {
  CircuitBreakerState,
  ConnectionStatus,
  ContextWarning,
  MemoryPressureState
} from '../../types/runtime/state.js'
import { KABOOM_LOG_PREFIX } from '../../lib/brand.js'
import { errorMessage } from '../../lib/error-utils.js'
import type { MessageHandlerOwner } from './types.js'
import type {
  StateRecoveryDiagnostic,
  StateRecoveryLifecycle
} from '../../types/runtime-messages.js'

export interface StatusHandlerDependencies {
  getConnectionStatus: () => ConnectionStatus
  getServerUrl: () => string
  getScreenshotOnError: () => boolean
  getSourceMapEnabled: () => boolean
  getDebugMode: () => boolean
  getContextWarning: () => ContextWarning | null
  getCircuitBreakerState: () => CircuitBreakerState
  getMemoryPressureState: () => MemoryPressureState
  clearLogs: () => Promise<{ success: boolean; error?: string }>
  exportDebugLog: () => string
  clearDebugLog: () => void
  debugLog: (category: string, message: string, data?: unknown) => void
  reportStateRecovery: (
    diagnostic: StateRecoveryDiagnostic,
    lifecycle: StateRecoveryLifecycle
  ) => void
}

export function createStatusMessageHandler(deps: StatusHandlerDependencies): MessageHandlerOwner {
  return {
    feature: 'status',
    handle(message, _sender, sendResponse) {
      switch (message.type) {
        case 'get_status':
          sendResponse({
            ...deps.getConnectionStatus(),
            serverUrl: deps.getServerUrl(),
            screenshotOnError: deps.getScreenshotOnError(),
            sourceMapEnabled: deps.getSourceMapEnabled(),
            debugMode: deps.getDebugMode(),
            contextWarning: deps.getContextWarning(),
            circuitBreakerState: deps.getCircuitBreakerState(),
            memoryPressure: deps.getMemoryPressureState()
          })
          return false
        case 'clear_logs':
          deps
            .clearLogs()
            .then(sendResponse)
            .catch((error) => {
              console.error(`${KABOOM_LOG_PREFIX} Failed to clear logs:`, error)
              sendResponse({ error: errorMessage(error) })
            })
          return true
        case 'report_state_recovery':
          deps.reportStateRecovery(message.diagnostic, message.lifecycle)
          sendResponse({ success: true })
          return false
        case 'get_debug_log':
          sendResponse({ log: deps.exportDebugLog() })
          return false
        case 'clear_debug_log':
          deps.clearDebugLog()
          deps.debugLog('lifecycle', 'Debug log cleared')
          sendResponse({ success: true })
          return false
        default:
          return undefined
      }
    }
  }
}
