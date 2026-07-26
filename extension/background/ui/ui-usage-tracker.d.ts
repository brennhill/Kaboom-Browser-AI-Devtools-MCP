/**
 * Features that can be triggered from the extension UI.
 * WIRE-SYNCED: this union is the allowlist's source of truth on the extension side.
 * Its Go counterpart is `allowedFeatureKeys` in internal/capture/sync.go — add a
 * key to BOTH or the daemon silently drops it (CLAUDE.md rule 12).
 */
export type UIFeature = 'screenshot' | 'annotations' | 'video' | 'dom_action' | 'action_recording';
/**
 * Record that a UI feature was used. Called from context menus, popup buttons,
 * keyboard shortcuts — anywhere the user triggers an action without AI.
 */
export declare function trackUIFeature(feature: UIFeature): void;
/**
 * Atomically drain pending features for inclusion in the next sync request.
 * Uses swap-and-replace so no events are lost between iteration and clear.
 * Returns undefined if empty.
 */
export declare function drainUIFeatures(): Record<string, boolean> | undefined;
/**
 * Re-merge features back into pending after a failed sync.
 * Preserves any new features tracked since the drain.
 */
export declare function restoreUIFeatures(features: Record<string, boolean>): void;
//# sourceMappingURL=ui-usage-tracker.d.ts.map