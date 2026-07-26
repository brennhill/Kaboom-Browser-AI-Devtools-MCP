// eslint-disable max-lines - Auto-generated from template + partials; must be a single self-contained function for chrome.scripting.executeScript.
/**
 * Purpose: Core DOM primitives for selector-based actions (click, type, wait_for, etc.).
 * #502: Intent/overlay/stability actions extracted to separate self-contained modules:
 *   - dom-primitives-intent.ts (open_composer, submit_active_composer, confirm_top_dialog)
 *   - dom-primitives-overlay.ts (dismiss_top_overlay, auto_dismiss_overlays)
 *   - dom-primitives-stability.ts (wait_for_stable, action_diff)
 * Docs: docs/features/feature/interact-explore/index.md
 */
// dom-primitives.ts — Pre-compiled DOM interaction functions for chrome.scripting.executeScript.
// These bypass CSP restrictions because they use the `func` parameter (no eval/new Function).
// Each function MUST be self-contained — no closures over external variables.

import type { DOMMutationEntry, DOMPrimitiveOptions, DOMResult } from '../dom-types.js'

// Re-export list_interactive primitive for backward compatibility
export { domPrimitiveListInteractive } from './dom-primitives-list-interactive.js'

/**
 * Single self-contained function for all DOM primitives.
 * Passed to chrome.scripting.executeScript({ func: domPrimitive, args: [...] }).
 * MUST NOT reference any module-level variables — Chrome serializes the function source only.
 */
export function domPrimitive(
  action: string,
  selector: string,
  options: DOMPrimitiveOptions
): DOMResult | Promise<DOMResult> | {
  success: boolean
  elements: unknown[]
  candidate_count?: number
  scope_rect_used?: { x: number; y: number; width: number; height: number }
  error?: string
  message?: string
} {
  // @include _dom-selectors.tpl

  // @include _dom-semantic-resolvers.tpl

  // @include _dom-overlay-helpers.tpl

  // @include _dom-intent.tpl

  // @include _dom-intent-actions.tpl

  // @include _dom-ranking.tpl

  function resolveActionTarget(): {
    element?: Element
    error?: DOMResult
    match_count?: number
    match_strategy?: string
    scope_selector_used?: string
    ranked_candidates?: { element_id: string; tag: string; text_preview?: string; score: number }[]
    ambiguous_matches?: { total_count: number; warning: string; candidates: { tag: string; element_id: string; text_preview?: string }[] }
  } {
    const requestedScope = (options.scope_selector || '').trim()
    if (requestedScope && !scopeRoot) {
      return {
        error: domError('scope_not_found', `No scope element matches selector: ${requestedScope}`)
      }
    }
    const activeScope = scopeRoot || document
    const scopeSelectorUsed = requestedScope || undefined
    const scopeRectUsed = scopeRect || undefined

    // #502: wait_for_text and wait_for_absent target document.body — no selector resolution needed.
    // Other former intent actions (open_composer, submit_active_composer, confirm_top_dialog,
    // dismiss_top_overlay, auto_dismiss_overlays, wait_for_stable, action_diff) are now
    // dispatched directly by dom-dispatch.ts to extracted self-contained modules.
    if (action === 'wait_for_text' || action === 'wait_for_absent') {
      return { element: document.body, match_count: 1, match_strategy: action }
    }

    // key_press without selector: dispatch on activeElement or body (#321)
    if (action === 'key_press' && !selector && !options.element_id) {
      const target = document.activeElement || document.body
      if (target) {
        return {
          element: target,
          match_count: 1,
          match_strategy: 'active_element_fallback'
        }
      }
    }

    const requestedElementID = (options.element_id || '').trim()
    if (requestedElementID) {
      const resolvedByID = resolveElementByID(requestedElementID)
      if (!resolvedByID) {
        return {
          error: domError(
            'stale_element_id',
            `Element handle is stale or unknown: ${requestedElementID}. Call list_interactive again.`
          )
        }
      }
      if (activeScope !== document && typeof (activeScope as Element).contains === 'function') {
        const contains = (activeScope as Element).contains(resolvedByID)
        if (!contains) {
          return {
            error: domError(
              'element_id_scope_mismatch',
              `Element handle does not belong to scope: ${requestedScope || '<none>'}`
            )
          }
        }
      }
      if (scopeRect && !intersectsScopeRect(resolvedByID)) {
        return {
          error: domError(
            'element_id_scope_mismatch',
            `Element handle does not intersect scope_rect (${scopeRect.x}, ${scopeRect.y}, ${scopeRect.width}, ${scopeRect.height}).`
          )
        }
      }
      return {
        element: resolvedByID,
        match_count: 1,
        match_strategy: 'element_id',
        scope_selector_used: scopeSelectorUsed
      }
    }

    // #385: nth parameter for explicit disambiguation — works for all selector-based actions
    const nthParam = options.nth
    if (nthParam !== undefined && nthParam !== null) {
      const nth = Number(nthParam)
      if (!Number.isInteger(nth)) {
        return { error: domError('invalid_nth', `nth must be an integer, got: ${nthParam}`) }
      }
      const allMatches = resolveElements(selector, activeScope)
      const uniqueAll = uniqueElements(allMatches)
      const rectFiltered = filterByScopeRect(uniqueAll)
      const visibleFiltered = rectFiltered.filter(isActionableVisible)
      const candidates = visibleFiltered.length > 0 ? visibleFiltered : rectFiltered
      if (candidates.length === 0) {
        return { error: domError('element_not_found', `No element matches selector: ${selector}`) }
      }
      const resolvedIndex = nth < 0 ? candidates.length + nth : nth
      if (resolvedIndex < 0 || resolvedIndex >= candidates.length) {
        return {
          error: domError(
            'nth_out_of_range',
            `nth=${nth} is out of range — selector matched ${candidates.length} element(s). Use nth 0..${candidates.length - 1} or -1..-${candidates.length}.`
          )
        }
      }
      return {
        element: candidates[resolvedIndex]!,
        match_count: candidates.length,
        match_strategy: 'nth_param',
        scope_selector_used: scopeSelectorUsed
      }
    }

    const ambiguitySensitiveActions = new Set([
      'click', 'type', 'select', 'check', 'set_attribute',
      'paste', 'key_press', 'focus', 'scroll_to', 'hover'
    ])

    if (!ambiguitySensitiveActions.has(action)) {
      // #316: For text= selectors, always check total match count to add disambiguation warning
      const allMatches = selector.startsWith('text=') ? resolveElements(selector, activeScope) : null
      const ambiguousInfo = (() => {
        if (!allMatches || allMatches.length <= 1) return undefined
        const uniqueAll = uniqueElements(allMatches)
        if (uniqueAll.length <= 1) return undefined
        return {
          total_count: uniqueAll.length,
          warning: `Selector "${selector}" matched ${uniqueAll.length} elements. First match was used. Use nth, :nth-match(N), or scope_selector to disambiguate.`,
          candidates: uniqueAll.slice(0, 5).map((c) => ({
            tag: c.tagName.toLowerCase(),
            element_id: getOrCreateElementID(c),
            text_preview: ((c as HTMLElement).textContent || '').trim().slice(0, 60) || undefined
          }))
        }
      })()

      const direct = resolveElement(selector, activeScope)
      if (direct && intersectsScopeRect(direct)) {
        return {
          element: direct,
          match_count: 1,
          match_strategy: selector.includes(':nth-match(')
            ? 'nth_match_selector'
            : (scopeRect ? 'rect_selector' : (requestedScope ? 'scoped_selector' : 'selector')),
          scope_selector_used: scopeSelectorUsed,
          ...(ambiguousInfo ? { ambiguous_matches: ambiguousInfo } : {})
        }
      }
      const scopedMatches = filterByScopeRect(uniqueElements(resolveElements(selector, activeScope)))
      const found = (() => {
        if (scopedMatches.length === 0) return null
        const visible = scopedMatches.filter(isActionableVisible)
        return visible[0] || scopedMatches[0] || null
      })()
      if (!found) return { error: domError('element_not_found', `No element matches selector: ${selector}`) }
      return {
        element: found,
        match_count: 1,
        match_strategy: scopeRect ? 'rect_selector' : (requestedScope ? 'scoped_selector' : 'selector'),
        scope_selector_used: scopeSelectorUsed,
        ...(ambiguousInfo ? { ambiguous_matches: ambiguousInfo } : {})
      }
    }

    const rawMatches = resolveElements(selector, activeScope)
    const uniqueMatches: Element[] = []
    const seen = new Set<Element>()
    for (const match of rawMatches) {
      if (seen.has(match)) continue
      seen.add(match)
      uniqueMatches.push(match)
    }

    const rectScopedMatches = filterByScopeRect(uniqueMatches)

    const viableMatches = (() => {
      if (rectScopedMatches.length === 0) return rectScopedMatches
      const visible = rectScopedMatches.filter(isActionableVisible)
      return visible.length > 0 ? visible : rectScopedMatches
    })()

    if (viableMatches.length > 1) {
      const ranking = rankAmbiguousCandidates(viableMatches, action, selector)
      const topCandidates = ranking.ranked.slice(0, 3).map((entry) => ({
        element_id: getOrCreateElementID(entry.element),
        tag: entry.element.tagName.toLowerCase(),
        text_preview: ((entry.element as HTMLElement).textContent || '').trim().slice(0, 60) || undefined,
        score: entry.score
      }))

      if (ranking.winner) {
        return {
          element: ranking.winner,
          match_count: 1,
          match_strategy: 'ranked_resolution',
          ranked_candidates: topCandidates
        }
      }

      const sortedCandidates = ranking.ranked.map((entry) => entry.element)
      return {
        error: {
          success: false,
          action,
          selector,
          error: 'ambiguous_target',
          message: `Selector matches multiple viable elements: ${selector}. Add nth, scope/scope_rect, or use list_interactive element_id/index.`,
          match_count: viableMatches.length,
          match_strategy: 'ambiguous_ranked',
          ...(scopeRect ? { scope_rect_used: scopeRect } : {}),
          candidates: summarizeCandidates(sortedCandidates),
          ranked_candidates: topCandidates,
          suggested_element_id: getOrCreateElementID(ranking.ranked[0]!.element)
        }
      }
    }

    const found = viableMatches[0] || null
    if (!found) return { error: domError('element_not_found', `No element matches selector: ${selector}`) }
    const strategy = (() => {
      if (selector.includes(':nth-match(')) return 'nth_match_selector'
      if (scopeRectUsed) return 'rect_selector'
      if (requestedScope) return 'scoped_selector'
      return 'selector'
    })()
    return {
      element: found,
      match_count: 1,
      match_strategy: strategy,
      scope_selector_used: scopeSelectorUsed
    }
  }

  const resolved = resolveActionTarget()
  if (resolved.error) return resolved.error
  const el = resolved.element!
  const resolvedMatchCount = resolved.match_count || 1
  const resolvedMatchStrategy = resolved.match_strategy || 'selector'
  const resolvedScopeSelector = resolved.scope_selector_used
  const resolvedRankedCandidates = resolved.ranked_candidates
  const resolvedAmbiguousMatches = resolved.ambiguous_matches

  // @include _dom-action-helpers.tpl

  // @include _dom-action-handlers-core.tpl

  // @include _dom-action-handlers-input.tpl

  // @include _dom-action-handlers-overlay.tpl

  const handlers = buildActionHandlers(el)
  const handler = handlers[action]
  if (!handler) {
    return domError('unknown_action', `Unknown DOM action: ${action}`)
  }

  // #316: Enrich result with ambiguous_matches warning if text= matched multiple elements
  const rawResult = handler()
  if (!resolvedAmbiguousMatches) return rawResult
  if (rawResult instanceof Promise) {
    return rawResult.then((r) => {
      if (r && typeof r === 'object' && r.success) {
        return { ...r, ambiguous_matches: resolvedAmbiguousMatches }
      }
      return r
    })
  }
  if (rawResult && typeof rawResult === 'object' && (rawResult as DOMResult).success) {
    return { ...(rawResult as DOMResult), ambiguous_matches: resolvedAmbiguousMatches }
  }
  return rawResult
}

/**
 * Backward-compatible wait helper used by unit tests and legacy call sites.
 * Polls wait_for and listens for DOM mutations for fast resolution.
 */
export function domWaitFor(selector: string, timeoutMs: number = 5000): Promise<DOMResult> {
  const timeout = Math.max(1, timeoutMs)
  const startedAt = Date.now()
  const pollIntervalMs = 50

  return new Promise<DOMResult>((resolve) => {
    let settled = false
    let timer: ReturnType<typeof setTimeout> | null = null
    let observer: MutationObserver | null = null

    const done = (result: DOMResult) => {
      if (settled) return
      settled = true
      if (timer) clearTimeout(timer)
      if (observer) observer.disconnect()
      resolve(result)
    }

    const check = () => {
      const result = domPrimitive('wait_for', selector, { timeout_ms: timeout }) as DOMResult
      if (result?.success) {
        done(result)
        return
      }
      if (Date.now() - startedAt >= timeout) {
        done({
          success: false,
          action: 'wait_for',
          selector,
          error: 'timeout',
          message: `Element not found within ${timeout}ms: ${selector}`
        } as DOMResult)
        return
      }
      timer = setTimeout(check, pollIntervalMs)
    }

    try {
      observer = new MutationObserver(() => {
        if (settled) return
        const immediate = domPrimitive('wait_for', selector, { timeout_ms: timeout }) as DOMResult
        if (immediate?.success) done(immediate)
      })
      observer.observe(document.body || document.documentElement, {
        childList: true,
        subtree: true,
        attributes: true,
        characterData: true
      })
    } catch {
      // Best-effort optimization only; polling remains authoritative.
    }

    check()
  })
}

// Dispatcher utilities (parseDOMParams, executeDOMAction, etc.) moved to ./dom-dispatch.ts
