  // --- PARTIAL: DOM Selector Resolution ---
  // Purpose: Shadow DOM traversal, element ownership, visibility, scoping, and element handle store.
  // Why: Core selector infrastructure used by all other DOM primitives.

  // — Shadow DOM: deep traversal utilities —

  function getShadowRoot(el: Element): ShadowRoot | null {
    return el.shadowRoot ?? null
    // Closed root support: see feat/closed-shadow-capture branch
  }

  function querySelectorDeep(selector: string, root: ParentNode = document): Element | null {
    const fast = root.querySelector(selector)
    if (fast && !isKaboomOwnedElement(fast)) return fast
    return querySelectorDeepWalk(selector, root)
  }

  function querySelectorDeepWalk(selector: string, root: ParentNode, depth: number = 0): Element | null {
    if (depth > 10) return null
    // Navigate to children: handle Document (has body/documentElement) and Element/ShadowRoot (has children)
    const children = 'children' in root
      ? (root as Element).children
      : (root as Document).body?.children || (root as Document).documentElement?.children
    if (!children) return null
    for (let i = 0; i < children.length; i++) {
      const child = children[i]!
      const shadow = getShadowRoot(child)
      if (shadow) {
        const match = shadow.querySelector(selector)
        if (match && !isKaboomOwnedElement(match)) return match
        const deep = querySelectorDeepWalk(selector, shadow, depth + 1)
        if (deep) return deep
      }
      if (child.children.length > 0) {
        const deep = querySelectorDeepWalk(selector, child, depth + 1)
        if (deep) return deep
      }
    }
    return null
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
      if (!isKaboomOwnedElement(match)) {
        results.push(match)
      }
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

  function resolveDeepCombinator(selector: string, root: ParentNode = document): Element | null {
    const parts = selector.split(' >>> ')
    if (parts.length <= 1) return null

    let current: ParentNode = root
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i]!.trim()
      if (i < parts.length - 1) {
        const host = querySelectorDeep(part, current)
        if (!host) return null
        const shadow = getShadowRoot(host)
        if (!shadow) return null
        current = shadow
      } else {
        return querySelectorDeep(part, current)
      }
    }
    return null
  }

  // — Selector resolver: CSS or semantic (text=, role=, placeholder=, label=, aria-label=) —

  function isKaboomOwnedElement(element: Element | null): boolean {
    let node: Element | null = element
    while (node) {
      if (node.getAttribute && node.getAttribute('data-kaboom-owned') === 'true') return true
      node = node.parentElement
    }
    return false
  }

  // Visibility check: skip display:none, visibility:hidden, zero-size elements
  function isVisible(el: Element): boolean {
    if (isKaboomOwnedElement(el)) return false
    if (!(el instanceof HTMLElement)) return true
    const style = getComputedStyle(el)
    if (style.visibility === 'hidden' || style.display === 'none') return false
    if (el.offsetParent === null && style.position !== 'fixed' && style.position !== 'sticky') {
      const rect = el.getBoundingClientRect()
      if (rect.width === 0 && rect.height === 0) return false
    }
    return true
  }

  // Return first visible match from a list, falling back to first match
  function firstVisible(els: NodeListOf<Element> | Element[]): Element | null {
    let fallback: Element | null = null
    for (const el of els) {
      if (!fallback) fallback = el
      if (isVisible(el)) return el
    }
    return fallback
  }

  function resolveScopeRoot(rawScope?: string): ParentNode | null {
    const scope = (rawScope || '').trim()
    if (!scope) return document
    try {
      const matches = querySelectorAllDeep(scope)
      if (matches.length === 0) return null
      return firstVisible(matches) || matches[0] || null
    } catch {
      return null
    }
  }

  const scopeRoot = resolveScopeRoot(options.scope_selector)

  type ScopeRect = { x: number; y: number; width: number; height: number }

  function parseScopeRect(raw: unknown): ScopeRect | null {
    if (!raw || typeof raw !== 'object') return null
    const rect = raw as { x?: unknown; y?: unknown; width?: unknown; height?: unknown }
    const x = Number(rect.x)
    const y = Number(rect.y)
    const width = Number(rect.width)
    const height = Number(rect.height)
    if (![x, y, width, height].every((v) => Number.isFinite(v))) return null
    if (width <= 0 || height <= 0) return null
    return { x, y, width, height }
  }

  const scopeRect = parseScopeRect(options.scope_rect)
  if (options.scope_rect !== undefined && !scopeRect) {
    return {
      success: false,
      action,
      selector,
      error: 'invalid_scope_rect',
      message: 'scope_rect must include finite x, y, width, and height > 0'
    }
  }

  function intersectsScopeRect(el: Element): boolean {
    if (!scopeRect) return true
    const htmlEl = el as HTMLElement
    if (!htmlEl || typeof htmlEl.getBoundingClientRect !== 'function') return false
    const rect = htmlEl.getBoundingClientRect()
    const left = typeof rect.left === 'number' ? rect.left : (typeof rect.x === 'number' ? rect.x : 0)
    const top = typeof rect.top === 'number' ? rect.top : (typeof rect.y === 'number' ? rect.y : 0)
    const right = typeof rect.right === 'number' ? rect.right : left + rect.width
    const bottom = typeof rect.bottom === 'number' ? rect.bottom : top + rect.height
    const scopeRight = scopeRect.x + scopeRect.width
    const scopeBottom = scopeRect.y + scopeRect.height
    const overlapX = left < scopeRight && right > scopeRect.x
    const overlapY = top < scopeBottom && bottom > scopeRect.y
    return overlapX && overlapY
  }

  function filterByScopeRect(elements: Element[]): Element[] {
    if (!scopeRect) return elements
    return elements.filter((el) => intersectsScopeRect(el))
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
      // Migrate legacy stores that lack selectorByID (#361)
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

  // #361: When element is stale (disconnected after SPA navigation), try to re-resolve
  // using the stored selector. This allows persistent elements (nav links, sidebars)
  // to survive SPA navigations without requiring a fresh list_interactive call.
  function resolveElementByID(rawElementID?: string): Element | null {
    const elementID = (rawElementID || '').trim()
    if (!elementID) return null
    const store = getElementHandleStore()
    const node = store.byID.get(elementID)
    if (node && (node as Node).isConnected !== false) return node
    // Element is stale or missing — try re-resolution via stored selector
    const storedSelector = store.selectorByID.get(elementID)
    if (storedSelector) {
      const reresolved = resolveElement(storedSelector, document)
      if (reresolved && (reresolved as Node).isConnected !== false) {
        // Update mappings to point to the new element
        store.byElement.set(reresolved, elementID)
        store.byID.set(elementID, reresolved)
        return reresolved
      }
    }
    // Truly stale — clean up
    if (node) store.byID.delete(elementID)
    store.selectorByID.delete(elementID)
    return null
  }
