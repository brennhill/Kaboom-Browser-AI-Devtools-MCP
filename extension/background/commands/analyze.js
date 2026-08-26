/**
 * Purpose: Command handlers for the analyze MCP tool (DOM inspection, accessibility audits, link health, draw mode) with frame routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 */
// analyze.ts — Command handlers for the analyze MCP tool.
// Handles: dom, a11y, link_health, draw_mode.
// Includes frame routing helpers used by dom and a11y.
import { registerCommand } from './registry.js';
import { isContentScriptUnreachableError, requireAiWebPilot } from './helpers.js';
import { KABOOM_LOG_PREFIX } from '../../lib/brand.js';
import { errorMessage } from '../../lib/error-utils.js';
import { domFrameProbe } from '../dom/primitives/dom-frame-probe.js';
import { normalizeFrameArg, resolveMatchedFrameIds } from '../exec/frame-targeting.js';
import { recordExtensionDiagnosticLifecycle } from '../runtime-state/log-queue.js';
import { createDefaultPerformanceTraceController, isTargetNotDebuggableError } from '../dom/cdp/performance-trace.js';
let performanceTraceController;
function getPerformanceTraceController() {
    performanceTraceController ??= createDefaultPerformanceTraceController();
    return performanceTraceController;
}
async function resolveAnalyzeFrameSelection(tabId, frame) {
    const normalized = normalizeFrameArg(frame);
    // No frame targeting requested — skip the probe entirely and target the main frame.
    if (normalized === undefined) {
        return { frameIds: [0], mode: 'main' };
    }
    const frameIds = await resolveMatchedFrameIds(tabId, normalized, domFrameProbe);
    if (normalized === 'all') {
        return { frameIds, mode: 'all' };
    }
    return { frameIds, mode: 'targeted' };
}
function stripFrameParam(params) {
    const copy = { ...params };
    delete copy.frame;
    return copy;
}
async function sendFrameQueries(tabId, frameIds, message) {
    return Promise.all(frameIds.map(async (frameId) => {
        try {
            const result = (await chrome.tabs.sendMessage(tabId, message, { frameId }));
            return { frame_id: frameId, result };
        }
        catch (err) {
            return {
                frame_id: frameId,
                error: errorMessage(err, 'frame_query_failed')
            };
        }
    }));
}
function buildSingleFrameResult(perFrame, errorCode) {
    const first = perFrame[0];
    if (!first) {
        return { error: errorCode, message: 'No frame response received' };
    }
    if (first.error) {
        return { error: errorCode, message: first.error, frame_id: first.frame_id };
    }
    return { ...(first.result || {}), frame_id: first.frame_id };
}
function isFrameRoutingError(message) {
    return message.startsWith('frame_not_found') || message.startsWith('invalid_frame');
}
function toNonNegativeInt(value) {
    if (typeof value !== 'number' || !Number.isFinite(value))
        return 0;
    const n = Math.floor(value);
    return n > 0 ? n : 0;
}
function aggregateDOMFrameResults(results) {
    const MAX_MATCHES = 200;
    const matches = [];
    const frames = [];
    let totalMatchCount = 0;
    let totalReturnedCount = 0;
    let url = '';
    let title = '';
    for (const entry of results) {
        if (entry.error) {
            frames.push({ frame_id: entry.frame_id, error: entry.error });
            continue;
        }
        const payload = entry.result || {};
        const frameMatchCount = toNonNegativeInt(payload.matchCount);
        const frameReturnedCount = toNonNegativeInt(payload.returnedCount);
        const frameMatches = Array.isArray(payload.matches) ? payload.matches : [];
        if (!url && typeof payload.url === 'string') {
            url = payload.url;
        }
        if (!title && typeof payload.title === 'string') {
            title = payload.title;
        }
        totalMatchCount += frameMatchCount;
        totalReturnedCount += frameReturnedCount;
        if (matches.length < MAX_MATCHES) {
            matches.push(...frameMatches.slice(0, MAX_MATCHES - matches.length));
        }
        frames.push({
            frame_id: entry.frame_id,
            match_count: frameMatchCount,
            returned_count: frameReturnedCount,
            ...(payload.error ? { error: payload.error } : {})
        });
    }
    return {
        url,
        title,
        matchCount: totalMatchCount,
        returnedCount: totalReturnedCount,
        matches,
        frames
    };
}
function collectA11yFrameRollup(entry) {
    const payload = entry.result || {};
    const violations = Array.isArray(payload.violations) ? payload.violations : [];
    const passes = Array.isArray(payload.passes) ? payload.passes : [];
    const incomplete = Array.isArray(payload.incomplete) ? payload.incomplete : [];
    const inapplicable = Array.isArray(payload.inapplicable) ? payload.inapplicable : [];
    const frameSummary = payload.summary;
    const frame = {
        frame_id: entry.frame_id,
        summary: {
            violations: toNonNegativeInt(frameSummary?.violations ?? violations.length),
            passes: toNonNegativeInt(frameSummary?.passes ?? passes.length),
            incomplete: toNonNegativeInt(frameSummary?.incomplete ?? incomplete.length),
            inapplicable: toNonNegativeInt(frameSummary?.inapplicable ?? inapplicable.length)
        },
        ...(payload.error ? { error: payload.error } : {})
    };
    const error = typeof payload.error === 'string' && payload.error.length > 0 ? payload.error : undefined;
    return { violations, passes, incomplete, inapplicable, frame, error };
}
function aggregateA11yFrameResults(results) {
    const violations = [];
    const passes = [];
    const incomplete = [];
    const inapplicable = [];
    const frames = [];
    const errors = [];
    for (const entry of results) {
        if (entry.error) {
            frames.push({ frame_id: entry.frame_id, error: entry.error });
            errors.push(entry.error);
            continue;
        }
        const rollup = collectA11yFrameRollup(entry);
        violations.push(...rollup.violations);
        passes.push(...rollup.passes);
        incomplete.push(...rollup.incomplete);
        inapplicable.push(...rollup.inapplicable);
        frames.push(rollup.frame);
        if (rollup.error)
            errors.push(rollup.error);
    }
    return {
        violations,
        passes,
        incomplete,
        inapplicable,
        summary: {
            violations: violations.length,
            passes: passes.length,
            incomplete: incomplete.length,
            inapplicable: inapplicable.length
        },
        frames,
        ...(errors.length > 0 ? { error: errors.join('; ') } : {})
    };
}
/**
 * Fallback DOM query implementation executed via chrome.scripting in ISOLATED world.
 * This keeps analyze(dom) working when content-script messaging is temporarily unavailable.
 */
function executeDOMQueryInIsolatedWorld(params) {
    const selector = typeof params.selector === 'string' && params.selector.trim().length > 0 ? params.selector : '*';
    const includeStyles = params.include_styles === true;
    const includeChildren = params.include_children === true;
    const styleProps = (Array.isArray(params.properties) ? params.properties : []).filter((prop) => typeof prop === 'string' && prop.length > 0);
    const rawDepth = typeof params.max_depth === 'number' ? params.max_depth : 3;
    const maxDepth = Math.max(0, Math.min(5, Math.floor(rawDepth)));
    const MAX_ELEMENTS = 50;
    const MAX_TEXT = 500;
    const collectAttributes = (el) => {
        if (!el.attributes || el.attributes.length === 0)
            return undefined;
        const attrs = {};
        for (const attr of Array.from(el.attributes)) {
            attrs[attr.name] = attr.value;
        }
        return attrs;
    };
    const isElementVisible = (el, rect) => el.offsetParent !== null ||
        (typeof rect?.width === 'number' && rect.width > 0) ||
        (typeof rect?.height === 'number' && rect.height > 0);
    const serializeElement = (el, depth) => {
        const rect = el.getBoundingClientRect?.();
        const out = {
            tag: el.tagName?.toLowerCase() || '',
            text: (el.textContent || '').slice(0, MAX_TEXT),
            visible: isElementVisible(el, rect)
        };
        const attrs = collectAttributes(el);
        if (attrs)
            out.attributes = attrs;
        if (rect) {
            out.boundingBox = {
                x: rect.x,
                y: rect.y,
                width: rect.width,
                height: rect.height
            };
        }
        if (includeStyles && typeof window.getComputedStyle === 'function') {
            const computed = window.getComputedStyle(el);
            if (styleProps.length > 0) {
                const styles = {};
                for (const prop of styleProps) {
                    styles[prop] = computed.getPropertyValue(prop);
                }
                out.styles = styles;
            }
            else {
                out.styles = {
                    display: computed.display,
                    color: computed.color,
                    position: computed.position
                };
            }
        }
        if (includeChildren && depth < maxDepth && el.children.length > 0) {
            const children = [];
            const childLimit = Math.min(el.children.length, MAX_ELEMENTS);
            for (let i = 0; i < childLimit; i++) {
                const child = el.children[i];
                if (child)
                    children.push(serializeElement(child, depth + 1));
            }
            out.children = children;
        }
        return out;
    };
    try {
        const allMatches = Array.from(document.querySelectorAll(selector));
        const matches = allMatches.slice(0, MAX_ELEMENTS).map((el) => serializeElement(el, 0));
        return {
            url: window.location.href,
            title: document.title,
            matchCount: allMatches.length,
            returnedCount: matches.length,
            matches
        };
    }
    catch (err) {
        return {
            error: 'dom_query_failed',
            message: err?.message || 'Failed to execute DOM query'
        };
    }
}
async function executeDOMQueryFallbackViaScripting(tabId, params, fallbackReason) {
    const execution = await chrome.scripting.executeScript({
        target: { tabId },
        world: 'ISOLATED',
        func: executeDOMQueryInIsolatedWorld,
        args: [params]
    });
    const first = execution?.[0]?.result;
    const payload = first && typeof first === 'object' ? first : {};
    return {
        ...payload,
        execution_world: 'ISOLATED',
        fallback_reason: fallbackReason
    };
}
async function runMainDOMAnalyzeQuery(ctx) {
    try {
        return (await chrome.tabs.sendMessage(ctx.tabId, {
            type: 'dom_query',
            params: ctx.query.params
        }));
    }
    catch (err) {
        const fallbackReason = isContentScriptUnreachableError(err)
            ? 'content_script_unreachable'
            : 'content_script_send_failed';
        try {
            return await executeDOMQueryFallbackViaScripting(ctx.tabId, stripFrameParam(ctx.params), fallbackReason);
        }
        catch {
            throw err;
        }
    }
}
async function runFrameAwareAnalyzeQuery(ctx, config) {
    const frameSelection = await resolveAnalyzeFrameSelection(ctx.tabId, ctx.params.frame);
    if (frameSelection.mode === 'main') {
        if (config.mainQuery) {
            return config.mainQuery(ctx);
        }
        return (await chrome.tabs.sendMessage(ctx.tabId, {
            type: config.messageType,
            params: ctx.query.params
        }));
    }
    const frameParams = stripFrameParam(ctx.params);
    const perFrame = await sendFrameQueries(ctx.tabId, frameSelection.frameIds, {
        type: config.messageType,
        params: frameParams
    });
    if (perFrame.length === 1) {
        return buildSingleFrameResult(perFrame, config.singleFrameErrorCode);
    }
    return config.aggregate(perFrame);
}
// =============================================================================
// DOM
// =============================================================================
registerCommand('dom', async (ctx) => {
    try {
        const result = await runFrameAwareAnalyzeQuery(ctx, {
            messageType: 'dom_query',
            singleFrameErrorCode: 'dom_query_failed',
            aggregate: aggregateDOMFrameResults,
            mainQuery: runMainDOMAnalyzeQuery
        });
        ctx.sendResult(result);
    }
    catch (err) {
        const message = errorMessage(err, 'Failed to execute DOM query');
        console.error(`${KABOOM_LOG_PREFIX}[DOM] Command failed:`, message, err.stack || err);
        const routingError = isFrameRoutingError(message);
        ctx.sendResult({
            error: routingError ? message : 'dom_query_failed',
            message: routingError ? message : `Failed to execute DOM query: ${message}`
        });
    }
});
// =============================================================================
// A11Y
// =============================================================================
registerCommand('a11y', async (ctx) => {
    try {
        const result = await runFrameAwareAnalyzeQuery(ctx, {
            messageType: 'a11y_query',
            singleFrameErrorCode: 'a11y_audit_failed',
            aggregate: aggregateA11yFrameResults
        });
        ctx.sendResult(result);
    }
    catch (err) {
        const message = errorMessage(err, 'Failed to execute accessibility audit');
        console.error(`${KABOOM_LOG_PREFIX}[A11Y] Command failed:`, message, err.stack || err);
        const routingError = isFrameRoutingError(message);
        ctx.sendResult({
            error: routingError ? message : 'a11y_audit_failed',
            message: routingError ? message : `Failed to execute accessibility audit: ${message}`
        });
    }
});
// =============================================================================
// CONTENT SCRIPT PASS-THROUGH COMMANDS
// =============================================================================
/** Register an analyze command that forwards params to a content script message type. */
function registerPassthrough(command, messageType, fallbackMessage) {
    registerCommand(command, async (ctx) => {
        try {
            const result = await chrome.tabs.sendMessage(ctx.tabId, {
                type: messageType,
                params: ctx.query.params
            });
            ctx.sendResult(result);
        }
        catch (err) {
            ctx.sendResult({
                error: `${command}_failed`,
                message: errorMessage(err, fallbackMessage)
            });
        }
    });
}
registerPassthrough('link_health', 'link_health_query', 'Link health check failed');
registerPassthrough('computed_styles', 'computed_styles_query', 'Computed styles query failed');
registerPassthrough('form_discovery', 'form_discovery_query', 'Form discovery failed');
registerPassthrough('form_state', 'form_state_query', 'Form state extraction failed');
registerPassthrough('data_table', 'data_table_query', 'Data table extraction failed');
/**
 * Report a target Chrome will never let the debugger attach to.
 *
 * A profile describes one tab, so this fails against the tab that was refused and
 * is never retargeted: handing back an artifact for a page the caller did not ask
 * about is worse than handing back nothing. It is also not retryable — Chrome will
 * refuse the same target again.
 *
 * Tracking is deliberately left alone. Only CDP attach was refused; the same tab
 * still serves execute_js, DOM primitives, and every other tool, so untracking it
 * would discard a working workspace over one unavailable capability.
 */
function reportUndebuggableTarget(ctx, error) {
    const message = errorMessage(error, 'Chrome refused to attach the debugger');
    recordExtensionDiagnosticLifecycle('performance_trace_target_not_debuggable', ctx.query.correlation_id || ctx.query.id || '', { tab_id: ctx.tabId });
    return {
        error: 'performance_trace_target_not_debuggable',
        message: `${message} (tab ${ctx.tabId}). Chrome will not expose this target to the extension. ` +
            'Profile a different tab with tab_id, or move this workspace to a normal web page.',
        tab_id: ctx.tabId,
        retryable: false
    };
}
registerCommand('performance_trace', async (ctx) => {
    const action = typeof ctx.params.action === 'string' ? ctx.params.action : '';
    try {
        if (action === 'start') {
            const cache = ctx.params.cache;
            if (cache !== undefined && cache !== 'warm' && cache !== 'cold') {
                ctx.sendResult({ error: 'invalid_performance_trace_cache', message: 'cache must be warm or cold' });
                return;
            }
            const started = await getPerformanceTraceController().start(ctx.tabId, {
                reload: ctx.params.reload === true,
                cache
            });
            if (started.recovered) {
                recordExtensionDiagnosticLifecycle('performance_trace_recovered', ctx.query.correlation_id || ctx.query.id || '', {
                    trace_id: started.trace_id,
                    tab_id: started.tab_id
                });
            }
            ctx.sendResult(started);
            return;
        }
        if (action === 'stop') {
            ctx.sendResult(await getPerformanceTraceController().stop(ctx.tabId));
            return;
        }
        ctx.sendResult({
            error: 'invalid_performance_trace_action',
            message: 'performance_trace requires action=start or action=stop'
        });
    }
    catch (error) {
        if (isTargetNotDebuggableError(error)) {
            ctx.sendResult(reportUndebuggableTarget(ctx, error));
            return;
        }
        ctx.sendResult({
            error: 'performance_trace_failed',
            message: errorMessage(error, 'Performance trace command failed')
        });
    }
});
registerCommand('react_profile', async (ctx) => {
    const action = typeof ctx.params.action === 'string' ? ctx.params.action : '';
    if (action !== 'start' && action !== 'stop') {
        ctx.sendResult({
            error: 'invalid_react_profile_action',
            message: 'react_profile requires action=start or action=stop'
        });
        return;
    }
    try {
        const execution = await chrome.scripting.executeScript({
            target: { tabId: ctx.tabId },
            world: 'MAIN',
            func: (profileAction) => {
                const api = window.__kaboom;
                if (!api)
                    return { status: 'unsupported', reason: 'kaboom_page_api_unavailable' };
                return profileAction === 'start' ? api.startReactProfile() : api.stopReactProfile();
            },
            args: [action]
        });
        ctx.sendResult(execution[0]?.result ?? { status: 'unsupported', reason: 'no_main_world_result' });
    }
    catch (error) {
        ctx.sendResult({ error: 'react_profile_failed', message: errorMessage(error, 'React profile command failed') });
    }
});
// =============================================================================
// DRAW MODE
// =============================================================================
registerCommand('draw_mode', async (ctx) => {
    if (!requireAiWebPilot(ctx))
        return;
    const params = ctx.params;
    if (params.action === 'start') {
        try {
            const result = await chrome.tabs.sendMessage(ctx.tabId, {
                type: 'kaboom_draw_mode_start',
                started_by: 'llm',
                annot_session_name: params.annot_session || '',
                correlation_id: ctx.query.correlation_id || ctx.query.id || ''
            });
            ctx.sendResult({
                status: result?.status || 'active',
                message: 'Draw mode activated. User can now draw annotations on the page. Results will be delivered when user finishes (presses ESC).',
                annotation_count: result?.annotation_count || 0
            });
        }
        catch (err) {
            ctx.sendResult({
                error: 'draw_mode_failed',
                message: errorMessage(err, 'Failed to activate draw mode. Ensure content script is loaded (try refreshing the page).')
            });
        }
    }
    else {
        ctx.sendResult({
            error: 'unknown_draw_mode_action',
            message: `Unknown draw mode action: ${params.action}. Use 'start'.`
        });
    }
});
// Navigation command handler extracted to analyze-navigation.ts (#335)
//# sourceMappingURL=analyze.js.map