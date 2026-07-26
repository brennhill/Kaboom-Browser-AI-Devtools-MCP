// ui-usage-tracker.ts — Tracks extension-UI-originated feature usage for sync payload.
// Only tracks actions triggered by the user in the extension UI (popup, context menu,
// keyboard shortcut) — NOT actions dispatched by AI/MCP tool calls.

// =============================================================================
// TYPES
// =============================================================================

/**
 * Features that can be triggered from the extension UI.
 * WIRE-SYNCED: this union is the allowlist's source of truth on the extension side.
 * Its Go counterpart is `allowedFeatureKeys` in internal/capture/sync.go — add a
 * key to BOTH or the daemon silently drops it (CLAUDE.md rule 12).
 */
export type UIFeature = 'screenshot' | 'annotations' | 'video' | 'dom_action' | 'action_recording'

// =============================================================================
// STATE
// =============================================================================

let pending: Map<UIFeature, boolean> = new Map()

// =============================================================================
// PUBLIC API
// =============================================================================

/**
 * Record that a UI feature was used. Called from context menus, popup buttons,
 * keyboard shortcuts — anywhere the user triggers an action without AI.
 */
export function trackUIFeature(feature: UIFeature): void {
  pending.set(feature, true)
}

/**
 * Atomically drain pending features for inclusion in the next sync request.
 * Uses swap-and-replace so no events are lost between iteration and clear.
 * Returns undefined if empty.
 */
export function drainUIFeatures(): Record<string, boolean> | undefined {
  if (pending.size === 0) return undefined
  const old = pending
  pending = new Map()
  const result: Record<string, boolean> = {}
  for (const [key, val] of old) {
    result[key] = val
  }
  return result
}

/**
 * Re-merge features back into pending after a failed sync.
 * Preserves any new features tracked since the drain.
 */
export function restoreUIFeatures(features: Record<string, boolean>): void {
  for (const [key, val] of Object.entries(features)) {
    if (val) {
      pending.set(key as UIFeature, true)
    }
  }
}
