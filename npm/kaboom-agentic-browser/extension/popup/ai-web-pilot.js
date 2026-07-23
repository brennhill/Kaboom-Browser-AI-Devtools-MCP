/**
 * Purpose: Manages popup-side AI Web Pilot toggle state and background command dispatch.
 * Why: Enforces a single state-authority path through background updates to avoid UI/storage divergence.
 * Docs: docs/features/feature/ai-web-pilot/index.md
 */
/**
 * @fileoverview AI Web Pilot Toggle Module
 * Manages the AI Web Pilot feature toggle
 */
import { StorageKey } from '../lib/constants.js';
import { getLocal } from '../lib/storage-utils.js';
/**
 * Apply pre-loaded AI Web Pilot value to the toggle and wire up change handler.
 * Called from the orchestrator after a single batched storage read.
 */
export function applyAiWebPilotToggle(value) {
    const toggle = document.getElementById('aiWebPilotEnabled');
    if (!toggle)
        return;
    toggle.checked = value !== false;
    toggle.addEventListener('change', () => {
        handleAiWebPilotToggle(toggle.checked);
    });
}
/**
 * Initialize the AI Web Pilot toggle (self-contained async version for backward compat).
 * Read the current state from local storage via the storage facade.
 */
export async function initAiWebPilotToggle() {
    const value = await getLocal(StorageKey.AI_WEB_PILOT_ENABLED);
    applyAiWebPilotToggle(value);
}
/**
 * Handle AI Web Pilot toggle change.
 *
 * CRITICAL: ONLY background.js updates the state via setAiWebPilotEnabled message.
 * Popup NEVER writes to chrome.storage directly.
 *
 * This ensures single source of truth. If popup wrote to storage directly:
 * 1. Popup updates storage
 * 2. Background cache doesn't update (no listener yet)
 * 3. Pilot command checks cache and gets wrong value
 * 4. User sees toggle "on" but commands fail saying "off"
 *
 * By routing through background, we guarantee:
 * 1. Popup sends message to background
 * 2. Background updates cache immediately
 * 3. Background writes to storage
 * 4. Pilot commands see correct cache state
 * 5. Everything is consistent
 */
export async function handleAiWebPilotToggle(enabled) {
    // ONLY communicate with background - do NOT write to storage directly
    chrome.runtime.sendMessage({ type: 'set_ai_web_pilot_enabled', enabled }, (response) => {
        if (!response || !response.success) {
            console.error('[KaBOOM!] Failed to set AI Web Pilot toggle in background');
            // Revert the UI if background didn't accept the change
            const toggle = document.getElementById('aiWebPilotEnabled');
            if (toggle) {
                toggle.checked = !enabled;
            }
        }
    });
}
//# sourceMappingURL=ai-web-pilot.js.map