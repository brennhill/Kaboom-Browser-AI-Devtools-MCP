---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Design Spec: Framework & Routing Detection for `page_structure`

**Issue:** #341
**Status:** proposed
**Author:** design-spec (auto)
**Date:** 2026-02-28

---

## 1. Problem Statement

The current `analyze(what='page_structure')` implementation detects frameworks and routing at a surface level. It identifies a few frameworks via window globals and DOM markers, but:

- Version detection is incomplete (React version never extracted, Angular version only in ISOLATED mode).
- Router detection is limited to hash presence and Next.js/Nuxt markers -- no detection of React Router, Vue Router, Angular Router, or SvelteKit routing.
- No meta-framework distinction (e.g., the response says "React" but not "Remix" or "Gatsby").
- No build tool / bundler signals (Vite, Webpack, Turbopack).
- No SSR vs. CSR classification.
- No micro-frontend detection.

This spec defines a comprehensive detection strategy that replaces the current framework/routing sections while preserving the existing scroll, modal, shadow DOM, and meta analysis unchanged.

---

## 2. Frameworks to Detect

### 2.1 Core Frameworks

| Framework | MAIN World Signals | ISOLATED (DOM-only) Signals |
|-----------|-------------------|----------------------------|
| React | `_reactRootContainer` on root, `__REACT_DEVTOOLS_GLOBAL_HOOK__` | `data-reactroot`, `data-reactid`, `__reactFiber$` prefix on element properties |
| Vue 2 | `window.Vue`, `__VUE_DEVTOOLS_GLOBAL_HOOK__` | `data-v-*` scoped attributes, `data-vue-meta` |
| Vue 3 | `window.__VUE__`, `__VUE_DEVTOOLS_GLOBAL_HOOK__.Vue` | `data-v-*` scoped attributes, `#app` with `__vue_app__` |
| Angular | `window.ng`, `getAllAngularRootElements()` | `ng-version` attribute, `_nghost-*` / `_ngcontent-*` attributes, `<app-root>` |
| Svelte | None (compiles away) | `class="svelte-XXXXX"` hash classes |
| Preact | `window.__PREACT_DEVTOOLS__` | `data-reactroot` (Preact compat), smaller bundle size heuristic via script tags |
| Lit / Web Components | `window.litElementVersions` | Heavy `<template>` + shadow DOM usage, custom element registrations |
| Solid | `window._$HY` (hydration marker) | `data-hk` hydration attributes |
| Alpine.js | `window.Alpine` | `x-data`, `x-bind`, `x-on` attributes |
| htmx | `window.htmx` | `hx-get`, `hx-post`, `hx-swap` attributes |
| jQuery | `window.jQuery`, `window.$` | None (no reliable DOM signal) |
| Ember | `window.Ember`, `window.Em` | `id="ember*"` element IDs |

### 2.2 Meta-Frameworks

| Meta-Framework | MAIN World Signals | ISOLATED (DOM-only) Signals |
|----------------|-------------------|----------------------------|
| Next.js | `window.__NEXT_DATA__` | `#__next`, `<script id="__NEXT_DATA__">`, `/_next/` script paths |
| Nuxt 2 | `window.$nuxt` | `#__nuxt`, `#__layout`, `/_nuxt/` script paths |
| Nuxt 3 | `window.__NUXT__` | `#__nuxt`, `<script type="application/json" id="__NUXT_DATA__">` |
| Remix | `window.__remixContext` | `<script id="__remix-*">`, `<link rel="modulepreload">` patterns |
| Gatsby | `window.___gatsby` | `#___gatsby`, `gatsby-*` IDs |
| SvelteKit | `window.__sveltekit_*` | `<script>` containing `__sveltekit`, `data-sveltekit-*` attributes |
| Astro | `window.__astro_*` (rare) | `<astro-island>` custom elements, `astro-*` attributes |
| Qwik | `window.qwikloader` | `q:container`, `on:qvisible` attributes |

### 2.3 Version Extraction

For each detected framework, attempt version extraction in this priority order:

1. **Window globals:** `window.React.version`, `window.Vue.version`, `window.ng.VERSION.full`, `window.jQuery.fn.jquery`
2. **DOM attributes:** `[ng-version]` attribute value
3. **Script content inspection:** Parse `__NEXT_DATA__` JSON for `buildId`, check `<script>` src paths for version patterns like `/react@18.2.0/`
4. **Generator meta tag:** `<meta name="generator" content="Next.js">`, `<meta name="generator" content="Gatsby 5.x">`

Version is best-effort. Return empty string when not determinable.

---

## 3. Router Detection

### 3.1 Router Type Identification

| Router | Detection Method |
|--------|-----------------|
| React Router (v5/v6) | `window.__reactRouterVersion`, `data-reactrouter` attr, `<a data-discover>` (v7), `RouterProvider` in React fiber tree |
| Vue Router | `window.__VUE_DEVTOOLS_GLOBAL_HOOK__?.Vue?.config?.globalProperties?.$router`, `<router-view>` / `<router-link>` elements |
| Angular Router | `<router-outlet>` element, `routerLink` attributes |
| SvelteKit Router | `data-sveltekit-*` link attributes |
| Next.js Router | Presence of `__NEXT_DATA__` (implies file-based routing) |
| Nuxt Router | Presence of `__NUXT__` (implies file-based routing) |
| TanStack Router | `window.__TSR__`, `<script>` containing `@tanstack/router` |
| Wouter | `window.__wouter` (rare), small bundle heuristic |

### 3.2 Routing Mode

Classify the active routing mode:

| Mode | Evidence |
|------|----------|
| `history` | Framework router detected + no hash in URL, or `pushState` / `replaceState` monkey-patched |
| `hash` | `window.location.hash.length > 1` with hash containing path-like segments (`#/dashboard`) |
| `file_based` | Next.js, Nuxt, SvelteKit, or Remix detected (these use file-system routing conventions) |
| `memory` | Router detected but no URL changes observed (e.g., test environments) |
| `static` | No SPA router detected; traditional multi-page navigation |
| `unknown` | Cannot determine |

### 3.3 Navigation Pattern Detection

In addition to router type, detect navigation-relevant patterns:

- **Link elements:** Count `<a>` with `href` starting with `/` vs. external links.
- **Router links:** Count framework-specific link components (`<router-link>`, `<Link>`, `[routerLink]`).
- **Programmatic navigation indicators:** `data-navigate`, `onclick` handlers on non-`<a>` elements that resemble navigation.

---

## 4. Rendering Mode Detection

Classify whether the page was server-rendered or client-rendered:

| Mode | Heuristic |
|------|-----------|
| `ssr` | Visible text content exists before any JavaScript runs (check `<noscript>` fallback, `__NEXT_DATA__` with `"page"` key, `data-server-rendered` attr) |
| `ssg` | Generator meta tag indicates static generation (Gatsby, Astro, Next.js export) |
| `csr` | Root container is empty (`#root` or `#app` with no children in initial HTML), content appears after hydration |
| `hybrid` | Mix of server-rendered and client-rendered regions (Next.js App Router with server components) |
| `unknown` | Cannot determine |

Evidence-based heuristics (no guaranteed accuracy -- documented as best-effort):

- **SSR indicator:** `<div id="__next">` with substantial child content in DOM at script execution time.
- **CSR indicator:** `<div id="root"></div>` with zero children before React hydration (detectable only in MAIN world by checking before/after state, which we cannot do retrospectively -- so we infer from `__NEXT_DATA__` presence or absence).
- **Hydration markers:** `data-reactroot`, `data-server-rendered="true"` (Vue SSR), Solid's `data-hk`.

---

## 5. Output Schema

The `frameworks` and `routing` fields in the existing `PageStructureResult` are replaced with richer types. All other fields (scroll_containers, modals, shadow_roots, meta) remain unchanged.

```typescript
interface FrameworkInfo {
  name: string           // e.g., "React", "Vue", "Angular"
  version: string        // e.g., "18.2.0", "" if unknown
  evidence: string       // What signal was used: "window.__NEXT_DATA__", "ng-version attr"
  confidence: "high" | "medium" | "low"
}

interface MetaFrameworkInfo {
  name: string           // e.g., "Next.js", "Nuxt", "Remix"
  version: string
  evidence: string
  confidence: "high" | "medium" | "low"
}

interface RoutingInfo {
  router: string         // e.g., "react-router", "vue-router", "next", "angular-router", "none"
  mode: string           // "history" | "hash" | "file_based" | "static" | "memory" | "unknown"
  evidence: string
  router_links_count: number   // Count of framework router-link elements
  internal_links_count: number // Count of <a href="/..."> elements
}

interface RenderingInfo {
  mode: string           // "ssr" | "ssg" | "csr" | "hybrid" | "unknown"
  evidence: string
  has_hydration_markers: boolean
}

interface PageStructureResult {
  frameworks: FrameworkInfo[]
  meta_frameworks: MetaFrameworkInfo[]
  routing: RoutingInfo
  rendering: RenderingInfo
  scroll_containers: ScrollContainer[]  // unchanged
  modals: ModalInfo[]                   // unchanged
  shadow_roots: number                  // unchanged
  meta: MetaInfo                        // unchanged
}
```

### 5.1 Confidence Levels

- **high**: Direct global variable or unique DOM marker (e.g., `window.__NEXT_DATA__`, `[ng-version]`).
- **medium**: Indirect signal (e.g., `#__next` div exists but no `__NEXT_DATA__` global -- could be custom).
- **low**: Heuristic-based (e.g., Svelte inferred from `class="svelte-xxxxx"` pattern, which could be custom CSS).

---

## 6. Execution Strategy

### 6.1 Two-World Approach (Preserved)

The existing pattern of trying MAIN world first and falling back to ISOLATED world is correct and should be kept:

1. **MAIN world** (`useGlobals=true`): Access `window.*` globals for high-confidence detection. Fails on pages with strict CSP.
2. **ISOLATED world fallback** (`useGlobals=false`): DOM-only heuristics. Always works but produces lower-confidence results.

### 6.2 Performance Budget

The script runs synchronously in the page context. Budget: **< 50ms** total execution.

- Framework detection (globals check): < 5ms
- DOM marker scanning: < 20ms (use `querySelector` not `querySelectorAll` where possible)
- Script tag inspection: < 10ms (limit to first 20 `<script>` elements)
- Scroll/modal/shadow DOM scanning: existing budget (already capped with MAX limits)

### 6.3 Script Tag Inspection

For meta-framework and bundler detection, inspect `<script>` elements:

```typescript
const scripts = document.querySelectorAll('script[src]')
// Cap at 30 to avoid performance issues on script-heavy pages
const scriptSrcs = Array.from(scripts).slice(0, 30).map(s => s.src)
```

Look for path patterns:
- `/_next/` -> Next.js
- `/_nuxt/` -> Nuxt
- `/@vite/` or `/@fs/` -> Vite dev server
- `/webpack-` or `bundle.js` -> Webpack
- `chunk-` patterns -> Code splitting (any bundler)

---

## 7. Edge Cases

### 7.1 CSP-Restricted Pages

When MAIN world execution fails due to Content Security Policy:
- Fall back to ISOLATED world (existing behavior).
- Mark all detections with `confidence: "medium"` or `"low"` since we lack global access.
- Add `"csp_restricted": true` to the top-level result so the consumer knows detection quality is degraded.

### 7.2 Multiple Frameworks

Real-world pages sometimes load multiple frameworks (e.g., React + jQuery, Angular migrating to React). Return all detected frameworks in the `frameworks` array. Do not deduplicate -- let the consumer decide relevance.

Preact with compat mode will appear as both "React" and "Preact". If both `__PREACT_DEVTOOLS__` and `_reactRootContainer` are found, report only "Preact" and set evidence to "preact-compat".

### 7.3 Micro-Frontends

Pages using Module Federation, single-spa, or iframe-based micro-frontends may host multiple frameworks. Detection strategy:

- Check for `window.__SINGLE_SPA_DEVTOOLS__` or `single-spa` in script sources.
- If detected, add a top-level `micro_frontend: true` boolean.
- Framework detection will naturally return multiple entries.

### 7.4 Obfuscated / Minified Builds

Production builds may strip devtools hooks (`__REACT_DEVTOOLS_GLOBAL_HOOK__`, `__VUE_DEVTOOLS_GLOBAL_HOOK__`). Rely on DOM markers as fallback:

- React: `_reactFiber$` or `_reactProps$` property prefix on DOM elements (check first child of root).
- Vue: `__vue_app__` property on mount element (Vue 3) or `__vue__` (Vue 2).
- Angular: `ng-version` attribute persists in production.

### 7.5 Web Components / Vanilla JS

Pages without any framework should return `frameworks: []` and `routing: { router: "none", mode: "static" }`. This is a valid result, not an error.

### 7.6 iframes

The current implementation runs on the top-level document only. Iframe framework detection is out of scope for this iteration. If the `frame` parameter is provided, detection should run in that frame context using the existing frame targeting infrastructure.

---

## 8. What Is NOT in Scope

- **State management detection** (Redux, Vuex, Pinia, Zustand) -- useful but separate concern.
- **CSS framework detection** (Tailwind, Bootstrap) -- could be a future addition but not part of #341.
- **Build tool detection** (Webpack, Vite version) -- mentioned for script path heuristics only, not a dedicated output field.
- **Performance profiling** -- handled by existing `analyze(what='performance')`.
- **Continuous monitoring** -- this is a one-shot analysis, not an observer.

---

## 9. Migration from Current Schema

The current `FrameworkInfo` and `RoutingInfo` types are replaced. Since `page_structure` is a new feature that has not shipped in a stable release, no backward-compatibility shim is needed. The wire type changes:

- `frameworks[]` gains `confidence` field.
- New `meta_frameworks[]` array added.
- `routing` gains `router`, `mode`, `router_links_count`, `internal_links_count` fields; drops the bare `type` field.
- New `rendering` object added.
- New optional `csp_restricted` boolean at top level.
- New optional `micro_frontend` boolean at top level.

Wire type files to update: `wire_analyze.go` (if exists) and corresponding TypeScript types in the command file.

---

## 10. Testing Strategy

### Unit Tests (Go side)

- Dispatch and schema tests (already exist in `tools_analyze_page_structure_test.go`).
- Validate new fields are present in response structure.
- Ensure `snake_case` on all output fields.

### Extension Tests (TypeScript)

- Mock `window` globals for each framework and verify detection.
- Mock DOM with framework-specific markers and verify ISOLATED world detection.
- Performance test: detection script completes in < 50ms on a DOM with 1000 elements.
- Edge case: no frameworks detected returns empty arrays, not errors.
- Edge case: multiple frameworks detected returns all of them.

### Integration / UAT

- Run against known sites: a Next.js app, a Vue/Nuxt app, an Angular app, a static HTML page.
- Verify CSP fallback works on a page with strict CSP.
- Verify the response is valid JSON with all required fields.

---

## 11. Implementation Order

1. Expand `FrameworkInfo` type and add `MetaFrameworkInfo`, `RoutingInfo`, `RenderingInfo` types.
2. Implement MAIN world detection for all frameworks in Section 2.
3. Implement ISOLATED world fallback for all frameworks.
4. Implement router detection (Section 3).
5. Implement rendering mode detection (Section 4).
6. Add confidence scoring.
7. Add micro-frontend and CSP-restricted flags.
8. Update Go wire types.
9. Write tests (per TDD, tests first for each sub-step).
10. Run UAT against real sites.
