/**
 * Purpose: Dispatches window.postMessage commands from the content script to specialized inject-context handlers (settings, state, JS execution, DOM/a11y queries).
 * Docs: docs/features/feature/interact-explore/index.md
 */
import { executeDOMQuery, runAxeAuditWithTimeout } from '../lib/dom-queries.js';
import { checkLinkHealth } from '../lib/link-health.js';
import { queryComputedStyles } from './computed-styles.js';
import { discoverForms } from './form-discovery.js';
import { extractDataTables } from './data-table.js';
import { getNetworkWaterfall } from '../lib/network.js';
import { executeJavaScript } from './execute-js.js';
import { errorMessage } from '../lib/error-utils.js';
import { isValidSettingPayload, handleSetting, handleStateCommand } from './settings.js';
// Re-export for barrel (src/inject/index.ts)
export { executeJavaScript, safeSerializeForExecute } from './execute-js.js';
/** Read the page nonce set by the content script on the inject script element */
let pageNonce = '';
if (typeof document !== 'undefined' && typeof document.querySelector === 'function') {
    const nonceEl = document.querySelector('script[data-kaboom-nonce]');
    if (nonceEl) {
        pageNonce = nonceEl.getAttribute('data-kaboom-nonce') || '';
    }
}
/** Send a nonce-authenticated response back to the content script */
function postResponse(data) {
    window.postMessage({ ...data, _nonce: pageNonce }, window.location.origin);
}
/**
 * Handle link health check request from content script
 */
async function handleLinkHealthQuery(data) {
    try {
        const params = data.params || {};
        const result = await checkLinkHealth(params);
        return result;
    }
    catch (err) {
        return {
            error: 'link_health_error',
            message: errorMessage(err, 'Failed to check link health')
        };
    }
}
/**
 * Install message listener for handling content script messages
 */
function handleLinkHealthMessage(data) {
    handleLinkHealthQuery(data)
        .then((result) => {
        postResponse({ type: 'kaboom_link_health_response', requestId: data.requestId, result });
    })
        .catch((err) => {
        postResponse({
            type: 'kaboom_link_health_response',
            requestId: data.requestId,
            result: { error: 'link_health_error', message: err.message || 'Failed to check link health' }
        });
    });
}
export function installMessageListener(captureStateFn, restoreStateFn) {
    if (typeof window === 'undefined')
        return;
    const messageHandlers = {
        kaboom_setting: (data) => {
            const settingData = data;
            if (isValidSettingPayload(settingData))
                handleSetting(settingData);
        },
        kaboom_state_command: (data) => handleStateCommand(data, captureStateFn, restoreStateFn),
        kaboom_execute_js: (data) => handleExecuteJs(data),
        kaboom_a11y_query: (data) => handleA11yQuery(data),
        kaboom_dom_query: (data) => handleDomQuery(data),
        kaboom_get_waterfall: (data) => handleGetWaterfall(data),
        kaboom_link_health_query: (data) => handleLinkHealthMessage(data),
        kaboom_computed_styles_query: (data) => handleComputedStylesMessage(data),
        kaboom_form_discovery_query: (data) => handleFormDiscoveryMessage(data),
        kaboom_form_state_query: (data) => handleFormStateMessage(data),
        kaboom_data_table_query: (data) => handleDataTableMessage(data),
        kaboom_inject_bridge_ping: (data) => handleBridgePingMessage(data)
    };
    window.addEventListener('message', (event) => {
        if (event.source !== window || event.origin !== window.location.origin)
            return;
        if (pageNonce && event.data?._nonce !== pageNonce)
            return;
        const msgType = event.data?.type;
        if (!msgType)
            return;
        const handler = messageHandlers[msgType]; // nosemgrep: unsafe-dynamic-method
        if (handler)
            handler(event.data);
    });
}
function handleBridgePingMessage(data) {
    postResponse({
        type: 'kaboom_inject_bridge_pong',
        requestId: data.requestId
    });
}
function handleComputedStylesMessage(data) {
    try {
        const params = (data.params || {});
        const result = queryComputedStyles({
            selector: params.selector || '*',
            properties: params.properties
        });
        postResponse({
            type: 'kaboom_computed_styles_response',
            requestId: data.requestId,
            result: { elements: result, count: result.length }
        });
    }
    catch (err) {
        postResponse({
            type: 'kaboom_computed_styles_response',
            requestId: data.requestId,
            result: { error: 'computed_styles_error', message: errorMessage(err, 'Failed to query computed styles') }
        });
    }
}
function handleFormDiscoveryMessage(data) {
    try {
        const params = (data.params || {});
        const result = discoverForms({
            selector: params.selector,
            mode: params.mode === 'validate' ? 'validate' : 'discover'
        });
        postResponse({
            type: 'kaboom_form_discovery_response',
            requestId: data.requestId,
            result: { forms: result, count: result.length }
        });
    }
    catch (err) {
        postResponse({
            type: 'kaboom_form_discovery_response',
            requestId: data.requestId,
            result: { error: 'form_discovery_error', message: errorMessage(err, 'Failed to discover forms') }
        });
    }
}
function handleFormStateMessage(data) {
    try {
        const params = (data.params || {});
        const forms = discoverForms({
            selector: params.selector,
            mode: 'discover'
        });
        postResponse({
            type: 'kaboom_form_state_response',
            requestId: data.requestId,
            result: { forms, count: forms.length }
        });
    }
    catch (err) {
        postResponse({
            type: 'kaboom_form_state_response',
            requestId: data.requestId,
            result: { error: 'form_state_error', message: errorMessage(err, 'Failed to extract form state') }
        });
    }
}
function handleDataTableMessage(data) {
    try {
        const params = (data.params || {});
        const result = extractDataTables({
            selector: params.selector,
            max_rows: params.max_rows,
            max_cols: params.max_cols
        });
        postResponse({
            type: 'kaboom_data_table_response',
            requestId: data.requestId,
            result
        });
    }
    catch (err) {
        postResponse({
            type: 'kaboom_data_table_response',
            requestId: data.requestId,
            result: { error: 'data_table_error', message: errorMessage(err, 'Failed to extract table data') }
        });
    }
}
function handleExecuteJs(data) {
    const { requestId, script, timeoutMs } = data;
    // Validate parameters
    if (typeof script !== 'string') {
        console.warn('[KaBOOM!] Script must be a string');
        postResponse({
            type: 'kaboom_execute_js_result',
            requestId,
            result: { success: false, error: 'invalid_script', message: 'Script must be a string' }
        });
        return;
    }
    if (typeof requestId !== 'number' && typeof requestId !== 'string') {
        console.warn('[KaBOOM!] Invalid requestId type');
        return;
    }
    executeJavaScript(script, timeoutMs)
        .then((result) => {
        postResponse({
            type: 'kaboom_execute_js_result',
            requestId,
            result
        });
    })
        .catch((err) => {
        console.error('[KaBOOM!] Failed to execute JS:', err);
        postResponse({
            type: 'kaboom_execute_js_result',
            requestId,
            result: { success: false, error: 'execution_failed', message: err.message }
        });
    });
}
function handleA11yQuery(data) {
    const { requestId, params } = data;
    if (typeof runAxeAuditWithTimeout !== 'function') {
        postResponse({
            type: 'kaboom_a11y_query_response',
            requestId,
            result: {
                error: 'runAxeAuditWithTimeout not available - try reloading the extension'
            }
        });
        return;
    }
    try {
        runAxeAuditWithTimeout(params || {})
            .then((result) => {
            postResponse({
                type: 'kaboom_a11y_query_response',
                requestId,
                result
            });
        })
            .catch((err) => {
            console.error('[KaBOOM!] Accessibility audit error:', err);
            postResponse({
                type: 'kaboom_a11y_query_response',
                requestId,
                result: { error: err.message || 'Accessibility audit failed' }
            });
        });
    }
    catch (err) {
        console.error('[KaBOOM!] Failed to run accessibility audit:', err);
        postResponse({
            type: 'kaboom_a11y_query_response',
            requestId,
            result: { error: errorMessage(err, 'Failed to run accessibility audit') }
        });
    }
}
function handleDomQuery(data) {
    const { requestId, params } = data;
    if (typeof executeDOMQuery !== 'function') {
        postResponse({
            type: 'kaboom_dom_query_response',
            requestId,
            result: {
                error: 'executeDOMQuery not available - try reloading the extension'
            }
        });
        return;
    }
    try {
        executeDOMQuery((params || {}))
            .then((result) => {
            postResponse({
                type: 'kaboom_dom_query_response',
                requestId,
                result
            });
        })
            .catch((err) => {
            console.error('[KaBOOM!] DOM query error:', err);
            postResponse({
                type: 'kaboom_dom_query_response',
                requestId,
                result: { error: err.message || 'DOM query failed' }
            });
        });
    }
    catch (err) {
        console.error('[KaBOOM!] Failed to run DOM query:', err);
        postResponse({
            type: 'kaboom_dom_query_response',
            requestId,
            result: { error: errorMessage(err, 'Failed to run DOM query') }
        });
    }
}
function handleGetWaterfall(data) {
    const { requestId } = data;
    try {
        const entries = getNetworkWaterfall({});
        postResponse({
            type: 'kaboom_waterfall_response',
            requestId,
            entries: entries || [],
            page_url: window.location.href
        });
    }
    catch (err) {
        console.error('[KaBOOM!] Failed to get network waterfall:', err);
        postResponse({
            type: 'kaboom_waterfall_response',
            requestId,
            entries: []
        });
    }
}
//# sourceMappingURL=message-handlers.js.map