---
feature: Shadow DOM Support (Open Roots)
status: proposed
related: closed-shadow-root-capture-spec.md (follow-up for closed roots)
last_reviewed: 2026-02-16
---

# Tech Spec: Shadow DOM Deep Traversal

> Plain language only. Describes HOW the implementation works.

## 1. Problem

AI agents cannot see or interact with elements inside shadow roots. Every resolver in `dom-primitives.ts` uses `document.querySelector`, `document.querySelectorAll`, or `document.createTreeWalker` — all of which stop at shadow boundaries by spec.

Affected sites: YouTube, Salesforce, Shopify admin, GitHub (Primer components), any site using Lit, Stencil, or native Web Components.

**Scope:** This spec covers **open** shadow roots only (`element.shadowRoot` accessible). Closed root support is a separate spec that builds on this one.

## 2. Design Principles

1. **Fast path first.** Try native `querySelector` before deep traversal. Zero overhead on pages without shadow DOM.
2. **No new APIs.** Existing selectors and actions become shadow-aware. LLMs don't need to learn anything new.
3. **Self-contained.** All logic must work inside `chrome.scripting.executeScript({ func })` — no imports, no closures.

## 3. Implementation

### 3.1 Shadow-Aware Element Resolver

Replace the current `resolveElement` fallback with a two-step strategy: try fast path, then deep path.

```typescript
// Inside domPrimitive (self-contained function)

function getShadowRoot(el: Element): ShadowRoot | null {
  return el.shadowRoot ?? null
  // Closed root support (future): also check window.__KABOOM_CLOSED_SHADOWS__
}

function querySelectorDeep(selector: string, root: ParentNode = document): Element | null {
  // Fast path — covers ~95% of pages
  const fast = root.querySelector(selector)
  if (fast) return fast

  // Deep path — recurse into shadow roots
  return querySelectorDeepWalk(selector, root)
}

function querySelectorDeepWalk(selector: string, root: ParentNode): Element | null {
  const children = root.children
  for (let i = 0; i < children.length; i++) {
    const child = children[i]!
    const shadow = getShadowRoot(child)
    if (shadow) {
      const match = shadow.querySelector(selector)
      if (match) return match
      // Recurse into this shadow root's children
      const deep = querySelectorDeepWalk(selector, shadow)
      if (deep) return deep
    }
    // Also check children of this element (they may host shadow roots)
    if (child.children.length > 0) {
      const deep = querySelectorDeepWalk(selector, child)
      if (deep) return deep
    }
  }
  return null
}
```

**Why `children` iteration instead of `querySelectorAll('*')`:**
- `querySelectorAll('*')` materializes the entire subtree into an array — on YouTube that's 10k+ elements allocated per call
- Walking `children` is lazy: we stop as soon as we find a match
- For `querySelector` (find first), most calls resolve within the first few shadow roots

### 3.2 Deep `querySelectorAll` (for `list_interactive`)

`list_interactive` needs all matches, not just the first. Different function:

```typescript
function querySelectorAllDeep(
  selector: string,
  root: ParentNode = document,
  results: Element[] = [],
  depth: number = 0
): Element[] {
  if (depth > 10) return results  // Safety cap

  results.push(...Array.from(root.querySelectorAll(selector)))

  const children = root.children
  for (let i = 0; i < children.length; i++) {
    const child = children[i]!
    const shadow = getShadowRoot(child)
    if (shadow) {
      querySelectorAllDeep(selector, shadow, results, depth + 1)
    }
  }
  return results
}
```

**Depth limit of 10:** Real-world shadow DOM nesting rarely exceeds 3-4 levels. 10 is generous. Prevents runaway recursion from pathological DOMs.

**Note:** This still calls `querySelectorAll` at each level, which materializes a NodeList per shadow root. This is acceptable because `list_interactive` already caps at 100 elements and runs infrequently (once per agent "look at the page" step). It is NOT acceptable in the hot path for `resolveElement` — which is why 3.1 uses the `children` walk instead.

### 3.3 Shadow-Aware Text Resolver

The current `resolveByText` uses `document.createTreeWalker` which cannot cross shadow boundaries. Replace with a custom recursive walker:

```typescript
function resolveByTextDeep(searchText: string): Element | null {
  let fallback: Element | null = null

  function walkNode(root: ParentNode): Element | null {
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT)
    while (walker.nextNode()) {
      const node = walker.currentNode
      if (node.textContent && node.textContent.trim().includes(searchText)) {
        const parent = node.parentElement
        if (!parent) continue
        const interactive = parent.closest(
          'a, button, [role="button"], [role="link"], label, summary'
        )
        const target = interactive || parent
        if (!fallback) fallback = target
        if (isVisible(target)) return target
      }
    }
    // Recurse into shadow roots
    const children = root.children || root.childNodes
    for (let i = 0; i < children.length; i++) {
      const child = children[i]
      if (child instanceof Element) {
        const shadow = getShadowRoot(child)
        if (shadow) {
          const result = walkNode(shadow)
          if (result) return result
        }
      }
    }
    return null
  }

  return walkNode(document.body || document.documentElement) || fallback
}
```

**How this works:** Create a TreeWalker per scope (document body, then each shadow root). At each level, walk all text nodes, then check all children for shadow hosts and recurse. The TreeWalker handles the text-node walking within a single scope efficiently; we handle the cross-root jumping manually.

The same pattern applies to `resolveByLabel` and `resolveByAriaLabel` — they use `querySelectorAll` internally, so they switch to `querySelectorAllDeep`.

### 3.4 Deep Combinator Syntax (`>>>`)

For agents to target a specific element inside a specific shadow root:

```
my-component >>> button.submit
app-shell >>> nav-bar >>> button.menu
```

Resolution logic:

```typescript
function resolveDeepCombinator(selector: string): Element | null {
  const parts = selector.split(' >>> ')
  if (parts.length === 1) return null  // not a deep selector

  let current: ParentNode = document
  for (let i = 0; i < parts.length; i++) {
    const part = parts[i]!.trim()
    if (i < parts.length - 1) {
      // Intermediate: find the host, then enter its shadow root
      const host = querySelectorDeep(part, current)
      if (!host) return null
      const shadow = getShadowRoot(host)
      if (!shadow) return null
      current = shadow
    } else {
      // Final: find the target inside the current root
      return querySelectorDeep(part, current)
    }
  }
  return null
}
```

**Chaining:** `a >>> b >>> c` resolves `a` in document, enters its shadow, resolves `b`, enters its shadow, resolves `c`. No limit on depth.

**Disambiguation:** `>>>` is Kaboom-specific syntax, never passed to native CSS APIs. It was a deprecated CSS proposal (`::shadow` and `/deep/` combinators, removed from spec in 2015) — no browser interprets it today.

### 3.5 Selector Generation for `list_interactive`

When listing elements found inside shadow roots, we need to generate selectors that work when passed back to `resolveElement`:

```typescript
function buildShadowSelector(el: Element): string | null {
  // Walk up to find if this element is inside a shadow root
  let node: Node | null = el
  const parts: string[] = []

  while (node) {
    const root = node.getRootNode()
    if (root instanceof ShadowRoot) {
      const host = root.host
      // Build selector for element within this shadow root
      const inner = buildUniqueSelector(el, el as HTMLElement, el.tagName.toLowerCase())
      parts.unshift(inner)
      // Move up to the host and continue
      node = host
      el = host
    } else {
      // We're in the document — build the host selector
      if (parts.length > 0) {
        const hostSelector = buildUniqueSelector(el, el as HTMLElement, el.tagName.toLowerCase())
        parts.unshift(hostSelector)
        return parts.join(' >>> ')
      }
      return null  // Element is not in a shadow root
    }
  }
  return null
}
```

**Disambiguation for multiple identical hosts:** If there are three `<user-card>` elements, `user-card >>> #edit-btn` is ambiguous. The selector generator falls back to the host's `nth-of-type`:

```
user-card:nth-of-type(2) >>> #edit-btn
```

### 3.6 `isVisible` Adjustment

The current `isVisible` uses `offsetParent === null` to detect hidden elements. This can return `null` for elements inside shadow roots whose host has `display: contents` or `position: fixed/sticky`. Add a fallback:

```typescript
function isVisible(el: Element): boolean {
  if (!(el instanceof HTMLElement)) return true
  const style = getComputedStyle(el)
  if (style.visibility === 'hidden' || style.display === 'none') return false
  // offsetParent is unreliable inside shadow DOM — check bounding rect as fallback
  if (el.offsetParent === null
    && style.position !== 'fixed'
    && style.position !== 'sticky') {
    const rect = el.getBoundingClientRect()
    if (rect.width === 0 && rect.height === 0) return false
  }
  return true
}
```

### 3.7 `domWaitFor` Shadow Support

`domWaitFor` (line 519 in dom-primitives.ts) has its own independent `resolveElement` and `MutationObserver`. Both need shadow awareness.

**Problem:** A `MutationObserver` on `document.documentElement` does NOT see mutations inside shadow roots. If an element appears inside a shadow root after page load, `domWaitFor` will time out.

**Solution:** Observe shadow roots too. When a mutation adds an element with a shadow root, attach an observer to that shadow root.

```typescript
function domWaitForDeep(selector: string, timeoutMs: number): Promise<DOMResult> {
  // ... existing resolveElement logic, upgraded to use querySelectorDeep ...

  return new Promise((resolve) => {
    // Check immediately with deep traversal
    const existing = querySelectorDeep(selector)
    if (existing) {
      resolve({ success: true, action: 'wait_for', selector, value: existing.tagName.toLowerCase() })
      return
    }

    let resolved = false
    const observers: MutationObserver[] = []

    function check() {
      const el = querySelectorDeep(selector)
      if (el && !resolved) {
        resolved = true
        clearTimeout(timer)
        observers.forEach(o => o.disconnect())
        resolve({ success: true, action: 'wait_for', selector, value: el.tagName.toLowerCase() })
      }
    }

    function observeRoot(root: Node) {
      const observer = new MutationObserver((mutations) => {
        check()
        // If new shadow hosts appeared, observe their roots too
        for (const mutation of mutations) {
          for (const added of mutation.addedNodes) {
            if (added instanceof Element) {
              const shadow = getShadowRoot(added)
              if (shadow) observeRoot(shadow)
            }
          }
        }
      })
      observer.observe(root, { childList: true, subtree: true })
      observers.push(observer)
    }

    // Observe document + all existing shadow roots
    observeRoot(document.documentElement)
    walkShadowRoots(document, (shadow) => observeRoot(shadow))

    const timer = setTimeout(() => {
      if (!resolved) {
        resolved = true
        observers.forEach(o => o.disconnect())
        resolve({
          success: false,
          action: 'wait_for',
          selector,
          error: 'timeout',
          message: `Element not found within ${timeoutMs}ms: ${selector}`
        })
      }
    }, timeoutMs)
  })
}

// Helper: walk all existing shadow roots in a subtree
function walkShadowRoots(root: ParentNode, callback: (shadow: ShadowRoot) => void) {
  const children = root.children
  for (let i = 0; i < children.length; i++) {
    const child = children[i]!
    const shadow = getShadowRoot(child)
    if (shadow) {
      callback(shadow)
      walkShadowRoots(shadow, callback)
    }
    if (child.children.length > 0) {
      walkShadowRoots(child, callback)
    }
  }
}
```

**Cost:** One additional MutationObserver per shadow root. On most pages this is 0-5 extra observers. On YouTube it could be 20-30. Each observer is lightweight (Chrome handles them efficiently), and they're all disconnected on resolution or timeout.

## 4. Performance

### 4.1 Fast Path Guarantee

The critical invariant: **pages without shadow DOM pay zero cost.**

```typescript
function resolveElement(sel: string): Element | null {
  // ... semantic prefix checks (unchanged) ...

  // Fast path: native querySelector
  const fast = document.querySelector(sel)
  if (fast) return fast

  // Deep path: only reached if fast path returned null
  return querySelectorDeepWalk(sel, document)
}
```

On a page with no shadow DOM, `querySelectorDeepWalk` finds zero shadow hosts and returns immediately. The overhead is one function call and a `children.length === 0` check at the document level.

### 4.2 Deep Path Cost

Worst case (YouTube-scale, ~50 shadow hosts):
- `querySelectorDeep` (find first): walks `children` lazily, typically resolves within 2-3 shadow roots. Measured estimate: <5ms.
- `querySelectorAllDeep` (find all, for `list_interactive`): visits all shadow roots. Measured estimate: 10-30ms on complex pages. Acceptable — `list_interactive` is not latency-sensitive.

### 4.3 Depth Limit

Recursion capped at 10 levels. Real-world nesting is rarely >3-4 levels. Prevents pathological DOMs from causing stack overflow.

### 4.4 Scope Restriction

When `frame` is provided, deep traversal only runs within that frame's document. No cross-frame shadow walking.

## 5. API Changes

### 5.1 No Schema Changes

This is an implicit upgrade. All existing actions become shadow-aware. LLMs don't need to change their behavior.

### 5.2 New Selector Syntax

`>>>` is the only visible addition. It appears in:
- Selectors returned by `list_interactive` for shadow DOM elements
- Selectors that agents can use in subsequent `click`, `type`, `get_text` calls

Example `list_interactive` response (new):
```json
{
  "tag": "button",
  "selector": "user-card >>> #edit-btn",
  "label": "Edit Profile",
  "visible": true
}
```

### 5.3 Frame Interaction

Frames and shadow DOM are orthogonal. `chrome.scripting.executeScript` runs per frame. Deep traversal happens within each frame's document independently. No special handling needed — the `>>>` selector is resolved inside the frame context where the script runs.

## 6. Testing

### 6.1 Unit Tests: `extension/background/dom-primitives-shadow.test.js`

Mock DOM structure:
```
document
├── <div id="light-btn"><button>Light</button></div>
├── <my-component>
│   └── #shadow-root (open)
│       ├── <button id="shadow-btn">Shadow</button>
│       └── <nested-comp>
│           └── #shadow-root (open)
│               └── <input placeholder="Deep Input">
└── <another-component>
    └── #shadow-root (open)
        └── <a href="/link">Shadow Link</a>
```

Tests:

**querySelectorDeep:**
```
1. Returns light DOM element on fast path (no shadow traversal)
2. Finds element in first-level shadow root
3. Finds element in nested shadow root (2 levels)
4. Returns null for non-existent element
5. Respects depth limit (returns null at depth 11)
```

**resolveByTextDeep:**
```
6. Finds text node in light DOM (unchanged behavior)
7. Finds text node inside shadow root
8. Finds text node in nested shadow root
9. Prefers visible match over hidden match across roots
```

**Deep combinator (>>>):**
```
10. Resolves single-level: my-component >>> #shadow-btn
11. Resolves chained: my-component >>> nested-comp >>> input
12. Returns null if host not found
13. Returns null if host has no shadow root
14. Returns null if inner selector not found
```

**list_interactive:**
```
15. Returns elements from both light DOM and shadow roots
16. Generated selectors for shadow elements use >>> syntax
17. Cap at 100 elements still works across shadow roots
```

**Selector generation:**
```
18. Light DOM element: no >>> in selector
19. Shadow element: host >>> inner
20. Nested shadow element: host >>> inner-host >>> target
21. Disambiguates multiple identical hosts with nth-of-type
```

**isVisible:**
```
22. Works for elements inside shadow roots
23. Handles display:contents shadow hosts correctly
```

### 6.2 Regression Tests

```
24. All existing dom-primitives tests pass unchanged
25. Pages with zero shadow DOM: resolveElement uses fast path only
26. Standard CSS selectors resolve without deep traversal when element is in light DOM
```

### 6.3 E2E Tests: `tests/e2e/shadow-dom.test.ts`

Playwright test with a real page containing web components:

```
27. Create test page with open shadow root components
28. list_interactive finds buttons inside shadow roots
29. click action works on shadow DOM button via >>> selector
30. type action works on shadow DOM input via >>> selector
31. get_text returns content from inside shadow root
32. text= semantic selector finds text inside shadow roots
33. wait_for detects element added dynamically inside shadow root
```

## 7. Security Considerations

Shadow DOM is a scoping mechanism, not a security boundary (unlike iframes). The extension already has full DOM access via `chrome.scripting.executeScript` in MAIN world. This feature does not grant new permissions — it uses the same APIs more thoroughly.

## 8. Files Modified

| File | Change |
|------|--------|
| `src/background/dom-primitives.ts` | Add deep traversal functions, update all resolvers, update `isVisible`, update `list_interactive`, add `>>>` combinator, add shadow-aware selector generation |
| `src/background/dom-primitives.ts` (`domWaitFor`) | Add shadow-aware MutationObserver strategy |
| `extension/background/dom-primitives-shadow.test.js` | New: unit tests (26 tests) |
| `tests/e2e/shadow-dom.test.ts` | New: E2E tests with real web components (7 tests) |

No Go server changes. No schema changes. No new files in `src/` (all logic is self-contained inside `domPrimitive` and `domWaitFor`).

## 9. Rollout

1. Ship this spec (open root deep traversal)
2. Ship closed-shadow-root-capture-spec.md (adds `getShadowRoot` WeakMap lookup)
3. Deep traversal automatically picks up closed roots — `getShadowRoot` is the single integration point
