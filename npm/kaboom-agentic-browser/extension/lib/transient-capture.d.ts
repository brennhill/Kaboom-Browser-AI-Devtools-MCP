/**
 * Purpose: Captures transient UI elements (toasts, alerts, snackbars) via MutationObserver.
 * Why: AI agents miss transient UI because elements disappear before the next screenshot.
 * Docs: docs/features/feature/transient-capture/index.md
 */
interface TransientInfo {
    classification: string;
    role: string;
    text: string;
}
/**
 * Classify an element as a transient UI element, or return null if not transient.
 * Priority: ARIA > class fingerprints > computed style heuristic.
 */
export declare function classifyTransient(el: Element): TransientInfo | null;
/**
 * Classify an element and its children as transient candidates.
 * Walks one level of children to catch framework wrapper patterns
 * (e.g., React portals wrapping an inner element with role="alert").
 */
export declare function classifyCandidates(el: Element): TransientInfo | null;
/**
 * Install the transient element capture MutationObserver.
 */
export declare function installTransientCapture(): void;
/**
 * Uninstall the transient element capture MutationObserver.
 */
export declare function uninstallTransientCapture(): void;
export {};
//# sourceMappingURL=transient-capture.d.ts.map