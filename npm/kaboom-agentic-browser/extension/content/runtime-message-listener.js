// runtime-message-listener.ts — Message routing between background and content contexts.
import { KABOOM_LOG_PREFIX } from '../lib/brand.js';
import { SettingName } from '../lib/constants.js';
import { isValidBackgroundSender, handlePing, handleToggleMessage, forwardHighlightMessage, handleStateCommand, handleExecuteJs, handleExecuteQuery, handleA11yQuery, handleDomQuery, handleGetNetworkWaterfall, handleLinkHealthQuery, handleComputedStylesQuery, handleFormDiscoveryQuery, handleFormStateQuery, handleDataTableQuery, handleGetReadable, handleGetMarkdown, handlePageSummary } from './message-handlers.js';
import { showActionToast } from './ui/toast.js';
import { showSubtitle, toggleRecordingWatermark } from './ui/subtitle.js';
import { toggleChatWidget } from './ui/chat-widget.js';
// Toggle state caches — updated by forwarded setting messages from background
let actionToastsEnabled = true;
let subtitlesEnabled = true;
function applyOverlayToggleState(result) {
    if (result.actionToastsEnabled !== undefined)
        actionToastsEnabled = result.actionToastsEnabled;
    if (result.subtitlesEnabled !== undefined)
        subtitlesEnabled = result.subtitlesEnabled;
}
function hydrateOverlayToggleState() {
    if (typeof chrome === 'undefined' || !chrome.storage?.local)
        return;
    try {
        const maybePromise = chrome.storage.local.get(['actionToastsEnabled', 'subtitlesEnabled'], applyOverlayToggleState);
        if (maybePromise && typeof maybePromise.then === 'function') {
            void maybePromise.then((result) => applyOverlayToggleState(result));
        }
    }
    catch {
        // Storage hydration is best-effort. Keep defaults if the content context cannot read storage.
    }
}
/**
 * Initialize runtime message listener
 * Listens for messages from background (feature toggles and pilot commands)
 */
export function initRuntimeMessageListener() {
    actionToastsEnabled = true;
    subtitlesEnabled = true;
    hydrateOverlayToggleState();
    const syncHandlers = {
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
    const delegatedHandlers = {
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
    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
        if (!isValidBackgroundSender(sender)) {
            console.warn(KABOOM_LOG_PREFIX, 'Rejected message from untrusted sender:', sender.id);
            return false;
        }
        // Ping is special: sync handler that needs sendResponse
        if (message.type === 'kaboom_ping')
            return handlePing(sendResponse);
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