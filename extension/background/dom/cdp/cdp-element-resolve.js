/**
 * Purpose: Page-injected element resolution for CDP escalation — finds elements by selector, gets bounding rects.
 * Why: Self-contained function injected via chrome.scripting.executeScript; must have no outer-scope closures.
 * Docs: docs/features/feature/interact-explore/index.md
 */
/**
 * Injected into the page via chrome.scripting.executeScript to resolve an
 * element by selector, get its bounding rect, and optionally focus it.
 * Must be fully self-contained — no closures over outer scope.
 */
function cdpResolveAndPrepare(selectorStr, actionType, scopeSelectorStr, elementIdStr) {
    function querySelectorSafe(root, sel) {
        try {
            return root.querySelector(sel);
        }
        catch {
            // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
            // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
            /* invalid selector */
            return null;
        }
    }
    function findByText(searchRoot, value) {
        const all = searchRoot.querySelectorAll('*');
        for (let i = 0; i < all.length; i++) {
            const candidate = all[i];
            if (!candidate)
                continue;
            const textContent = candidate.textContent?.trim() || '';
            if (textContent === value || textContent.startsWith(value)) {
                return candidate;
            }
        }
        return null;
    }
    // Resolve a CSS or semantic (text=, role=, label=, placeholder=) selector
    function resolveSelector(root, sel) {
        const eqIdx = sel.indexOf('=');
        if (eqIdx <= 0)
            return querySelectorSafe(root, sel);
        const prefix = sel.substring(0, eqIdx);
        const value = sel.substring(eqIdx + 1);
        switch (prefix) {
            case 'text': {
                const searchRoot = root === document ? document.body : root;
                return searchRoot ? findByText(searchRoot, value) : null;
            }
            case 'role':
                return root.querySelector(`[role="${value}"]`);
            case 'label':
            case 'aria-label':
                return root.querySelector(`[aria-label="${value}"]`);
            case 'placeholder':
                return root.querySelector(`[placeholder="${value}"]`);
            default:
                return querySelectorSafe(root, sel);
        }
    }
    let root = document;
    if (scopeSelectorStr) {
        const scope = document.querySelector(scopeSelectorStr);
        if (scope)
            root = scope;
    }
    let el = null;
    // Try element_id first
    if (elementIdStr) {
        el = root.querySelector(`[data-kaboom-eid="${elementIdStr}"]`);
    }
    // Resolve selector (CSS or semantic)
    if (!el && selectorStr) {
        el = resolveSelector(root, selectorStr);
    }
    if (!el)
        return null;
    const rect = el.getBoundingClientRect();
    if (rect.width === 0 && rect.height === 0)
        return null; // Hidden element
    // Focus for type/key_press so CDP key events land on the right element
    if (actionType === 'type' || actionType === 'key_press') {
        ;
        el.focus?.();
    }
    return {
        x: rect.left + rect.width / 2,
        y: rect.top + rect.height / 2,
        tag: el.tagName.toLowerCase(),
        text_preview: (el.textContent || '').trim().substring(0, 80),
        selector: selectorStr,
        element_id: el.getAttribute('data-kaboom-eid') || undefined,
        aria_label: el.getAttribute('aria-label') || undefined,
        role: el.getAttribute('role') || undefined,
        bbox: { x: rect.x, y: rect.y, width: rect.width, height: rect.height }
    };
}
export async function resolveElement(tabId, params) {
    const results = await chrome.scripting.executeScript({
        target: { tabId },
        world: 'MAIN',
        func: cdpResolveAndPrepare,
        args: [params.selector || '', params.action || '', params.scope_selector ?? null, params.element_id ?? null]
    });
    return results?.[0]?.result ?? null;
}
export function buildCDPResult(action, selector, resolved, elapsedMs, extra) {
    return {
        success: true,
        action,
        selector,
        matched: {
            tag: resolved.tag,
            text_preview: resolved.text_preview,
            selector: resolved.selector,
            element_id: resolved.element_id,
            aria_label: resolved.aria_label,
            role: resolved.role,
            bbox: resolved.bbox
        },
        timing: { total_ms: elapsedMs },
        insertion_strategy: 'cdp',
        ...extra
    };
}
//# sourceMappingURL=cdp-element-resolve.js.map