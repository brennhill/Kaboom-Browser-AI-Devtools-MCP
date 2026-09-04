/**
 * Purpose: Hardware-level pointer gestures over CDP — drag along a path, right/double/triple
 *          click, coordinate-addressed hover and scroll, and clipped region capture.
 * Why: Kaboom could express exactly three CDP inputs (click, type, key_press). Every other
 *      gesture fell back to synthetic DOM events with isTrusted:false, which anti-bot systems
 *      and many SPA handlers ignore — a right_click never opened a menu, a drag never moved a
 *      canvas, and a double_click never produced dblclick because separate clicks do not
 *      coalesce.
 * Docs: docs/features/feature/interact-explore/index.md
 */
export interface GesturePoint {
    x: number;
    y: number;
}
/** The gesture-relevant slice of an interact action's parameters. */
export interface GestureParams {
    modifiers?: string[];
    drag_path?: GesturePoint[];
    delta_x?: number;
    delta_y?: number;
    width?: number;
    height?: number;
    scale?: number;
}
/**
 * What a gesture is allowed to do: send CDP commands over an already-held lease, and move the
 * supervision cursor. Narrower than `Lease` on purpose — a gesture must not be able to release
 * or invalidate the session it borrows, and tests can drive it without a session manager.
 */
export interface GestureContext {
    send(method: string, params: Record<string, unknown>): Promise<unknown>;
    /** Move the phantom cursor the user watches. Called BEFORE each dispatch, never after. */
    cursor(x: number, y: number): void;
}
/** Gestures that dispatch hardware pointer input. `zoom_region` is capture, not input. */
export declare const CDP_GESTURE_ACTIONS: ReadonlySet<string>;
export declare function isCDPGesture(action: string): boolean;
/**
 * Minimum mouseMoved events per drag segment.
 *
 * A press-then-jump-then-release delivers zero movement, and both HTML5 drag-and-drop and
 * every canvas/pointer drag library start their drag on the FIRST move past a threshold. Two
 * points supplied by the caller therefore have to become several dispatched moves or the drag
 * never begins.
 */
export declare const DRAG_STEPS_PER_SEGMENT = 4;
/**
 * Explicit coordinates carried by the action itself, or null when the target must be resolved
 * from a selector. Drag anchors on its path's first point.
 */
export declare function explicitGesturePoint(params: {
    x?: number;
    y?: number;
    drag_path?: GesturePoint[];
}): GesturePoint | null;
/**
 * Validate and normalize a caller-supplied drag route. Throws when it cannot be dragged along.
 *
 * The parameter is `drag_path`, not `path`: interact already spends `path` on the cookie path
 * string for set_cookie/delete_cookie, and one name cannot be both a string and an array of
 * points in the same tool schema.
 */
export declare function normalizeDragPath(path: GesturePoint[] | undefined): GesturePoint[];
/** Fill each supplied segment with intermediate points so movement is continuous. */
export declare function densifyDragPath(path: GesturePoint[], stepsPerSegment?: number): GesturePoint[];
/**
 * One press/release pair carrying the whole clickCount.
 *
 * `clickCount` is what makes a double or triple click real: Blink raises `dblclick` from a
 * mouseReleased whose clickCount is 2, and selects a paragraph at 3. Sending two or three
 * separate single clicks does NOT coalesce — the page sees N unrelated clicks.
 */
export declare function dispatchClickBurst(ctx: GestureContext, point: GesturePoint, options: {
    button: string;
    clickCount: number;
    modifiers: number;
}): Promise<void>;
/**
 * The one ordinary left click, shared by every CDP click path.
 *
 * There is exactly one implementation because there was nearly a third: `click` on the
 * selector-escalation path built its own press/release pair and never read `params.modifiers`,
 * so a ctrl+click on a link navigated in place instead of opening a tab — and reported success.
 * Returns the bitmask actually dispatched so the caller can report it as evidence.
 */
export declare function dispatchSingleClick(ctx: GestureContext, point: GesturePoint, modifierNames?: readonly string[]): Promise<number>;
/**
 * In-page expression that raises `contextmenu` at a coordinate.
 *
 * A right mousePressed alone does not reach page handlers on every platform, and web apps
 * build their own menus from the `contextmenu` event. Without this a right_click looks
 * dispatched and opens nothing.
 */
export declare function buildContextMenuExpression(point: GesturePoint, modifiers: number): string;
/**
 * Dispatch one gesture over the lease. Returns the gesture-specific evidence fields that the
 * caller merges into its DOMResult. Throws on an unknown action so a routing mistake cannot
 * be reported as a successful gesture.
 */
export declare function executeCDPGesture(ctx: GestureContext, action: string, params: GestureParams, point: GesturePoint): Promise<Record<string, unknown>>;
export interface ZoomRegionClip {
    x: number;
    y: number;
    width: number;
    height: number;
    scale: number;
}
/** Reject a clip that Chrome would silently turn into an empty or absurd capture. */
export declare function normalizeZoomClip(params: GestureParams & {
    x?: number;
    y?: number;
}): ZoomRegionClip;
export type CDPSend = (method: string, params: Record<string, unknown>) => Promise<unknown>;
/** Capture a clipped PNG of the region. Returns a data URL ready to post to the daemon. */
export declare function captureZoomRegion(send: CDPSend, clip: ZoomRegionClip): Promise<string>;
/**
 * Capture a region, persist it through the daemon, and return the terminal result to send.
 *
 * Same persistence path as observe({what:"screenshot"}): the image lands in the screenshots
 * directory and the query is answered with a path plus the image itself. The result is RETURNED
 * rather than left to the daemon's out-of-band query resolution, because a command handler that
 * sends no terminal result is overwritten by the registry's `no_result` error — which would
 * replace the saved capture with a failure every single time.
 */
export declare function deliverZoomRegion(send: CDPSend, params: GestureParams & {
    x?: number;
    y?: number;
}, target: {
    tabId: number;
    queryId: string;
}): Promise<Record<string, unknown>>;
//# sourceMappingURL=cdp-gestures.d.ts.map