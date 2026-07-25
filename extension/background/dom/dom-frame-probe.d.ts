/**
 * Purpose: Frame-matching probe executed in page context for targeted DOM actions.
 * Why: Must be self-contained for chrome.scripting.executeScript injection (no closures allowed).
 * Docs: docs/features/feature/interact-explore/index.md
 */
/**
 * Must stay self-contained for chrome.scripting.executeScript({ func }).
 */
export declare function domFrameProbe(frameTarget: string | number): {
    matches: boolean;
};
//# sourceMappingURL=dom-frame-probe.d.ts.map