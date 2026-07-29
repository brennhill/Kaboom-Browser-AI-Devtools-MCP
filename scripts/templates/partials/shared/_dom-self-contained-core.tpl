  // --- PARTIAL: Self-Contained DOM Traversal Core ---
  // Emitted into each injected function because Chrome serializes functions without module scope.

  function isKaboomOwnedElement(element: Element | null): boolean {
    let node: Element | null = element
    while (node) {
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
    const children =
      'children' in root
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
