/**
 * Purpose: Installs Chrome extension event listeners for alarms, tab lifecycle, storage changes, and runtime startup.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
import { KABOOM_LOG_PREFIX } from '../lib/brand.js';
import { StorageKey } from '../lib/constants.js';
import { getLocal, setLocal, setLocals, onStorageChanged, persist } from '../lib/storage-utils.js';
import { clearTrackedTab as clearTrackedTabState } from './ui/tab-state.js';
// =============================================================================
// CONSTANTS - Rate Limiting & DoS Protection
// =============================================================================
/**
 * Reconnect interval: 5 seconds
 * DoS Protection: If MCP server is down, we check every 5s (circuit breaker
 * will back off exponentially if failures continue).
 * Ensures connection restored quickly when server comes back up.
 */
const RECONNECT_INTERVAL_MINUTES = 5 / 60; // 5 seconds in minutes
/**
 * Error group flush interval: 30 seconds
 * DoS Protection: Deduplicates identical errors within a 5-second window
 * before sending to server. Reduces network traffic and API quota usage.
 * Flushed every 30 seconds to keep errors reasonably fresh.
 */
const ERROR_GROUP_FLUSH_INTERVAL_MINUTES = 0.5; // 30 seconds
/**
 * Memory check interval: 30 seconds
 * DoS Protection: Monitors estimated buffer memory and triggers circuit breaker
 * if soft limit (20MB) or hard limit (50MB) is exceeded.
 * Prevents memory exhaustion from unbounded capture buffer growth.
 */
const MEMORY_CHECK_INTERVAL_MINUTES = 0.5; // 30 seconds
/**
 * Error group cleanup interval: 10 minutes
 * DoS Protection: Removes stale error group deduplication state that is >5min old.
 * Prevents unbounded growth of error group metadata.
 */
const ERROR_GROUP_CLEANUP_INTERVAL_MINUTES = 10;
// =============================================================================
// ALARM NAMES
// =============================================================================
const ALARM_NAMES = {
    RECONNECT: 'reconnect',
    ERROR_GROUP_FLUSH: 'errorGroupFlush',
    MEMORY_CHECK: 'memoryCheck',
    ERROR_GROUP_CLEANUP: 'errorGroupCleanup'
};
// =============================================================================
// CHROME ALARMS
// =============================================================================
/**
 * Setup Chrome alarms for periodic tasks
 *
 * RATE LIMITING & DoS PROTECTION:
 * 1. RECONNECT (5s): Maintains MCP connection with exponential backoff
 * 2. ERROR_GROUP_FLUSH (30s): Deduplicates errors, reduces server load
 * 3. MEMORY_CHECK (30s): Monitors buffer memory, prevents exhaustion
 * 4. ERROR_GROUP_CLEANUP (10min): Removes stale deduplication state
 *
 * Note: Alarms are re-created on service worker startup (not persistent)
 * If service worker restarts, alarms must be recreated by this function
 */
export function setupChromeAlarms() {
    if (typeof chrome === 'undefined' || !chrome.alarms)
        return;
    chrome.alarms.create(ALARM_NAMES.RECONNECT, { periodInMinutes: RECONNECT_INTERVAL_MINUTES });
    chrome.alarms.create(ALARM_NAMES.ERROR_GROUP_FLUSH, { periodInMinutes: ERROR_GROUP_FLUSH_INTERVAL_MINUTES });
    chrome.alarms.create(ALARM_NAMES.MEMORY_CHECK, { periodInMinutes: MEMORY_CHECK_INTERVAL_MINUTES });
    chrome.alarms.create(ALARM_NAMES.ERROR_GROUP_CLEANUP, { periodInMinutes: ERROR_GROUP_CLEANUP_INTERVAL_MINUTES });
}
/**
 * Install Chrome alarm listener.
 * Handlers may be async -- the listener awaits them to keep the SW alive
 * until the work completes (prevents badge updates from being lost).
 */
export function installAlarmListener(handlers) {
    if (typeof chrome === 'undefined' || !chrome.alarms)
        return;
    chrome.alarms.onAlarm.addListener(async (alarm) => {
        switch (alarm.name) {
            case ALARM_NAMES.RECONNECT:
                await handlers.onReconnect();
                break;
            case ALARM_NAMES.ERROR_GROUP_FLUSH:
                handlers.onErrorGroupFlush();
                break;
            case ALARM_NAMES.MEMORY_CHECK:
                handlers.onMemoryCheck();
                break;
            case ALARM_NAMES.ERROR_GROUP_CLEANUP:
                handlers.onErrorGroupCleanup();
                break;
        }
    });
}
// =============================================================================
// TAB LISTENERS
// =============================================================================
/**
 * Install tab removed listener
 */
export function installTabRemovedListener(onTabRemoved) {
    if (typeof chrome === 'undefined' || !chrome.tabs || !chrome.tabs.onRemoved)
        return;
    chrome.tabs.onRemoved.addListener((tabId) => {
        onTabRemoved(tabId);
    });
}
/**
 * Install tab updated listener to track URL changes
 */
export function installTabUpdatedListener(onTabUpdated) {
    if (typeof chrome === 'undefined' || !chrome.tabs || !chrome.tabs.onUpdated)
        return;
    chrome.tabs.onUpdated.addListener((tabId, changeInfo) => {
        // Only care about URL changes
        if (changeInfo.url) {
            onTabUpdated(tabId, changeInfo.url);
        }
    });
}
/**
 * Handle tracked tab URL change
 * Updates the stored URL and title when the tracked tab navigates
 */
export async function handleTrackedTabUrlChange(updatedTabId, newUrl, logFn) {
    const trackedTabId = (await getLocal(StorageKey.TRACKED_TAB_ID));
    if (trackedTabId === updatedTabId) {
        // Update URL immediately, then refresh title from the tab
        try {
            const tab = await chrome.tabs.get(updatedTabId);
            const updates = { [StorageKey.TRACKED_TAB_URL]: newUrl };
            if (tab?.title)
                updates[StorageKey.TRACKED_TAB_TITLE] = tab.title;
            await setLocals(updates);
            if (logFn) {
                logFn(`${KABOOM_LOG_PREFIX} Tracked tab updated: ${newUrl}`);
            }
        }
        catch {
            // Tab may have been closed -- update URL only (best-effort, logs on failure)
            persist(setLocal(StorageKey.TRACKED_TAB_URL, newUrl), 'tracked-tab-url');
        }
    }
}
/**
 * Handle tracked tab being closed
 * SECURITY: Clears ephemeral tracking state when tab closes
 * Uses session storage for ephemeral tab tracking data
 */
export async function handleTrackedTabClosed(closedTabId, logFn) {
    const trackedTabId = (await getLocal(StorageKey.TRACKED_TAB_ID));
    if (trackedTabId === closedTabId) {
        if (logFn)
            logFn(`${KABOOM_LOG_PREFIX} Tracked tab closed (id:`, closedTabId);
        await clearTrackedTabState();
    }
}
// =============================================================================
// STORAGE LISTENERS
// =============================================================================
/**
 * Install storage change listener
 */
export function installStorageChangeListener(handlers) {
    onStorageChanged((changes, areaName) => {
        if (areaName === 'local') {
            if (changes[StorageKey.AI_WEB_PILOT_ENABLED] && handlers.onAiWebPilotChanged) {
                handlers.onAiWebPilotChanged(changes[StorageKey.AI_WEB_PILOT_ENABLED].newValue === true);
            }
            if (changes[StorageKey.TRACKED_TAB_ID] && handlers.onTrackedTabChanged) {
                const newTabId = changes[StorageKey.TRACKED_TAB_ID].newValue ?? null;
                const oldTabId = changes[StorageKey.TRACKED_TAB_ID].oldValue ?? null;
                handlers.onTrackedTabChanged(newTabId, oldTabId);
            }
        }
    });
}
// =============================================================================
// RUNTIME LISTENERS
// =============================================================================
/**
 * Install browser startup listener (clears tracking state)
 */
export function installStartupListener(logFn) {
    if (typeof chrome === 'undefined' || !chrome.runtime || !chrome.runtime.onStartup)
        return;
    chrome.runtime.onStartup.addListener(async () => {
        try {
            const trackedTabId = (await getLocal(StorageKey.TRACKED_TAB_ID));
            if (trackedTabId) {
                try {
                    await chrome.tabs.get(trackedTabId);
                    if (logFn)
                        logFn(`${KABOOM_LOG_PREFIX} Browser restarted - tracked tab still exists, keeping tracking`);
                }
                catch {
                    if (logFn)
                        logFn(`${KABOOM_LOG_PREFIX} Browser restarted - tracked tab gone, clearing tracking state`);
                    await clearTrackedTabState();
                }
            }
        }
        catch {
            // Safety fallback: clear if we can't check (best-effort, logs on failure)
            persist(clearTrackedTabState(), 'tracked-tab-state-clear');
        }
    });
}
//# sourceMappingURL=event-listeners.js.map