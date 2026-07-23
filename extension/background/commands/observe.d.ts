/**
 * Purpose: Command handlers for the observe MCP tool (screenshot capture, network waterfall, page info, tab listing).
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * Self-contained function injected via chrome.scripting.executeScript.
 * Temporarily expands scrollable containers so CDP captures full content.
 * Stores original styles in data attributes for restoration.
 */
export declare function screenshotExpandContainers(): {
    expanded: number;
    content_height_hint: number;
};
/** Self-contained: restore containers after full-page capture. */
export declare function screenshotRestoreContainers(): void;
/** Derive bounded screenshot dimensions with fallback defaults and optional expanded-content hint. */
export declare function computeFullPageCaptureDimensions(contentWidth: number, contentHeight: number, hintedHeight: number): {
    width: number;
    height: number;
};
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