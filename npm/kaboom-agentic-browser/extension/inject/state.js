/**
 * Purpose: Captures and restores browser state snapshots (localStorage, cookies, scroll position) and manages element highlighting for the AI Web Pilot.
 * Docs: docs/features/feature/state-time-travel/index.md
 */
import { sendPerformanceSnapshot } from '../lib/perf-snapshot.js';
/** Read the page nonce set by the content script on the inject script element */
let pageNonce = '';
if (typeof document !== 'undefined' && typeof document.querySelector === 'function') {
    const nonceEl = document.querySelector('script[data-kaboom-nonce]');
    if (nonceEl) {
        pageNonce = nonceEl.getAttribute('data-kaboom-nonce') || '';
    }
}
/** Patterns for sensitive storage keys whose values should be redacted */
const SENSITIVE_KEY_PATTERNS = /token|secret|password|api.?key|auth|session.?id|csrf|jwt/i;
let kaboomHighlighter = null;
/**
 * Capture browser state (localStorage, sessionStorage, cookies).
 * Returns a snapshot that can be restored later.
 */
export function captureState() {
    const state = {
        url: window.location.href,
        timestamp: Date.now(),
        localStorage: {},
        sessionStorage: {},
        cookies: document.cookie
            .split(';')
            .map((c) => {
            const [name, ...rest] = c.split('=');
            if (name && SENSITIVE_KEY_PATTERNS.test(name.trim())) {
                return `${name}=[REDACTED]`;
            }
            return c;
        })
            .join(';')
    };
    const localStorageData = {};
    for (let i = 0; i < localStorage.length; i++) {
        const key = localStorage.key(i);
        if (key) {
            localStorageData[key] = SENSITIVE_KEY_PATTERNS.test(key) ? '[REDACTED]' : localStorage.getItem(key) || '';
        }
    }
    ;
    state.localStorage = localStorageData;
    const sessionStorageData = {};
    for (let i = 0; i < sessionStorage.length; i++) {
        const key = sessionStorage.key(i);
        if (key) {
            sessionStorageData[key] = SENSITIVE_KEY_PATTERNS.test(key) ? '[REDACTED]' : sessionStorage.getItem(key) || '';
        }
    }
    ;
    state.sessionStorage = sessionStorageData;
    return state;
}
/**
 * Validates a storage key to prevent prototype pollution and other attacks
 */
function isValidStorageKey(key) {
    if (typeof key !== 'string')
        return false;
    if (key.length === 0 || key.length > 256)
        return false;
    // Reject prototype pollution vectors
    const dangerous = ['__proto__', 'constructor', 'prototype'];
    const lowerKey = key.toLowerCase();
    for (const pattern of dangerous) {
        if (lowerKey.includes(pattern))
            return false;
    }
    return true;
}
/**
 * Restore browser state from a snapshot.
 * Clears existing state before restoring.
 */
const MAX_STORAGE_VALUE_SIZE = 10 * 1024 * 1024;
// #lizard forgives
function restoreStorageEntries(storage, entries, label) {
    let skipped = 0;
    for (const [key, value] of Object.entries(entries)) {
        if (!isValidStorageKey(key)) {
            skipped++;
            console.warn(`[KaBOOM!] Skipped ${label} key with invalid pattern:`, key); // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.warn with internal state key, not user-controlled
            continue;
        }
        if (typeof value === 'string' && value.length > MAX_STORAGE_VALUE_SIZE) {
            skipped++;
            console.warn(`[KaBOOM!] Skipped ${label} value exceeding 10MB:`, key); // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.warn with internal state key, not user-controlled
            continue;
        }
        storage.setItem(key, value);
    }
    return skipped;
}
function clearAllCookies() {
    const isSecure = window.location.protocol === 'https:';
    document.cookie.split(';').forEach((c) => {
        const name = (c.split('=')[0] || '').trim();
        if (!name)
            return;
        let deleteCookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`;
        if (isSecure)
            deleteCookie += '; Secure';
        deleteCookie += '; SameSite=Strict';
        document.cookie = deleteCookie;
    });
}
function restoreCookies(cookieString) {
    const isSecure = window.location.protocol === 'https:';
    cookieString.split(';').forEach((c) => {
        const trimmed = c.trim();
        if (!trimmed)
            return;
        let securedCookie = trimmed;
        if (isSecure && !securedCookie.toLowerCase().includes('secure'))
            securedCookie += '; Secure';
        if (!securedCookie.toLowerCase().includes('samesite'))
            securedCookie += '; SameSite=Strict';
        document.cookie = securedCookie;
    });
}
function navigateSameOrigin(url) {
    if (url === window.location.href)
        return;
    try {
        const parsed = new URL(url);
        if ((parsed.protocol === 'http:' || parsed.protocol === 'https:') && parsed.origin === window.location.origin) {
            window.location.href = url;
        }
        else {
            console.warn('[KaBOOM!] Skipped navigation: URL must be same origin', url, 'current:', window.location.origin);
        }
    }
    catch (e) {
        console.warn('[KaBOOM!] Invalid URL for navigation:', url, e);
    }
}
// #lizard forgives
export function restoreState(state, includeUrl = true) {
    if (!state || typeof state !== 'object') {
        return { success: false, error: 'Invalid state object' };
    }
    let skipped = restoreStorageEntries(localStorage, state.localStorage || {}, 'localStorage');
    skipped += restoreStorageEntries(sessionStorage, state.sessionStorage || {}, 'sessionStorage');
    clearAllCookies();
    if (state.cookies)
        restoreCookies(state.cookies);
    const restored = {
        localStorage: Object.keys(state.localStorage || {}).length - skipped,
        sessionStorage: Object.keys(state.sessionStorage || {}).length,
        cookies: (state.cookies || '').split(';').filter((c) => c.trim()).length,
        skipped
    };
    if (includeUrl && state.url)
        navigateSameOrigin(state.url);
    if (skipped > 0)
        console.warn(`[KaBOOM!] restoreState completed with ${skipped} skipped item(s)`);
    return { success: true, restored };
}
/**
 * Highlight a DOM element by injecting a blue glow overlay div.
 */
// #lizard forgives
export function highlightElement(selector, durationMs = 5000) {
    // Remove existing highlight
    if (kaboomHighlighter) {
        kaboomHighlighter.remove();
        kaboomHighlighter = null;
    }
    const element = document.querySelector(selector);
    if (!element) {
        return { success: false, error: 'element_not_found', selector };
    }
    const rect = element.getBoundingClientRect();
    kaboomHighlighter = document.createElement('div');
    kaboomHighlighter.id = 'kaboom-highlighter';
    kaboomHighlighter.dataset.selector = selector;
    Object.assign(kaboomHighlighter.style, {
        position: 'fixed',
        top: `${rect.top}px`,
        left: `${rect.left}px`,
        width: `${rect.width}px`,
        height: `${rect.height}px`,
        border: '2px solid rgba(59, 130, 246, 0.7)',
        borderRadius: '4px',
        backgroundColor: 'rgba(59, 130, 246, 0.08)',
        boxShadow: '0 0 12px rgba(59, 130, 246, 0.5)',
        zIndex: '2147483647',
        pointerEvents: 'none',
        boxSizing: 'border-box'
    });
    const targetElement = document.body || document.documentElement;
    if (targetElement) {
        targetElement.appendChild(kaboomHighlighter);
    }
    else {
        console.warn('[KaBOOM!] No document body available for highlighter injection');
        return;
    }
    setTimeout(() => {
        if (kaboomHighlighter) {
            kaboomHighlighter.remove();
            kaboomHighlighter = null;
        }
    }, durationMs);
    return {
        success: true,
        selector,
        bounds: { x: rect.x, y: rect.y, width: rect.width, height: rect.height }
    };
}
/**
 * Clear any existing highlight
 */
// #lizard forgives
export function clearHighlight() {
    if (kaboomHighlighter) {
        kaboomHighlighter.remove();
        kaboomHighlighter = null;
    }
}
/**
 * Handle scroll - update highlight position
 */
if (typeof window !== 'undefined') {
    window.addEventListener('scroll', () => {
        if (kaboomHighlighter) {
            const selector = kaboomHighlighter.dataset.selector;
            if (selector) {
                const el = document.querySelector(selector);
                if (el) {
                    const rect = el.getBoundingClientRect();
                    kaboomHighlighter.style.top = `${rect.top}px`;
                    kaboomHighlighter.style.left = `${rect.left}px`;
                }
            }
        }
    }, { passive: true });
}
/**
 * Handle KABOOM_HIGHLIGHT_REQUEST messages from content script
 */
if (typeof window !== 'undefined') {
    window.addEventListener('message', (event) => {
        if (event.source !== window || event.origin !== window.location.origin)
            return;
        if (pageNonce && event.data?._nonce !== pageNonce)
            return;
        if (event.data?.type === 'kaboom_highlight_request') {
            const { requestId, params } = event.data;
            const { selector, duration_ms } = params || { selector: '' };
            const result = highlightElement(selector, duration_ms);
            window.postMessage({
                type: 'kaboom_highlight_response',
                requestId,
                result
            }, window.location.origin);
        }
    });
}
/**
 * Wrapper for sending performance snapshot (exported for compatibility)
 */
export function sendPerformanceSnapshotWrapper() {
    sendPerformanceSnapshot();
}
//# sourceMappingURL=state.js.map