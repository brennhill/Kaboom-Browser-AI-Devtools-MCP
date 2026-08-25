  // --- PARTIAL: Action Helper Functions ---
  // Purpose: Viewport capture, mutation tracking, rich editor detection, keyboard simulation,
  //          auto-scroll, interactive ancestor detection, and overlay blocking detection.
  // Why: Separated from main template to keep each partial under 500 LOC.

  /** Capture current viewport/scroll position for action responses. */
  function captureViewport(): { scroll_x: number; scroll_y: number; viewport_width: number; viewport_height: number; page_height: number } {
    const w = typeof window !== 'undefined' ? window : null
    const docEl = document?.documentElement
    const body = document?.body
    return {
      scroll_x: Math.round((w?.scrollX ?? w?.pageXOffset ?? 0)),
      scroll_y: Math.round((w?.scrollY ?? w?.pageYOffset ?? 0)),
      viewport_width: w?.innerWidth ?? docEl?.clientWidth ?? 0,
      viewport_height: w?.innerHeight ?? docEl?.clientHeight ?? 0,
      page_height: Math.max(
        body?.scrollHeight || 0,
        docEl?.scrollHeight || 0
      )
    }
  }

  function dispatchEventIfPossible(target: EventTarget | null | undefined, event: Event): void {
    if (!target) return
    const dispatch = (target as { dispatchEvent?: unknown }).dispatchEvent
    if (typeof dispatch !== 'function') return
    dispatch.call(target, event)
  }

  // #502: readDismissStamp, writeDismissStamp, clearDismissStamp removed —
  // now in dom-primitives-overlay.ts (self-contained).

  // #368: Check if an overlay might be obscuring the target element
  function detectOverlayWarning(targetEl: Element): { overlay_warning?: string; overlay_selector?: string } {
    const overlay = findTopmostOverlay()
    if (!overlay) return {}
    // If the target is inside the overlay, no warning needed — the action is targeting the overlay correctly
    if (typeof (overlay as { contains?: unknown }).contains === 'function' && overlay.contains(targetEl)) return {}
    const overlayInfo = describeOverlay(overlay)
    return {
      overlay_warning: `An overlay (${overlayInfo.overlay_type}) is covering the page. The action targeted the intended element, but input may be intercepted. Use dismiss_top_overlay to close it first.`,
      overlay_selector: overlayInfo.overlay_selector
    }
  }

  function mutatingSuccess(
    node: Element,
    extra?: Omit<Partial<DOMResult>, 'success' | 'action' | 'selector' | 'matched' | 'match_count' | 'match_strategy'>
  ): DOMResult {
    const overlayInfo = detectOverlayWarning(node)
    return {
      success: true,
      action,
      selector,
      ...(scopeRect ? { scope_rect_used: scopeRect } : {}),
      ...(extra || {}),
      ...(overlayInfo.overlay_warning ? overlayInfo : {}),
      matched: matchedTarget(node),
      match_count: resolvedMatchCount,
      match_strategy: resolvedMatchStrategy,
      ...(resolvedRankedCandidates ? { ranked_candidates: resolvedRankedCandidates } : {}),
      viewport: captureViewport()
    }
  }

  // — Mutation tracking: MutationObserver wrapper for DOM change capture —
  function withMutationTracking(fn: () => DOMResult): Promise<DOMResult> {
    const t0 = performance.now()
    const mutations: MutationRecord[] = []
    const observer = new MutationObserver((records) => {
      mutations.push(...records)
    })
    observer.observe(document.body || document.documentElement, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeOldValue: !!options.observe_mutations
    })

    const result = fn()

    if (!result.success) {
      observer.disconnect()
      return Promise.resolve(result)
    }

    function describeMutationSummary(): { added: number; removed: number; modified: number; summary: string } {
      const added = mutations.reduce((s, m) => s + m.addedNodes.length, 0)
      const removed = mutations.reduce((s, m) => s + m.removedNodes.length, 0)
      const modified = mutations.filter((m) => m.type === 'attributes').length
      const parts: string[] = []
      if (added > 0) parts.push(`${added} added`)
      if (removed > 0) parts.push(`${removed} removed`)
      if (modified > 0) parts.push(`${modified} modified`)
      return { added, removed, modified, summary: parts.length > 0 ? parts.join(', ') : 'no DOM changes' }
    }

    function pushNodeEntries(nodes: NodeList, type: 'added' | 'removed', entries: DOMMutationEntry[], maxEntries: number): void {
      for (let i = 0; i < nodes.length && entries.length < maxEntries; i++) {
        const n = nodes[i] as Node | undefined
        if (n && n.nodeType === 1) {
          const el = n as Element
          entries.push({ type, tag: el.tagName?.toLowerCase(), id: el.id || undefined, class: el.className?.toString()?.slice(0, 80) || undefined, text_preview: el.textContent?.slice(0, 100) || undefined })
        }
      }
    }

    function collectMutationEntries(maxEntries: number): DOMMutationEntry[] {
      const entries: DOMMutationEntry[] = []
      for (const m of mutations) {
        if (entries.length >= maxEntries) break
        if (m.type === 'childList') {
          pushNodeEntries(m.addedNodes, 'added', entries, maxEntries)
          pushNodeEntries(m.removedNodes, 'removed', entries, maxEntries)
        } else if (m.type === 'attributes' && m.target.nodeType === 1) {
          const el = m.target as Element
          entries.push({ type: 'attribute', tag: el.tagName?.toLowerCase(), id: el.id || undefined, attribute: m.attributeName || undefined, old_value: m.oldValue?.slice(0, 100) || undefined, new_value: el.getAttribute(m.attributeName || '')?.slice(0, 100) || undefined })
        }
      }
      return entries
    }

    return new Promise((resolve) => {
      let resolved = false
      function finish() {
        if (resolved) return
        resolved = true
        observer.disconnect()
        const totalMs = Math.round(performance.now() - t0)
        const { added, removed, modified, summary } = describeMutationSummary()

        const enriched: DOMResult = { ...result, dom_summary: summary }

        if (options.analyze) {
          enriched.timing = { total_ms: totalMs }
          enriched.dom_changes = { added, removed, modified, summary }
          enriched.analysis = `${result.action} completed in ${totalMs}ms. ${summary}.`
        }

        if (options.observe_mutations) {
          enriched.dom_mutations = collectMutationEntries(50)
        }

        resolve(enriched)
      }

      // setTimeout fallback — always fires, even in backgrounded/headless tabs
      // where requestAnimationFrame is suppressed
      setTimeout(finish, 80)

      // Try rAF for better timing when tab is visible, but don't depend on it
      if (typeof requestAnimationFrame === 'function') {
        requestAnimationFrame(() => setTimeout(finish, 50))
      }
    })
  }

  // — Rich editor detection: walk up from target to find known editor containers —
  function detectRichEditor(node: Node): { type: string; target: HTMLElement } | null {
    const el = node instanceof HTMLElement ? node : (node.parentElement || null)
    if (!el) return null
    const checks: Array<{ selector: string; type: string }> = [
      { selector: '.ql-editor', type: 'quill' },
      { selector: '.ProseMirror', type: 'prosemirror' },
      { selector: '[data-contents="true"]', type: 'draftjs' },
      { selector: '[data-editor]', type: 'draftjs' },
      { selector: '.mce-content-body', type: 'tinymce' },
      { selector: '#tinymce', type: 'tinymce' },
      { selector: '.ck-editor__editable', type: 'ckeditor' },
    ]
    for (const check of checks) {
      if (typeof el.matches === 'function' && el.matches(check.selector)) {
        return { type: check.type, target: el }
      }
      if (typeof el.closest === 'function') {
        const ancestor = el.closest(check.selector)
        if (ancestor instanceof HTMLElement) {
          return { type: check.type, target: ancestor }
        }
      }
    }
    return null
  }

  // — Native DOM insertion for detected rich editors (Quill, ProseMirror, etc.) —
  function insertViaRichEditor(
    _editorType: string,
    target: HTMLElement,
    text: string,
    clear: boolean
  ): { success: boolean } {
    const lines = text.split('\n')
    const htmlParts: string[] = []
    for (const line of lines) {
      if (line.length > 0) {
        htmlParts.push('<p>' + line.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;') + '</p>')
      } else {
        htmlParts.push('<p><br></p>')
      }
    }
    const html = htmlParts.join('')
    if (clear) {
      target.innerHTML = html
    } else {
      target.insertAdjacentHTML('beforeend', html)
    }
    target.dispatchEvent(new Event('input', { bubbles: true }))
    return { success: true }
  }

  // — Keyboard event helpers —
  function keyCodeForChar(char: string): { key: string; code: string; keyCode: number } {
    if (char === '\n') return { key: 'Enter', code: 'Enter', keyCode: 13 }
    if (char === '\t') return { key: 'Tab', code: 'Tab', keyCode: 9 }
    if (char === ' ') return { key: ' ', code: 'Space', keyCode: 32 }

    const upper = char.toUpperCase()
    const isLetter = upper >= 'A' && upper <= 'Z'
    const isDigit = char >= '0' && char <= '9'

    let code: string
    let keyCode: number

    if (isLetter) {
      code = 'Key' + upper
      keyCode = upper.charCodeAt(0)
    } else if (isDigit) {
      code = 'Digit' + char
      keyCode = char.charCodeAt(0)
    } else {
      // Punctuation / symbols: use Unidentified code, charCode as keyCode
      code = ''
      keyCode = char.charCodeAt(0)
    }

    return { key: char, code, keyCode }
  }

  function dispatchKeySequence(target: EventTarget, char: string, isContentEditable: boolean): void {
    const { key, code, keyCode } = keyCodeForChar(char)
    const shiftKey = char !== char.toLowerCase() && char === char.toUpperCase() && char.toLowerCase() !== char.toUpperCase()

    const kbOpts: KeyboardEventInit & { keyCode?: number } = { key, code, keyCode, bubbles: true, cancelable: true, shiftKey }

    target.dispatchEvent(new KeyboardEvent('keydown', kbOpts))
    target.dispatchEvent(new KeyboardEvent('keypress', kbOpts))

    if (isContentEditable) {
      // Browsers fire beforeinput/input as InputEvents on contenteditable
      target.dispatchEvent(new InputEvent('beforeinput', {
        bubbles: true, cancelable: true, inputType: 'insertText', data: char,
      }))
      // Insert text at selection (replaces execCommand)
      const sel = document.getSelection()
      if (sel && sel.rangeCount > 0) {
        const range = sel.getRangeAt(0)
        range.deleteContents()
        if (char === '\n') {
          range.insertNode(document.createElement('br'))
        } else {
          range.insertNode(document.createTextNode(char))
        }
        range.collapse(false)
        sel.removeAllRanges()
        sel.addRange(range)
      }
      target.dispatchEvent(new InputEvent('input', {
        bubbles: true, inputType: 'insertText', data: char,
      }))
    }

    target.dispatchEvent(new KeyboardEvent('keyup', kbOpts))
  }

  // — Keyboard simulation for generic contenteditable (no framework detected) —
  function insertViaKeyboardSim(node: HTMLElement, text: string): { success: boolean } {
    for (const char of text) {
      dispatchKeySequence(node, char, true)
    }
    return { success: true }
  }

  // --- #336: Check if element is outside the viewport and auto-scroll into view ---
  function isElementOutsideViewport(el: Element): boolean {
    if (!(el instanceof HTMLElement) || typeof el.getBoundingClientRect !== 'function') return false
    const rect = el.getBoundingClientRect()
    const { width: viewWidth, height: viewHeight } = viewportSize()
    if (viewHeight === 0 && viewWidth === 0) return false
    return rect.bottom < 0 || rect.top > viewHeight || rect.right < 0 || rect.left > viewWidth
  }

  function autoScrollIfNeeded(el: Element): boolean {
    if (isElementOutsideViewport(el)) {
      el.scrollIntoView({ behavior: 'instant', block: 'center' })
      return true
    }
    return false
  }

  // --- #332: Find nearest interactive ancestor for non-interactive wrapper elements ---
  function findInteractiveAncestor(el: Element): Element | null {
    const tag = el.tagName.toLowerCase()
    const role = el.getAttribute('role') || ''
    const interactiveTags = new Set(['a', 'button', 'input', 'select', 'textarea'])
    const interactiveRoles = new Set(['button', 'link', 'menuitem', 'tab', 'option', 'switch'])
    // Already interactive — no need to bubble up
    if (interactiveTags.has(tag) || interactiveRoles.has(role)) return null
    if (typeof el.closest === 'function') {
      const ancestor = el.closest('a, button, [role="button"], [role="link"], [role="menuitem"], [role="tab"], input, select, textarea')
      if (ancestor && ancestor !== el) return ancestor
    }
    return null
  }

  type ActionHandler = () => DOMResult | Promise<DOMResult>

  // Detect if an element is obscured by a modal/dialog overlay.
  // Returns the overlay element if blocking, null otherwise.
  function detectBlockingOverlay(el: Element): Element | null {
    const dialogs = collectDialogs()
    if (dialogs.length === 0) return null
    const topDialog = pickTopDialog(dialogs)
    if (!topDialog) return null
    // If the element is inside the top dialog, it's not blocked
    if (typeof topDialog.contains === 'function' && topDialog.contains(el)) return null
    // Element is outside the top dialog — it's blocked by the overlay
    return topDialog
  }

  function describeBlockingOverlay(overlay: Element): string {
    const overlayTag = overlay.tagName.toLowerCase()
    const overlayRole = overlay.getAttribute('role') || ''
    const overlayLabel = overlay.getAttribute('aria-label') || ''
    if (overlayLabel) return `${overlayTag}[aria-label="${overlayLabel}"]`
    if (overlayRole) return `${overlayTag}[role="${overlayRole}"]`
    return overlayTag
  }

  function blockedByOverlayError(target: Element): DOMResult | null {
    const blockingOverlay = detectBlockingOverlay(target)
    if (!blockingOverlay) return null
    const overlayDesc = describeBlockingOverlay(blockingOverlay)
    return domError(
      'blocked_by_overlay',
      `Element is behind a modal overlay (${overlayDesc}). Use interact({what:"dismiss_top_overlay"}) to close it first.`
    )
  }
