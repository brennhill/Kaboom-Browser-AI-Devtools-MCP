// @ts-nocheck
/**
 * Test-only composition for feature-owned background message handlers.
 */
import { createTelemetryMessageHandler } from '../../../extension/background/message-routing/telemetry-handler.js'
import { createStatusMessageHandler } from '../../../extension/background/message-routing/status-handler.js'
import { createSettingsMessageHandler } from '../../../extension/background/message-routing/settings-handler.js'
import { createPilotMessageHandler } from '../../../extension/background/message-routing/pilot-handler.js'
import { createCaptureMessageHandler } from '../../../extension/background/message-routing/capture-handler.js'
import { createUtilityMessageHandler } from '../../../extension/background/message-routing/utility-handler.js'

export function composeBackgroundHandlers(deps) {
  return [
    createTelemetryMessageHandler({
      addLog: deps.addToLogBatcher, addWebSocket: deps.addToWsBatcher,
      addEnhancedAction: deps.addToEnhancedActionBatcher, addNetworkBody: deps.addToNetworkBodyBatcher,
      addPerformance: deps.addToPerfBatcher, handleLog: deps.handleLogMessage,
      isNetworkBodyCaptureDisabled: deps.isNetworkBodyCaptureDisabled, debugLog: deps.debugLog
      ,addDiagnostic: deps.addDiagnostic
      // Mirrors init.ts. A dependency missing here dies inside the handler's switch, so the
      // route it belongs to silently stops delivering.
      ,environmentPinFor: deps.environmentPinFor ?? (() => undefined)
    }),
    createStatusMessageHandler({
      getConnectionStatus: deps.getConnectionStatus, getServerUrl: deps.getServerUrl,
      getScreenshotOnError: deps.getScreenshotOnError, getSourceMapEnabled: deps.getSourceMapEnabled,
      getDebugMode: deps.getDebugMode, getContextWarning: deps.getContextWarning,
      getCircuitBreakerState: deps.getCircuitBreakerState, getMemoryPressureState: deps.getMemoryPressureState,
      clearLogs: deps.handleClearLogs, exportDebugLog: deps.exportDebugLog,
      clearDebugLog: deps.clearDebugLog, debugLog: deps.debugLog
    }),
    createSettingsMessageHandler({
      getServerUrl: deps.getServerUrl, setServerUrl: deps.setServerUrl,
      setLogLevel: deps.setCurrentLogLevel, setScreenshotOnError: deps.setScreenshotOnError,
      setSourceMapEnabled: deps.setSourceMapEnabled, setDebugMode: deps.setDebugMode,
      clearSourceMapCache: deps.clearSourceMapCache, saveSetting: deps.saveSetting,
      forwardToContentScripts: deps.forwardToAllContentScripts,
      checkConnection: deps.checkConnectionAndUpdate, debugLog: deps.debugLog
    }),
    createPilotMessageHandler({
      isEnabled: deps.getAiWebPilotEnabled,
      setEnabled: deps.setAiWebPilotEnabled,
      getTrackingContinuity: () => ({ phase: 'idle', is_tracked: false }),
      confirmTracking: () => {}
    }),
    createCaptureMessageHandler({
      getServerUrl: deps.getServerUrl, captureScreenshot: deps.captureScreenshot,
      addLog: deps.addToLogBatcher, debugLog: deps.debugLog
    }),
    createUtilityMessageHandler({ getServerUrl: deps.getServerUrl })
  ]
}
