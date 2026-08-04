import { type SyncFeaturesUsed, type SyncUIFeature } from '../../types/wire/wire-sync.js';
/**
 * Record that a UI feature was used. Called from context menus, popup buttons,
 * keyboard shortcuts — anywhere the user triggers an action without AI.
 */
export declare function trackUIFeature(feature: SyncUIFeature): void;
/**
 * Atomically drain pending features for inclusion in the next sync request.
 * Uses swap-and-replace so no events are lost between iteration and clear.
 * Returns undefined if empty.
 */
export declare function drainUIFeatures(): SyncFeaturesUsed | undefined;
/**
 * Re-merge features back into pending after a failed sync.
 * Preserves any new features tracked since the drain.
 */
export declare function restoreUIFeatures(features: SyncFeaturesUsed | Readonly<Record<string, boolean>>): void;
//# sourceMappingURL=ui-usage-tracker.d.ts.map