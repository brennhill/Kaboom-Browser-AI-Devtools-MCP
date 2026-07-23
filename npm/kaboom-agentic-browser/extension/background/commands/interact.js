/**
 * Purpose: Dispatches interact tool commands to extension-side handlers.
 * Why: Routes MCP interact actions to DOM primitives, browser actions, and CDP operations.
 * Docs: docs/features/feature/interact-explore/index.md
 */
// interact.ts — Command handlers for the interact MCP tool.
// Handles: subtitle, highlight, browser_action, dom_action, upload,
//          execute, screen_recording_start, screen_recording_stop, state_*.
import { isAiWebPilotEnabled } from '../state.js';
import { executeDOMAction } from '../dom-dispatch.js';
import { executeCDPAction } from '../cdp-dispatch.js';
import { executeUpload } from '../upload-handler.js';
import { startRecording, stopRecording } from '../recording.js';
import { executeWithWorldRouting } from '../query-execution.js';
import { handleBrowserAction, handleAsyncBrowserAction, handleAsyncExecuteCommand } from '../browser-actions.js';
import { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot } from '../message-handlers.js';
import { registerCommand } from './registry.js';
import { requireAiWebPilot, isContentScriptUnreachableError } from './helpers.js';
import { errorMessage } from '../../lib/error-utils.js';
// =============================================================================
// SUBTITLE
// =============================================================================
registerCommand('subtitle', async (ctx) => {
    const params = ctx.params;
    chrome.tabs
        .sendMessage(ctx.tabId, {
        type: 'kaboom_subtitle',
        text: params.text ?? ''
    })
        .catch(() => { });
    ctx.sendResult({ success: true, subtitle: params.text || 'cleared' });
});
// =============================================================================
// HIGHLIGHT
// =============================================================================
registerCommand('highlight', async (ctx) => {
    const params = ctx.params;
    const result = await handlePilotCommand('kaboom_highlight', params, ctx.tabId);
    ctx.sendResult(result);
});
// =============================================================================
// BROWSER ACTION
// =============================================================================
registerCommand('browser_action', async (ctx) => {
    const params = ctx.params;
    if (ctx.query.correlation_id) {
        await handleAsyncBrowserAction(ctx.query, ctx.tabId, params, ctx.syncClient, ctx.sendAsyncResult, ctx.actionToast);
    }
    else {
        const result = await handleBrowserAction(ctx.tabId, params, ctx.actionToast);
        ctx.sendResult(result);
    }
});
// =============================================================================
// DOM ACTION
// =============================================================================
registerCommand('dom_action', async (ctx) => {
    if (!requireAiWebPilot(ctx))
        return;
    await executeDOMAction(ctx.query, ctx.tabId, ctx.syncClient, ctx.sendAsyncResult, ctx.actionToast);
});
// =============================================================================
// CDP ACTION
// =============================================================================
registerCommand('cdp_action', async (ctx) => {
    if (!requireAiWebPilot(ctx))
        return;
    await executeCDPAction(ctx.query, ctx.tabId, ctx.syncClient, ctx.sendAsyncResult, ctx.actionToast);
});
// =============================================================================
// UPLOAD
// =============================================================================
registerCommand('upload', async (ctx) => {
    if (!requireAiWebPilot(ctx))
        return;
    await executeUpload(ctx.query, ctx.tabId, ctx.syncClient, ctx.sendAsyncResult, ctx.actionToast);
});
// =============================================================================
// SCREEN RECORDING START
// =============================================================================
registerCommand('screen_recording_start', async (ctx) => {
    if (!requireAiWebPilot(ctx))
        return;
    const params = ctx.params;
    const result = await startRecording(params.name ?? 'recording', params.fps ?? 15, ctx.query.id, params.audio ?? '', false, ctx.tabId);
    ctx.sendResult(result);
});
// =============================================================================
// SCREEN RECORDING STOP
// =============================================================================
registerCommand('screen_recording_stop', async (ctx) => {
    if (!requireAiWebPilot(ctx))
        return;
    const result = await stopRecording();
    ctx.sendResult(result);
});
// =============================================================================
// STATE QUERIES (state_capture, state_save, state_load, state_list, state_delete)
// =============================================================================
registerCommand('state_*', async (ctx) => {
    if (!requireAiWebPilot(ctx))
        return;
    const params = ctx.params;
    const action = params.action;
    // Use the tracked tab from the command context instead of querying for active tab.
    // chrome.tabs.query({ active: true, currentWindow: true }) is unreliable from a service worker.
    const tabId = ctx.tabId;
    if (!tabId) {
        ctx.sendResult({ error: 'no_tracked_tab', message: 'No tracked tab available for state command' });
        return;
    }
    try {
        let result;
        switch (action) {
            case 'capture': {
                const captureData = (await chrome.tabs.sendMessage(tabId, {
                    type: 'kaboom_manage_state',
                    params: { action: 'capture' }
                }));
                // Enrich with chrome.cookies API for full cookie metadata (HttpOnly, Secure, etc.)
                try {
                    const tab = await chrome.tabs.get(tabId);
                    if (tab.url) {
                        const chromeCookies = await chrome.cookies.getAll({ url: tab.url });
                        captureData.cookies = chromeCookies.map((c) => ({
                            name: c.name,
                            value: c.value,
                            domain: c.domain,
                            path: c.path,
                            secure: c.secure,
                            httpOnly: c.httpOnly,
                            sameSite: c.sameSite,
                            expirationDate: c.expirationDate
                        }));
                    }
                }
                catch {
                    // Falls back to document.cookie string from captureState()
                }
                result = captureData;
                break;
            }
            case 'save': {
                const captureResult = (await chrome.tabs.sendMessage(tabId, {
                    type: 'kaboom_manage_state',
                    params: { action: 'capture' }
                }));
                if (captureResult.error) {
                    ctx.sendResult({ error: captureResult.error });
                    return;
                }
                result = await saveStateSnapshot(params.name, captureResult);
                break;
            }
            case 'load': {
                const snapshot = await loadStateSnapshot(params.name);
                if (!snapshot) {
                    ctx.sendResult({
                        error: `Snapshot '${params.name}' not found`
                    });
                    return;
                }
                result = await chrome.tabs.sendMessage(tabId, {
                    type: 'kaboom_manage_state',
                    params: {
                        action: 'restore',
                        state: snapshot,
                        include_url: params.include_url !== false
                    }
                });
                break;
            }
            case 'list':
                result = { snapshots: await listStateSnapshots() };
                break;
            case 'delete':
                result = await deleteStateSnapshot(params.name);
                break;
            default:
                result = { error: `Unknown action: ${action}` };
        }
        ctx.sendResult(result);
    }
    catch (err) {
        ctx.sendResult({ error: errorMessage(err) });
    }
});
// =============================================================================
// EXECUTE
// =============================================================================
registerCommand('execute', async (ctx) => {
    const execParams = ctx.params;
    const world = execParams.world || 'auto';
    if (ctx.query.correlation_id) {
        if (!isAiWebPilotEnabled()) {
            ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'error', {
                success: false,
                error: 'ai_web_pilot_disabled',
                message: 'AI Web Pilot is not enabled'
            }, 'ai_web_pilot_disabled');
            return;
        }
        await handleAsyncExecuteCommand(ctx.query, ctx.tabId, world, ctx.syncClient, ctx.sendAsyncResult, ctx.actionToast);
    }
    else {
        if (!requireAiWebPilot(ctx))
            return;
        try {
            const result = await executeWithWorldRouting(ctx.tabId, ctx.query.params, world);
            ctx.sendResult(result);
        }
        catch (err) {
            ctx.sendResult({
                success: false,
                error: 'execution_failed',
                message: errorMessage(err, 'Execution failed')
            });
        }
    }
});
// =============================================================================
// PILOT COMMAND (exported for use by index.ts re-export chain)
// =============================================================================
function buildFallbackStatusMessage(status) {
    return `Error: MAIN world execution FAILED. Fallback in ISOLATED is ${status}.`;
}
function runHighlightFallback(params) {
    const selector = typeof params?.selector === 'string' && params.selector.trim().length > 0 ? params.selector : 'body';
    const rawDuration = typeof params?.duration_ms === 'number' ? params.duration_ms : 5000;
    const durationMs = Math.max(0, Math.min(30000, Math.floor(rawDuration)));
    try {
        const target = document.querySelector(selector);
        if (!target) {
            return {
                success: false,
                error: `Element not found: ${selector}`,
                selector
            };
        }
        const existing = document.getElementById('kaboom-highlighter');
        existing?.remove();
        const rect = target.getBoundingClientRect();
        const overlay = document.createElement('div');
        overlay.id = 'kaboom-highlighter';
        overlay.dataset.selector = selector;
        overlay.style.position = 'fixed';
        overlay.style.top = `${rect.top}px`;
        overlay.style.left = `${rect.left}px`;
        overlay.style.width = `${rect.width}px`;
        overlay.style.height = `${rect.height}px`;
        overlay.style.border = '4px solid red';
        overlay.style.backgroundColor = 'rgba(255, 0, 0, 0.1)';
        overlay.style.zIndex = '2147483647';
        overlay.style.pointerEvents = 'none';
        overlay.style.boxSizing = 'border-box';
        overlay.style.borderRadius = '4px';
        (document.body || document.documentElement).appendChild(overlay);
        const syncOverlay = () => {
            const element = document.querySelector(selector);
            if (!element || !overlay.isConnected)
                return;
            const bounds = element.getBoundingClientRect();
            overlay.style.top = `${bounds.top}px`;
            overlay.style.left = `${bounds.left}px`;
            overlay.style.width = `${bounds.width}px`;
            overlay.style.height = `${bounds.height}px`;
        };
        const onViewportChange = () => syncOverlay();
        window.addEventListener('scroll', onViewportChange, { passive: true });
        window.addEventListener('resize', onViewportChange, { passive: true });
        setTimeout(() => {
            window.removeEventListener('scroll', onViewportChange);
            window.removeEventListener('resize', onViewportChange);
            overlay.remove();
        }, durationMs);
        return {
            success: true,
            selector,
            duration_ms: durationMs,
            bounds: {
                x: rect.x,
                y: rect.y,
                width: rect.width,
                height: rect.height
            }
        };
    }
    catch (err) {
        return {
            success: false,
            error: 'highlight_fallback_failed',
            message: errorMessage(err, 'Highlight fallback failed')
        };
    }
}
async function executeHighlightFallback(tabId, params, mainWorldError) {
    try {
        const execution = await chrome.scripting.executeScript({
            target: { tabId },
            world: 'ISOLATED',
            func: runHighlightFallback,
            args: [typeof params === 'object' && params ? params : {}]
        });
        const first = execution?.[0]?.result;
        const payload = first && typeof first === 'object' ? first : {};
        const success = payload.success !== false;
        const fallbackStatus = success ? 'SUCCESS' : 'ERROR';
        return {
            ...payload,
            execution_world: 'ISOLATED',
            fallback_reason: 'content_script_unreachable',
            fallback_status: fallbackStatus,
            main_world_error: mainWorldError,
            message: typeof payload.message === 'string' && payload.message.length > 0
                ? payload.message
                : buildFallbackStatusMessage(fallbackStatus)
        };
    }
    catch (err) {
        const fallbackError = errorMessage(err, 'highlight_fallback_failed');
        return {
            success: false,
            error: 'highlight_fallback_failed',
            execution_world: 'ISOLATED',
            fallback_reason: 'content_script_unreachable',
            fallback_status: 'ERROR',
            main_world_error: mainWorldError,
            fallback_error: fallbackError,
            message: `${buildFallbackStatusMessage('ERROR')} ${fallbackError}`
        };
    }
}
async function handlePilotCommandOnTab(tabId, command, params) {
    try {
        const result = await chrome.tabs.sendMessage(tabId, {
            type: command,
            params
        });
        return result || { success: true };
    }
    catch (err) {
        if (command === 'kaboom_highlight' && isContentScriptUnreachableError(err)) {
            return executeHighlightFallback(tabId, params, errorMessage(err, 'command_failed'));
        }
        throw err;
    }
}
export async function handlePilotCommand(command, params, preferredTabId) {
    if (!isAiWebPilotEnabled()) {
        return { error: 'ai_web_pilot_disabled' };
    }
    try {
        if (typeof preferredTabId === 'number' && Number.isFinite(preferredTabId) && preferredTabId > 0) {
            return await handlePilotCommandOnTab(preferredTabId, command, params);
        }
        const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
        const firstTab = tabs[0];
        const tabId = firstTab?.id;
        if (!tabId) {
            return { error: 'no_active_tab' };
        }
        return await handlePilotCommandOnTab(tabId, command, params);
    }
    catch (err) {
        return { error: errorMessage(err, 'command_failed') };
    }
}
//# sourceMappingURL=interact.js.map