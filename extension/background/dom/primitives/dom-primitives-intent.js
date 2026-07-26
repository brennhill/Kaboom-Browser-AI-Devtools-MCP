/**
 * Purpose: Self-contained DOM primitives for intent-based actions (open_composer, submit_active_composer, confirm_top_dialog).
 * Why: Extracted from dom-primitives.ts to keep file sizes under the 800 LOC limit (#502).
 *      These actions use heuristic scoring to find the best target element for high-level intent.
 * Docs: docs/features/feature/interact-explore/index.md
 */
// dom-primitives-intent.ts — Self-contained intent DOM primitives for chrome.scripting.executeScript.
// This function MUST remain self-contained — Chrome serializes the function source only (no closures).
/**
 * Self-contained function that resolves intent-based targets (composer triggers, submit buttons, dialog confirms).
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveIntent }).
 * MUST NOT reference any module-level variables.
 */
export function domPrimitiveIntent(action, options) {
    // — Shared helpers (duplicated for self-containment) —
    function isKaboomOwnedElement(element) {
        let node = element;
        while (node) {
            const id = node.id || '';
            if (id.startsWith('kaboom-'))
                return true;
            const className = node.className;
            if (typeof className === 'string' && className.includes('kaboom-'))
                return true;
            if (node.getAttribute && node.getAttribute('data-kaboom-owned') === 'true')
                return true;
            node = node.parentElement;
        }
        return false;
    }
    function getShadowRoot(el) {
        return el.shadowRoot ?? null;
    }
    function querySelectorAllDeep(selector, root = document, results = [], depth = 0) {
        if (depth > 10)
            return results;
        const matches = Array.from(root.querySelectorAll(selector));
        for (const match of matches) {
            if (!isKaboomOwnedElement(match))
                results.push(match);
        }
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
    function querySelectorDeep(selector, root = document) {
        const fast = root.querySelector(selector);
        if (fast && !isKaboomOwnedElement(fast))
            return fast;
        return querySelectorDeepWalk(selector, root);
    }
    function querySelectorDeepWalk(selector, root, depth = 0) {
        if (depth > 10)
            return null;
        const children = 'children' in root
            ? root.children
            : root.body?.children || root.documentElement?.children;
        if (!children)
            return null;
        for (let i = 0; i < children.length; i++) {
            const child = children[i];
            const shadow = getShadowRoot(child);
            if (shadow) {
                const match = shadow.querySelector(selector);
                if (match && !isKaboomOwnedElement(match))
                    return match;
                const deep = querySelectorDeepWalk(selector, shadow, depth + 1);
                if (deep)
                    return deep;
            }
            if (child.children.length > 0) {
                const deep = querySelectorDeepWalk(selector, child, depth + 1);
                if (deep)
                    return deep;
            }
        }
        return null;
    }
    function firstVisible(els) {
        let fallback = null;
        for (const el of els) {
            if (!fallback)
                fallback = el;
            if (isVisible(el))
                return el;
        }
        return fallback;
    }
    function isVisible(el) {
        if (isKaboomOwnedElement(el))
            return false;
        if (!(el instanceof HTMLElement))
            return true;
        const style = getComputedStyle(el);
        if (style.visibility === 'hidden' || style.display === 'none')
            return false;
        if (el.offsetParent === null && style.position !== 'fixed' && style.position !== 'sticky') {
            const rect = el.getBoundingClientRect();
            if (rect.width === 0 && rect.height === 0)
                return false;
        }
        return true;
    }
    function isActionableVisible(el) {
        if (!(el instanceof HTMLElement))
            return true;
        const rect = typeof el.getBoundingClientRect === 'function'
            ? el.getBoundingClientRect()
            : { width: 0, height: 0 };
        if (!(rect.width > 0 && rect.height > 0))
            return false;
        if (el.offsetParent === null) {
            const style = typeof getComputedStyle === 'function' ? getComputedStyle(el) : null;
            const position = style?.position || '';
            if (position !== 'fixed' && position !== 'sticky')
                return false;
        }
        return true;
    }
    function isVisibleElement(el) {
        const htmlEl = el;
        if (!htmlEl || typeof htmlEl.getBoundingClientRect !== 'function')
            return true;
        const rect = htmlEl.getBoundingClientRect();
        return rect.width > 0 && rect.height > 0 && htmlEl.offsetParent !== null;
    }
    function uniqueElements(elements) {
        const out = [];
        const seen = new Set();
        for (const element of elements) {
            if (seen.has(element))
                continue;
            seen.add(element);
            out.push(element);
        }
        return out;
    }
    function elementZIndexScore(el) {
        if (!(el instanceof HTMLElement))
            return 0;
        const style = getComputedStyle(el);
        const raw = style.zIndex || '';
        const parsed = Number.parseInt(raw, 10);
        if (Number.isNaN(parsed))
            return 0;
        return parsed;
    }
    function areaScore(el, max) {
        if (!(el instanceof HTMLElement) || typeof el.getBoundingClientRect !== 'function')
            return 0;
        const rect = el.getBoundingClientRect();
        if (rect.width <= 0 || rect.height <= 0)
            return 0;
        return Math.min(max, Math.round((rect.width * rect.height) / 10000));
    }
    function extractElementLabel(el) {
        const htmlEl = el;
        return (el.getAttribute('aria-label') ||
            el.getAttribute('title') ||
            el.getAttribute('placeholder') ||
            (htmlEl?.textContent || '').trim().slice(0, 80) ||
            el.tagName.toLowerCase());
    }
    function getElementHandleStore() {
        const root = globalThis;
        if (root.__kaboomElementHandles) {
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
    function getOrCreateElementID(el) {
        const store = getElementHandleStore();
        const existing = store.byElement.get(el);
        if (existing) {
            store.byID.set(existing, el);
            return existing;
        }
        const elementID = `el_${(store.nextID++).toString(36)}`;
        store.byElement.set(el, elementID);
        store.byID.set(elementID, el);
        return elementID;
    }
    function extractBoundingBox(el) {
        if (!(el instanceof HTMLElement) || typeof el.getBoundingClientRect !== 'function') {
            return { x: 0, y: 0, width: 0, height: 0 };
        }
        const rect = el.getBoundingClientRect();
        const x = typeof rect.left === 'number' ? rect.left : (typeof rect.x === 'number' ? rect.x : 0);
        const y = typeof rect.top === 'number' ? rect.top : (typeof rect.y === 'number' ? rect.y : 0);
        const width = Number.isFinite(rect.width) ? rect.width : 0;
        const height = Number.isFinite(rect.height) ? rect.height : 0;
        return { x, y, width, height };
    }
    function captureViewport() {
        const w = typeof window !== 'undefined' ? window : null;
        const docEl = document?.documentElement;
        const body = document?.body;
        return {
            scroll_x: Math.round((w?.scrollX ?? w?.pageXOffset ?? 0)),
            scroll_y: Math.round((w?.scrollY ?? w?.pageYOffset ?? 0)),
            viewport_width: w?.innerWidth ?? docEl?.clientWidth ?? 0,
            viewport_height: w?.innerHeight ?? docEl?.clientHeight ?? 0,
            page_height: Math.max(body?.scrollHeight || 0, docEl?.scrollHeight || 0)
        };
    }
    // — Scope resolution —
    function resolveScopeRoot(rawScope) {
        const scope = (rawScope || '').trim();
        if (!scope)
            return document;
        try {
            const matches = querySelectorAllDeep(scope);
            if (matches.length === 0)
                return null;
            return firstVisible(matches) || matches[0] || null;
        }
        catch {
            return null;
        }
    }
    // — Dialog/overlay helpers —
    function collectDialogs() {
        const selectors = ['[role="dialog"]', '[aria-modal="true"]', 'dialog[open]'];
        const dialogs = [];
        for (const dialogSelector of selectors) {
            dialogs.push(...querySelectorAllDeep(dialogSelector));
        }
        return uniqueElements(dialogs).filter(isActionableVisible);
    }
    function pickTopDialog(dialogs) {
        if (dialogs.length === 0)
            return null;
        const ranked = dialogs
            .map((dialog, index) => ({
            element: dialog,
            score: elementZIndexScore(dialog) * 1000 + areaScore(dialog, 200) + index
        }))
            .sort((a, b) => b.score - a.score);
        return ranked[0]?.element || null;
    }
    function chooseBestScopeMatch(matches) {
        if (matches.length === 1)
            return matches[0];
        const submitVerbLocal = /(post|share|publish|send|submit|save|done|continue|next|create|apply)/i;
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
                if (submitVerbLocal.test(extractElementLabel(btn)))
                    submitLikeButtons++;
            }
            const interactiveCandidates = querySelectorAllDeep('a[href], button, input, select, textarea, [role="button"], [role="link"], [role="tab"], [role="menuitem"], [contenteditable="true"]', candidate);
            const visibleInteractive = interactiveCandidates.filter(isVisibleElement).length;
            const hiddenInteractive = Math.max(0, interactiveCandidates.length - visibleInteractive);
            const rect = candidate.getBoundingClientRect?.();
            const areaScoreVal = rect && rect.width > 0 && rect.height > 0
                ? Math.min(20, Math.round((rect.width * rect.height) / 50000))
                : 0;
            const score = visibleTextboxes * 1000 + submitLikeButtons * 250 + visibleButtons * 10 + visibleInteractive - hiddenInteractive + areaScoreVal;
            if (score > bestScore) {
                bestScore = score;
                best = candidate;
            }
        }
        return best;
    }
    // — Error/result helpers —
    const submitVerb = /(post|share|publish|send|submit|save|done|continue|next|create|apply|confirm|yes|allow|accept)/i;
    const dismissVerb = /(close|dismiss|cancel|not now|no thanks|skip|x|×|hide|back)/i;
    const composerVerb = /(start( a)? post|create post|write (a )?post|what'?s on your mind|share( an)? update|compose|new post)/i;
    function domError(error, message) {
        return { success: false, action, selector: '', error, message };
    }
    function summarizeCandidates(candidates) {
        return candidates.slice(0, 8).map((candidate) => {
            const htmlEl = candidate;
            const fallback = candidate.tagName.toLowerCase();
            return {
                tag: fallback,
                role: candidate.getAttribute('role') || undefined,
                aria_label: candidate.getAttribute('aria-label') || undefined,
                text_preview: (htmlEl.textContent || '').trim().slice(0, 80) || undefined,
                selector: '',
                element_id: getOrCreateElementID(candidate),
                bbox: extractBoundingBox(candidate),
                visible: isActionableVisible(candidate)
            };
        });
    }
    function pickBestIntentTarget(ranked, matchStrategy, notFoundError, notFoundMessage) {
        const viable = ranked
            .filter((entry) => entry.score > 0 && isActionableVisible(entry.element))
            .sort((a, b) => b.score - a.score);
        if (viable.length === 0) {
            return { error: domError(notFoundError, notFoundMessage) };
        }
        const topScore = viable[0].score;
        const tiedTop = viable.filter((entry) => entry.score === topScore);
        if (tiedTop.length > 1) {
            return {
                error: {
                    ...domError('ambiguous_target', `Multiple candidates tie for ${action}. Use scope_selector to narrow down.`),
                    match_count: tiedTop.length,
                    match_strategy: matchStrategy,
                    candidates: summarizeCandidates(tiedTop.map((entry) => entry.element))
                }
            };
        }
        return { element: viable[0].element, match_count: 1, match_strategy: matchStrategy };
    }
    // — Resolve scope —
    const requestedScope = (options?.scope_selector || '').trim();
    const scopeRoot = resolveScopeRoot(requestedScope);
    if (requestedScope && !scopeRoot) {
        return domError('scope_not_found', `No scope element matches selector: ${requestedScope}`);
    }
    const activeScope = scopeRoot || document;
    function resolveIntentTarget() {
        if (action === 'open_composer') {
            const selectors = [
                'button', '[role="button"]', 'a[href]', '[role="link"]',
                '[contenteditable="true"]', '[role="textbox"]', 'textarea',
                'input[type="text"]', 'input:not([type])'
            ];
            const candidates = [];
            for (const candidateSelector of selectors) {
                candidates.push(...querySelectorAllDeep(candidateSelector, activeScope));
            }
            const ranked = uniqueElements(candidates).map((candidate) => {
                const label = extractElementLabel(candidate).toLowerCase();
                const tag = candidate.tagName.toLowerCase();
                const role = candidate.getAttribute('role') || '';
                const contentEditable = candidate.getAttribute('contenteditable') === 'true';
                let score = 0;
                if (composerVerb.test(label))
                    score += 700;
                if (/\b(post|share|publish|compose|write|update)\b/i.test(label))
                    score += 280;
                if (contentEditable || role === 'textbox' || tag === 'textarea' || tag === 'input')
                    score += 220;
                if (tag === 'button' || role === 'button')
                    score += 80;
                score += areaScore(candidate, 50);
                score += elementZIndexScore(candidate);
                return { element: candidate, score };
            });
            const best = pickBestIntentTarget(ranked, 'intent_open_composer', 'composer_not_found', 'No composer trigger was found. Try a tighter scope_selector.');
            return { ...best, scope_selector_used: requestedScope || undefined };
        }
        if (action === 'submit_active_composer') {
            let resolvedScope = activeScope;
            let scopeUsed = requestedScope || undefined;
            if (!requestedScope) {
                const dialogs = collectDialogs();
                const rankedDialogs = dialogs.map((dialog) => {
                    const textboxes = querySelectorAllDeep('[role="textbox"], textarea, [contenteditable="true"]', dialog).filter(isActionableVisible).length;
                    const buttons = querySelectorAllDeep('button, [role="button"], input[type="submit"]', dialog);
                    const submitLikeButtons = buttons.filter((button) => isActionableVisible(button) && submitVerb.test(extractElementLabel(button))).length;
                    return {
                        element: dialog,
                        score: textboxes * 1200 + submitLikeButtons * 300 + elementZIndexScore(dialog) * 2 + areaScore(dialog, 80)
                    };
                }).sort((a, b) => b.score - a.score);
                if ((rankedDialogs[0]?.score || 0) > 0) {
                    resolvedScope = rankedDialogs[0].element;
                    scopeUsed = 'intent:auto_composer_scope';
                }
            }
            const candidates = querySelectorAllDeep('button, [role="button"], input[type="submit"]', resolvedScope);
            const ranked = uniqueElements(candidates).map((candidate) => {
                const label = extractElementLabel(candidate);
                let score = 0;
                if (submitVerb.test(label))
                    score += 700;
                if (dismissVerb.test(label))
                    score -= 500;
                score += areaScore(candidate, 30);
                score += elementZIndexScore(candidate);
                return { element: candidate, score };
            });
            const best = pickBestIntentTarget(ranked, 'intent_submit_active_composer', 'composer_submit_not_found', 'No submit control found in active composer scope.');
            return { ...best, scope_selector_used: scopeUsed };
        }
        if (action === 'confirm_top_dialog') {
            const dialogScope = requestedScope ? activeScope : pickTopDialog(collectDialogs());
            if (!dialogScope) {
                return { error: domError('dialog_not_found', 'No visible dialog/overlay found to confirm.') };
            }
            const candidates = querySelectorAllDeep('button, [role="button"], input[type="submit"]', dialogScope);
            const ranked = uniqueElements(candidates).map((candidate) => {
                const label = extractElementLabel(candidate);
                let score = 0;
                if (submitVerb.test(label))
                    score += 700;
                if (dismissVerb.test(label))
                    score -= 500;
                score += areaScore(candidate, 30);
                score += elementZIndexScore(candidate);
                return { element: candidate, score };
            });
            const best = pickBestIntentTarget(ranked, 'intent_confirm_top_dialog', 'confirm_action_not_found', 'No confirm control found in the top dialog.');
            return { ...best, scope_selector_used: requestedScope || 'intent:auto_top_dialog' };
        }
        return { error: domError('unknown_action', `Unknown intent action: ${action}`) };
    }
    // — Execute action —
    const resolved = resolveIntentTarget();
    if (resolved.error)
        return resolved.error;
    const node = resolved.element;
    if (!(node instanceof HTMLElement)) {
        return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`);
    }
    const matchedInfo = {
        tag: node.tagName.toLowerCase(),
        role: node.getAttribute('role') || undefined,
        aria_label: node.getAttribute('aria-label') || undefined,
        text_preview: (node.textContent || '').trim().slice(0, 80) || undefined,
        selector: '',
        element_id: getOrCreateElementID(node),
        bbox: extractBoundingBox(node),
        scope_selector_used: resolved.scope_selector_used
    };
    // Execute the action
    if (action === 'open_composer') {
        const tag = node.tagName.toLowerCase();
        const isInputLike = node.isContentEditable ||
            node.getAttribute('role') === 'textbox' ||
            tag === 'textarea' ||
            tag === 'input';
        if (isInputLike) {
            node.focus();
            return {
                success: true, action, selector: '', reason: 'composer_ready',
                matched: matchedInfo, match_count: resolved.match_count || 1,
                match_strategy: resolved.match_strategy || 'intent_open_composer',
                viewport: captureViewport()
            };
        }
        node.click();
    }
    else {
        // submit_active_composer and confirm_top_dialog both click
        node.click();
    }
    return {
        success: true, action, selector: '',
        matched: matchedInfo, match_count: resolved.match_count || 1,
        match_strategy: resolved.match_strategy || 'selector',
        viewport: captureViewport()
    };
}
//# sourceMappingURL=dom-primitives-intent.js.map