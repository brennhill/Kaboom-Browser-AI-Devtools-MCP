  // --- PARTIAL: Overlay Detection Helpers ---
  // Purpose: Find topmost overlay, describe overlays, detect extension-injected overlays.
  // Why: Separated from _dom-intent.tpl to keep each partial under 500 LOC.

  // --- Helper: Find topmost visible overlay using z-index analysis + role detection (#334) ---
  function isOverlaySizedHighZIndexElement(el: Element): boolean {
    if (!(el instanceof HTMLElement)) return false
    const style = getComputedStyle(el)
    const zIndex = Number.parseInt(style.zIndex || '', 10)
    if (Number.isNaN(zIndex) || zIndex < 1000) return false
    const position = style.position || ''
    if (position !== 'fixed' && position !== 'absolute') return false
    const rect = el.getBoundingClientRect()
    // Must be reasonably sized (not a tiny tooltip)
    if (rect.width < 100 || rect.height < 100) return false
    // Must be visible
    if (style.display === 'none' || style.visibility === 'hidden' || style.opacity === '0') return false
    return true
  }

  function findTopmostOverlay(): Element | null {
    // Collect all dialog/modal candidates
    const dialogSelectors = [
      '[role="dialog"]', '[role="alertdialog"]', '[aria-modal="true"]', 'dialog[open]',
      '.modal.show', '.modal.in', '.modal.is-active', '.modal[style*="display: block"]',
      '.overlay', '.popup', '.lightbox',
      '[data-modal]', '[data-overlay]', '[data-dialog]',
    ]
    const candidates: Element[] = []
    for (const dialogSelector of dialogSelectors) {
      candidates.push(...querySelectorAllDeep(dialogSelector))
    }

    // Also check for high z-index elements that look like overlays
    const allElements = document.querySelectorAll('*')
    for (let i = 0; i < allElements.length; i++) {
      const el = allElements[i]!
      if (isOverlaySizedHighZIndexElement(el)) candidates.push(el)
    }

    const unique = uniqueElements(candidates).filter(isActionableVisible)
    if (unique.length === 0) return null

    // Score and pick the topmost
    const ranked = unique.map((candidate, index) => ({
      element: candidate,
      score: elementZIndexScore(candidate) * 1000 + areaScore(candidate, 200) + index
    }))
    ranked.sort((a, b) => b.score - a.score)
    return ranked[0]?.element || null
  }

  function describeOverlay(el: Element): { overlay_type: string; overlay_selector: string; overlay_text_preview: string } {
    const tag = el.tagName.toLowerCase()
    const role = el.getAttribute('role') || ''
    const ariaModal = el.getAttribute('aria-modal') || ''
    let overlayType = 'unknown'
    if (tag === 'dialog') overlayType = 'dialog'
    else if (role === 'dialog' || role === 'alertdialog') overlayType = role
    else if (ariaModal === 'true') overlayType = 'modal'
    else overlayType = 'overlay'
    const overlaySelector = (() => {
      if (el.id) return `#${el.id}`
      if (role) return `${tag}[role="${role}"]`
      const className = (el as HTMLElement).className
      if (typeof className === 'string' && className.trim()) return `${tag}.${className.trim().split(/\s+/)[0]}`
      return tag
    })()
    const textPreview = ((el as HTMLElement).textContent || '').trim().slice(0, 120)
    return { overlay_type: overlayType, overlay_selector: overlaySelector, overlay_text_preview: textPreview }
  }

  // #502: detectExtensionOverlay removed — now in dom-primitives-overlay.ts (self-contained).
