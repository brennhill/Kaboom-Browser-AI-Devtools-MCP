/**
 * Purpose: Bridges the page hover launcher to the terminal side panel.
 * Why: Keeps the page overlay focused on quick actions while terminal visibility
 * and writes are coordinated through session state and runtime messages.
 * Docs: docs/features/feature/terminal/index.md
 */
import { StorageKey, TERMINAL_PANEL_SHORTCUT } from '../../lib/constants.js';
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
    console.error(`[KaBOOM!] Terminal side panel did not open: ${reason}`);
    try {
        // Chrome grants message listeners only a restricted user gesture, which
        // sidePanel.open() rejects on some builds (crbug 355266358). Both fallbacks
        // are gesture-native, so point at them rather than leaving a dead end.
        showActionToast('Terminal side panel did not open', `${reason} — press ${TERMINAL_PANEL_SHORTCUT} or right-click the page and choose "Open Kaboom Terminal".`, 'error', 8000);
    }
    catch {
        // Toast is best-effort; the console error above is the durable signal.
    }
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