/**
 * Purpose: Shared frame-targeting normalization used by both DOM dispatch and analyze commands.
 */
// frame-utils.ts — Frame target normalization utilities.
/**
 * Normalize a raw frame parameter into a validated frame target.
 *
 * Returns:
 *   - `undefined` if no frame targeting was requested (null/undefined input)
 *   - a trimmed `string` for CSS selector or "all"
 *   - a non-negative `number` for 0-based frame index
 *   - `null` if the input is invalid (bad type, negative number, empty string)
 */
export function normalizeFrameTarget(frame) {
    if (frame === undefined || frame === null)
        return undefined;
    if (typeof frame === 'number') {
        if (!Number.isInteger(frame) || frame < 0)
            return null;
        return frame;
    }
    if (typeof frame === 'string') {
        const trimmed = frame.trim();
        if (trimmed.length === 0)
            return null;
        return trimmed;
    }
    return null;
}
//# sourceMappingURL=frame-utils.js.map