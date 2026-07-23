/**
 * Purpose: Implements popup settings controls for websocket capture mode and safe log clearing actions.
 * Why: Keeps destructive and behavior-changing popup operations centralized with explicit UX safeguards.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */
import { SettingName, StorageKey } from '../lib/constants.js';
import { setLocal, getLocal } from '../lib/storage-utils.js';
/**
 * Handle WebSocket mode change
 */
export function handleWebSocketModeChange(mode) {
    void setLocal(StorageKey.WEBSOCKET_CAPTURE_MODE, mode);
    chrome.runtime.sendMessage({ type: SettingName.WEBSOCKET_CAPTURE_MODE, mode });
}
/**
 * Apply pre-loaded WS mode value to the selector.
 * Called from the orchestrator after a single batched storage read.
 */
export function applyWebSocketMode(value) {
    const modeSelect = document.getElementById('ws-mode');
    if (!modeSelect)
        return;
    modeSelect.value = value || 'medium';
}
/**
 * Initialize the WebSocket mode selector (self-contained async version for backward compat)
 */
export async function initWebSocketModeSelector() {
    const value = await getLocal(StorageKey.WEBSOCKET_CAPTURE_MODE);
    applyWebSocketMode(value);
}
// Track clear-logs confirmation state
let clearConfirmPending = false;
let clearConfirmTimer = null;
/**
 * Reset clear confirmation state (exported for testing)
 */
export function resetClearConfirm() {
    clearConfirmPending = false;
    if (clearConfirmTimer) {
        clearTimeout(clearConfirmTimer);
        clearConfirmTimer = null;
    }
}
/**
 * Handle clear logs button click (with confirmation)
 */
export async function handleClearLogs() {
    const clearBtn = document.getElementById('clear-btn');
    const entriesEl = document.getElementById('entries-count');
    // Two-click confirmation: first click changes to "Confirm?", second click clears
    if (clearBtn && !clearConfirmPending) {
        clearConfirmPending = true;
        clearBtn.textContent = 'Confirm Clear?';
        // Reset after 3 seconds if not confirmed
        clearConfirmTimer = setTimeout(() => {
            clearConfirmPending = false;
            if (clearBtn)
                clearBtn.textContent = 'Clear Logs';
        }, 3000);
        return Promise.resolve(null);
    }
    // Second click: actually clear
    clearConfirmPending = false;
    if (clearConfirmTimer) {
        clearTimeout(clearConfirmTimer);
        clearConfirmTimer = null;
    }
    if (clearBtn) {
        clearBtn.disabled = true;
        clearBtn.textContent = 'Clearing...';
    }
    return new Promise((resolve) => {
        chrome.runtime.sendMessage({ type: 'clear_logs' }, (response) => {
            if (clearBtn) {
                clearBtn.disabled = false;
                clearBtn.textContent = 'Clear Logs';
            }
            if (response?.success) {
                if (entriesEl) {
                    entriesEl.textContent = '0 / 1000';
                }
            }
            else if (response?.error) {
                const errorEl = document.getElementById('error-message');
                if (errorEl) {
                    errorEl.textContent = response.error;
                }
            }
            resolve(response || null);
        });
    });
}
//# sourceMappingURL=settings.js.map