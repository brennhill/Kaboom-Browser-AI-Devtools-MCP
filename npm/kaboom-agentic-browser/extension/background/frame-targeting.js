/**
 * Purpose: Shared frame-target normalization and probing for background command handlers.
 * Why: Keep frame matching behavior/error contracts consistent across analyze/interact paths.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import { normalizeFrameTarget } from '../lib/frame-utils.js';
const INVALID_FRAME_ERROR = 'invalid_frame: frame parameter must be a CSS selector, 0-based index, or "all". Got unsupported type or value';
const FRAME_NOT_FOUND_ERROR = 'frame_not_found: no iframe matched the given selector or index. Verify the iframe exists and is loaded on the page';
/**
 * Normalize and validate a frame argument from tool params.
 * Throws with a stable error contract for unsupported values.
 */
export function normalizeFrameArg(frame) {
    const normalized = normalizeFrameTarget(frame);
    if (normalized === null) {
        throw new Error(INVALID_FRAME_ERROR);
    }
    return normalized;
}
/**
 * Probe all frames and return frame IDs matching the supplied target.
 * The probe function must be self-contained for chrome.scripting.executeScript.
 */
export async function resolveMatchedFrameIds(tabId, frameTarget, probeFn) {
    const probeResults = await chrome.scripting.executeScript({
        target: { tabId, allFrames: true },
        world: 'MAIN',
        func: probeFn,
        args: [frameTarget]
    });
    const frameIds = Array.from(new Set(probeResults
        .filter((r) => !!r.result?.matches)
        .map((r) => r.frameId)
        .filter((id) => typeof id === 'number')));
    if (frameIds.length === 0) {
        throw new Error(FRAME_NOT_FOUND_ERROR);
    }
    return frameIds;
}
//# sourceMappingURL=frame-targeting.js.map