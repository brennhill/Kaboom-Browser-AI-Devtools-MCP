/**
 * Purpose: Build the coordinate frame that ties a captured image's pixels to the CSS coordinates an action accepts.
 * Docs: docs/features/feature/observe/index.md
 */
import type { WireCoordinateFrame } from '../../types/wire/wire-screenshot.js';
/**
 * What the page reports about itself at capture time. Every field is a CSS-pixel
 * measurement read from the page, never inferred.
 */
export interface PageViewportMetrics {
    readonly viewport_width: number;
    readonly viewport_height: number;
    readonly scroll_x: number;
    readonly scroll_y: number;
    readonly document_width: number;
    readonly document_height: number;
    readonly device_pixel_ratio: number;
}
/** The image the caller is actually holding, in real pixels. */
export interface CapturedImageSize {
    readonly width: number;
    readonly height: number;
}
/**
 * The CSS-pixel rectangle the image covers, in VIEWPORT coordinates.
 *
 * This one input is what makes every capture kind the same arithmetic. A viewport
 * capture covers {0, 0, innerWidth, innerHeight}. An element crop covers the
 * element's bounding rect. A zoom region covers the requested rectangle. A
 * full-page image covers the document, whose origin sits `scrollY` above the top
 * of the screen, so its y is negative whenever the page is scrolled.
 */
export interface CoveredCssRegion {
    readonly x: number;
    readonly y: number;
    readonly width: number;
    readonly height: number;
}
/**
 * Runs in the page's MAIN world via chrome.scripting.executeScript.
 *
 * Self-contained by necessity — an injected function cannot close over anything in
 * this module.
 *
 * `innerWidth`/`innerHeight` rather than `visualViewport`: these are the CSS
 * coordinate space that CDP input events are expressed in, and they are the extent
 * `chrome.tabs.captureVisibleTab` photographs (scrollbars included). visualViewport
 * shrinks under pinch-zoom and would make the frame describe a region the image
 * does not cover, which moves every click by the zoom offset.
 */
export declare function readPageViewportMetrics(): PageViewportMetrics;
/** Capture kinds, mirroring internal/screenshotframe.CaptureKinds(). */
export type CaptureKind = 'viewport' | 'full_page' | 'element' | 'region';
/**
 * The CSS region a capture covers, in viewport coordinates.
 *
 * `covered` is what the capture path itself reports it photographed, and it always
 * wins: only that path knows whether it clipped to CDP's cssVisualViewport, to a
 * clamped full-page box, or to an element's rect, and the difference between those
 * and the page's own idea of its viewport is tens of pixels at the far edge of the
 * image. The kind-derived fallbacks below are for the paths that cannot report one
 * — `captureVisibleTab` photographs the visible viewport and nothing else.
 *
 * Kept in one place rather than at the four call sites so a new capture path cannot
 * quietly ship a frame with the wrong origin: the difference between a viewport
 * origin and a document origin is the scroll offset, and getting it wrong misplaces
 * every click on a scrolled page by exactly the amount the user scrolled.
 */
export declare function coveredRegionFor(kind: CaptureKind, metrics: PageViewportMetrics, covered?: CoveredCssRegion | null): CoveredCssRegion;
/**
 * Build the frame, or return null when it cannot be measured.
 *
 * Null is a real answer and the only honest one when an input is missing: a frame
 * assembled from a zero image width would report a scale of Infinity, and a caller
 * would act on it because it looks like a number. `observe` reports the absence
 * (see screenshot_frame_unavailable) instead of shipping a frame that misplaces
 * every click.
 */
export declare function buildCoordinateFrame(kind: CaptureKind, metrics: PageViewportMetrics, image: CapturedImageSize, covered?: CoveredCssRegion | null): WireCoordinateFrame | null;
//# sourceMappingURL=coordinate-frame.d.ts.map