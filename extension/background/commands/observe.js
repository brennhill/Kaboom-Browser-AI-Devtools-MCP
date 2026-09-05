/**
 * Purpose: Command handlers for the observe MCP tool (screenshot capture, network waterfall, page info, tab listing).
 * Docs: docs/features/feature/observe/index.md
 */
import { DebugCategory, debugLog } from '../debug.js';
import { recordScreenshot } from '../caches/cache-limits.js';
import { domPrimitiveListInteractive } from '../dom/primitives/dom-primitives-list-interactive.js';
import { registerCommand } from './registry.js';
import { collectCommandElements, commandPageMetadata, selectCommandElements } from './results/element-results.js';
import { errorMessage } from '../../lib/error-utils.js';
import { KABOOM_LOG_PREFIX } from '../../lib/brand.js';
import { delay } from '../../lib/timeout-utils.js';
import { captureTabImage } from '../ui/tracked-tab-state.js';
import { cdpSessions } from '../dom/cdp/cdp-session.js';
import { dataUrlToBlob } from '../../lib/screenshot/image-size.js';
import { describeCapture, describeCaptureWith, postScreenshot, readViewportMetrics } from './results/screenshot-delivery.js';
// =============================================================================
// SCREENSHOT
// =============================================================================
const MAX_CAPTURE_HEIGHT = 16384; // Chrome max texture size
const MAX_CAPTURE_WIDTH = 16384; // Chrome max texture size
const DEFAULT_CAPTURE_WIDTH = 1280;
const DEFAULT_CAPTURE_HEIGHT = 720;
/**
 * Self-contained function injected via chrome.scripting.executeScript.
 * Temporarily expands scrollable containers so CDP captures full content.
 * Stores original styles in data attributes for restoration.
 */
export function screenshotExpandContainers() {
    let count = 0;
    let contentHeightHint = Math.max(document.documentElement?.scrollHeight || 0, document.body?.scrollHeight || 0);
    function tryExpand(el) {
        const style = getComputedStyle(el);
        const oy = style.overflowY || '';
        const ov = style.overflow || '';
        const isScrollable = oy === 'auto' ||
            oy === 'scroll' ||
            oy === 'hidden' ||
            oy === 'clip' ||
            ov === 'auto' ||
            ov === 'scroll' ||
            ov === 'hidden' ||
            ov === 'clip';
        if (isScrollable && el.scrollHeight > el.clientHeight + 1) {
            const targetHeight = Math.max(el.scrollHeight, el.clientHeight);
            el.setAttribute('data-kaboom-fpx', JSON.stringify({
                o: el.style.overflow,
                oy: el.style.overflowY,
                ox: el.style.overflowX,
                h: el.style.height,
                n: el.style.minHeight,
                m: el.style.maxHeight,
                f: el.style.flex,
                c: el.style.contain
            }));
            el.style.overflow = 'visible';
            el.style.overflowY = 'visible';
            el.style.overflowX = 'visible';
            el.style.height = `${targetHeight}px`;
            el.style.minHeight = `${targetHeight}px`;
            el.style.maxHeight = 'none';
            el.style.flex = 'none';
            el.style.contain = 'none';
            const top = (el.getBoundingClientRect().top || 0) + (window.scrollY || window.pageYOffset || 0);
            contentHeightHint = Math.max(contentHeightHint, top + targetHeight);
            count++;
        }
    }
    tryExpand(document.documentElement);
    tryExpand(document.body);
    const all = document.body.querySelectorAll('*');
    for (let i = 0; i < all.length; i++) {
        if (all[i] instanceof HTMLElement)
            tryExpand(all[i]);
    }
    return { expanded: count, content_height_hint: Math.ceil(contentHeightHint) };
}
/** Self-contained: restore containers after full-page capture. */
export function screenshotRestoreContainers() {
    function tryRestore(el) {
        const raw = el.getAttribute('data-kaboom-fpx');
        if (!raw)
            return;
        try {
            const s = JSON.parse(raw);
            el.style.overflow = s.o || '';
            el.style.overflowY = s.oy || '';
            el.style.overflowX = s.ox || '';
            el.style.height = s.h || '';
            el.style.minHeight = s.n || '';
            el.style.maxHeight = s.m || '';
            el.style.flex = s.f || '';
            el.style.contain = s.c || '';
        }
        catch {
            // EXPECTED_ABSENCE: optional enrichment can normally fail while the primary
            // operation keeps a valid fallback; logging it would misleadingly report fallback as failure.
            /* ignore parse errors */
        }
        el.removeAttribute('data-kaboom-fpx');
    }
    tryRestore(document.documentElement);
    const all = document.querySelectorAll('[data-kaboom-fpx]');
    for (let i = 0; i < all.length; i++) {
        tryRestore(all[i]);
    }
}
/** Derive bounded screenshot dimensions with fallback defaults and optional expanded-content hint. */
export function computeFullPageCaptureDimensions(contentWidth, contentHeight, hintedHeight) {
    const safeWidth = Number.isFinite(contentWidth) && contentWidth > 0 ? Math.ceil(contentWidth) : DEFAULT_CAPTURE_WIDTH;
    const safeHeight = Number.isFinite(contentHeight) && contentHeight > 0 ? Math.ceil(contentHeight) : DEFAULT_CAPTURE_HEIGHT;
    const safeHint = Number.isFinite(hintedHeight) && hintedHeight > 0 ? Math.ceil(hintedHeight) : 0;
    return {
        width: Math.max(1, Math.min(safeWidth, MAX_CAPTURE_WIDTH)),
        height: Math.max(1, Math.min(Math.max(safeHeight, safeHint), MAX_CAPTURE_HEIGHT))
    };
}
/**
 * Capture the viewport and post it. Runs entirely in the background: `captureTabImage`
 * takes a CDP lease rather than activating the tab, so an observe(screenshot) while the
 * user is reading a different tab no longer yanks their window away.
 */
async function captureAndPostViewport(ctx, tab, format, quality) {
    const capture = await captureTabImage(ctx.tabId, tab.windowId, { format, quality });
    recordScreenshot(ctx.tabId);
    const delivery = await describeCapture(ctx.tabId, capture.data_url, 'viewport', capture.covered_css_region);
    if (!(await postScreenshot(capture.data_url, tab.url, ctx.query.id, delivery))) {
        ctx.sendResult({ error: 'screenshot_upload_failed', message: 'Server rejected screenshot' });
    }
}
registerCommand('screenshot', async (ctx) => {
    const format = ctx.params.format === 'png' ? 'png' : 'jpeg';
    const quality = typeof ctx.params.quality === 'number' ? ctx.params.quality : 80;
    const fullPage = ctx.params.full_page === true;
    try {
        const tab = await chrome.tabs.get(ctx.tabId);
        if (fullPage) {
            await captureFullPage(ctx, tab, format, quality);
            return;
        }
        // #597: selector-scoped capture. Honor the advertised `selector` param
        // (previously silently ignored) by scrolling the element into view and
        // cropping the viewport capture to it.
        const selector = typeof ctx.params.selector === 'string' ? ctx.params.selector.trim() : '';
        if (selector) {
            await captureElement(ctx, tab, selector, format, quality);
            return;
        }
        await captureAndPostViewport(ctx, tab, format, quality);
        // No sendResult needed — server resolves the query via query_id
    }
    catch (err) {
        ctx.sendResult({
            error: 'screenshot_failed',
            message: errorMessage(err, 'Failed to capture screenshot')
        });
    }
});
/**
 * Runs in the page's MAIN world. Scrolls the matched element into view —
 * `scrollIntoView` honors *every* scrollable ancestor, including nested
 * `overflow:auto` containers, which is the exact case #597 reported — then
 * returns its viewport-relative rect and the device pixel ratio.
 */
function screenshotResolveElementRect(selector) {
    const el = document.querySelector(selector);
    if (!el)
        return { found: false, x: 0, y: 0, width: 0, height: 0, dpr: 1 };
    // Instant (default behavior) so layout settles before we read the rect.
    el.scrollIntoView({ block: 'center', inline: 'center' });
    const r = el.getBoundingClientRect();
    return {
        found: true,
        x: r.left,
        y: r.top,
        width: r.width,
        height: r.height,
        dpr: window.devicePixelRatio || 1
    };
}
/**
 * Compute the source crop rectangle (in image/device pixels) for an element's
 * CSS-pixel viewport rect. The capture is clipped to the visual viewport and
 * scaled by the page's device pixel ratio, and the rect is viewport-relative
 * CSS pixels — so the crop is `rect * dpr`, clamped to the image bounds.
 * Returns null when there is nothing to crop (non-positive size, or the element
 * lies outside the image).
 */
export function computeElementCropRect(rect, dpr, imageWidth, imageHeight) {
    if (rect.width <= 0 || rect.height <= 0)
        return null;
    const scale = dpr > 0 ? dpr : 1;
    const sx = Math.max(0, Math.round(rect.x * scale));
    const sy = Math.max(0, Math.round(rect.y * scale));
    const sw = Math.min(Math.round(rect.width * scale), imageWidth - sx);
    const sh = Math.min(Math.round(rect.height * scale), imageHeight - sy);
    if (sw <= 0 || sh <= 0)
        return null;
    return { sx, sy, sw, sh };
}
async function blobToDataUrl(blob) {
    const bytes = new Uint8Array(await blob.arrayBuffer());
    let binary = '';
    const chunk = 0x8000;
    for (let i = 0; i < bytes.length; i += chunk) {
        binary += String.fromCharCode(...bytes.subarray(i, i + chunk));
    }
    return `data:${blob.type};base64,${btoa(binary)}`;
}
/**
 * Crop a captured data URL to the element rect via OffscreenCanvas.
 *
 * Always reports WHY it could not crop. A bare `return null` here made #597 look
 * fixed while the caller silently posted the uncropped viewport — the failure was
 * invisible in unit tests (which never run a service worker) and in the product.
 * Any user-visible fallback must carry a reason.
 */
async function cropDataUrlToRect(dataUrl, rect, format, quality) {
    if (typeof OffscreenCanvas === 'undefined')
        return { reason: 'offscreencanvas_unavailable' };
    if (typeof createImageBitmap === 'undefined')
        return { reason: 'createimagebitmap_unavailable' };
    // Decode the data: URL by hand rather than via fetch(). MV3 service workers
    // restrict fetching data: URLs, and a throw there is what silently disabled the
    // whole crop path (#597) — the caller just posted the uncropped viewport.
    // atob + Uint8Array has no such restriction.
    let blob;
    try {
        blob = dataUrlToBlob(dataUrl);
    }
    catch (err) {
        return { reason: `datauri_decode_failed: ${errorMessage(err)}` };
    }
    const bitmap = await createImageBitmap(blob);
    try {
        const crop = computeElementCropRect(rect, rect.dpr, bitmap.width, bitmap.height);
        if (!crop) {
            return {
                reason: `empty_crop_rect (element ${rect.width}x${rect.height} @ ${rect.x},${rect.y} ` +
                    `dpr=${rect.dpr} vs image ${bitmap.width}x${bitmap.height})`
            };
        }
        const canvas = new OffscreenCanvas(crop.sw, crop.sh);
        const c = canvas.getContext('2d');
        if (!c)
            return { reason: 'no_2d_context' };
        c.drawImage(bitmap, crop.sx, crop.sy, crop.sw, crop.sh, 0, 0, crop.sw, crop.sh);
        const mime = format === 'png' ? 'image/png' : 'image/jpeg';
        const outBlob = await canvas.convertToBlob({
            type: mime,
            quality: format === 'jpeg' ? quality / 100 : undefined
        });
        return { dataUrl: await blobToDataUrl(outBlob) };
    }
    finally {
        bitmap.close?.();
    }
}
/**
 * Selector-scoped screenshot (#597): scroll the element into view (honoring
 * nested scroll containers) and crop the viewport capture to it. Falls back to
 * the (correctly scrolled) uncropped viewport if the crop cannot be produced,
 * so the capture never fails outright.
 */
async function captureElement(ctx, tab, selector, format, quality) {
    let rect = null;
    try {
        const res = await chrome.scripting.executeScript({
            target: { tabId: ctx.tabId },
            world: 'MAIN',
            func: screenshotResolveElementRect,
            args: [selector]
        });
        rect = res[0]?.result ?? null;
    }
    catch (err) {
        debugLog(DebugCategory.CAPTURE, 'Selector resolve script failed', { error: errorMessage(err) });
    }
    if (!rect || !rect.found) {
        ctx.sendResult({ error: 'element_not_found', message: `No element matched selector: ${selector}` });
        return;
    }
    // Let the (instant) scroll repaint before the compositor raster.
    await delay(120);
    const capture = await captureTabImage(ctx.tabId, tab.windowId, { format, quality });
    recordScreenshot(ctx.tabId);
    const dataUrl = capture.data_url;
    let outUrl = dataUrl;
    let cropFallbackReason = null;
    try {
        const cropped = await cropDataUrlToRect(dataUrl, rect, format, quality);
        if ('dataUrl' in cropped) {
            outUrl = cropped.dataUrl;
        }
        else {
            cropFallbackReason = cropped.reason;
        }
    }
    catch (err) {
        cropFallbackReason = `crop_threw: ${errorMessage(err)}`;
    }
    if (cropFallbackReason) {
        // console.warn (not debugLog) so this is visible in the service worker console
        // even with extension debug logging off — a selector screenshot that silently
        // returns the whole viewport looks like the feature simply does not work.
        console.warn(`${KABOOM_LOG_PREFIX} screenshot(selector=${selector}): returning uncropped viewport — ${cropFallbackReason}`);
        debugLog(DebugCategory.CAPTURE, 'Selector crop fell back to full viewport', {
            selector,
            reason: cropFallbackReason
        });
    }
    // The crop covers the element's own CSS rect; the uncropped fallback covers
    // whatever the capture path photographed. Reporting the element rect for an image
    // that is actually the whole viewport would scale every coordinate by the ratio
    // between them, so the kind follows what was really produced.
    const cropped = outUrl !== dataUrl;
    const delivery = await describeCapture(ctx.tabId, outUrl, cropped ? 'element' : 'viewport', cropped ? { x: rect.x, y: rect.y, width: rect.width, height: rect.height } : capture.covered_css_region);
    const ok = await postScreenshot(outUrl, tab.url, ctx.query.id, delivery);
    if (!ok) {
        ctx.sendResult({ error: 'screenshot_upload_failed', message: 'Server rejected screenshot' });
    }
}
async function captureFullPageOverCDP(ctx, options, sessions) {
    const { format, quality, hintedHeight } = options;
    const lease = await sessions.acquire(ctx.tabId);
    try {
        const metrics = (await lease.send('Page.getLayoutMetrics', {}));
        const contentSize = metrics.cssContentSize ||
            metrics.contentSize || { width: DEFAULT_CAPTURE_WIDTH, height: DEFAULT_CAPTURE_HEIGHT };
        const { width: captureWidth, height: captureHeight } = computeFullPageCaptureDimensions(contentSize.width, contentSize.height, hintedHeight);
        await lease.send('Emulation.setDeviceMetricsOverride', {
            width: captureWidth,
            height: captureHeight,
            deviceScaleFactor: 1,
            mobile: false
        });
        try {
            // Brief pause for layout reflow after viewport resize
            await delay(150);
            const screenshotResult = (await lease.send('Page.captureScreenshot', {
                format,
                quality: format === 'jpeg' ? quality : undefined,
                captureBeyondViewport: true,
                clip: { x: 0, y: 0, width: captureWidth, height: captureHeight, scale: 1 }
            }));
            const mimeType = format === 'png' ? 'image/png' : 'image/jpeg';
            recordScreenshot(ctx.tabId);
            return {
                dataUrl: `data:${mimeType};base64,${screenshotResult.data}`,
                captureWidth,
                captureHeight
            };
        }
        finally {
            try {
                // Clear the override even when capture fails, or the tab keeps the forced viewport.
                await lease.send('Emulation.clearDeviceMetricsOverride', {});
            }
            catch {
                // EXPECTED_ABSENCE: optional enrichment can normally fail while the primary
                // operation keeps a valid fallback; logging it would misleadingly report fallback as failure.
                /* best effort */
            }
        }
    }
    finally {
        lease.release();
    }
}
/**
 * Post a full-page image with the frame that makes it addressable.
 *
 * The metrics are read AFTER the containers are restored and the device-metrics
 * override is cleared, because a document-origin image maps to viewport coordinates
 * through the scroll offset the caller will actually act at — not the one that was
 * in force while the page was temporarily resized to its own full height.
 */
async function postFullPage(ctx, tab, image) {
    const metrics = await readViewportMetrics(ctx.tabId);
    const covered = metrics
        ? {
            x: -metrics.scroll_x,
            y: -metrics.scroll_y,
            width: image.captureWidth,
            height: image.captureHeight
        }
        : null;
    const delivery = await describeCaptureWith(metrics, image.dataUrl, 'full_page', covered);
    if (!(await postScreenshot(image.dataUrl, tab.url, ctx.query.id, delivery))) {
        ctx.sendResult({ error: 'screenshot_upload_failed', message: 'Server rejected screenshot' });
    }
}
async function captureFullPage(ctx, tab, format, quality) {
    // Step 1: Expand scrollable containers in the page
    let hintedHeight = 0;
    try {
        const expansionResult = await chrome.scripting.executeScript({
            target: { tabId: ctx.tabId },
            world: 'MAIN',
            func: screenshotExpandContainers
        });
        const expansionMeta = expansionResult[0]?.result;
        hintedHeight = typeof expansionMeta?.content_height_hint === 'number' ? expansionMeta.content_height_hint : 0;
    }
    catch (err) {
        // Best effort only: continue with CDP dimensions if expansion script cannot run.
        debugLog(DebugCategory.CAPTURE, 'Full-page expansion script failed; using CDP layout metrics only', {
            error: errorMessage(err)
        });
    }
    const sessions = cdpSessions();
    if (!sessions) {
        debugLog(DebugCategory.CAPTURE, 'chrome.debugger unavailable; using viewport capture', {});
        await captureAndPostViewport(ctx, tab, format, quality);
        return;
    }
    let image = null;
    try {
        // Step 2: Capture over a lease on the tab's shared CDP session
        image = await captureFullPageOverCDP(ctx, { format, quality, hintedHeight }, sessions);
    }
    catch (err) {
        // Full-page CDP failed — fall back to a viewport capture, which is itself CDP-first
        debugLog(DebugCategory.CAPTURE, 'Full-page CDP failed, falling back to viewport capture', {
            error: errorMessage(err)
        });
    }
    finally {
        // Step 8: Always restore containers
        await chrome.scripting
            .executeScript({
            target: { tabId: ctx.tabId },
            world: 'MAIN',
            func: screenshotRestoreContainers
        })
            .catch(() => {
            // EXPECTED_ABSENCE: a detached screenshot target is normal after capture;
            // logging restoration failure would misleadingly mark the completed capture failed.
        });
    }
    // Delivery happens after restoration so the frame describes the page the caller
    // will act on rather than the temporarily-expanded one it was photographed from.
    if (!image) {
        await captureAndPostViewport(ctx, tab, format, quality);
        return;
    }
    await postFullPage(ctx, tab, image);
}
// =============================================================================
// WATERFALL
// =============================================================================
registerCommand('waterfall', async (ctx) => {
    debugLog(DebugCategory.CAPTURE, 'Handling waterfall query', { queryId: ctx.query.id, tabId: ctx.tabId });
    try {
        const tab = await chrome.tabs.get(ctx.tabId);
        debugLog(DebugCategory.CAPTURE, 'Got tab for waterfall', { tabId: ctx.tabId, url: tab.url });
        const result = (await chrome.tabs.sendMessage(ctx.tabId, {
            type: 'get_network_waterfall'
        }));
        const entries = result?.entries || [];
        debugLog(DebugCategory.CAPTURE, 'Waterfall result from content script', {
            entries: entries.length,
            error: result?.error
        });
        ctx.sendResult({
            entries,
            ...(result?.error ? { error: result.error, message: result.message || 'Waterfall bridge failed' } : {}),
            page_url: tab.url || '',
            count: entries.length
        });
        debugLog(DebugCategory.CAPTURE, 'Posted waterfall result', { queryId: ctx.query.id });
    }
    catch (err) {
        debugLog(DebugCategory.CAPTURE, 'Waterfall query error', {
            queryId: ctx.query.id,
            error: errorMessage(err)
        });
        ctx.sendResult({
            error: 'waterfall_query_failed',
            message: errorMessage(err, 'Failed to fetch network waterfall'),
            entries: []
        });
    }
});
// =============================================================================
// PAGE INFO
// =============================================================================
registerCommand('page_info', async (ctx) => {
    try {
        const tab = await chrome.tabs.get(ctx.tabId);
        ctx.sendResult({
            url: tab.url,
            title: tab.title,
            favicon: tab.favIconUrl,
            status: tab.status,
            viewport: {
                width: tab.width,
                height: tab.height
            }
        });
    }
    catch (err) {
        ctx.sendResult({
            error: 'page_info_failed',
            message: errorMessage(err) || `Failed to get tab ${ctx.tabId}`
        });
    }
});
// =============================================================================
// TABS
// =============================================================================
registerCommand('tabs', async (ctx) => {
    try {
        const allTabs = await chrome.tabs.query({});
        const tabsList = allTabs.map((tab) => ({
            id: tab.id,
            url: tab.url,
            title: tab.title,
            active: tab.active,
            windowId: tab.windowId,
            index: tab.index
        }));
        ctx.sendResult({ tabs: tabsList });
    }
    catch (err) {
        ctx.sendResult({
            error: 'tabs_query_failed',
            message: errorMessage(err, 'Failed to query tabs')
        });
    }
});
// =============================================================================
// PAGE INVENTORY (#318)
// =============================================================================
registerCommand('page_inventory', async (ctx) => {
    try {
        // 1. Get tab info (page metadata)
        const tab = await chrome.tabs.get(ctx.tabId);
        // 2. Run list_interactive via chrome.scripting in the page
        const interactiveResults = await chrome.scripting.executeScript({
            target: { tabId: ctx.tabId, allFrames: true },
            world: 'MAIN',
            func: domPrimitiveListInteractive,
            args: ['']
        });
        // Merge interactive elements from all frames (up to 100)
        const { elements: cappedElements, firstError } = collectCommandElements(interactiveResults, 100);
        const finalElements = selectCommandElements(cappedElements, ctx.params);
        const payload = {
            ...commandPageMetadata(tab),
            interactive_elements: finalElements,
            interactive_count: finalElements.length,
            total_candidates: cappedElements.length
        };
        if (firstError && finalElements.length === 0) {
            payload.interactive_error = firstError;
        }
        ctx.sendResult(payload);
    }
    catch (err) {
        const message = errorMessage(err, 'Page inventory failed');
        ctx.sendResult({
            error: 'page_inventory_failed',
            message
        });
    }
});
//# sourceMappingURL=observe.js.map