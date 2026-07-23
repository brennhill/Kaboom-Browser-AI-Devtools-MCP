/**
 * Purpose: Listens for window.postMessage events from inject.js and resolves pending request promises or forwards telemetry to the background.
 * Docs: docs/features/feature/observe/index.md
 */
import { resolveHighlightRequest, resolveExecuteRequest, resolveA11yRequest, resolveDomRequest } from './request-tracking.js';
import { MESSAGE_MAP, safeSendMessage } from './message-forwarding.js';
import { getIsTrackedTab, getCurrentTabId } from './tab-tracking.js';
import { getPageNonce } from './script-injection.js';
const RESPONSE_HANDLERS = {
    kaboom_highlight_response: (id, result) => resolveHighlightRequest(id, result),
    kaboom_execute_js_result: (id, result) => resolveExecuteRequest(id, result),
    kaboom_a11y_query_response: (id, result) => resolveA11yRequest(id, result),
    kaboom_dom_query_response: (id, result) => resolveDomRequest(id, result)
};
export function initWindowMessageListener() {
    window.addEventListener('message', (event) => {
        if (event.source !== window || event.origin !== window.location.origin)
            return;
        const { type: messageType, requestId, result, payload } = event.data || {};
        const responseHandler = messageType ? RESPONSE_HANDLERS[messageType] : undefined;
        if (responseHandler) {
            // Validate nonce on response messages (spoofing prevention).
            // Accept responses with no nonce for backwards compat during migration.
            const nonce = event.data?._nonce;
            if (nonce && nonce !== getPageNonce())
                return;
            if (requestId !== undefined)
                responseHandler(requestId, result);
            return;
        }
        // Tab isolation filter: only forward captured data from the tracked tab.
        // Response messages (highlight, execute JS, a11y) are NOT filtered because
        // they are responses to explicit commands from the background script.
        if (!getIsTrackedTab())
            return;
        if (messageType && messageType in MESSAGE_MAP && payload && typeof payload === 'object') {
            const mappedType = MESSAGE_MAP[messageType];
            if (mappedType) {
                safeSendMessage({
                    type: mappedType,
                    payload,
                    tabId: getCurrentTabId()
                });
            }
        }
    });
}
//# sourceMappingURL=window-message-listener.js.map