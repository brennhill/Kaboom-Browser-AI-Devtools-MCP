  // --- PARTIAL: Semantic Selector Resolution ---
  // Purpose: Text, label, aria-label, and CSS selector resolution with nth-match support.
  // Why: Separated from _dom-selectors.tpl to keep each partial under 500 LOC.

  // #449: Filter interactive child candidates with visibility/actionability checks
  // and exclude hidden inputs to avoid selecting non-actionable elements.
  function closestInteractiveTarget(parent: Element): Element {
    const interactive = parent.closest('a, button, [role="button"], [role="link"], label, summary')
    if (interactive) return interactive
    if (typeof parent.querySelectorAll === 'function') {
      const childCandidates = parent.querySelectorAll('a[href], button, input:not([type="hidden"]), select, textarea, [role="button"], [role="link"]')
      for (let ci = 0; ci < childCandidates.length; ci++) {
        const child = childCandidates[ci]!
        if (isActionableVisible(child)) return child
      }
    }
    return parent
  }

  function walkShadowScopes(root: ParentNode, visit: (shadow: ShadowRoot) => void): void {
    const children = 'children' in root
      ? (root as Element).children
      : (root as Document).body?.children || (root as Document).documentElement?.children
    if (!children) return
    for (let i = 0; i < children.length; i++) {
      const shadow = getShadowRoot(children[i]!)
      if (shadow) visit(shadow)
    }
  }

  function resolveByTextAll(searchText: string, scope: ParentNode = document): Element[] {
    const results: Element[] = []
    const seen = new Set<Element>()

    function walkScope(root: ParentNode): void {
      const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT)
      while (walker.nextNode()) {
        const node = walker.currentNode
        if (node.textContent && node.textContent.trim().includes(searchText)) {
          const parent = node.parentElement
          if (!parent) continue
          const target = closestInteractiveTarget(parent)
          if (isKaboomOwnedElement(target) || !isVisible(target)) continue
          if (!seen.has(target)) {
            seen.add(target)
            results.push(target)
          }
        }
      }
      walkShadowScopes(root, walkScope)
    }

    walkScope(scope)
    return results
  }

  function resolveByLabelAll(labelText: string, scope: ParentNode = document): Element[] {
    const labels = querySelectorAllDeep('label', scope)
    const results: Element[] = []
    const seen = new Set<Element>()
    const allowGlobalIdLookup =
      scope === document || scope === document.body || scope === document.documentElement
    for (const label of labels) {
      if (label.textContent && label.textContent.trim().includes(labelText)) {
        const forAttr = label.getAttribute('for')
        if (forAttr) {
          const local = querySelectorAllDeep(`#${CSS.escape(forAttr)}`, scope)[0]
          const target = local || (allowGlobalIdLookup ? document.getElementById(forAttr) : null)
          if (target && !seen.has(target)) { seen.add(target); results.push(target) }
        }
        const nested = label.querySelector('input, select, textarea')
        if (nested && !seen.has(nested)) { seen.add(nested); results.push(nested) }
        if (!seen.has(label)) { seen.add(label); results.push(label) }
      }
    }
    return results
  }

  function resolveByAriaLabelAll(al: string, scope: ParentNode = document): Element[] {
    const results: Element[] = []
    const seen = new Set<Element>()
    const exact = querySelectorAllDeep(`[aria-label="${CSS.escape(al)}"]`, scope)
    for (const el of exact) {
      if (!seen.has(el)) { seen.add(el); results.push(el) }
    }
    const all = querySelectorAllDeep('[aria-label]', scope)
    for (const el of all) {
      const label = el.getAttribute('aria-label') || ''
      if (label.startsWith(al) && !seen.has(el)) { seen.add(el); results.push(el) }
    }
    return results
  }

  function resolveByText(searchText: string, scope: ParentNode = document): Element | null {
    let fallback: Element | null = null

    function walkScope(root: ParentNode): Element | null {
      const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT)
      while (walker.nextNode()) {
        const node = walker.currentNode
        if (node.textContent && node.textContent.trim().includes(searchText)) {
          const parent = node.parentElement
          if (!parent) continue
          const target = closestInteractiveTarget(parent)
          if (isKaboomOwnedElement(target)) continue
          if (!fallback) fallback = target
          if (isVisible(target)) return target
        }
      }
      let shadowResult: Element | null = null
      walkShadowScopes(root, (shadow) => {
        if (!shadowResult) shadowResult = walkScope(shadow)
      })
      return shadowResult
    }

    return walkScope(scope) || fallback
  }

  function resolveByLabel(labelText: string, scope: ParentNode = document): Element | null {
    const labels = querySelectorAllDeep('label', scope)
    const allowGlobalIdLookup =
      scope === document || scope === document.body || scope === document.documentElement
    for (const label of labels) {
      if (label.textContent && label.textContent.trim().includes(labelText)) {
        const forAttr = label.getAttribute('for')
        if (forAttr) {
          const local = querySelectorAllDeep(`#${CSS.escape(forAttr)}`, scope)[0]
          if (local) return local
          const target = allowGlobalIdLookup ? document.getElementById(forAttr) : null
          if (target) return target
        }
        const nested = label.querySelector('input, select, textarea')
        if (nested) return nested
        return label
      }
    }
    return null
  }

  function resolveByAriaLabel(al: string, scope: ParentNode = document): Element | null {
    const exact = querySelectorAllDeep(`[aria-label="${CSS.escape(al)}"]`, scope)
    if (exact.length > 0) return firstVisible(exact)
    const all = querySelectorAllDeep('[aria-label]', scope)
    let fallback: Element | null = null
    for (const el of all) {
      const label = el.getAttribute('aria-label') || ''
      if (label.startsWith(al)) {
        if (!fallback) fallback = el
        if (isVisible(el)) return el
      }
    }
    return fallback
  }

  function parseNthMatchSelector(sel: string): { base: string; n: number } | null {
    const nthMatch = sel.match(/^(.*):nth-match\((\d+)\)$/)
    if (!nthMatch) return null
    const base = nthMatch[1] || ''
    const n = Number.parseInt(nthMatch[2] || '0', 10)
    if (!base || Number.isNaN(n) || n < 1) return null
    return { base, n }
  }

  function resolveElements(sel: string, scope: ParentNode = document): Element[] {
    if (!sel) return []
    if (sel.includes(' >>> ')) {
      const deep = resolveDeepCombinator(sel, scope)
      return deep ? [deep] : []
    }
    const parsedNth = parseNthMatchSelector(sel)
    if (parsedNth) {
      const matches = resolveElements(parsedNth.base, scope)
      const target = matches[parsedNth.n - 1]
      return target ? [target] : []
    }
    if (sel.startsWith('text=')) return resolveByTextAll(sel.slice('text='.length), scope)
    if (sel.startsWith('role=')) return querySelectorAllDeep(`[role="${CSS.escape(sel.slice('role='.length))}"]`, scope)
    if (sel.startsWith('placeholder=')) return querySelectorAllDeep(`[placeholder="${CSS.escape(sel.slice('placeholder='.length))}"]`, scope)
    if (sel.startsWith('label=')) return resolveByLabelAll(sel.slice('label='.length), scope)
    if (sel.startsWith('aria-label=')) return resolveByAriaLabelAll(sel.slice('aria-label='.length), scope)
    try {
      return querySelectorAllDeep(sel, scope)
    } catch {
      return []
    }
  }

  function resolveElement(sel: string, scope: ParentNode = document): Element | null {
    if (!sel) return null
    if (sel.includes(' >>> ')) return resolveDeepCombinator(sel, scope)

    const parsedNth = parseNthMatchSelector(sel)
    if (parsedNth) {
      const matches = resolveElements(parsedNth.base, scope)
      return matches[parsedNth.n - 1] || null
    }

    if (sel.startsWith('text=')) return resolveByText(sel.slice('text='.length), scope)
    if (sel.startsWith('role=')) return firstVisible(querySelectorAllDeep(`[role="${CSS.escape(sel.slice('role='.length))}"]`, scope))
    if (sel.startsWith('placeholder=')) return firstVisible(querySelectorAllDeep(`[placeholder="${CSS.escape(sel.slice('placeholder='.length))}"]`, scope))
    if (sel.startsWith('label=')) return resolveByLabel(sel.slice('label='.length), scope)
    if (sel.startsWith('aria-label=')) return resolveByAriaLabel(sel.slice('aria-label='.length), scope)

    return querySelectorDeep(sel, scope)
  }

  // list_interactive is handled by domPrimitiveListInteractive in production dispatch,
  // but remains available here for backward compatibility and direct tests.
  function buildUniqueSelector(el: Element, htmlEl: HTMLElement, fallbackSelector: string): string {
    if (el.id) return `#${CSS.escape(el.id)}`
    if (el instanceof HTMLInputElement && el.name) return `input[name="${CSS.escape(el.name)}"]`
    const ariaLabel = el.getAttribute('aria-label')
    // Use CSS attribute selectors — these resolve via querySelectorAll directly,
    // avoiding semantic resolver ordering mismatches (#360).
    if (ariaLabel) return `[aria-label="${CSS.escape(ariaLabel)}"]`
    const placeholder = el.getAttribute('placeholder')
    if (placeholder) return `[placeholder="${CSS.escape(placeholder)}"]`
    const text = (htmlEl.textContent || '').trim().slice(0, 40)
    if (text) return `text=${text}`
    return fallbackSelector
  }

  function buildShadowSelector(el: Element): string | null {
    const rootNode = el.getRootNode()
    if (!(rootNode instanceof ShadowRoot)) return null

    const parts: string[] = []
    let node: Element = el
    let root: Node = rootNode
    while (root instanceof ShadowRoot) {
      const inner = buildUniqueSelector(node, node as HTMLElement, node.tagName.toLowerCase())
      parts.unshift(inner)
      node = root.host
      root = node.getRootNode()
    }
    const hostSelector = buildUniqueSelector(node, node as HTMLElement, node.tagName.toLowerCase())
    parts.unshift(hostSelector)
    return parts.join(' >>> ')
  }

  function classifyInput(el: Element): string {
    const inputType = (el as HTMLInputElement).type || 'text'
    if (inputType === 'submit' || inputType === 'button' || inputType === 'reset') return 'button'
    if (inputType === 'checkbox' || inputType === 'radio') return 'checkbox'
    return 'input'
  }

  function classifyElement(el: Element): string {
    const tag = el.tagName.toLowerCase()
    if (tag === 'a') return 'link'
    if (tag === 'button' || el.getAttribute('role') === 'button') return 'button'
    if (tag === 'input') return classifyInput(el)
    if (tag === 'select') return 'select'
    if (tag === 'textarea') return 'textarea'
    const roleClass: Record<string, string> = { link: 'link', tab: 'tab', menuitem: 'menuitem' }
    const role = el.getAttribute('role') || ''
    const byRole = Object.prototype.hasOwnProperty.call(roleClass, role) ? roleClass[role] : undefined
    if (byRole) return byRole
    if (el.getAttribute('contenteditable') === 'true') return 'textarea'
    return 'interactive'
  }

  function isVisibleElement(el: Element): boolean {
    const htmlEl = el as HTMLElement
    if (!htmlEl || typeof htmlEl.getBoundingClientRect !== 'function') return true
    const rect = htmlEl.getBoundingClientRect()
    return rect.width > 0 && rect.height > 0 && htmlEl.offsetParent !== null
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

  function chooseBestScopeMatch(matches: Element[]): Element {
    if (matches.length === 1) return matches[0]!

    const submitVerb = /(post|share|publish|send|submit|save|done|continue|next|create|apply)/i
    let best = matches[0]!
    let bestScore = -1

    for (const candidate of matches) {
      const textboxes = querySelectorAllDeep('[role="textbox"], textarea, [contenteditable="true"]', candidate)
      const visibleTextboxes = textboxes.filter(isVisibleElement).length

      const buttonCandidates = querySelectorAllDeep('button, [role="button"], input[type="submit"]', candidate)
      let visibleButtons = 0
      let submitLikeButtons = 0
      for (const btn of buttonCandidates) {
        if (!isVisibleElement(btn)) continue
        visibleButtons++
        if (submitVerb.test(extractElementLabel(btn))) {
          submitLikeButtons++
        }
      }

      const interactiveCandidates = querySelectorAllDeep(
        'a[href], button, input, select, textarea, [role="button"], [role="link"], [role="tab"], [role="menuitem"], [contenteditable="true"]',
        candidate
      )
      const visibleInteractive = interactiveCandidates.filter(isVisibleElement).length
      const hiddenInteractive = Math.max(0, interactiveCandidates.length - visibleInteractive)

      const rect = (candidate as HTMLElement).getBoundingClientRect?.()
      const areaScore = rect && rect.width > 0 && rect.height > 0
        ? Math.min(20, Math.round((rect.width * rect.height) / 50000))
        : 0

      const score =
        visibleTextboxes*1000 +
        submitLikeButtons*250 +
        visibleButtons*10 +
        visibleInteractive -
        hiddenInteractive +
        areaScore

      if (score > bestScore) {
        bestScore = score
        best = candidate
      }
    }

    return best
  }
