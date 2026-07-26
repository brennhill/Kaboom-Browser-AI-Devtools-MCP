/**
 * Purpose: Self-contained DOM primitives for overlay dismiss actions (dismiss_top_overlay, auto_dismiss_overlays).
 * Why: Extracted from dom-primitives.ts to keep file sizes under the 800 LOC limit (#502).
 *      These actions find and dismiss overlays/modals/consent banners using multi-strategy resolution.
 * Docs: docs/features/feature/interact-explore/index.md
 */

// dom-primitives-overlay.ts — Self-contained overlay dismiss DOM primitives for chrome.scripting.executeScript.
// This function MUST remain self-contained — Chrome serializes the function source only (no closures).

/**
 * Self-contained function that finds and dismisses overlays, modals, and consent banners.
 * Handles both dismiss_top_overlay and auto_dismiss_overlays actions.
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveOverlay }).
 * MUST NOT reference any module-level variables.
 */
export function domPrimitiveOverlay(
  action: 'dismiss_top_overlay' | 'auto_dismiss_overlays',
  options?: {
    scope_selector?: string
    timeout_ms?: number
  }
): {
  success: boolean
  action: string
  selector: string
  error?: string
  message?: string
  strategy?: string
  selector_used?: string
  overlay_type?: string
  overlay_selector?: string
  overlay_text_preview?: string
  overlay_source?: 'extension' | 'page'
  dismissed_count?: number
  matched?: {
    tag?: string
    role?: string
    aria_label?: string
    text_preview?: string
    selector?: string
    element_id?: string
    bbox?: { x: number; y: number; width: number; height: number }
  }
  match_count?: number
  match_strategy?: string
  viewport?: { scroll_x: number; scroll_y: number; viewport_width: number; viewport_height: number; page_height: number }
} {
  // — Shared helpers (duplicated for self-containment) —

  function isKaboomOwnedElement(element: Element | null): boolean {
    let node: Element | null = element
    while (node) {
      const id = (node as HTMLElement).id || ''
      if (id.startsWith('kaboom-')) return true
      const className = (node as HTMLElement).className
      if (typeof className === 'string' && className.includes('kaboom-')) return true
      if (node.getAttribute && node.getAttribute('data-kaboom-owned') === 'true') return true
      node = node.parentElement
    }
    return false
  }

  function getShadowRoot(el: Element): ShadowRoot | null {
    return el.shadowRoot ?? null
  }

  function querySelectorAllDeep(
    selector: string,
    root: ParentNode = document,
    results: Element[] = [],
    depth: number = 0
  ): Element[] {
    if (depth > 10) return results
    const matches = Array.from(root.querySelectorAll(selector))
    for (const match of matches) {
      if (!isKaboomOwnedElement(match)) results.push(match)
    }
    const children = 'children' in root
      ? (root as Element).children
      : (root as Document).body?.children || (root as Document).documentElement?.children
    if (!children) return results
    for (let i = 0; i < children.length; i++) {
      const child = children[i]!
      const shadow = getShadowRoot(child)
      if (shadow) {
        querySelectorAllDeep(selector, shadow, results, depth + 1)
      }
    }
    return results
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
    return true
  }

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

  function extractElementLabel(el: Element): string {
    const htmlEl = el as HTMLElement
    return (
      el.getAttribute('aria-label') ||
      el.getAttribute('title') ||
      el.getAttribute('placeholder') ||
      (htmlEl?.textContent || '').trim().slice(0, 80) ||
      el.tagName.toLowerCase()
    )
  }

  type ElementHandleStore = {
    byElement: WeakMap<Element, string>
    byID: Map<string, Element>
    selectorByID: Map<string, string>
    nextID: number
  }

  function getElementHandleStore(): ElementHandleStore {
    const root = globalThis as typeof globalThis & { __kaboomElementHandles?: ElementHandleStore }
    if (root.__kaboomElementHandles) {
      if (!root.__kaboomElementHandles.selectorByID) {
        root.__kaboomElementHandles.selectorByID = new Map<string, string>()
      }
      return root.__kaboomElementHandles
    }
    const created: ElementHandleStore = {
      byElement: new WeakMap<Element, string>(),
      byID: new Map<string, Element>(),
      selectorByID: new Map<string, string>(),
      nextID: 1
    }
    root.__kaboomElementHandles = created
    return created
  }

  function getOrCreateElementID(el: Element): string {
    const store = getElementHandleStore()
    const existing = store.byElement.get(el)
    if (existing) {
      store.byID.set(existing, el)
      return existing
    }
    const elementID = `el_${(store.nextID++).toString(36)}`
    store.byElement.set(el, elementID)
    store.byID.set(elementID, el)
    return elementID
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
    return { x, y, width, height }
  }

  function captureViewport(): { scroll_x: number; scroll_y: number; viewport_width: number; viewport_height: number; page_height: number } {
    const w = typeof window !== 'undefined' ? window : null
    const docEl = document?.documentElement
    const body = document?.body
    return {
      scroll_x: Math.round((w?.scrollX ?? w?.pageXOffset ?? 0)),
      scroll_y: Math.round((w?.scrollY ?? w?.pageYOffset ?? 0)),
      viewport_width: w?.innerWidth ?? docEl?.clientWidth ?? 0,
      viewport_height: w?.innerHeight ?? docEl?.clientHeight ?? 0,
      page_height: Math.max(body?.scrollHeight || 0, docEl?.scrollHeight || 0)
    }
  }

  function dispatchEventIfPossible(target: EventTarget | null | undefined, event: Event): void {
    if (!target) return
    const dispatch = (target as { dispatchEvent?: unknown }).dispatchEvent
    if (typeof dispatch !== 'function') return
    dispatch.call(target, event)
  }

  // — Overlay detection —

  function findTopmostOverlay(): Element | null {
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
    const allElements = document.querySelectorAll('*')
    for (let i = 0; i < allElements.length; i++) {
      const el = allElements[i]!
      if (!(el instanceof HTMLElement)) continue
      const style = getComputedStyle(el)
      const zIndex = Number.parseInt(style.zIndex || '', 10)
      if (Number.isNaN(zIndex) || zIndex < 1000) continue
      const position = style.position || ''
      if (position !== 'fixed' && position !== 'absolute') continue
      const rect = el.getBoundingClientRect()
      if (rect.width < 100 || rect.height < 100) continue
      if (style.display === 'none' || style.visibility === 'hidden' || style.opacity === '0') continue
      candidates.push(el)
    }
    const unique = uniqueElements(candidates).filter(isActionableVisible)
    if (unique.length === 0) return null
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

  function detectExtensionOverlay(el: Element): boolean {
    const iframes = el instanceof HTMLElement ? el.querySelectorAll('iframe, img, script, link') : []
    for (let i = 0; i < iframes.length; i++) {
      const child = iframes[i]!
      const src = child.getAttribute('src') || child.getAttribute('href') || ''
      if (src.startsWith('chrome-extension://') || src.startsWith('moz-extension://')) return true
    }
    const extensionTagPrefixes = ['grammarly-', 'lastpass-', 'bitwarden-', '1password-', 'dashlane-', 'honey-', 'loom-']
    const extensionAttrPatterns = ['data-extension-id', 'data-ext-', '__ext']
    let node: Node | null = el
    while (node) {
      const root: Node | null = typeof node.getRootNode === 'function' ? node.getRootNode() : null
      if (root && root !== document && root instanceof ShadowRoot) {
        const host: Element | undefined = (root as ShadowRoot & { host?: Element }).host
        if (host) {
          const hostTag = host.tagName?.toLowerCase() || ''
          const hostResources = host.querySelectorAll('iframe, img, script, link')
          for (let j = 0; j < hostResources.length; j++) {
            const res = hostResources[j]!
            const resSrc = res.getAttribute('src') || res.getAttribute('href') || ''
            if (resSrc.startsWith('chrome-extension://') || resSrc.startsWith('moz-extension://')) return true
          }
          if (extensionTagPrefixes.some(prefix => hostTag.startsWith(prefix))) return true
          if (host.hasAttributes()) {
            const attrs = host.attributes
            for (let k = 0; k < attrs.length; k++) {
              const attrName = attrs[k]!.name.toLowerCase()
              if (extensionAttrPatterns.some(pat => attrName.startsWith(pat) || attrName.includes(pat))) return true
            }
          }
          node = host
          continue
        }
      }
      node = (node as Element).parentElement || null
    }
    return false
  }

  // — Dismiss stamp helpers (loop detection) —

  const dismissStampTTL = 30000

  function readDismissStamp(element: Element | null | undefined): string | null {
    if (!element) return null
    const getAttr = (element as { getAttribute?: unknown }).getAttribute
    if (typeof getAttr !== 'function') return null
    const value = getAttr.call(element, 'data-kaboom-dismiss-ts')
    return typeof value === 'string' && value.length > 0 ? value : null
  }

  function writeDismissStamp(element: Element | null | undefined): void {
    if (!element) return
    const setAttr = (element as { setAttribute?: unknown }).setAttribute
    if (typeof setAttr !== 'function') return
    setAttr.call(element, 'data-kaboom-dismiss-ts', String(Date.now()))
  }

  function clearDismissStamp(element: Element | null | undefined): void {
    if (!element) return
    const removeAttr = (element as { removeAttribute?: unknown }).removeAttribute
    if (typeof removeAttr !== 'function') return
    removeAttr.call(element, 'data-kaboom-dismiss-ts')
  }

  const dismissVerb = /(close|dismiss|cancel|not now|no thanks|skip|x|×|hide|back)/i
  const submitVerb = /(post|share|publish|send|submit|save|done|continue|next|create|apply|confirm|yes|allow|accept)/i

  function domError(error: string, message: string): {
    success: false; action: string; selector: string; error: string; message: string
    overlay_type?: string; overlay_selector?: string; overlay_text_preview?: string; overlay_source?: 'extension' | 'page'
  } {
    return { success: false, action, selector: '', error, message }
  }

  function matchedTarget(node: Element): {
    tag: string; role?: string; aria_label?: string; text_preview?: string; selector?: string; element_id?: string
    bbox?: { x: number; y: number; width: number; height: number }
  } {
    const htmlEl = node as HTMLElement
    const textPreview = (htmlEl.textContent || '').trim().slice(0, 80)
    return {
      tag: node.tagName.toLowerCase(),
      role: node.getAttribute('role') || undefined,
      aria_label: node.getAttribute('aria-label') || undefined,
      text_preview: textPreview || undefined,
      selector: '',
      element_id: getOrCreateElementID(node),
      bbox: extractBoundingBox(node)
    }
  }

  // — Resolve dismiss target —

  type ResolveResult = {
    element?: Element
    match_strategy?: string
    error?: ReturnType<typeof domError>
  }

  function resolveDismissTarget(): ResolveResult {
    const overlayElement = findTopmostOverlay()
    if (!overlayElement) {
      return { error: domError('overlay_not_found', 'No visible dialog/overlay/modal found to dismiss.') }
    }

    // #444: Dismiss loop detection
    const priorStamp = readDismissStamp(overlayElement)
    if (priorStamp) {
      const elapsed = Date.now() - Number(priorStamp)
      if (elapsed < dismissStampTTL) {
        const info = describeOverlay(overlayElement)
        const loopError = domError(
          'dismiss_loop_detected',
          `Overlay (${info.overlay_selector}) was already attempted ${Math.round(elapsed / 1000)}s ago and is still visible. ` +
          'It may be non-dismissable. Try a different approach: use a specific selector to target its close mechanism, ' +
          'navigate away, or ignore it if it does not block interaction.'
        )
        loopError.overlay_type = info.overlay_type
        loopError.overlay_selector = info.overlay_selector
        loopError.overlay_text_preview = info.overlay_text_preview
        loopError.overlay_source = detectExtensionOverlay(overlayElement) ? 'extension' : 'page'
        return { error: loopError }
      }
      clearDismissStamp(overlayElement)
    }

    // Strategy A: close button selectors
    const closeButtonSelectors = [
      'button.close', '.btn-close',
      '[aria-label="Close"]', '[aria-label="close"]', '[aria-label="Dismiss"]', '[aria-label="dismiss"]',
      '[data-dismiss="modal"]', '[data-bs-dismiss="modal"]', '[data-dismiss="dialog"]',
      '[data-dismiss="alert"]', '[data-bs-dismiss="alert"]',
      'button.modal-close', '.dialog-close', '.overlay-close', '.popup-close',
    ]
    for (const closeSelector of closeButtonSelectors) {
      const matches = querySelectorAllDeep(closeSelector, overlayElement as ParentNode)
      const visible = matches.filter(isActionableVisible)
      if (visible.length > 0) {
        return { element: visible[0], match_strategy: 'intent_dismiss_top_overlay' }
      }
    }

    // Strategy B: dismiss-like text buttons
    const dismissTextPatterns = action === 'auto_dismiss_overlays'
      ? /^(close|dismiss|cancel|not now|no thanks|skip|hide|got it|maybe later|x|\u00d7|\u2715|\u2716|\u2573|accept|allow|agree|ok|okay)$/i
      : /^(close|dismiss|cancel|not now|no thanks|skip|hide|back|got it|maybe later|x|\u00d7|\u2715|\u2716|\u2573)$/i
    const allButtons = querySelectorAllDeep('button, [role="button"], [aria-label], [data-testid], [title]', overlayElement as ParentNode)
    type RankedCandidate = { element: Element; score: number }
    const dismissButtons: RankedCandidate[] = []
    for (const btn of uniqueElements(allButtons)) {
      if (!isActionableVisible(btn)) continue
      const label = extractElementLabel(btn).trim()
      let score = 0
      if (dismissTextPatterns.test(label)) score += 900
      else if (dismissVerb.test(label)) score += 700
      if (submitVerb.test(label)) score -= 600
      const hasSvgIcon = typeof btn.querySelector === 'function' && btn.querySelector('svg') !== null
      const textLen = (btn.textContent || '').trim().length
      if (hasSvgIcon && textLen <= 2) score += 500
      const rect = (btn as HTMLElement).getBoundingClientRect()
      if (rect.width > 0 && rect.width < 60 && rect.height > 0 && rect.height < 60) score += 100
      score += elementZIndexScore(btn)
      if (score > 0) dismissButtons.push({ element: btn, score })
    }
    if (dismissButtons.length > 0) {
      dismissButtons.sort((a, b) => b.score - a.score)
      return { element: dismissButtons[0]!.element, match_strategy: 'intent_dismiss_top_overlay' }
    }

    // Strategy C: dismiss-related attributes
    if (action === 'dismiss_top_overlay') {
      const attrCandidates = querySelectorAllDeep('[data-testid], [title]', overlayElement as ParentNode)
      for (const candidate of uniqueElements(attrCandidates)) {
        if (!isActionableVisible(candidate)) continue
        const testId = candidate.getAttribute('data-testid') || ''
        const title = candidate.getAttribute('title') || ''
        if (dismissVerb.test(testId) || dismissVerb.test(title)) {
          return { element: candidate, match_strategy: 'intent_dismiss_top_overlay' }
        }
      }
    }

    // Strategy D: Escape key fallback
    return { element: overlayElement, match_strategy: 'dismiss_escape_fallback' }
  }

  function resolveAutoDismissTarget(): ResolveResult {
    const overlayElement = findTopmostOverlay()

    // #453: Check dismiss loop BEFORE consent-selector short-circuit
    if (overlayElement) {
      const priorAutoStamp = readDismissStamp(overlayElement)
      if (priorAutoStamp) {
        const elapsed = Date.now() - Number(priorAutoStamp)
        if (elapsed < dismissStampTTL) {
          const info = describeOverlay(overlayElement)
          const loopError = domError(
            'dismiss_loop_detected',
            `Overlay (${info.overlay_selector}) was already attempted ${Math.round(elapsed / 1000)}s ago and is still visible. ` +
            'It may be non-dismissable. Try a different approach: use a specific selector to target its close mechanism, ' +
            'navigate away, or ignore it if it does not block interaction.'
          )
          loopError.overlay_type = info.overlay_type
          loopError.overlay_selector = info.overlay_selector
          loopError.overlay_text_preview = info.overlay_text_preview
          loopError.overlay_source = detectExtensionOverlay(overlayElement) ? 'extension' : 'page'
          return { error: loopError }
        }
        clearDismissStamp(overlayElement)
      }
    }

    // Strategy 1: Known consent framework selectors
    const consentSelectors = [
      '#CybotCookiebotDialogBodyLevelButtonLevelOptinAllowAll',
      '#CybotCookiebotDialogBodyButtonDecline',
      '#onetrust-accept-btn-handler',
      '.onetrust-close-btn-handler',
      '.cky-btn-accept',
      '[data-cookieconsent="accept"]',
      '.cc-accept',
      '.cc-dismiss',
      'button[id*="cookie" i][id*="accept" i]',
      'button[id*="consent" i][id*="accept" i]',
    ]
    for (const consentSelector of consentSelectors) {
      try {
        const matches = querySelectorAllDeep(consentSelector)
        const visible = matches.filter(isActionableVisible)
        if (visible.length > 0) {
          return { element: visible[0], match_strategy: 'consent_framework_selector' }
        }
      } catch { continue }
    }

    // Strategy 2: Fall back to dismiss_top_overlay approach
    if (overlayElement) {
      return resolveDismissTarget()
    }

    return { error: domError('no_overlays', 'No cookie consent banners or overlays found to dismiss.') }
  }

  // — Execute action —

  const resolved = action === 'auto_dismiss_overlays'
    ? resolveAutoDismissTarget()
    : resolveDismissTarget()

  if (resolved.error) return resolved.error
  const node = resolved.element!
  const resolvedMatchStrategy = resolved.match_strategy || 'selector'

  if (!(node instanceof HTMLElement)) {
    return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`)
  }

  // Resolve overlay info for response enrichment
  const collectDialogsFn = (): Element[] => {
    const selectors = ['[role="dialog"]', '[aria-modal="true"]', 'dialog[open]']
    const dialogs: Element[] = []
    for (const dialogSelector of selectors) {
      dialogs.push(...querySelectorAllDeep(dialogSelector))
    }
    return uniqueElements(dialogs).filter(isActionableVisible)
  }

  const overlayEl = (() => {
    const dialogs = collectDialogsFn()
    if (dialogs.length === 0) return node
    const ranked = dialogs
      .map((dialog, index) => ({
        element: dialog,
        score: elementZIndexScore(dialog) * 1000 + areaScore(dialog, 200) + index
      }))
      .sort((a, b) => b.score - a.score)
    return ranked[0]?.element || node
  })()
  const overlayInfo = describeOverlay(overlayEl)

  writeDismissStamp(overlayEl)

  const extSource = detectExtensionOverlay(overlayEl)
  const sourceInfo = extSource ? { overlay_source: 'extension' as const } : { overlay_source: 'page' as const }

  // Strategy: escape_fallback
  if (resolvedMatchStrategy === 'dismiss_escape_fallback') {
    const escKb: KeyboardEventInit & { keyCode?: number } = {
      key: 'Escape', code: 'Escape', keyCode: 27,
      bubbles: true, cancelable: true
    }
    dispatchEventIfPossible(document, new KeyboardEvent('keydown', escKb))
    dispatchEventIfPossible(document, new KeyboardEvent('keyup', escKb))
    dispatchEventIfPossible(node, new KeyboardEvent('keydown', escKb))
    dispatchEventIfPossible(node, new KeyboardEvent('keyup', escKb))
    if (!isActionableVisible(overlayEl)) clearDismissStamp(overlayEl)
    return {
      success: true,
      action,
      selector: '',
      strategy: 'escape_key',
      ...overlayInfo,
      ...sourceInfo,
      ...(action === 'auto_dismiss_overlays' ? { dismissed_count: 1 } : {}),
      matched: matchedTarget(node),
      match_count: 1,
      match_strategy: resolvedMatchStrategy,
      viewport: captureViewport()
    }
  }

  // Strategy: click the resolved dismiss button
  const strategy = (() => {
    if (resolvedMatchStrategy === 'dismiss_close_button_selector') return 'close_button'
    if (resolvedMatchStrategy === 'dismiss_text_button') return 'text_button'
    if (resolvedMatchStrategy === 'dismiss_attr_match') return 'attribute_match'
    if (resolvedMatchStrategy === 'consent_framework_selector') return 'consent_framework'
    if (resolvedMatchStrategy === 'auto_dismiss_close_button') return 'close_button'
    if (resolvedMatchStrategy === 'auto_dismiss_text_button') return 'text_button'
    return 'close_button'
  })()

  node.click()
  if (!isActionableVisible(overlayEl)) clearDismissStamp(overlayEl)
  return {
    success: true,
    action,
    selector: '',
    strategy,
    selector_used: resolvedMatchStrategy,
    ...overlayInfo,
    ...sourceInfo,
    ...(action === 'auto_dismiss_overlays' ? { dismissed_count: 1 } : {}),
    matched: matchedTarget(node),
    match_count: 1,
    match_strategy: resolvedMatchStrategy,
    viewport: captureViewport()
  }
}
