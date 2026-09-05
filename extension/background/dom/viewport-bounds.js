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
// viewport-bounds.ts — The screen-bounds rule for every coordinate-addressed action.
//
// The extent is read with readPageViewportMetrics, the SAME injected probe that builds a
// screenshot's coordinate_frame. That is deliberate: coordinate_frame publishes the mapping an
// agent uses to turn an image pixel into an x/y, and this checks that x/y against the same
// measured viewport. Two different measurements would let a point the frame calls on-screen be
// refused here, or the reverse.
import { KABOOM_LOG_PREFIX } from '../../lib/brand.js';
import { errorMessage } from '../../lib/error-utils.js';
import { readPageViewportMetrics } from '../../lib/screenshot/coordinate-frame.js';
/**
 * Every viewport point the call names.
 *
 * Half a coordinate names no point: the daemon already refuses that case, and treating a lone x
 * as a point here would invent a y of zero and refuse a call for a coordinate nobody sent. An
 * action that names nothing costs no probe, which is why this returns an array rather than a
 * flag.
 */
export function addressedPoints(params) {
    const points = [];
    if (typeof params.x === 'number' && typeof params.y === 'number') {
        points.push({ x: params.x, y: params.y });
    }
    for (const point of Array.isArray(params.drag_path) ? params.drag_path : []) {
        if (typeof point?.x === 'number' && typeof point?.y === 'number')
            points.push({ x: point.x, y: point.y });
    }
    return points;
}
/**
 * The message naming the first point that is not on the screen, or null when every point is.
 *
 * An unmeasurable extent returns null rather than refusing: a bound nobody measured is a guess,
 * and refusing a correct click because the probe failed is worse than dispatching it. Every
 * waypoint of a drag is checked, not just its ends — a route that leaves the screen in the middle
 * drops its payload at the clamped edge and reports the route it was asked for.
 */
export function outOfViewportMessage(action, points, extent) {
    if (!extent || !(extent.width > 0) || !(extent.height > 0))
        return null;
    for (const point of points) {
        if (onScreen(point, extent))
            continue;
        return (`${action} was aimed at (${point.x}, ${point.y}), which is outside the viewport — ` +
            `the visible area is ${extent.width}x${extent.height} CSS pixels. ` +
            'Viewport coordinates are CSS pixels from the top-left of the visible area, the space a ' +
            "screenshot's coordinate_frame maps image pixels into. Scroll the target into view, or map " +
            'the pixel through coordinate_frame.image_to_viewport before sending it.');
    }
    return null;
}
function onScreen(point, extent) {
    if (!Number.isFinite(point.x) || !Number.isFinite(point.y))
        return false;
    return point.x >= 0 && point.y >= 0 && point.x <= extent.width && point.y <= extent.height;
}
/**
 * Measure the tab's visible area, or null when the probe cannot run.
 *
 * Null is a real answer: an internal page, a tab mid-navigation, or a missing scripting
 * permission all mean nobody knows the extent, and the caller dispatches rather than refusing on
 * a number it does not have.
 */
export async function readViewportExtent(tabId) {
    try {
        const probed = await chrome.scripting.executeScript({
            target: { tabId },
            world: 'MAIN',
            func: readPageViewportMetrics
        });
        const metrics = probed[0]?.result;
        if (!metrics)
            return null;
        return { width: metrics.viewport_width, height: metrics.viewport_height };
    }
    catch (err) {
        console.warn(`${KABOOM_LOG_PREFIX} viewport probe failed; the coordinate is dispatched unchecked —`, errorMessage(err));
        return null;
    }
}
/**
 * The one call both dispatch paths make: null when the action may proceed, otherwise the message
 * to fail it with. Actions that name no point never reach the probe.
 */
export async function coordinateOutOfViewport(tabId, action, params) {
    const points = addressedPoints(params);
    if (points.length === 0)
        return null;
    return outOfViewportMessage(action, points, await readViewportExtent(tabId));
}
//# sourceMappingURL=viewport-bounds.js.map