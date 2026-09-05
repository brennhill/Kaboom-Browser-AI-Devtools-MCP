/**
 * Purpose: Measure a captured screenshot against the page it came from and deliver both to the daemon.
 * Docs: docs/features/feature/observe/index.md
 */
import { type CaptureKind, type CoveredCssRegion, type PageViewportMetrics } from '../../../lib/screenshot/coordinate-frame.js';
import type { WireCoordinateFrame } from '../../../types/wire/wire-screenshot.js';
/**
 * What a capture reports about itself besides the pixels: the frame that makes the
 * image addressable, or the reason there is none.
 */
export interface ScreenshotDelivery {
    readonly frame: WireCoordinateFrame | null;
    readonly frameError: string | null;
}
/** Post screenshot data to server for saving and query resolution. */
export declare function postScreenshot(dataUrl: string, pageUrl: string | undefined, queryId: string, delivery: ScreenshotDelivery): Promise<boolean>;
/**
 * Read the page's own measurements, or report why they are unavailable.
 *
 * A frame built without them would have to assume the viewport, and an assumed
 * viewport is a scale that is wrong by whatever the assumption missed. The caller
 * ships `coordinate_frame_error` instead, so `observe` answers "no frame, because
 * the probe could not run" rather than handing back numbers that misplace clicks.
 */
export declare function readViewportMetrics(tabId: number): Promise<PageViewportMetrics | null>;
/**
 * Measure the image and the page, and build the frame that ties them together.
 *
 * Never throws and never guesses: every failure becomes a reason string that ships
 * with the screenshot. The image is still returned in all cases — a capture the
 * agent can look at but not click is worth more than no capture at all.
 */
export declare function describeCapture(tabId: number, dataUrl: string, kind: CaptureKind, covered: CoveredCssRegion | null): Promise<ScreenshotDelivery>;
/** describeCapture for a caller that already read the page's metrics. */
export declare function describeCaptureWith(metrics: PageViewportMetrics | null, dataUrl: string, kind: CaptureKind, covered: CoveredCssRegion | null): Promise<ScreenshotDelivery>;
//# sourceMappingURL=screenshot-delivery.d.ts.map