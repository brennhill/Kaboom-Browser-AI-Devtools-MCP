// AUTO-GENERATED FILE. DO NOT EDIT DIRECTLY.
// Source: scripts/templates/dom-primitives.ts.tpl + partials/
//   _dom-selectors.tpl, _dom-semantic-resolvers.tpl, _dom-overlay-helpers.tpl,
//   _dom-intent.tpl, _dom-intent-actions.tpl, _dom-ranking.tpl,
//   _dom-action-helpers.tpl, _dom-action-handlers-core.tpl,
//   _dom-action-handlers-input.tpl, _dom-action-handlers-overlay.tpl
// Generator: scripts/generate-dom-primitives.js

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

import type { DOMMutationEntry, DOMPrimitiveOptions, DOMResult } from './dom-types.js'

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
      const id = (node as HTMLElement).id || ''
      if (id.startsWith('kaboom-')) return true
      const className = (node as HTMLElement).className
      if (typeof className === 'string' && className.includes('kaboom-')) return true
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

  // --- PARTIAL: Semantic Selector Resolution ---
  // Purpose: Text, label, aria-label, and CSS selector resolution with nth-match support.
  // Why: Separated from _dom-selectors.tpl to keep each partial under 500 LOC.

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
          const interactive = parent.closest('a, button, [role="button"], [role="link"], label, summary')
          // #449: Filter interactive child candidates with visibility/actionability checks
          // and exclude hidden inputs to avoid selecting non-actionable elements.
          let interactiveChild: Element | null = null
          if (!interactive && typeof parent.querySelectorAll === 'function') {
            const childCandidates = parent.querySelectorAll('a[href], button, input:not([type="hidden"]), select, textarea, [role="button"], [role="link"]')
            for (let ci = 0; ci < childCandidates.length; ci++) {
              const child = childCandidates[ci]!
              if (isActionableVisible(child)) { interactiveChild = child; break }
            }
          }
          const target = interactive || interactiveChild || parent
          if (isKaboomOwnedElement(target) || !isVisible(target)) continue
          if (!seen.has(target)) {
            seen.add(target)
            results.push(target)
          }
        }
      }
      const children = 'children' in root
        ? (root as Element).children
        : (root as Document).body?.children || (root as Document).documentElement?.children
      if (children) {
        for (let i = 0; i < children.length; i++) {
          const child = children[i]!
          const shadow = getShadowRoot(child)
          if (shadow) walkScope(shadow)
        }
      }
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
          const interactive = parent.closest('a, button, [role="button"], [role="link"], label, summary')
          // #449: Filter interactive child candidates with visibility/actionability checks
          // and exclude hidden inputs to avoid selecting non-actionable elements.
          let interactiveChild: Element | null = null
          if (!interactive && typeof parent.querySelectorAll === 'function') {
            const childCandidates = parent.querySelectorAll('a[href], button, input:not([type="hidden"]), select, textarea, [role="button"], [role="link"]')
            for (let ci = 0; ci < childCandidates.length; ci++) {
              const child = childCandidates[ci]!
              if (isActionableVisible(child)) { interactiveChild = child; break }
            }
          }
          const target = interactive || interactiveChild || parent
          if (isKaboomOwnedElement(target)) continue
          if (!fallback) fallback = target
          if (isVisible(target)) return target
        }
      }
      const children = 'children' in root
        ? (root as Element).children
        : (root as Document).body?.children || (root as Document).documentElement?.children
      if (children) {
        for (let i = 0; i < children.length; i++) {
          const child = children[i]!
          const shadow = getShadowRoot(child)
          if (shadow) {
            const result = walkScope(shadow)
            if (result) return result
          }
        }
      }
      return null
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

  function classifyElement(el: Element): string {
    const tag = el.tagName.toLowerCase()
    if (tag === 'a') return 'link'
    if (tag === 'button' || el.getAttribute('role') === 'button') return 'button'
    if (tag === 'input') {
      const inputType = (el as HTMLInputElement).type || 'text'
      if (inputType === 'submit' || inputType === 'button' || inputType === 'reset') return 'button'
      if (inputType === 'checkbox' || inputType === 'radio') return 'checkbox'
      return 'input'
    }
    if (tag === 'select') return 'select'
    if (tag === 'textarea') return 'textarea'
    if (el.getAttribute('role') === 'link') return 'link'
    if (el.getAttribute('role') === 'tab') return 'tab'
    if (el.getAttribute('role') === 'menuitem') return 'menuitem'
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

  // --- PARTIAL: Overlay Detection Helpers ---
  // Purpose: Find topmost overlay, describe overlays, detect extension-injected overlays.
  // Why: Separated from _dom-intent.tpl to keep each partial under 500 LOC.

  // --- Helper: Find topmost visible overlay using z-index analysis + role detection (#334) ---
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
      if (!(el instanceof HTMLElement)) continue
      const style = getComputedStyle(el)
      const zIndex = Number.parseInt(style.zIndex || '', 10)
      if (Number.isNaN(zIndex) || zIndex < 1000) continue
      const position = style.position || ''
      if (position !== 'fixed' && position !== 'absolute') continue
      const rect = el.getBoundingClientRect()
      // Must be reasonably sized (not a tiny tooltip)
      if (rect.width < 100 || rect.height < 100) continue
      // Must be visible
      if (style.display === 'none' || style.visibility === 'hidden' || style.opacity === '0') continue
      candidates.push(el)
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
    return { x, y, width, height }
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

  // --- PARTIAL: Intent Action Resolution ---
  // #502: Extracted to self-contained modules for < 800 LOC file sizes.
  //   - dom-primitives-intent.ts: open_composer, submit_active_composer, confirm_top_dialog
  //   - dom-primitives-overlay.ts: dismiss_top_overlay, auto_dismiss_overlays
  //   - dom-primitives-stability.ts: wait_for_stable, action_diff
  // These actions are dispatched directly by dom-dispatch.ts and no longer go through domPrimitive.

  // --- PARTIAL: Ambiguous Target Ranking ---

  function rankAmbiguousCandidates(
    candidates: Element[],
    action: string,
    selectorText: string
  ): { winner: Element | null; gap: number; ranked: { element: Element; score: number }[] } {
    const dialogs = collectDialogs()
    const topDialog = dialogs.length > 0 ? pickTopDialog(dialogs) : null

    // Extract the text portion from semantic selectors (text=Post → "Post")
    const selectorLabel = (() => {
      if (selectorText.startsWith('text=')) return selectorText.slice(5)
      if (selectorText.startsWith('aria-label=')) return selectorText.slice(11)
      if (selectorText.startsWith('label=')) return selectorText.slice(6)
      if (selectorText.startsWith('placeholder=')) return selectorText.slice(12)
      return ''
    })()

    const clickLikeActions = new Set(['click', 'key_press', 'focus', 'scroll_to', 'set_attribute', 'paste'])
    const typeLikeActions = new Set(['type', 'select', 'check'])

    const scored = candidates.map((el) => {
      const tag = el.tagName.toLowerCase()
      const role = el.getAttribute('role') || ''
      let score = 0

      // Modal scoping: element inside the top open dialog
      if (topDialog && typeof topDialog.contains === 'function' && topDialog.contains(el)) {
        score += 200
      }

      // Element type match
      if (clickLikeActions.has(action)) {
        if (tag === 'button' || role === 'button' || tag === 'input' && ((el as HTMLInputElement).type === 'submit' || (el as HTMLInputElement).type === 'button')) {
          score += 100
        } else if (tag === 'a' || role === 'link') {
          score += 40
        }
      } else if (typeLikeActions.has(action)) {
        if (tag === 'input' || tag === 'textarea' || tag === 'select' || el.getAttribute('contenteditable') === 'true' || role === 'textbox') {
          score += 100
        } else if (tag === 'button' || role === 'button') {
          score += 10
        }
      }

      // Text matching (only when selector provides text)
      if (selectorLabel) {
        const elLabel = extractElementLabel(el)
        const trimmedLabel = elLabel.trim()
        if (trimmedLabel === selectorLabel) {
          score += 80 // exact match
        } else if (trimmedLabel.startsWith(selectorLabel) && trimmedLabel.length <= selectorLabel.length + 5) {
          score += 60 // tight prefix
        }
      }

      // Primary button heuristic
      if (tag === 'button' || role === 'button') {
        const htmlEl = el as HTMLElement
        const cls = (typeof htmlEl.className === 'string' ? htmlEl.className : '').toLowerCase()
        const type = el.getAttribute('type') || ''
        if (type === 'submit') score += 60
        else if (/\bprimary\b|\bbtn-primary\b|\bcta\b/.test(cls)) score += 60
        else {
          const style = typeof getComputedStyle === 'function' ? getComputedStyle(htmlEl) : null
          if (style) {
            const bg = style.backgroundColor || ''
            // Colored background (not transparent, not white, not gray-ish)
            if (bg && !/transparent|rgba\(0,\s*0,\s*0,\s*0\)|rgb\(255,\s*255,\s*255\)|rgb\(2[45]\d,\s*2[45]\d,\s*2[45]\d\)/.test(bg)) {
              score += 30
            }
          }
        }
      }

      // z-index (0–50)
      score += Math.min(50, Math.max(0, elementZIndexScore(el)))

      // Area (0–30)
      score += areaScore(el, 30)

      return { element: el, score }
    })

    scored.sort((a, b) => b.score - a.score)

    const topScore = scored[0]?.score ?? 0
    const secondScore = scored[1]?.score ?? 0
    const gap = topScore - secondScore
    const winner = gap >= 50 ? (scored[0]?.element ?? null) : null

    return { winner, gap, ranked: scored }
  }

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

    return new Promise((resolve) => {
      let resolved = false
      function finish() {
        if (resolved) return
        resolved = true
        observer.disconnect()
        const totalMs = Math.round(performance.now() - t0)
        const added = mutations.reduce((s, m) => s + m.addedNodes.length, 0)
        const removed = mutations.reduce((s, m) => s + m.removedNodes.length, 0)
        const modified = mutations.filter((m) => m.type === 'attributes').length
        const parts: string[] = []
        if (added > 0) parts.push(`${added} added`)
        if (removed > 0) parts.push(`${removed} removed`)
        if (modified > 0) parts.push(`${modified} modified`)
        const summary = parts.length > 0 ? parts.join(', ') : 'no DOM changes'

        const enriched: DOMResult = { ...result, dom_summary: summary }

        if (options.analyze) {
          enriched.timing = { total_ms: totalMs }
          enriched.dom_changes = { added, removed, modified, summary }
          enriched.analysis = `${result.action} completed in ${totalMs}ms. ${summary}.`
        }

        if (options.observe_mutations) {
          const maxEntries = 50
          const entries: DOMMutationEntry[] = []
          for (const m of mutations) {
            if (entries.length >= maxEntries) break
            if (m.type === 'childList') {
              for (let i = 0; i < m.addedNodes.length && entries.length < maxEntries; i++) {
                const n = m.addedNodes[i] as Node | undefined
                if (n && n.nodeType === 1) {
                  const el = n as Element
                  entries.push({ type: 'added', tag: el.tagName?.toLowerCase(), id: el.id || undefined, class: el.className?.toString()?.slice(0, 80) || undefined, text_preview: el.textContent?.slice(0, 100) || undefined })
                }
              }
              for (let i = 0; i < m.removedNodes.length && entries.length < maxEntries; i++) {
                const n = m.removedNodes[i] as Node | undefined
                if (n && n.nodeType === 1) {
                  const el = n as Element
                  entries.push({ type: 'removed', tag: el.tagName?.toLowerCase(), id: el.id || undefined, class: el.className?.toString()?.slice(0, 80) || undefined, text_preview: el.textContent?.slice(0, 100) || undefined })
                }
              }
            } else if (m.type === 'attributes' && m.target.nodeType === 1) {
              const el = m.target as Element
              entries.push({ type: 'attribute', tag: el.tagName?.toLowerCase(), id: el.id || undefined, attribute: m.attributeName || undefined, old_value: m.oldValue?.slice(0, 100) || undefined, new_value: el.getAttribute(m.attributeName || '')?.slice(0, 100) || undefined })
            }
          }
          enriched.dom_mutations = entries
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
    const viewHeight = typeof window !== 'undefined' && typeof window.innerHeight === 'number'
      ? window.innerHeight
      : (typeof document !== 'undefined' && document.documentElement ? document.documentElement.clientHeight : 0)
    const viewWidth = typeof window !== 'undefined' && typeof window.innerWidth === 'number'
      ? window.innerWidth
      : (typeof document !== 'undefined' && document.documentElement ? document.documentElement.clientWidth : 0)
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

  // --- PARTIAL: Core Action Handlers ---
  // Purpose: click, type, select, check, get_text, get_value, get_attribute, set_attribute, focus, scroll_to, wait_for.
  // Why: Separated from main template to keep each partial under 500 LOC.

  function buildActionHandlers(node: Element): Record<string, ActionHandler> {
    return {
      click: () =>
        withMutationTracking(() => {
          if (!(node instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`)

          // #332: Bubble up to nearest interactive ancestor if the matched element is a wrapper
          const interactiveAncestor = findInteractiveAncestor(node)
          const clickTarget = (interactiveAncestor instanceof HTMLElement ? interactiveAncestor : node) as HTMLElement

          // Check if element is behind a modal overlay before clicking
          const overlayErr = blockedByOverlayError(node)
          if (overlayErr) return overlayErr
          if (options.new_tab) {
            const linkNode = (() => {
              const tag = clickTarget.tagName.toLowerCase()
              if (tag === 'a') return clickTarget as Element
              if (typeof clickTarget.closest === 'function') {
                return clickTarget.closest('a[href]')
              }
              return null
            })()

            const href = linkNode
              ? (linkNode.getAttribute('href') || (linkNode as HTMLAnchorElement).href || '')
              : ''
            if (!href) {
              return domError('new_tab_requires_link', 'new_tab=true requires a link target with href')
            }

            let opened = false
            try {
              if (typeof window !== 'undefined' && typeof window.open === 'function') {
                window.open(href, '_blank', 'noopener,noreferrer')
                opened = true
              }
            } catch {
              // Fall through to target=_blank click fallback.
            }

            if (!opened && linkNode instanceof Element) {
              const previousTarget = linkNode.getAttribute('target')
              linkNode.setAttribute('target', '_blank')
              ;(linkNode as HTMLElement).click()
              if (previousTarget == null) {
                linkNode.removeAttribute('target')
              } else {
                linkNode.setAttribute('target', previousTarget)
              }
            }

            return mutatingSuccess(clickTarget, { value: href, reason: 'opened_new_tab' })
          }


          // #336: Auto-scroll off-screen elements into view before clicking
          const didScroll = autoScrollIfNeeded(clickTarget)
          clickTarget.click()
          return mutatingSuccess(clickTarget, didScroll ? { auto_scrolled: true } : undefined)
        }),

      type: () =>
        withMutationTracking(() => {
          const overlayErr = blockedByOverlayError(node)
          if (overlayErr) return overlayErr

          // Normalize literal \n sequences to actual newlines (MCP parameter encoding)
          const text = (options.text || '').replace(/\\n/g, '\n')

          // Contenteditable elements (Gmail compose body, rich text editors)
          if (node instanceof HTMLElement && node.isContentEditable) {
            node.focus()
            if (options.clear) {
              const selection = document.getSelection()
              if (selection) {
                selection.selectAllChildren(node)
                selection.deleteFromDocument()
              }
            }

            // Detect rich editor framework
            const editor = detectRichEditor(node)
            let strategy: string

            if (editor) {
              // Native DOM insertion — bypasses CSP, works with Quill/ProseMirror/etc
              insertViaRichEditor(editor.type, editor.target, text, !!options.clear)
              strategy = editor.type + '_native'
            } else {
              // Per-character keyboard event simulation for all generic contenteditable
              insertViaKeyboardSim(node, text)
              strategy = 'keyboard_simulation'
            }

            return mutatingSuccess(node, { value: node.innerText, insertion_strategy: strategy })
          }

          if (!(node instanceof HTMLInputElement) && !(node instanceof HTMLTextAreaElement)) {
            return domError('not_typeable', `Element is not an input, textarea, or contenteditable: ${node.tagName}`)
          }

          // Dispatch per-character keyboard events so React/Vue onChange handlers fire
          node.focus()
          for (const char of text) {
            dispatchKeySequence(node, char, false)
          }

          // Set the value via native setter (needed to bypass React's synthetic event system)
          const proto = node instanceof HTMLTextAreaElement ? HTMLTextAreaElement : HTMLInputElement
          const nativeSetter = Object.getOwnPropertyDescriptor(proto.prototype, 'value')?.set
          if (nativeSetter) {
            const newValue = options.clear ? text : node.value + text
            nativeSetter.call(node, newValue)
          } else {
            node.value = options.clear ? text : node.value + text
          }
          node.dispatchEvent(new InputEvent('input', { bubbles: true, data: text, inputType: 'insertText' }))
          node.dispatchEvent(new Event('change', { bubbles: true }))
          return mutatingSuccess(node, { value: node.value, insertion_strategy: 'native_setter' })
        }),

      select: () =>
        withMutationTracking(() => {
          const overlayErr = blockedByOverlayError(node)
          if (overlayErr) return overlayErr

          if (!(node instanceof HTMLSelectElement)) return domError('not_select', `Element is not a <select>: ${node.tagName}`) // nosemgrep: html-in-template-string
          const nativeSelectSetter = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, 'value')?.set
          if (nativeSelectSetter) {
            nativeSelectSetter.call(node, options.value || '')
          } else {
            node.value = options.value || ''
          }
          node.dispatchEvent(new Event('change', { bubbles: true }))
          return mutatingSuccess(node, { value: node.value })
        }),

      check: () =>
        withMutationTracking(() => {
          const overlayErr = blockedByOverlayError(node)
          if (overlayErr) return overlayErr

          if (!(node instanceof HTMLInputElement) || (node.type !== 'checkbox' && node.type !== 'radio')) {
            return domError('not_checkable', `Element is not a checkbox or radio: ${node.tagName} type=${(node as HTMLInputElement).type || 'N/A'}`)
          }
          const desired = options.checked !== undefined ? options.checked : true
          if (node.checked !== desired) {
            node.click()
          }
          return mutatingSuccess(node, { value: node.checked })
        }),

      get_text: () => {
        if (options.structured && node instanceof HTMLElement) {
          // Structured extraction: preserve hierarchy for accordions, lists, etc.
          const sections: Array<{header?: string; content: string; expanded?: boolean; tag: string}> = []
          const children = node.children
          for (let i = 0; i < children.length && sections.length < 50; i++) {
            const child = children[i] as HTMLElement
            if (!child.tagName) continue
            const tag = child.tagName.toLowerCase()
            // Detect accordion/collapsible patterns
            const heading = child.querySelector('h1, h2, h3, h4, h5, h6, [role="heading"], summary, button[aria-expanded]')
            if (heading) {
              const headerText = (heading as HTMLElement).innerText?.trim() || ''
              const ariaExpanded = heading.getAttribute('aria-expanded')
              const expanded = ariaExpanded !== null ? ariaExpanded === 'true' : undefined
              // Get content from sibling/next panel or remaining text
              const contentParts: string[] = []
              const contentNodes = child.querySelectorAll('p, li, span, div, td, pre, code')
              contentNodes.forEach((cn) => {
                if (cn !== heading && !heading.contains(cn)) {
                  const t = (cn as HTMLElement).innerText?.trim()
                  if (t && t.length > 0) contentParts.push(t)
                }
              })
              sections.push({
                header: headerText,
                content: contentParts.join('\n') || (child.innerText?.replace(headerText, '').trim() || ''),
                expanded,
                tag,
              })
            } else {
              // Non-accordion child: just capture its text
              const t = child.innerText?.trim()
              if (t && t.length > 0) {
                sections.push({ content: t, tag })
              }
            }
          }
          return { success: true, action, selector, sections, section_count: sections.length }
        }
        const text = node instanceof HTMLElement ? node.innerText : node.textContent
        if (text === null || text === undefined) {
          return {
            success: true,
            action,
            selector,
            value: text,
            reason: 'no_text_content',
            message: 'Resolved text content is null'
          }
        }
        return { success: true, action, selector, value: text }
      },

      get_value: () => {
        if (!('value' in node)) return domError('no_value_property', `Element has no value property: ${node.tagName}`)
        const value = (node as HTMLInputElement).value
        if (value === null || value === undefined) {
          return {
            success: true,
            action,
            selector,
            value,
            reason: 'no_value',
            message: 'Element value is null'
          }
        }
        return { success: true, action, selector, value }
      },

      get_attribute: () => {
        const attrName = options.name || ''
        const value = node.getAttribute(attrName)
        if (value === null) {
          return {
            success: true,
            action,
            selector,
            value,
            reason: 'attribute_not_found',
            message: `Attribute "${attrName}" not found`
          }
        }
        return { success: true, action, selector, value }
      },

      set_attribute: () =>
        withMutationTracking(() => {
          node.setAttribute(options.name || '', options.value || '')
          return mutatingSuccess(node, { value: node.getAttribute(options.name || '') })
        }),

      focus: () => {
        const overlayErr = blockedByOverlayError(node)
        if (overlayErr) return overlayErr

        if (!(node instanceof HTMLElement)) return domError('not_focusable', `Element is not an HTMLElement: ${node.tagName}`)
        node.focus()
        return mutatingSuccess(node)
      },

      scroll_to: () => {
        // #387: Container-aware scroll_to — find scrollable ancestor and support directional scrolling
        function findScrollableContainer(el: Element): HTMLElement | null {
          let current: Element | null = el
          while (current && current !== document.documentElement) {
            if (current instanceof HTMLElement && current.scrollHeight > current.clientHeight + 10) {
              const style = typeof getComputedStyle === 'function' ? getComputedStyle(current) : null
              if (style) {
                const ov = style.overflow || ''
                const ovY = style.overflowY || ''
                if (ov === 'auto' || ov === 'scroll' || ovY === 'auto' || ovY === 'scroll') {
                  return current
                }
              }
            }
            current = current.parentElement
          }
          return null
        }

        function scrollToY(container: HTMLElement, top: number): void {
          if (typeof (container as { scrollTo?: unknown }).scrollTo === 'function') {
            container.scrollTo({ top, behavior: 'smooth' })
            return
          }
          ;(container as { scrollTop?: number }).scrollTop = top
        }

        function scrollByY(container: HTMLElement, deltaY: number): void {
          if (typeof (container as { scrollBy?: unknown }).scrollBy === 'function') {
            container.scrollBy({ top: deltaY, behavior: 'smooth' })
            return
          }
          const currentTop = typeof (container as { scrollTop?: unknown }).scrollTop === 'number'
            ? Number((container as { scrollTop?: unknown }).scrollTop)
            : 0
          ;(container as { scrollTop?: number }).scrollTop = currentTop + deltaY
        }

        // Accept both `direction` (preferred) and legacy `value` for backward compatibility.
        const direction = (options.direction || options.value || '').toLowerCase()
        const tag = node.tagName.toLowerCase()

        // Check if the target itself is a scrollable container
        const isContainer = node instanceof HTMLElement &&
          node.scrollHeight > node.clientHeight + 10 && (() => {
            const s = typeof getComputedStyle === 'function' ? getComputedStyle(node) : null
            if (!s) return false
            const ov = s.overflow || ''
            const ovY = s.overflowY || ''
            return ov === 'auto' || ov === 'scroll' || ovY === 'auto' || ovY === 'scroll'
          })()

        // Directional scrolling within the resolved container (target, ancestor, or page root)
        const directionalContainer = (() => {
          if (isContainer) return node as HTMLElement
          const ancestor = findScrollableContainer(node)
          if (ancestor) return ancestor
          if (typeof document !== 'undefined' && document.scrollingElement instanceof HTMLElement) {
            return document.scrollingElement
          }
          if (tag === 'body' || tag === 'html') return document.documentElement as HTMLElement
          return document.documentElement as HTMLElement
        })()

        if (direction && directionalContainer) {
          const container = directionalContainer
          switch (direction) {
            case 'top':
              scrollToY(container, 0)
              return mutatingSuccess(node, { reason: 'scrolled_container_top' })
            case 'bottom':
              scrollToY(container, container.scrollHeight)
              return mutatingSuccess(node, { reason: 'scrolled_container_bottom' })
            case 'up':
              scrollByY(container, -container.clientHeight * 0.8)
              return mutatingSuccess(node, { reason: 'scrolled_container_up' })
            case 'down':
              scrollByY(container, container.clientHeight * 0.8)
              return mutatingSuccess(node, { reason: 'scrolled_container_down' })
          }
        }

        // #333: For body/html targets, find the actual scrollable container in SPA layouts
        if (tag === 'body' || tag === 'html') {
          const scrollable = findScrollableContainer(document.body)
          if (scrollable) {
            scrollable.scrollIntoView({ behavior: 'smooth', block: 'center' })
            return mutatingSuccess(node, { reason: 'scrolled_nested_container' })
          }
        }

        // If element is inside a scrollable container, scroll it into view within that container
        const parentContainer = findScrollableContainer(node)
        if (parentContainer && parentContainer !== document.documentElement) {
          node.scrollIntoView({ behavior: 'smooth', block: 'center' })
          return mutatingSuccess(node, { reason: 'scrolled_within_container' })
        }

        node.scrollIntoView({ behavior: 'smooth', block: 'center' })
        return mutatingSuccess(node)
      },

      wait_for: () => ({ success: true, action, selector, value: node.tagName.toLowerCase() }),

      wait_for_text: () => {
        const searchText = options.text || ''
        if (!searchText) {
          return { success: false, action, selector: '', error: 'empty_text', message: 'text parameter is required for wait_for_text' } as DOMResult
        }
        const bodyText = document.body?.innerText ?? ''
        if (bodyText.includes(searchText)) {
          return { success: true, action, selector: '', matched_text: searchText } as DOMResult
        }
        return { success: false, action, selector: '', error: 'text_not_found' } as DOMResult
      },

      wait_for_absent: () => {
        if (!selector) {
          return { success: false, action, selector: '', error: 'missing_selector', message: 'selector is required for wait_for_absent' } as DOMResult
        }
        const el = resolveElement(selector)
        if (!el) {
          return { success: true, action, selector, absent: true } as DOMResult
        }
        return { success: false, action, selector, error: 'element_still_present' } as DOMResult
      },

  // --- PARTIAL: Input & Interaction Action Handlers ---
  // Purpose: paste, key_press, hover.
  // #502: open_composer, submit_active_composer, confirm_top_dialog extracted to dom-primitives-intent.ts.
  // Why: Separated from main template to keep each partial under 500 LOC.

      paste: () =>
        withMutationTracking(() => {
          const overlayErr = blockedByOverlayError(node)
          if (overlayErr) return overlayErr

          if (!(node instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`)
          node.focus()
          if (options.clear) {
            const selection = document.getSelection()
            if (selection) {
              selection.selectAllChildren(node)
              selection.deleteFromDocument()
            }
          }
          // Normalize literal \n sequences to actual newlines (MCP parameter encoding)
          const pasteText = (options.text || '').replace(/\\n/g, '\n')
          let strategy: string

          // Try rich editor native insertion first
          const editor = detectRichEditor(node)
          if (editor && node.isContentEditable) {
            insertViaRichEditor(editor.type, editor.target, pasteText, !!options.clear)
            strategy = editor.type + '_native'
          } else {
            // Fallback: synthetic ClipboardEvent (existing behavior)
            const dt = new DataTransfer()
            dt.setData('text/plain', pasteText)
            const event = new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true })
            node.dispatchEvent(event)
            strategy = 'clipboard_event'
          }

          return mutatingSuccess(node, { value: node.innerText, insertion_strategy: strategy })
        }),

      key_press: () =>
        withMutationTracking(() => {
          const overlayErr = blockedByOverlayError(node)
          if (overlayErr) return overlayErr

          if (!(node instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`)
          const key = options.text || options.key || 'Enter'

          // Tab/Shift+Tab: manually move focus (dispatchEvent can't trigger native tab traversal)
          if (key === 'Tab' || key === 'Shift+Tab') {
            const focusable = Array.from(
              node.ownerDocument.querySelectorAll(
                'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
              )
            ).filter((e) => (e as HTMLElement).offsetParent !== null) as HTMLElement[]
            const idx = focusable.indexOf(node)
            const next = key === 'Shift+Tab' ? focusable[idx - 1] : focusable[idx + 1]
            if (next) {
              next.focus()
              return mutatingSuccess(node, { value: key })
            }
            return mutatingSuccess(node, { value: key, message: 'No next focusable element' })
          }

          const keyMap: Record<string, { key: string; code: string; keyCode: number }> = {
            Enter: { key: 'Enter', code: 'Enter', keyCode: 13 },
            Tab: { key: 'Tab', code: 'Tab', keyCode: 9 },
            Escape: { key: 'Escape', code: 'Escape', keyCode: 27 },
            Backspace: { key: 'Backspace', code: 'Backspace', keyCode: 8 },
            ArrowDown: { key: 'ArrowDown', code: 'ArrowDown', keyCode: 40 },
            ArrowUp: { key: 'ArrowUp', code: 'ArrowUp', keyCode: 38 },
            Space: { key: ' ', code: 'Space', keyCode: 32 }
          }
          const mapped = keyMap[key] || { key, code: key, keyCode: 0 }
          node.dispatchEvent(
            new KeyboardEvent('keydown', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true })
          )
          node.dispatchEvent(
            new KeyboardEvent('keypress', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true })
          )
          node.dispatchEvent(
            new KeyboardEvent('keyup', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true })
          )
          return mutatingSuccess(node, { value: key })
        }),

      // #502: open_composer, submit_active_composer, confirm_top_dialog removed —
      // now in dom-primitives-intent.ts (self-contained, dispatched by dom-dispatch.ts).

      hover: () =>
        withMutationTracking(() => {
          if (!(node instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`)
          const rect = node.getBoundingClientRect()
          const centerX = rect.left + rect.width / 2
          const centerY = rect.top + rect.height / 2
          const eventInit = { bubbles: true, cancelable: true, clientX: centerX, clientY: centerY }
          node.dispatchEvent(new MouseEvent('mouseenter', { ...eventInit, bubbles: false }))
          node.dispatchEvent(new MouseEvent('mouseover', eventInit))
          node.dispatchEvent(new MouseEvent('mousemove', eventInit))
          return mutatingSuccess(node)
        }),

  // --- PARTIAL: Overlay & Stability Action Handlers ---
  // #502: Extracted to self-contained modules for < 800 LOC file sizes.
  //   - dom-primitives-overlay.ts: dismiss_top_overlay, auto_dismiss_overlays
  //   - dom-primitives-stability.ts: wait_for_stable, action_diff
  // These actions are dispatched directly by dom-dispatch.ts and no longer go through domPrimitive.
    }
  }

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
