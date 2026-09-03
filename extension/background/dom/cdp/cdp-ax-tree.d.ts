/**
 * Purpose: Targets elements by their ACCESSIBILITY semantics — role, accessible name, state —
 *          rather than by DOM shape, and resolves a natural-language query to candidates.
 * Why: list_interactive is a hand-rolled DOM scan that infers roles from tag names. On a
 *      canvas-drawn control, a custom grid, or any widget whose meaning lives in ARIA rather
 *      than markup, kaboom had no way to name the target at all. Accessibility.getFullAXTree
 *      appeared 0 times in this repo.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import type { Lease } from './cdp-session.js';
/** One actionable node from the accessibility tree. */
export interface AXNode {
    ref: string;
    role: string;
    name: string;
    value?: string;
    states: string[];
    /**
     * Viewport centre — the point CDP input would target. Undefined until resolveAXGeometry
     * runs: the accessibility tree carries no coordinates, and a box-model round trip for
     * every node on the page would cost one CDP call per element.
     */
    x?: number;
    y?: number;
    width?: number;
    height?: number;
    backend_node_id?: number;
}
export interface AXCandidate {
    node: AXNode;
    confidence: number;
    why: string;
}
/** Below this a match is noise, and reporting it would invite a blind click. */
export declare const AX_MIN_CONFIDENCE = 0.3;
/** Split a query into comparable tokens, discarding case, punctuation and spacing. */
export declare function normalizeQuery(query: string | null | undefined): string[];
/** True when the query names the role, e.g. "search bar" for role searchbox. */
export declare function roleMatchesQuery(role: string, query: string): boolean;
/**
 * Rank accessibility nodes against a natural-language query.
 *
 * Pure, so it is testable with no browser. Returns EVERY candidate above the confidence
 * floor rather than one answer: an ambiguous query must stay ambiguous in the response so
 * the caller can disambiguate instead of blind-clicking the first hit.
 */
export declare function rankAXCandidates(nodes: readonly AXNode[] | null | undefined, query: string | null | undefined): AXCandidate[];
/**
 * Read the page's accessibility tree.
 *
 * This is the semantic view assistive technology sees, so it names controls the DOM cannot:
 * a canvas-drawn widget with ARIA attributes, an aria-label that differs from visible text,
 * a role overridden on a plain div.
 */
export declare function fetchAXNodes(lease: Lease): Promise<AXNode[]>;
/**
 * Fill in viewport geometry for the nodes given, dropping any whose box cannot be read.
 *
 * Called for ranked candidates only. A node scrolled out of layout, or removed between the
 * snapshot and this call, has no box model — dropping it is correct, because inventing 0,0
 * would send a click to the top-left corner of the page.
 */
export declare function resolveAXGeometry(lease: Lease, nodes: readonly AXNode[]): Promise<AXNode[]>;
//# sourceMappingURL=cdp-ax-tree.d.ts.map