// interact-explore.ts — explore_page compound command handler (#338).
// Combines page metadata, interactive elements, readable text, and navigation
// links into a single extension response, reducing MCP round-trips for AI agents.

import { domPrimitiveListInteractive } from '../dom/primitives/dom-primitives-list-interactive.js'
import { domPrimitiveNavDiscovery } from '../dom/primitives/dom-primitives-nav-discovery.js'
import { readableFallbackScript } from '../exec/content-fallback-scripts.js'
import { registerCommand } from './registry.js'
import { collectCommandElements, commandPageMetadata, selectCommandElements } from './results/element-results.js'
import { errorMessage } from '../../lib/error-utils.js'

// =============================================================================
// EXPLORE_PAGE COMMAND (#338)
// =============================================================================

async function navigateAndWaitForLoad(tabId: number, targetUrl: string): Promise<void> {
  // Validate URL scheme — only http/https allowed (security: prevent javascript:/data:/chrome: injection)
  if (!/^https?:\/\//i.test(targetUrl)) {
    throw new Error(
      'Only http/https URLs are supported for explore_page navigation, got: ' + targetUrl.split(':')[0] + ':'
    )
  }
  // Register onUpdated listener BEFORE calling tabs.update to prevent race condition
  // where the page load completes before the listener is attached (#9.3.2, #9.7.5).
  await new Promise<void>((resolve) => {
    const timeout = setTimeout(() => {
      chrome.tabs.onUpdated.removeListener(onUpdated)
      resolve()
    }, 15000)
    const onUpdated = (updatedTabId: number, changeInfo: { status?: string }): void => {
      if (updatedTabId === tabId && changeInfo.status === 'complete') {
        chrome.tabs.onUpdated.removeListener(onUpdated)
        clearTimeout(timeout)
        resolve()
      }
    }
    chrome.tabs.onUpdated.addListener(onUpdated)
    chrome.tabs.update(tabId, { url: targetUrl }).catch(() => {
      chrome.tabs.onUpdated.removeListener(onUpdated)
      clearTimeout(timeout)
      resolve() // continue with current page state
    })
  })
}

function firstResultObject(results: readonly { result?: unknown }[] | undefined): Record<string, unknown> | null {
  const first = results?.[0]?.result
  return first && typeof first === 'object' ? (first as Record<string, unknown>) : null
}

function componentError(component: string, payload: Record<string, unknown>): { component: string; error: string } {
  const reason = payload._reason
  return { component, error: String(reason || payload.error) }
}

function collectExploreErrors(
  payload: Record<string, unknown>,
  interactiveError: string | undefined,
  interactiveCount: number,
  readable: Record<string, unknown> | null,
  navigation: Record<string, unknown> | null
): void {
  // Build unified _errors array for partial failures (UX Review R6)
  const errors: Array<{ component: string; error: string }> = []
  if (interactiveError && interactiveCount === 0) {
    payload.interactive_error = interactiveError
    errors.push({ component: 'interactive', error: interactiveError })
  }
  if (readable && typeof readable === 'object' && 'error' in readable) {
    errors.push(componentError('readable', readable))
  }
  if (navigation && typeof navigation === 'object' && 'error' in navigation) {
    errors.push(componentError('navigation', navigation))
  }
  if (errors.length > 0) {
    payload._errors = errors
  }
}

registerCommand('explore_page', async (ctx) => {
  try {
    // If URL is provided, navigate first
    const targetUrl = typeof ctx.params.url === 'string' ? ctx.params.url : undefined
    if (targetUrl) {
      await navigateAndWaitForLoad(ctx.tabId, targetUrl)
    }

    // 1. Get tab info (page metadata)
    const tab = await chrome.tabs.get(ctx.tabId)

    // 2. Run all data collection in parallel — capture errors for _errors array (#9.7.4)
    const [interactiveResults, readableResults, navResults] = await Promise.all([
      // Interactive elements
      chrome.scripting
        .executeScript({
          target: { tabId: ctx.tabId, allFrames: true },
          world: 'MAIN',
          func: domPrimitiveListInteractive,
          args: ['']
        })
        .catch((err: Error) => [{ result: { success: false, error: err.message, _source: 'interactive' } }]),

      // Readable content
      chrome.scripting
        .executeScript({
          target: { tabId: ctx.tabId },
          world: 'ISOLATED',
          func: readableFallbackScript
        })
        .catch((err: Error) => [{ result: { error: 'extraction_failed', _reason: err.message, _source: 'readable' } }]),

      // Navigation links (uses shared dom-primitives-nav-discovery)
      chrome.scripting
        .executeScript({
          target: { tabId: ctx.tabId },
          world: 'ISOLATED',
          func: domPrimitiveNavDiscovery
        })
        .catch((err: Error) => [
          { result: { error: 'extraction_failed', _reason: err.message, _source: 'navigation' } }
        ])
    ])

    // Process interactive elements (capped at 100)
    const { elements: cappedElements, firstError: interactiveError } = collectCommandElements(interactiveResults, 100)
    const finalElements = selectCommandElements(cappedElements, ctx.params)

    // Process readable content and navigation links
    const readable = firstResultObject(readableResults)
    const navigation = firstResultObject(navResults)

    // Build composite payload
    const payload: Record<string, unknown> = {
      ...commandPageMetadata(tab),

      // Interactive elements
      interactive_elements: finalElements,
      interactive_count: finalElements.length,

      // Readable text
      readable: readable || { error: 'extraction_failed' },

      // Navigation links
      navigation: navigation || { error: 'extraction_failed' }
    }

    collectExploreErrors(payload, interactiveError, finalElements.length, readable, navigation)

    ctx.sendResult(payload)
  } catch (err) {
    const message = errorMessage(err, 'Explore page failed')
    ctx.sendResult({
      error: 'explore_page_failed',
      message
    })
  }
})
