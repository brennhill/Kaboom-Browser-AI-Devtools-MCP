// readable.ts — Readable content extraction for get_readable query type.
// Runs in the content script's ISOLATED world (CSP-safe, no eval).
// Issue #257: Replaces the IIFE string that was embedded in the Go handler.
import { findMainContentElement } from './shared.js';
/** Tags and selectors to strip from content before extracting text. */
const REMOVE_SELECTORS = [
    'nav',
    'header',
    'footer',
    'aside',
    'script',
    'style',
    'noscript',
    'svg',
    '[role="navigation"]',
    '[role="banner"]',
    '[role="contentinfo"]',
    '[aria-hidden="true"]',
    '.ad',
    '.ads',
    '.advertisement',
    '.social-share',
    '.comments',
    '.sidebar',
    '.related-posts',
    '.newsletter'
];
/**
 * Strip navigation, ads, and other non-content elements from a cloned node
 * and return the cleaned text content.
 */
function cleanText(el) {
    if (!el)
        return '';
    const clone = el.cloneNode(true);
    for (const sel of REMOVE_SELECTORS) {
        const els = clone.querySelectorAll(sel);
        for (const child of Array.from(els))
            child.remove();
    }
    return (clone.innerText || clone.textContent || '').replace(/\s+/g, ' ').trim();
}
/**
 * Extract the author/byline from common page metadata patterns.
 */
function getByline() {
    const selectors = ['.author', '[rel="author"]', '.byline', '.post-author', 'meta[name="author"]'];
    for (const sel of selectors) {
        const el = document.querySelector(sel);
        if (el) {
            const text = (el.getAttribute('content') || el.innerText || '').trim();
            if (text.length > 0 && text.length < 200)
                return text;
        }
    }
    return '';
}
/**
 * Extract readable content from the current page.
 * Returns structured data with title, content, excerpt, byline, word count, and URL.
 */
export function extractReadable() {
    const main = findMainContentElement(100);
    const content = cleanText(main);
    const excerpt = content.slice(0, 300);
    const words = content.split(/\s+/).filter(Boolean);
    return {
        title: document.title || '',
        content,
        excerpt,
        byline: getByline(),
        word_count: words.length,
        url: window.location.href
    };
}
//# sourceMappingURL=readable.js.map