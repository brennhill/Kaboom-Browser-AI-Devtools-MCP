/**
 * Purpose: Enumerates interactive elements on a page for AI-driven automation.
 * Why: Self-contained for chrome.scripting.executeScript (no closures allowed).
 * Docs: docs/features/feature/interact-explore/index.md
 */
// dom-primitives-list-interactive.ts — Self-contained list_interactive DOM primitive for chrome.scripting.executeScript.
// Extracted from dom-primitives.ts to keep file sizes under the 800 LOC limit.
// This function MUST remain self-contained — Chrome serializes the function source only (no closures).
/**
 * Self-contained function that scans a page for interactive elements.
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveListInteractive }).
 * MUST NOT reference any module-level variables.
 */
export function domPrimitiveListInteractive(scopeSelector, options) {
    function getElementHandleStore() {
        const root = globalThis;
        if (root.__kaboomElementHandles) {
            // Migrate legacy stores that lack selectorByID (#361)
            if (!root.__kaboomElementHandles.selectorByID) {
                root.__kaboomElementHandles.selectorByID = new Map();
            }
            return root.__kaboomElementHandles;
        }
        const created = {
            byElement: new WeakMap(),
            byID: new Map(),
            selectorByID: new Map(),
            nextID: 1
        };
        root.__kaboomElementHandles = created;
        return created;
    }
    // #361: Store selector alongside element_id so stale handles can be re-resolved
    function getOrCreateElementID(el, selector) {
        const store = getElementHandleStore();
        const existing = store.byElement.get(el);
        if (existing) {
            store.byID.set(existing, el);
            if (selector)
                store.selectorByID.set(existing, selector);
            return existing;
        }
        const elementID = `el_${(store.nextID++).toString(36)}`;
        store.byElement.set(el, elementID);
        store.byID.set(elementID, el);
        if (selector)
            store.selectorByID.set(elementID, selector);
        return elementID;
    }
    // — Shadow DOM: deep traversal utilities (duplicated from dom-primitives.ts, required for self-containment) —
    function getShadowRoot(el) {
        return el.shadowRoot ?? null;
    }
    function querySelectorAllDeep(selector, root = document, results = [], depth = 0) {
        if (depth > 10)
            return results;
        results.push(...Array.from(root.querySelectorAll(selector)));
        const children = 'children' in root
            ? root.children
            : root.body?.children || root.documentElement?.children;
        if (!children)
            return results;
        for (let i = 0; i < children.length; i++) {
            const child = children[i];
            const shadow = getShadowRoot(child);
            if (shadow) {
                querySelectorAllDeep(selector, shadow, results, depth + 1);
            }
        }
        return results;
    }
    // — Selector and classification helpers —
    function cssEscape(raw) {
        const maybeCSS = globalThis.CSS;
        if (maybeCSS && typeof maybeCSS.escape === 'function') {
            return maybeCSS.escape(raw);
        }
        // Minimal fallback for test/non-browser environments where CSS.escape is unavailable.
        return raw.replace(/["\\]/g, '\\$&');
    }
    function buildUniqueSelector(el, htmlEl, fallbackSelector) {
        if (el.id)
            return `#${cssEscape(el.id)}`;
        if (el instanceof HTMLInputElement && el.name)
            return `input[name="${cssEscape(el.name)}"]`;
        const ariaLabel = el.getAttribute('aria-label');
        // Use CSS attribute selectors — these resolve via querySelectorAll directly,
        // avoiding semantic resolver ordering mismatches (#360).
        if (ariaLabel)
            return `[aria-label="${cssEscape(ariaLabel)}"]`;
        const placeholder = el.getAttribute('placeholder');
        if (placeholder)
            return `[placeholder="${cssEscape(placeholder)}"]`;
        const text = (htmlEl.textContent || '').trim().slice(0, 40);
        if (text)
            return `text=${text}`;
        return fallbackSelector;
    }
    // Build >>> selector for an element inside a shadow root
    function buildShadowSelector(el) {
        const rootNode = el.getRootNode();
        if (!(rootNode instanceof ShadowRoot))
            return null;
        const parts = [];
        let node = el;
        let root = rootNode;
        while (root instanceof ShadowRoot) {
            const inner = buildUniqueSelector(node, node, node.tagName.toLowerCase());
            parts.unshift(inner);
            node = root.host;
            root = node.getRootNode();
        }
        // Add the outermost host selector
        const hostSelector = buildUniqueSelector(node, node, node.tagName.toLowerCase());
        parts.unshift(hostSelector);
        return parts.join(' >>> ');
    }
    function classifyElement(el) {
        const tag = el.tagName.toLowerCase();
        if (tag === 'a')
            return 'link';
        if (tag === 'button' || el.getAttribute('role') === 'button')
            return 'button';
        if (tag === 'input') {
            const inputType = el.type || 'text';
            if (inputType === 'submit' || inputType === 'button' || inputType === 'reset')
                return 'button';
            if (inputType === 'checkbox' || inputType === 'radio')
                return 'checkbox';
            return 'input';
        }
        if (tag === 'select')
            return 'select';
        if (tag === 'textarea')
            return 'textarea';
        if (el.getAttribute('role') === 'link')
            return 'link';
        if (el.getAttribute('role') === 'tab')
            return 'tab';
        if (el.getAttribute('role') === 'menuitem')
            return 'menuitem';
        if (el.getAttribute('contenteditable') === 'true')
            return 'textarea';
        return 'interactive';
    }
    function isVisibleElement(el) {
        const htmlEl = el;
        if (!htmlEl || typeof htmlEl.getBoundingClientRect !== 'function')
            return true;
        const rect = htmlEl.getBoundingClientRect();
        return rect.width > 0 && rect.height > 0 && htmlEl.offsetParent !== null;
    }
    // #368: Detect if an element is inside an overlay (position:fixed/sticky with high z-index)
    function isInsideOverlay(el) {
        let node = el;
        while (node && node !== document.documentElement) {
            if (node instanceof HTMLElement) {
                if (typeof getComputedStyle !== 'function')
                    return false;
                const style = getComputedStyle(node);
                const position = style.position || '';
                if (position === 'fixed' || position === 'sticky') {
                    const zIndex = Number.parseInt(style.zIndex || '', 10);
                    if (zIndex >= 100)
                        return true;
                    // Common overlay indicators
                    const role = node.getAttribute('role') || '';
                    if (role === 'dialog' || role === 'alertdialog' || node.getAttribute('aria-modal') === 'true')
                        return true;
                }
            }
            node = node.parentElement;
        }
        return false;
    }
    const LANDMARK_TAGS = new Set(['nav', 'header', 'footer', 'aside', 'main']);
    const LANDMARK_ROLES = new Set(['navigation', 'banner', 'contentinfo', 'complementary', 'main']);
    function findNearestLandmark(el) {
        let node = el.parentElement;
        while (node && node !== document.documentElement) {
            const tag = node.tagName.toLowerCase();
            const role = node.getAttribute('role') || undefined;
            if (LANDMARK_TAGS.has(tag) || (role && LANDMARK_ROLES.has(role))) {
                return { tag, role };
            }
            node = node.parentElement;
        }
        return undefined;
    }
    function extractBoundingBox(el) {
        const htmlEl = el;
        if (!htmlEl || typeof htmlEl.getBoundingClientRect !== 'function') {
            return { x: 0, y: 0, width: 0, height: 0 };
        }
        const rect = htmlEl.getBoundingClientRect();
        const x = typeof rect.left === 'number' ? rect.left : typeof rect.x === 'number' ? rect.x : 0;
        const y = typeof rect.top === 'number' ? rect.top : typeof rect.y === 'number' ? rect.y : 0;
        const width = Number.isFinite(rect.width) ? rect.width : 0;
        const height = Number.isFinite(rect.height) ? rect.height : 0;
        return { x, y, width, height };
    }
    function extractLabel(el) {
        const htmlEl = el;
        return (el.getAttribute('aria-label') ||
            el.getAttribute('title') ||
            el.getAttribute('placeholder') ||
            (htmlEl?.textContent || '').trim().slice(0, 80) ||
            el.tagName.toLowerCase());
    }
    function parseScopeRect(raw) {
        if (!raw || typeof raw !== 'object')
            return null;
        const rect = raw;
        const x = Number(rect.x);
        const y = Number(rect.y);
        const width = Number(rect.width);
        const height = Number(rect.height);
        if (![x, y, width, height].every((v) => Number.isFinite(v)))
            return null;
        if (width <= 0 || height <= 0)
            return null;
        return { x, y, width, height };
    }
    const scopeRect = parseScopeRect(options?.scope_rect);
    if (options?.scope_rect !== undefined && !scopeRect) {
        return {
            success: false,
            elements: [],
            error: 'invalid_scope_rect',
            message: 'scope_rect must include finite x, y, width, and height > 0'
        };
    }
    // #369: Extract filter options
    const textContains = (options?.text_contains || '').toLowerCase();
    const roleFilter = (options?.role || '').toLowerCase();
    const visibleOnly = options?.visible_only === true;
    const excludeNav = options?.exclude_nav === true;
    function intersectsScopeRect(el) {
        if (!scopeRect)
            return true;
        const htmlEl = el;
        if (!htmlEl || typeof htmlEl.getBoundingClientRect !== 'function')
            return false;
        const rect = htmlEl.getBoundingClientRect();
        const left = typeof rect.left === 'number' ? rect.left : typeof rect.x === 'number' ? rect.x : 0;
        const top = typeof rect.top === 'number' ? rect.top : typeof rect.y === 'number' ? rect.y : 0;
        const right = typeof rect.right === 'number' ? rect.right : left + rect.width;
        const bottom = typeof rect.bottom === 'number' ? rect.bottom : top + rect.height;
        const scopeRight = scopeRect.x + scopeRect.width;
        const scopeBottom = scopeRect.y + scopeRect.height;
        const overlapX = left < scopeRight && right > scopeRect.x;
        const overlapY = top < scopeBottom && bottom > scopeRect.y;
        return overlapX && overlapY;
    }
    function chooseBestScopeMatch(matches) {
        if (matches.length === 1)
            return matches[0];
        const submitVerb = /(post|share|publish|send|submit|save|done|continue|next|create|apply)/i;
        let best = matches[0];
        let bestScore = -1;
        for (const candidate of matches) {
            const textboxes = querySelectorAllDeep('[role="textbox"], textarea, [contenteditable="true"]', candidate);
            const visibleTextboxes = textboxes.filter(isVisibleElement).length;
            const buttonCandidates = querySelectorAllDeep('button, [role="button"], input[type="submit"]', candidate);
            let visibleButtons = 0;
            let submitLikeButtons = 0;
            for (const btn of buttonCandidates) {
                if (!isVisibleElement(btn))
                    continue;
                visibleButtons++;
                if (submitVerb.test(extractLabel(btn))) {
                    submitLikeButtons++;
                }
            }
            const interactiveCandidates = querySelectorAllDeep('a[href], button, input, select, textarea, [role="button"], [role="link"], [role="tab"], [role="menuitem"], [contenteditable="true"]', candidate);
            const visibleInteractive = interactiveCandidates.filter(isVisibleElement).length;
            const hiddenInteractive = Math.max(0, interactiveCandidates.length - visibleInteractive);
            const rect = candidate.getBoundingClientRect?.();
            const areaScore = rect && rect.width > 0 && rect.height > 0 ? Math.min(20, Math.round((rect.width * rect.height) / 50000)) : 0;
            // Heuristic weighting:
            // - Visible textbox strongly indicates active editor/dialog.
            // - Submit-like visible controls indicate actionable composer.
            // - Prefer visible-rich over hidden-heavy containers.
            const score = visibleTextboxes * 1000 +
                submitLikeButtons * 250 +
                visibleButtons * 10 +
                visibleInteractive -
                hiddenInteractive +
                areaScore;
            if (score > bestScore) {
                bestScore = score;
                best = candidate;
            }
        }
        return best;
    }
    function resolveScopeRoot(rawScope) {
        const scope = (rawScope || '').trim();
        if (!scope)
            return document;
        try {
            const matches = querySelectorAllDeep(scope);
            if (matches.length === 0)
                return null;
            return chooseBestScopeMatch(matches);
        }
        catch {
            return null;
        }
    }
    // — Main scan logic —
    const interactiveSelectors = [
        'a[href]',
        'button',
        'input',
        'select',
        'textarea',
        '[role="button"]',
        '[role="link"]',
        '[role="tab"]',
        '[role="menuitem"]',
        '[contenteditable="true"]',
        '[onclick]',
        '[tabindex]'
    ];
    const seen = new Set();
    const elements = [];
    // First pass: collect raw entries with their base selectors
    const rawEntries = [];
    const scopeRoot = resolveScopeRoot(scopeSelector);
    if (!scopeRoot) {
        return {
            success: false,
            elements: [],
            error: 'scope_not_found',
            message: `No scope element matches selector: ${scopeSelector || ''}`
        };
    }
    for (const cssSelector of interactiveSelectors) {
        const matches = querySelectorAllDeep(cssSelector, scopeRoot);
        for (const el of matches) {
            if (seen.has(el))
                continue;
            seen.add(el);
            const htmlEl = el;
            const rect = htmlEl.getBoundingClientRect();
            const visible = rect.width > 0 && rect.height > 0 && htmlEl.offsetParent !== null;
            if (!intersectsScopeRect(el))
                continue;
            // #369: Apply filters early to maximize useful elements within the 100-cap
            if (visibleOnly && !visible)
                continue;
            if (excludeNav) {
                const lm = findNearestLandmark(el);
                if (lm && (lm.tag === 'nav' || lm.tag === 'header' || lm.role === 'navigation' || lm.role === 'banner'))
                    continue;
            }
            const bbox = extractBoundingBox(el);
            // Use >>> selector for shadow DOM elements, regular selector otherwise
            const shadowSel = buildShadowSelector(el);
            const baseSelector = shadowSel || buildUniqueSelector(el, htmlEl, cssSelector);
            // Build human-readable label
            const label = el.getAttribute('aria-label') ||
                el.getAttribute('title') ||
                el.getAttribute('placeholder') ||
                (htmlEl.textContent || '').trim().slice(0, 60) ||
                el.tagName.toLowerCase();
            // #369: Apply text and role filters after label/type are computed
            if (textContains && !label.toLowerCase().includes(textContains))
                continue;
            const elementType = classifyElement(el);
            const ariaRole = el.getAttribute('role') || '';
            if (roleFilter && elementType !== roleFilter && ariaRole.toLowerCase() !== roleFilter)
                continue;
            const landmark = findNearestLandmark(el);
            rawEntries.push({
                el,
                htmlEl,
                baseSelector,
                finalSelector: baseSelector, // will be updated with :nth-match before sort
                tag: el.tagName.toLowerCase(),
                inputType: el instanceof HTMLInputElement ? el.type : undefined,
                elementType,
                label,
                role: ariaRole || undefined,
                placeholder: el.getAttribute('placeholder') || undefined,
                bbox,
                visible,
                inOverlay: isInsideOverlay(el),
                landmarkTag: landmark?.tag,
                landmarkRole: landmark?.role
            });
            if (rawEntries.length >= 100)
                break;
        }
        if (rawEntries.length >= 100)
            break;
    }
    // Second pass: find cursor:pointer elements not caught by selector scan.
    // Catches framework-bound click handlers (React onClick, Vue @click, etc.)
    // that render as plain divs/spans with no semantic interactive attributes.
    if (rawEntries.length < 100) {
        const cursorPointerTags = new Set(['div', 'span', 'li', 'td', 'p', 'img', 'svg', 'label', 'figure', 'section', 'article']);
        const candidates = scopeRoot.querySelectorAll('*');
        let checked = 0;
        const maxCheck = 500; // Budget: don't scan more than 500 elements for cursor style
        for (const el of candidates) {
            if (rawEntries.length >= 100)
                break;
            if (checked >= maxCheck)
                break;
            if (seen.has(el))
                continue;
            const tag = el.tagName.toLowerCase();
            if (!cursorPointerTags.has(tag))
                continue;
            checked++;
            const htmlEl = el;
            if (!htmlEl.offsetParent && htmlEl !== document.body)
                continue;
            let cursor;
            try {
                cursor = getComputedStyle(htmlEl).cursor;
            }
            catch {
                continue;
            }
            if (cursor !== 'pointer')
                continue;
            // Confirmed: cursor:pointer on a non-interactive element
            seen.add(el);
            const rect = htmlEl.getBoundingClientRect();
            const visible = rect.width > 0 && rect.height > 0;
            if (visibleOnly && !visible)
                continue;
            if (!intersectsScopeRect(el))
                continue;
            const bbox = extractBoundingBox(el);
            const shadowSel = buildShadowSelector(el);
            const baseSelector = shadowSel || buildUniqueSelector(el, htmlEl, '*');
            const label = el.getAttribute('aria-label') ||
                el.getAttribute('title') ||
                (htmlEl.textContent || '').trim().slice(0, 60) ||
                tag;
            if (textContains && !label.toLowerCase().includes(textContains))
                continue;
            const ariaRole = el.getAttribute('role') || '';
            if (roleFilter && ariaRole.toLowerCase() !== roleFilter)
                continue;
            if (excludeNav) {
                const lm = findNearestLandmark(el);
                if (lm && (lm.tag === 'nav' || lm.tag === 'header' || lm.role === 'navigation' || lm.role === 'banner'))
                    continue;
            }
            const landmark = findNearestLandmark(el);
            rawEntries.push({
                el,
                htmlEl,
                baseSelector,
                finalSelector: baseSelector,
                tag,
                inputType: undefined,
                elementType: 'clickable',
                label,
                role: ariaRole || undefined,
                placeholder: undefined,
                bbox,
                visible,
                inOverlay: isInsideOverlay(el),
                landmarkTag: landmark?.tag,
                landmarkRole: landmark?.role
            });
        }
    }
    // Disambiguate selectors in DOM order BEFORE dedup and spatial sort.
    // The resolver (resolveByTextAll, querySelectorAllDeep) returns elements in DOM order,
    // so :nth-match(N) numbering must match DOM order, not spatial order (#360).
    // IMPORTANT: assign :nth-match on the FULL pre-dedup set so indices match the resolver (#366).
    const selectorCount = new Map();
    for (const entry of rawEntries) {
        selectorCount.set(entry.baseSelector, (selectorCount.get(entry.baseSelector) || 0) + 1);
    }
    const selectorIndex = new Map();
    for (const entry of rawEntries) {
        const count = selectorCount.get(entry.baseSelector) || 1;
        if (count > 1) {
            const nth = (selectorIndex.get(entry.baseSelector) || 0) + 1;
            selectorIndex.set(entry.baseSelector, nth);
            entry.finalSelector = `${entry.baseSelector}:nth-match(${nth})`;
        }
    }
    // #366: Deduplicate responsive variants — when hidden and visible copies of the same
    // element exist (e.g., mobile + desktop nav links), keep only the visible one.
    // Use a coarse semantic key (tag + type + normalized label) so hidden responsive
    // variants with different hrefs still collapse into the visible copy.
    const normalizeDedupLabel = (label) => label.trim().replace(/\s+/g, ' ').toLowerCase();
    const dedupKey = (e) => {
        return `${e.tag}|${e.elementType}|${normalizeDedupLabel(e.label)}`;
    };
    const dedupGroups = new Map();
    for (const entry of rawEntries) {
        const key = dedupKey(entry);
        const group = dedupGroups.get(key);
        if (group) {
            group.push(entry);
        }
        else {
            dedupGroups.set(key, [entry]);
        }
    }
    const finalEntries = [];
    for (const group of dedupGroups.values()) {
        if (group.length > 1) {
            const visible = group.filter((e) => e.visible);
            const hidden = group.filter((e) => !e.visible);
            if (visible.length > 0 && hidden.length > 0) {
                // Keep only visible copies when both visible and hidden exist
                finalEntries.push(...visible);
            }
            else {
                // All same visibility — keep all
                finalEntries.push(...group);
            }
        }
        else {
            finalEntries.push(group[0]);
        }
    }
    // #448: When scope_rect is provided, compute distance from center and sort by proximity.
    // Otherwise, sort spatially in reading order (top-to-bottom, left-to-right).
    if (scopeRect) {
        const centerX = scopeRect.x + scopeRect.width / 2;
        const centerY = scopeRect.y + scopeRect.height / 2;
        for (const entry of finalEntries) {
            const elCenterX = entry.bbox.x + entry.bbox.width / 2;
            const elCenterY = entry.bbox.y + entry.bbox.height / 2;
            const dx = elCenterX - centerX;
            const dy = elCenterY - centerY;
            entry.distance_px = Math.round(Math.sqrt(dx * dx + dy * dy));
        }
        finalEntries.sort((a, b) => {
            if (a.visible && !b.visible)
                return -1;
            if (!a.visible && b.visible)
                return 1;
            return (a.distance_px || 0) - (b.distance_px || 0);
        });
    }
    else {
        // Default: spatial reading order
        const ROW_THRESHOLD = 10;
        finalEntries.sort((a, b) => {
            if (a.visible && !b.visible)
                return -1;
            if (!a.visible && b.visible)
                return 1;
            if (!a.visible && !b.visible)
                return 0;
            const sameRow = Math.abs(a.bbox.y - b.bbox.y) <= ROW_THRESHOLD;
            if (sameRow)
                return a.bbox.x - b.bbox.x;
            return a.bbox.y - b.bbox.y;
        });
    }
    for (let i = 0; i < finalEntries.length; i++) {
        const entry = finalEntries[i];
        const distPx = entry.distance_px;
        elements.push({
            index: i,
            tag: entry.tag,
            type: entry.inputType,
            element_type: entry.elementType,
            selector: entry.finalSelector,
            element_id: getOrCreateElementID(entry.el, entry.finalSelector),
            label: entry.label,
            role: entry.role,
            placeholder: entry.placeholder,
            bbox: entry.bbox,
            visible: entry.visible,
            ...(distPx !== undefined ? { distance_px: distPx } : {}),
            ...(entry.inOverlay ? { in_overlay: true } : {}),
            ...(entry.landmarkTag ? { landmark_tag: entry.landmarkTag } : {}),
            ...(entry.landmarkRole ? { landmark_role: entry.landmarkRole } : {})
        });
    }
    // #369: Build filter metadata for the response
    const filters = {};
    if (textContains)
        filters.text_contains = options.text_contains;
    if (roleFilter)
        filters.role = options.role;
    if (visibleOnly)
        filters.visible_only = true;
    if (excludeNav)
        filters.exclude_nav = true;
    return {
        success: true,
        elements,
        candidate_count: elements.length,
        ...(scopeRect ? { scope_rect_used: scopeRect } : {}),
        ...(Object.keys(filters).length > 0 ? { filters_applied: filters } : {})
    };
}
//# sourceMappingURL=dom-primitives-list-interactive.js.map