// ui-usage-tracker.ts — Tracks extension-UI-originated feature usage for sync payload.
// Only tracks actions triggered by the user in the extension UI (popup, context menu,
// keyboard shortcut) — NOT actions dispatched by AI/MCP tool calls.
import { SYNC_UI_FEATURES } from '../../types/wire/wire-sync.js';
import { reportStateRecovery, resolveStateRecovery } from '../runtime-state/state-recovery.js';
// =============================================================================
// TYPES
// =============================================================================
const validFeatures = new Set(SYNC_UI_FEATURES);
let pending = new Map();
let restoreSchemaIncidentActive = false;
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
    let rejectedCount = 0;
    for (const [key, val] of Object.entries(features)) {
        if (!val)
            continue;
        if (!validFeatures.has(key)) {
            rejectedCount++;
            continue;
        }
        pending.set(key, true);
    }
    if (rejectedCount > 0) {
        restoreSchemaIncidentActive = true;
        reportStateRecovery({
            name: 'ui_feature_restore_schema',
            detail: `A failed sync rejected ${rejectedCount} feature usage entr${rejectedCount === 1 ? 'y' : 'ies'} outside the canonical bounded schema.`,
            fix: 'Reload the extension and report this diagnostic if it recurs.',
            correlation_id: 'ui_feature_restore_schema',
            expected_next_transition: 'valid_feature_restore',
            recovery_attempt: 1,
            recovery_outcome: 'fallback'
        });
        return;
    }
    if (restoreSchemaIncidentActive) {
        restoreSchemaIncidentActive = false;
        resolveStateRecovery('ui_feature_restore_schema');
    }
}
//# sourceMappingURL=ui-usage-tracker.js.map