/**
 * Purpose: Shared infrastructure for command dispatch -- result helpers, target tab resolution, action toast, and type aliases.
 */
import { getTrackedTabInfo, getActiveTab } from '../ui/tracked-tab-state.js';
import { clearTrackedTab } from '../../lib/tabs/tracked-tab-storage.js';
import { DebugCategory } from '../debug.js';
import { isAiWebPilotEnabled } from '../runtime-state/pilot-state.js';
import { KABOOM_LOG_PREFIX } from '../../lib/brand.js';
import { errorMessage, isNoReceiverError } from '../../lib/error-utils.js';
import { delay } from '../../lib/timeout-utils.js';
import { setLocals } from '../../lib/storage/local.js';
import { isInternalUrl } from '../../lib/tabs/internal-url.js';
export function debugLog(category, message, data = null) {
    const globalLogger = globalThis
        .__KABOOM_DEBUG_LOG__;
    if (typeof globalLogger === 'function') {
        globalLogger(category, message, data);
        return;
    }
    // Keep helpers usable before the main debug logger is initialized.
    const debugEnabled = globalThis.__KABOOM_REGISTRY_DEBUG__ === true;
    if (!debugEnabled)
        return;
    const prefix = `${KABOOM_LOG_PREFIX.slice(0, -1)}:${category}]`;
    if (data === null) {
        console.debug(`${prefix} ${message}`);
        return;
    }
    console.debug(`${prefix} ${message}`, data);
}
function diagnosticLog(message) {
    debugLog(DebugCategory.CONNECTION, message);
}
// =============================================================================
// RESULT HELPERS
// =============================================================================
/** Send a query result back through /sync */
export function sendResult(syncClient, queryId, result) {
    debugLog(DebugCategory.CONNECTION, 'sendResult via /sync', { queryId, hasResult: result != null });
    syncClient.queueCommandResult({ id: queryId, status: 'complete', result });
}
/** Send an async command result back through /sync */
export function sendAsyncResult(syncClient, queryId, correlationId, status, result, error) {
    debugLog(DebugCategory.CONNECTION, 'sendAsyncResult via /sync', {
        queryId,
        correlationId,
        status,
        hasResult: result != null,
        error: error || null
    });
    syncClient.queueCommandResult({
        id: queryId,
        correlation_id: correlationId,
        status,
        result,
        error
    });
}
// =============================================================================
// ACTION TOAST
// =============================================================================
/** Map raw action names to human-readable toast labels */
const PRETTY_LABELS = {
    navigate: 'Navigate to',
    refresh: 'Refresh',
    execute_js: 'Execute',
    click: 'Click',
    type: 'Type',
    select: 'Select',
    check: 'Check',
    focus: 'Focus',
    scroll_to: 'Scroll to',
    wait_for: 'Wait for',
    wait_for_stable: 'Waiting for page to stabilize...',
    key_press: 'Key press',
    highlight: 'Highlight',
    subtitle: 'Subtitle',
    upload: 'Upload file'
};
const PRETTY_TRYING_LABELS = {
    scroll_to: 'Scrolling to',
    open_composer: 'Opening composer',
    submit_active_composer: 'Submitting active composer',
    confirm_top_dialog: 'Confirming top dialog',
    dismiss_top_overlay: 'Dismissing top overlay',
    auto_dismiss_overlays: 'Dismissing overlays'
};
function humanizeActionLabel(action) {
    const explicit = PRETTY_LABELS[action];
    if (explicit)
        return explicit;
    if (!/^[a-z0-9]+(?:_[a-z0-9]+)+$/.test(action))
        return action;
    const sentence = action.replaceAll('_', ' ');
    return sentence.charAt(0).toUpperCase() + sentence.slice(1);
}
function inferWaitTarget(detail) {
    if (!detail)
        return undefined;
    const trimmed = detail.trim();
    if (!trimmed || trimmed.toLowerCase() === 'page')
        return undefined;
    return trimmed;
}
function resolveToastCopy(action, detail, state) {
    if (state !== 'trying')
        return { text: humanizeActionLabel(action), detail };
    if (action === 'wait_for') {
        const waitTarget = inferWaitTarget(detail);
        if (waitTarget)
            return { text: `Waiting for ${waitTarget}` };
        return { text: 'Waiting for condition...' };
    }
    const tryingText = PRETTY_TRYING_LABELS[action];
    if (tryingText)
        return { text: tryingText, detail };
    return { text: humanizeActionLabel(action), detail };
}
/** Show a visual action toast on the tracked tab */
export function actionToast(tabId, action, detail, state = 'success', durationMs = 3000) {
    const toastCopy = resolveToastCopy(action, detail, state);
    chrome.tabs
        .sendMessage(tabId, {
        type: 'kaboom_action_toast',
        text: toastCopy.text,
        detail: toastCopy.detail,
        state,
        duration_ms: durationMs
    })
        .catch(() => {
        // EXPECTED_ABSENCE: missing optional toast recipients is normal between
        // documents; logging it would misleadingly duplicate command status.
    });
}
// =============================================================================
// PARAMS PARSING
// =============================================================================
export function parseQueryParamsObject(params) {
    if (typeof params === 'string') {
        try {
            const parsed = JSON.parse(params);
            if (parsed && typeof parsed === 'object') {
                return parsed;
            }
        }
        catch {
            return {};
        }
        return {};
    }
    if (params && typeof params === 'object') {
        return params;
    }
    return {};
}
// =============================================================================
// TARGET RESOLUTION
// =============================================================================
export function withTargetContext(result, target) {
    const targetContext = {
        resolved_tab_id: target.tabId,
        resolved_url: target.url,
        target_context: {
            source: target.source,
            requested_tab_id: target.requestedTabId ?? null,
            tracked_tab_id: target.trackedTabId ?? null,
            use_active_tab: target.useActiveTab
        }
    };
    if (result && typeof result === 'object' && !Array.isArray(result)) {
        return {
            ...result,
            ...targetContext
        };
    }
    return {
        value: result ?? null,
        ...targetContext
    };
}
const TARGETED_QUERY_TYPES = new Set([
    'subtitle',
    'screenshot',
    'browser_action',
    'highlight',
    'page_info',
    'waterfall',
    'dom',
    'a11y',
    'dom_action',
    'upload',
    'screen_recording_start',
    'execute',
    'link_health',
    'draw_mode',
    'cdp_action',
    'computed_styles',
    'form_discovery',
    'form_state',
    'data_table',
    'state_capture',
    'state_save',
    'state_load',
    'explore_page',
    'get_readable',
    'get_markdown',
    'page_summary',
    'page_inventory',
    'page_structure',
    'navigation',
    'feature_gates'
]);
export function requiresTargetTab(queryType) {
    return TARGETED_QUERY_TYPES.has(queryType);
}
export function isBrowserEscapeAction(queryType, paramsObj) {
    if (queryType !== 'browser_action')
        return false;
    const action = typeof paramsObj.action === 'string' ? paramsObj.action : typeof paramsObj.what === 'string' ? paramsObj.what : '';
    return (action === 'navigate' ||
        action === 'refresh' ||
        action === 'back' ||
        action === 'forward' ||
        action === 'new_tab' ||
        action === 'switch_tab' ||
        action === 'close_tab');
}
async function getTabWithRetry(tabId, retry = false) {
    try {
        return await chrome.tabs.get(tabId);
    }
    catch {
        if (!retry) {
            return null;
        }
        await delay(300);
        try {
            return await chrome.tabs.get(tabId);
        }
        catch {
            return null;
        }
    }
}
function buildMissingTargetError(queryType, useActiveTab, trackedTabId) {
    const message = "No target tab resolved. Provide 'tab_id', enable tab tracking, or set 'use_active_tab=true' explicitly.";
    return {
        message,
        payload: {
            success: false,
            error: 'missing_target',
            message,
            query_type: queryType,
            use_active_tab: useActiveTab,
            tracked_tab_id: trackedTabId
        }
    };
}
export async function persistTrackedTab(tab) {
    if (!tab.id)
        return;
    await setLocals({
        trackedTabId: tab.id,
        trackedTabUrl: tab.url || '',
        trackedTabTitle: tab.title || ''
    });
}
function isTrackableTab(tab) {
    return !!tab?.id && typeof tab.url === 'string' && !isRestrictedUrl(tab.url);
}
function pickRandom(items) {
    if (items.length === 0)
        return null;
    const idx = Math.floor(Math.random() * items.length);
    return items[idx] || null;
}
function buildNoTrackableTabError(queryType, useActiveTab, trackedTabId, attempts) {
    return {
        message: 'no_trackable_tab',
        payload: {
            success: false,
            error: 'no_trackable_tab',
            message: 'Unable to resolve a trackable tab for this command. Recovery attempts exhausted: active tab, non-internal tab switch, and opening a new tab.',
            query_type: queryType,
            use_active_tab: useActiveTab,
            tracked_tab_id: trackedTabId,
            attempted_recovery: attempts,
            retry: 'Open any normal web page tab (http/https), keep AI Web Pilot enabled, then retry the command.',
            retryable: false
        }
    };
}
async function tryAutoTrackFallback(queryType, useActiveTab, trackedTabId) {
    const attempts = [];
    const activeTab = await getActiveTab();
    if (isTrackableTab(activeTab)) {
        await persistTrackedTab(activeTab);
        attempts.push({
            step: 'track_active_tab',
            status: 'success',
            detail: `Tracked active tab ${activeTab.id} (${activeTab.url})`
        });
        diagnosticLog(`[Diagnostic] Auto-tracked active tab ${activeTab.id} for query ${queryType}`);
        return {
            target: {
                tabId: activeTab.id,
                url: activeTab.url,
                source: 'auto_tracked_active_tab',
                trackedTabId: activeTab.id,
                useActiveTab
            }
        };
    }
    if (!activeTab?.id) {
        attempts.push({
            step: 'track_active_tab',
            status: 'failed',
            detail: 'No active tab available'
        });
    }
    else {
        attempts.push({
            step: 'track_active_tab',
            status: 'failed',
            detail: `Active tab ${activeTab.id} is not trackable (${activeTab.url || 'unknown URL'})`
        });
    }
    try {
        const allTabs = await chrome.tabs.query({});
        const nonInternalTabs = allTabs.filter(isTrackableTab);
        const candidate = pickRandom(nonInternalTabs);
        if (candidate?.id) {
            const focused = await chrome.tabs.update(candidate.id, { active: true });
            const resolved = isTrackableTab(focused) ? focused : candidate;
            await persistTrackedTab(resolved);
            attempts.push({
                step: 'switch_to_non_internal_tab',
                status: 'success',
                detail: `Switched to trackable tab ${resolved.id} (${resolved.url})`
            });
            diagnosticLog(`[Diagnostic] Auto-tracked non-internal tab ${resolved.id} for query ${queryType}`);
            return {
                target: {
                    tabId: resolved.id,
                    url: resolved.url,
                    source: 'auto_tracked_random_tab',
                    trackedTabId: resolved.id,
                    useActiveTab: true
                }
            };
        }
        attempts.push({
            step: 'switch_to_non_internal_tab',
            status: 'failed',
            detail: 'No existing non-internal tabs available'
        });
    }
    catch (err) {
        attempts.push({
            step: 'switch_to_non_internal_tab',
            status: 'failed',
            detail: `Failed to enumerate/switch tabs: ${errorMessage(err)}`
        });
    }
    try {
        const createdTab = await chrome.tabs.create({
            url: 'https://example.com',
            active: true
        });
        const hydratedTab = createdTab?.id ? await getTabWithRetry(createdTab.id, true) : createdTab;
        if (isTrackableTab(hydratedTab)) {
            await persistTrackedTab(hydratedTab);
            attempts.push({
                step: 'open_new_tab_and_track',
                status: 'success',
                detail: `Opened and tracked tab ${hydratedTab.id} (${hydratedTab.url})`
            });
            diagnosticLog(`[Diagnostic] Auto-opened and tracked tab ${hydratedTab.id} for query ${queryType}`);
            return {
                target: {
                    tabId: hydratedTab.id,
                    url: hydratedTab.url,
                    source: 'auto_tracked_new_tab',
                    trackedTabId: hydratedTab.id,
                    useActiveTab: true
                }
            };
        }
        attempts.push({
            step: 'open_new_tab_and_track',
            status: 'failed',
            detail: `Opened tab is not trackable (${hydratedTab?.url || 'unknown URL'})`
        });
    }
    catch (err) {
        attempts.push({
            step: 'open_new_tab_and_track',
            status: 'failed',
            detail: `Failed to open tab: ${errorMessage(err)}`
        });
    }
    return {
        error: buildNoTrackableTabError(queryType, useActiveTab, trackedTabId, attempts)
    };
}
export async function resolveTargetTab(query, paramsObj) {
    const explicitTabId = typeof query.tab_id === 'number' && query.tab_id > 0 ? query.tab_id : undefined;
    const useActiveTab = paramsObj.use_active_tab === true;
    async function resolveAutoTrackOrEscapeFallback(trackedTabId, fallbackMessage) {
        if (isAiWebPilotEnabled()) {
            const recovered = await tryAutoTrackFallback(query.type, useActiveTab, trackedTabId);
            if (recovered.target || recovered.error) {
                return recovered;
            }
        }
        if (isBrowserEscapeAction(query.type, paramsObj)) {
            const activeTab = await getActiveTab();
            if (activeTab?.id) {
                diagnosticLog(`[Diagnostic] ${fallbackMessage} ${activeTab.id} for escape action ${query.type}`);
                return {
                    target: {
                        tabId: activeTab.id,
                        url: activeTab.url || '',
                        source: 'active_tab_fallback',
                        trackedTabId,
                        useActiveTab: true
                    }
                };
            }
        }
        return { error: buildMissingTargetError(query.type, useActiveTab, trackedTabId) };
    }
    if (explicitTabId) {
        const explicitTab = await getTabWithRetry(explicitTabId);
        if (!explicitTab?.id) {
            const message = `Requested tab_id ${explicitTabId} is not available`;
            return {
                error: {
                    message,
                    payload: {
                        success: false,
                        error: 'target_tab_not_found',
                        message,
                        requested_tab_id: explicitTabId
                    }
                }
            };
        }
        return {
            target: {
                tabId: explicitTab.id,
                url: explicitTab.url || '',
                source: 'explicit_tab',
                requestedTabId: explicitTabId,
                trackedTabId: null,
                useActiveTab
            }
        };
    }
    if (useActiveTab) {
        const activeTab = await getActiveTab();
        if (!activeTab?.id) {
            return {
                error: {
                    message: 'No active tab available',
                    payload: {
                        success: false,
                        error: 'no_active_tab',
                        message: 'No active tab available',
                        use_active_tab: true
                    }
                }
            };
        }
        let latchedTrackedTabId = null;
        if (isTrackableTab(activeTab)) {
            await persistTrackedTab(activeTab);
            latchedTrackedTabId = activeTab.id;
            diagnosticLog(`[Diagnostic] use_active_tab latched tracking to tab ${activeTab.id} for query ${query.type}`);
        }
        return {
            target: {
                tabId: activeTab.id,
                url: activeTab.url || '',
                source: 'active_tab',
                trackedTabId: latchedTrackedTabId,
                useActiveTab
            }
        };
    }
    const storage = await getTrackedTabInfo();
    const trackedTabId = storage.trackedTabId ?? null;
    if (trackedTabId) {
        diagnosticLog(`[Diagnostic] Using tracked tab ${trackedTabId} for query ${query.type}`);
        const trackedTab = await getTabWithRetry(trackedTabId, true);
        if (trackedTab?.id) {
            return {
                target: {
                    tabId: trackedTab.id,
                    url: trackedTab.url || storage.trackedTabUrl || '',
                    source: 'tracked_tab',
                    trackedTabId,
                    useActiveTab
                }
            };
        }
        diagnosticLog(`[Diagnostic] Tracked tab ${trackedTabId} unavailable, clearing tracking state`);
        await clearTrackedTab();
        try {
            const toastTab = await getActiveTab();
            if (toastTab?.id) {
                chrome.tabs
                    .sendMessage(toastTab.id, {
                    type: 'kaboom_action_toast',
                    text: 'Tracked tab unavailable',
                    detail: "Provide tab_id or use 'use_active_tab=true'",
                    state: 'warning',
                    duration_ms: 5000
                })
                    .catch(() => {
                    // EXPECTED_ABSENCE: this recovery toast targets whichever tab remains;
                    // no recipient is normal and logging would obscure the primary diagnostic.
                });
            }
        }
        catch {
            // EXPECTED_ABSENCE: active-tab discovery is best-effort after tracked-tab
            // loss; the primary diagnostic already records the actionable failure.
        }
        return resolveAutoTrackOrEscapeFallback(trackedTabId, 'Falling back to active tab');
    }
    return resolveAutoTrackOrEscapeFallback(trackedTabId, 'Using active tab fallback');
}
/**
 * Check if a URL is restricted — content scripts cannot run on these pages.
 * Covers internal browser pages and known CSP-restricted origins.
 * Delegates to the canonical predicate so the blocked-prefix list lives once.
 */
export function isRestrictedUrl(url) {
    return isInternalUrl(url);
}
// =============================================================================
// CONTENT SCRIPT ERROR DETECTION
// =============================================================================
/** Check if an error indicates the content script is not loaded on the target page. */
export function isContentScriptUnreachableError(err) {
    // Same underlying "no receiver" signal as a closed popup — delegate to the
    // canonical predicate so the message strings live in exactly one place.
    return isNoReceiverError(err);
}
/**
 * Guard that checks AI Web Pilot is enabled.
 * Returns true if enabled and the caller should proceed.
 * Returns false if disabled — the error response has already been sent.
 */
export function requireAiWebPilot(ctx) {
    if (isAiWebPilotEnabled())
        return true;
    ctx.sendResult({ error: 'ai_web_pilot_disabled' });
    return false;
}
//# sourceMappingURL=helpers.js.map