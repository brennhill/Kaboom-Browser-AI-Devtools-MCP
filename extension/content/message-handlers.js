/**
 * Purpose: Handles incoming chrome.runtime messages from the background script -- pings, setting toggles, highlights, JS execution, state management, and draw mode.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import { registerHighlightRequest, hasHighlightRequest, deleteHighlightRequest, registerExecuteRequest, registerA11yRequest, registerDomRequest } from './request-tracking.js';
import { createDeferredPromise, withTimeoutAndCleanup } from '../lib/timeout-utils.js';
import { isInjectScriptLoaded, getPageNonce, ensureInjectBridgeReady } from './script-injection.js';
import { ASYNC_COMMAND_TIMEOUT_MS, INJECT_FORWARDED_SETTINGS, SettingName } from '../lib/constants.js';
import { extractReadable as extractReadableContent } from './extractors/readable.js';
import { extractMarkdown as extractMarkdownContent } from './extractors/markdown.js';
import { extractPageSummary as extractPageSummaryContent } from './extractors/page-summary.js';
import { errorMessage } from '../lib/error-utils.js';
/** Auto-incrementing request ID — avoids Date.now() collisions for concurrent queries */
let nextRequestId = 1;
/** Parse query params from string (JSON) or object form into a plain object */
function parseQueryParams(params) {
    if (typeof params === 'string') {
        try {
            return JSON.parse(params);
        }
        catch {
            return {};
        }
    }
    return typeof params === 'object' ? params : {};
}
/** Send a nonce-authenticated message to inject.js (MAIN world) */
function postToInject(data) {
    window.postMessage({ ...data, _nonce: getPageNonce() }, window.location.origin);
}
// Feature toggle message types forwarded from background to inject.js — imported from canonical constants.
export const TOGGLE_MESSAGES = INJECT_FORWARDED_SETTINGS;
/**
 * Security: Validate sender is from the extension background script
 * Prevents content script from trusting messages from compromised page context
 */
export function isValidBackgroundSender(sender) {
    // Messages from background should NOT have a tab (or have tab with chrome-extension:// url)
    // Messages from content scripts have tab.id
    // We only want messages from the background service worker
    return typeof sender.id === 'string' && sender.id === chrome.runtime.id;
}
/**
 * Forward a highlight message from background to inject.js
 */
export function forwardHighlightMessage(message) {
    return ensureInjectBridgeReady(1500).then((ready) => {
        if (!ready) {
            return {
                success: false,
                error: isInjectScriptLoaded() ? 'inject_not_responding' : 'inject_not_loaded'
            };
        }
        const deferred = createDeferredPromise();
        const requestId = registerHighlightRequest((result) => deferred.resolve(result), () => deferred.resolve({ success: false, error: 'page_unloaded' }));
        // Post message to page context (inject.js)
        postToInject({
            type: 'kaboom_highlight_request',
            requestId,
            params: message.params
        });
        // Timeout fallback + cleanup stale entries after 30 seconds
        return withTimeoutAndCleanup(deferred.promise, 30000, {
            fallback: { success: false, error: 'timeout' },
            cleanup: () => {
                if (hasHighlightRequest(requestId)) {
                    deleteHighlightRequest(requestId);
                }
            }
        });
    });
}
/**
 * Handle state capture/restore commands
 */
export async function handleStateCommand(params) {
    const { action, name, state, include_url } = params || {};
    // Create a promise to receive response from inject.js
    const messageId = `state_${Date.now()}_${Math.random().toString(36).slice(2)}`;
    const deferred = createDeferredPromise();
    // Set up listener for response from inject.js
    const responseHandler = (event) => {
        if (event.source !== window)
            return;
        if (event.data?._nonce !== getPageNonce())
            return;
        if (event.data?.type === 'kaboom_state_response' && event.data?.messageId === messageId) {
            window.removeEventListener('message', responseHandler);
            deferred.resolve(event.data.result || { error: 'No result from state command' });
        }
    };
    window.addEventListener('message', responseHandler);
    // Send command to inject.js (include state for restore action)
    postToInject({
        type: 'kaboom_state_command',
        messageId,
        action,
        name,
        state,
        include_url
    });
    // Timeout after 5 seconds with cleanup
    return withTimeoutAndCleanup(deferred.promise, 5000, {
        fallback: { error: 'State command timeout' },
        cleanup: () => window.removeEventListener('message', responseHandler)
    });
}
/**
 * Handle KABOOM_PING message
 */
export function handlePing(sendResponse) {
    sendResponse({ status: 'alive', timestamp: Date.now() });
    return true;
}
/**
 * Handle toggle messages
 */
export function handleToggleMessage(message) {
    if (!TOGGLE_MESSAGES.has(message.type))
        return;
    const payload = { type: 'kaboom_setting', setting: message.type };
    if (message.type === SettingName.WEBSOCKET_CAPTURE_MODE) {
        payload.mode = message.mode;
    }
    else if (message.type === SettingName.SERVER_URL) {
        payload.url = message.url;
    }
    else {
        payload.enabled = message.enabled;
    }
    // SECURITY: Use explicit targetOrigin (window.location.origin) not "*"
    window.postMessage({ ...payload, _nonce: getPageNonce() }, window.location.origin);
}
/**
 * Execute JS in the MAIN world via inject script, with safety timeout.
 */
function executeInMainWorld(params, sendResponse) {
    const timeoutMs = params.timeout_ms || 5000;
    // Safety timeout: user's timeout + 2s buffer (NOT fixed 30s)
    // If inject script responds, its own timeout handles slow scripts.
    // This only fires if inject script never responds at all.
    const safetyTimeoutMs = timeoutMs + 2000;
    const requestId = registerExecuteRequest(sendResponse, safetyTimeoutMs, () => {
        sendResponse({
            success: false,
            error: 'inject_not_responding',
            message: `Inject script did not respond within ${safetyTimeoutMs}ms. The tab may not be tracked or the inject script failed to load.`
        });
    });
    postToInject({
        type: 'kaboom_execute_js',
        requestId,
        script: params.script || '',
        timeoutMs
    });
}
/**
 * Handle kaboom_execute_js message.
 * Always executes in MAIN world via inject script.
 * Returns inject_not_loaded error if inject script isn't available,
 * so background can fallback to chrome.scripting API.
 */
export function handleExecuteJs(params, sendResponse) {
    const injectReadyWaitMs = Math.max(750, Math.min(3000, (params.timeout_ms || 5000) + 500));
    void ensureInjectBridgeReady(injectReadyWaitMs).then((ready) => {
        if (!ready) {
            const fallbackError = isInjectScriptLoaded() ? 'inject_not_responding' : 'inject_not_loaded';
            sendResponse({
                success: false,
                error: fallbackError,
                message: fallbackError === 'inject_not_loaded'
                    ? 'Inject script not loaded in page context. Tab may not be tracked.'
                    : `Inject script did not respond within ${injectReadyWaitMs}ms. The tab may not be tracked or the inject script failed to load.`
            });
            return;
        }
        executeInMainWorld(params, sendResponse);
    });
    return true;
}
/**
 * Handle KABOOM_EXECUTE_QUERY message (async command path)
 */
export function handleExecuteQuery(params, sendResponse) {
    let parsedParams = {};
    if (typeof params === 'string') {
        try {
            parsedParams = JSON.parse(params);
        }
        catch {
            parsedParams = {};
        }
    }
    else if (typeof params === 'object') {
        parsedParams = params;
    }
    return handleExecuteJs(parsedParams, sendResponse);
}
/**
 * Handle A11Y_QUERY message
 */
export function handleA11yQuery(params, sendResponse) {
    const parsedParams = parseQueryParams(params);
    const requestId = registerA11yRequest(sendResponse, ASYNC_COMMAND_TIMEOUT_MS, () => {
        sendResponse({ error: 'Accessibility audit timeout' });
    });
    // Forward to inject.js via postMessage
    postToInject({
        type: 'kaboom_a11y_query',
        requestId,
        params: parsedParams
    });
    return true;
}
/**
 * Handle DOM_QUERY message
 */
export function handleDomQuery(params, sendResponse) {
    const parsedParams = parseQueryParams(params);
    const requestId = registerDomRequest(sendResponse, ASYNC_COMMAND_TIMEOUT_MS, () => {
        sendResponse({ error: 'DOM query timeout' });
    });
    // Forward to inject.js via postMessage
    postToInject({
        type: 'kaboom_dom_query',
        requestId,
        params: parsedParams
    });
    return true;
}
const waitForWaterfallResponse = (response, cleanup) => withTimeoutAndCleanup(response, 5000, { cleanup });
function reportWaterfallBridgeLifecycle(lifecycle, requestId, reason = '') {
    const diagnostic = lifecycle === 'active'
        ? {
            name: 'waterfall_bridge',
            detail: `Injected waterfall bridge request ${requestId} failed: ${reason}.`,
            fix: 'Reload the tracked page and retry; include System Doctor output if the bridge repeatedly fails.'
        }
        : { name: 'waterfall_bridge', detail: '', fix: '' };
    try {
        void chrome.runtime
            .sendMessage({ type: 'report_state_recovery', lifecycle, diagnostic })
            .catch((error) => console.warn('[KaBOOM!] Waterfall bridge diagnostic delivery failed:', errorMessage(error)));
    }
    catch (error) {
        console.warn('[KaBOOM!] Waterfall bridge diagnostic delivery failed:', errorMessage(error));
    }
}
export function handleGetNetworkWaterfall(sendResponse, waitForResponse = waitForWaterfallResponse) {
    const requestId = nextRequestId++;
    const deferred = createDeferredPromise();
    // Set up a one-time listener for the response — match requestId to prevent cross-wiring
    const responseHandler = (event) => {
        if (event.source !== window)
            return;
        const nonce = event.data?._nonce;
        if (nonce !== getPageNonce())
            return;
        if (event.data?.type === 'kaboom_waterfall_response' && event.data?.requestId === requestId) {
            window.removeEventListener('message', responseHandler);
            reportWaterfallBridgeLifecycle('recovered', requestId);
            deferred.resolve({ entries: event.data.entries || [] });
        }
    };
    window.addEventListener('message', responseHandler);
    // Post message to page context
    try {
        postToInject({
            type: 'kaboom_get_waterfall',
            requestId
        });
    }
    catch {
        window.removeEventListener('message', responseHandler);
        reportWaterfallBridgeLifecycle('active', requestId, 'request dispatch failed');
        sendResponse({
            entries: [],
            error: 'waterfall_bridge_failed',
            message: 'Failed to dispatch the injected waterfall request.'
        });
        return true;
    }
    // Timeout is a bridge failure, not evidence that the page has no requests.
    waitForResponse(deferred.promise, () => window.removeEventListener('message', responseHandler)).then((result) => {
        sendResponse(result);
    }, (error) => {
        window.removeEventListener('message', responseHandler);
        const timedOut = error instanceof Error && error.name === 'TimeoutError';
        reportWaterfallBridgeLifecycle('active', requestId, timedOut ? 'response timeout' : 'response rejected');
        sendResponse({
            entries: [],
            error: timedOut ? 'waterfall_bridge_timeout' : 'waterfall_bridge_failed',
            message: timedOut
                ? 'Injected waterfall bridge did not respond before the deadline.'
                : 'Injected waterfall bridge rejected the response wait.'
        });
    });
    return true;
}
/**
 * Generic inject-query forwarder: parse params, post to inject, wait for response with timeout.
 * Consolidates the identical pattern used by computed_styles, form_discovery, and link_health.
 */
function forwardInjectQuery(queryType, responseType, label, params, sendResponse) {
    const parsedParams = parseQueryParams(params);
    const requestId = nextRequestId++;
    const deferred = createDeferredPromise();
    const responseHandler = (event) => {
        if (event.source !== window)
            return;
        const nonce = event.data?._nonce;
        if (nonce !== getPageNonce())
            return;
        if (event.data?.type === responseType && event.data?.requestId === requestId) {
            window.removeEventListener('message', responseHandler);
            deferred.resolve(event.data.result || { error: `No result from ${label}` });
        }
    };
    window.addEventListener('message', responseHandler);
    postToInject({ type: queryType, requestId, params: parsedParams });
    withTimeoutAndCleanup(deferred.promise, ASYNC_COMMAND_TIMEOUT_MS, {
        fallback: { error: `${label} timeout` },
        cleanup: () => window.removeEventListener('message', responseHandler)
    }).then((result) => sendResponse(result), () => sendResponse({ error: `${label} failed` }));
    return true;
}
export function handleComputedStylesQuery(params, sendResponse) {
    return forwardInjectQuery('kaboom_computed_styles_query', 'kaboom_computed_styles_response', 'Computed styles query', params, sendResponse);
}
export function handleFormDiscoveryQuery(params, sendResponse) {
    return forwardInjectQuery('kaboom_form_discovery_query', 'kaboom_form_discovery_response', 'Form discovery', params, sendResponse);
}
export function handleFormStateQuery(params, sendResponse) {
    return forwardInjectQuery('kaboom_form_state_query', 'kaboom_form_state_response', 'Form state', params, sendResponse);
}
export function handleDataTableQuery(params, sendResponse) {
    return forwardInjectQuery('kaboom_data_table_query', 'kaboom_data_table_response', 'Data table extraction', params, sendResponse);
}
export function handleLinkHealthQuery(params, sendResponse) {
    return forwardInjectQuery('kaboom_link_health_query', 'kaboom_link_health_response', 'Link health check', params, sendResponse);
}
// ============================================
// Content-Script-Native Extractors (ISOLATED world, CSP-safe)
// Issue #257: These run directly in the content script — no inject bridge needed.
// ============================================
/**
 * Handle GET_READABLE message — extract readable content directly in ISOLATED world.
 */
export function handleGetReadable(sendResponse) {
    try {
        sendResponse(extractReadableContent());
    }
    catch (err) {
        sendResponse({ error: 'get_readable_failed', message: errorMessage(err, 'Readable extraction failed') });
    }
    // Synchronous — sendResponse called inline, no async channel needed.
    return false;
}
/**
 * Handle GET_MARKDOWN message — extract markdown content directly in ISOLATED world.
 */
export function handleGetMarkdown(sendResponse) {
    try {
        sendResponse(extractMarkdownContent());
    }
    catch (err) {
        sendResponse({ error: 'get_markdown_failed', message: errorMessage(err, 'Markdown extraction failed') });
    }
    // Synchronous — sendResponse called inline, no async channel needed.
    return false;
}
/**
 * Handle PAGE_SUMMARY message — extract page summary directly in ISOLATED world.
 */
export function handlePageSummary(sendResponse) {
    try {
        sendResponse(extractPageSummaryContent());
    }
    catch (err) {
        sendResponse({ error: 'page_summary_failed', message: errorMessage(err, 'Page summary extraction failed') });
    }
    // Synchronous — sendResponse called inline, no async channel needed.
    return false;
}
//# sourceMappingURL=message-handlers.js.map