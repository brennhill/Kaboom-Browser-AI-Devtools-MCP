/**
 * Purpose: Self-contained DOM query primitive for interact(what='query').
 * Why: Enables non-destructive element queries (exists, count, text_all, attributes)
 *      without erroring on missing elements. Complements get_text/get_attribute.
 * Docs: docs/features/feature/interact-explore/index.md
 */

// dom-primitives-query.ts — Self-contained query DOM primitive for chrome.scripting.executeScript.
// This function MUST remain self-contained — Chrome serializes the function source only (no closures).

/**
 * Self-contained function that queries the DOM for element existence, count, text, or attributes.
 * Unlike get_text/get_attribute which error on missing elements, this returns structured results
 * with exists=false or count=0 when no elements match.
 *
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveQuery }).
 * MUST NOT reference any module-level variables.
 */
export function domPrimitiveQuery(
  selector: string,
  options?: {
    query_type?: string
    attribute_names?: string[]
    scope_selector?: string
  }
): {
  success: boolean
  query_type: string
  selector: string
  exists?: boolean
  count?: number
  text?: string | null
  texts?: string[]
  attributes?: Record<string, string | null>
  error?: string
  message?: string
} {
  const queryType = options?.query_type || 'exists'

  // — Selector resolution (duplicated from dom-primitives for self-containment) —

  function resolveByText(text: string, root: ParentNode): Element[] {
    const results: Element[] = []
    const walker = document.createTreeWalker(root as Node, NodeFilter.SHOW_ELEMENT, null)
    let node: Node | null = walker.currentNode
    while (node) {
      if (node instanceof HTMLElement) {
        const nodeText = (node.textContent || '').trim()
        if (nodeText === text || nodeText.startsWith(text)) {
          results.push(node)
        }
      }
      node = walker.nextNode()
    }
    return results
  }

  function resolveElements(sel: string, root: ParentNode = document): Element[] {
    if (!sel || !sel.trim()) return []
    const trimmed = sel.trim()

    // Semantic selectors
    if (trimmed.startsWith('text=')) {
      return resolveByText(trimmed.slice(5), root)
    }
    if (trimmed.startsWith('role=')) {
      const role = CSS.escape(trimmed.slice(5))
      return Array.from(root.querySelectorAll(`[role="${role}"]`))
    }
    if (trimmed.startsWith('placeholder=')) {
      const ph = CSS.escape(trimmed.slice(12))
      return Array.from(root.querySelectorAll(`[placeholder="${ph}"]`))
    }
    if (trimmed.startsWith('label=')) {
      const label = CSS.escape(trimmed.slice(6))
      return Array.from(root.querySelectorAll(`[aria-label="${label}"]`))
    }
    if (trimmed.startsWith('aria-label=')) {
      const label = CSS.escape(trimmed.slice(11))
      return Array.from(root.querySelectorAll(`[aria-label="${label}"]`))
    }

    // CSS selector
    try {
      return Array.from(root.querySelectorAll(trimmed))
    } catch {
      return []
    }
  }

  // Resolve scope root
  function resolveScopeRoot(): ParentNode {
    if (!options?.scope_selector) return document
    try {
      const scopeEl = document.querySelector(options.scope_selector)
      if (scopeEl) return scopeEl
    } catch {
      // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
      // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
      /* use document */
    }
    return document
  }

  const elements = resolveElements(selector, resolveScopeRoot())

  function queryExists(): { success: boolean; query_type: string; selector: string; exists: boolean; count: number } {
    return {
      success: true,
      query_type: queryType,
      selector,
      exists: elements.length > 0,
      count: elements.length
    }
  }

  function queryCount(): { success: boolean; query_type: string; selector: string; count: number } {
    return {
      success: true,
      query_type: queryType,
      selector,
      count: elements.length
    }
  }

  function queryText():
    | { success: boolean; query_type: string; selector: string; exists: boolean; text: string | null }
    | { success: boolean; query_type: string; selector: string; exists: boolean; text: null } {
    if (elements.length === 0) {
      return {
        success: true,
        query_type: queryType,
        selector,
        exists: false,
        text: null
      }
    }
    const el = elements[0] as HTMLElement
    const text = el.innerText ?? el.textContent ?? null
    return {
      success: true,
      query_type: queryType,
      selector,
      exists: true,
      text: text ? text.trim() : text
    }
  }

  function queryTextAll(): { success: boolean; query_type: string; selector: string; count: number; texts: string[] } {
    const texts: string[] = []
    const limit = Math.min(elements.length, 100)
    for (let i = 0; i < limit; i++) {
      const el = elements[i] as HTMLElement
      const text = el.innerText ?? el.textContent ?? ''
      texts.push(text.trim())
    }
    return {
      success: true,
      query_type: queryType,
      selector,
      count: elements.length,
      texts
    }
  }

  function queryAttributes():
    | { success: false; query_type: string; selector: string; error: string; message: string }
    | {
        success: true
        query_type: string
        selector: string
        exists: boolean
        attributes: Record<string, string | null>
      } {
    const attrNames = options?.attribute_names || []
    if (attrNames.length === 0) {
      return {
        success: false,
        query_type: queryType,
        selector,
        error: 'missing_attribute_names',
        message: 'attribute_names parameter is required for query_type "attributes"'
      }
    }
    if (elements.length === 0) {
      return {
        success: true,
        query_type: queryType,
        selector,
        exists: false,
        attributes: {}
      }
    }
    const el = elements[0]!
    const attrs: Record<string, string | null> = {}
    for (const name of attrNames.slice(0, 20)) {
      attrs[name] = el.getAttribute(name)
    }
    return {
      success: true,
      query_type: queryType,
      selector,
      exists: true,
      attributes: attrs
    }
  }

  switch (queryType) {
    case 'exists':
      return queryExists()

    case 'count':
      return queryCount()

    case 'text':
      return queryText()

    case 'text_all':
      return queryTextAll()

    case 'attributes':
      return queryAttributes()

    default:
      return {
        success: false,
        query_type: queryType,
        selector,
        error: 'invalid_query_type',
        message: `Unknown query_type "${queryType}". Valid types: exists, count, text, text_all, attributes`
      }
  }
}
