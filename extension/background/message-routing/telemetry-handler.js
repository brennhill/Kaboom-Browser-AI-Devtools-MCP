import { KABOOM_LOG_PREFIX } from '../../lib/brand.js';
export function createTelemetryMessageHandler(deps) {
    return {
        feature: 'telemetry',
        handle(message, sender) {
            switch (message.type) {
                case 'capture_diagnostic':
                    deps.addDiagnostic(message.payload);
                    return false;
                case 'ws_event':
                    deps.addWebSocket(message.payload);
                    return false;
                case 'enhanced_action': {
                    // The page cannot know what the extension pinned over CDP, so the stamp happens
                    // here — in the context that applied it — rather than being inferred later.
                    const environment = deps.environmentPinFor(message.payload.tab_id ?? message.tabId);
                    deps.addEnhancedAction(environment ? { ...message.payload, environment } : message.payload);
                    return false;
                }
                case 'performance_snapshot':
                    deps.addPerformance(message.payload);
                    return false;
                case 'network_body':
                    if (deps.isNetworkBodyCaptureDisabled()) {
                        deps.debugLog('capture', 'Network body dropped: capture disabled');
                        return false;
                    }
                    deps.addNetworkBody({ ...message.payload, tab_id: message.payload.tab_id ?? message.tabId });
                    return false;
                case 'log':
                    deps.handleLog(message.payload, sender, message.tabId).catch((error) => {
                        console.error(`${KABOOM_LOG_PREFIX} Failed to handle log message:`, error);
                    });
                    return false;
                default:
                    return undefined;
            }
        }
    };
}
//# sourceMappingURL=telemetry-handler.js.map