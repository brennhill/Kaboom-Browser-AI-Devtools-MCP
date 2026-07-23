/**
 * Purpose: Handles browser navigation actions (navigate, refresh, back, forward, tab management) with CSP probing and async timeouts.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import { waitForTabLoad, pingContentScript, getActiveTab } from './event-listeners.js';
import { debugLog } from './index.js';
import { isAiWebPilotEnabled } from './state.js';
import { DebugCategory } from './debug.js';
import { broadcastTrackingState } from './message-handlers.js';
import { executeWithWorldRouting, probeCSPStatus } from './query-execution.js';
import { ASYNC_COMMAND_TIMEOUT_MS } from '../lib/constants.js';
import { persistTrackedTab } from './commands/helpers.js';
import { errorMessage } from '../lib/error-utils.js';
import { delay } from '../lib/timeout-utils.js';
// =============================================================================
// TIMEOUT CONFIGURATION
// =============================================================================
const ASYNC_EXECUTE_TIMEOUT_MS = ASYNC_COMMAND_TIMEOUT_MS;
const ASYNC_BROWSER_ACTION_TIMEOUT_MS = ASYNC_COMMAND_TIMEOUT_MS;
/**
 * Race a promise against a timeout. Properly clears the timer when the promise
 * settles first so no dangling setTimeout keeps the service worker alive.
 */
function withTimeout(promise, timeoutMs, message) {
    return new Promise((resolve, reject) => {
        const timer = setTimeout(() => reject(new Error(message)), timeoutMs);
        promise.then((value) => { clearTimeout(timer); resolve(value); }, (err) => { clearTimeout(timer); reject(err); });
    });
}
/** Cached CSP status from the most recent navigation */
let lastCSPStatus = { csp_restricted: false, csp_level: 'none' };
/** Get the CSP status from the most recent navigation (for sync layer) */
export function getLastCSPStatus() {
    return lastCSPStatus;
}
// =============================================================================
// NAVIGATION
// =============================================================================
/** Probe CSP status and enrich a BrowserActionResult with csp_restricted/csp_level */
async function enrichWithCSP(tabId, result) {
    try {
        const csp = await probeCSPStatus(tabId);
        lastCSPStatus = csp;
        return { ...result, csp_restricted: csp.csp_restricted, csp_level: csp.csp_level };
    }
    catch {
        return result;
    }
}
// #lizard forgives
async function handleNavigateAction(tabId, url, actionToast, reason) {
    if (url.startsWith('chrome://') || url.startsWith('chrome-extension://')) {
        return { success: false, error: 'restricted_url', message: 'Cannot navigate to Chrome internal pages' };
    }
    actionToast(tabId, reason || 'navigate', reason ? undefined : url, 'trying', 10000);
    await chrome.tabs.update(tabId, { url });
    await waitForTabLoad(tabId);
    await delay(500);
    const tab = await chrome.tabs.get(tabId);
    if (await pingContentScript(tabId)) {
        broadcastTrackingState().catch(() => { });
        actionToast(tabId, reason || 'navigate', reason ? undefined : url, 'success');
        return enrichWithCSP(tabId, {
            success: true,
            action: 'navigate',
            url,
            final_url: tab.url,
            title: tab.title,
            content_script_status: 'loaded',
            message: 'Content script ready'
        });
    }
    if (tab.url?.startsWith('file://')) {
        return {
            success: true,
            action: 'navigate',
            url,
            final_url: tab.url,
            title: tab.title,
            content_script_status: 'unavailable',
            message: 'Content script cannot load on file:// URLs. Enable "Allow access to file URLs" in extension settings.'
        };
    }
    debugLog(DebugCategory.CAPTURE, 'Content script not loaded after navigate, refreshing', { tabId, url });
    await chrome.tabs.reload(tabId);
    await waitForTabLoad(tabId);
    await delay(1000);
    const reloadedTab = await chrome.tabs.get(tabId);
    if (await pingContentScript(tabId)) {
        broadcastTrackingState().catch(() => { });
        return enrichWithCSP(tabId, {
            success: true,
            action: 'navigate',
            url,
            final_url: reloadedTab.url,
            title: reloadedTab.title,
            content_script_status: 'refreshed',
            message: 'Page refreshed to load content script'
        });
    }
    return enrichWithCSP(tabId, {
        success: true,
        action: 'navigate',
        url,
        final_url: reloadedTab.url,
        title: reloadedTab.title,
        content_script_status: 'failed',
        message: 'Navigation complete but content script could not be loaded. AI Web Pilot tools may not work.'
    });
}
async function handleNewTabAction(tabId, url, actionToast, reason) {
    if (!url)
        return { success: false, error: 'missing_url', message: 'URL required for new_tab action' };
    actionToast(tabId, reason || 'new_tab', reason ? undefined : 'opening new tab', 'trying', 5000);
    const newTab = await chrome.tabs.create({ url, active: false });
    actionToast(tabId, reason || 'new_tab', undefined, 'success');
    return {
        success: true,
        action: 'new_tab',
        url,
        tab_id: newTab.id,
        tab_index: typeof newTab.index === 'number' ? newTab.index : undefined,
        title: newTab.title
    };
}
function coerceNonNegativeInt(value) {
    if (typeof value !== 'number' || !Number.isInteger(value) || value < 0)
        return null;
    return value;
}
// =============================================================================
// BROWSER ACTION DISPATCH
// =============================================================================
export async function handleBrowserAction(tabId, params, actionToast) {
    const { url, reason } = params || {};
    const action = typeof params?.action === 'string' && params.action.trim() !== ''
        ? params.action
        : typeof params?.what === 'string'
            ? params.what
            : undefined;
    if (!isAiWebPilotEnabled()) {
        return { success: false, error: 'ai_web_pilot_disabled', message: 'AI Web Pilot is not enabled' };
    }
    try {
        switch (action) {
            case 'refresh': {
                actionToast(tabId, reason || 'refresh', reason ? undefined : 'reloading page', 'trying', 10000);
                await chrome.tabs.reload(tabId);
                await waitForTabLoad(tabId);
                actionToast(tabId, reason || 'refresh', undefined, 'success');
                const refreshedTab = await chrome.tabs.get(tabId);
                return enrichWithCSP(tabId, {
                    success: true,
                    action: 'refresh',
                    url: refreshedTab.url,
                    title: refreshedTab.title
                });
            }
            case 'navigate':
                if (!url)
                    return { success: false, error: 'missing_url', message: 'URL required for navigate action' };
                if (params?.new_tab) {
                    return handleNewTabAction(tabId, url, actionToast, reason || 'navigate');
                }
                return handleNavigateAction(tabId, url, actionToast, reason);
            case 'back': {
                actionToast(tabId, reason || 'back', reason ? undefined : 'going back', 'trying', 10000);
                await chrome.tabs.goBack(tabId);
                await waitForTabLoad(tabId);
                actionToast(tabId, reason || 'back', undefined, 'success');
                const backTab = await chrome.tabs.get(tabId);
                return { success: true, action: 'back', url: backTab.url, title: backTab.title };
            }
            case 'forward': {
                actionToast(tabId, reason || 'forward', reason ? undefined : 'going forward', 'trying', 10000);
                await chrome.tabs.goForward(tabId);
                await waitForTabLoad(tabId);
                actionToast(tabId, reason || 'forward', undefined, 'success');
                const fwdTab = await chrome.tabs.get(tabId);
                return { success: true, action: 'forward', url: fwdTab.url, title: fwdTab.title };
            }
            case 'new_tab': {
                return handleNewTabAction(tabId, url || '', actionToast, reason);
            }
            case 'switch_tab': {
                const requestedTabID = coerceNonNegativeInt(params?.tab_id);
                const requestedTabIndex = coerceNonNegativeInt(params?.tab_index);
                if (requestedTabID === null && requestedTabIndex === null) {
                    return {
                        success: false,
                        error: 'missing_tab_target',
                        message: "switch_tab requires 'tab_id' or 'tab_index'"
                    };
                }
                let targetTab = null;
                if (requestedTabID !== null) {
                    targetTab = await chrome.tabs.get(requestedTabID);
                }
                else {
                    const tabs = await chrome.tabs.query({ currentWindow: true });
                    const sortable = tabs.filter((tab) => typeof tab.id === 'number');
                    sortable.sort((a, b) => (a.index ?? 0) - (b.index ?? 0));
                    targetTab = sortable[requestedTabIndex] || null;
                }
                if (!targetTab?.id) {
                    return {
                        success: false,
                        error: 'tab_not_found',
                        message: 'No matching tab found for switch_tab request'
                    };
                }
                const updated = await chrome.tabs.update(targetTab.id, { active: true });
                const activeTab = updated || targetTab;
                // Persist tracked tab so the extension-side state matches the server-side
                // update (issue #271). This ensures subsequent /sync heartbeats report
                // the correct tracked tab.
                await persistTrackedTab(activeTab);
                broadcastTrackingState().catch(() => { });
                return {
                    success: true,
                    action: 'switch_tab',
                    tab_id: activeTab.id || targetTab.id,
                    tab_index: typeof activeTab.index === 'number' ? activeTab.index : targetTab.index,
                    url: activeTab.url || targetTab.url,
                    title: activeTab.title || targetTab.title
                };
            }
            case 'activate_tab': {
                actionToast(tabId, reason || 'activate_tab', reason ? undefined : 'bringing tab to foreground', 'trying', 5000);
                await chrome.tabs.update(tabId, { active: true });
                // Also focus the window containing this tab
                const tab = await chrome.tabs.get(tabId);
                if (tab.windowId) {
                    await chrome.windows.update(tab.windowId, { focused: true });
                }
                actionToast(tabId, reason || 'activate_tab', undefined, 'success');
                return {
                    success: true,
                    action: 'activate_tab',
                    tab_id: tabId,
                    url: tab.url,
                    title: tab.title
                };
            }
            case 'close_tab': {
                const requestedTabID = coerceNonNegativeInt(params?.tab_id);
                const targetTabID = requestedTabID !== null ? requestedTabID : tabId;
                if (!targetTabID || targetTabID < 0) {
                    return {
                        success: false,
                        error: 'missing_tab_target',
                        message: "close_tab requires a valid 'tab_id' or resolved tab context"
                    };
                }
                await chrome.tabs.remove(targetTabID);
                const activeTab = await getActiveTab();
                return {
                    success: true,
                    action: 'close_tab',
                    closed_tab_id: targetTabID,
                    tab_id: activeTab?.id,
                    url: activeTab?.url,
                    title: activeTab?.title
                };
            }
            default:
                return { success: false, error: 'unknown_action', message: `Unknown action: ${action}` };
        }
    }
    catch (err) {
        return { success: false, error: 'browser_action_failed', message: errorMessage(err) };
    }
}
// =============================================================================
// ASYNC EXECUTE COMMAND
// =============================================================================
export async function handleAsyncExecuteCommand(query, tabId, world, syncClient, sendAsyncResult, actionToast) {
    const startTime = Date.now();
    if (!isAiWebPilotEnabled()) {
        sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', {
            success: false,
            error: 'ai_web_pilot_disabled',
            message: 'AI Web Pilot is not enabled'
        }, 'ai_web_pilot_disabled');
        return;
    }
    // Extract reason for toast display
    let reason;
    try {
        const p = typeof query.params === 'string' ? JSON.parse(query.params) : query.params;
        reason = p?.reason;
    }
    catch {
        /* ignore parse errors */
    }
    try {
        const result = await withTimeout(executeWithWorldRouting(tabId, query.params, world), ASYNC_EXECUTE_TIMEOUT_MS, `Script execution timed out after ${ASYNC_EXECUTE_TIMEOUT_MS}ms. Script may be stuck in a loop or waiting for user input.`);
        if (result.success) {
            actionToast(tabId, reason || 'execute_js', undefined, 'success');
        }
        let enrichedResult = result;
        try {
            const tab = await chrome.tabs.get(tabId);
            enrichedResult = { ...result, effective_tab_id: tabId, effective_url: tab.url, effective_title: tab.title };
        }
        catch {
            /* tab may have closed */
        }
        const status = result.success ? 'complete' : 'error';
        const error = result.success ? undefined : result.error || result.message || 'execution_failed';
        sendAsyncResult(syncClient, query.id, query.correlation_id, status, enrichedResult, error);
        debugLog(DebugCategory.CONNECTION, 'Completed async command', {
            correlationId: query.correlation_id,
            elapsed: Date.now() - startTime,
            success: result.success
        });
    }
    catch {
        const timeoutMessage = `JavaScript execution exceeded ${ASYNC_EXECUTE_TIMEOUT_MS / 1000}s timeout. RECOMMENDED ACTIONS:

1. Break your task into smaller discrete steps that execute in < ${ASYNC_EXECUTE_TIMEOUT_MS / 1000}s
2. Check your script for infinite loops or blocking operations
3. Simplify the operation or target a smaller DOM scope`;
        sendAsyncResult(syncClient, query.id, query.correlation_id, 'timeout', null, timeoutMessage);
        debugLog(DebugCategory.CONNECTION, 'Async command timeout', {
            correlationId: query.correlation_id,
            elapsed: Date.now() - startTime
        });
    }
}
// =============================================================================
// ASYNC BROWSER ACTION
// =============================================================================
function isCSPFailure(errorCode, message) {
    const haystack = `${errorCode || ''} ${message || ''}`.toLowerCase();
    if (!haystack)
        return false;
    return (haystack.includes('csp') ||
        haystack.includes('content script') ||
        haystack.includes('blocked') ||
        haystack.includes('chrome://') ||
        haystack.includes('extension://'));
}
function enrichCSPFailure(result) {
    if (!isCSPFailure(result.error, result.message)) {
        return result;
    }
    return {
        ...result,
        csp_blocked: true,
        failure_cause: 'csp'
    };
}
export async function handleAsyncBrowserAction(query, tabId, params, syncClient, sendAsyncResult, actionToast) {
    const startTime = Date.now();
    const executionPromise = handleBrowserAction(tabId, params, actionToast)
        .then((result) => {
        return result;
    })
        .catch((err) => {
        return {
            success: false,
            error: err.message || 'Browser action failed'
        };
    });
    try {
        const execResult = await withTimeout(executionPromise, ASYNC_BROWSER_ACTION_TIMEOUT_MS, `Browser action execution timed out after ${ASYNC_BROWSER_ACTION_TIMEOUT_MS}ms. Action may be waiting for user interaction or network response.`);
        if (execResult.success !== false) {
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', execResult);
        }
        else {
            const enrichedFailure = enrichCSPFailure(execResult);
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', enrichedFailure, enrichedFailure.error || 'browser_action_failed');
        }
        debugLog(DebugCategory.CONNECTION, 'Completed async browser action', {
            correlationId: query.correlation_id,
            elapsed: Date.now() - startTime,
            success: execResult.success !== false
        });
    }
    catch {
        // nosemgrep: missing-template-string-indicator
        const timeoutMessage = `Browser action exceeded ${ASYNC_BROWSER_ACTION_TIMEOUT_MS / 1000}s timeout. DIAGNOSTIC STEPS:

1. Check page status: observe({what: 'page'})
2. Check for console errors: observe({what: 'errors'})
3. Check network requests: observe({what: 'network_waterfall', status_min: 400})`;
        sendAsyncResult(syncClient, query.id, query.correlation_id, 'timeout', null, timeoutMessage);
        debugLog(DebugCategory.CONNECTION, 'Async browser action timeout', {
            correlationId: query.correlation_id,
            elapsed: Date.now() - startTime
        });
    }
}
//# sourceMappingURL=browser-actions.js.map