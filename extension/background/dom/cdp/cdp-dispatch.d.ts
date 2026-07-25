/**
 * Purpose: Dispatches hardware-level input via Chrome DevTools Protocol.
 * Why: Synthetic DOM events have isTrusted:false which anti-bot systems and complex SPAs ignore.
 *      CDP Input.dispatch* commands produce true hardware events indistinguishable from real user input.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import type { PendingQuery } from '../../../types/queries.js';
import type { SyncClient } from '../../sync-client.js';
import type { DOMActionParams, DOMResult } from '../dom-types.js';
import type { SendAsyncResultFn, ActionToastFn } from '../../commands/helpers.js';
/** Check whether an action should attempt CDP before DOM primitives. */
export declare function isCDPEscalatable(action: string): boolean;
/**
 * Decide whether an action should attempt CDP hardware events before falling
 * back to DOM primitives. Callers should route through this single predicate.
 *
 * `dispatch: "dom"` is the #599 escape hatch: it forces the DOM-primitives path
 * (native-setter value + real element.click()), which drives React/Vue/Svelte
 * controlled inputs and delegated onClick handlers reliably, at the cost of
 * CDP's trusted (isTrusted:true) events. Frame/nth-scoped actions never use CDP
 * because CDP input targets the main frame by coordinate only.
 */
export declare function shouldEscalateToCDP(action: string, params: DOMActionParams): boolean;
/**
 * Build an in-page JS expression that reconciles a controlled-input framework's
 * value tracker after a CDP type (#599).
 *
 * CDP `Input.dispatchKeyEvent` updates the element's DOM value, but React tracks
 * controlled-input values with a private `_valueTracker` and only fires its
 * synthetic `onChange` when a native `input` event reports a value that differs
 * from the tracked one. Re-applying the current value through the *prototype*
 * `value` setter (which bypasses React's instance-level override) and dispatching
 * a bubbling `input`/`change` makes React observe the change and fire onChange.
 *
 * The reconciliation is gated on a detected React tracker/fiber so it is a no-op
 * on plain inputs (no spurious double input/change), and idempotent on React
 * inputs whose onChange already fired (the tracker is current → no second fire).
 */
export declare function buildReactValueReconcileExpression(selector: string): string;
/**
 * Attempt CDP-first execution for click/type/key_press.
 * Returns a DOMResult on success, or null to signal fallback to DOM primitives.
 * Any error is caught internally — callers just check for null.
 */
export declare function tryCDPEscalation(tabId: number, action: string, params: DOMActionParams): Promise<DOMResult | null>;
export declare function executeCDPAction(query: PendingQuery, tabId: number, syncClient: SyncClient, sendAsyncResult: SendAsyncResultFn, actionToast: ActionToastFn): Promise<void>;
//# sourceMappingURL=cdp-dispatch.d.ts.map