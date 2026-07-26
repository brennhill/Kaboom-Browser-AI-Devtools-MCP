/**
 * Purpose: Shared draw-mode toggle handshake used by keyboard shortcuts and context-menu actions.
 * Why: Keep draw-mode control behavior consistent across all user entry points.
 * Docs: docs/features/feature/interact-explore/index.md
 */
/**
 * Toggle draw mode for a tab using the current content-script state when available.
 * If state lookup fails, it falls back to attempting a start command.
 */
export declare function toggleDrawModeForTab(tabId: number): Promise<void>;
//# sourceMappingURL=draw-mode-toggle.d.ts.map