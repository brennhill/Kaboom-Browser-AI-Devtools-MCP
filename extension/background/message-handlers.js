/**
 * Purpose: Routes all chrome.runtime.onMessage events to type-safe handlers for logs, settings, screenshots, and state management.
 * Why: Centralizes message validation and sender security checks in one place.
 */
import { SettingName, StorageKey, DEFAULT_SERVER_URL } from '../lib/constants.js';
import { KABOOM_LOG_PREFIX } from '../lib/brand.js';
import { pushChatMessage } from './push-handler.js';
import { errorMessage } from '../lib/error-utils.js';
import { postDaemonJSON } from '../lib/daemon-http.js';
import { getLocal, getLocals, setLocal } from '../lib/storage-utils.js';
import { resolveTerminalWorkspaceTarget, setKaboomOverlayVisibility } from './tab-state.js';
import { trackUIFeature } from './ui-usage-tracker.js';
// Static import: dynamic import() is not allowed in MV3 service workers.
// terminal-widget-types.js is a pure helper module (constants + types, no DOM access at load time).
import { getTerminalServerUrl } from '../content/ui/terminal-widget-types.js';
async function openTerminalSidePanel(tabId) {
    if (typeof chrome === 'undefined' || !chrome.sidePanel?.open) {
        return { success: false, error: 'side panel unavailable' };
    }
    try {
        const workspace = await resolveTerminalWorkspaceTarget(tabId);
        if (!workspace) {
            return { success: false, error: 'missing workspace tab' };
        }
        const path = `sidepanel.html?tabId=${encodeURIComponent(workspace.hostTabId)}&tabGroupId=${encodeURIComponent(workspace.tabGroupId)}&mainTabId=${encodeURIComponent(workspace.mainTabId)}`;
        const setOptionsPromise = chrome.sidePanel.setOptions
            ? chrome.sidePanel
                .setOptions({ tabId: workspace.hostTabId, path, enabled: true })
                .catch(() => undefined)
            : null;
        // sidePanel.open must stay in the original user-gesture path. Awaiting
        // another async API first can cause Chrome to reject the open request.
        await chrome.sidePanel.open({ tabId: workspace.hostTabId });
        void setOptionsPromise;
        return { success: true };
    }
    catch (error) {
        return { success: false, error: errorMessage(error) };
    }
}
// =============================================================================
// MESSAGE HANDLER
// =============================================================================
/**
 * Security: Validate that sender is from extension or content script
 * Prevents messages from untrusted sources
 */
function isValidMessageSender(sender) {
    // Content scripts have sender.tab with tabId and url
    // Background/popup scripts have sender.id === chrome.runtime.id
    // Extension pages (popup, options) have sender.tab?.url starting with 'chrome-extension://'
    if (sender.tab?.id !== undefined && sender.tab?.url) {
        // Content script: has tab context
        return true;
    }
    if (typeof chrome !== 'undefined' && chrome.runtime && sender.id === chrome.runtime.id) {
        // Internal extension message
        return true;
    }
    // Reject messages from web pages
    return false;
}
/**
 * Install the main message listener
 * All messages are validated for sender origin to ensure they come from trusted extension contexts
 */
// #lizard forgives
export function installMessageListener(deps) {
    if (typeof chrome === 'undefined' || !chrome.runtime)
        return;
    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
        // SECURITY: Validate sender before processing any message
        if (!isValidMessageSender(sender)) {
            deps.debugLog('error', 'Rejected message from untrusted sender', { senderId: sender.id, senderUrl: sender.url });
            return false;
        }
        return handleMessage(message, sender, sendResponse, deps);
    });
}
/**
 * Type guard to validate message structure before processing
 * Returns true if message passes validation, logs rejection otherwise
 */
function validateMessageType(message, expectedType, deps) {
    if (typeof message !== 'object' || message === null) {
        deps.debugLog('error', `Invalid message: not an object`, { messageType: typeof message });
        return false;
    }
    const msg = message;
    if (msg.type !== expectedType) {
        deps.debugLog('error', `Message type mismatch`, { expected: expectedType, received: msg.type });
        return false;
    }
    return true;
}
/**
 * Handle incoming message
 * Returns true if response will be sent asynchronously
 * Security: All messages are type-validated using discriminated unions
 */
function handleMessage(message, sender, sendResponse, deps) {
    const messageType = message.type;
    // Type validation: ensure message conforms to expected discriminated union
    // TypeScript's type system ensures exhaustiveness, but add logging for debugging
    switch (messageType) {
        case 'get_tab_id':
            sendResponse({ tabId: sender.tab?.id });
            return true;
        case 'ws_event':
            deps.addToWsBatcher(message.payload);
            return false;
        case 'enhanced_action':
            deps.addToEnhancedActionBatcher(message.payload);
            return false;
        case 'network_body':
            if (deps.isNetworkBodyCaptureDisabled()) {
                deps.debugLog('capture', 'Network body dropped: capture disabled');
                // Fire-and-forget: returning true without calling sendResponse would
                // reject the sender's awaited sendMessage with "message port closed".
                return false;
            }
            // Attach tab_id from sender before batching (v5.3+)
            deps.addToNetworkBodyBatcher({ ...message.payload, tab_id: message.payload.tab_id ?? message.tabId });
            return false;
        case 'performance_snapshot':
            deps.addToPerfBatcher(message.payload);
            return false;
        case 'log':
            // Fire-and-forget: content scripts never await a response for log messages.
            // Returning true without calling sendResponse would reject awaited senders.
            handleLogMessageAsync(message, sender, deps);
            return false;
        case 'get_status':
            sendResponse({
                ...deps.getConnectionStatus(),
                serverUrl: deps.getServerUrl(),
                screenshotOnError: deps.getScreenshotOnError(),
                sourceMapEnabled: deps.getSourceMapEnabled(),
                debugMode: deps.getDebugMode(),
                contextWarning: deps.getContextWarning(),
                circuitBreakerState: deps.getCircuitBreakerState(),
                memoryPressure: deps.getMemoryPressureState()
            });
            return false;
        case 'clear_logs':
            handleClearLogsAsync(sendResponse, deps);
            return true;
        case 'set_log_level':
            deps.setCurrentLogLevel(message.level);
            deps.saveSetting(StorageKey.LOG_LEVEL, message.level);
            return false;
        case 'set_screenshot_on_error':
            deps.setScreenshotOnError(message.enabled);
            deps.saveSetting(StorageKey.SCREENSHOT_ON_ERROR, message.enabled);
            sendResponse({ success: true });
            return false;
        case 'set_ai_web_pilot_enabled':
            handleSetAiWebPilotEnabled(message.enabled, sendResponse, deps);
            return false;
        case 'get_ai_web_pilot_enabled':
            sendResponse({ enabled: deps.getAiWebPilotEnabled() });
            return false;
        case 'open_terminal_panel':
            openTerminalSidePanel(sender.tab?.id)
                .then((result) => sendResponse(result))
                .catch((error) => sendResponse({ success: false, error: errorMessage(error) }));
            return true;
        case 'get_tracking_state':
            handleGetTrackingState(sendResponse, deps, sender.tab?.id);
            return true;
        case 'get_diagnostic_state':
            handleGetDiagnosticState(sendResponse, deps);
            return true;
        case 'capture_screenshot':
            trackUIFeature('screenshot');
            handleCaptureScreenshot(sendResponse, deps);
            return true;
        case 'set_source_map_enabled':
            deps.setSourceMapEnabled(message.enabled);
            deps.saveSetting(StorageKey.SOURCE_MAP_ENABLED, message.enabled);
            if (!message.enabled) {
                deps.clearSourceMapCache();
            }
            sendResponse({ success: true });
            return false;
        case 'set_network_waterfall_enabled':
        case 'set_performance_marks_enabled':
        case 'set_action_replay_enabled':
        case 'set_web_socket_capture_enabled':
        case 'set_web_socket_capture_mode':
        case 'set_performance_snapshot_enabled':
        case 'set_deferral_enabled':
        case 'set_network_body_capture_enabled':
        case 'set_action_toasts_enabled':
        case 'set_subtitles_enabled':
            handleForwardedSetting(message, sendResponse, deps);
            return false;
        case 'set_debug_mode':
            deps.setDebugMode(message.enabled);
            deps.saveSetting(StorageKey.DEBUG_MODE, message.enabled);
            sendResponse({ success: true });
            return false;
        case 'get_debug_log':
            sendResponse({ log: deps.exportDebugLog() });
            return false;
        case 'clear_debug_log':
            deps.clearDebugLog();
            deps.debugLog('lifecycle', 'Debug log cleared');
            sendResponse({ success: true });
            return false;
        case 'set_server_url':
            handleSetServerUrl(message.url, sendResponse, deps);
            return false;
        case 'kaboom_capture_screenshot':
            // Content script requests screenshot capture (while draw mode overlay is still visible)
            handleDrawModeCaptureScreenshot(sender, sendResponse);
            return true;
        case 'kaboom_push_chat':
            handlePushChatAsync(message, sender, sendResponse);
            return true;
        case 'draw_mode_completed':
            // Fire-and-forget: content script sends draw mode results
            handleDrawModeCompletedAsync(message, sender, deps);
            return false;
        case 'qa_scan_requested':
            handleQaScanRequestedAsync(message, sendResponse, deps);
            return true;
        default:
            // screen_recording_start/stop, offscreen_*, mic_granted_close_tab, reveal_file
            // are handled by recording-listeners.ts — return false so they can handle it.
            return false;
    }
}
// =============================================================================
// ASYNC HANDLERS
// =============================================================================
async function handleLogMessageAsync(message, sender, deps) {
    try {
        await deps.handleLogMessage(message.payload, sender, message.tabId);
    }
    catch (err) {
        console.error(`${KABOOM_LOG_PREFIX} Failed to handle log message:`, err);
    }
}
// #lizard forgives
async function handleClearLogsAsync(sendResponse, deps) {
    try {
        const result = await deps.handleClearLogs();
        sendResponse(result);
    }
    catch (err) {
        console.error(`${KABOOM_LOG_PREFIX} Failed to clear logs:`, err);
        sendResponse({ error: errorMessage(err) });
    }
}
function handleSetAiWebPilotEnabled(enabled, sendResponse, deps) {
    const newValue = enabled === true;
    console.log(`${KABOOM_LOG_PREFIX} AI Web Pilot toggle: -> ${newValue}`);
    deps.setAiWebPilotEnabled(newValue, () => {
        console.log(`${KABOOM_LOG_PREFIX} AI Web Pilot persisted to storage: ${newValue}`);
        // Settings now sent automatically via /sync
        // Broadcast tracking state change to tracked tab (for favicon flicker)
        broadcastTrackingState();
    });
    sendResponse({ success: true });
}
/**
 * Handle getTrackingState request from content script.
 * Returns current tracking and AI Pilot state for favicon replacer.
 * Uses sender's tab ID (not active tab query) to correctly identify the requesting tab.
 */
async function handleGetTrackingState(sendResponse, deps, senderTabId) {
    try {
        const trackedTabId = (await getLocal(StorageKey.TRACKED_TAB_ID));
        const aiPilotEnabled = deps.getAiWebPilotEnabled();
        sendResponse({
            state: {
                isTracked: senderTabId !== undefined && senderTabId === trackedTabId,
                aiPilotEnabled: aiPilotEnabled
            }
        });
    }
    catch (err) {
        console.error(`${KABOOM_LOG_PREFIX} Failed to get tracking state:`, err);
        sendResponse({ state: { isTracked: false, aiPilotEnabled: false } });
    }
}
/**
 * Broadcast tracking state to the tracked tab.
 * Used by favicon replacer to show/hide flicker animation.
 * Exported for use in init.ts storage change handlers.
 * @param untrackedTabId - Optional tab ID that was just untracked (to notify it to stop flicker)
 */
export async function broadcastTrackingState(untrackedTabId) {
    try {
        const result = await getLocals([StorageKey.TRACKED_TAB_ID, StorageKey.AI_WEB_PILOT_ENABLED]);
        const trackedTabId = result[StorageKey.TRACKED_TAB_ID];
        const aiPilotEnabled = result[StorageKey.AI_WEB_PILOT_ENABLED] === true;
        // Notify the currently tracked tab it's being tracked
        if (trackedTabId) {
            chrome.tabs
                .sendMessage(trackedTabId, {
                type: 'tracking_state_changed',
                state: {
                    isTracked: true,
                    aiPilotEnabled: aiPilotEnabled
                }
            })
                .catch(() => {
                // Tab might not have content script loaded yet, ignore
            });
        }
        // Notify the previously tracked tab it's no longer tracked (to stop favicon flicker)
        if (untrackedTabId && untrackedTabId !== trackedTabId) {
            chrome.tabs
                .sendMessage(untrackedTabId, {
                type: 'tracking_state_changed',
                state: {
                    isTracked: false,
                    aiPilotEnabled: false
                }
            })
                .catch(() => {
                // Tab might not have content script loaded, ignore
            });
        }
    }
    catch (err) {
        console.error(`${KABOOM_LOG_PREFIX} Failed to broadcast tracking state:`, err);
    }
}
async function handleGetDiagnosticState(sendResponse, deps) {
    if (typeof chrome === 'undefined' || !chrome.storage) {
        sendResponse({
            cache: deps.getAiWebPilotEnabled(),
            storage: undefined,
            timestamp: new Date().toISOString()
        });
        return;
    }
    const value = await getLocal(StorageKey.AI_WEB_PILOT_ENABLED);
    sendResponse({
        cache: deps.getAiWebPilotEnabled(),
        storage: value,
        timestamp: new Date().toISOString()
    });
}
function handleCaptureScreenshot(sendResponse, deps) {
    deps.debugLog('capture', 'handleCaptureScreenshot ENTER');
    if (typeof chrome === 'undefined' || !chrome.tabs) {
        deps.debugLog('capture', 'handleCaptureScreenshot: no chrome.tabs');
        sendResponse({ success: false, error: 'Chrome tabs API not available' });
        return;
    }
    chrome.tabs.query({ active: true, currentWindow: true }, async (tabs) => {
        deps.debugLog('capture', 'handleCaptureScreenshot: tabs.query', { count: tabs.length, tabId: tabs[0]?.id });
        if (tabs[0]?.id) {
            try {
                const result = await deps.captureScreenshot(tabs[0].id, null);
                deps.debugLog('capture', 'handleCaptureScreenshot: result', { success: result.success, error: result.error });
                if (result.success && result.entry) {
                    deps.addToLogBatcher(result.entry);
                }
                sendResponse(result);
            }
            catch (err) {
                deps.debugLog('error', 'handleCaptureScreenshot: EXCEPTION', { error: errorMessage(err) });
                sendResponse({ success: false, error: errorMessage(err) });
            }
        }
        else {
            deps.debugLog('capture', 'handleCaptureScreenshot: no active tab');
            sendResponse({ success: false, error: 'No active tab' });
        }
    });
}
function handleForwardedSetting(message, sendResponse, deps) {
    deps.debugLog('settings', `Setting ${message.type}: ${message.enabled ?? message.mode}`);
    deps.forwardToAllContentScripts(message);
    sendResponse({ success: true });
}
/**
 * Handle KABOOM_CAPTURE_SCREENSHOT from content script.
 * Captures visible tab while draw mode overlay is still visible (annotations in screenshot).
 */
async function handleDrawModeCaptureScreenshot(sender, sendResponse) {
    const tabId = sender.tab?.id;
    if (!tabId) {
        sendResponse({ dataUrl: '' });
        return;
    }
    try {
        const tab = await chrome.tabs.get(tabId);
        await setKaboomOverlayVisibility(tabId, false);
        const dataUrl = await chrome.tabs.captureVisibleTab(tab.windowId, { format: 'png' });
        await setKaboomOverlayVisibility(tabId, true);
        sendResponse({ dataUrl });
    }
    catch (err) {
        console.error(`${KABOOM_LOG_PREFIX} Draw mode screenshot capture failed:`, errorMessage(err));
        await setKaboomOverlayVisibility(tabId, true).catch(() => { });
        sendResponse({ dataUrl: '' });
    }
}
/**
 * Handle draw mode completion from content script.
 * Uses screenshot already captured by content script (before overlay removal).
 */
async function handleDrawModeCompletedAsync(message, sender, deps) {
    const tabId = sender.tab?.id;
    if (!tabId)
        return;
    try {
        const serverUrl = deps.getServerUrl();
        const body = {
            screenshot_data_url: message.screenshot_data_url || '',
            annotations: message.annotations || [],
            element_details: message.elementDetails || {},
            page_url: message.page_url || '',
            tab_id: tabId,
            correlation_id: message.correlation_id || ''
        };
        if (message.annot_session_name) {
            body.annot_session_name = message.annot_session_name;
        }
        const response = await postDaemonJSON(`${serverUrl}/draw-mode/complete`, body);
        if (!response.ok) {
            const respBody = await response.text().catch(() => '');
            deps.debugLog('error', `Draw mode POST failed: ${response.status} ${respBody}`);
        }
        else {
            deps.debugLog('draw', `Draw mode results delivered (${message.annotations?.length || 0} annotations)`);
        }
    }
    catch (err) {
        deps.debugLog('error', `Draw mode completion error: ${errorMessage(err)}. Server may be unreachable.`);
    }
}
/**
 * Handle KABOOM_PUSH_CHAT from content script (chat widget).
 * Pushes a text message to the daemon's push pipeline.
 */
async function handlePushChatAsync(message, sender, sendResponse) {
    try {
        const tabId = sender.tab?.id ?? 0;
        const result = await pushChatMessage(message.message, message.page_url, tabId);
        if (result) {
            sendResponse({ success: true, status: result.status, event_id: result.event_id });
        }
        else {
            sendResponse({ success: false, error: 'Failed to push message' });
        }
    }
    catch (err) {
        sendResponse({ success: false, error: errorMessage(err) });
    }
}
function handleSetServerUrl(url, sendResponse, deps) {
    deps.setServerUrl(url || DEFAULT_SERVER_URL);
    deps.saveSetting(StorageKey.SERVER_URL, deps.getServerUrl());
    deps.debugLog('settings', `Server URL changed to: ${deps.getServerUrl()}`);
    // Broadcast to all content scripts
    deps.forwardToAllContentScripts({ type: SettingName.SERVER_URL, url: deps.getServerUrl() });
    // Re-check connection with new URL
    deps.checkConnectionAndUpdate();
    sendResponse({ success: true });
}
// =============================================================================
// STATE SNAPSHOT STORAGE
// =============================================================================
const SNAPSHOT_KEY = 'kaboom_state_snapshots';
/**
 * Save a state snapshot to persistent storage
 */
export async function saveStateSnapshot(name, state) {
    const existing = (await getLocal(SNAPSHOT_KEY));
    const snapshots = existing || {};
    const sizeBytes = JSON.stringify(state).length; // nosemgrep: no-stringify-keys
    snapshots[name] = { ...state, name, size_bytes: sizeBytes };
    await setLocal(SNAPSHOT_KEY, snapshots);
    return { success: true, snapshot_name: name, size_bytes: sizeBytes };
}
/**
 * Load a state snapshot from persistent storage
 */
export async function loadStateSnapshot(name) {
    const existing = (await getLocal(SNAPSHOT_KEY));
    const snapshots = existing || {};
    return snapshots[name] || null;
}
/**
 * List all state snapshots with metadata
 */
export async function listStateSnapshots() {
    const existing = (await getLocal(SNAPSHOT_KEY));
    const snapshots = existing || {};
    return Object.values(snapshots).map((s) => ({
        name: s.name,
        url: s.url,
        timestamp: s.timestamp,
        size_bytes: s.size_bytes
    }));
}
/**
 * Delete a state snapshot from persistent storage
 */
export async function deleteStateSnapshot(name) {
    const existing = (await getLocal(SNAPSHOT_KEY));
    const snapshots = existing || {};
    delete snapshots[name];
    await setLocal(SNAPSHOT_KEY, snapshots);
    return { success: true, deleted: name };
}
// =============================================================================
// QA SCAN INTENT HANDLER
// =============================================================================
// Single-line prompt: PTY interprets \n as Enter, so multi-line text would execute as separate commands.
const QA_SCAN_PROMPT = 'The user clicked "Audit". Run the KaBOOM! audit workflow for the tracked site. Use /kaboom/audit if available, otherwise /audit, and produce the six-lane Phase 1 report.';
const QA_SCAN_FETCH_TIMEOUT_MS = 3000;
async function handleQaScanRequestedAsync(message, sendResponse, deps) {
    const termUrl = getTerminalServerUrl(deps.getServerUrl());
    // Try PTY injection first — works whether the side panel is open or closed.
    try {
        const controller = new AbortController();
        const timer = setTimeout(() => controller.abort(), QA_SCAN_FETCH_TIMEOUT_MS);
        const resp = await fetch(`${termUrl}/terminal/inject`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ text: QA_SCAN_PROMPT }),
            signal: controller.signal
        });
        clearTimeout(timer);
        if (resp.ok) {
            const result = (await resp.json());
            if (result.injected) {
                sendResponse({ success: true, method: 'terminal_inject' });
                return;
            }
        }
    }
    catch {
        // Terminal server unreachable or no active session — fall through to intent.
    }
    // Fallback: store intent on the daemon for the AI to pick up via tool response.
    try {
        const controller = new AbortController();
        const timer = setTimeout(() => controller.abort(), QA_SCAN_FETCH_TIMEOUT_MS);
        const resp = await fetch(`${termUrl}/intent`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                page_url: message.page_url || '',
                action: 'qa_scan'
            }),
            signal: controller.signal
        });
        clearTimeout(timer);
        if (resp.ok) {
            sendResponse({ success: true, method: 'intent_stored' });
            return;
        }
    }
    catch {
        // Intent endpoint also unreachable.
    }
    sendResponse({ success: false, error: 'No terminal session and intent store unreachable' });
}
//# sourceMappingURL=message-handlers.js.map