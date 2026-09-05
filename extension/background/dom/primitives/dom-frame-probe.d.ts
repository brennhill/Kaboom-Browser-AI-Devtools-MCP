/**
 * Purpose: Frame-matching probe executed in page context for targeted DOM actions.
 * Why: Must be self-contained for chrome.scripting.executeScript injection (no closures allowed).
 * Docs: docs/features/feature/interact-explore/index.md
 */
/**
 * Report which origin a frame is, so element results merged across frames stay attributable.
 *
 * `list_interactive` flattens every frame's elements into one array. Without this an agent cannot
 * tell the site's own checkout button from a button drawn by an ad iframe: both arrive in the same
 * list. `location.origin` already excludes the path and query string (rule 13).
 *
 * Must stay self-contained for chrome.scripting.executeScript({ func }).
 */
export declare function domFrameOriginProbe(): {
    origin: string;
    is_top_level_document: boolean;
};
/**
 * Must stay self-contained for chrome.scripting.executeScript({ func }).
 */
export declare function domFrameProbe(frameTarget: string | number): {
    matches: boolean;
};
//# sourceMappingURL=dom-frame-probe.d.ts.map