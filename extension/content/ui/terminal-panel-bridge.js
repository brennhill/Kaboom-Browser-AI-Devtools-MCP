/**
 * Purpose: Bridges the page hover launcher to the terminal side panel.
 * Why: Keeps the page overlay focused on quick actions while terminal visibility
 * and writes are coordinated through session state and runtime messages.
 * Docs: docs/features/feature/terminal/index.md
 */
import { StorageKey, TERMINAL_PANEL_FALLBACK_HINT, TERMINAL_PANEL_STALE_CONTEXT_HINT } from '../../lib/constants.js';
import { getSession, onStorageChanged } from '../../lib/storage-utils.js';
import { showActionToast } from './toast.js';
let panelVisible = false;
let bridgeInitialized = false;
let storageListenerInstalled = false;
const visibilityListeners = new Set();
function notifyVisibilityListeners(visible) {
    for (const listener of visibilityListeners) {
        listener(visible);
    }
}
function setPanelVisible(nextVisible) {
    if (panelVisible === nextVisible)
        return;
    panelVisible = nextVisible;
    notifyVisibilityListeners(panelVisible);
}
async function syncPanelVisibilityFromStorage() {
    try {
        const value = await getSession(StorageKey.TERMINAL_UI_STATE);
        const uiState = value;
        setPanelVisible(uiState === 'open');
    }
    catch {
        // Extension context invalidated - keep the last known visibility.
    }
}
function installStorageListener() {
    if (storageListenerInstalled)
        return;
    storageListenerInstalled = true;
    onStorageChanged((changes, areaName) => {
        if (areaName !== 'session')
            return;
        const change = changes[StorageKey.TERMINAL_UI_STATE];
        if (!change)
            return;
        const nextValue = change.newValue;
        setPanelVisible(nextValue === 'open');
    });
}
export async function initTerminalPanelBridge() {
    if (bridgeInitialized)
        return;
    bridgeInitialized = true;
    installStorageListener();
    await syncPanelVisibilityFromStorage();
}
export function isTerminalVisible() {
    return panelVisible;
}
export function onTerminalPanelVisibilityChanged(listener) {
    visibilityListeners.add(listener);
    return () => {
        visibilityListeners.delete(listener);
    };
}
/**
 * Report why the side panel refused to open.
 *
 * This used to be a bare `catch { return false }`, so a rejected
 * `chrome.sidePanel.open()` left no trace anywhere — the Terminal button looked
 * simply dead. console.error is deliberate: the daemon captures page errors, so
 * the Chrome message becomes retrievable via observe(what:"errors").
 */
function reportPanelOpenFailure(reason) {
    // Chrome grants message listeners only a restricted user gesture, which
    // sidePanel.open() rejects on some builds (crbug 355266358). Both fallbacks
    // are gesture-native, so point at them rather than leaving a dead end — unless
    // the page itself is the problem, in which case only a reload fixes it.
    const stale = isStaleContextError(reason);
    const hint = stale ? TERMINAL_PANEL_STALE_CONTEXT_HINT : TERMINAL_PANEL_FALLBACK_HINT;
    // The raw Chrome error always reaches the console — it is the only diagnostic
    // signal, and the daemon captures it. The toast drops it when it would only
    // add noise to advice the user can act on directly.
    console.error(`[KaBOOM!] Terminal side panel did not open: ${reason} ${hint}`);
    try {
        showActionToast('Terminal side panel did not open', stale ? hint : `${reason} ${hint}`, 'error', 8000);
    }
    catch {
        // Toast is best-effort; the console error above is the durable signal.
    }
}
/**
 * Whether this page has been cut off from the extension for good.
 *
 * Reloading the extension orphans the content script in every tab that was
 * already open; from then on every runtime call throws this. It is not a
 * terminal fault and no terminal advice applies.
 */
function isStaleContextError(reason) {
    return reason.includes('Extension context invalidated');
}
export async function openTerminalPanel() {
    try {
        const result = (await chrome.runtime.sendMessage({ type: 'open_terminal_panel' }));
        if (result?.success === true)
            return true;
        reportPanelOpenFailure(result?.error ?? 'the background service worker sent no response');
        return false;
    }
    catch (err) {
        reportPanelOpenFailure(err instanceof Error ? err.message : String(err));
        return false;
    }
}
export function writeToTerminal(text) {
    if (!panelVisible)
        return;
    try {
        chrome.runtime.sendMessage({ type: 'terminal_panel_write', text });
    }
    catch {
        // Extension context invalidated - writes are dropped.
    }
}
export const _terminalPanelBridgeForTests = {
    reset() {
        panelVisible = false;
        bridgeInitialized = false;
        storageListenerInstalled = false;
        visibilityListeners.clear();
    }
};
//# sourceMappingURL=terminal-panel-bridge.js.map