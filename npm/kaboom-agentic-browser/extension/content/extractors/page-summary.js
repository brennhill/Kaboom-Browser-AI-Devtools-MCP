// page-summary.ts — Page summary extraction for page_summary query type.
// Runs in the content script's ISOLATED world (CSP-safe, no eval).
// Issue #257: Replaces the IIFE string that was embedded in the Go handler.
import { findMainContentElement } from './shared.js';
function cleanText(value, maxLen) {
    let text = (value || '')
        .replace(/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]/g, '')
        .replace(/\s+/g, ' ')
        .trim();
    if (maxLen > 0 && text.length > maxLen) {
        text = text.slice(0, maxLen);
    }
    return text;
}
function absoluteHref(value) {
    try {
        return new URL(value || '', window.location.href).href;
    }
    catch {
        return value || '';
    }
}
function visibleInteractiveCount() {
    const nodes = document.querySelectorAll('a[href],button,input:not([type="hidden"]),select,textarea,[role="button"],[role="link"],[tabindex]');
    let count = 0;
    for (const node of Array.from(nodes)) {
        if (node.disabled)
            continue;
        const style = window.getComputedStyle(node);
        if (style.display === 'none' || style.visibility === 'hidden')
            continue;
        const rect = node.getBoundingClientRect();
        if (rect.width <= 0 || rect.height <= 0)
            continue;
        count += 1;
    }
    return count;
}
function findMainNode() {
    return findMainContentElement(120);
}
function classifyPage(forms, interactiveCount, linkCount, paragraphCount, headingCount, previewText) {
    const hasSearchInput = !!document.querySelector('input[type="search"], input[name*="search" i], input[placeholder*="search" i]');
    const likelySearchURL = /[?&](q|query|search)=/i.test(window.location.search);
    const hasArticle = document.querySelectorAll('article').length > 0;
    const hasTable = document.querySelectorAll('table').length > 0;
    let totalFormFields = 0;
    for (const form of forms) {
        totalFormFields += form.fields.length;
    }
    if (hasSearchInput && (likelySearchURL || linkCount > 10))
        return 'search_results';
    if (forms.length > 0 && totalFormFields >= 3 && paragraphCount < 8)
        return 'form';
    if (hasArticle || (paragraphCount >= 8 && linkCount < paragraphCount * 2))
        return 'article';
    if (hasTable || (interactiveCount > 25 && headingCount >= 2))
        return 'dashboard';
    if (linkCount > 30 && paragraphCount < 10)
        return 'link_list';
    if (previewText.length < 80 && interactiveCount > 10)
        return 'app';
    return 'generic';
}
/**
 * Extract a structured page summary from the current page.
 * Returns headings, navigation links, forms, interactive count, content preview, and classification.
 */
export function extractPageSummary() {
    // Headings
    const headingNodes = document.querySelectorAll('h1, h2, h3');
    const headings = [];
    for (const heading of Array.from(headingNodes)) {
        if (headings.length >= 30)
            break;
        const text = cleanText(heading.innerText || heading.textContent || '', 200);
        if (!text)
            continue;
        headings.push(heading.tagName.toLowerCase() + ': ' + text);
    }
    // Navigation links
    const navCandidates = document.querySelectorAll('nav a[href], header a[href], [role="navigation"] a[href]');
    const navLinks = [];
    const seenNav = {};
    for (const link of Array.from(navCandidates)) {
        if (navLinks.length >= 25)
            break;
        const linkText = cleanText(link.innerText || link.textContent || '', 80);
        const href = absoluteHref(link.getAttribute('href') || '');
        if (!href)
            continue;
        const key = linkText + '|' + href;
        if (seenNav[key])
            continue;
        seenNav[key] = true;
        navLinks.push({ text: linkText, href });
    }
    // Forms
    const forms = [];
    const formNodes = document.querySelectorAll('form');
    for (const form of Array.from(formNodes)) {
        if (forms.length >= 10)
            break;
        const fieldNodes = form.querySelectorAll('input, select, textarea');
        const fields = [];
        const seenFields = {};
        for (const field of Array.from(fieldNodes)) {
            if (fields.length >= 25)
                break;
            const candidate = field.getAttribute('name') ||
                field.getAttribute('id') ||
                field.getAttribute('aria-label') ||
                field.getAttribute('type') ||
                field.tagName.toLowerCase();
            const cleaned = cleanText(candidate || '', 60);
            if (!cleaned || seenFields[cleaned])
                continue;
            seenFields[cleaned] = true;
            fields.push(cleaned);
        }
        forms.push({
            action: absoluteHref(form.getAttribute('action') || window.location.href),
            method: (form.getAttribute('method') || 'GET').toUpperCase(),
            fields
        });
    }
    // Main content preview
    const mainNode = findMainNode();
    const mainText = cleanText(mainNode ? mainNode.innerText || mainNode.textContent || '' : '', 20000);
    const preview = mainText.slice(0, 500);
    const wordCount = mainText ? mainText.split(/\s+/).filter(Boolean).length : 0;
    // Counts and classification
    const linkCount = document.querySelectorAll('a[href]').length;
    const paragraphCount = document.querySelectorAll('p').length;
    const interactiveCount = visibleInteractiveCount();
    const pageType = classifyPage(forms, interactiveCount, linkCount, paragraphCount, headings.length, preview);
    return {
        url: window.location.href,
        title: document.title || '',
        type: pageType,
        headings,
        nav_links: navLinks,
        forms,
        interactive_element_count: interactiveCount,
        main_content_preview: preview,
        word_count: wordCount
    };
}
//# sourceMappingURL=page-summary.js.map