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
export function dataUrlToBlob(dataUrl) {
    const comma = dataUrl.indexOf(',');
    if (!dataUrl.startsWith('data:') || comma === -1) {
        throw new Error('not a data URL');
    }
    const header = dataUrl.slice(5, comma);
    if (!header.includes(';base64')) {
        throw new Error('data URL is not base64-encoded');
    }
    const mime = header.split(';')[0] || 'application/octet-stream';
    const binary = atob(dataUrl.slice(comma + 1));
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++)
        bytes[i] = binary.charCodeAt(i);
    return new Blob([bytes], { type: mime });
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
export async function measureImageSize(dataUrl) {
    if (typeof createImageBitmap === 'undefined')
        return { reason: 'createimagebitmap_unavailable' };
    let blob;
    try {
        blob = dataUrlToBlob(dataUrl);
    }
    catch (err) {
        return { reason: `datauri_decode_failed: ${err instanceof Error ? err.message : String(err)}` };
    }
    let bitmap;
    try {
        bitmap = await createImageBitmap(blob);
    }
    catch (err) {
        return { reason: `bitmap_decode_failed: ${err instanceof Error ? err.message : String(err)}` };
    }
    try {
        if (!(bitmap.width > 0) || !(bitmap.height > 0)) {
            return { reason: `empty_image (${bitmap.width}x${bitmap.height})` };
        }
        return { size: { width: bitmap.width, height: bitmap.height } };
    }
    finally {
        bitmap.close?.();
    }
}
//# sourceMappingURL=image-size.js.map