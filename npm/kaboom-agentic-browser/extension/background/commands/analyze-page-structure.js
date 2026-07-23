// analyze-page-structure.ts — Structural page analysis command handler (#341).
import { registerCommand } from './registry.js';
import { errorMessage } from '../../lib/error-utils.js';
/**
 * Combined page structure script. When useGlobals=true (MAIN world), detects
 * frameworks via window globals. When false (ISOLATED fallback), uses DOM hints only.
 */
function pageStructureScript(useGlobals) {
    const MAX_SCROLL_CONTAINERS = 20;
    const MAX_MODALS = 20;
    // --- Framework detection ---
    const frameworks = [];
    if (useGlobals) {
        const win = window;
        // Vue
        if (win.__VUE__ || win.__VUE_DEVTOOLS_GLOBAL_HOOK__) {
            const vueObj = win.__VUE__;
            const version = typeof vueObj?.version === 'string' ? vueObj.version : '';
            frameworks.push({ name: 'Vue', version, evidence: 'window.__VUE__' });
        }
        // React (DOM + fiber check)
        const reactRoot = document.querySelector('[data-reactroot]');
        const reactContainer = document.getElementById('root');
        const hasReactFiber = reactContainer ? '_reactRootContainer' in reactContainer : false;
        if (reactRoot || hasReactFiber) {
            frameworks.push({ name: 'React', version: '', evidence: reactRoot ? 'data-reactroot' : '_reactRootContainer' });
        }
        // Next.js
        if (win.__NEXT_DATA__) {
            const nextData = win.__NEXT_DATA__;
            frameworks.push({ name: 'Next.js', version: nextData.buildId || '', evidence: 'window.__NEXT_DATA__' });
        }
        // Nuxt
        if (win.__NUXT__ || win.$nuxt) {
            frameworks.push({ name: 'Nuxt', version: '', evidence: win.__NUXT__ ? 'window.__NUXT__' : 'window.$nuxt' });
        }
        // Angular
        if (typeof win.ng === 'object' ||
            typeof win.getAllAngularRootElements === 'function') {
            frameworks.push({ name: 'Angular', version: '', evidence: win.ng ? 'window.ng' : 'getAllAngularRootElements' });
        }
        // Svelte (DOM hint — Svelte adds class="svelte-XXXXX" hashes)
        if (document.querySelector('[class^="svelte-"], [class*=" svelte-"]')) {
            frameworks.push({ name: 'Svelte', version: '', evidence: 'class^="svelte-"' });
        }
    }
    else {
        // ISOLATED world: DOM hints only
        if (document.querySelector('[data-reactroot]') || document.querySelector('[data-reactid]')) {
            frameworks.push({ name: 'React', version: '', evidence: 'data-reactroot' });
        }
        if (document.querySelector('[class^="svelte-"], [class*=" svelte-"]')) {
            frameworks.push({ name: 'Svelte', version: '', evidence: 'class^="svelte-"' });
        }
        if (document.querySelector('[ng-version]') ||
            document.querySelector('[_nghost]') ||
            document.querySelector('app-root')) {
            const ver = document.querySelector('[ng-version]')?.getAttribute('ng-version') || '';
            frameworks.push({ name: 'Angular', version: ver, evidence: 'ng-version' });
        }
        if (document.querySelector('#__next') || document.querySelector('[data-nextjs-page]')) {
            frameworks.push({ name: 'Next.js', version: '', evidence: '#__next' });
        }
        if (document.querySelector('#__nuxt') || document.querySelector('#__layout')) {
            frameworks.push({ name: 'Nuxt', version: '', evidence: '#__nuxt' });
        }
        // Vue: data-v-XXXXX scoped attributes (CSS can't match attribute name prefix, need JS check)
        const hasVueScopedAttr = document.querySelector('[data-vue-meta]') ||
            Array.from(document.querySelector('#app')?.attributes || document.documentElement.attributes).some((a) => a.name.startsWith('data-v-'));
        if (hasVueScopedAttr) {
            frameworks.push({ name: 'Vue', version: '', evidence: 'data-v-*' });
        }
    }
    // --- Routing detection ---
    let routing = { type: 'unknown', evidence: '' };
    if (useGlobals) {
        const win = window;
        if (win.__NEXT_DATA__) {
            routing = { type: 'next', evidence: '__NEXT_DATA__' };
        }
        else if (win.__NUXT__) {
            routing = { type: 'nuxt', evidence: '__NUXT__' };
        }
        else if (window.location.hash.length > 1) {
            routing = { type: 'hash', evidence: 'location.hash' };
        }
    }
    else {
        if (document.querySelector('#__next')) {
            routing = { type: 'next', evidence: '#__next' };
        }
        else if (document.querySelector('#__nuxt')) {
            routing = { type: 'nuxt', evidence: '#__nuxt' };
        }
        else if (window.location.hash.length > 1) {
            routing = { type: 'hash', evidence: 'location.hash' };
        }
    }
    // --- Scroll containers ---
    const scrollContainers = [];
    const allElements = document.querySelectorAll('*');
    // Bail out on massive DOMs to avoid expensive getComputedStyle calls (#9.7.6)
    const skipScrollDetection = allElements.length > 50000;
    let scCount = 0;
    for (const el of Array.from(skipScrollDetection ? [] : allElements)) {
        if (scCount >= MAX_SCROLL_CONTAINERS)
            break;
        const htmlEl = el;
        if (htmlEl.scrollHeight > htmlEl.clientHeight + 50 && htmlEl.clientHeight > 0) {
            const style = getComputedStyle(htmlEl);
            if (style.overflow === 'auto' ||
                style.overflow === 'scroll' ||
                style.overflowY === 'auto' ||
                style.overflowY === 'scroll') {
                const tag = htmlEl.tagName.toLowerCase();
                const id = htmlEl.id ? `#${htmlEl.id}` : '';
                const cls = htmlEl.className && typeof htmlEl.className === 'string'
                    ? '.' + htmlEl.className.trim().split(/\s+/).slice(0, 2).join('.')
                    : '';
                scrollContainers.push({
                    selector: tag + id + cls,
                    scroll_height: htmlEl.scrollHeight,
                    client_height: htmlEl.clientHeight
                });
                scCount++;
            }
        }
    }
    // --- Modal/dialog detection ---
    const modals = [];
    const dialogEls = document.querySelectorAll('dialog, [role="dialog"], [role="alertdialog"], .modal, [aria-modal="true"]');
    let modalCount = 0;
    for (const el of Array.from(dialogEls)) {
        if (modalCount >= MAX_MODALS)
            break;
        const htmlEl = el;
        const tag = htmlEl.tagName.toLowerCase();
        const id = htmlEl.id ? `#${htmlEl.id}` : '';
        const role = htmlEl.getAttribute('role') || '';
        const isDialog = tag === 'dialog';
        const visible = isDialog
            ? htmlEl.open
            : htmlEl.offsetParent !== null || getComputedStyle(htmlEl).display !== 'none';
        let modalType = 'unknown';
        if (tag === 'dialog')
            modalType = 'dialog';
        else if (role === 'dialog' || role === 'alertdialog')
            modalType = role;
        else if (htmlEl.classList.contains('modal'))
            modalType = 'modal';
        else if (htmlEl.getAttribute('aria-modal') === 'true')
            modalType = 'aria-modal';
        modals.push({ selector: tag + id, visible, type: modalType });
        modalCount++;
    }
    // --- Shadow DOM count ---
    // Cap iteration to avoid blocking on massive DOMs (#9.R8)
    const MAX_SHADOW_WALK = 50000;
    let shadowRoots = 0;
    let shadowWalked = 0;
    const walker = document.createTreeWalker(document.body || document.documentElement, NodeFilter.SHOW_ELEMENT);
    while (walker.nextNode()) {
        shadowWalked++;
        if (shadowWalked > MAX_SHADOW_WALK)
            break;
        if (walker.currentNode.shadowRoot) {
            shadowRoots++;
        }
    }
    // --- Meta tags ---
    const meta = {
        viewport: document.querySelector('meta[name="viewport"]')?.getAttribute('content') || '',
        charset: document.querySelector('meta[charset]')?.getAttribute('charset') ||
            document.querySelector('meta[http-equiv="Content-Type"]')?.getAttribute('content') ||
            '',
        og_title: document.querySelector('meta[property="og:title"]')?.getAttribute('content') || '',
        description: document.querySelector('meta[name="description"]')?.getAttribute('content') || ''
    };
    return {
        frameworks,
        routing,
        scroll_containers: scrollContainers,
        modals,
        shadow_roots: shadowRoots,
        meta
    };
}
registerCommand('page_structure', async (ctx) => {
    try {
        // Try MAIN world first (for framework globals)
        let results;
        try {
            results = await chrome.scripting.executeScript({
                target: { tabId: ctx.tabId },
                world: 'MAIN',
                func: pageStructureScript,
                args: [true]
            });
        }
        catch {
            // MAIN world failed (CSP restriction), fallback to ISOLATED
            results = await chrome.scripting.executeScript({
                target: { tabId: ctx.tabId },
                world: 'ISOLATED',
                func: pageStructureScript,
                args: [false]
            });
        }
        const first = results?.[0]?.result;
        const payload = first && typeof first === 'object' ? first : {};
        ctx.sendResult(payload);
    }
    catch (err) {
        const message = errorMessage(err, 'Page structure analysis failed');
        ctx.sendResult({
            error: 'page_structure_failed',
            message
        });
    }
});
//# sourceMappingURL=analyze-page-structure.js.map