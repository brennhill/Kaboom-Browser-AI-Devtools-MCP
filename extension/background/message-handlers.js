export { createTelemetryMessageHandler } from './message-routing/telemetry-handler.js';
export { createStatusMessageHandler } from './message-routing/status-handler.js';
export { createSettingsMessageHandler } from './message-routing/settings-handler.js';
export { createPilotMessageHandler, broadcastTrackingState } from './message-routing/pilot-handler.js';
export { createCaptureMessageHandler } from './message-routing/capture-handler.js';
export { createUtilityMessageHandler } from './message-routing/utility-handler.js';
function isValidMessageSender(sender) {
    if (sender.tab?.id !== undefined && sender.tab?.url)
        return true;
    return typeof chrome !== 'undefined' && Boolean(chrome.runtime) && sender.id === chrome.runtime.id;
}
function dispatch(message, sender, sendResponse, handlers) {
    for (const owner of handlers) {
        const result = owner.handle(message, sender, sendResponse);
        if (result !== undefined)
            return result;
    }
    // Recording listeners own their separate message types.
    return false;
}
export function installMessageListener(deps) {
    if (typeof chrome === 'undefined' || !chrome.runtime)
        return;
    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
        if (!isValidMessageSender(sender)) {
            deps.debugLog('error', 'Rejected message from untrusted sender', {
                senderId: sender.id,
                senderUrl: sender.url
            });
            return false;
        }
        return dispatch(message, sender, sendResponse, deps.handlers);
    });
}
//# sourceMappingURL=message-handlers.js.map