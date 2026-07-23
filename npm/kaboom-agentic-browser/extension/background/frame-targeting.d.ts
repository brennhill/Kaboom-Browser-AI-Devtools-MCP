/**
 * Purpose: Shared frame-target normalization and probing for background command handlers.
 * Why: Keep frame matching behavior/error contracts consistent across analyze/interact paths.
 * Docs: docs/features/feature/interact-explore/index.md
 */
export type NormalizedFrameTarget = string | number | 'all' | undefined;
/**
 * Normalize and validate a frame argument from tool params.
 * Throws with a stable error contract for unsupported values.
 */
export declare function normalizeFrameArg(frame: unknown): NormalizedFrameTarget;
/**
 * Probe all frames and return frame IDs matching the supplied target.
 * The probe function must be self-contained for chrome.scripting.executeScript.
 */
export declare function resolveMatchedFrameIds(tabId: number, frameTarget: string | number | 'all', probeFn: (frameTarget: string | number) => {
    matches: boolean;
}): Promise<number[]>;
//# sourceMappingURL=frame-targeting.d.ts.map