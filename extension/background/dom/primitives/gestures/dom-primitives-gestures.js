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
/** Modifier bits, matching the CDP mask so both dispatch paths report the same number. */
const GESTURE_MODIFIER_BITS = {
    alt: 1,
    ctrl: 2,
    control: 2,
    meta: 4,
    cmd: 4,
    command: 4,
    shift: 8
};
/** Gestures this primitive owns. dom-dispatch routes exactly these here. */
export const DOM_GESTURE_ACTIONS = new Set([
    'drag',
    'right_click',
    'double_click',
    'triple_click',
    'hover_at',
    'scroll_at'
]);
export function isDOMGestureAction(action) {
    return DOM_GESTURE_ACTIONS.has(action);
}
/**
 * A click that carries modifiers cannot go through the plain DOM click handler.
 *
 * `element.click()` synthesizes a MouseEvent with every modifier flag false, so a ctrl+click
 * asked for through dispatch:"dom" would land as an ordinary click and navigate in place
 * instead of opening a tab. Route it here instead.
 */
export function needsGestureDispatch(action, modifiers) {
    if (isDOMGestureAction(action))
        return true;
    return action === 'click' && Array.isArray(modifiers) && modifiers.length > 0;
}
export function gestureModifierMask(modifiers) {
    if (!modifiers || modifiers.length === 0)
        return 0;
    let mask = 0;
    for (const name of modifiers) {
        mask |= GESTURE_MODIFIER_BITS[String(name).trim().toLowerCase()] ?? 0;
    }
    return mask;
}
// jscpd:ignore-start — the MouseEventInit block below repeats in each injected function on
// purpose. chrome.scripting.executeScript({func}) serializes ONLY the function source, so a
// hoisted helper would be undefined in page scope and every gesture would throw at dispatch.
/**
 * Injected click gesture: right, double, triple, and modifier-held single click.
 * MUST NOT reference module-level values — Chrome serializes the function source only.
 */
export function domGestureClick(action, at, options) {
    const node = document.elementFromPoint(at.x, at.y);
    if (!node)
        return { dispatched: false, error: 'no_element_at_point' };
    const mask = options.modifiers;
    function fire(type, extra) {
        node.dispatchEvent(new MouseEvent(type, {
            bubbles: true,
            cancelable: true,
            composed: true,
            view: window,
            clientX: at.x,
            clientY: at.y,
            altKey: (mask & 1) !== 0,
            ctrlKey: (mask & 2) !== 0,
            metaKey: (mask & 4) !== 0,
            shiftKey: (mask & 8) !== 0,
            ...extra
        }));
    }
    /** One press/release/click cycle. `detail` is the click count the page reads. */
    function cycle(button, detail) {
        fire('mousedown', { button, buttons: button === 2 ? 2 : 1, detail });
        fire('mouseup', { button, buttons: 0, detail });
        fire('click', { button, buttons: 0, detail });
    }
    if (action === 'right_click') {
        cycle(2, 1);
        // Web apps build their own menus from `contextmenu`; without it a right click opens nothing.
        fire('contextmenu', { button: 2, buttons: 2, detail: 0 });
        return { dispatched: true, button: 'right', click_count: 1, context_menu: true };
    }
    // Inline, not a module constant: the injected source carries no module scope.
    const total = { click: 1, double_click: 2, triple_click: 3 }[action];
    if (!total)
        return { dispatched: false, error: 'unknown_action' };
    for (let detail = 1; detail <= total; detail += 1)
        cycle(0, detail);
    if (total === 2)
        fire('dblclick', { button: 0, buttons: 0, detail: 2 });
    return { dispatched: true, button: 'left', click_count: total };
}
/**
 * Injected hover and wheel gesture at a coordinate.
 * MUST NOT reference module-level values — Chrome serializes the function source only.
 */
export function domGestureScroll(action, at, options) {
    const node = document.elementFromPoint(at.x, at.y);
    if (!node)
        return { dispatched: false, error: 'no_element_at_point' };
    const mask = options.modifiers;
    function base(extra) {
        return {
            bubbles: true,
            cancelable: true,
            composed: true,
            view: window,
            clientX: at.x,
            clientY: at.y,
            altKey: (mask & 1) !== 0,
            ctrlKey: (mask & 2) !== 0,
            metaKey: (mask & 4) !== 0,
            shiftKey: (mask & 8) !== 0,
            ...extra
        };
    }
    /** The pane the wheel would actually move. */
    function scrollable() {
        let box = node;
        while (box && box !== document.documentElement) {
            const style = typeof getComputedStyle === 'function' ? getComputedStyle(box) : null;
            const overflow = `${style?.overflow || ''} ${style?.overflowY || ''} ${style?.overflowX || ''}`;
            const scrolls = box.scrollHeight > box.clientHeight || box.scrollWidth > box.clientWidth;
            if (scrolls && /auto|scroll/.test(overflow))
                return box;
            box = box.parentElement;
        }
        return document.scrollingElement || document.documentElement;
    }
    if (action === 'hover_at') {
        node.dispatchEvent(new MouseEvent('mouseover', base({ button: 0, buttons: 0 })));
        node.dispatchEvent(new MouseEvent('mouseenter', base({ button: 0, buttons: 0, bubbles: false })));
        node.dispatchEvent(new MouseEvent('mousemove', base({ button: 0, buttons: 0 })));
        return { dispatched: true, button: 'none' };
    }
    if (action !== 'scroll_at')
        return { dispatched: false, error: 'unknown_action' };
    const deltaX = typeof options.delta_x === 'number' ? options.delta_x : 0;
    const deltaY = typeof options.delta_y === 'number' ? options.delta_y : 0;
    node.dispatchEvent(new WheelEvent('wheel', { ...base({ button: 0, buttons: 0 }), deltaX, deltaY, deltaMode: 0 }));
    // An untrusted wheel event never scrolls natively, so the pane is also moved directly.
    // Without this scroll_at would report success and nothing on the page would move.
    const container = scrollable();
    container.scrollLeft += deltaX;
    container.scrollTop += deltaY;
    return { dispatched: true, delta_x: deltaX, delta_y: deltaY };
}
/**
 * Injected drag: press, follow the densified route, drop, release.
 * MUST NOT reference module-level values — Chrome serializes the function source only.
 */
export function domGestureDrag(_action, at, options) {
    const route = Array.isArray(options.drag_path) ? options.drag_path : [];
    if (route.length < 2)
        return { dispatched: false, error: 'invalid_drag_path' };
    const mask = options.modifiers;
    /** A press-then-jump-then-release moves nothing: drag libraries start on the first move. */
    function densify() {
        const dense = [route[0]];
        for (let i = 1; i < route.length; i += 1) {
            const from = route[i - 1];
            const to = route[i];
            for (let step = 1; step <= 4; step += 1) {
                dense.push({ x: from.x + ((to.x - from.x) * step) / 4, y: from.y + ((to.y - from.y) * step) / 4 });
            }
        }
        return dense;
    }
    function base(p, extra) {
        return {
            bubbles: true,
            cancelable: true,
            composed: true,
            view: window,
            clientX: p.x,
            clientY: p.y,
            altKey: (mask & 1) !== 0,
            ctrlKey: (mask & 2) !== 0,
            metaKey: (mask & 4) !== 0,
            shiftKey: (mask & 8) !== 0,
            ...extra
        };
    }
    const data = new DataTransfer();
    function drag(node, type, p) {
        node.dispatchEvent(new DragEvent(type, { ...base(p, { button: 0, buttons: 1 }), dataTransfer: data }));
    }
    const dense = densify();
    const start = dense[0];
    const end = dense[dense.length - 1];
    const source = document.elementFromPoint(start.x, start.y) || document.elementFromPoint(at.x, at.y);
    if (!source)
        return { dispatched: false, error: 'no_element_at_point' };
    source.dispatchEvent(new MouseEvent('mousedown', base(start, { button: 0, buttons: 1 })));
    drag(source, 'dragstart', start);
    let over = source;
    for (let i = 1; i < dense.length; i += 1) {
        const point = dense[i];
        const under = document.elementFromPoint(point.x, point.y) || source;
        under.dispatchEvent(new MouseEvent('mousemove', base(point, { button: 0, buttons: 1 })));
        if (under !== over) {
            drag(over, 'dragleave', point);
            drag(under, 'dragenter', point);
            over = under;
        }
        drag(under, 'dragover', point);
    }
    const target = document.elementFromPoint(end.x, end.y) || source;
    drag(target, 'drop', end);
    drag(source, 'dragend', end);
    target.dispatchEvent(new MouseEvent('mouseup', base(end, { button: 0, buttons: 0 })));
    return { dispatched: true, button: 'left', path_points: route.length, move_events: dense.length - 1, html5_drag: true };
}
// jscpd:ignore-end
/** Which injected function a gesture uses. */
export function gesturePrimitiveFor(action) {
    if (action === 'drag')
        return domGestureDrag;
    if (action === 'hover_at' || action === 'scroll_at')
        return domGestureScroll;
    return domGestureClick;
}
/**
 * Turn an injected gesture's outcome into the DOMResult the async lifecycle expects.
 *
 * Built in the service worker, not in the page, because the matched-element evidence already
 * came back from resolveElement — asking the page for it a second time would let the two
 * disagree about which element the gesture hit.
 */
export function buildGestureDOMResult(action, selector, point, resolved, outcome, modifiers) {
    if (!outcome || !outcome.dispatched) {
        return {
            success: false,
            action,
            selector,
            error: outcome?.error || 'gesture_not_dispatched',
            message: `${action} found nothing to act on at (${point.x}, ${point.y})`
        };
    }
    const { dispatched: _dispatched, error: _error, ...evidence } = outcome;
    return {
        success: true,
        action,
        selector,
        x: point.x,
        y: point.y,
        insertion_strategy: 'dom',
        modifiers,
        ...(resolved ? { matched: gestureMatchedEvidence(resolved) } : {}),
        ...evidence
    };
}
function gestureMatchedEvidence(resolved) {
    return {
        tag: resolved.tag,
        text_preview: resolved.text_preview,
        selector: resolved.selector,
        element_id: resolved.element_id,
        aria_label: resolved.aria_label,
        role: resolved.role,
        bbox: resolved.bbox
    };
}
//# sourceMappingURL=dom-primitives-gestures.js.map