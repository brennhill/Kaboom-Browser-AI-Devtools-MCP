/**
 * Purpose: Record which parts of the DOM arrived after the document finished loading.
 * Why: Timing is the one provenance fact an agent cannot recover from the payload it is handed.
 *      A block present in the document Chrome parsed and a block a third-party tag wrote into the
 *      page seconds later are indistinguishable once both are text — unless something watched.
 * Docs: docs/features/feature/content-provenance/index.md
 */
// post-load-tracker.ts — MutationObserver-backed record of post-load DOM insertions.
import { toOrigin } from '../../lib/provenance/origins.js';
/** Element nodeType, as the DOM spec numbers it. */
const ELEMENT_NODE = 1;
/**
 * How many injected roots are retained.
 *
 * A page that rewrites itself continuously would otherwise grow this without bound. Hitting the
 * cap is reported through `overflowed` rather than silently dropping the signal.
 */
const MAX_TRACKED_ROOTS = 200;
/** Elements whose own URL names a candidate initiator for what followed. */
const RESOURCE_TAGS = new Set(['SCRIPT', 'IFRAME', 'FRAME']);
/**
 * Watches a document for insertions made after its `load` event.
 *
 * The boundary is `load` rather than `DOMContentLoaded` because deferred and async scripts are
 * still part of getting the page up: counting their output as an injection would classify most
 * of the web as injected and make the signal useless. Content a script writes after `load` — the
 * shape an ad network or a late third-party tag takes — is what this records.
 */
export class PostLoadInjectionTracker {
    active = false;
    loaded = false;
    capped = false;
    baseHref = '';
    observer = null;
    roots = new Set();
    resourceOrigins = new Set();
    get is_active() {
        return this.active;
    }
    get overflowed() {
        return this.capped;
    }
    get max_tracked_roots() {
        return MAX_TRACKED_ROOTS;
    }
    /** Begin observing. Idempotent: a second call on a live tracker is a no-op. */
    start(doc, win, makeObserver) {
        if (this.active)
            return;
        this.active = true;
        this.baseHref = doc.baseURI ?? '';
        this.loaded = doc.readyState === 'complete';
        if (!this.loaded)
            win.addEventListener('load', () => this.markDocumentLoaded(), { once: true });
        const factory = makeObserver ?? ((callback) => new MutationObserver(callback));
        this.observer = factory((records) => this.consume(records));
        this.observer.observe(doc.documentElement ?? doc, { childList: true, subtree: true });
    }
    /** Stop observing and drop the active flag, so later queries report unknown rather than false. */
    disconnect() {
        this.observer?.disconnect();
        this.observer = null;
        this.active = false;
    }
    markDocumentLoaded() {
        this.loaded = true;
    }
    /** Record one inserted node. Insertions before `load` are part of the initial document. */
    recordInsertion(node) {
        if (!this.loaded || !node)
            return;
        const element = this.elementFor(node);
        if (!element)
            return;
        this.recordResourceOrigin(element);
        if (this.roots.has(element) || this.capped)
            return;
        if (this.roots.size >= MAX_TRACKED_ROOTS)
            this.pruneDetachedRoots();
        if (this.roots.size >= MAX_TRACKED_ROOTS) {
            this.capped = true;
            return;
        }
        this.roots.add(element);
    }
    /**
     * Whether `node` arrived after load.
     *
     * `null` means the answer is unknown — the tracker never observed this document, or it stopped
     * recording at the retention cap. Reporting `false` in either case would read as an assurance
     * the tracker cannot give.
     */
    wasInjectedAfterLoad(node) {
        if (!this.active)
            return null;
        let cursor = this.elementFor(node);
        while (cursor) {
            if (this.roots.has(cursor))
                return true;
            cursor = cursor.parentElement;
        }
        // Past the cap the tracker stopped recording, so "not recorded" no longer means "not injected".
        return this.capped ? null : false;
    }
    /** Injected roots still in the document, with any root nested inside another dropped. */
    injectedRoots() {
        const live = [...this.roots].filter((element) => element.isConnected !== false);
        return live.filter((element) => !live.some((other) => other !== element && other.contains(element)));
    }
    /** Origins of scripts and frames added after load — candidate initiators, not a culprit. */
    postLoadResourceOrigins() {
        return [...this.resourceOrigins];
    }
    consume(records) {
        for (const record of records) {
            for (const added of record.addedNodes)
                this.recordInsertion(added);
        }
    }
    elementFor(node) {
        if (!node)
            return null;
        if (node.nodeType === ELEMENT_NODE)
            return node;
        return node.parentElement ?? null;
    }
    recordResourceOrigin(element) {
        if (!RESOURCE_TAGS.has(element.tagName))
            return;
        const origin = toOrigin(element.getAttribute('src'), this.baseHref || null);
        if (origin)
            this.resourceOrigins.add(origin);
    }
    /** Drop roots the page has since removed, so churn does not consume the retention budget. */
    pruneDetachedRoots() {
        for (const element of this.roots) {
            if (element.isConnected === false)
                this.roots.delete(element);
        }
    }
}
//# sourceMappingURL=post-load-tracker.js.map