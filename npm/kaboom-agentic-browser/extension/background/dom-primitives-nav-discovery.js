// dom-primitives-nav-discovery.ts — Self-contained navigation discovery DOM primitive for chrome.scripting.executeScript.
// Extracted from analyze-navigation.ts and interact-explore.ts to eliminate duplication (#9.6).
// This function MUST remain self-contained — Chrome serializes the function source only (no closures).
/**
 * Self-contained script injected via chrome.scripting.executeScript to
 * discover navigable links grouped by landmark region.
 *
 * Returns regions (nav, header, footer, aside, etc.) with their links,
 * unregioned top-level links, and a summary with link counts.
 */
export function domPrimitiveNavDiscovery() {
    const MAX_LINKS_PER_REGION = 50;
    const MAX_REGIONS = 20;
    function cleanText(value, maxLen) {
        const text = (value || '')
            .replace(/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]/g, '')
            .replace(/\s+/g, ' ')
            .trim();
        if (maxLen > 0 && text.length > maxLen)
            return text.slice(0, maxLen);
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
    function isSameOrigin(href) {
        try {
            return new URL(href).origin === window.location.origin;
        }
        catch {
            return false;
        }
    }
    function getRegionLabel(el) {
        const ariaLabel = el.getAttribute('aria-label');
        if (ariaLabel)
            return cleanText(ariaLabel, 80);
        const ariaLabelledBy = el.getAttribute('aria-labelledby');
        if (ariaLabelledBy) {
            const labelEl = document.getElementById(ariaLabelledBy);
            if (labelEl) {
                const text = cleanText(labelEl.textContent || '', 80);
                if (text)
                    return text;
            }
        }
        const heading = el.querySelector('h1, h2, h3, h4, h5, h6');
        if (heading) {
            const text = cleanText(heading.textContent || '', 80);
            if (text)
                return text;
        }
        return '';
    }
    function getPositionLabel(el) {
        const rect = el.getBoundingClientRect();
        const viewH = window.innerHeight;
        if (rect.top < viewH * 0.15)
            return 'top';
        if (rect.bottom > viewH * 0.85)
            return 'bottom';
        if (rect.left < window.innerWidth * 0.25 && rect.width < window.innerWidth * 0.35)
            return 'left_sidebar';
        if (rect.right > window.innerWidth * 0.75 && rect.width < window.innerWidth * 0.35)
            return 'right_sidebar';
        return 'main';
    }
    const regionSelectors = [
        'nav',
        '[role="navigation"]',
        'header',
        'footer',
        'aside',
        '[role="banner"]',
        '[role="contentinfo"]',
        '[role="complementary"]'
    ];
    const seenRegions = new Set();
    const regions = [];
    for (const sel of regionSelectors) {
        const elements = document.querySelectorAll(sel);
        for (const el of Array.from(elements)) {
            if (seenRegions.has(el) || regions.length >= MAX_REGIONS)
                continue;
            seenRegions.add(el);
            const anchors = el.querySelectorAll('a[href]');
            if (anchors.length === 0)
                continue;
            const links = [];
            const seenHrefs = new Set();
            for (const a of Array.from(anchors)) {
                if (links.length >= MAX_LINKS_PER_REGION)
                    break;
                const rawHref = a.getAttribute('href') || '';
                if (!rawHref || rawHref === '#' || rawHref.startsWith('javascript:'))
                    continue;
                const href = absoluteHref(rawHref);
                if (seenHrefs.has(href))
                    continue;
                seenHrefs.add(href);
                links.push({ text: cleanText(a.textContent || '', 100), href, is_internal: isSameOrigin(href) });
            }
            if (links.length === 0)
                continue;
            const tag = el.tagName.toLowerCase();
            const role = el.getAttribute('role') || '';
            const label = getRegionLabel(el) || tag;
            const position = getPositionLabel(el);
            regions.push({ tag, role, label, position, links });
        }
    }
    // Unregioned links
    const allAnchors = document.querySelectorAll('a[href]');
    const regionedAnchors = new Set();
    for (const region of seenRegions) {
        for (const a of Array.from(region.querySelectorAll('a[href]')))
            regionedAnchors.add(a);
    }
    const unregionedLinks = [];
    const seenUnregioned = new Set();
    for (const a of Array.from(allAnchors)) {
        if (unregionedLinks.length >= MAX_LINKS_PER_REGION)
            break;
        if (regionedAnchors.has(a))
            continue;
        const rawHref = a.getAttribute('href') || '';
        if (!rawHref || rawHref === '#' || rawHref.startsWith('javascript:'))
            continue;
        const href = absoluteHref(rawHref);
        if (seenUnregioned.has(href))
            continue;
        seenUnregioned.add(href);
        unregionedLinks.push({ text: cleanText(a.textContent || '', 100), href, is_internal: isSameOrigin(href) });
    }
    const totalLinks = regions.reduce((sum, r) => sum + r.links.length, 0) + unregionedLinks.length;
    const internalLinks = regions.reduce((sum, r) => sum + r.links.filter((l) => l.is_internal).length, 0) +
        unregionedLinks.filter((l) => l.is_internal).length;
    return {
        regions,
        unregioned_links: unregionedLinks,
        summary: {
            total_regions: regions.length,
            total_links: totalLinks,
            internal_links: internalLinks,
            external_links: totalLinks - internalLinks
        }
    };
}
//# sourceMappingURL=dom-primitives-nav-discovery.js.map