/**
 * Purpose: Extension startup initialization -- loads settings, installs listeners, recovers state after service worker restart, and initiates first connection.
 * Docs: docs/features/feature/cold-start-queuing/index.md
 */
/**
 * @fileoverview Extension Initialization
 * Handles startup logic: loading settings, installing listeners, and initial connection setup.
 * Uses async/await for cleaner control flow (replaces callback nesting).
 */
import { getTrackedTabLostToastDetail, KABOOM_LOG_PREFIX } from '../lib/brand.js';
import { DEFAULT_SERVER_URL } from '../lib/constants.js';
import { syncTerminalPanelAvailability } from './ui/side-panel-availability.js';
import { watchTerminalPanelState } from './ui/terminal-panel.js';
import { DebugCategory } from './debug.js';
import { debugLog, setDebugMode, resetSyncClientConnection, sharedServerCircuitBreaker, logBatcher, wsBatcher, enhancedActionBatcher, networkBodyBatcher, perfBatcher, handleLogMessage, handleClearLogs, checkConnectionAndUpdate, exportDebugLog, clearDebugLog } from './index.js';
import { getServerUrl, isDebugMode, isScreenshotOnError, getCurrentLogLevel, setServerUrl, setCurrentLogLevel, setScreenshotOnError } from './runtime-state/settings-state.js';
import { getConnectionStatus } from './runtime-state/connection-state.js';
import { isAiWebPilotEnabled, isAiWebPilotCacheInitialized, getPilotInitCallback, setAiWebPilotEnabledCache, setAiWebPilotCacheInitialized, setPilotInitCallback } from './runtime-state/pilot-state.js';
import { markInitComplete } from './runtime-state/startup-state.js';
import { isSourceMapEnabled, setSourceMapEnabled, canTakeScreenshot, recordScreenshot, clearSourceMapCache, getMemoryPressureState, isNetworkBodyCaptureDisabled, clearScreenshotTimestamps } from './caches/cache-limits.js';
import { flushErrorGroups, cleanupStaleErrorGroups } from './caches/error-groups.js';
import { getContextWarning } from './caches/snapshots.js';
import { installStorageChangeListener, setupChromeAlarms, installAlarmListener, installTabRemovedListener, installTabUpdatedListener, installStartupListener, handleTrackedTabClosed, handleTrackedTabUrlChange } from './event-listeners.js';
import { installDrawModeCommandListener, installRecordingShortcutCommandListener, installTerminalPanelCommandListener, installScreenRecordingCommandListener } from './ui/keyboard-shortcuts.js';
import { installContextMenus } from './ui/context-menus.js';
import { saveSetting, forwardToAllContentScripts, getActiveTab, sendTabToast, loadDebugModeState, loadAiWebPilotState, loadSavedSettings } from './ui/tab-state.js';
import { installPushCommandListener, installChatCommandListener } from './push-handler.js';
import { isRecording, startRecording, stopRecording, initRecording } from './recording/index.js';
import { installMessageListener, broadcastTrackingState, createTelemetryMessageHandler, createStatusMessageHandler, createSettingsMessageHandler, createPilotMessageHandler, createCaptureMessageHandler, createUtilityMessageHandler } from './message-handlers.js';
import { captureScreenshot } from './sync/screenshot.js';
import { updateBadge } from './sync/server.js';
import { getLocal, setLocal } from '../lib/storage/local.js';
import { markStateVersion, setSessionAccessLevel, wasServiceWorkerRestarted } from '../lib/storage/session.js';
import { loadServerInstallId } from './sync/install-identity.js';
/**
 * Initialize the extension on startup
 * Handles state recovery after service worker restart, loads settings, installs listeners.
 * Uses async/await for readable, linear control flow.
 */
export function initializeExtension() {
    if (typeof chrome === 'undefined' || !chrome.runtime) {
        return;
    }
    // Synchronously, before any await: when the service worker is woken *by* the
    // side panel reconnecting, Chrome dispatches that connect right after the top
    // level runs. A listener installed later in the async sequence would miss it,
    // and the background would believe no panel exists.
    watchTerminalPanelState();
    // Rehydrate any recording that survived a service-worker restart. Explicit
    // call (formerly a recording-module import side effect) so it fires exactly
    // once, here at startup. Best-effort — not awaited at the top level.
    void initRecording();
    // Fire async initialization without awaiting at top level
    // (Service worker will remain alive as long as event handlers are installed)
    initializeExtensionAsync().catch((err) => {
        console.error(`${KABOOM_LOG_PREFIX} Failed to initialize extension:`, err);
    });
}
/**
 * Async initialization sequence
 * Reads settings, installs listeners, sets up connection checking.
 */
async function initializeExtensionAsync() {
    try {
        // ============= STEP 1: Check service worker restart =============
        const wasRestarted = await wasServiceWorkerRestarted();
        if (wasRestarted) {
            console.warn(`${KABOOM_LOG_PREFIX} Service worker restarted - ephemeral state cleared. ` +
                'User preferences restored from persistent storage.');
            debugLog(DebugCategory.LIFECYCLE, 'Service worker restarted, ephemeral state recovered');
        }
        // Mark the current state version. Best-effort: a session-storage write that
        // rejects (over quota, invalidated context) must NOT abort the rest of init —
        // that would skip installing the message handler and leave the extension deaf
        // until the next worker restart. Log and continue (rule 25).
        try {
            await markStateVersion();
        }
        catch (err) {
            console.warn(`${KABOOM_LOG_PREFIX} markStateVersion failed (non-fatal):`, err);
        }
        // Allow content scripts to access chrome.storage.session (required for terminal state persistence).
        // Without this, content scripts silently fail to read/write session storage.
        try {
            await setSessionAccessLevel('TRUSTED_AND_UNTRUSTED_CONTEXTS');
        }
        catch (err) {
            console.warn(`${KABOOM_LOG_PREFIX} setSessionAccessLevel failed (non-fatal):`, err);
        }
        // ============= STEP 2: Load debug mode =============
        const debugEnabled = await loadDebugModeState();
        setDebugMode(debugEnabled);
        if (debugEnabled) {
            console.log(`${KABOOM_LOG_PREFIX} Debug mode enabled on startup`);
        }
        // ============= STEP 3: Install startup listener =============
        installStartupListener((msg) => console.log(msg));
        // ============= STEP 4: Load AI Web Pilot state =============
        const aiPilotEnabled = await loadAiWebPilotState();
        setAiWebPilotEnabledCache(aiPilotEnabled);
        setAiWebPilotCacheInitialized(true);
        console.log(`${KABOOM_LOG_PREFIX} Storage value:`, aiPilotEnabled, '| Cache value:', isAiWebPilotEnabled());
        // Execute any pending pilot init callback
        const pilotCb = getPilotInitCallback();
        if (pilotCb) {
            pilotCb();
            setPilotInitCallback(null);
        }
        // ============= STEP 5: Load saved settings =============
        const settings = await loadSavedSettings();
        setServerUrl(settings.serverUrl || DEFAULT_SERVER_URL);
        setCurrentLogLevel('all');
        setScreenshotOnError(settings.screenshotOnError === true);
        setSourceMapEnabled(settings.sourceMapEnabled !== false);
        setDebugMode(settings.debugMode || false);
        // ============= STEP 6: Install storage change listener =============
        installStorageChangeListener({
            onAiWebPilotChanged: (newValue) => {
                setAiWebPilotEnabledCache(newValue);
                console.log(`${KABOOM_LOG_PREFIX} AI Web Pilot cache updated from storage:`, newValue);
                // Reset connection when AI Web Pilot is enabled to allow immediate reconnection
                if (newValue) {
                    resetSyncClientConnection();
                    console.log(`${KABOOM_LOG_PREFIX} Sync client reset due to AI Web Pilot enabled`);
                }
                // Broadcast to tracked tab for favicon flicker
                broadcastTrackingState().catch((err) => console.error(`${KABOOM_LOG_PREFIX} Error broadcasting tracking state:`, err));
            },
            onTrackedTabChanged: (newTabId, oldTabId) => {
                // Tracking state is reflected on the next /sync poll — no separate
                // status ping needed. (Previously called a non-existent endpoint.)
                if (newTabId !== null) {
                    resetSyncClientConnection();
                    console.log(`${KABOOM_LOG_PREFIX} Sync client reset due to tracking enabled`);
                }
                else if (oldTabId !== null) {
                    // Tracking was lost — notify user on active tab
                    console.log(`${KABOOM_LOG_PREFIX} Tracking lost for tab`, oldTabId);
                    getActiveTab()
                        .then((tab) => {
                        if (tab?.id) {
                            sendTabToast(tab.id, 'Tab tracking lost', getTrackedTabLostToastDetail(), 'warning', 5000);
                        }
                    })
                        .catch(() => { });
                }
                broadcastTrackingState(oldTabId).catch((err) => console.error(`${KABOOM_LOG_PREFIX} Error broadcasting tracking state:`, err));
            }
        });
        // ============= STEP 7: Install message handler =============
        const setPilotEnabled = (enabled, callback) => {
            setLocal('aiWebPilotEnabled', enabled)
                .then(() => {
                setAiWebPilotEnabledCache(enabled);
                // Reset connection when enabling to allow immediate reconnection
                if (enabled) {
                    resetSyncClientConnection();
                    console.log(`${KABOOM_LOG_PREFIX} Sync client reset due to AI Web Pilot enabled (direct)`);
                }
                if (callback)
                    callback();
            })
                .catch((err) => {
                console.error(`${KABOOM_LOG_PREFIX} Failed to save aiWebPilotEnabled:`, err);
                if (callback)
                    callback();
            });
        };
        installMessageListener({
            debugLog,
            handlers: [
                createTelemetryMessageHandler({
                    addLog: (entry) => logBatcher.add(entry),
                    addWebSocket: (event) => wsBatcher.add(event),
                    addEnhancedAction: (action) => enhancedActionBatcher.add(action),
                    addNetworkBody: (body) => networkBodyBatcher.add(body),
                    addPerformance: (snapshot) => perfBatcher.add(snapshot),
                    handleLog: handleLogMessage,
                    isNetworkBodyCaptureDisabled,
                    debugLog
                }),
                createStatusMessageHandler({
                    getConnectionStatus,
                    getServerUrl,
                    getScreenshotOnError: isScreenshotOnError,
                    getSourceMapEnabled: isSourceMapEnabled,
                    getDebugMode: isDebugMode,
                    getContextWarning,
                    getCircuitBreakerState: () => sharedServerCircuitBreaker.getState(),
                    getMemoryPressureState,
                    clearLogs: handleClearLogs,
                    exportDebugLog,
                    clearDebugLog,
                    debugLog
                }),
                createSettingsMessageHandler({
                    getServerUrl,
                    setServerUrl,
                    setLogLevel: setCurrentLogLevel,
                    setScreenshotOnError,
                    setSourceMapEnabled,
                    setDebugMode,
                    clearSourceMapCache,
                    saveSetting,
                    forwardToContentScripts: (message) => forwardToAllContentScripts(message, debugLog),
                    checkConnection: checkConnectionAndUpdate,
                    debugLog
                }),
                createPilotMessageHandler({ isEnabled: isAiWebPilotEnabled, setEnabled: setPilotEnabled }),
                createCaptureMessageHandler({
                    getServerUrl,
                    captureScreenshot: (tabId, relatedErrorId) => captureScreenshot(tabId, getServerUrl(), relatedErrorId, canTakeScreenshot, recordScreenshot, debugLog),
                    addLog: (entry) => logBatcher.add(entry),
                    debugLog
                }),
                createUtilityMessageHandler({ getServerUrl })
            ]
        });
        // ============= STEP 8: Setup Chrome alarms =============
        setupChromeAlarms();
        installAlarmListener({
            onReconnect: checkConnectionAndUpdate,
            onErrorGroupFlush: () => {
                const aggregatedEntries = flushErrorGroups();
                if (aggregatedEntries.length > 0) {
                    aggregatedEntries.forEach((entry) => logBatcher.add(entry));
                }
            },
            onMemoryCheck: () => {
                debugLog(DebugCategory.LIFECYCLE, 'Memory check alarm fired');
            },
            onErrorGroupCleanup: () => cleanupStaleErrorGroups(debugLog)
        });
        // ============= STEP 8.5: Load server install ID for analytics =============
        await loadServerInstallId();
        // ============= STEP 9: Install tab removed listener =============
        installTabRemovedListener((tabId) => {
            clearScreenshotTimestamps(tabId);
            handleTrackedTabClosed(tabId, (msg) => console.log(msg));
        });
        // ============= STEP 9.5: Install tab updated listener =============
        installTabUpdatedListener((tabId, newUrl) => {
            handleTrackedTabUrlChange(tabId, newUrl, (msg) => console.log(msg));
        });
        // ============= STEP 9.6: Install draw mode keyboard shortcut listener =============
        installDrawModeCommandListener((msg) => console.log(`${KABOOM_LOG_PREFIX} ${msg}`));
        // ============= STEP 9.6a: Scope the side panel to the tracked tab =============
        // Without this the manifest default makes the panel available on every tab,
        // where it renders empty.
        void (async () => {
            const trackedTabId = (await getLocal('trackedTabId'));
            await syncTerminalPanelAvailability(typeof trackedTabId === 'number' ? trackedTabId : undefined);
        })();
        // ============= STEP 9.6b: Install terminal side panel shortcut =============
        // Gesture-native path to the side panel; the in-page launcher button cannot be
        // relied on alone (Chrome grants message listeners only a restricted gesture).
        installTerminalPanelCommandListener((msg) => console.log(`${KABOOM_LOG_PREFIX} ${msg}`));
        // ============= STEP 9.7: Install push keyboard shortcut listeners =============
        installPushCommandListener((msg) => console.log(`${KABOOM_LOG_PREFIX} ${msg}`));
        installChatCommandListener((msg) => console.log(`${KABOOM_LOG_PREFIX} ${msg}`));
        // ============= STEP 9.8: Install recording keyboard shortcut listener =============
        installRecordingShortcutCommandListener({
            isRecording,
            startRecording,
            stopRecording
        }, (msg) => console.log(`${KABOOM_LOG_PREFIX} ${msg}`));
        // ============= STEP 9.9: Install screen recording shortcut + context menus =============
        const screenRecHandlers = { isRecording, startRecording, stopRecording };
        const actionRecHandlers = { isRecording, startRecording, stopRecording };
        installScreenRecordingCommandListener(screenRecHandlers, (msg) => console.log(`${KABOOM_LOG_PREFIX} ${msg}`));
        installContextMenus(screenRecHandlers, actionRecHandlers, (msg) => console.log(`${KABOOM_LOG_PREFIX} ${msg}`));
        // ============= STEP 10: Set disconnected badge immediately =============
        // Badge must reflect disconnected state BEFORE the async health check.
        // Without this, a stale "connected" badge persists from a previous SW session
        // until the health check completes (could be seconds if server is slow to refuse).
        updateBadge(getConnectionStatus());
        // ============= STEP 11: Initial connection check =============
        // Await the connection check to keep the SW alive until the badge is updated.
        // Without await, Chrome may suspend the SW before the fetch completes.
        if (isAiWebPilotCacheInitialized()) {
            await checkConnectionAndUpdate();
        }
        else {
            setPilotInitCallback(checkConnectionAndUpdate);
        }
        // ============= INITIALIZATION COMPLETE =============
        markInitComplete();
        debugLog(DebugCategory.LIFECYCLE, 'Extension initialized', {
            serverUrl: getServerUrl(),
            logLevel: getCurrentLogLevel(),
            screenshotOnError: isScreenshotOnError(),
            sourceMapEnabled: isSourceMapEnabled(),
            debugMode: isDebugMode()
        });
    }
    catch (error) {
        console.error(`${KABOOM_LOG_PREFIX} Error during extension initialization:`, error);
        debugLog(DebugCategory.LIFECYCLE, 'Extension initialization failed', { error: String(error) });
    }
}
//# sourceMappingURL=init.js.map