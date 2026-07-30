import { KABOOM_LOG_PREFIX } from '../../lib/brand.js';
import { errorMessage } from '../../lib/error-utils.js';
export function createStatusMessageHandler(deps) {
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
                    });
                    return false;
                case 'clear_logs':
                    deps
                        .clearLogs()
                        .then(sendResponse)
                        .catch((error) => {
                        console.error(`${KABOOM_LOG_PREFIX} Failed to clear logs:`, error);
                        sendResponse({ error: errorMessage(error) });
                    });
                    return true;
                case 'report_state_recovery':
                    deps.reportStateRecovery(message.diagnostic, message.lifecycle);
                    sendResponse({ success: true });
                    return false;
                case 'get_debug_log':
                    sendResponse({ log: deps.exportDebugLog() });
                    return false;
                case 'clear_debug_log':
                    deps.clearDebugLog();
                    deps.debugLog('lifecycle', 'Debug log cleared');
                    sendResponse({ success: true });
                    return false;
                default:
                    return undefined;
            }
        }
    };
}
//# sourceMappingURL=status-handler.js.map