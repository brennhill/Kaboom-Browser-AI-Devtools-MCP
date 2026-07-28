---
feature: Visual-Semantic Bridge
status: proposed
doc_type: tech-spec
feature_id: feature-visual-semantic-bridge
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Visual-Semantic Bridge

> Plain language only. Describes HOW the implementation works at a high level, without code.

## Architecture Overview

The Visual-Semantic Bridge extends Kaboom's DOM observation pipeline to enrich raw HTML with three layers of metadata:

1. **Computed Layout Layer** — Uses browser APIs to calculate visibility, bounding boxes, occlusion
2. **Semantic Identifier Layer** — Generates and injects unique, stable test-IDs into the DOM
3. **Component Mapping Layer** — Links DOM nodes back to source code (React/Vue components, file locations)

The enriched DOM is then passed through a **Smart Pruning Filter** that removes non-interactive noise before sending to the AI model.

## Key Components

### 1. Layout Calculator
**Purpose:** Compute visibility and position information for every interactive element.

**Inputs:** DOM element, document viewport

**Outputs:** For each element:
- `is_visible: boolean` — Passes all visibility checks (display !== none, visibility !== hidden, opacity > 0)
- `bounding_box: {top, left, width, height}` — In viewport coordinates
- `off_screen: boolean` — Falls outside viewport bounds
- `covered_by: {selector, element_id, z_index}` — If hit-test reveals occlusion

#### Algorithm:
1. For each interactive element (buttons, inputs, links, focusable divs)
2. Get computed styles via `getComputedStyle(element)`
3. Check visibility flags (display, visibility, opacity, pointer-events)
4. Get bounding box via `getBoundingClientRect()`
5. Test occlusion via hit-test: call `elementFromPoint(centerX, centerY)` and check if it returns the target element
6. If hit-test returns a different element, that element is "covering"

#### Edge cases:
- Elements with `visibility: hidden` vs `display: none` (both treated as not visible)
- Elements with `opacity: 0` (treated as not visible, but technically clickable)
- Elements covered by fixed/sticky overlays (modal backdrops, floating menus)
- Elements inside scrollable containers that are scrolled out of view
- SVG elements (use `getBBox()` instead of `getBoundingClientRect()`)

### 2. Semantic Identifier Generator
**Purpose:** Create unique, stable IDs for every interactive element at runtime.

#### Approach:
- Scan the DOM for interactive elements (role="button", `<button>`, `<a>`, `<input>`, focusable=true)
- For each element, generate a semantic ID based on its role + content
- Format: `data-kaboom-id="[type]-[hash]"` where:
  - `type` = element type (btn, link, input, checkbox, radio, etc.)
  - `hash` = 4-char alphanumeric hash of a stable identifier (text content, aria-label, aria-labelledby, name attribute)

#### Algorithm:
1. Identify element type from `role`, tag name, or semantic meaning
2. Extract stable label from: text content, `aria-label`, `aria-labelledby`, `title`, `placeholder`, `name`, `id`
3. Normalize label: lowercase, trim whitespace, remove punctuation
4. Hash via simple checksum (not cryptographic; we just need uniqueness)
5. Inject `data-kaboom-id` attribute into the element
6. Store mapping in memory for later retrieval

#### Uniqueness guarantees:
- If two elements have identical content, hash collision occurs; in this case, append a counter (-1, -2, etc.)
- Deterministic: same page state produces same IDs (important for testing)
- Stable across page reloads: based on semantic content, not DOM position

#### Durability:
- IDs are stored in a WeakMap keyed by DOM element reference
- When element is removed from DOM, entry is automatically garbage collected
- IDs are not persisted to localStorage (session-scoped only)

### 3. Component Mapper
**Purpose:** Link DOM nodes back to source code when using modern frameworks.

#### Approach:
- **React:** Access React DevTools hook via `__REACT_DEVTOOLS_GLOBAL_HOOK__`
- **Vue:** Access Vue DevTools hook via `__VUE__` and `__VUE_DEVTOOLS_GLOBAL_HOOK__`
- **Other frameworks:** Fall back to nearest semantic element (id, class, role, label)

#### Algorithm:
1. Check if React/Vue DevTools hooks are available
2. If React: walk the Fiber tree to find the component that rendered this DOM node
3. If Vue: use `__VUE_COMPONENT__` property on instances to identify component
4. Extract component name + relevant props (for display only; don't send prop values to context window for privacy)
5. If no framework hooks available, use semantic fallback (nearest id, class selector, role)

#### Privacy consideration:
- Component names are safe to share (SPA structure is visible in source anyway)
- Props may contain sensitive data; include names only, not values
- Store only component name + file path, no user/business data

### 4. Smart DOM Pruning Filter
**Purpose:** Remove non-interactive noise to fit DOM in <25% of context window.

#### Approach:
- Traverse DOM tree
- Keep only elements that are:
  - Interactive (buttons, inputs, links, focusable, or labeled)
  - Text content with semantic meaning (headings, labels, section text)
- Strip:
  - SVG paths (decorative)
  - Tracking pixels (analytics)
  - Hidden spacers and layout helpers
  - Script and style tags
  - Comments and empty nodes

#### Algorithm:
1. Build pruned DOM tree by filtering the full DOM
2. For each node:
   - If interactive: keep with all metadata
   - If text: keep if word length > 3 and not inside a script
   - If decorative (svg, img, span with no text): strip
   - If container (div, section): keep if has interactive children or meaningful text
3. Collapse deep nesting: if a container only has one meaningful child, flatten one level
4. Output pruned tree with all annotations (kaboom_id, visibility, bounding_box, etc.)

#### Size reduction targets:
- Typical unpruned DOM: 500+ lines of HTML
- Pruned interactive DOM: 50-100 lines
- Tokens saved: ~3-5x reduction in context window usage

## Data Flows

### Initial Page Load

```
Page loads
  ↓
Extension content.js detects DOM ready
  ↓
observe({what: 'page'}) called by AI
  ↓
Background: Extract DOM tree
  ↓
Layout Calculator: Walk DOM, compute visibility/bounding boxes for each element
  ↓
Semantic ID Generator: Assign data-kaboom-id to interactive elements
  ↓
Component Mapper: Link elements back to React/Vue components (if available)
  ↓
Smart DOM Pruning: Filter to interactive + semantic content only
  ↓
Return enriched DOM to AI
```

### On User Action (AI clicks button)

```
AI: interact({what: 'execute_js', script: 'document.querySelector("[data-kaboom-id=\'btn-submit\']").click()'})
  ↓
Extension injects click event
  ↓
Page updates DOM (e.g., navigation, modal closes, form submits)
  ↓
Layout Calculator re-runs (visibility has changed)
  ↓
Return updated DOM state to context window
  ↓
AI observes: "Button was visible before, now the form is gone—success!"
```

## Implementation Strategy

### Phase 1: Layout Calculator (Week 1-2)
- Implement `computeElementVisibility()` for all interactive elements
- Implement hit-test algorithm to detect occlusion
- Test against real sites (Gmail, Google Docs, GitHub)
- Measure performance: ensure <50ms for 1000-element page

### Phase 2: Semantic Identifier Generator (Week 1-2, parallel)
- Implement `generateKaboomId()` with collision detection
- Inject into DOM via `setAttribute()` (non-destructive)
- Store mappings in WeakMap
- Test stability: reload page, verify same IDs

### Phase 3: Component Mapper (Week 2-3)
- Detect React/Vue DevTools hooks
- Walk Fiber tree (React) or component instances (Vue)
- Extract component name + file path
- Fall back to semantic selectors for non-framework code
- Test against React/Vue example apps

### Phase 4: Smart DOM Pruning (Week 2-3)
- Implement tree filter to remove non-interactive elements
- Measure context window impact
- Iterate on filter rules until 3-5x reduction achieved

### Phase 5: Integration & Testing (Week 3)
- Add `visual_semantic` toggle to `observe({what: 'page'})` response
- Include in regular DOM snapshots
- Run UAT against real debugging scenarios

## Edge Cases & Assumptions

### Edge Cases

| Case | Description | Handling |
|------|-------------|----------|
| **Shadows DOM** | Elements hidden in Shadow DOM | Use `composedPath()` to walk through shadow boundaries |
| **iframes** | Elements inside `<iframe>` | Kaboom can't access cross-origin iframes; skip them. Same-origin iframes: recursive walk |
| **Dynamic content** | Elements added after initial page load | Layout Calculator re-runs on every `observe()` call; captures latest state |
| **Fixed/Sticky overlays** | Modals, toasts, notifications that float over content | Hit-test algorithm detects these as "covered_by" correctly |
| **Virtualized lists** | React Window, windowed tables (elements only rendered when in viewport) | Only see elements currently in DOM; off-screen entries are genuinely absent |
| **CSS-in-JS** | Styled-components, Emotion (no visible class names) | Fallback to semantic content (text, aria-label, role) instead |
| **Canvas/WebGL elements** | Not part of DOM tree | Skip (not clickable via normal interact) |

### Assumptions

- **Browser APIs available** — `getComputedStyle()`, `getBoundingClientRect()`, `elementFromPoint()` work on target pages
- **DOM is stable during walk** — No mutations while Layout Calculator is running (should be <50ms, reasonable assumption)
- **React/Vue hooks are global** — Modern versions make hooks available at `window.__REACT_DEVTOOLS_GLOBAL_HOOK__` or equivalent
- **Users expect page-like visibility** — "Visible" means "clickable" from user perspective (matches browser behavior)
- **Semantic IDs are non-conflicting** — Hash collisions are rare enough (<1% on typical pages); counter fallback handles them

## Risks & Mitigations

| Risk | Description | Mitigation |
|------|-------------|-----------|
| **Performance degradation** | Layout Calculator + hit-tests could be slow on complex pages (10k+ DOM nodes) | Implement lazy evaluation: only compute visibility for visible subtrees. Cache results. Timeout at 50ms and return partial results. |
| **Memory leaks** | Holding references to DOM nodes prevents garbage collection | Use WeakMap for mappings. Automatically clear caches when DOM is pruned. |
| **Privacy leak** | Component props or sensitive data exposed in semantic IDs or mappings | Never include prop values in semantic IDs. Component names are public (visible in source). Scrub URLs/user data from element attributes. |
| **Framework-specific failures** | React/Vue DevTools hooks unavailable on older versions or custom builds | Fallback to semantic selectors. Test on React 16.8+, Vue 2.7+. Document limitations. |
| **Selector brittleness** | Semantic IDs based on text content break if content changes (e.g., "Log Out" → "Logout") | Document that semantic IDs are stable for structure, not content. Recommend aria-labels for stable identifiers. |

## Dependencies

### Existing Features Required
- **DOM observation** — Already exists via `observe({what: 'page'})`
- **Content script injection** — Already exists to manipulate DOM
- **Message passing** — Already exists to send data back to AI

### Browser APIs
- `getComputedStyle()` — Standard in all browsers
- `getBoundingClientRect()` — Standard in all browsers
- `elementFromPoint()` — Standard in all browsers
- `composedPath()` — For Shadow DOM support (modern browsers)

### Framework Integration
- **React DevTools Protocol** — Hook into `__REACT_DEVTOOLS_GLOBAL_HOOK__` (React 16+)
- **Vue DevTools Protocol** — Hook into Vue 3's `__VUE_DEVTOOLS_GLOBAL_HOOK__` (Vue 2.7+, Vue 3+)

### No External Dependencies
- Do not add npm packages (matches Kaboom's zero-deps philosophy)
- Use only browser APIs and stdlib

## Performance Considerations

### Target Benchmarks
- **Layout calculation:** <50ms for 1000-element page
- **Semantic ID generation:** <20ms for 500 interactive elements
- **Component mapping:** <30ms (React hook traversal)
- **DOM pruning:** <30ms filtering pass
- **Total:** <150ms per observe() call (5 calls/sec max = 0.75s/sec CPU overhead)

### Optimization Strategies
- **Lazy evaluation** — Only compute visibility for visible viewport (off-screen elements fast-path)
- **Caching** — Cache layout info between calls; invalidate on DOM mutations
- **Batching** — Process 50 elements per frame (requestAnimationFrame) to avoid blocking main thread
- **Early exit** — Stop hit-testing if element is clearly hidden (display: none → skip hit-test)

### Memory Profile
- **Semantic ID WeakMap** — Auto-garbage-collected; no memory leak
- **Layout cache** — Bounded by DOM size (typically <100KB for typical pages)
- **Component mappings** — <10KB per page (stored as strings, not references)

## Security Considerations

### Data Exposure
- **Semantic IDs** — Safe (contain only element type + generic hash, no user data)
- **Component names** — Safe (visible in React DevTools, source code inspection)
- **Component props** — Exclude values (only names, for audit purposes)
- **Bounding boxes** — Safe (layout info, not sensitive)

### Injection Risks
- **HTML injection via data-kaboom-id** — Mitigated by using `setAttribute()` (not `innerHTML`)
- **XSS via component mapping** — Component names are read-only from DevTools hooks (no user input)

### Privacy Impact
- **No new PII exposure** — Visual Bridge doesn't access sensitive attributes beyond what observe() already does
- **Page structure is visible** — Component mapping reveals app structure (minor, already visible in source)

### Mitigation Checklist
- [ ] Never execute user content as code
- [ ] Always use `setAttribute()` over `innerHTML` when injecting IDs
- [ ] Filter component names through allowlist (only alphanumeric + underscore)
- [ ] Test against XSS payloads in element text and attributes
