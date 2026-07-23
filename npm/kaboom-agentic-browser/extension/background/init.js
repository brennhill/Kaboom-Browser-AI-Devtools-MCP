/**
 * Purpose: Extension startup initialization -- loads settings, installs listeners, recovers state after service worker restart, and initiates first connection.
 * Docs: docs/features/feature/cold-start-queuing/index.md
 */
/**
 * @fileoverview Extension Initialization
 * Handles startup logic: loading settings, installing listeners, and initial connection setup.
 * Uses async/await for cleaner control flow (replaces callback nesting).
 */
import { beacon } from '../lib/telemetry-beacon.js';
import { getTrackedTabLostToastDetail, KABOOM_LOG_PREFIX } from '../lib/brand.js';
import { debugLog, DebugCategory, setDebugMode, resetSyncClientConnection, sharedServerCircuitBreaker, logBatcher, wsBatcher, enhancedActionBatcher, networkBodyBatcher, perfBatcher, handleLogMessage, handleClearLogs, checkConnectionAndUpdate, exportDebugLog, clearDebugLog, sendStatusPingWrapper, DEFAULT_SERVER_URL } from './index.js';
import { getServerUrl, getConnectionStatus, isDebugMode, isScreenshotOnError, getCurrentLogLevel, isAiWebPilotEnabled, isAiWebPilotCacheInitialized, getPilotInitCallback, markInitComplete, setServerUrl, setCurrentLogLevel, setScreenshotOnError, setAiWebPilotEnabledCache, setAiWebPilotCacheInitialized, setPilotInitCallback } from './state.js';
import { isSourceMapEnabled, setSourceMapEnabled, canTakeScreenshot, recordScreenshot, clearSourceMapCache, getContextWarning, getMemoryPressureState, isNetworkBodyCaptureDisabled, flushErrorGroups, cleanupStaleErrorGroups, clearScreenshotTimestamps } from './state-manager.js';
import { loadDebugModeState, installStartupListener, loadAiWebPilotState, loadSavedSettings, installStorageChangeListener, setupChromeAlarms, installAlarmListener, installTabRemovedListener, installTabUpdatedListener, installDrawModeCommandListener, installRecordingShortcutCommandListener, installScreenRecordingCommandListener, installContextMenus, saveSetting, forwardToAllContentScripts, getActiveTab, sendTabToast, handleTrackedTabClosed, handleTrackedTabUrlChange } from './event-listeners.js';
import { installPushCommandListener, installChatCommandListener } from './push-handler.js';
import { isRecording, startRecording, stopRecording } from './recording.js';
import { installMessageListener, broadcastTrackingState } from './message-handlers.js';
import { captureScreenshot, updateBadge } from './communication.js';
import { wasServiceWorkerRestarted, markStateVersion, setSessionAccessLevel, setLocal } from '../lib/storage-utils.js';
import { loadServerInstallId } from './sync-client.js';
/**
 * Initialize the extension on startup
 * Handles state recovery after service worker restart, loads settings, installs listeners.
 * Uses async/await for readable, linear control flow.
 */
export function initializeExtension() {
    if (typeof chrome === 'undefined' || !chrome.runtime) {
        return;
    }
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
        // Anonymous telemetry: service worker activation (once per session)
        beacon('extension_start');
        // ============= STEP 1: Check service worker restart =============
        const wasRestarted = await wasServiceWorkerRestarted();
        if (wasRestarted) {
            console.warn(`${KABOOM_LOG_PREFIX} Service worker restarted - ephemeral state cleared. ` +
                'User preferences restored from persistent storage.');
            debugLog(DebugCategory.LIFECYCLE, 'Service worker restarted, ephemeral state recovered');
        }
        // Mark the current state version
        await markStateVersion();
        // Allow content scripts to access chrome.storage.session (required for terminal state persistence).
        // Without this, content scripts silently fail to read/write session storage.
        await setSessionAccessLevel('TRUSTED_AND_UNTRUSTED_CONTEXTS');
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
                sendStatusPingWrapper();
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
        // #lizard forgives
        const deps = {
            getServerUrl: () => getServerUrl(),
            getConnectionStatus: () => getConnectionStatus(),
            getDebugMode: () => isDebugMode(),
            getScreenshotOnError: () => isScreenshotOnError(),
            getSourceMapEnabled: () => isSourceMapEnabled(),
            getCurrentLogLevel: () => getCurrentLogLevel(),
            getContextWarning,
            getCircuitBreakerState: () => sharedServerCircuitBreaker.getState(),
            getMemoryPressureState,
            getAiWebPilotEnabled: () => isAiWebPilotEnabled(),
            isNetworkBodyCaptureDisabled,
            setServerUrl: (url) => {
                setServerUrl(url || DEFAULT_SERVER_URL);
            },
            setCurrentLogLevel: (level) => {
                setCurrentLogLevel(level);
            },
            setScreenshotOnError: (enabled) => {
                setScreenshotOnError(enabled);
            },
            setSourceMapEnabled: (enabled) => {
                setSourceMapEnabled(enabled);
            },
            setDebugMode: (enabled) => {
                setDebugMode(enabled);
            },
            setAiWebPilotEnabled: (enabled, callback) => {
                setLocal('aiWebPilotEnabled', enabled).then(() => {
                    setAiWebPilotEnabledCache(enabled);
                    // Reset connection when enabling to allow immediate reconnection
                    if (enabled) {
                        resetSyncClientConnection();
                        console.log(`${KABOOM_LOG_PREFIX} Sync client reset due to AI Web Pilot enabled (direct)`);
                    }
                    if (callback)
                        callback();
                }).catch((err) => {
                    console.error(`${KABOOM_LOG_PREFIX} Failed to save aiWebPilotEnabled:`, err);
                    if (callback)
                        callback();
                });
            },
            addToLogBatcher: (entry) => logBatcher.add(entry),
            addToWsBatcher: (event) => wsBatcher.add(event),
            addToEnhancedActionBatcher: (action) => enhancedActionBatcher.add(action),
            addToNetworkBodyBatcher: (body) => networkBodyBatcher.add(body),
            addToPerfBatcher: (snapshot) => perfBatcher.add(snapshot),
            handleLogMessage,
            handleClearLogs,
            captureScreenshot: (tabId, relatedErrorId) => captureScreenshot(tabId, getServerUrl(), relatedErrorId, null, canTakeScreenshot, recordScreenshot, debugLog),
            checkConnectionAndUpdate,
            clearSourceMapCache,
            debugLog,
            exportDebugLog,
            clearDebugLog,
            saveSetting,
            forwardToAllContentScripts: (msg) => forwardToAllContentScripts(msg, debugLog)
        };
        installMessageListener(deps);
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