/**
 * Purpose: Command handlers for the observe MCP tool (screenshot capture, network waterfall, page info, tab listing).
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * Compute the source crop rectangle (in image/device pixels) for an element's
 * CSS-pixel viewport rect. `captureVisibleTab` returns an image scaled by the
 * device pixel ratio, and the rect is viewport-relative CSS pixels — so the
 * crop is `rect * dpr`, clamped to the image bounds. Returns null when there is
 * nothing to crop (non-positive size, or the element lies outside the image).
 */
export declare function computeElementCropRect(rect: {
    x: number;
    y: number;
    width: number;
    height: number;
}, dpr: number, imageWidth: number, imageHeight: number): {
    sx: number;
    sy: number;
    sw: number;
    sh: number;
} | null;
//# sourceMappingURL=observe.d.ts.map