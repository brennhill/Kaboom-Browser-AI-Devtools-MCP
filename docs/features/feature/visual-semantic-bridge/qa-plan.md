---
feature: Visual-Semantic Bridge
doc_type: qa-plan
feature_id: feature-visual-semantic-bridge
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Visual-Semantic Bridge

> How to test computed layout maps, semantic identifiers, and component mapping.

## Testing Strategy

### Code Testing (Automated)

#### Unit Tests: Layout Calculator

- [ ] **Visibility detection** — Element with display: none is marked `visible: false`
- [ ] **Visibility detection** — Element with visibility: hidden is marked `visible: false`
- [ ] **Visibility detection** — Element with opacity: 0 is marked `visible: false`
- [ ] **Opacity edge case** — Element with opacity: 0.01 is marked `visible: true` (barely visible)
- [ ] **Bounding box calculation** — getBoundingClientRect() values correctly converted
- [ ] **Off-screen detection** — Element with left: -1000px is marked `off_screen: true`
- [ ] **Off-screen detection** — Element with top > viewport.height is marked `off_screen: true`
- [ ] **Hit-test occlusion** — Modal overlay detected as "covered_by"
- [ ] **Hit-test occlusion** — Z-index correctly reported for covering element
- [ ] **SVG element handling** — SVG elements use getBBox() instead of getBoundingClientRect()
- [ ] **Nested visibility** — Child of hidden parent is marked `visible: false` even if child.style.display !== 'none'
- [ ] **Fixed overlay** — Fixed-position overlay correctly detected as covering element beneath it

#### Unit Tests: Semantic Identifier Generator

- [ ] **ID generation** — Button with text "Save" gets semantic ID starting with "btn-"
- [ ] **ID generation** — Input with aria-label="Email" gets semantic ID reflecting label
- [ ] **ID generation** — Link with text "Home" gets semantic ID starting with "link-"
- [ ] **Determinism** — Same page state produces identical IDs on reload
- [ ] **Uniqueness** — Two buttons with same text get different IDs (counter appended: -1, -2)
- [ ] **Hash collision** — Multiple identical elements don't crash ID generator
- [ ] **DOM insertion** — ID attribute inserted without affecting element functionality
- [ ] **DOM cleanup** — WeakMap entry removed when element is deleted
- [ ] **Stability** — ID survives minor CSS changes (color, padding, opacity)
- [ ] **Stability** — ID survives minor text changes (inner HTML mutation)

#### Unit Tests: Component Mapper

- [ ] **React detection** — React DevTools hook detected when available
- [ ] **React mapping** — Fiber tree walked to find component name
- [ ] **React props** — Props extracted (names only, not values)
- [ ] **Vue detection** — Vue DevTools hook detected when available
- [ ] **Vue mapping** — Component instance found via __VUE_COMPONENT__
- [ ] **Vue props** — Props extracted (names only, not values)
- [ ] **Fallback** — Vanilla JS element mapped to nearest id/class/role
- [ ] **File path** — Component file location included in mapping (e.g., "Button.tsx:42")
- [ ] **Privacy** — Prop values never included in output

#### Unit Tests: Smart DOM Pruning

- [ ] **SVG removal** — `<svg>` elements stripped from output
- [ ] **Tracking pixel removal** — Hidden 1x1 images removed
- [ ] **Empty spacer removal** — Empty `<div>` and `<span>` removed
- [ ] **Script/style removal** — `<script>` and `<style>` tags removed
- [ ] **Interactive preservation** — Buttons and inputs always preserved
- [ ] **Text preservation** — Headings and labeled text preserved
- [ ] **Nesting collapse** — Container with single child flattened
- [ ] **Size reduction** — Pruned DOM is 3-5x smaller than raw DOM

#### Integration Tests: End-to-End

- [ ] **Full page load** — observe({what: 'page'}) returns enriched DOM with all metadata
- [ ] **Metadata consistency** — Semantic ID appears on element both in DOM and in returned metadata
- [ ] **After mutation** — After `interact({action: 'execute_js', script: 'element.click()'})`, re-observe shows updated visibility
- [ ] **React app** — Real React app (Create React App, Next.js) component mapping works
- [ ] **Vue app** — Real Vue app component mapping works
- [ ] **Mixed framework** — Page with both React and Vue components maps correctly
- [ ] **Shadow DOM** — Elements inside Shadow DOM included (same-origin iframes)
- [ ] **Performance** — Full pipeline completes in <150ms on typical page

#### Edge Case Tests

- [ ] **Inaccessible iframe** — Cross-origin iframe skipped gracefully (not crash)
- [ ] **Removed element** — If element is removed mid-calculation, handle gracefully
- [ ] **Circular layout** — Avoid infinite loops in visibility calculation
- [ ] **Large page** — 5000+ element page still completes in <200ms
- [ ] **Virtual scrolling** — React Window virtualized list: only renders visible items

### Security/Compliance Testing

- [ ] **XSS prevention** — Semantic ID insertion uses setAttribute (not innerHTML)
- [ ] **XSS prevention** — Test with malicious content in element attributes (e.g., `<button onclick="alert(1)">`)
- [ ] **PII prevention** — No user data (emails, phone numbers) in semantic IDs or component mappings
- [ ] **Data scrubbing** — URLs containing sensitive paths are redacted before returning
- [ ] **Framework hook safety** — Component names filtered through allowlist (alphanumeric + underscore only)

---

## Human UAT Walkthrough

### Scenario 1: Basic Visibility Detection

#### Setup:
1. Load a test page with buttons in different visibility states
2. Page HTML:
   ```html
   <button id="visible-btn">Visible Button</button>
   <button id="hidden-css" style="display: none;">Hidden Button</button>
   <button id="covered-btn">Covered Button</button>
   <div id="modal-overlay" style="position: fixed; top: 0; left: 0; width: 100%; height: 100%; z-index: 100;"></div>
   ```

#### Steps:
1. [ ] Open Kaboom DevTools
2. [ ] Call `observe({what: 'page'})`
3. [ ] Review returned DOM metadata

#### Expected Result:
- Visible button marked with `visible: true`, `off_screen: false`, `covered_by: null`
- Hidden button marked with `visible: false`
- Covered button marked with `visible: false, covered_by: "modal-overlay"`

#### Verification:
- Each button has a unique `kaboom_id` (e.g., `btn-visible`, `btn-hidden-css`, `btn-covered`)
- Bounding boxes are present and non-zero for visible elements
- Bounding box is present but off-screen for hidden elements

### Scenario 2: Ghost Click Problem Solved

#### Setup:
1. Load a page with a mobile menu
2. Page HTML:
   ```html
   <button id="menu-toggle">☰ Menu</button>
   <nav id="mobile-menu" style="position: fixed; left: -100%; transition: left 0.3s;">
     <button id="menu-close">✕ Close</button>
   </nav>
   ```

#### Steps:
1. [ ] AI agent starts with `observe({what: 'page'})`
2. [ ] Menu is closed (off-screen)
3. [ ] Agent observes: Close button is `off_screen: true`
4. [ ] Agent does NOT try to click it
5. [ ] Agent clicks the Menu toggle button
6. [ ] Menu slides in (CSS animation)
7. [ ] Agent calls `observe({what: 'page'})` again
8. [ ] Close button now shows `off_screen: false`
9. [ ] Agent can now click Close button safely

#### Expected Result:
- No timeout or failed click on off-screen element
- Menu toggle works correctly
- Menu can be closed after opening

#### Verification:
- Bounding box changes between calls (before: left -100, after: left 0)
- Off-screen flag changes correctly
- No 5-second retry loops

### Scenario 3: Hallucinated Selector Problem Solved

#### Setup:
1. Load a page with many blue buttons
2. Page HTML:
   ```html
   <button class="btn-primary bg-blue-500">Save</button>
   <button class="btn-primary bg-blue-500">Submit</button>
   <button class="btn-primary bg-blue-500">Publish</button>
   ```

#### Steps:
1. [ ] AI observes DOM
2. [ ] Reviews returned metadata
3. [ ] AI agent is instructed to click "Submit" button
4. [ ] Agent uses `data-kaboom-id="btn-submit"` instead of `.bg-blue-500`
5. [ ] Agent executes: `document.querySelector('[data-kaboom-id="btn-submit"]').click()`

#### Expected Result:
- Each button has a unique semantic ID (different from others)
- Agent clicks the correct button (Submit, not Save)
- Form submits successfully

#### Verification:
- Semantic IDs are stable across page reloads
- No ambiguous clicks
- Test passes on first try

### Scenario 4: Framework Component Mapping

#### Setup:
1. Load a React app (e.g., Next.js example)
2. App contains: UserCard component, SaveButton component, Modal component

#### Steps:
1. [ ] Enable DevTools in React app
2. [ ] Open Kaboom and observe page
3. [ ] Review component metadata for each element

#### Expected Result:
- Button element is mapped to: `component: "SaveButton", file: "components/SaveButton.tsx:42"`
- Card element is mapped to: `component: "UserCard", file: "components/UserCard.tsx:10"`
- Modal element is mapped to: `component: "Modal", file: "components/Modal.tsx:5"`

#### Verification:
- Component names are readable and useful for AI
- File paths are accurate
- Prop names are listed (values are redacted)

### Scenario 5: Smart DOM Pruning Reduces Context

#### Setup:
1. Load a complex web app (Gmail, Google Docs, etc.)
2. Extract full DOM and pruned DOM

#### Steps:
1. [ ] Measure size of raw `outerHTML` for the page
2. [ ] Call `observe({what: 'page'})` with pruning enabled
3. [ ] Measure returned DOM string
4. [ ] Calculate tokens for each (rough estimate: 1 token ≈ 4 chars)

#### Expected Result:
- Raw DOM: 50KB+ (12,000+ tokens)
- Pruned DOM: 10-15KB (2,500-3,750 tokens)
- Reduction: 3-5x

#### Verification:
- No interactive elements are stripped (buttons, inputs, links still present)
- Semantic text (headings, labels) still present
- No visible content lost

---

## Regression Testing

### Existing features to verify don't break:

- [ ] `observe({what: 'page'})` still works without enrichment (backward compat)
- [ ] `observe({what: 'dom'})` (if distinct from 'page') still works
- [ ] `interact({action: 'execute_js'})` still works with injected IDs
- [ ] `observe({what: 'network'})` unaffected by semantic enhancements
- [ ] `observe({what: 'console'})` unaffected
- [ ] Tab switching still works with semantic metadata
- [ ] Session diffing still works with enriched DOM

### Performance regression:

- [ ] observe() latency increase is <50ms (p95)
- [ ] Extension doesn't hang after repeated observe() calls
- [ ] Memory usage doesn't grow unbounded (WeakMap cleanup working)

---

## Performance/Load Testing

- [ ] **Scale test** — 5000-element page completes in <200ms
- [ ] **Repeated calls** — 100 sequential observe() calls complete without performance degradation
- [ ] **Large form** — Form with 500 inputs: all get semantic IDs, all visibility computed
- [ ] **Deep nesting** — Page with 50+ levels of nested divs: pruning completes without stack overflow
- [ ] **Long content** — 10,000 word page: pruning correctly preserves text while removing whitespace

---

## Sign-Off Criteria

- ✅ All unit tests passing
- ✅ All integration tests passing
- ✅ All UAT scenarios passing
- ✅ No regressions in existing features
- ✅ Performance benchmarks met (<150ms total, 3-5x context reduction)
- ✅ Security tests passing (XSS, PII prevention)
- ✅ Manual testing on real React and Vue apps successful
