---
feature: Visual-Semantic Bridge
status: proposed
tool: observe
mode: dom
version: 0.7.12
doc_type: product-spec
feature_id: feature-visual-semantic-bridge
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Visual-Semantic Bridge

## Problem Statement

AI agents are "code-sighted" but "pixel-blind." They see the HTML code for a button, but don't know:
- If the button is actually visible (opacity, display, visibility states)
- If it's covered by a modal, popup, or overlay (z-index and hit-test results)
- If it's off-screen or scrolled out of view (bounding box calculations)
- Which actual DOM node to click when multiple selectors match (ambiguous element problem)

This leads to two critical failures:

1. **Ghost Click Problem** — AI tries to click elements that exist in HTML but are invisible (off-screen, hidden, covered). Tests timeout or fail mysteriously.
2. **Hallucinated Selector Problem** — AI generates selectors that match multiple elements (e.g., `.bg-blue-500` appears 10 times) and clicks the wrong one.

Without this information, AI agents waste time debugging interactions that should "just work" and engineers lose trust in autonomous debugging.

## Solution

The **Visual-Semantic Bridge** extends Kaboom's DOM observation to include computed visual and semantic information for every interactive element. This gives AI agents human-like vision:

- **Computed Layout Maps** — Bounding boxes, visibility states (visible/hidden/covered), z-index layers
- **Self-Healing Selectors** — Auto-generate unique, robust identifiers (test-IDs) for every interactive element
- **Component Mapping** — Link DOM nodes back to source code locations (which file, which component)
- **Accessibility-Aware Pruning** — Surface only semantically meaningful elements; strip decorative noise

Result: AI agents see the page as humans see it—not as a flat HTML dump, but as a structured, layered, interactive interface.

## Requirements

### Core Requirements

- **Computed Visibility** — Report for each interactive element:
  - `is_visible: true|false` (passes `getComputedStyle` checks)
  - `covered_by: [selector, element_name, z-index]` (if covered by another element via hit-test)
  - `bounding_box: {top, left, width, height}` (in viewport coordinates)
  - `off_screen: true|false` (outside viewport bounds)

- **Semantic Identifiers** — Auto-generate unique, stable IDs:
  - For every clickable, focusable, or form-interactive element
  - Format: `data-kaboom-id="type-hash"` (e.g., `data-kaboom-id="btn-a7x2"`)
  - Survive minor UI refactors (not brittle like `.nth-child(7)`)
  - Injected at runtime; include in context window

- **Component Mapping** — Map DOM nodes back to source when available:
  - React: Component name + props (e.g., `<UserCard name="Alice" />`)
  - Vue: Component name + props
  - Vanilla JS: nearest `id`, class, role, or semantic label

- **Smart DOM Pruning** — Remove non-interactive noise:
  - Strip hidden SVG paths, tracking pixels, invisible spacers
  - Keep only: buttons, inputs, links, labels, text nodes with semantic meaning
  - Compress Accessibility Tree for inclusion in <25% of context window

### Non-Functional Requirements

- **Performance** — Semantic Bridge calculation must complete in <50ms per page state
- **Memory** — No unbounded caches; use weak references for DOM node mappings
- **Accuracy** — Hit-test algorithm must match browser's actual click behavior
- **Stability** — IDs must be deterministic across page reloads and minor DOM mutations

## Out of Scope

- Generating fixes or suggestions (that's the AI's job)
- Handling dynamic component loading that happens after page load
- Supporting legacy frameworks without DevTools hooks (React < 16.8, Vue < 2.7)
- Pixel-perfect visual regression detection (screenshots handle that)
- CSS animation/transition prediction

## Success Criteria

- AI agents can navigate and interact with pages without "ghost clicks" or ambiguous element errors
- Test failure rate drops by >50% due to selector brittleness
- Context window usage for DOM representation stays <25% of model limit
- Engineers report increased confidence in autonomous debugging ("the AI actually sees what I see")
- Self-healing tests can run multiple times without manual selector adjustments

### Metrics

- **Selector stability:** % of generated test selectors that survive UI refactoring without adjustment
- **Click accuracy:** % of clicks that hit the intended element (vs. ambiguous/wrong element)
- **Visibility correctness:** % of visibility predictions that match actual DOM computed styles
- **Context efficiency:** Tokens used for DOM representation before/after (should improve by 3-5x)

## User Workflow

### For AI Agents

1. Agent observes the page via `observe({what: 'page'})`
2. Kaboom returns DOM with augmented metadata:
   ```
   {
     "elements": [
       {
         "tag": "button",
         "text": "Save",
         "id": "btn-save",
         "class": "px-4 py-2 bg-blue-500",
         "kaboom_id": "btn-a7x2",        // NEW: Unique, stable ID
         "visible": true,                   // NEW: Visibility state
         "bounding_box": {...},            // NEW: Layout info
         "covered_by": null,               // NEW: Occlusion info
         "component": "SaveButton.tsx:42"  // NEW: Source location
       }
     ]
   }
   ```
3. Agent uses `data-kaboom-id="btn-a7x2"` when clicking or testing
4. Agent sees that off-screen elements are marked `off_screen: true` and doesn't try to click them
5. Agent uses visibility info to diagnose "button should be visible but user reports it's not" issues

### For Engineers

1. Enable Visual-Semantic Bridge in extension settings (checkbox: "Show layout maps + semantic IDs")
2. Open Kaboom UI in DevTools sidebar
3. See DOM rendered with bounding boxes, z-index layers, and unique IDs overlaid on the page
4. Hover over elements to see which component they belong to + computed styles
5. Use this as a debugging aid when AI or tests fail ("Wait, the button is actually covered by the modal!")

## Examples

### Example 1: Fixing the Ghost Click Problem

**Scenario:** Mobile menu with "Close" button that's initially off-screen.

#### Without Visual-Semantic Bridge:
```
AI observes DOM: <button class="close-btn">Close</button>
AI thinks: "I see the Close button, I'll click it"
AI tries: document.querySelector('.close-btn').click()
Result: Click does nothing (element is off-screen at left: -100%)
AI fails after 5 retries
```

#### With Visual-Semantic Bridge:
```
AI observes DOM:
{
  "tag": "button",
  "text": "Close",
  "kaboom_id": "btn-close-menu",
  "visible": false,
  "off_screen": true,
  "bounding_box": {"left": -100, "top": 20}
}
AI thinks: "The Close button is off-screen. I must open the menu first."
AI steps:
  1. Click the hamburger menu (which is on-screen)
  2. Wait for animation
  3. Now the Close button's bounding_box is {"left": 10, "top": 20}
  4. Click it
Result: Success
```

### Example 2: Fixing the Hallucinated Selector Problem

**Scenario:** Page with 10 blue buttons using Tailwind.

#### Without Visual-Semantic Bridge:
```
HTML: <button class="px-4 py-2 bg-blue-500">Save</button>
      <button class="px-4 py-2 bg-blue-500">Submit</button>
      ... (8 more blue buttons)

AI tries to click the Submit button:
AI thinks: "I'll use a unique selector. How about .bg-blue-500?"
AI: document.querySelector('.bg-blue-500').click()
Result: Clicks the Save button instead (first match)
Test fails: "Oops, I saved something instead of submitting"
```

#### With Visual-Semantic Bridge:
```
AI observes DOM:
[
  {
    "tag": "button",
    "text": "Save",
    "class": "px-4 py-2 bg-blue-500",
    "kaboom_id": "btn-save"     // Unique, semantic ID
  },
  {
    "tag": "button",
    "text": "Submit",
    "class": "px-4 py-2 bg-blue-500",
    "kaboom_id": "btn-submit"   // Different ID, not just .bg-blue-500
  },
  ... (8 more with unique IDs)
]

AI thinks: "Perfect, each button has a unique ID. I'll click data-kaboom-id='btn-submit'"
AI: document.querySelector('[data-kaboom-id="btn-submit"]').click()
Result: Clicks the Submit button correctly
```

### Example 3: Debugging Covered Elements

**Scenario:** User reports "the Save button doesn't work" but it's actually hidden by a modal overlay.

#### With Visual-Semantic Bridge:
```
Engineer opens Kaboom DevTools:
- Hovers over Save button
- Sees: "visible: false, covered_by: [ModalOverlay, z-index: 9999]"
- Root cause immediately clear: modal is blocking the button
- No need to ask "is it visible?" — Kaboom just showed them
```

## Notes

### Related specs:
- DOM Fingerprinting (v6.1) — Extends selector stability for self-healing tests
- Smart DOM Pruning (v6.1) — Reduces context window by removing decorative noise
- Deep Framework Intelligence (v6.1) — Shows React/Vue component tree alongside DOM

### Dependencies:
- Requires browser APIs: `getComputedStyle()`, `getBoundingClientRect()`, `elementFromPoint()`
- React DevTools hooks (for React component mapping)
- Vue DevTools hooks (for Vue component mapping)

### Design decision rationale:
- Using `data-kaboom-id` instead of modifying real IDs: doesn't pollute user's HTML, survives page reloads
- Hit-test algorithm over z-index inspection alone: matches actual browser behavior (what matters to users)
- Injecting semantic IDs at runtime: avoids build-time dependencies, works with any framework
