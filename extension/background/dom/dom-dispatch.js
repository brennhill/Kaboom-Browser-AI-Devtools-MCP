/**
 * Purpose: Dispatches DOM actions (click, type, wait_for, list_interactive, query) to injected page scripts with frame targeting and CDP escalation.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import { domFrameProbe } from './primitives/dom-frame-probe.js';
import { domPrimitivePointer } from './primitives/dom-primitives-pointer.js';
import { domPrimitiveForm } from './primitives/dom-primitives-form.js';
import { domPrimitiveRead } from './primitives/dom-primitives-read.js';
import { domPrimitiveListInteractive } from './primitives/dom-primitives-list-interactive.js';
import { domPrimitiveQuery } from './primitives/dom-primitives-query.js';
import { domPrimitiveWaitForStable, domPrimitiveActionDiff } from './primitives/dom-primitives-stability.js';
import { domPrimitiveOverlay } from './primitives/dom-primitives-overlay.js';
import { domPrimitiveIntent } from './primitives/dom-primitives-intent.js';
import { shouldEscalateToCDP, tryCDPEscalation } from './cdp/cdp-dispatch.js';
import { isReadOnlyAction } from '../exec/action-metadata.js';
import { errorMessage } from '../../lib/error-utils.js';
import { delay } from '../../lib/timeout-utils.js';
import { normalizeFrameArg, resolveMatchedFrameIds } from '../exec/frame-targeting.js';
import { toDOMResult, pickFrameResult, mergeListInteractive, deriveAsyncStatusFromDOMResult, enrichWithEffectiveContext, sendToastForResult } from './dom-result-reconcile.js';
function parseDOMParams(query) {
    try {
        return typeof query.params === 'string' ? JSON.parse(query.params) : query.params;
    }
    catch {
        // EXPECTED_ABSENCE: malformed external parameters are an expected validation case; logging would duplicate the client error.
        return null;
    }
}
async function resolveExecutionTarget(tabId, frame) {
    const normalized = normalizeFrameArg(frame);
    if (normalized === undefined || normalized === 'all') {
        return { tabId, allFrames: true };
    }
    const frameIds = await resolveMatchedFrameIds(tabId, normalized, domFrameProbe);
    return { tabId, frameIds };
}
const WAIT_FOR_POLL_INTERVAL_MS = 80;
/** Resolve which DOM action name to dispatch for wait_for based on params.
 *  Callers must validate mutual exclusivity before calling this. */
function resolveWaitForAction(params) {
    if (params.absent)
        return 'wait_for_absent';
    if (params.text)
        return 'wait_for_text';
    return 'wait_for';
}
async function executeWaitForURL(tabId, params) {
    const urlSubstring = params.url_contains;
    const timeoutMs = Math.max(1, params.timeout_ms ?? 5000);
    const startedAt = Date.now();
    while (true) {
        const tab = await chrome.tabs.get(tabId);
        if (tab.url && tab.url.includes(urlSubstring)) {
            return {
                success: true,
                action: 'wait_for',
                selector: '',
                value: tab.url
            };
        }
        if (Date.now() - startedAt >= timeoutMs) {
            return {
                success: false,
                action: 'wait_for',
                selector: '',
                error: 'timeout',
                message: `URL did not contain "${urlSubstring}" within ${timeoutMs}ms`
            };
        }
        const remaining = timeoutMs - (Date.now() - startedAt);
        await delay(Math.min(WAIT_FOR_POLL_INTERVAL_MS, Math.max(1, remaining)));
    }
}
async function executeWaitFor(target, params) {
    const selector = params.selector || '';
    const timeoutMs = Math.max(1, params.timeout_ms ?? 5000);
    const domAction = resolveWaitForAction(params);
    const domOpts = { timeout_ms: timeoutMs, text: params.text };
    const startedAt = Date.now();
    const quickCheck = await chrome.scripting.executeScript({
        target,
        world: 'MAIN',
        func: domPrimitiveRead,
        args: [domAction, selector, domOpts]
    });
    const quickPicked = pickFrameResult(quickCheck);
    const quickResult = toDOMResult(quickPicked?.result);
    if (quickResult?.success) {
        return quickResult;
    }
    let lastResult = toDOMResult(quickPicked?.result) ?? null;
    while (Date.now() - startedAt < timeoutMs) {
        const remaining = timeoutMs - (Date.now() - startedAt);
        await delay(Math.min(WAIT_FOR_POLL_INTERVAL_MS, Math.max(1, remaining)));
        const probeResults = await chrome.scripting.executeScript({
            target,
            world: 'MAIN',
            func: domPrimitiveRead,
            args: [domAction, selector, domOpts]
        });
        const picked = pickFrameResult(probeResults);
        const result = toDOMResult(picked?.result);
        if (result)
            lastResult = result;
        if (result?.success) {
            return result;
        }
    }
    const label = domAction === 'wait_for_text'
        ? `Text "${params.text}" not found within ${timeoutMs}ms`
        : domAction === 'wait_for_absent'
            ? `Element still present within ${timeoutMs}ms: ${selector}`
            : undefined;
    if (lastResult?.error === 'timeout') {
        return lastResult;
    }
    return {
        success: false,
        action: 'wait_for',
        selector,
        error: 'timeout',
        message: label || `Element not found within ${timeoutMs}ms: ${selector}`
    };
}
async function executeStandardAction(target, params) {
    const primitive = POINTER_ACTIONS.has(params.action)
        ? domPrimitivePointer
        : FORM_ACTIONS.has(params.action)
            ? domPrimitiveForm
            : domPrimitiveRead;
    return chrome.scripting.executeScript({
        target,
        world: 'MAIN',
        func: primitive,
        args: [
            params.action,
            params.selector || '',
            {
                text: params.text,
                key: params.key,
                value: params.value,
                direction: params.direction,
                clear: params.clear,
                checked: params.checked,
                name: params.name,
                timeout_ms: params.timeout_ms,
                stability_ms: params.stability_ms,
                analyze: params.analyze,
                observe_mutations: params.observe_mutations,
                element_id: params.element_id,
                scope_selector: params.scope_selector,
                scope_rect: params.scope_rect,
                nth: params.nth,
                new_tab: params.new_tab,
                structured: params.structured
            }
        ]
    });
}
async function executeListInteractive(target, params) {
    // Build options object with scope_rect and filter params (#369)
    const opts = {};
    if (params.scope_rect)
        opts.scope_rect = params.scope_rect;
    if (params.text_contains)
        opts.text_contains = params.text_contains;
    if (params.role)
        opts.role = params.role;
    if (params.visible_only)
        opts.visible_only = params.visible_only;
    if (params.exclude_nav)
        opts.exclude_nav = params.exclude_nav;
    const hasOpts = Object.keys(opts).length > 0;
    const args = hasOpts
        ? [params.selector || '', opts]
        : [params.selector || ''];
    return chrome.scripting.executeScript({
        target,
        world: 'MAIN',
        func: domPrimitiveListInteractive,
        args
    });
}
// #370: Execute DOM query (exists, count, text, text_all, attributes)
async function executeQuery(target, params) {
    const opts = {};
    if (params.query_type)
        opts.query_type = params.query_type;
    if (params.attribute_names)
        opts.attribute_names = params.attribute_names;
    if (params.scope_selector)
        opts.scope_selector = params.scope_selector;
    return chrome.scripting.executeScript({
        target,
        world: 'MAIN',
        func: domPrimitiveQuery,
        args: [params.selector || '', Object.keys(opts).length > 0 ? opts : undefined]
    });
}
// #502: Execute stability actions (wait_for_stable, action_diff) via extracted self-contained functions
async function executeStabilityAction(target, params) {
    if (params.action === 'wait_for_stable') {
        return chrome.scripting.executeScript({
            target,
            world: 'MAIN',
            func: domPrimitiveWaitForStable,
            args: [{ stability_ms: params.stability_ms, timeout_ms: params.timeout_ms }]
        });
    }
    // action_diff
    return chrome.scripting.executeScript({
        target,
        world: 'MAIN',
        func: domPrimitiveActionDiff,
        args: [{ timeout_ms: params.timeout_ms }]
    });
}
// #502: Execute overlay actions (dismiss_top_overlay, auto_dismiss_overlays) via extracted self-contained function
async function executeOverlayAction(target, params) {
    return chrome.scripting.executeScript({
        target,
        world: 'MAIN',
        func: domPrimitiveOverlay,
        args: [
            params.action,
            { scope_selector: params.scope_selector, timeout_ms: params.timeout_ms }
        ]
    });
}
// #502: Execute intent actions (open_composer, submit_active_composer, confirm_top_dialog) via extracted self-contained function
async function executeIntentAction(target, params) {
    return chrome.scripting.executeScript({
        target,
        world: 'MAIN',
        func: domPrimitiveIntent,
        args: [
            params.action,
            { scope_selector: params.scope_selector }
        ]
    });
}
const STABILITY_ACTIONS = new Set(['wait_for_stable', 'action_diff']);
const OVERLAY_ACTIONS = new Set(['dismiss_top_overlay', 'auto_dismiss_overlays']);
const INTENT_ACTIONS = new Set(['open_composer', 'submit_active_composer', 'confirm_top_dialog']);
const POINTER_ACTIONS = new Set(['click', 'hover', 'focus', 'scroll_to']);
const FORM_ACTIONS = new Set(['type', 'paste', 'select', 'check', 'key_press', 'set_attribute']);
function sendDOMError(syncClient, query, message, sendAsyncResult) {
    sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, message);
}
/** Validate wait_for condition exclusivity; returns the rejection message or null. */
function waitForParamsError(params, selector) {
    const hasSelector = !!(selector || params.element_id);
    const hasText = !!params.text;
    const hasURL = !!params.url_contains;
    const condCount = (hasSelector || params.absent ? 1 : 0) + (hasText ? 1 : 0) + (hasURL ? 1 : 0);
    if (condCount === 0)
        return 'wait_for requires selector, text, or url_contains';
    if (condCount > 1)
        return 'wait_for conditions are mutually exclusive';
    if (params.absent && !hasSelector)
        return 'wait_for with absent requires a selector';
    return null;
}
/** Returns true when the wait_for params are invalid and the query has been rejected. */
function rejectInvalidWaitFor(params, syncClient, query, sendAsyncResult) {
    const error = waitForParamsError(params, params.selector || '');
    if (!error)
        return false;
    sendDOMError(syncClient, query, error, sendAsyncResult);
    return true;
}
function resolveToastContext(params) {
    const label = params.reason || params.action;
    const detail = params.reason ? undefined : params.selector || 'page';
    return { label, detail };
}
async function executeWaitForURLFlow(tabId, params, syncClient, query, actionToast, sendAsyncResult) {
    try {
        const urlResult = await executeWaitForURL(tabId, params);
        sendAsyncResult(syncClient, query.id, query.correlation_id, urlResult.success ? 'complete' : 'error', await enrichWithEffectiveContext(tabId, urlResult), urlResult.success ? undefined : urlResult.error);
    }
    catch (err) {
        actionToast(tabId, 'wait_for', errorMessage(err), 'error');
        sendDOMError(syncClient, query, errorMessage(err), sendAsyncResult);
    }
}
/** Dispatch a reconciled DOM result to the client, enriched with effective context. */
async function sendReconciledAsyncResult(ctx, status, reconciledResult, error) {
    ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, status, await enrichWithEffectiveContext(ctx.tabId, reconciledResult), error);
}
async function attemptCDPEscalation(ctx, action, params, selector) {
    // CDP auto-escalation: try hardware events first for click/type/key_press (main frame only).
    // Falls back to DOM primitives silently if CDP is unavailable or fails.
    // `dispatch: "dom"` opts out entirely (React escape hatch, #599).
    if (!shouldEscalateToCDP(action, params))
        return false;
    try {
        const cdpResult = await tryCDPEscalation(ctx.tabId, action, params);
        if (!cdpResult)
            return false;
        const { result: reconciledResult, status, error } = deriveAsyncStatusFromDOMResult(action, selector, cdpResult);
        const domResult = toDOMResult(reconciledResult);
        if (domResult) {
            sendToastForResult(ctx.tabId, false, domResult, ctx.actionToast, ctx.toastLabel, ctx.toastDetail);
        }
        else {
            ctx.actionToast(ctx.tabId, ctx.toastLabel, ctx.toastDetail, 'success');
        }
        await sendReconciledAsyncResult(ctx, status, reconciledResult, error);
        return true;
    }
    catch {
        // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
        // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
        // CDP failed — fall through to DOM primitives
        return false;
    }
}
async function routeDOMExecution(action, target, params) {
    if (action === 'list_interactive')
        return executeListInteractive(target, params);
    if (action === 'query')
        return executeQuery(target, params);
    if (action === 'wait_for')
        return executeWaitFor(target, params);
    if (STABILITY_ACTIONS.has(action))
        return executeStabilityAction(target, params);
    if (OVERLAY_ACTIONS.has(action))
        return executeOverlayAction(target, params);
    if (INTENT_ACTIONS.has(action))
        return executeIntentAction(target, params);
    return executeStandardAction(target, params);
}
function notifyMissingResult(ctx) {
    if (!ctx.readOnly)
        ctx.actionToast(ctx.tabId, ctx.toastLabel, 'no result', 'error');
    sendDOMError(ctx.syncClient, ctx.query, 'no_result', ctx.sendAsyncResult);
}
async function sendDirectDOMResult(ctx, action, selector, rawResult) {
    const { result: reconciledResult, status, error } = deriveAsyncStatusFromDOMResult(action, selector, rawResult);
    const domResult = toDOMResult(reconciledResult);
    if (domResult) {
        sendToastForResult(ctx.tabId, ctx.readOnly, domResult, ctx.actionToast, ctx.toastLabel, ctx.toastDetail);
    }
    else if (!ctx.readOnly && status === 'complete') {
        ctx.actionToast(ctx.tabId, ctx.toastLabel, ctx.toastDetail, 'success');
    }
    else if (!ctx.readOnly && status === 'error') {
        ctx.actionToast(ctx.tabId, ctx.toastLabel, error || 'failed', 'error');
    }
    await sendReconciledAsyncResult(ctx, status, reconciledResult, error);
}
async function sendListInteractiveResult(tabId, rawResult, syncClient, query, sendAsyncResult) {
    const merged = mergeListInteractive(rawResult);
    sendAsyncResult(syncClient, query.id, query.correlation_id, merged.success ? 'complete' : 'error', await enrichWithEffectiveContext(tabId, merged), merged.success ? undefined : merged.error || 'list_interactive_failed');
}
function buildFramePayload(picked, firstResult) {
    const base = { ...firstResult, frame_id: picked.frameId };
    const matched = base['matched'];
    if (matched && typeof matched === 'object' && !Array.isArray(matched)) {
        base['matched'] = { ...matched, frame_id: picked.frameId };
    }
    return base;
}
async function sendFrameResult(ctx, action, selector, resultPayload) {
    const { result: reconciledResult, status, error } = deriveAsyncStatusFromDOMResult(action, selector, resultPayload);
    const domResult = toDOMResult(reconciledResult);
    if (domResult) {
        sendToastForResult(ctx.tabId, ctx.readOnly, domResult, ctx.actionToast, ctx.toastLabel, ctx.toastDetail);
    }
    else if (!ctx.readOnly && status === 'error') {
        ctx.actionToast(ctx.tabId, ctx.toastLabel, error || 'failed', 'error');
    }
    await sendReconciledAsyncResult(ctx, status, reconciledResult, error);
}
async function finalizeFrameResults(ctx, action, selector, rawResult, tryingShownAt) {
    // Ensure "trying" toast is visible for at least 500ms
    const MIN_TOAST_MS = 500;
    const elapsed = Date.now() - tryingShownAt;
    if (!ctx.readOnly && elapsed < MIN_TOAST_MS)
        await delay(MIN_TOAST_MS - elapsed);
    // list_interactive: merge elements from all frames
    if (action === 'list_interactive') {
        await sendListInteractiveResult(ctx.tabId, rawResult, ctx.syncClient, ctx.query, ctx.sendAsyncResult);
        return;
    }
    const picked = pickFrameResult(rawResult);
    const firstResult = picked?.result;
    if (firstResult && typeof firstResult === 'object') {
        const resultPayload = picked ? buildFramePayload(picked, firstResult) : firstResult;
        await sendFrameResult(ctx, action, selector, resultPayload);
    }
    else {
        notifyMissingResult(ctx);
    }
}
export async function executeDOMAction(query, tabId, syncClient, sendAsyncResult, actionToast) {
    const params = parseDOMParams(query);
    if (!params) {
        sendDOMError(syncClient, query, 'invalid_params', sendAsyncResult);
        return;
    }
    const { action, selector } = params;
    if (!action) {
        sendDOMError(syncClient, query, 'missing_action', sendAsyncResult);
        return;
    }
    if (action === 'wait_for' && rejectInvalidWaitFor(params, syncClient, query, sendAsyncResult)) {
        return;
    }
    const { label: toastLabel, detail: toastDetail } = resolveToastContext(params);
    const readOnly = isReadOnlyAction(action);
    const selectorArg = selector || '';
    const outcome = {
        tabId,
        readOnly,
        toastLabel,
        toastDetail,
        syncClient,
        query,
        actionToast,
        sendAsyncResult
    };
    // URL-based wait_for: polls chrome.tabs.get from background — no page injection needed.
    if (action === 'wait_for' && params.url_contains) {
        await executeWaitForURLFlow(tabId, params, syncClient, query, actionToast, sendAsyncResult);
        return;
    }
    try {
        const executionTarget = await resolveExecutionTarget(tabId, params.frame);
        const tryingShownAt = Date.now();
        if (!readOnly)
            actionToast(tabId, toastLabel, toastDetail, 'trying', 10000);
        if (await attemptCDPEscalation(outcome, action, params, selectorArg)) {
            return;
        }
        const rawResult = await routeDOMExecution(action, executionTarget, params);
        // wait_for quick-check can return a DOMResult directly
        if (!Array.isArray(rawResult)) {
            if (!rawResult) {
                notifyMissingResult(outcome);
                return;
            }
            await sendDirectDOMResult(outcome, action, selectorArg, rawResult);
            return;
        }
        await finalizeFrameResults(outcome, action, selectorArg, rawResult, tryingShownAt);
    }
    catch (err) {
        actionToast(tabId, action, errorMessage(err), 'error');
        sendDOMError(syncClient, query, errorMessage(err), sendAsyncResult);
    }
}
//# sourceMappingURL=dom-dispatch.js.map