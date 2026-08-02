import { errorMessage } from '../lib/error-utils.js';
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
        try {
            return dispatch(message, sender, sendResponse, deps.handlers);
        }
        catch (error) {
            const correlationId = `runtime_message_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
            deps.debugLog('error', 'Runtime message handler failed', {
                correlation_id: correlationId,
                message_type: message.type,
                error: errorMessage(error)
            });
            sendResponse({ success: false, error: 'message_handler_failed' });
            return false;
        }
    });
}
//# sourceMappingURL=message-handlers.js.map