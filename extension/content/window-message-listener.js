/**
 * Purpose: Listens for window.postMessage events from inject.js and resolves pending request promises or forwards telemetry to the background.
 * Docs: docs/features/feature/observe/index.md
 */
import { resolveHighlightRequest, resolveExecuteRequest, resolveA11yRequest, resolveDomRequest } from './request-tracking.js';
import { MESSAGE_MAP, safeSendMessage } from './message-forwarding.js';
import { getIsTrackedTab, getCurrentTabId } from './tab-tracking.js';
import { getPageNonce } from './script-injection.js';
import { validatePageTelemetry } from './page-telemetry.js';
const reportedTelemetryRejections = new Set();
function reportTelemetryRejection(reason) {
    if (reportedTelemetryRejections.has(reason))
        return;
    reportedTelemetryRejections.add(reason);
    safeSendMessage({
        type: 'capture_diagnostic',
        payload: {
            category: 'page_telemetry_validation',
            message: 'Authenticated page telemetry was rejected before extension ingestion.',
            error_type: reason
        },
        tabId: getCurrentTabId() ?? undefined
    });
}
const RESPONSE_HANDLERS = {
    kaboom_highlight_response: (id, result) => resolveHighlightRequest(id, result),
    kaboom_execute_js_result: (id, result) => resolveExecuteRequest(id, result),
    kaboom_a11y_query_response: (id, result) => resolveA11yRequest(id, result),
    kaboom_dom_query_response: (id, result) => resolveDomRequest(id, result)
};
function handlePageResponse(data, requestId, result, handler) {
    if (data._nonce !== getPageNonce())
        return;
    if (requestId !== undefined)
        handler(requestId, result);
}
function forwardTelemetryMessage(messageType, payload) {
    if (!messageType ||
        !Object.prototype.hasOwnProperty.call(MESSAGE_MAP, messageType) ||
        !payload ||
        typeof payload !== 'object')
        return;
    const mappedType = MESSAGE_MAP[messageType];
    if (!mappedType)
        return;
    const rejection = validatePageTelemetry(messageType, payload);
    if (rejection) {
        reportTelemetryRejection(rejection);
        return;
    }
    safeSendMessage({
        type: mappedType,
        payload,
        tabId: getCurrentTabId()
    });
}
function onWindowMessage(event) {
    if (event.source !== window || event.origin !== window.location.origin)
        return;
    const { type: messageType, requestId, result, payload } = event.data || {};
    const responseHandler = messageType && Object.prototype.hasOwnProperty.call(RESPONSE_HANDLERS, messageType)
        ? RESPONSE_HANDLERS[messageType]
        : undefined;
    if (responseHandler) {
        handlePageResponse(event.data, requestId, result, responseHandler);
        return;
    }
    // Tab isolation filter: only forward captured data from the tracked tab.
    // Response messages (highlight, execute JS, a11y) are NOT filtered because
    // they are responses to explicit commands from the background script.
    if (!getIsTrackedTab())
        return;
    if (event.data._nonce !== getPageNonce())
        return;
    forwardTelemetryMessage(messageType, payload);
}
export function initWindowMessageListener() {
    window.addEventListener('message', onWindowMessage);
}
//# sourceMappingURL=window-message-listener.js.map