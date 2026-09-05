/**
 * Purpose: Decode a captured data URL far enough to report its real pixel size.
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * Decode a base64 `data:` URL into a Blob without fetch().
 *
 * MV3 service workers do not reliably allow `fetch('data:...')`, and a throw there
 * is what silently disabled the whole selector-crop path in #597 — the caller just
 * posted the uncropped viewport. atob + Uint8Array has no such restriction.
 */
export declare function dataUrlToBlob(dataUrl: string): Blob;
/** A decoded image's real pixel dimensions. */
export interface DecodedImageSize {
    readonly width: number;
    readonly height: number;
}
/**
 * Measure a captured image, or return null with a reason when it cannot be decoded.
 *
 * The coordinate frame's scale is image size divided by the CSS extent the image
 * covers, so this measurement is the difference between a mapping and a guess. It
 * is deliberately MEASURED rather than predicted from viewport times device pixel
 * ratio: the capture may come from Page.captureScreenshot, from
 * chrome.tabs.captureVisibleTab, or from a crop of either, and browser zoom moves
 * the ratio underneath all three.
 */
export declare function measureImageSize(dataUrl: string): Promise<{
    size: DecodedImageSize;
} | {
    reason: string;
}>;
//# sourceMappingURL=image-size.d.ts.map