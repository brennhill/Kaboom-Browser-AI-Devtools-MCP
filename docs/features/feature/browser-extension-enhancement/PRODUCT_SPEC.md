---
status: draft
priority: tier-1
phase: v6.0-sprint-1
effort: 2 weeks
relates-to: [ring-buffer, normalized-event-schema]
blocks: [query-service, checkpoint-validation]
last-updated: 2026-01-31
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Browser Extension Enhancement — Product Specification

**Goal:** Extend v5.3 browser telemetry capture with performance timing, DOM snapshots, and accessibility events needed for AI-native testing paradigm.

---

## Problem Statement

Current v5.3 capture provides:
- ✅ Console logs
- ✅ Network errors (4xx, 5xx)
- ✅ Exceptions
- ✅ WebSocket events

Missing for AI-native testing:
- ❌ Performance metrics per action (how fast was that click?)
- ❌ DOM state snapshots (what changed after the click?)
- ❌ Accessibility context (can the AI understand semantic meaning?)
- ❌ Action metadata (what element was clicked, with what selector?)

**Impact:** Without this, AI can see "form submitted" but not understand *why* it failed, or whether the UI visually changed.

---

## User Stories

### Story 1: Performance-Aware Debugging
**As:** AI testing agent
**I want:** Timing data for every user action
**So that:** I can detect performance regressions and slow paths

```
Example:
User clicks "Add to Cart" button
  ├─ Interaction latency: 2ms (button click → JS handler)
  ├─ DOM update latency: 15ms (cart count changes)
  ├─ Network latency: 320ms (API response)
  ├─ Total action latency: 337ms
  └─ Status: "Action complete, cart updated"

AI inference: "Good, added to cart in 337ms (acceptable range)"
```

### Story 2: DOM Snapshot for Regression Detection
**As:** AI regression detector
**I want:** Snapshots of DOM before/after each action
**So that:** I can detect unexpected UI changes

```
Example:
Before click:
  <button class="btn-primary">Add to Cart</button>
  <div class="cart-count">5</div>

After click (500ms):
  <button class="btn-primary btn-disabled">Add to Cart</button>
  <div class="cart-count">6</div>

AI inference: "Cart count increased (expected), button disabled (expected for UX feedback)"
```

### Story 3: Semantic UI Understanding
**As:** AI specification validator
**I want:** Accessibility labels and semantic meaning of UI elements
**So that:** I can reason about UI intent, not just DOM structure

```
Example:
Button with:
  - aria-label: "Add selected items to shopping cart"
  - aria-disabled: false
  - role: button
  - visible text: "Add"

AI inference: "This is a cart button, currently enabled, can click it"
(vs. parsing raw HTML and guessing)
```

### Story 4: Action Context Tracking
**As:** AI test generator
**I want:** Metadata about each user action (selector, coordinates, element text)
**So that:** I can generate maintainable Playwright tests

```
Example action:
{
  "type": "click",
  "selector": "button.add-to-cart",
  "coordinates": [152, 48],
  "element_text": "Add to Cart",
  "aria_label": "Add selected items to shopping cart",
  "timestamp": 1704067200000,
  "duration_ms": 337,
  "dom_changes": ["cart-count: 5 → 6"],
  "network_requests": [
    {
      "method": "POST",
      "url": "/api/cart/add",
      "status": 200,
      "latency_ms": 320
    }
  ]
}
```

---

## Features to Implement

### Feature 1: Action Performance Timing
**What:** Measure and record timing for every user action

#### How:
- ✅ Start: User initiates action (click, type, navigate)
- Measure: JavaScript handler execution
- Measure: DOM mutation observer (when first change detected)
- Measure: Network requests (if action triggers API calls)
- ✅ End: All network requests complete + DOM mutations settle (1s timeout)

#### Data captured:
```typescript
interface ActionTiming {
  start_time: number;           // ms timestamp
  interaction_latency_ms: number; // JS handler → first JS execution
  dom_latency_ms: number;        // JS handler → DOM mutation
  network_latency_ms: number;    // Total API response time
  total_latency_ms: number;      // Start → all complete
  network_requests: {
    method: string;
    url: string;
    status: number;
    latency_ms: number;
  }[];
}
```

#### Success Criteria:
- [ ] Timing accurate within ±5ms for interaction latency
- [ ] Timing accurate within ±10ms for DOM latency
- [ ] Timing accurate within ±20ms for network latency
- [ ] No blocking on main thread (use requestIdleCallback for measurement)
- [ ] Test: Click button, measure timing, verify against known delay

### Feature 2: DOM State Snapshots
**What:** Capture DOM structure before/after significant actions

#### How:
- Snapshot before action: Full innerHTML of affected subtree
- Snapshot after action: Full innerHTML after mutations settle
- Diff: Compute what changed (element count, attribute changes, text changes)

#### Data captured:
```typescript
interface DOMSnapshot {
  timestamp: number;
  action_id: string;
  before: {
    html: string;  // up to 10KB
    element_count: number;
    checksum: string;
  };
  after: {
    html: string;
    element_count: number;
    checksum: string;
  };
  changes: {
    type: "added" | "removed" | "modified" | "attribute_changed" | "text_changed";
    selector?: string;
    before_value?: string;
    after_value?: string;
  }[];
}
```

#### Success Criteria:
- [ ] Snapshot captures full HTML of action target + children
- [ ] Diff algorithm detects all meaningful changes
- [ ] Snapshots compressed to < 5KB each (remove whitespace, comments)
- [ ] Test: Change button text, verify snapshot detects change
- [ ] Test: Add to DOM, verify snapshot detects new element

### Feature 3: Accessibility Event Capture
**What:** Record accessibility labels, roles, and state changes

#### How:
- On every DOM mutation: Extract aria-* attributes, role, label
- On every action: Record semantic meaning (button clicked, input focused, etc.)
- Track accessibility violations: WCAG 2.1 AA basic checks

#### Data captured:
```typescript
interface AccessibilityEvent {
  timestamp: number;
  action_id: string;
  element_selector: string;
  aria_label?: string;
  aria_role?: string;
  aria_disabled?: boolean;
  aria_hidden?: boolean;
  aria_live?: string;
  semantic_type: "button" | "input" | "link" | "heading" | "region" | string;
  visible_text: string;
  violations: {
    wcag_criterion: string;  // e.g., "1.4.3 Contrast (Minimum)"
    severity: "critical" | "major" | "minor";
    message: string;
  }[];
}
```

#### Success Criteria:
- [ ] aria-label captured for all elements with labels
- [ ] role attribute extracted and classified
- [ ] Disabled state tracked
- [ ] Basic WCAG checks working (color contrast, text alternatives)
- [ ] Test: Add aria-label to element, verify captured
- [ ] Test: Hidden element, verify aria-hidden recorded

### Feature 4: Smart Selector Generation
**What:** Generate maintainable selectors for captured actions

#### How:
- Prioritize: data-testid > aria-label > visible text > class > nth-child
- Fallback: Generate semantic selector (e.g., "button with text 'Add to Cart'")
- Validate: Test selector against current DOM to ensure it matches

#### Data captured:
```typescript
interface SmartSelector {
  primary: string;      // "button[data-testid='add-to-cart']"
  fallback: string;     // "button:has-text('Add to Cart')"
  semantic: string;     // "submit button for product form"
  confidence: number;   // 0-100, likelihood selector still works after changes
}
```

#### Success Criteria:
- [ ] Selector syntax valid for Playwright/Puppeteer
- [ ] Selector works against live DOM
- [ ] Selector stable across minor CSS changes
- [ ] Test: Add data-testid, verify used as primary selector
- [ ] Test: Remove data-testid, verify fallback used instead

---

## Acceptance Criteria

### Functional
- [ ] Performance timing captured for 100% of tracked actions
- [ ] DOM snapshots compressed to <5KB and diff accurate
- [ ] Accessibility data captured for 100% of interactive elements
- [ ] Smart selectors generated and validated for 100% of actions
- [ ] Extension still functional after enhancements (no broken v5.3 features)

### Performance
- [ ] Extension overhead: <0.1ms per event (measured with performance.now())
- [ ] Memory footprint: <2MB additional RAM for timing/snapshot tracking
- [ ] DOM snapshot computation: <50ms per action (not blocking main thread)
- [ ] Selector generation: <10ms per action

### Browser Compatibility
- [ ] Chrome 120+
- [ ] Firefox 121+
- [ ] Safari 17+

### Data Quality
- [ ] Zero events lost during normal operation
- [ ] Timing data accurate within ±20ms
- [ ] Snapshots capture 95%+ of DOM mutations

---

## Out of Scope (v6.0)

- [ ] Video recording of actions (v6.1+)
- [ ] Pixel-level visual regression detection (v6.1+)
- [ ] Scroll position tracking (v6.1+)
- [ ] Form data redaction (v7.0+)
- [ ] PII detection in DOM (v7.0+)

---

## Success Metrics

### v6.0 Sprint 1 Success
- Demo 1 (Spec-driven validation): AI validates 8-char password requirement in <3 minutes
  - Must see: Button click → form submission → validation → error message
  - Must measure: Action latency (form submit should be <500ms)
  - Must understand: Accessibility context (this is a form, not random text)

- Demo 2 (Checkpoint validation): AI compares checkout before/after feature
  - Must see: DOM structure before → after adding reviews feature
  - Must detect: "Cart count displays correctly" (regression detection)
  - Must measure: No unexpected performance degradation

---

## Related Documents

- **Tech Spec:** [browser-extension-enhancement/TECH_SPEC.md](TECH_SPEC.md)
- **QA Plan:** [browser-extension-enhancement/QA_PLAN.md](QA_PLAN.md)
- **Architecture:** [360-observability-architecture.md](../../../core/architecture/360-observability-architecture.md#ingestion-layer)
- **Sequencing:** [implementation-sequencing.md](../../../core/architecture/implementation-sequencing.md#sprint-a1-browser-extension--buffer-layer-week-1)

---

**Status:** Ready for spec review and tech spec development
**Owner:** (Assign 1 engineer)
**Duration:** 1 week (A1 timing)
