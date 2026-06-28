/**
 * Purpose: Captures transient UI elements (toasts, alerts, snackbars) via MutationObserver.
 * Why: AI agents miss transient UI because elements disappear before the next screenshot.
 * Docs: docs/features/feature/transient-capture/index.md
 */
import { recordEnhancedAction } from './reproduction.js';
// Tags to skip entirely — never transient UI
const SKIP_TAGS = new Set(['SCRIPT', 'STYLE', 'LINK', 'META', 'NOSCRIPT', 'BR', 'HR']);
// Class fingerprints mapped to classification
const CLASS_FINGERPRINTS = [
    [/toast/i, 'toast'],
    [/snackbar/i, 'snackbar'],
    [/notification/i, 'notification'],
    [/tooltip/i, 'tooltip'],
    [/alert/i, 'alert'],
    [/banner/i, 'banner'],
    [/flash/i, 'flash']
];
// Dedup window in milliseconds
const DEDUP_WINDOW_MS = 2000;
// Max dedup map entries before cleanup
const DEDUP_MAP_CAP = 100;
// Max text length to record
const MAX_TEXT_LENGTH = 500;
// Max text length for dedup key
const DEDUP_KEY_TEXT_LENGTH = 100;
// MutationObserver instance
let observer = null;
// Dedup map: key → last seen timestamp
const dedupMap = new Map();
/**
 * Classify an element as a transient UI element, or return null if not transient.
 * Priority: ARIA > class fingerprints > computed style heuristic.
 */
export function classifyTransient(el) {
    const tag = el.tagName;
    if (!tag || SKIP_TAGS.has(tag))
        return null;
    const text = extractText(el);
    if (!text)
        return null;
    // Priority 1: ARIA attributes
    const role = el.getAttribute('role');
    const ariaLive = el.getAttribute('aria-live');
    if (role === 'alert' || ariaLive === 'assertive') {
        return { classification: 'alert', role: role || 'alert', text };
    }
    if (role === 'status' || ariaLive === 'polite') {
        return { classification: 'toast', role: role || 'status', text };
    }
    // Priority 2: Class fingerprints
    const className = el.className;
    if (className && typeof className === 'string') {
        for (const [pattern, classification] of CLASS_FINGERPRINTS) {
            if (pattern.test(className)) {
                return { classification, role: role || '', text };
            }
        }
    }
    // Priority 3: Computed style heuristic
    if (typeof window !== 'undefined' && window.getComputedStyle) {
        try {
            const style = window.getComputedStyle(el);
            const position = style.position;
            if (position === 'fixed' || position === 'absolute') {
                const zIndex = parseInt(style.zIndex, 10);
                const height = el.getBoundingClientRect().height;
                if (zIndex > 1000 && height > 0 && height < 200) {
                    return { classification: 'flash', role: role || '', text };
                }
            }
        }
        catch {
            // getComputedStyle can throw in detached elements
        }
    }
    return null;
}
/**
 * Extract visible text content from an element, trimmed and capped.
 */
function extractText(el) {
    const raw = (el.textContent || '').trim();
    return raw.slice(0, MAX_TEXT_LENGTH);
}
/**
 * Generate a dedup key for a transient element.
 */
function dedupKey(classification, text) {
    return `${classification}:${text.slice(0, DEDUP_KEY_TEXT_LENGTH)}`;
}
/**
 * Check if this transient was recently seen (within DEDUP_WINDOW_MS).
 */
function isDuplicate(key, now) {
    const entry = dedupMap.get(key);
    return entry !== undefined && now - entry.timestamp < DEDUP_WINDOW_MS;
}
/**
 * Record a dedup entry and clean stale entries if map is over capacity.
 */
function recordDedup(key, now) {
    dedupMap.set(key, { timestamp: now });
    if (dedupMap.size > DEDUP_MAP_CAP) {
        // Safe: JS Map spec allows deletion during for...of iteration
        for (const [k, v] of dedupMap) {
            if (now - v.timestamp > DEDUP_WINDOW_MS) {
                dedupMap.delete(k);
            }
        }
    }
}
/**
 * Classify an element and its children as transient candidates.
 * Walks one level of children to catch framework wrapper patterns
 * (e.g., React portals wrapping an inner element with role="alert").
 */
export function classifyCandidates(el) {
    // Try the element itself first
    const info = classifyTransient(el);
    if (info)
        return info;
    // Walk direct children — frameworks often add a wrapper div around the ARIA element
    for (let i = 0; i < el.children.length; i++) {
        const child = el.children[i];
        if (child) {
            const childInfo = classifyTransient(child);
            if (childInfo)
                return childInfo;
        }
    }
    return null;
}
/**
 * Record a batch of pre-classified transients (deferred from mutation callback).
 */
function recordPendingTransients(pending) {
    const now = Date.now();
    for (const { element, info } of pending) {
        const key = dedupKey(info.classification, info.text);
        if (isDuplicate(key, now))
            continue;
        recordDedup(key, now);
        // Pass null for element: by the time this deferred callback fires, the transient
        // element is likely removed from the DOM, making computeSelectors return incomplete
        // CSS paths. Selectors are not useful for transients (observed, not interacted with).
        recordEnhancedAction('transient', null, {
            classification: info.classification,
            duration_ms: 0, // MVP: capture moment only; removal tracking not yet implemented
            role: info.role,
            value: info.text
        });
    }
}
/**
 * MutationObserver callback — classifies elements synchronously (while still attached
 * to DOM) then defers recording to avoid blocking the main thread.
 *
 * Classification must happen synchronously because transient elements may be removed
 * before an idle callback fires, making getComputedStyle return default values.
 */
function mutationCallback(mutations) {
    const pending = [];
    for (const mutation of mutations) {
        if (mutation.type !== 'childList')
            continue;
        for (let i = 0; i < mutation.addedNodes.length; i++) {
            const node = mutation.addedNodes[i];
            if (node.nodeType !== Node.ELEMENT_NODE)
                continue;
            const el = node;
            const info = classifyCandidates(el);
            if (info) {
                pending.push({ element: el, info });
            }
        }
    }
    if (pending.length === 0)
        return;
    // Defer recording (postMessage + buffer append) to avoid blocking layout
    if (typeof requestIdleCallback === 'function') {
        requestIdleCallback(() => recordPendingTransients(pending));
    }
    else {
        setTimeout(() => recordPendingTransients(pending), 0);
    }
}
/**
 * Install the transient element capture MutationObserver.
 */
export function installTransientCapture() {
    if (observer)
        return;
    if (typeof document === 'undefined' || !document.body)
        return;
    if (typeof MutationObserver === 'undefined')
        return;
    observer = new MutationObserver(mutationCallback);
    observer.observe(document.body, { childList: true, subtree: true });
}
/**
 * Uninstall the transient element capture MutationObserver.
 */
export function uninstallTransientCapture() {
    if (observer) {
        observer.disconnect();
        observer = null;
    }
    dedupMap.clear();
}
//# sourceMappingURL=transient-capture.js.map