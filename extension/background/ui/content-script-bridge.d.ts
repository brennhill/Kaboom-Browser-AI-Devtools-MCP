/**
 * Purpose: Owns background-to-content-script liveness, broadcast, overlay, and toast messaging.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */
export declare function pingContentScript(tabId: number, timeoutMs?: number): Promise<boolean>;
export declare function forwardToAllContentScripts(message: {
    type: string;
    [key: string]: unknown;
}, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<void>;
/**
 * Hide or restore every Kaboom overlay in a tab, so a screenshot captures the page and not
 * our own UI.
 *
 * Overlays are selected by the `data-kaboom-overlay` marker attribute, never by an id list.
 * The previous implementation hid a hardcoded ['kaboom-tracked-hover-launcher',
 * 'kaboom-draw-toolbar'] — and nothing in the codebase ever created `kaboom-draw-toolbar`
 * (the draw roots are kaboom-draw-overlay/-badge/-instruction), so every screenshot taken
 * during draw mode contained Kaboom's own overlay and the agent then read its own UI as page
 * content. A marker cannot be forgotten by a new overlay the way a list can.
 *
 * The original inline `display` is stashed and restored: the old code forced `flex` on
 * restore, which silently rewrote the layout of any overlay that was not a flex container.
 */
export declare function setKaboomOverlayVisibility(tabId: number, visible: boolean): Promise<void>;
export declare function sendTabToast(tabId: number, text: string, detail?: string, state?: 'trying' | 'success' | 'warning' | 'error' | 'audio', duration_ms?: number): void;
/**
 * Drive the supervision overlay in a tab (kaboom-05ue.3).
 *
 * Fire-and-forget: a missing content script must never fail the action the user asked for.
 * The overlay is decoration around the work, not a precondition of it.
 */
export declare function sendAgentIndicator(tabId: number, phase: 'driving' | 'idle' | 'cursor' | 'heartbeat', detail?: {
    action?: string;
    x?: number;
    y?: number;
}): void;
//# sourceMappingURL=content-script-bridge.d.ts.map