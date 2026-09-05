/**
 * Purpose: Refuse a coordinate that is not on the screen, instead of dispatching it and letting
 *          Chrome decide where it lands.
 * Why: `Input.dispatchMouseEvent` does not reject an out-of-range point. It clamps to the nearest
 *      edge and answers success, so a click at (1900, 400) on a 1280-wide viewport lands on the
 *      right border and is reported as a click at (1900, 400). The agent then reasons about a
 *      page state that never happened. The daemon cannot make this call — only the page knows how
 *      big its viewport is — so the far edges are held here.
 * Docs: docs/features/feature/interact-explore/index.md
 */
/** The visible area in CSS pixels. */
export interface ViewportExtent {
    readonly width: number;
    readonly height: number;
}
/** One viewport coordinate an action was aimed at. */
export interface AddressedPoint {
    readonly x: number;
    readonly y: number;
}
/** The parameter slice a bounds check reads. Everything else about the action is irrelevant. */
export interface CoordinateParams {
    x?: number;
    y?: number;
    drag_path?: AddressedPoint[];
}
/**
 * Every viewport point the call names.
 *
 * Half a coordinate names no point: the daemon already refuses that case, and treating a lone x
 * as a point here would invent a y of zero and refuse a call for a coordinate nobody sent. An
 * action that names nothing costs no probe, which is why this returns an array rather than a
 * flag.
 */
export declare function addressedPoints(params: CoordinateParams): AddressedPoint[];
/**
 * The message naming the first point that is not on the screen, or null when every point is.
 *
 * An unmeasurable extent returns null rather than refusing: a bound nobody measured is a guess,
 * and refusing a correct click because the probe failed is worse than dispatching it. Every
 * waypoint of a drag is checked, not just its ends — a route that leaves the screen in the middle
 * drops its payload at the clamped edge and reports the route it was asked for.
 */
export declare function outOfViewportMessage(action: string, points: readonly AddressedPoint[], extent: ViewportExtent | null): string | null;
/**
 * Measure the tab's visible area, or null when the probe cannot run.
 *
 * Null is a real answer: an internal page, a tab mid-navigation, or a missing scripting
 * permission all mean nobody knows the extent, and the caller dispatches rather than refusing on
 * a number it does not have.
 */
export declare function readViewportExtent(tabId: number): Promise<ViewportExtent | null>;
/**
 * The one call both dispatch paths make: null when the action may proceed, otherwise the message
 * to fail it with. Actions that name no point never reach the probe.
 */
export declare function coordinateOutOfViewport(tabId: number, action: string, params: CoordinateParams): Promise<string | null>;
//# sourceMappingURL=viewport-bounds.d.ts.map