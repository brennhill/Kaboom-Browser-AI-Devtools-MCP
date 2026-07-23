/**
 * Purpose: Single source of truth for interact action classification (read-only, mutating, requires pilot).
 * Docs: docs/features/feature/interact-explore/index.md
 *
 * SYNC NOTE: The Go side maintains a parallel readOnlyInteractActions map in
 * cmd/browser-agent/tools_interact_dispatch.go for jitter gating. When adding or
 * reclassifying actions here, update the Go map to match.
 */
export interface ActionMeta {
    /** True if the action only reads page state and never modifies the DOM. */
    readonly: boolean;
    /** True if the action modifies the DOM and requires match-evidence validation. */
    mutating: boolean;
    /** True if the action requires the AI Web Pilot extension to be connected. */
    requiresPilot?: boolean;
}
/** Returns true if the action only reads page state (no DOM mutation, no side effects worth toasting). */
export declare function isReadOnlyAction(action: string): boolean;
/** Returns true if the action modifies the DOM and requires match-evidence validation. */
export declare function isMutatingAction(action: string): boolean;
//# sourceMappingURL=action-metadata.d.ts.map