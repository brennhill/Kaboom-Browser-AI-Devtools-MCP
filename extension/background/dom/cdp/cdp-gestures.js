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
// cdp-gestures.ts — CDP Input.* gesture sequences dispatched over a cdp-session lease.
// This module NEVER attaches or detaches: cdp-session.ts is the single owner of chrome.debugger.
import { KABOOM_LOG_PREFIX } from '../../../lib/brand.js';
import { errorMessage } from '../../../lib/error-utils.js';
import { postDaemonJSON } from '../../../lib/daemon-http.js';
import { getServerUrl } from '../../runtime-state/settings-state.js';
import { modifierBitmask } from './cdp-key-mappings.js';
import { buildCoordinateFrame, readPageViewportMetrics } from '../../../lib/screenshot/coordinate-frame.js';
import { measureImageSize } from '../../../lib/screenshot/image-size.js';
/** Gestures that dispatch hardware pointer input. `zoom_region` is capture, not input. */
export const CDP_GESTURE_ACTIONS = new Set([
    'drag',
    'right_click',
    'double_click',
    'triple_click',
    'hover_at',
    'scroll_at'
]);
export function isCDPGesture(action) {
    return CDP_GESTURE_ACTIONS.has(action);
}
/** Chrome's `buttons` bitfield for a held button. Left=1, Right=2, Middle=4. */
const BUTTON_MASK = { left: 1, right: 2, middle: 4, none: 0 };
/**
 * Minimum mouseMoved events per drag segment.
 *
 * A press-then-jump-then-release delivers zero movement, and both HTML5 drag-and-drop and
 * every canvas/pointer drag library start their drag on the FIRST move past a threshold. Two
 * points supplied by the caller therefore have to become several dispatched moves or the drag
 * never begins.
 */
export const DRAG_STEPS_PER_SEGMENT = 4;
/**
 * Explicit coordinates carried by the action itself, or null when the target must be resolved
 * from a selector. Drag anchors on its path's first point.
 */
export function explicitGesturePoint(params) {
    if (typeof params.x === 'number' && typeof params.y === 'number') {
        return { x: params.x, y: params.y };
    }
    const first = params.drag_path?.[0];
    if (first && typeof first.x === 'number' && typeof first.y === 'number') {
        return { x: first.x, y: first.y };
    }
    return null;
}
/**
 * Validate and normalize a caller-supplied drag route. Throws when it cannot be dragged along.
 *
 * The parameter is `drag_path`, not `path`: interact already spends `path` on the cookie path
 * string for set_cookie/delete_cookie, and one name cannot be both a string and an array of
 * points in the same tool schema.
 */
export function normalizeDragPath(path) {
    if (!Array.isArray(path) || path.length < 2) {
        throw new Error('drag requires drag_path with at least 2 points: [{x,y},{x,y},...]');
    }
    return path.map((point, index) => {
        if (typeof point?.x !== 'number' || typeof point?.y !== 'number') {
            throw new Error(`drag_path point ${index} must have numeric x and y`);
        }
        return { x: point.x, y: point.y };
    });
}
/** Fill each supplied segment with intermediate points so movement is continuous. */
export function densifyDragPath(path, stepsPerSegment = DRAG_STEPS_PER_SEGMENT) {
    const dense = [path[0]];
    for (let i = 1; i < path.length; i += 1) {
        const from = path[i - 1];
        const to = path[i];
        for (let step = 1; step <= stepsPerSegment; step += 1) {
            const ratio = step / stepsPerSegment;
            dense.push({ x: from.x + (to.x - from.x) * ratio, y: from.y + (to.y - from.y) * ratio });
        }
    }
    return dense;
}
async function dispatchMouse(ctx, type, point, extra) {
    await ctx.send('Input.dispatchMouseEvent', { type, x: point.x, y: point.y, ...extra });
}
/**
 * One press/release pair carrying the whole clickCount.
 *
 * `clickCount` is what makes a double or triple click real: Blink raises `dblclick` from a
 * mouseReleased whose clickCount is 2, and selects a paragraph at 3. Sending two or three
 * separate single clicks does NOT coalesce — the page sees N unrelated clicks.
 */
export async function dispatchClickBurst(ctx, point, options) {
    const { button, clickCount, modifiers } = options;
    ctx.cursor(point.x, point.y);
    await dispatchMouse(ctx, 'mouseMoved', point, { button: 'none', buttons: 0, modifiers });
    await dispatchMouse(ctx, 'mousePressed', point, {
        button,
        buttons: BUTTON_MASK[button] ?? 0,
        clickCount,
        modifiers
    });
    await dispatchMouse(ctx, 'mouseReleased', point, { button, buttons: 0, clickCount, modifiers });
}
/**
 * The one ordinary left click, shared by every CDP click path.
 *
 * There is exactly one implementation because there was nearly a third: `click` on the
 * selector-escalation path built its own press/release pair and never read `params.modifiers`,
 * so a ctrl+click on a link navigated in place instead of opening a tab — and reported success.
 * Returns the bitmask actually dispatched so the caller can report it as evidence.
 */
export async function dispatchSingleClick(ctx, point, modifierNames) {
    const modifiers = modifierBitmask(modifierNames);
    await dispatchClickBurst(ctx, point, { button: 'left', clickCount: 1, modifiers });
    return modifiers;
}
/**
 * In-page expression that raises `contextmenu` at a coordinate.
 *
 * A right mousePressed alone does not reach page handlers on every platform, and web apps
 * build their own menus from the `contextmenu` event. Without this a right_click looks
 * dispatched and opens nothing.
 */
export function buildContextMenuExpression(point, modifiers) {
    const x = JSON.stringify(point.x);
    const y = JSON.stringify(point.y);
    return `(() => {
    const el = document.elementFromPoint(${x}, ${y});
    if (!el) return { dispatched: false };
    el.dispatchEvent(new MouseEvent('contextmenu', {
      bubbles: true, cancelable: true, composed: true, view: window,
      clientX: ${x}, clientY: ${y}, button: 2, buttons: 2,
      altKey: ${(modifiers & 1) !== 0}, ctrlKey: ${(modifiers & 2) !== 0},
      metaKey: ${(modifiers & 4) !== 0}, shiftKey: ${(modifiers & 8) !== 0}
    }));
    return { dispatched: true, tag: el.tagName.toLowerCase() };
  })()`;
}
async function gestureRightClick(ctx, point, modifiers) {
    await dispatchClickBurst(ctx, point, { button: 'right', clickCount: 1, modifiers });
    const evaluated = (await ctx.send('Runtime.evaluate', {
        expression: buildContextMenuExpression(point, modifiers),
        returnByValue: true
    }));
    return {
        button: 'right',
        click_count: 1,
        modifiers,
        context_menu: evaluated?.result?.value?.dispatched === true
    };
}
async function gestureMultiClick(ctx, point, modifiers, clickCount) {
    await dispatchClickBurst(ctx, point, { button: 'left', clickCount, modifiers });
    return { button: 'left', click_count: clickCount, modifiers };
}
async function gestureHoverAt(ctx, point, modifiers) {
    ctx.cursor(point.x, point.y);
    await dispatchMouse(ctx, 'mouseMoved', point, { button: 'none', buttons: 0, modifiers });
    return { button: 'none', modifiers };
}
async function gestureScrollAt(ctx, point, modifiers, params) {
    const deltaX = typeof params.delta_x === 'number' ? params.delta_x : 0;
    const deltaY = typeof params.delta_y === 'number' ? params.delta_y : 0;
    ctx.cursor(point.x, point.y);
    await dispatchMouse(ctx, 'mouseMoved', point, { button: 'none', buttons: 0, modifiers });
    await dispatchMouse(ctx, 'mouseWheel', point, { deltaX, deltaY, modifiers });
    return { delta_x: deltaX, delta_y: deltaY, modifiers };
}
/**
 * HTML5 drag-and-drop half of a drag.
 *
 * Reported, never swallowed: Chrome refuses `Input.dispatchDragEvent` in some configurations,
 * and by the time it does the mouse path has already run. Aborting there would leave the page
 * mid-drag; instead the failure is logged and reported as `html5_drag:false` so the caller can
 * see that only the pointer path landed.
 */
async function dispatchDragEvents(ctx, target, modifiers) {
    const data = { items: [], dragOperationsMask: 1 };
    try {
        for (const type of ['dragEnter', 'dragOver', 'drop']) {
            await ctx.send('Input.dispatchDragEvent', { type, x: target.x, y: target.y, data, modifiers });
        }
        return true;
    }
    catch (err) {
        console.warn(`${KABOOM_LOG_PREFIX} drag: HTML5 drag events were refused, pointer path only — ${errorMessage(err, 'unknown_error')}`);
        return false;
    }
}
async function gestureDrag(ctx, modifiers, params) {
    const path = normalizeDragPath(params.drag_path);
    const dense = densifyDragPath(path);
    const start = dense[0];
    const end = dense[dense.length - 1];
    ctx.cursor(start.x, start.y);
    await dispatchMouse(ctx, 'mouseMoved', start, { button: 'none', buttons: 0, modifiers });
    await dispatchMouse(ctx, 'mousePressed', start, { button: 'left', buttons: 1, clickCount: 1, modifiers });
    for (let i = 1; i < dense.length; i += 1) {
        const point = dense[i];
        ctx.cursor(point.x, point.y);
        await dispatchMouse(ctx, 'mouseMoved', point, { button: 'left', buttons: 1, modifiers });
    }
    const html5 = await dispatchDragEvents(ctx, end, modifiers);
    await dispatchMouse(ctx, 'mouseReleased', end, { button: 'left', buttons: 0, clickCount: 1, modifiers });
    return {
        button: 'left',
        modifiers,
        path_points: path.length,
        move_events: dense.length,
        html5_drag: html5
    };
}
/**
 * Dispatch one gesture over the lease. Returns the gesture-specific evidence fields that the
 * caller merges into its DOMResult. Throws on an unknown action so a routing mistake cannot
 * be reported as a successful gesture.
 */
export async function executeCDPGesture(ctx, action, params, point) {
    const modifiers = modifierBitmask(params.modifiers);
    switch (action) {
        case 'right_click':
            return gestureRightClick(ctx, point, modifiers);
        case 'double_click':
            return gestureMultiClick(ctx, point, modifiers, 2);
        case 'triple_click':
            return gestureMultiClick(ctx, point, modifiers, 3);
        case 'hover_at':
            return gestureHoverAt(ctx, point, modifiers);
        case 'scroll_at':
            return gestureScrollAt(ctx, point, modifiers, params);
        case 'drag':
            return gestureDrag(ctx, modifiers, params);
        default:
            throw new Error(`Unknown CDP gesture: ${action}`);
    }
}
/** Reject a clip that Chrome would silently turn into an empty or absurd capture. */
export function normalizeZoomClip(params) {
    const { x, y, width, height } = params;
    if (typeof x !== 'number' || typeof y !== 'number') {
        throw new Error('zoom_region requires numeric x and y');
    }
    if (typeof width !== 'number' || typeof height !== 'number' || width <= 0 || height <= 0) {
        throw new Error('zoom_region requires width and height greater than 0');
    }
    const scale = typeof params.scale === 'number' && params.scale > 0 ? Math.min(params.scale, 4) : 1;
    return { x, y, width, height, scale };
}
/** Capture a clipped PNG of the region. Returns a data URL ready to post to the daemon. */
export async function captureZoomRegion(send, clip) {
    const shot = (await send('Page.captureScreenshot', {
        format: 'png',
        captureBeyondViewport: false,
        clip
    }));
    if (!shot?.data) {
        throw new Error('zoom_region: Page.captureScreenshot returned no image data');
    }
    return `data:image/png;base64,${shot.data}`;
}
/**
 * Capture a region, persist it through the daemon, and return the terminal result to send.
 *
 * Same persistence path as observe({what:"screenshot"}): the image lands in the screenshots
 * directory and the query is answered with a path plus the image itself. The result is RETURNED
 * rather than left to the daemon's out-of-band query resolution, because a command handler that
 * sends no terminal result is overwritten by the registry's `no_result` error — which would
 * replace the saved capture with a failure every single time.
 */
export async function deliverZoomRegion(send, params, target) {
    const requested = normalizeZoomClip(params);
    const metrics = await readZoomViewportMetrics(target.tabId);
    const clip = pageClipFor(requested, metrics);
    const dataUrl = await captureZoomRegion(send, clip);
    const tab = await chrome.tabs.get(target.tabId);
    const response = await postDaemonJSON(`${getServerUrl()}/screenshots`, {
        data_url: dataUrl,
        url: tab.url,
        query_id: target.queryId
    });
    if (!response.ok) {
        throw new Error(`zoom_region: the daemon rejected the capture (HTTP ${response.status})`);
    }
    const saved = (await response.json());
    return {
        success: true,
        action: 'zoom_region',
        method: 'cdp',
        clip: requested,
        page_clip: clip,
        filename: saved?.filename,
        path: saved?.path,
        data_url: dataUrl,
        ...(await zoomRegionFrame(dataUrl, requested, metrics))
    };
}
/**
 * Translate the caller's VIEWPORT rectangle into the PAGE rectangle CDP clips by.
 *
 * `Page.captureScreenshot`'s clip is in document coordinates — this file's own
 * viewport capture proves it, passing `cssVisualViewport.pageX/pageY` as the clip
 * origin to photograph the visible area. zoom_region documents its x and y as
 * viewport pixels, which is the only sensible contract for a region read off a
 * screenshot, so the scroll offset has to be added here. Without it, zooming
 * (320,180) on a page scrolled 900px down captures a rectangle 900px above the one
 * the caller pointed at, and every zoom on a scrolled page inspects the wrong
 * content while looking entirely successful.
 */
export function pageClipFor(clip, metrics) {
    if (!metrics)
        return clip;
    return { ...clip, x: clip.x + metrics.scroll_x, y: clip.y + metrics.scroll_y };
}
/**
 * Read the page's metrics, or null when the probe cannot run.
 *
 * Null is load-bearing twice over: the clip falls back to the caller's coordinates
 * untranslated (the un-scrolled case, where the two spaces coincide) and the result
 * carries no coordinate frame. Both are reported rather than guessed.
 */
async function readZoomViewportMetrics(tabId) {
    try {
        const res = await chrome.scripting.executeScript({
            target: { tabId },
            world: 'MAIN',
            func: readPageViewportMetrics
        });
        return res[0]?.result ?? null;
    }
    catch (err) {
        console.warn(`${KABOOM_LOG_PREFIX} zoom_region: viewport probe failed, capturing untranslated —`, errorMessage(err));
        return null;
    }
}
/**
 * The coordinate frame for a zoomed region, so a detail read off the enlarged image
 * can be clicked without the caller undoing the supersampling by hand.
 */
async function zoomRegionFrame(dataUrl, requested, metrics) {
    if (!metrics)
        return { coordinate_frame_error: 'viewport_metrics_unavailable' };
    const measured = await measureImageSize(dataUrl);
    if ('reason' in measured)
        return { coordinate_frame_error: measured.reason };
    const frame = buildCoordinateFrame('region', metrics, measured.size, {
        x: requested.x,
        y: requested.y,
        width: requested.width,
        height: requested.height
    });
    return frame ? { coordinate_frame: frame } : { coordinate_frame_error: 'unmeasurable_region' };
}
//# sourceMappingURL=cdp-gestures.js.map