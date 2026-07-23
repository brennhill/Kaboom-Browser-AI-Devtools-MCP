/**
 * Purpose: Renders action toast overlays showing real-time status (trying/success/error) for AI-driven browser actions.
 * Docs: docs/features/feature/interact-explore/index.md
 */
/**
 * Show a brief visual toast overlay for AI actions.
 * Supports color-coded states and structured content with truncation.
 * For audio-related toasts, adds animated arrow pointing to extension icon.
 */
export declare function showActionToast(text: string, detail?: string, state?: 'trying' | 'success' | 'warning' | 'error' | 'audio', durationMs?: number): void;
//# sourceMappingURL=toast.d.ts.map