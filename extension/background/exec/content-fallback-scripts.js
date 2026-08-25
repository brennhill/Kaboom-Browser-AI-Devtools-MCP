/**
 * Purpose: Self-contained extraction fallbacks used when content scripts are unavailable.
 * Why: Keep fallback script implementations centralized and reusable across command handlers.
 * Docs: docs/features/feature/interact-explore/index.md
 *
 * Each exported script function is injected via chrome.scripting.executeScript({func}):
 * Chrome serializes the function alone, so every helper and selector list MUST be
 * nested inside the function body — module-scope references are lost on injection.
 */
export function readableFallbackScript() {
    const mainSelectors = [
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
    const removeSelectors = [
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
    // jscpd:ignore-start -- each fallback script must be self-contained for chrome.scripting
    // serialization; the shared pickMainElement logic is deliberately duplicated.
    const pickMainElement = (minTextLength) => {
        const fallback = document.body || document.documentElement;
        for (const selector of mainSelectors) {
            const el = document.querySelector(selector);
            if (!el)
                continue;
            const text = (el.innerText || el.textContent || '').trim();
            if (text.length > minTextLength) {
                return el;
            }
        }
        return fallback;
    };
    // jscpd:ignore-end
    const extractCleanMainText = () => {
        const clone = pickMainElement(100).cloneNode(true);
        for (const sel of removeSelectors) {
            for (const child of Array.from(clone.querySelectorAll(sel)))
                child.remove();
        }
        return (clone.innerText || clone.textContent || '').replace(/\s+/g, ' ').trim();
    };
    const readByline = () => {
        for (const sel of ['.author', '[rel="author"]', '.byline', '.post-author', 'meta[name="author"]']) {
            const el = document.querySelector(sel);
            if (el) {
                const text = (el.getAttribute('content') || el.innerText || '').trim();
                if (text.length > 0 && text.length < 200) {
                    return text;
                }
            }
        }
        return '';
    };
    const content = extractCleanMainText();
    return {
        title: document.title || '',
        content,
        excerpt: content.slice(0, 300),
        byline: readByline(),
        word_count: content.split(/\s+/).filter(Boolean).length,
        url: window.location.href,
        fallback: true
    };
}
function markdownFallbackScript() {
    const mainSelectors = [
        'main',
        'article',
        '[role="main"]',
        '#main',
        '.main',
        '.post-content',
        '.entry-content',
        '.article-body',
        '.article-content'
    ];
    const removeSelectors = [
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
        '[aria-hidden="true"]'
    ];
    // jscpd:ignore-start -- each fallback script must be self-contained for chrome.scripting
    // serialization; the shared pickMainElement logic is deliberately duplicated.
    const pickMainElement = (minTextLength) => {
        const fallback = document.body || document.documentElement;
        for (const selector of mainSelectors) {
            const el = document.querySelector(selector);
            if (!el)
                continue;
            const text = (el.innerText || el.textContent || '').trim();
            if (text.length > minTextLength) {
                return el;
            }
        }
        return fallback;
    };
    // jscpd:ignore-end
    const MAX_OUTPUT = 200000;
    const clone = pickMainElement(100).cloneNode(true);
    for (const sel of removeSelectors) {
        for (const child of Array.from(clone.querySelectorAll(sel)))
            child.remove();
    }
    let markdown = (clone.innerText || clone.textContent || '').replace(/\s+/g, ' ').trim();
    if (markdown.length > MAX_OUTPUT) {
        markdown = markdown.slice(0, MAX_OUTPUT);
    }
    return {
        title: document.title || '',
        markdown,
        url: window.location.href,
        word_count: markdown.split(/\s+/).filter(Boolean).length,
        fallback: true
    };
}
function pageSummaryFallbackScript() {
    const mainSelectors = ['main', 'article', '[role="main"]', '#main', '.main', '.post-content', '.entry-content'];
    const cleanText = (value) => value.replace(/\s+/g, ' ').trim();
    const normalizeURL = (value) => {
        try {
            return new URL(value, window.location.href).href;
        }
        catch {
            // EXPECTED_ABSENCE: optional enrichment can normally fail while the primary
            // operation keeps a valid fallback; logging it would misleadingly report fallback as failure.
            return value;
        }
    };
    const collectHeadings = () => {
        const headings = [];
        for (const heading of Array.from(document.querySelectorAll('h1, h2, h3'))) {
            if (headings.length >= 30)
                break;
            const text = cleanText(heading.innerText || heading.textContent || '').slice(0, 200);
            if (text)
                headings.push(heading.tagName.toLowerCase() + ': ' + text);
        }
        return headings;
    };
    const collectNavLinks = () => {
        const navCandidates = document.querySelectorAll('nav a[href], header a[href], [role="navigation"] a[href]');
        const navLinks = [];
        const seenNav = {};
        for (const link of Array.from(navCandidates)) {
            if (navLinks.length >= 25)
                break;
            const linkText = cleanText(link.innerText || link.textContent || '').slice(0, 80);
            const href = normalizeURL(link.getAttribute('href') || '');
            if (!href)
                continue;
            const key = linkText + '|' + href;
            if (seenNav[key])
                continue;
            seenNav[key] = true;
            navLinks.push({ text: linkText, href });
        }
        return navLinks;
    };
    const collectFormFields = (form) => {
        const fields = [];
        const seenFields = {};
        for (const field of Array.from(form.querySelectorAll('input, select, textarea'))) {
            if (fields.length >= 25)
                break;
            const name = field.getAttribute('name') ||
                field.getAttribute('id') ||
                field.getAttribute('aria-label') ||
                field.getAttribute('type') ||
                field.tagName.toLowerCase();
            const cleaned = cleanText(name || '').slice(0, 60);
            if (!cleaned || seenFields[cleaned])
                continue;
            seenFields[cleaned] = true;
            fields.push(cleaned);
        }
        return fields;
    };
    const collectForms = () => {
        const forms = [];
        for (const form of Array.from(document.querySelectorAll('form'))) {
            if (forms.length >= 10)
                break;
            forms.push({
                action: normalizeURL(form.getAttribute('action') || window.location.href),
                method: (form.getAttribute('method') || 'GET').toUpperCase(),
                fields: collectFormFields(form)
            });
        }
        return forms;
    };
    // jscpd:ignore-start -- each fallback script must be self-contained for chrome.scripting
    // serialization; the shared pickMainElement logic is deliberately duplicated.
    const pickMainElement = (minTextLength) => {
        const fallback = document.body || document.documentElement;
        for (const selector of mainSelectors) {
            const el = document.querySelector(selector);
            if (!el)
                continue;
            const text = (el.innerText || el.textContent || '').trim();
            if (text.length > minTextLength) {
                return el;
            }
        }
        return fallback;
    };
    // jscpd:ignore-end
    const countFormFields = (forms) => {
        let totalFormFields = 0;
        for (const f of forms)
            totalFormFields += f.fields.length;
        return totalFormFields;
    };
    const looksLikeArticle = (paragraphCount, linkCount) => document.querySelectorAll('article').length > 0 || (paragraphCount >= 8 && linkCount < paragraphCount * 2);
    const looksLikeSearchResults = (linkCount) => {
        const hasSearchInput = !!document.querySelector('input[type="search"], input[name*="search" i], input[placeholder*="search" i]');
        const likelySearchURL = /[?&](q|query|search)=/i.test(window.location.search);
        return hasSearchInput && (likelySearchURL || linkCount > 10);
    };
    const classifyPageType = (headings, forms, preview, interactiveCount) => {
        const linkCount = document.querySelectorAll('a[href]').length;
        const paragraphCount = document.querySelectorAll('p').length;
        const hasTable = document.querySelectorAll('table').length > 0;
        if (looksLikeSearchResults(linkCount))
            return 'search_results';
        if (forms.length > 0 && countFormFields(forms) >= 3 && paragraphCount < 8)
            return 'form';
        if (looksLikeArticle(paragraphCount, linkCount))
            return 'article';
        if (hasTable || (interactiveCount > 25 && headings.length >= 2))
            return 'dashboard';
        if (linkCount > 30 && paragraphCount < 10)
            return 'link_list';
        if (preview.length < 80 && interactiveCount > 10)
            return 'app';
        return 'generic';
    };
    const headings = collectHeadings();
    const navLinks = collectNavLinks();
    const forms = collectForms();
    const mainEl = pickMainElement(120);
    const mainText = cleanText(mainEl.innerText || mainEl.textContent || '').slice(0, 20000);
    const preview = mainText.slice(0, 500);
    const wordCount = mainText ? mainText.split(/\s+/).filter(Boolean).length : 0;
    const interactiveCount = document.querySelectorAll('a[href],button,input:not([type="hidden"]),select,textarea,[role="button"],[role="link"]').length;
    return {
        url: window.location.href,
        title: document.title || '',
        type: classifyPageType(headings, forms, preview, interactiveCount),
        headings,
        nav_links: navLinks,
        forms,
        interactive_element_count: interactiveCount,
        main_content_preview: preview,
        word_count: wordCount,
        fallback: true
    };
}
// Keys MUST be the runtime message types dispatched by interact-content.ts and
// handled in content/runtime-message-listener.ts. They were previously UPPERCASE,
// which made every FALLBACK_SCRIPTS[messageType] lookup miss — silently disabling
// the content-script-unreachable fallback for all three extractors.
export const FALLBACK_SCRIPTS = {
    kaboom_get_readable: readableFallbackScript,
    kaboom_get_markdown: markdownFallbackScript,
    kaboom_page_summary: pageSummaryFallbackScript
};
//# sourceMappingURL=content-fallback-scripts.js.map