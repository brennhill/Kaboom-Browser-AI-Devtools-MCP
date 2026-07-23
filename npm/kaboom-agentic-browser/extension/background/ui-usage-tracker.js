// ui-usage-tracker.ts — Tracks extension-UI-originated feature usage for sync payload.
// Only tracks actions triggered by the user in the extension UI (popup, context menu,
// keyboard shortcut) — NOT actions dispatched by AI/MCP tool calls.
// =============================================================================
// STATE
// =============================================================================
let pending = new Map();
// =============================================================================
// PUBLIC API
// =============================================================================
/**
 * Record that a UI feature was used. Called from context menus, popup buttons,
 * keyboard shortcuts — anywhere the user triggers an action without AI.
 */
export function trackUIFeature(feature) {
    pending.set(feature, true);
}
/**
 * Atomically drain pending features for inclusion in the next sync request.
 * Uses swap-and-replace so no events are lost between iteration and clear.
 * Returns undefined if empty.
 */
export function drainUIFeatures() {
    if (pending.size === 0)
        return undefined;
    const old = pending;
    pending = new Map();
    const result = {};
    for (const [key, val] of old) {
        result[key] = val;
    }
    return result;
}
/**
 * Re-merge features back into pending after a failed sync.
 * Preserves any new features tracked since the drain.
 */
export function restoreUIFeatures(features) {
    for (const [key, val] of Object.entries(features)) {
        if (val) {
            pending.set(key, true);
        }
    }
}
//# sourceMappingURL=ui-usage-tracker.js.map