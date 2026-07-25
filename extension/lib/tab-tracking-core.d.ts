/**
 * Purpose: Context-independent core for starting/stopping tab tracking, shared by
 *          the popup and the context menu (repo rule 19).
 * Why: The context-menu "Control Tab" path used to persist tracking directly,
 *      bypassing the internal-page and cloaked-domain guards the popup enforced —
 *      tracking a cloaked domain is a privacy leak (rule 7). This is the single
 *      gate both entry points go through, so the guards can never diverge again.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
/** Result of an attempt to start tracking. Only 'tracked' persisted anything. */
export type TrackTabOutcome = 'tracked' | 'internal_page' | 'cloaked';
/**
 * Start tracking `tab`, enforcing the internal-page and cloaked-domain guards.
 * Returns the outcome so UI callers can render the right state; any outcome other
 * than 'tracked' means nothing was persisted.
 */
export declare function trackTab(tab: Pick<chrome.tabs.Tab, 'id' | 'url' | 'title'>): Promise<TrackTabOutcome>;
/**
 * Stop tracking. `onStopped` lets each context stop screen recording its own way
 * — the popup messages the background, the background stops the handler directly,
 * because a runtime message does not self-deliver inside the service worker.
 */
export declare function untrackTab(prevTabId: number | undefined, onStopped?: () => void | Promise<void>): Promise<void>;
//# sourceMappingURL=tab-tracking-core.d.ts.map