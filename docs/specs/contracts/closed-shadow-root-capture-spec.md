---
feature: Closed Shadow Root Capture
status: proposed
depends_on: shadow-dom-support-spec.md (open root support ships first)
last_reviewed: 2026-02-16
---

# Tech Spec: Closed Shadow Root Capture

> Plain language only. Describes HOW the implementation works.

## Problem

`element.shadowRoot` returns `null` for closed shadow roots. Once `attachShadow({ mode: 'closed' })` executes, the ShadowRoot reference is gone forever — unless intercepted at creation time.

Without interception, Kaboom's deep traversal engine (from the open shadow DOM spec) stops at every closed boundary. Components using closed roots include: Salesforce Lightning, some Shopify Polaris internals, banking/fintech widgets, and any security-conscious web component.

## Approach

Monkey-patch `Element.prototype.attachShadow` in `early-patch.ts` to capture closed ShadowRoot references in a WeakMap before page JavaScript runs. This follows the exact same pattern Kaboom already uses for WebSocket capture: early-patch saves originals, inject script adopts them.

## Architecture

```
Timeline (per page load):
──────────────────────────────────────────────────────────────

1. early-patch.bundled.js (MAIN world, document_start)
   └─ Patches Element.prototype.attachShadow
   └─ Stores closed ShadowRoots in WeakMap on window

2. Page JavaScript runs
   └─ Calls attachShadow({ mode: 'closed' }) — captured transparently

3. inject.bundled.js loads (MAIN world, programmatic)
   └─ Adopts WeakMap from window global
   └─ Cleans up globals

4. dom-primitives.ts executeScript calls (MAIN world, on-demand)
   └─ Reads WeakMap to traverse closed roots
```

## Implementation

### 1. Early Patch: `src/early-patch.ts`

Add the `attachShadow` patch alongside the existing WebSocket patch. Same IIFE, same file, same bundle.

```typescript
// --- Closed Shadow Root Capture ---

const OriginalAttachShadow = Element.prototype.attachShadow
if (OriginalAttachShadow) {
  const closedRoots = new WeakMap<Element, ShadowRoot>()
  window.__KABOOM_CLOSED_SHADOWS__ = closedRoots
  window.__KABOOM_ORIGINAL_ATTACH_SHADOW__ = OriginalAttachShadow

  Element.prototype.attachShadow = function (
    this: Element,
    init: ShadowRootInit
  ): ShadowRoot {
    const root = OriginalAttachShadow.call(this, init)
    if (init.mode === 'closed') {
      closedRoots.set(this, root)
    }
    return root
  }
}
```

**Why this is safe:**
- `attachShadow` is a method, not a constructor — no prototype chain to preserve
- The patched function calls through to the original with identical arguments
- Return value is the real ShadowRoot (not a wrapper)
- WeakMap means captured roots are GC'd when their host element is removed
- No observable side-effects: `init.mode` is still `'closed'`, `element.shadowRoot` still returns `null`

**Why early-patch and not inject:**
- `attachShadow` is called during custom element construction, which often happens during HTML parsing
- If a `<my-component>` tag appears in the initial HTML, its constructor runs during parse — before inject.bundled.js loads
- `early-patch.bundled.js` runs at `document_start` via manifest declaration — this is the only reliable interception point

### 2. Type Declarations: `src/types/global.d.ts`

Add to the `Window` interface:

```typescript
/** Early-patch: captured closed shadow roots (host → ShadowRoot) */
__KABOOM_CLOSED_SHADOWS__?: WeakMap<Element, ShadowRoot>

/** Early-patch: original attachShadow saved before patch */
__KABOOM_ORIGINAL_ATTACH_SHADOW__?: typeof Element.prototype.attachShadow
```

### 3. Adoption: `src/inject/shadow-registry.ts` (new file)

Follows the same adoption pattern as `lib/websocket.ts:586-730`:

```typescript
// shadow-registry.ts — Adopts captured closed shadow roots from early-patch.

let closedRoots: WeakMap<Element, ShadowRoot> | null = null

/**
 * Adopt the closed shadow root registry from early-patch globals.
 * Called once during inject Phase 1 initialization.
 */
export function adoptClosedShadowRoots(): void {
  closedRoots = window.__KABOOM_CLOSED_SHADOWS__ ?? null

  // Clean up globals
  delete window.__KABOOM_CLOSED_SHADOWS__
  delete window.__KABOOM_ORIGINAL_ATTACH_SHADOW__
}

/**
 * Look up a closed shadow root by its host element.
 * Returns null if the element has no captured closed root.
 */
export function getClosedShadowRoot(host: Element): ShadowRoot | null {
  return closedRoots?.get(host) ?? null
}

/**
 * Teardown: release references. Called on extension unload.
 */
export function destroyShadowRegistry(): void {
  closedRoots = null
  delete window.__KABOOM_CLOSED_SHADOWS__
  delete window.__KABOOM_ORIGINAL_ATTACH_SHADOW__
}
```

### 4. Integration with Deep Traversal (`dom-primitives.ts`)

The deep traversal engine (from the open shadow DOM spec) needs access to closed roots. Since `domPrimitive` runs as a self-contained function via `chrome.scripting.executeScript`, it cannot import modules. The closed root WeakMap must be passed as an argument or accessed via a window global.

**Option A: Window global (recommended)**

The WeakMap stays on `window.__KABOOM_CLOSED_SHADOWS__` and `domPrimitive` reads it directly. The inject script does NOT delete the global — it only ensures the early-patch is cleaned up on unload.

```typescript
// Inside domPrimitive (self-contained)
function getShadowRoot(el: Element): ShadowRoot | null {
  if (el.shadowRoot) return el.shadowRoot // open
  const closed = (window as any).__KABOOM_CLOSED_SHADOWS__
  return closed?.get(el) ?? null
}
```

**Why not Option B (pass as arg):** `chrome.scripting.executeScript` serializes arguments. WeakMap is not serializable. The global is the only viable path.

**Implication:** The `adoptClosedShadowRoots()` function should NOT delete `window.__KABOOM_CLOSED_SHADOWS__`. Instead, it should only delete `__KABOOM_ORIGINAL_ATTACH_SHADOW__` (the saved original, which is no longer needed). The WeakMap must remain accessible for the lifetime of the page.

Revised adoption:

```typescript
export function adoptClosedShadowRoots(): void {
  closedRoots = window.__KABOOM_CLOSED_SHADOWS__ ?? null
  // Keep __KABOOM_CLOSED_SHADOWS__ on window — dom-primitives needs it
  // Only clean up the original function reference
  delete window.__KABOOM_ORIGINAL_ATTACH_SHADOW__
}
```

## Risks and Mitigations

### Risk 1: Page detects the patch

**Likelihood:** Low but possible. A page could save a reference to `Element.prototype.attachShadow` before early-patch runs (impossible — early-patch runs at `document_start` before any page script) or compare the function's `toString()` output.

**Mitigation:** None needed. This is standard practice for browser automation tools (Playwright, Puppeteer, Selenium all do this). Kaboom is a developer tool, not a stealth tool.

### Risk 2: Patch doesn't run before page JS

**Likelihood:** Very low. Chrome's `document_start` content script injection is the earliest possible timing in MV3. However, there are two edge cases:
- **Prerendered pages:** Chrome may not inject content scripts into prerendered pages until activation. Components constructed during prerender would be missed.
- **Extension reload:** If the extension reloads while a page is already loaded, the patch misses components that already called `attachShadow`.

**Mitigation:** The guard `if (window.__KABOOM_ORIGINAL_ATTACH_SHADOW__) return` prevents double-patching. Missed closed roots degrade gracefully — the deep traversal simply won't enter those roots, same as today. No errors, no crashes.

### Risk 3: WeakMap memory pressure

**Likelihood:** Negligible. WeakMap entries are keyed by the host Element. When the element is removed from DOM and GC'd, the WeakMap entry (and the ShadowRoot reference) is also collected. No unbounded growth.

### Risk 4: Framework interference

**Likelihood:** Low. Some frameworks (LitElement, Stencil) save a reference to `attachShadow` during module initialization. If their module loads before `early-patch.ts` (impossible in normal operation — `document_start` runs first), they'd bypass the patch. But even if a framework calls `Element.prototype.attachShadow` directly via a cached reference, the patch still works because we patch the prototype method, not instance methods.

**Edge case:** If a framework overwrites `Element.prototype.attachShadow` itself (extremely rare), it would overwrite our patch. We could detect this in inject.ts and log a warning.

### Risk 5: early-patch.ts bundle size increase

**Likelihood:** Certain but minor. The patch adds ~15 lines (~300 bytes minified) to the early-patch bundle. Current early-patch is ~800 bytes minified. A ~35% size increase is acceptable for a script that runs once at page load.

## Performance

- **attachShadow overhead:** One WeakMap `.set()` call per closed shadow root creation. WeakMap operations are O(1). Unmeasurable overhead.
- **Deep traversal overhead:** One WeakMap `.get()` call per element checked during traversal. Same O(1) cost. No overhead on pages without closed shadow roots (the WeakMap is simply empty).
- **Memory:** WeakMap holds strong references to ShadowRoot objects. A closed ShadowRoot stays in memory as long as its host element exists — this is identical to what happens with open shadow roots (the browser holds the same reference via `element.shadowRoot`). No additional memory pressure.

## Testing

### Unit Tests: `extension/background/closed-shadow-capture.test.js`

```
1. early-patch saves original attachShadow
2. Patched attachShadow captures closed roots in WeakMap
3. Patched attachShadow does NOT capture open roots (shadowRoot already accessible)
4. Original attachShadow behavior preserved (return value, mode, delegatesFocus)
5. Guard prevents double-patching on extension reload
6. WeakMap entries are per-host (multiple components work independently)
```

### Integration Tests: `tests/e2e/shadow-dom-closed.test.ts`

```
1. Create page with closed shadow root component
2. Verify list_interactive finds elements inside closed root
3. Verify click/type actions work on elements inside closed root
4. Verify get_text returns content from inside closed root
5. Verify selector generation produces working >>> selectors for closed roots
```

### Regression Tests

```
1. Pages with zero shadow DOM: no performance regression
2. Pages with only open shadow roots: behavior unchanged
3. attachShadow({ mode: 'open' }) still returns accessible shadowRoot
4. Extension unload cleans up globals
```

## Files Modified

| File | Change |
|------|--------|
| `src/early-patch.ts` | Add attachShadow patch (~15 lines) |
| `src/types/global.d.ts` | Add Window interface members (2 lines) |
| `src/inject/shadow-registry.ts` | New file: adoption + lookup (~30 lines) |
| `src/inject/index.ts` | Call `adoptClosedShadowRoots()` in Phase 1 |
| `src/background/dom-primitives.ts` | Add `getShadowRoot()` helper used by deep traversal |

## Dependencies

This spec **requires** the open shadow DOM support spec to ship first. The deep traversal engine (recursive `querySelectorAllDeep`, `>>>` combinator, TreeWalker cross-root walking) must exist before closed root support adds value. This spec only adds the **capture mechanism** — the traversal engine consumes it.

## Rollout

1. Ship open shadow DOM support (deep traversal for open roots)
2. Ship this spec (closed root capture via early-patch)
3. The deep traversal engine picks up closed roots automatically via `getShadowRoot()`
4. No feature flags needed — the patch is zero-cost on pages without closed roots
