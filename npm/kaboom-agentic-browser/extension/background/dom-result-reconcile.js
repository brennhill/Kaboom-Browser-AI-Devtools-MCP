/**
 * Purpose: DOM action result validation, lifecycle reconciliation, and frame result picking.
 * Why: Separates result shape validation and status derivation from chrome.scripting execution.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import { isMutatingAction } from './action-metadata.js';
export function toDOMResult(value) {
    if (!value || typeof value !== 'object')
        return null;
    const candidate = value;
    if (typeof candidate.success !== 'boolean')
        return null;
    if (typeof candidate.action !== 'string' || typeof candidate.selector !== 'string')
        return null;
    return candidate;
}
function hasMatchedTargetEvidence(result) {
    const matched = result.matched;
    if (!matched || typeof matched !== 'object' || Array.isArray(matched))
        return false;
    return (typeof matched.selector === 'string' ||
        typeof matched.tag === 'string' ||
        typeof matched.element_id === 'string' ||
        typeof matched.aria_label === 'string' ||
        typeof matched.role === 'string' ||
        typeof matched.text_preview === 'string');
}
/** Pick the best result from multi-frame executeScript. Prefers main frame, falls back to first success. */
export function pickFrameResult(results) {
    const mainFrame = results.find((r) => r.frameId === 0);
    if (mainFrame?.result && mainFrame.result.success) {
        return { result: mainFrame.result, frameId: 0 };
    }
    for (const r of results) {
        if (r.result && r.result.success) {
            return { result: r.result, frameId: r.frameId };
        }
    }
    if (mainFrame?.result)
        return { result: mainFrame.result, frameId: 0 };
    return results[0] ? { result: results[0].result, frameId: results[0].frameId } : null;
}
/** Merge list_interactive results from all frames (up to 100 elements). */
export function mergeListInteractive(results) {
    const elements = [];
    let firstError = null;
    let firstScopeRectUsed;
    for (const r of results) {
        const res = r.result;
        if (res?.success === false) {
            if (!firstError)
                firstError = { error: res.error, message: res.message };
            continue;
        }
        if (firstScopeRectUsed === undefined && res?.scope_rect_used !== undefined) {
            firstScopeRectUsed = res.scope_rect_used;
        }
        if (res?.elements)
            elements.push(...res.elements);
        if (elements.length >= 100)
            break;
    }
    if (elements.length === 0 && firstError?.error) {
        return { success: false, elements: [], error: firstError.error, message: firstError.message };
    }
    const cappedElements = elements.slice(0, 100);
    const merged = {
        success: true,
        elements: cappedElements,
        candidate_count: cappedElements.length
    };
    if (firstScopeRectUsed !== undefined) {
        merged.scope_rect_used = firstScopeRectUsed;
    }
    return merged;
}
function reconcileDOMLifecycle(action, selector, result) {
    const domResult = toDOMResult(result);
    if (!domResult) {
        if (!isMutatingAction(action))
            return { result, status: 'complete' };
        const coerced = {
            success: false,
            action,
            selector,
            error: 'status_mismatch',
            message: `Mutating action returned non-DOM payload: ${action}`
        };
        return { result: coerced, status: 'error', error: 'status_mismatch' };
    }
    if (!domResult.success) {
        return {
            result: domResult,
            status: 'error',
            error: domResult.error || domResult.message || 'dom_action_failed'
        };
    }
    if (domResult.error) {
        const coerced = {
            ...domResult,
            success: false,
            error: 'status_mismatch',
            message: `Payload marked success but includes error: ${domResult.error}`
        };
        return { result: coerced, status: 'error', error: 'status_mismatch' };
    }
    if (isMutatingAction(action) && !hasMatchedTargetEvidence(domResult)) {
        const coerced = {
            ...domResult,
            success: false,
            error: 'missing_match_evidence',
            message: `Mutating action completed without matched target evidence: ${action}`
        };
        return { result: coerced, status: 'error', error: 'missing_match_evidence' };
    }
    return { result: domResult, status: 'complete' };
}
export function deriveAsyncStatusFromDOMResult(action, selector, result) {
    const reconciled = reconcileDOMLifecycle(action, selector, result);
    if (reconciled.status === 'complete') {
        return reconciled;
    }
    return {
        status: 'error',
        error: reconciled.error || 'dom_action_failed',
        result: reconciled.result
    };
}
// Enrich results with effective tab context (post-execution URL).
// Agents compare resolved_url (dispatch time) vs effective_url (execution time) to detect drift.
export async function enrichWithEffectiveContext(tabId, result) {
    try {
        const tab = await chrome.tabs.get(tabId);
        if (result && typeof result === 'object' && !Array.isArray(result)) {
            return {
                ...result,
                effective_tab_id: tabId,
                effective_url: tab.url,
                effective_title: tab.title
            };
        }
        return result;
    }
    catch {
        return result;
    }
}
export function sendToastForResult(tabId, readOnly, result, actionToast, toastLabel, toastDetail) {
    if (readOnly)
        return;
    if (result.success) {
        actionToast(tabId, toastLabel, toastDetail, 'success');
    }
    else {
        actionToast(tabId, toastLabel, result.error || 'failed', 'error');
    }
}
//# sourceMappingURL=dom-result-reconcile.js.map