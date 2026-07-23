/**
 * Purpose: Self-contained DOM primitives for wait_for_stable and action_diff actions.
 * Why: Extracted from dom-primitives.ts to keep file sizes under the 800 LOC limit (#502).
 *      These actions need no selector infrastructure — they observe DOM mutations and classify changes.
 * Docs: docs/features/feature/interact-explore/index.md
 */
// dom-primitives-stability.ts — Self-contained stability/diff DOM primitives for chrome.scripting.executeScript.
// These functions MUST remain self-contained — Chrome serializes the function source only (no closures).
/**
 * Self-contained function that waits for the DOM to become stable (no mutations for a configurable period).
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveWaitForStable }).
 * MUST NOT reference any module-level variables.
 */
export function domPrimitiveWaitForStable(options) {
    const stabilityMs = typeof options?.stability_ms === 'number' && options.stability_ms > 0
        ? options.stability_ms : 500;
    const maxTimeout = typeof options?.timeout_ms === 'number' && options.timeout_ms > 0
        ? options.timeout_ms : 5000;
    return new Promise((resolve) => {
        let mutationCount = 0;
        let lastMutationTime = performance.now();
        const startTime = performance.now();
        const observer = new MutationObserver(() => {
            mutationCount++;
            lastMutationTime = performance.now();
        });
        observer.observe(document.body || document.documentElement, {
            childList: true,
            subtree: true,
            attributes: true,
            characterData: true
        });
        function checkStability() {
            const elapsed = performance.now() - startTime;
            const sinceLastMutation = performance.now() - lastMutationTime;
            if (sinceLastMutation >= stabilityMs) {
                observer.disconnect();
                resolve({
                    success: true,
                    action: 'wait_for_stable',
                    selector: '',
                    stable: true,
                    waited_ms: Math.round(elapsed),
                    mutations_observed: mutationCount,
                    stability_ms: stabilityMs
                });
                return;
            }
            if (elapsed >= maxTimeout) {
                observer.disconnect();
                resolve({
                    success: true,
                    action: 'wait_for_stable',
                    selector: '',
                    stable: false,
                    timed_out: true,
                    waited_ms: Math.round(elapsed),
                    mutations_observed: mutationCount,
                    stability_ms: stabilityMs
                });
                return;
            }
            setTimeout(checkStability, Math.min(100, stabilityMs / 2));
        }
        setTimeout(checkStability, Math.min(100, stabilityMs / 2));
    });
}
/**
 * Self-contained function that instruments a MutationObserver, waits for DOM to settle,
 * then classifies mutations into categories (overlays, toasts, form errors, etc.).
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveActionDiff }).
 * MUST NOT reference any module-level variables.
 */
export function domPrimitiveActionDiff(options) {
    const timeoutMs = typeof options?.timeout_ms === 'number' && options.timeout_ms > 0
        ? options.timeout_ms : 3000;
    const settleMs = 500;
    return new Promise((resolve) => {
        // Snapshot "before" state
        const beforeURL = location.href;
        const beforeTitle = document.title;
        // Track text content of elements we can observe
        const textSnapshots = new Map();
        const snapshotSelectors = ['.status', '[role="status"]', '[data-status]', 'h1', 'h2', '.title', '.heading'];
        for (const snapSel of snapshotSelectors) {
            try {
                const matches = document.querySelectorAll(snapSel);
                for (let i = 0; i < matches.length && i < 20; i++) {
                    const el = matches[i];
                    textSnapshots.set(el, (el.textContent || '').trim().slice(0, 200));
                }
            }
            catch { /* ignore invalid selectors */ }
        }
        // Track overlays that exist before the action
        const beforeOverlays = new Set();
        const overlaySelectors = [
            '[role="dialog"]', '[role="alertdialog"]', '[aria-modal="true"]', 'dialog[open]',
            '.modal.show', '.modal.in', '.modal.is-active'
        ];
        for (const oSel of overlaySelectors) {
            try {
                const matches = document.querySelectorAll(oSel);
                for (let i = 0; i < matches.length; i++) {
                    beforeOverlays.add(matches[i]);
                }
            }
            catch { /* ignore */ }
        }
        let elementsAdded = 0;
        let elementsRemoved = 0;
        let networkRequests = 0;
        let lastMutationTime = performance.now();
        const addedNodes = [];
        const startTime = performance.now();
        // Count network requests triggered by the action using PerformanceObserver.
        let perfObserver = null;
        if (typeof PerformanceObserver !== 'undefined') {
            try {
                perfObserver = new PerformanceObserver((list) => {
                    networkRequests += list.getEntries().length;
                });
                perfObserver.observe({ entryTypes: ['resource'] });
            }
            catch { /* PerformanceObserver not available */ }
        }
        const observer = new MutationObserver((records) => {
            lastMutationTime = performance.now();
            for (const record of records) {
                if (record.type === 'childList') {
                    for (let i = 0; i < record.addedNodes.length; i++) {
                        const n = record.addedNodes[i];
                        if (n && n.nodeType === 1) {
                            elementsAdded++;
                            if (addedNodes.length < 500)
                                addedNodes.push(n);
                        }
                    }
                    for (let i = 0; i < record.removedNodes.length; i++) {
                        const n = record.removedNodes[i];
                        if (n && n.nodeType === 1) {
                            elementsRemoved++;
                        }
                    }
                }
            }
        });
        observer.observe(document.body || document.documentElement, {
            childList: true,
            subtree: true,
            attributes: true,
            characterData: true
        });
        // Helper: check if element looks like an overlay
        function isOverlayElement(el) {
            if (!(el instanceof HTMLElement))
                return false;
            const role = el.getAttribute('role') || '';
            if (role === 'dialog' || role === 'alertdialog')
                return true;
            if (el.getAttribute('aria-modal') === 'true')
                return true;
            if (el.tagName.toLowerCase() === 'dialog')
                return true;
            const style = getComputedStyle(el);
            const zIndex = Number.parseInt(style.zIndex || '', 10);
            if (!Number.isNaN(zIndex) && zIndex >= 1000) {
                const position = style.position || '';
                if (position === 'fixed' || position === 'absolute') {
                    const rect = el.getBoundingClientRect();
                    if (rect.width >= 100 && rect.height >= 100)
                        return true;
                }
            }
            return false;
        }
        // Helper: check if element matches any selector from a list
        function matchesAnySelectorSafe(el, sels) {
            for (const sel of sels) {
                try {
                    if (typeof el.matches === 'function' && el.matches(sel))
                        return true;
                }
                catch { /* ignore */ }
            }
            return false;
        }
        // Helper: classify toast type from element classes/attributes
        function classifyToastType(el) {
            const cls = (el.className || '').toString().toLowerCase();
            const role = el.getAttribute('role') || '';
            if (cls.includes('success') || cls.includes('positive'))
                return 'success';
            if (cls.includes('error') || cls.includes('danger') || cls.includes('negative'))
                return 'error';
            if (cls.includes('warning') || cls.includes('caution'))
                return 'warning';
            if (cls.includes('info') || cls.includes('information'))
                return 'info';
            if (role === 'alert')
                return 'alert';
            if (role === 'status')
                return 'status';
            return 'info';
        }
        // Helper: generate a compact selector description for an element
        function describeSelector(el) {
            const tag = el.tagName.toLowerCase();
            if (el.id)
                return `#${el.id}`;
            const role = el.getAttribute('role');
            if (role)
                return `${tag}[role="${role}"]`;
            const cls = el.className;
            if (typeof cls === 'string' && cls.trim()) {
                return `${tag}.${cls.trim().split(/\s+/)[0]}`;
            }
            return tag;
        }
        function classifyAndResolve() {
            observer.disconnect();
            if (perfObserver) {
                try {
                    perfObserver.disconnect();
                }
                catch { /* ignore */ }
            }
            const urlChanged = location.href !== beforeURL;
            const titleChanged = document.title !== beforeTitle;
            const overlaysOpened = [];
            const overlaysClosed = [];
            const afterOverlays = new Set();
            for (const oSel of overlaySelectors) {
                try {
                    const matches = document.querySelectorAll(oSel);
                    for (let i = 0; i < matches.length; i++) {
                        afterOverlays.add(matches[i]);
                    }
                }
                catch { /* ignore */ }
            }
            for (const added of addedNodes) {
                if (isOverlayElement(added))
                    afterOverlays.add(added);
                try {
                    for (const oSel of overlaySelectors) {
                        const children = added.querySelectorAll(oSel);
                        for (let i = 0; i < children.length; i++) {
                            afterOverlays.add(children[i]);
                        }
                    }
                }
                catch { /* ignore */ }
            }
            for (const el of afterOverlays) {
                if (!beforeOverlays.has(el)) {
                    overlaysOpened.push({
                        selector: describeSelector(el),
                        text: (el.textContent || '').trim().slice(0, 120)
                    });
                }
            }
            for (const el of beforeOverlays) {
                if (!afterOverlays.has(el) || !document.contains(el)) {
                    overlaysClosed.push({
                        selector: describeSelector(el),
                        text: ''
                    });
                }
            }
            const toasts = [];
            const toastSelectors = [
                '[role="alert"]', '[role="status"]', '[aria-live="polite"]', '[aria-live="assertive"]',
                '.toast', '.snackbar', '.notification', '.alert',
                '[class*="toast"]', '[class*="snackbar"]', '[class*="notification"]'
            ];
            for (const added of addedNodes) {
                if (matchesAnySelectorSafe(added, toastSelectors)) {
                    const text = (added.textContent || '').trim().slice(0, 200);
                    if (text) {
                        toasts.push({ text, type: classifyToastType(added) });
                    }
                }
                try {
                    for (const tSel of toastSelectors) {
                        const children = added.querySelectorAll(tSel);
                        for (let i = 0; i < children.length; i++) {
                            const child = children[i];
                            const text = (child.textContent || '').trim().slice(0, 200);
                            if (text) {
                                toasts.push({ text, type: classifyToastType(child) });
                            }
                        }
                    }
                }
                catch { /* ignore */ }
            }
            // Detect form errors
            const formErrors = [];
            const errorSelectors = [
                '.error', '.invalid', '.field-error', '.form-error', '.validation-error',
                '[aria-invalid="true"]', '.has-error', '.is-invalid'
            ];
            for (const added of addedNodes) {
                if (matchesAnySelectorSafe(added, errorSelectors)) {
                    const text = (added.textContent || '').trim().slice(0, 200);
                    if (text)
                        formErrors.push(text);
                }
                try {
                    for (const eSel of errorSelectors) {
                        const children = added.querySelectorAll(eSel);
                        for (let i = 0; i < children.length; i++) {
                            const text = (children[i].textContent || '').trim().slice(0, 200);
                            if (text && !formErrors.includes(text))
                                formErrors.push(text);
                        }
                    }
                }
                catch { /* ignore */ }
            }
            // Detect loading indicators
            const loadingIndicators = [];
            const loadingSelectors = [
                '.spinner', '.loading', '.skeleton', '[aria-busy="true"]',
                '[class*="spinner"]', '[class*="loading"]', '[class*="skeleton"]'
            ];
            for (const added of addedNodes) {
                if (matchesAnySelectorSafe(added, loadingSelectors)) {
                    loadingIndicators.push(describeSelector(added));
                }
            }
            const textChanges = [];
            for (const [el, oldText] of textSnapshots) {
                if (!document.contains(el))
                    continue;
                const newText = (el.textContent || '').trim().slice(0, 200);
                if (newText !== oldText) {
                    textChanges.push({
                        selector: describeSelector(el),
                        from: oldText.slice(0, 100),
                        to: newText.slice(0, 100)
                    });
                }
            }
            resolve({
                success: true,
                action: 'action_diff',
                selector: '',
                action_diff: {
                    url_changed: urlChanged,
                    title_changed: titleChanged,
                    overlays_opened: overlaysOpened.slice(0, 10),
                    overlays_closed: overlaysClosed.slice(0, 10),
                    toasts: toasts.slice(0, 10),
                    form_errors: formErrors.slice(0, 20),
                    loading_indicators: loadingIndicators.slice(0, 10),
                    elements_added: elementsAdded,
                    elements_removed: elementsRemoved,
                    text_changes: textChanges.slice(0, 20),
                    network_requests: networkRequests
                }
            });
        }
        // Wait for mutations to settle, then classify
        function checkSettled() {
            const elapsed = performance.now() - startTime;
            const sinceLastMutation = performance.now() - lastMutationTime;
            if (sinceLastMutation >= settleMs || elapsed >= timeoutMs) {
                classifyAndResolve();
                return;
            }
            setTimeout(checkSettled, Math.min(100, settleMs / 2));
        }
        // Start checking after a brief delay to capture initial mutations
        setTimeout(checkSettled, Math.min(100, settleMs / 2));
    });
}
//# sourceMappingURL=dom-primitives-stability.js.map