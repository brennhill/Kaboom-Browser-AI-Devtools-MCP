/**
 * Purpose: Command handlers for the observe MCP tool (screenshot capture, network waterfall, page info, tab listing).
 * Docs: docs/features/feature/observe/index.md
 */

// observe.ts — Command handlers for the observe MCP tool.
// Handles: screenshot, waterfall, page_info, tabs.

import { getServerUrl } from '../runtime-state/settings-state.js'
import { DebugCategory, debugLog } from '../debug.js'
import { recordScreenshot } from '../caches/cache-limits.js'
import { domPrimitiveListInteractive } from '../dom/primitives/dom-primitives-list-interactive.js'
import { registerCommand } from './registry.js'
import { collectCommandElements, commandPageMetadata, selectCommandElements } from './results/element-results.js'
import { errorMessage } from '../../lib/error-utils.js'
import { KABOOM_LOG_PREFIX } from '../../lib/brand.js'
import { delay } from '../../lib/timeout-utils.js'
import { postDaemonJSON } from '../../lib/daemon-http.js'
import { captureTabImage } from '../ui/tracked-tab-state.js'
import { cdpSessions } from '../dom/cdp/cdp-session.js'

// =============================================================================
// SCREENSHOT
// =============================================================================
const MAX_CAPTURE_HEIGHT = 16384 // Chrome max texture size
const MAX_CAPTURE_WIDTH = 16384 // Chrome max texture size
const DEFAULT_CAPTURE_WIDTH = 1280
const DEFAULT_CAPTURE_HEIGHT = 720

/**
 * Self-contained function injected via chrome.scripting.executeScript.
 * Temporarily expands scrollable containers so CDP captures full content.
 * Stores original styles in data attributes for restoration.
 */
export function screenshotExpandContainers(): { expanded: number; content_height_hint: number } {
  let count = 0
  let contentHeightHint = Math.max(document.documentElement?.scrollHeight || 0, document.body?.scrollHeight || 0)
  function tryExpand(el: HTMLElement): void {
    const style = getComputedStyle(el)
    const oy = style.overflowY || ''
    const ov = style.overflow || ''
    const isScrollable =
      oy === 'auto' ||
      oy === 'scroll' ||
      oy === 'hidden' ||
      oy === 'clip' ||
      ov === 'auto' ||
      ov === 'scroll' ||
      ov === 'hidden' ||
      ov === 'clip'
    if (isScrollable && el.scrollHeight > el.clientHeight + 1) {
      const targetHeight = Math.max(el.scrollHeight, el.clientHeight)
      el.setAttribute(
        'data-kaboom-fpx',
        JSON.stringify({
          o: el.style.overflow,
          oy: el.style.overflowY,
          ox: el.style.overflowX,
          h: el.style.height,
          n: el.style.minHeight,
          m: el.style.maxHeight,
          f: el.style.flex,
          c: el.style.contain
        })
      )
      el.style.overflow = 'visible'
      el.style.overflowY = 'visible'
      el.style.overflowX = 'visible'
      el.style.height = `${targetHeight}px`
      el.style.minHeight = `${targetHeight}px`
      el.style.maxHeight = 'none'
      el.style.flex = 'none'
      el.style.contain = 'none'
      const top = (el.getBoundingClientRect().top || 0) + (window.scrollY || window.pageYOffset || 0)
      contentHeightHint = Math.max(contentHeightHint, top + targetHeight)
      count++
    }
  }
  tryExpand(document.documentElement)
  tryExpand(document.body)
  const all = document.body.querySelectorAll('*')
  for (let i = 0; i < all.length; i++) {
    if (all[i] instanceof HTMLElement) tryExpand(all[i] as HTMLElement)
  }
  return { expanded: count, content_height_hint: Math.ceil(contentHeightHint) }
}

/** Self-contained: restore containers after full-page capture. */
export function screenshotRestoreContainers(): void {
  function tryRestore(el: HTMLElement): void {
    const raw = el.getAttribute('data-kaboom-fpx')
    if (!raw) return
    try {
      const s = JSON.parse(raw) as {
        o?: string
        oy?: string
        ox?: string
        h?: string
        n?: string
        m?: string
        f?: string
        c?: string
      }
      el.style.overflow = s.o || ''
      el.style.overflowY = s.oy || ''
      el.style.overflowX = s.ox || ''
      el.style.height = s.h || ''
      el.style.minHeight = s.n || ''
      el.style.maxHeight = s.m || ''
      el.style.flex = s.f || ''
      el.style.contain = s.c || ''
    } catch {
      // EXPECTED_ABSENCE: optional enrichment can normally fail while the primary
      // operation keeps a valid fallback; logging it would misleadingly report fallback as failure.
      /* ignore parse errors */
    }
    el.removeAttribute('data-kaboom-fpx')
  }
  tryRestore(document.documentElement)
  const all = document.querySelectorAll('[data-kaboom-fpx]')
  for (let i = 0; i < all.length; i++) {
    tryRestore(all[i] as HTMLElement)
  }
}

/** Derive bounded screenshot dimensions with fallback defaults and optional expanded-content hint. */
export function computeFullPageCaptureDimensions(
  contentWidth: number,
  contentHeight: number,
  hintedHeight: number
): { width: number; height: number } {
  const safeWidth = Number.isFinite(contentWidth) && contentWidth > 0 ? Math.ceil(contentWidth) : DEFAULT_CAPTURE_WIDTH
  const safeHeight =
    Number.isFinite(contentHeight) && contentHeight > 0 ? Math.ceil(contentHeight) : DEFAULT_CAPTURE_HEIGHT
  const safeHint = Number.isFinite(hintedHeight) && hintedHeight > 0 ? Math.ceil(hintedHeight) : 0
  return {
    width: Math.max(1, Math.min(safeWidth, MAX_CAPTURE_WIDTH)),
    height: Math.max(1, Math.min(Math.max(safeHeight, safeHint), MAX_CAPTURE_HEIGHT))
  }
}

/** Post screenshot data to server for saving and query resolution. */
async function postScreenshot(dataUrl: string, pageUrl: string | undefined, queryId: string): Promise<boolean> {
  try {
    const response = await postDaemonJSON(`${getServerUrl()}/screenshots`, {
      data_url: dataUrl,
      url: pageUrl,
      query_id: queryId
    })
    return response.ok
  } catch {
    // EXPECTED_ABSENCE: daemon disconnect during optional delivery is normal; logging would duplicate the caller's failure result.
    return false
  }
}

interface ScreenshotContext {
  tabId: number
  query: { id: string }
  sendResult: (result: unknown) => void
}

/**
 * Capture the viewport and post it. Runs entirely in the background: `captureTabImage`
 * takes a CDP lease rather than activating the tab, so an observe(screenshot) while the
 * user is reading a different tab no longer yanks their window away.
 */
async function captureAndPostViewport(
  ctx: ScreenshotContext,
  tab: chrome.tabs.Tab,
  format: 'png' | 'jpeg',
  quality: number
): Promise<void> {
  const dataUrl = await captureTabImage(ctx.tabId, tab.windowId, { format, quality })
  recordScreenshot(ctx.tabId)
  if (!(await postScreenshot(dataUrl, tab.url, ctx.query.id))) {
    ctx.sendResult({ error: 'screenshot_upload_failed', message: 'Server rejected screenshot' })
  }
}

registerCommand('screenshot', async (ctx) => {
  const format = ctx.params.format === 'png' ? 'png' : 'jpeg'
  const quality = typeof ctx.params.quality === 'number' ? ctx.params.quality : 80
  const fullPage = ctx.params.full_page === true

  try {
    const tab = await chrome.tabs.get(ctx.tabId)

    if (fullPage) {
      await captureFullPage(ctx, tab, format, quality)
      return
    }

    // #597: selector-scoped capture. Honor the advertised `selector` param
    // (previously silently ignored) by scrolling the element into view and
    // cropping the viewport capture to it.
    const selector = typeof ctx.params.selector === 'string' ? ctx.params.selector.trim() : ''
    if (selector) {
      await captureElement(ctx, tab, selector, format, quality)
      return
    }

    await captureAndPostViewport(ctx, tab, format, quality)
    // No sendResult needed — server resolves the query via query_id
  } catch (err) {
    ctx.sendResult({
      error: 'screenshot_failed',
      message: errorMessage(err, 'Failed to capture screenshot')
    })
  }
})

// =============================================================================
// SELECTOR-SCOPED SCREENSHOT (#597)
// =============================================================================

interface ScreenshotElementRect {
  found: boolean
  x: number
  y: number
  width: number
  height: number
  dpr: number
}

/**
 * Runs in the page's MAIN world. Scrolls the matched element into view —
 * `scrollIntoView` honors *every* scrollable ancestor, including nested
 * `overflow:auto` containers, which is the exact case #597 reported — then
 * returns its viewport-relative rect and the device pixel ratio.
 */
function screenshotResolveElementRect(selector: string): ScreenshotElementRect {
  const el = document.querySelector(selector)
  if (!el) return { found: false, x: 0, y: 0, width: 0, height: 0, dpr: 1 }
  // Instant (default behavior) so layout settles before we read the rect.
  el.scrollIntoView({ block: 'center', inline: 'center' })
  const r = el.getBoundingClientRect()
  return {
    found: true,
    x: r.left,
    y: r.top,
    width: r.width,
    height: r.height,
    dpr: window.devicePixelRatio || 1
  }
}

/**
 * Compute the source crop rectangle (in image/device pixels) for an element's
 * CSS-pixel viewport rect. The capture is clipped to the visual viewport and
 * scaled by the page's device pixel ratio, and the rect is viewport-relative
 * CSS pixels — so the crop is `rect * dpr`, clamped to the image bounds.
 * Returns null when there is nothing to crop (non-positive size, or the element
 * lies outside the image).
 */
export function computeElementCropRect(
  rect: { x: number; y: number; width: number; height: number },
  dpr: number,
  imageWidth: number,
  imageHeight: number
): { sx: number; sy: number; sw: number; sh: number } | null {
  if (rect.width <= 0 || rect.height <= 0) return null
  const scale = dpr > 0 ? dpr : 1
  const sx = Math.max(0, Math.round(rect.x * scale))
  const sy = Math.max(0, Math.round(rect.y * scale))
  const sw = Math.min(Math.round(rect.width * scale), imageWidth - sx)
  const sh = Math.min(Math.round(rect.height * scale), imageHeight - sy)
  if (sw <= 0 || sh <= 0) return null
  return { sx, sy, sw, sh }
}

/**
 * Decode a base64 `data:` URL into a Blob without fetch(). Service workers do not
 * reliably allow `fetch('data:...')`, so this keeps the crop path self-contained.
 */
export function dataUrlToBlob(dataUrl: string): Blob {
  const comma = dataUrl.indexOf(',')
  if (!dataUrl.startsWith('data:') || comma === -1) {
    throw new Error('not a data URL')
  }
  const header = dataUrl.slice(5, comma)
  if (!header.includes(';base64')) {
    throw new Error('data URL is not base64-encoded')
  }
  const mime = header.split(';')[0] || 'application/octet-stream'
  const binary = atob(dataUrl.slice(comma + 1))
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return new Blob([bytes], { type: mime })
}

async function blobToDataUrl(blob: Blob): Promise<string> {
  const bytes = new Uint8Array(await blob.arrayBuffer())
  let binary = ''
  const chunk = 0x8000
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk))
  }
  return `data:${blob.type};base64,${btoa(binary)}`
}

/**
 * Crop a captured data URL to the element rect via OffscreenCanvas.
 *
 * Always reports WHY it could not crop. A bare `return null` here made #597 look
 * fixed while the caller silently posted the uncropped viewport — the failure was
 * invisible in unit tests (which never run a service worker) and in the product.
 * Any user-visible fallback must carry a reason.
 */
async function cropDataUrlToRect(
  dataUrl: string,
  rect: ScreenshotElementRect,
  format: 'png' | 'jpeg',
  quality: number
): Promise<{ dataUrl: string } | { reason: string }> {
  if (typeof OffscreenCanvas === 'undefined') return { reason: 'offscreencanvas_unavailable' }
  if (typeof createImageBitmap === 'undefined') return { reason: 'createimagebitmap_unavailable' }

  // Decode the data: URL by hand rather than via fetch(). MV3 service workers
  // restrict fetching data: URLs, and a throw there is what silently disabled the
  // whole crop path (#597) — the caller just posted the uncropped viewport.
  // atob + Uint8Array has no such restriction.
  let blob: Blob
  try {
    blob = dataUrlToBlob(dataUrl)
  } catch (err) {
    return { reason: `datauri_decode_failed: ${errorMessage(err)}` }
  }

  const bitmap = await createImageBitmap(blob)
  try {
    const crop = computeElementCropRect(rect, rect.dpr, bitmap.width, bitmap.height)
    if (!crop) {
      return {
        reason:
          `empty_crop_rect (element ${rect.width}x${rect.height} @ ${rect.x},${rect.y} ` +
          `dpr=${rect.dpr} vs image ${bitmap.width}x${bitmap.height})`
      }
    }
    const canvas = new OffscreenCanvas(crop.sw, crop.sh)
    const c = canvas.getContext('2d')
    if (!c) return { reason: 'no_2d_context' }
    c.drawImage(bitmap, crop.sx, crop.sy, crop.sw, crop.sh, 0, 0, crop.sw, crop.sh)
    const mime = format === 'png' ? 'image/png' : 'image/jpeg'
    const outBlob = await canvas.convertToBlob({
      type: mime,
      quality: format === 'jpeg' ? quality / 100 : undefined
    })
    return { dataUrl: await blobToDataUrl(outBlob) }
  } finally {
    bitmap.close?.()
  }
}

/**
 * Selector-scoped screenshot (#597): scroll the element into view (honoring
 * nested scroll containers) and crop the viewport capture to it. Falls back to
 * the (correctly scrolled) uncropped viewport if the crop cannot be produced,
 * so the capture never fails outright.
 */
async function captureElement(
  ctx: ScreenshotContext,
  tab: chrome.tabs.Tab,
  selector: string,
  format: 'png' | 'jpeg',
  quality: number
): Promise<void> {
  let rect: ScreenshotElementRect | null = null
  try {
    const res = await chrome.scripting.executeScript({
      target: { tabId: ctx.tabId },
      world: 'MAIN',
      func: screenshotResolveElementRect,
      args: [selector]
    })
    rect = (res[0]?.result as ScreenshotElementRect | undefined) ?? null
  } catch (err) {
    debugLog(DebugCategory.CAPTURE, 'Selector resolve script failed', { error: errorMessage(err) })
  }

  if (!rect || !rect.found) {
    ctx.sendResult({ error: 'element_not_found', message: `No element matched selector: ${selector}` })
    return
  }

  // Let the (instant) scroll repaint before the compositor raster.
  await delay(120)
  const dataUrl = await captureTabImage(ctx.tabId, tab.windowId, { format, quality })
  recordScreenshot(ctx.tabId)

  let outUrl = dataUrl
  let cropFallbackReason: string | null = null
  try {
    const cropped = await cropDataUrlToRect(dataUrl, rect, format, quality)
    if ('dataUrl' in cropped) {
      outUrl = cropped.dataUrl
    } else {
      cropFallbackReason = cropped.reason
    }
  } catch (err) {
    cropFallbackReason = `crop_threw: ${errorMessage(err)}`
  }

  if (cropFallbackReason) {
    // console.warn (not debugLog) so this is visible in the service worker console
    // even with extension debug logging off — a selector screenshot that silently
    // returns the whole viewport looks like the feature simply does not work.
    console.warn(
      `${KABOOM_LOG_PREFIX} screenshot(selector=${selector}): returning uncropped viewport — ${cropFallbackReason}`
    )
    debugLog(DebugCategory.CAPTURE, 'Selector crop fell back to full viewport', {
      selector,
      reason: cropFallbackReason
    })
  }

  const ok = await postScreenshot(outUrl, tab.url, ctx.query.id)
  if (!ok) {
    ctx.sendResult({ error: 'screenshot_upload_failed', message: 'Server rejected screenshot' })
  }
}

/** Full-page screenshot via CDP with scrollable container expansion (#363). */

/** Options for one full-page CDP capture. Grouped so the helper stays inside the arg budget. */
interface FullPageCaptureOptions {
  format: 'png' | 'jpeg'
  quality: number
  hintedHeight: number
}

/**
 * Resize the viewport to the full content box, capture, and always restore the override.
 *
 * Split out of captureFullPage so each function owns one concern: the caller handles
 * container expansion and the viewport fallback, this handles the CDP capture itself.
 */
async function captureFullPageOverCDP(
  ctx: ScreenshotContext,
  tab: chrome.tabs.Tab,
  options: FullPageCaptureOptions,
  sessions: NonNullable<ReturnType<typeof cdpSessions>>
): Promise<void> {
  const { format, quality, hintedHeight } = options
  const lease = await sessions.acquire(ctx.tabId)
  try {
    const metrics = (await lease.send('Page.getLayoutMetrics', {})) as {
      cssContentSize?: { width: number; height: number }
      contentSize?: { width: number; height: number }
    }
    const contentSize = metrics.cssContentSize ||
      metrics.contentSize || { width: DEFAULT_CAPTURE_WIDTH, height: DEFAULT_CAPTURE_HEIGHT }
    const { width: captureWidth, height: captureHeight } = computeFullPageCaptureDimensions(
      contentSize.width,
      contentSize.height,
      hintedHeight
    )

    await lease.send('Emulation.setDeviceMetricsOverride', {
      width: captureWidth,
      height: captureHeight,
      deviceScaleFactor: 1,
      mobile: false
    })

    try {
      // Brief pause for layout reflow after viewport resize
      await delay(150)
      const screenshotResult = (await lease.send('Page.captureScreenshot', {
        format,
        quality: format === 'jpeg' ? quality : undefined,
        captureBeyondViewport: true,
        clip: { x: 0, y: 0, width: captureWidth, height: captureHeight, scale: 1 }
      })) as { data: string }

      const mimeType = format === 'png' ? 'image/png' : 'image/jpeg'
      recordScreenshot(ctx.tabId)
      const ok = await postScreenshot(`data:${mimeType};base64,${screenshotResult.data}`, tab.url, ctx.query.id)
      if (!ok) {
        ctx.sendResult({ error: 'screenshot_upload_failed', message: 'Server rejected screenshot' })
      }
    } finally {
      try {
        // Clear the override even when capture fails, or the tab keeps the forced viewport.
        await lease.send('Emulation.clearDeviceMetricsOverride', {})
      } catch {
        // EXPECTED_ABSENCE: optional enrichment can normally fail while the primary
        // operation keeps a valid fallback; logging it would misleadingly report fallback as failure.
        /* best effort */
      }
    }
  } finally {
    lease.release()
  }
}

async function captureFullPage(
  ctx: ScreenshotContext,
  tab: chrome.tabs.Tab,
  format: 'png' | 'jpeg',
  quality: number
): Promise<void> {
  // Step 1: Expand scrollable containers in the page
  let hintedHeight = 0
  try {
    const expansionResult = await chrome.scripting.executeScript({
      target: { tabId: ctx.tabId },
      world: 'MAIN',
      func: screenshotExpandContainers
    })
    const expansionMeta = expansionResult[0]?.result as { content_height_hint?: number } | undefined
    hintedHeight = typeof expansionMeta?.content_height_hint === 'number' ? expansionMeta.content_height_hint : 0
  } catch (err) {
    // Best effort only: continue with CDP dimensions if expansion script cannot run.
    debugLog(DebugCategory.CAPTURE, 'Full-page expansion script failed; using CDP layout metrics only', {
      error: errorMessage(err)
    })
  }

  const sessions = cdpSessions()
  if (!sessions) {
    debugLog(DebugCategory.CAPTURE, 'chrome.debugger unavailable; using viewport capture', {})
    await captureAndPostViewport(ctx, tab, format, quality)
    return
  }

  try {
    // Step 2: Capture over a lease on the tab's shared CDP session
    await captureFullPageOverCDP(ctx, tab, { format, quality, hintedHeight }, sessions)
  } catch (err) {
    // Full-page CDP failed — fall back to a viewport capture, which is itself CDP-first
    debugLog(DebugCategory.CAPTURE, 'Full-page CDP failed, falling back to viewport capture', {
      error: errorMessage(err)
    })
    await captureAndPostViewport(ctx, tab, format, quality)
  } finally {
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
      })
  }
}

// =============================================================================
// WATERFALL
// =============================================================================

registerCommand('waterfall', async (ctx) => {
  debugLog(DebugCategory.CAPTURE, 'Handling waterfall query', { queryId: ctx.query.id, tabId: ctx.tabId })
  try {
    const tab = await chrome.tabs.get(ctx.tabId)
    debugLog(DebugCategory.CAPTURE, 'Got tab for waterfall', { tabId: ctx.tabId, url: tab.url })
    const result = (await chrome.tabs.sendMessage(ctx.tabId, {
      type: 'get_network_waterfall'
    })) as { entries?: unknown[]; error?: string; message?: string }
    const entries = result?.entries || []
    debugLog(DebugCategory.CAPTURE, 'Waterfall result from content script', {
      entries: entries.length,
      error: result?.error
    })

    ctx.sendResult({
      entries,
      ...(result?.error ? { error: result.error, message: result.message || 'Waterfall bridge failed' } : {}),
      page_url: tab.url || '',
      count: entries.length
    })
    debugLog(DebugCategory.CAPTURE, 'Posted waterfall result', { queryId: ctx.query.id })
  } catch (err) {
    debugLog(DebugCategory.CAPTURE, 'Waterfall query error', {
      queryId: ctx.query.id,
      error: errorMessage(err)
    })
    ctx.sendResult({
      error: 'waterfall_query_failed',
      message: errorMessage(err, 'Failed to fetch network waterfall'),
      entries: []
    })
  }
})

// =============================================================================
// PAGE INFO
// =============================================================================

registerCommand('page_info', async (ctx) => {
  try {
    const tab = await chrome.tabs.get(ctx.tabId)
    ctx.sendResult({
      url: tab.url,
      title: tab.title,
      favicon: tab.favIconUrl,
      status: tab.status,
      viewport: {
        width: tab.width,
        height: tab.height
      }
    })
  } catch (err) {
    ctx.sendResult({
      error: 'page_info_failed',
      message: errorMessage(err) || `Failed to get tab ${ctx.tabId}`
    })
  }
})

// =============================================================================
// TABS
// =============================================================================

registerCommand('tabs', async (ctx) => {
  try {
    const allTabs = await chrome.tabs.query({})
    const tabsList = allTabs.map((tab) => ({
      id: tab.id,
      url: tab.url,
      title: tab.title,
      active: tab.active,
      windowId: tab.windowId,
      index: tab.index
    }))
    ctx.sendResult({ tabs: tabsList })
  } catch (err) {
    ctx.sendResult({
      error: 'tabs_query_failed',
      message: errorMessage(err, 'Failed to query tabs')
    })
  }
})

// =============================================================================
// PAGE INVENTORY (#318)
// =============================================================================

registerCommand('page_inventory', async (ctx) => {
  try {
    // 1. Get tab info (page metadata)
    const tab = await chrome.tabs.get(ctx.tabId)

    // 2. Run list_interactive via chrome.scripting in the page
    const interactiveResults = await chrome.scripting.executeScript({
      target: { tabId: ctx.tabId, allFrames: true },
      world: 'MAIN',
      func: domPrimitiveListInteractive,
      args: ['']
    })

    // Merge interactive elements from all frames (up to 100)
    const { elements: cappedElements, firstError } = collectCommandElements(interactiveResults, 100)

    const finalElements = selectCommandElements(cappedElements, ctx.params)

    const payload: Record<string, unknown> = {
      ...commandPageMetadata(tab),
      interactive_elements: finalElements,
      interactive_count: finalElements.length,
      total_candidates: cappedElements.length
    }

    if (firstError && finalElements.length === 0) {
      payload.interactive_error = firstError
    }

    ctx.sendResult(payload)
  } catch (err) {
    const message = errorMessage(err, 'Page inventory failed')
    ctx.sendResult({
      error: 'page_inventory_failed',
      message
    })
  }
})
