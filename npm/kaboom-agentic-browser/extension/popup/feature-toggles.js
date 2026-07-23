/**
 * Purpose: Manages popup feature-toggle definitions and background-dispatched toggle updates.
 * Why: Prevents state drift by routing all toggle mutations through one consistent popup workflow.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */
import { SettingName, StorageKey } from '../lib/constants.js';
import { getLocals } from '../lib/storage-utils.js';
/**
 * Feature toggle configuration
 */
export const FEATURE_TOGGLES = [
    {
        id: 'toggle-websocket',
        storageKey: StorageKey.WEBSOCKET_CAPTURE_ENABLED,
        messageType: SettingName.WEBSOCKET_CAPTURE,
        default: true
    },
    {
        id: 'toggle-network-waterfall',
        storageKey: StorageKey.NETWORK_WATERFALL_ENABLED,
        messageType: SettingName.NETWORK_WATERFALL,
        default: true
    },
    {
        id: 'toggle-performance-marks',
        storageKey: StorageKey.PERFORMANCE_MARKS_ENABLED,
        messageType: SettingName.PERFORMANCE_MARKS,
        default: true
    },
    {
        id: 'toggle-action-replay',
        storageKey: StorageKey.ACTION_REPLAY_ENABLED,
        messageType: SettingName.ACTION_REPLAY,
        default: true
    },
    {
        id: 'toggle-screenshot',
        storageKey: StorageKey.SCREENSHOT_ON_ERROR,
        messageType: 'set_screenshot_on_error',
        default: true
    },
    {
        id: 'toggle-source-maps',
        storageKey: StorageKey.SOURCE_MAP_ENABLED,
        messageType: 'set_source_map_enabled',
        default: true
    },
    {
        id: 'toggle-network-body-capture',
        storageKey: StorageKey.NETWORK_BODY_CAPTURE_ENABLED,
        messageType: SettingName.NETWORK_BODY_CAPTURE,
        default: true
    },
    {
        id: 'toggle-action-toasts',
        storageKey: StorageKey.ACTION_TOASTS_ENABLED,
        messageType: SettingName.ACTION_TOASTS,
        default: true
    },
    {
        id: 'toggle-subtitles',
        storageKey: StorageKey.SUBTITLES_ENABLED,
        messageType: SettingName.SUBTITLES,
        default: true
    }
];
/**
 * Handle feature toggle change
 * CRITICAL ARCHITECTURE: Popup NEVER writes storage directly.
 * It ONLY sends a message to background, which is the single writer.
 * This prevents desynchronization bugs where UI state diverges from actual state.
 */
export function handleFeatureToggle(storageKey, messageType, enabled) {
    // Send message to background (DO NOT write storage directly)
    // Background will handle the write after updating its internal state
    chrome.runtime.sendMessage({ type: messageType, enabled }, (response) => {
        if (chrome.runtime.lastError) {
            console.error(`[KaBOOM!] Message error for ${messageType}:`, chrome.runtime.lastError.message); // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal feature flag name, not user-controlled
        }
        else if (response?.success) {
            console.log(`[KaBOOM!] ${messageType} acknowledged by background`);
        }
        else {
            console.warn(`[KaBOOM!] ${messageType} - no response from background`);
        }
    });
}
/**
 * Apply pre-loaded toggle values to DOM checkboxes and wire up change handlers.
 * Called from the orchestrator after a single batched storage read.
 */
export function applyFeatureToggles(result) {
    for (const toggle of FEATURE_TOGGLES) {
        const checkbox = document.getElementById(toggle.id);
        if (checkbox) {
            const savedValue = result[toggle.storageKey];
            checkbox.checked = savedValue !== undefined ? savedValue : toggle.default;
            checkbox.addEventListener('change', () => {
                handleFeatureToggle(toggle.storageKey, toggle.messageType, checkbox.checked);
            });
        }
    }
}
/**
 * Initialize all feature toggles (self-contained async version for backward compat)
 */
export async function initFeatureToggles() {
    const storageKeys = FEATURE_TOGGLES.map((t) => t.storageKey);
    const result = await getLocals(storageKeys);
    applyFeatureToggles(result);
}
//# sourceMappingURL=feature-toggles.js.map