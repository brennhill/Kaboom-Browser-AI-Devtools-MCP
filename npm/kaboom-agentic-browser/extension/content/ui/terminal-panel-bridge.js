/**
 * Purpose: Bridges the page hover launcher to the terminal side panel.
 * Why: Keeps the page overlay focused on quick actions while terminal visibility
 * and writes are coordinated through session state and runtime messages.
 * Docs: docs/features/feature/terminal/index.md
 */
import { StorageKey } from '../../lib/constants.js';
import { getSession, onStorageChanged } from '../../lib/storage-utils.js';
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
export async function openTerminalPanel() {
    try {
        const result = (await chrome.runtime.sendMessage({ type: 'open_terminal_panel' }));
        return result?.success === true;
    }
    catch {
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