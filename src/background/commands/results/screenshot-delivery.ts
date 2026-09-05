/**
 * Purpose: Measure a captured screenshot against the page it came from and deliver both to the daemon.
 * Docs: docs/features/feature/observe/index.md
 */

import { getServerUrl } from '../../runtime-state/settings-state.js'
import { DebugCategory, debugLog } from '../../debug.js'
import { errorMessage } from '../../../lib/error-utils.js'
import { postDaemonJSON } from '../../../lib/daemon-http.js'
import {
  buildCoordinateFrame,
  readPageViewportMetrics,
  type CaptureKind,
  type CoveredCssRegion,
  type PageViewportMetrics
} from '../../../lib/screenshot/coordinate-frame.js'
import { measureImageSize } from '../../../lib/screenshot/image-size.js'
import type { WireCoordinateFrame } from '../../../types/wire/wire-screenshot.js'

/**
 * What a capture reports about itself besides the pixels: the frame that makes the
 * image addressable, or the reason there is none.
 */
export interface ScreenshotDelivery {
  readonly frame: WireCoordinateFrame | null
  readonly frameError: string | null
}

/** Post screenshot data to server for saving and query resolution. */
export async function postScreenshot(
  dataUrl: string,
  pageUrl: string | undefined,
  queryId: string,
  delivery: ScreenshotDelivery
): Promise<boolean> {
  try {
    const response = await postDaemonJSON(`${getServerUrl()}/screenshots`, {
      data_url: dataUrl,
      url: pageUrl,
      query_id: queryId,
      ...(delivery.frame ? { coordinate_frame: delivery.frame } : {}),
      ...(delivery.frameError ? { coordinate_frame_error: delivery.frameError } : {})
    })
    return response.ok
  } catch {
    // EXPECTED_ABSENCE: daemon disconnect during optional delivery is normal; logging would duplicate the caller's failure result.
    return false
  }
}

/**
 * Read the page's own measurements, or report why they are unavailable.
 *
 * A frame built without them would have to assume the viewport, and an assumed
 * viewport is a scale that is wrong by whatever the assumption missed. The caller
 * ships `coordinate_frame_error` instead, so `observe` answers "no frame, because
 * the probe could not run" rather than handing back numbers that misplace clicks.
 */
export async function readViewportMetrics(tabId: number): Promise<PageViewportMetrics | null> {
  try {
    const res = await chrome.scripting.executeScript({
      target: { tabId },
      world: 'MAIN',
      func: readPageViewportMetrics
    })
    return (res[0]?.result as PageViewportMetrics | undefined) ?? null
  } catch (err) {
    debugLog(DebugCategory.CAPTURE, 'Viewport metrics probe failed; screenshot ships without a coordinate frame', {
      tab_id: tabId,
      error: errorMessage(err)
    })
    return null
  }
}

/**
 * Measure the image and the page, and build the frame that ties them together.
 *
 * Never throws and never guesses: every failure becomes a reason string that ships
 * with the screenshot. The image is still returned in all cases — a capture the
 * agent can look at but not click is worth more than no capture at all.
 */
export async function describeCapture(
  tabId: number,
  dataUrl: string,
  kind: CaptureKind,
  covered: CoveredCssRegion | null
): Promise<ScreenshotDelivery> {
  return describeCaptureWith(await readViewportMetrics(tabId), dataUrl, kind, covered)
}

/** describeCapture for a caller that already read the page's metrics. */
export async function describeCaptureWith(
  metrics: PageViewportMetrics | null,
  dataUrl: string,
  kind: CaptureKind,
  covered: CoveredCssRegion | null
): Promise<ScreenshotDelivery> {
  if (!metrics) return { frame: null, frameError: 'viewport_metrics_unavailable' }

  const measured = await measureImageSize(dataUrl)
  if ('reason' in measured) return { frame: null, frameError: measured.reason }

  const frame = buildCoordinateFrame(kind, metrics, measured.size, covered)
  if (!frame) {
    return {
      frame: null,
      frameError:
        `unmeasurable_region (image ${measured.size.width}x${measured.size.height}, ` +
        `viewport ${metrics.viewport_width}x${metrics.viewport_height})`
    }
  }
  return { frame, frameError: null }
}

