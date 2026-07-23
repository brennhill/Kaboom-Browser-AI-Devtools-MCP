/**
 * Purpose: Shared frame-targeting normalization used by both DOM dispatch and analyze commands.
 */
/**
 * Normalize a raw frame parameter into a validated frame target.
 *
 * Returns:
 *   - `undefined` if no frame targeting was requested (null/undefined input)
 *   - a trimmed `string` for CSS selector or "all"
 *   - a non-negative `number` for 0-based frame index
 *   - `null` if the input is invalid (bad type, negative number, empty string)
 */
export declare function normalizeFrameTarget(frame: unknown): string | number | undefined | null;
//# sourceMappingURL=frame-utils.d.ts.map