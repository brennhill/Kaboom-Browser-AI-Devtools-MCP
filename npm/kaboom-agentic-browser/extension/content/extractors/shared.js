// shared.ts — Shared utilities for content extractors.
// Provides canonical main content detection used by readable, markdown, and page-summary.
/** Ordered by specificity — semantic elements first, then common class/ID patterns. */
const MAIN_CONTENT_SELECTORS = [
    'main',
    'article',
    '[role="main"]',
    '#main',
    '.main',
    '.post-content',
    '.entry-content',
    '.article-body',
    '.article-content',
    '.story-body',
    '.article',
    '.post',
    '#content',
    '.content',
    '.results'
];
/**
 * Find the main content element by probing semantic and common selectors.
 * Falls back to document.body (or document.documentElement as a null-safe fallback).
 */
export function findMainContentElement(minTextLength = 100) {
    for (const sel of MAIN_CONTENT_SELECTORS) {
        const el = document.querySelector(sel);
        if (!el)
            continue;
        const text = (el.innerText || el.textContent || '').trim();
        if (text.length > minTextLength)
            return el;
    }
    return document.body || document.documentElement;
}
//# sourceMappingURL=shared.js.map