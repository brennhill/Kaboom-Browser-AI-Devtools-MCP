// runtime-message-listener.ts — Message routing between background and content contexts.
import { KABOOM_LOG_PREFIX } from '../lib/brand.js';
import { SettingName } from '../lib/constants.js';
import { getLocals } from '../lib/storage/local.js';
import { reportStateRecovery, resolveStateRecovery } from '../lib/storage/recovery.js';
import { isValidBackgroundSender, handlePing, handleToggleMessage, forwardHighlightMessage, handleStateCommand, handleExecuteJs, handleExecuteQuery, handleA11yQuery, handleDomQuery, handleGetNetworkWaterfall, handleLinkHealthQuery, handleComputedStylesQuery, handleFormDiscoveryQuery, handleFormStateQuery, handleDataTableQuery, handleGetReadable, handleGetMarkdown, handlePageSummary } from './message-handlers.js';
import { showActionToast } from './ui/toast.js';
import { showSubtitle, toggleRecordingWatermark } from './ui/subtitle.js';
import { toggleChatWidget } from './ui/chat-widget.js';
import { AgentIndicator } from './ui/supervision/agent-indicator.js';
// Toggle state caches — updated by forwarded setting messages from background
let actionToastsEnabled = true;
let subtitlesEnabled = true;
function applyOverlayToggleState(result) {
    const actionToasts = result.actionToastsEnabled;
    const subtitles = result.subtitlesEnabled;
    if ((actionToasts !== undefined && typeof actionToasts !== 'boolean') ||
        (subtitles !== undefined && typeof subtitles !== 'boolean')) {
        reportStateRecovery({
            name: 'overlay_settings_state',
            detail: 'Saved overlay settings were malformed; enabled defaults are active.',
            fix: 'Open extension settings and save overlay preferences again.'
        });
        return;
    }
    if (typeof actionToasts === 'boolean')
        actionToastsEnabled = actionToasts;
    if (typeof subtitles === 'boolean')
        subtitlesEnabled = subtitles;
    resolveStateRecovery('overlay_settings_state');
}
function hydrateOverlayToggleState() {
    void getLocals(['actionToastsEnabled', 'subtitlesEnabled'])
        .then(applyOverlayToggleState)
        .catch(() => {
        reportStateRecovery({
            name: 'overlay_settings_state',
            detail: 'Saved overlay settings could not be read; enabled defaults are active.',
            fix: 'Reload the extension, then save overlay preferences again.'
        });
    });
}
/**
 * One supervision overlay per page.
 *
 * Created lazily so a tab nobody drives never builds one. The heartbeat timer is what makes
 * the overlay survivable: if the service worker is terminated mid-action (MV3 does this
 * without warning) no further heartbeats arrive and the overlay removes itself, instead of
 * stranding a permanent "an agent is driving this tab" badge on a tab nothing is driving.
 */
let agentIndicator = null;
let agentIndicatorTimer = null;
/** How often the overlay checks its own liveness. */
const AGENT_INDICATOR_TICK_MS = 2_000;
function ensureAgentIndicator() {
    if (agentIndicator)
        return agentIndicator;
    agentIndicator = new AgentIndicator({
        now: () => Date.now(),
        onStop: () => {
            // The user clicked Stop on a trusted event. Tell the background so it aborts the
            // in-flight action and releases the CDP lease; a page cannot reach this path.
            chrome.runtime
                .sendMessage({ type: 'kaboom_agent_stop_requested', at: Date.now() })
                .catch(() => {
                // EXPECTED_ABSENCE: a dead background worker is a normal outcome here — it is
                // itself one of the reasons a user presses Stop — and the overlay has already
                // torn itself down locally, so logging would report a failure for an abort that
                // actually succeeded from the user's point of view.
            });
        }
    });
    return agentIndicator;
}
function stopAgentIndicatorTimer() {
    if (agentIndicatorTimer === null)
        return;
    clearInterval(agentIndicatorTimer);
    agentIndicatorTimer = null;
}
function handleAgentIndicatorMessage(msg) {
    const indicator = ensureAgentIndicator();
    switch (msg.phase) {
        case 'driving':
            indicator.startDriving(msg.action ?? '');
            if (agentIndicatorTimer === null) {
                agentIndicatorTimer = setInterval(() => {
                    if (indicator.tick() !== null)
                        stopAgentIndicatorTimer();
                }, AGENT_INDICATOR_TICK_MS);
            }
            break;
        case 'cursor':
            if (typeof msg.x === 'number' && typeof msg.y === 'number')
                indicator.moveCursor(msg.x, msg.y);
            break;
        case 'heartbeat':
            indicator.heartbeat();
            break;
        case 'idle':
            indicator.unmount();
            stopAgentIndicatorTimer();
            break;
        default:
            // EXPECTED_ABSENCE: an unknown phase is the normal, expected symptom of version skew
            // between background and content immediately after an extension update. Ignoring it
            // leaves the overlay in its last valid state; logging would flag a routine upgrade
            // window as an error on every message until the content script reloads.
            break;
    }
}
/** Sync message handlers — return false (no async response needed). Extracted so
 *  initRuntimeMessageListener stays inside its length budget. */
function buildSyncHandlers() {
    return {
        kaboom_ping: () => {
            /* handled below via sendResponse */
        },
        kaboom_action_toast: (msg) => {
            if (!actionToastsEnabled)
                return false;
            const m = msg;
            if (m.text)
                showActionToast(m.text, m.detail, m.state || 'trying', m.duration_ms);
            return false;
        },
        kaboom_agent_indicator: (msg) => {
            handleAgentIndicatorMessage(msg);
            return false;
        },
        kaboom_toggle_chat: (msg) => {
            toggleChatWidget(msg.client_name);
            return false;
        },
        kaboom_recording_watermark: (msg) => {
            toggleRecordingWatermark(msg.visible ?? false);
            return false;
        },
        kaboom_subtitle: (msg) => {
            if (!subtitlesEnabled)
                return false;
            showSubtitle(msg.text ?? '');
            return false;
        },
        [SettingName.ACTION_TOASTS]: (msg) => {
            actionToastsEnabled = msg.enabled;
            return false;
        },
        [SettingName.SUBTITLES]: (msg) => {
            subtitlesEnabled = msg.enabled;
            return false;
        }
    };
}
/** Delegated handlers that own their own sendResponse lifecycle. */
function buildDelegatedHandlers() {
    return {
        kaboom_draw_mode_start: (msg, sr) => {
            const m = msg;
            import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'))
                .then((mod) => {
                const result = mod.activateDrawMode(m.started_by || 'user', m.annot_session_name || '', m.correlation_id || '');
                sr(result);
            })
                .catch((e) => sr({ error: 'draw_mode_load_failed', message: e.message }));
            return true;
        },
        kaboom_draw_mode_stop: (_msg, sr) => {
            import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'))
                .then((mod) => {
                const result = mod.deactivateAndSendResults?.() || mod.deactivateDrawMode?.();
                sr(result || { status: 'stopped' });
            })
                .catch((e) => sr({ error: 'draw_mode_load_failed', message: e.message }));
            return true;
        },
        kaboom_get_annotations: (_msg, sr) => {
            import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'))
                .then((mod) => {
                sr({ draw_mode_active: mod.isDrawModeActive?.() ?? false });
            })
                .catch(() => sr({ draw_mode_active: false }));
            return true;
        },
        kaboom_highlight: (msg, sr) => {
            forwardHighlightMessage({ params: msg.params })
                .then((r) => sr(r))
                .catch((e) => sr({ success: false, error: e.message }));
            return true;
        },
        kaboom_manage_state: (msg, sr) => {
            handleStateCommand(msg.params)
                .then((r) => sr(r))
                .catch((e) => sr({ error: e.message }));
            return true;
        },
        kaboom_execute_js: (msg, sr) => handleExecuteJs(msg.params || {}, sr),
        kaboom_execute_query: (msg, sr) => handleExecuteQuery((msg.params || {}), sr),
        a11y_query: (msg, sr) => handleA11yQuery((msg.params || {}), sr),
        dom_query: (msg, sr) => handleDomQuery((msg.params || {}), sr),
        get_network_waterfall: (_msg, sr) => handleGetNetworkWaterfall(sr),
        link_health_query: (msg, sr) => handleLinkHealthQuery((msg.params ?? {}), sr),
        computed_styles_query: (msg, sr) => handleComputedStylesQuery((msg.params ?? {}), sr),
        form_discovery_query: (msg, sr) => handleFormDiscoveryQuery((msg.params ?? {}), sr),
        form_state_query: (msg, sr) => handleFormStateQuery((msg.params ?? {}), sr),
        data_table_query: (msg, sr) => handleDataTableQuery((msg.params ?? {}), sr),
        kaboom_get_readable: (_msg, sr) => handleGetReadable(sr),
        kaboom_get_markdown: (_msg, sr) => handleGetMarkdown(sr),
        kaboom_page_summary: (_msg, sr) => handlePageSummary(sr)
    };
}
export function initRuntimeMessageListener() {
    actionToastsEnabled = true;
    subtitlesEnabled = true;
    hydrateOverlayToggleState();
    const syncHandlers = buildSyncHandlers();
    const delegatedHandlers = buildDelegatedHandlers();
    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
        if (!isValidBackgroundSender(sender)) {
            console.warn(KABOOM_LOG_PREFIX, 'Rejected message from untrusted sender:', sender.id);
            return false;
        }
        // Ping is special: sync handler that needs sendResponse
        if (message.type === 'kaboom_ping')
            return handlePing(sendResponse);
        if (message.type === 'tracking_readiness_probe') {
            sendResponse({
                ready: true,
                correlation_id: message.correlation_id,
                connection_generation: message.connection_generation
            });
            return false;
        }
        // Try sync handlers first
        const syncHandler = syncHandlers[message.type]; // nosemgrep: unsafe-dynamic-method
        if (syncHandler) {
            syncHandler(message);
            return false;
        }
        // Handle toggle messages (no dispatch needed, always runs)
        handleToggleMessage(message);
        // Try delegated handlers
        const delegated = delegatedHandlers[message.type]; // nosemgrep: unsafe-dynamic-method
        if (delegated)
            return delegated(message, sendResponse);
        return undefined;
    });
}
//# sourceMappingURL=runtime-message-listener.js.map