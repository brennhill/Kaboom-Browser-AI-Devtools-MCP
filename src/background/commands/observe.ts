/**
 * Purpose: Command handlers for the observe MCP tool (screenshot capture, network waterfall, page info, tab listing).
 * Docs: docs/features/feature/observe/index.md
 */

// observe.ts — Command handlers for the observe MCP tool.
// Handles: screenshot, waterfall, page_info, tabs.

import { debugLog } from '../index.js'
import { getServerUrl } from '../state.js'
import { DebugCategory } from '../debug.js'
import { recordScreenshot } from '../state-manager.js'
import { domPrimitiveListInteractive } from '../dom-primitives-list-interactive.js'
import { registerCommand } from './registry.js'
import { CDP_VERSION } from '../../lib/constants.js'
import { errorMessage } from '../../lib/error-utils.js'
import { delay } from '../../lib/timeout-utils.js'
import { postDaemonJSON } from '../../lib/daemon-http.js'
import { captureVisibleTabSafe } from '../tab-state.js'

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
    return false
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

    const dataUrl = await captureVisibleTabSafe(ctx.tabId, tab.windowId, {
      format: format as 'jpeg' | 'png',
      quality
    })
    recordScreenshot(ctx.tabId)

    // POST to /screenshots with query_id — server saves file and resolves query directly
    const ok = await postScreenshot(dataUrl, tab.url, ctx.query.id)
    if (!ok) {
      ctx.sendResult({ error: 'screenshot_upload_failed', message: 'Server rejected screenshot' })
    }
    // No sendResult needed — server resolves the query via query_id
  } catch (err) {
    ctx.sendResult({
      error: 'screenshot_failed',
      message: errorMessage(err, 'Failed to capture screenshot')
    })
  }
})

/** Full-page screenshot via CDP with scrollable container expansion (#363). */
async function captureFullPage(
  ctx: { tabId: number; query: { id: string }; sendResult: (r: unknown) => void },
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

  try {
    // Step 2: Attach CDP debugger
    await chrome.debugger.attach({ tabId: ctx.tabId }, CDP_VERSION)

    try {
      // Step 3: Get full content dimensions
      const metrics = (await chrome.debugger.sendCommand({ tabId: ctx.tabId }, 'Page.getLayoutMetrics', {})) as {
        cssContentSize?: { width: number; height: number }
        contentSize?: { width: number; height: number }
      }

      const contentSize = metrics.cssContentSize ||
        metrics.contentSize || {
          width: DEFAULT_CAPTURE_WIDTH,
          height: DEFAULT_CAPTURE_HEIGHT
        }
      const { width: captureWidth, height: captureHeight } = computeFullPageCaptureDimensions(
        contentSize.width,
        contentSize.height,
        hintedHeight
      )

      // Step 4: Override viewport to full content size
      await chrome.debugger.sendCommand({ tabId: ctx.tabId }, 'Emulation.setDeviceMetricsOverride', {
        width: captureWidth,
        height: captureHeight,
        deviceScaleFactor: 1,
        mobile: false
      })
      let metricsOverrideSet = true

      try {
        // Brief pause for layout reflow after viewport resize
        await delay(150)

        // Step 5: Capture full-page screenshot via CDP
        const screenshotResult = (await chrome.debugger.sendCommand({ tabId: ctx.tabId }, 'Page.captureScreenshot', {
          format,
          quality: format === 'jpeg' ? quality : undefined,
          captureBeyondViewport: true,
          clip: { x: 0, y: 0, width: captureWidth, height: captureHeight, scale: 1 }
        })) as { data: string }

        // Step 7: Build data URL and post to server
        const mimeType = format === 'png' ? 'image/png' : 'image/jpeg'
        const dataUrl = `data:${mimeType};base64,${screenshotResult.data}`
        recordScreenshot(ctx.tabId)

        const ok = await postScreenshot(dataUrl, tab.url, ctx.query.id)
        if (!ok) {
          ctx.sendResult({ error: 'screenshot_upload_failed', message: 'Server rejected screenshot' })
        }
      } finally {
        if (metricsOverrideSet) {
          try {
            // Step 6: Clear device metrics override, even when capture fails.
            await chrome.debugger.sendCommand({ tabId: ctx.tabId }, 'Emulation.clearDeviceMetricsOverride', {})
          } catch {
            /* best effort */
          }
          metricsOverrideSet = false
        }
      }
    } finally {
      try {
        await chrome.debugger.detach({ tabId: ctx.tabId })
      } catch {
        /* already detached */
      }
    }
  } catch (err) {
    // CDP unavailable — fall back to regular captureVisibleTab with warning
    debugLog(DebugCategory.CAPTURE, 'Full-page CDP failed, falling back to viewport capture', {
      error: errorMessage(err)
    })
    const dataUrl = await captureVisibleTabSafe(ctx.tabId, tab.windowId, {
      format: format as 'jpeg' | 'png',
      quality
    })
    recordScreenshot(ctx.tabId)
    const ok = await postScreenshot(dataUrl, tab.url, ctx.query.id)
    if (!ok) {
      ctx.sendResult({ error: 'screenshot_upload_failed', message: 'Server rejected screenshot' })
    }
  } finally {
    // Step 8: Always restore containers
    await chrome.scripting
      .executeScript({
        target: { tabId: ctx.tabId },
        world: 'MAIN',
        func: screenshotRestoreContainers
      })
      .catch(() => {
        /* best effort */
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
    })) as { entries?: unknown[] }
    debugLog(DebugCategory.CAPTURE, 'Waterfall result from content script', {
      entries: result?.entries?.length || 0
    })

    ctx.sendResult({
      entries: result?.entries || [],
      page_url: tab.url || '',
      count: result?.entries?.length || 0
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
    const elements: unknown[] = []
    let firstError: string | undefined
    for (const r of interactiveResults) {
      const res = r.result as {
        success?: boolean
        elements?: unknown[]
        error?: string
        message?: string
      } | null
      if (res?.success === false) {
        if (!firstError) firstError = res.error || res.message
        continue
      }
      if (res?.elements) {
        elements.push(...res.elements)
        if (elements.length >= 100) break
      }
    }
    const cappedElements = elements.slice(0, 100)

    // Apply visible_only filter if requested
    let filteredElements = cappedElements
    if (ctx.params.visible_only === true) {
      filteredElements = cappedElements.filter((el) => {
        const elem = el as { visible?: boolean }
        return elem.visible !== false
      })
    }

    // Apply limit if specified
    const limit =
      typeof ctx.params.limit === 'number' && ctx.params.limit > 0 ? ctx.params.limit : filteredElements.length
    const finalElements = filteredElements.slice(0, limit)

    const payload: Record<string, unknown> = {
      url: tab.url || '',
      title: tab.title || '',
      tab_status: tab.status || '',
      favicon: tab.favIconUrl || '',
      viewport: {
        width: tab.width,
        height: tab.height
      },
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
