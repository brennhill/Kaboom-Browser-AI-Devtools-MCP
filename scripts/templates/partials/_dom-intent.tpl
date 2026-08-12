  // --- PARTIAL: Element Resolution Helpers ---
  // #502: listInteractiveCompatibility removed — list_interactive is dispatched directly
  // by dom-dispatch.ts to domPrimitiveListInteractive.

  // — Resolve element for all other actions —
  function domError(error: string, message: string): DOMResult {
    return { success: false, action, selector, error, message }
  }

  function matchedTarget(node: Element): NonNullable<DOMResult['matched']> {
    const htmlEl = node as HTMLElement
    const textPreview = (htmlEl.textContent || '').trim().slice(0, 80)
    // #388: Include class list for selector diagnostics
    const classList = typeof htmlEl.className === 'string' && htmlEl.className
      ? htmlEl.className.split(/\s+/).filter(Boolean).slice(0, 5)
      : undefined
    return {
      tag: node.tagName.toLowerCase(),
      role: node.getAttribute('role') || undefined,
      aria_label: node.getAttribute('aria-label') || undefined,
      text_preview: textPreview || undefined,
      classes: classList && classList.length > 0 ? classList : undefined,
      selector,
      element_id: getOrCreateElementID(node),
      bbox: extractBoundingBox(node),
      scope_selector_used: resolvedScopeSelector,
      ...(scopeRect ? { scope_rect_used: scopeRect } : {})
    }
  }

  function isActionableVisible(el: Element): boolean {
    if (!(el instanceof HTMLElement)) return true
    const rect = typeof el.getBoundingClientRect === 'function'
      ? el.getBoundingClientRect()
      : ({ width: 0, height: 0 } as DOMRect)
    if (!(rect.width > 0 && rect.height > 0)) return false
    if (el.offsetParent === null) {
      const style = typeof getComputedStyle === 'function' ? getComputedStyle(el) : null
      const position = style?.position || ''
      if (position !== 'fixed' && position !== 'sticky') return false
    }

    // #384: Prefer in-viewport actionable targets for disambiguation.
    const viewHeight = typeof window !== 'undefined' && typeof window.innerHeight === 'number'
      ? window.innerHeight
      : (typeof document !== 'undefined' && document.documentElement ? Number(document.documentElement.clientHeight || 0) : 0)
    const viewWidth = typeof window !== 'undefined' && typeof window.innerWidth === 'number'
      ? window.innerWidth
      : (typeof document !== 'undefined' && document.documentElement ? Number(document.documentElement.clientWidth || 0) : 0)
    const left = typeof rect.left === 'number' ? rect.left : (typeof rect.x === 'number' ? rect.x : 0)
    const top = typeof rect.top === 'number' ? rect.top : (typeof rect.y === 'number' ? rect.y : 0)
    const right = typeof rect.right === 'number' ? rect.right : left + rect.width
    const bottom = typeof rect.bottom === 'number' ? rect.bottom : top + rect.height
    const intersectsX = viewWidth <= 0 || (right > 0 && left < viewWidth)
    const intersectsY = viewHeight <= 0 || (bottom > 0 && top < viewHeight)
    return intersectsX && intersectsY
  }

  function extractBoundingBox(el: Element): { x: number; y: number; width: number; height: number } {
    if (!(el instanceof HTMLElement) || typeof el.getBoundingClientRect !== 'function') {
      return { x: 0, y: 0, width: 0, height: 0 }
    }
    const rect = el.getBoundingClientRect()
    const x = typeof rect.left === 'number' ? rect.left : (typeof rect.x === 'number' ? rect.x : 0)
    const y = typeof rect.top === 'number' ? rect.top : (typeof rect.y === 'number' ? rect.y : 0)
    const width = Number.isFinite(rect.width) ? rect.width : 0
    const height = Number.isFinite(rect.height) ? rect.height : 0
    // Round to whole pixels. getBoundingClientRect returns sub-pixel floats, and
    // serializing 94.30208587646484 for a width costs ~15 bytes per element for
    // precision no consumer uses — clicks and highlights are integer-pixel.
    return { x: Math.round(x), y: Math.round(y), width: Math.round(width), height: Math.round(height) }
  }

  function summarizeCandidates(matches: Element[]): NonNullable<DOMResult['candidates']> {
    return matches.slice(0, 8).map((candidate) => {
      const htmlEl = candidate as HTMLElement
      const fallback = candidate.tagName.toLowerCase()
      return {
        tag: fallback,
        role: candidate.getAttribute('role') || undefined,
        aria_label: candidate.getAttribute('aria-label') || undefined,
        text_preview: (htmlEl.textContent || '').trim().slice(0, 80) || undefined,
        selector: buildUniqueSelector(candidate, htmlEl, fallback),
        element_id: getOrCreateElementID(candidate),
        bbox: extractBoundingBox(candidate),
        visible: isActionableVisible(candidate)
      }
    })
  }

  // #502: intentActions set removed — intent/overlay/stability actions are dispatched directly
  // by dom-dispatch.ts to self-contained extracted modules.

  function uniqueElements(elements: Element[]): Element[] {
    const out: Element[] = []
    const seen = new Set<Element>()
    for (const element of elements) {
      if (seen.has(element)) continue
      seen.add(element)
      out.push(element)
    }
    return out
  }

  function elementZIndexScore(el: Element): number {
    if (!(el instanceof HTMLElement)) return 0
    const style = getComputedStyle(el)
    const raw = style.zIndex || ''
    const parsed = Number.parseInt(raw, 10)
    if (Number.isNaN(parsed)) return 0
    return parsed
  }

  function areaScore(el: Element, max: number): number {
    if (!(el instanceof HTMLElement) || typeof el.getBoundingClientRect !== 'function') return 0
    const rect = el.getBoundingClientRect()
    if (rect.width <= 0 || rect.height <= 0) return 0
    return Math.min(max, Math.round((rect.width * rect.height) / 10000))
  }

  // #502: pickBestIntentTarget removed — now in dom-primitives-intent.ts and dom-primitives-overlay.ts.

  function collectDialogs(): Element[] {
    const selectors = ['[role="dialog"]', '[aria-modal="true"]', 'dialog[open]']
    const dialogs: Element[] = []
    for (const dialogSelector of selectors) {
      dialogs.push(...querySelectorAllDeep(dialogSelector))
    }
    return uniqueElements(dialogs).filter(isActionableVisible)
  }

  function pickTopDialog(dialogs: Element[]): Element | null {
    if (dialogs.length === 0) return null
    const ranked = dialogs
      .map((dialog, index) => ({
        element: dialog,
        score: elementZIndexScore(dialog) * 1000 + areaScore(dialog, 200) + index
      }))
      .sort((a, b) => b.score - a.score)
    return ranked[0]?.element || null
  }
