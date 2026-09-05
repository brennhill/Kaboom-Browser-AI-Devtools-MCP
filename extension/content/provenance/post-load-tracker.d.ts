/**
 * Purpose: Record which parts of the DOM arrived after the document finished loading.
 * Why: Timing is the one provenance fact an agent cannot recover from the payload it is handed.
 *      A block present in the document Chrome parsed and a block a third-party tag wrote into the
 *      page seconds later are indistinguishable once both are text — unless something watched.
 * Docs: docs/features/feature/content-provenance/index.md
 */
/** The narrow slice of MutationObserver this tracker uses, so tests can drive it directly. */
export interface MutationObserverLike {
    observe(target: Node, options: MutationObserverInit): void;
    disconnect(): void;
}
export type MutationObserverFactory = (callback: (records: MutationRecord[]) => void) => MutationObserverLike;
/** What a provenance collector needs to ask about delivery timing. */
export interface InjectionQuery {
    readonly is_active: boolean;
    readonly overflowed: boolean;
    wasInjectedAfterLoad(node: Node | null | undefined): boolean | null;
    injectedRoots(): Element[];
    postLoadResourceOrigins(): string[];
}
/**
 * Watches a document for insertions made after its `load` event.
 *
 * The boundary is `load` rather than `DOMContentLoaded` because deferred and async scripts are
 * still part of getting the page up: counting their output as an injection would classify most
 * of the web as injected and make the signal useless. Content a script writes after `load` — the
 * shape an ad network or a late third-party tag takes — is what this records.
 */
export declare class PostLoadInjectionTracker implements InjectionQuery {
    private active;
    private loaded;
    private capped;
    private baseHref;
    private observer;
    private readonly roots;
    private readonly resourceOrigins;
    get is_active(): boolean;
    get overflowed(): boolean;
    get max_tracked_roots(): number;
    /** Begin observing. Idempotent: a second call on a live tracker is a no-op. */
    start(doc: Document, win: Window, makeObserver?: MutationObserverFactory): void;
    /** Stop observing and drop the active flag, so later queries report unknown rather than false. */
    disconnect(): void;
    markDocumentLoaded(): void;
    /** Record one inserted node. Insertions before `load` are part of the initial document. */
    recordInsertion(node: Node | null | undefined): void;
    /**
     * Whether `node` arrived after load.
     *
     * `null` means the answer is unknown — the tracker never observed this document, or it stopped
     * recording at the retention cap. Reporting `false` in either case would read as an assurance
     * the tracker cannot give.
     */
    wasInjectedAfterLoad(node: Node | null | undefined): boolean | null;
    /** Injected roots still in the document, with any root nested inside another dropped. */
    injectedRoots(): Element[];
    /** Origins of scripts and frames added after load — candidate initiators, not a culprit. */
    postLoadResourceOrigins(): string[];
    private consume;
    private elementFor;
    private recordResourceOrigin;
    /** Drop roots the page has since removed, so churn does not consume the retention budget. */
    private pruneDetachedRoots;
}
//# sourceMappingURL=post-load-tracker.d.ts.map