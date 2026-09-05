/**
 * Purpose: DOM fallback for the pointer gestures — drag along a route, right/double/triple
 *          click, coordinate-addressed hover and scroll — when CDP is unavailable or the
 *          caller opted out with dispatch:"dom".
 * Why: CDP is the primary path because its events carry isTrusted:true, but it is not always
 *      reachable (no debugger permission, a tab held exclusively by a trace, an internal page)
 *      and controlled React inputs still need the #599 escape hatch. Without this module those
 *      cases would lose the gesture entirely instead of losing only event trust.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import type { DOMResult } from '../../dom-types.js';
import type { ResolvedElement } from '../../cdp/cdp-element-resolve.js';
/** Gestures this primitive owns. dom-dispatch routes exactly these here. */
export declare const DOM_GESTURE_ACTIONS: ReadonlySet<string>;
export declare function isDOMGestureAction(action: string): boolean;
/**
 * A click that carries modifiers cannot go through the plain DOM click handler.
 *
 * `element.click()` synthesizes a MouseEvent with every modifier flag false, so a ctrl+click
 * asked for through dispatch:"dom" would land as an ordinary click and navigate in place
 * instead of opening a tab. Route it here instead.
 */
export declare function needsGestureDispatch(action: string, modifiers: string[] | undefined): boolean;
export declare function gestureModifierMask(modifiers: readonly string[] | undefined): number;
export interface GesturePagePoint {
    x: number;
    y: number;
}
/** What an injected gesture still has to decide, with the modifier names already folded. */
export interface GesturePageOptions {
    modifiers: number;
    drag_path?: GesturePagePoint[];
    delta_x?: number;
    delta_y?: number;
}
/** What an injected gesture reports back. Never a DOMResult: the caller owns the evidence. */
export interface GesturePageOutcome {
    dispatched: boolean;
    error?: string;
    button?: string;
    click_count?: number;
    context_menu?: boolean;
    delta_x?: number;
    delta_y?: number;
    path_points?: number;
    move_events?: number;
    html5_drag?: boolean;
}
/**
 * Injected click gesture: right, double, triple, and modifier-held single click.
 * MUST NOT reference module-level values — Chrome serializes the function source only.
 */
export declare function domGestureClick(action: string, at: GesturePagePoint, options: GesturePageOptions): GesturePageOutcome;
/**
 * Injected hover and wheel gesture at a coordinate.
 * MUST NOT reference module-level values — Chrome serializes the function source only.
 */
export declare function domGestureScroll(action: string, at: GesturePagePoint, options: GesturePageOptions): GesturePageOutcome;
/**
 * Injected drag: press, follow the densified route, drop, release.
 * MUST NOT reference module-level values — Chrome serializes the function source only.
 */
export declare function domGestureDrag(_action: string, at: GesturePagePoint, options: GesturePageOptions): GesturePageOutcome;
/** Which injected function a gesture uses. */
export declare function gesturePrimitiveFor(action: string): typeof domGestureClick;
/**
 * Turn an injected gesture's outcome into the DOMResult the async lifecycle expects.
 *
 * Built in the service worker, not in the page, because the matched-element evidence already
 * came back from resolveElement — asking the page for it a second time would let the two
 * disagree about which element the gesture hit.
 */
export declare function buildGestureDOMResult(action: string, selector: string, point: GesturePagePoint, resolved: ResolvedElement | null, outcome: GesturePageOutcome | undefined, modifiers: number): DOMResult;
//# sourceMappingURL=dom-primitives-gestures.d.ts.map