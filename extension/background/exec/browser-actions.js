/**
 * Purpose: Handles browser navigation actions (navigate, refresh, back, forward, tab management) with CSP probing and async timeouts.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import { waitForTabLoad, getActiveTab } from '../ui/tracked-tab-state.js';
import { debugLog, DebugCategory } from '../debug.js';
import { isAiWebPilotEnabled } from '../runtime-state/pilot-state.js';
import { broadcastTrackingState } from '../message-routing/pilot-handler.js';
import { setLastCSPStatus } from '../runtime-state/csp-state.js';
import { executeWithWorldRouting, probeCSPStatus } from './query-execution.js';
import { ASYNC_COMMAND_TIMEOUT_MS } from '../../lib/constants.js';
import { persistTrackedTab } from '../commands/helpers.js';
import { errorMessage } from '../../lib/error-utils.js';
import { focusTabAndWindow } from '../../lib/tabs/tab-focus.js';
import { contentReadiness } from '../runtime-state/content-readiness.js';
import { delay } from '../../lib/timeout-utils.js';
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
        promise.then((value) => {
            clearTimeout(timer);
            resolve(value);
        }, (err) => {
            clearTimeout(timer);
            reject(err);
        });
    });
}
// =============================================================================
// NAVIGATION
// =============================================================================
/** Probe CSP status and enrich a BrowserActionResult with csp_restricted/csp_level */
async function enrichWithCSP(tabId, result) {
    try {
        const csp = await probeCSPStatus(tabId);
        setLastCSPStatus(csp);
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
    const tab = await chrome.tabs.get(tabId);
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
    actionToast(tabId, reason || 'navigate', reason ? undefined : url, 'success');
    return enrichWithCSP(tabId, {
        success: true,
        action: 'navigate',
        url,
        final_url: tab.url,
        title: tab.title,
        content_script_status: 'pending',
        message: 'Navigation complete; content readiness will be verified before the next page command.'
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
const HISTORY_FALLBACK_DELAYS_MS = [50, 100, 200, 400, 800, 800, 800, 800];
const HISTORY_API_REJECTION = /Cannot find a (?:next|previous) page in history/i;
async function navigateHistory(tabId, offset, correlationId) {
    const initialTab = await chrome.tabs.get(tabId);
    try {
        if (offset === -1)
            await chrome.tabs.goBack(tabId);
        else
            await chrome.tabs.goForward(tabId);
    }
    catch (err) {
        const message = errorMessage(err);
        if (!HISTORY_API_REJECTION.test(message))
            throw err;
        debugLog(DebugCategory.CONNECTION, 'Chrome history API rejected a valid transition; trying page history', {
            tab_id: tabId,
            correlation_id: correlationId,
            direction: offset === -1 ? 'back' : 'forward',
            browser_error: message
        });
        const injected = await chrome.scripting.executeScript({
            target: { tabId },
            func: (historyOffset) => {
                const historyLength = window.history.length;
                window.history.go(historyOffset);
                return { history_length: historyLength };
            },
            args: [offset]
        });
        const historyLength = injected[0]?.result?.history_length;
        if (typeof historyLength !== 'number' || historyLength <= 1)
            throw err;
        for (let attempt = 0; attempt < HISTORY_FALLBACK_DELAYS_MS.length; attempt += 1) {
            await delay(HISTORY_FALLBACK_DELAYS_MS[attempt]);
            const transitionedTab = await chrome.tabs.get(tabId);
            if (transitionedTab.url !== initialTab.url || transitionedTab.status === 'loading') {
                await waitForTabLoad(tabId);
                return chrome.tabs.get(tabId);
            }
        }
        throw new Error(`Page history fallback did not acknowledge transition for ${correlationId}`);
    }
    await waitForTabLoad(tabId);
    return chrome.tabs.get(tabId);
}
// =============================================================================
// BROWSER ACTION DISPATCH
// =============================================================================
export async function handleBrowserAction(tabId, params, actionToast, correlationId) {
    const { url, reason } = params || {};
    const action = typeof params?.action === 'string' && params.action.trim() !== ''
        ? params.action
        : typeof params?.what === 'string'
            ? params.what
            : undefined;
    if (!isAiWebPilotEnabled()) {
        return { success: false, error: 'ai_web_pilot_disabled', message: 'AI Web Pilot is not enabled' };
    }
    const changesDocument = action === 'refresh' || action === 'navigate' || action === 'back' || action === 'forward';
    try {
        switch (action) {
            case 'refresh': {
                contentReadiness.begin(tabId, correlationId);
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
                contentReadiness.begin(tabId, correlationId);
                return handleNavigateAction(tabId, url, actionToast, reason);
            case 'back': {
                contentReadiness.begin(tabId, correlationId);
                actionToast(tabId, reason || 'back', reason ? undefined : 'going back', 'trying', 10000);
                const backTab = await navigateHistory(tabId, -1, correlationId);
                actionToast(tabId, reason || 'back', undefined, 'success');
                return { success: true, action: 'back', url: backTab.url, title: backTab.title };
            }
            case 'forward': {
                contentReadiness.begin(tabId, correlationId);
                actionToast(tabId, reason || 'forward', reason ? undefined : 'going forward', 'trying', 10000);
                const fwdTab = await navigateHistory(tabId, 1, correlationId);
                actionToast(tabId, reason || 'forward', undefined, 'success');
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
                broadcastTrackingState().catch(() => {
                    debugLog(DebugCategory.CAPTURE, 'Tracking broadcast failed after tab switch');
                });
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
                // Activate the tab and focus its window (shared helper — one definition of
                // "bring a tab to the foreground"). Returns the updated tab for the result.
                const tab = await focusTabAndWindow(tabId);
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
        if (changesDocument)
            contentReadiness.cancel(tabId, correlationId);
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
        // EXPECTED_ABSENCE: optional enrichment can normally fail while the primary
        // operation keeps a valid fallback; logging it would misleadingly report fallback as failure.
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
            // EXPECTED_ABSENCE: optional enrichment can normally fail while the primary
            // operation keeps a valid fallback; logging it would misleadingly report fallback as failure.
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
    const executionPromise = handleBrowserAction(tabId, params, actionToast, query.correlation_id || query.id)
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