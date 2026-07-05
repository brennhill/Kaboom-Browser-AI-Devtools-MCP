---
status: draft
priority: tier-1
phase: v6.0-sprint-1
relates-to: [PRODUCT_SPEC.md, TECH_SPEC.md]
last-updated: 2026-01-31
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Browser Extension Enhancement — QA Plan

**Goal:** Ensure performance timing, DOM snapshots, and accessibility capture are reliable and performant.

---

## Test Environment Setup

### Browser Versions
- Chrome 120+
- Firefox 121+
- Safari 17+ (if applicable)

### Test Site: ShopBroken
- URL: `http://localhost:3000`
- Has: Product page, checkout form, payment flow
- Allows: Automated clicking, form submission, network monitoring

### Tools
- Chrome DevTools (manual verification)
- Performance.now() for timing validation
- DOM snapshot comparison tools
- WCAG accessibility scanner

---

## Unit Tests

### 1. Action Timing

#### Test 1.1: Basic Click Timing
```
Given: Button with click handler that sets timeout 100ms
When: User clicks button
Then:
  - interaction_latency_ms ≤ 5ms (immediate JS execution)
  - total_latency_ms ≈ 100ms ± 10ms
```

#### Test 1.2: Network Request Timing
```
Given: Form submit that posts to /api/checkout (simulated 300ms latency)
When: User submits form
Then:
  - interaction_latency_ms ≤ 5ms
  - network_latency_ms ≈ 300ms ± 20ms
  - total_latency_ms ≈ 300ms ± 20ms
```

#### Test 1.3: Multiple Network Requests
```
Given: Action triggers 3 parallel API calls (100ms, 200ms, 300ms)
When: Action completes
Then:
  - network_latency_ms = max(100, 200, 300) ≈ 300ms
  - All requests tracked in metadata
```

#### Test 1.4: Timeout on Slow Action
```
Given: Action that never completes (infinite loop)
When: 1 second passes
Then:
  - Action recorded with timeout flag
  - Event still emitted with partial data
```

### 2. DOM Snapshots

#### Test 2.1: Before/After Snapshot
```
Given: Button that appends text to div
When: User clicks button
Then:
  - before.html contains original text
  - after.html contains appended text
  - diff shows "text_changed"
```

#### Test 2.2: HTML Compression
```
Given: Large HTML with whitespace and comments
When: Snapshot captured
Then:
  - Whitespace removed
  - Comments removed
  - Result < 5KB
```

#### Test 2.3: Element Count Tracking
```
Given: Action adds 5 elements to DOM
When: Snapshot captured
Then:
  - before.element_count = N
  - after.element_count = N + 5
  - diff shows "added" change
```

#### Test 2.4: Checksum Consistency
```
Given: Same HTML snapshot twice
When: Checksums computed
Then:
  - Both checksums match
  - Can use for quick comparison
```

### 3. Accessibility Context

#### Test 3.1: ARIA Label Capture
```
Given: Button with aria-label="Add to cart"
When: Element captured
Then:
  - accessibility.aria_label = "Add to cart"
```

#### Test 3.2: Role Detection
```
Given: Custom element with role="button"
When: Element captured
Then:
  - accessibility.element_role = "button"
```

#### Test 3.3: Violation Detection - Missing Alt Text
```
Given: Image without alt attribute
When: Element captured
Then:
  - violations contains WCAG 1.1.1
  - severity = "critical"
```

#### Test 3.4: Violation Detection - No Label
```
Given: Input field without label or aria-label
When: Element captured
Then:
  - violations contains WCAG 1.3.1
  - severity = "major"
```

#### Test 3.5: Contrast Ratio
```
Given: Text with low contrast (2:1)
When: Element captured
Then:
  - violations contains WCAG 1.4.3
  - severity = "major"
  - message includes "2.0:1 (need ≥4.5:1)"
```

### 4. Smart Selector Generation

#### Test 4.1: data-testid Priority
```
Given: Button with data-testid="add-to-cart"
When: Selector generated
Then:
  - primary = "[data-testid='add-to-cart']"
  - confidence_pct ≥ 90
```

#### Test 4.2: Fallback to Class
```
Given: Button without testid, with class="btn-primary"
When: Selector generated
Then:
  - primary = "button.btn-primary"
  - fallback includes text matching
  - confidence_pct ≥ 70
```

#### Test 4.3: Text-Based Fallback
```
Given: Link with text "Sign out"
When: Selector generated
Then:
  - fallback = "a:has-text('Sign out')"
  - confidence_pct ≥ 60
```

#### Test 4.4: Selector Validation
```
Given: Generated selector for specific element
When: Selector tested against live DOM
Then:
  - Selector matches exactly 1 element
  - Matched element is the original element
```

#### Test 4.5: Selector Stability
```
Given: Selector with high confidence (data-testid)
When: CSS classes change
Then:
  - Selector still matches same element
  - Low confidence selectors may fail (expected)
```

---

## Integration Tests

### 1. Full Action Cycle

#### Test: Click Button → Complete Action
```
Given: ShopBroken product page
When: User clicks "Add to Cart" button
Then:
  - Action recorded with id
  - Timing captured (<1 second total)
  - DOM snapshot before/after captured
  - Accessibility context captured
  - All linked to same action_id
  - Event emitted to MCP bridge
```

#### Test: Type → Submit → Network
```
Given: Login form with email + password fields
When: User types email, types password, clicks submit
Then:
  - 3 action events (type, type, click)
  - Submit triggers POST /api/login
  - Network latency captured
  - Response body (if error) captured
  - All correlated in timeline
```

### 2. Concurrent Actions

#### Test: Multiple Buttons Clicked Rapidly
```
Given: Toolbar with 5 buttons
When: User clicks each button in rapid succession (100ms apart)
Then:
  - 5 action events recorded
  - Each has correct timing (not overlapping)
  - No events lost
  - Correct order preserved
```

### 3. Performance Under Load

#### Test: 100 Actions in 1 Second
```
Given: Auto-generate 100 clicks in 1 second
When: Monitor extension overhead
Then:
  - No crashes
  - No lost events
  - Extension latency <0.1ms per event
  - Memory usage increase <10MB
```

### 4. Event Flow to MCP Bridge

#### Test: Events Reach Query API
```
Given: Action captured by extension
When: Query /buffers/timeline?limit=1
Then:
  - Event appears in response
  - timing data present
  - snapshot data present
  - accessibility data present
```

---

## Performance Tests

### Memory Footprint

#### Test: Memory Overhead
```
Tool: Chrome DevTools Memory Profiler
Given: Fresh extension load
When: Record 1000 actions
Then:
  - Additional memory <5MB
  - No memory leaks on repeated cycles
  - GC clears snapshots after events published
```

### Timing Accuracy

#### Test: Timing Within Budget
```
Tool: performance.now() on live DOM
Given: Known delay actions (100ms, 500ms, 1000ms)
When: Action timing measured
Then:
  - Measured within ±5% of actual
  - For 100ms: 95-105ms acceptable
  - For 500ms: 475-525ms acceptable
```

### DOM Snapshot Performance

#### Test: Snapshot Creation Speed
```
Given: Large DOM (10K elements)
When: Snapshot captured after action
Then:
  - Snapshot <50ms (non-blocking)
  - Compression <100ms
  - Results in <5KB
```

### Selector Generation Performance

#### Test: Selector Generation Speed
```
Given: Various element types
When: Selector generated
Then:
  - Generation <5ms per element
  - Can handle 100 elements in <500ms
```

---

## Browser Compatibility Tests

### Chrome
- [ ] v120: Performance timing works
- [ ] v120: DOM snapshots captured
- [ ] v120: Accessibility detection works
- [ ] v120: No console errors

### Firefox
- [ ] v121: All above
- [ ] (PerformanceObserver API present)
- [ ] (MutationObserver works)

### Safari
- [ ] v17: All above
- [ ] (Check for API differences)

---

## Regression Tests (Running Every Sprint)

### Test: v5.3 Features Still Work
```
Given: v5.3 console logging active
When: extension running with v6.0 enhancements
Then:
  - Console logs still captured
  - Network errors still captured
  - Exceptions still captured
  - No breaking changes
```

### Test: Performance Regression
```
Given: Same test action on v5.3 vs v6.0
When: Timing measured
Then:
  - v6.0 overhead <2x (tolerable)
  - Extension still responsive
  - Memory increase <10MB
```

---

## Accessibility Tests

### WCAG Compliance

#### Test: Extension UI Accessible
```
Tool: Axe accessibility scanner
Given: Extension popup open
When: Scan run
Then:
  - No critical violations
  - All buttons accessible
  - Proper contrast ratios
```

---

## End-to-End Tests (ShopBroken)

### Scenario 1: Product Purchase Flow
```
1. Navigate to product page
2. Click "Add to Cart"
3. View cart
4. Click "Checkout"
5. Fill form (email, address, card)
6. Click "Place Order"
7. See success message

Verify:
  - All 6 actions captured
  - Network requests for cart add, checkout submit
  - Form validation events (if errors)
  - Timeline is coherent and in order
  - AI can understand flow from events
```

### Scenario 2: Form Validation Error
```
1. Navigate to checkout
2. Leave email empty
3. Click "Submit"
4. See validation error

Verify:
  - Form submit event captured
  - Network request (POST) captured
  - Response with 400 error captured
  - Error message visible in event
  - Timeline shows: action → network → error
```

### Scenario 3: Login with Invalid Credentials
```
1. Navigate to login
2. Type incorrect password
3. Click "Login"
4. See "Invalid credentials" error

Verify:
  - All typing captured
  - Submit event captured
  - Network request and response captured
  - Error message visible
  - Timing shows slow network (if applicable)
```

---

## Test Automation

### Jest Tests
```typescript
// File: tests/browser-extension/performance-timing.test.ts

describe('Browser Extension - Performance Timing', () => {
  test('click timing within ±5ms', async () => {
    const startTime = performance.now();
    await page.click('button.test-button');
    const duration = performance.now() - startTime;

    // Browser extension should measure this too
    const extension_event = await getLatestEvent();
    expect(extension_event.metadata.action.timing_ms).toBeCloseTo(duration, 1);
  });

  test('network latency captured correctly', async () => {
    // Mock API with 300ms delay
    await page.route('/api/**', route => {
      setTimeout(() => route.continue(), 300);
    });

    const before = performance.now();
    await page.click('button.submit');
    const total = performance.now() - before;

    const event = await getLatestEvent();
    expect(event.metadata.action.timing_ms).toBeCloseTo(total, 2);
  });
});
```

### E2E Tests with Playwright
```typescript
// File: tests/e2e/checkout-flow.test.ts

test('full checkout flow captured', async () => {
  await page.goto('http://localhost:3000/products/1');

  // Click Add to Cart
  await page.click('button:has-text("Add to Cart")');
  let event = await getLatestEvent();
  expect(event.metadata.action.type).toBe('click');
  expect(event.metadata.action.dom_changes).toContain('cart-count');

  // Verify snapshot captured
  expect(event.metadata.snapshot).toBeDefined();
  expect(event.metadata.snapshot.before).toBeDefined();
  expect(event.metadata.snapshot.after).toBeDefined();

  // Verify accessibility context
  expect(event.metadata.accessibility).toBeDefined();
  expect(event.metadata.accessibility.aria_label).toBeDefined();
});
```

---

## Success Criteria (Sprint 1)

- [ ] All unit tests passing (35+ tests)
- [ ] All integration tests passing (10+ tests)
- [ ] Performance tests meet budget:
  - Extension overhead <0.1ms per event
  - Memory <5MB additional
  - Snapshot creation <50ms
- [ ] Compatibility: Chrome 120+, Firefox 121+, Safari 17+
- [ ] E2E test on ShopBroken completes successfully
- [ ] No memory leaks over 1-hour session
- [ ] All v5.3 functionality still works (no regressions)

---

## Known Issues / Limitations (v6.0)

- [ ] Selector generation may fail on dynamically generated elements (v6.1: improve)
- [ ] Accessibility checks basic only; full WCAG checks in v6.1
- [ ] DOM snapshots truncated at 10KB (sufficient for v6.0)
- [ ] Timing accuracy ±5ms (good enough for AI understanding)

---

## Related Documents

- **Product Spec:** [PRODUCT_SPEC.md](PRODUCT_SPEC.md)
- **Tech Spec:** [TECH_SPEC.md](TECH_SPEC.md)
- **Test Site:** ShopBroken (localhost:3000)

---

**Status:** Ready for QA implementation
**Estimated Effort:** 1 week (parallel with development)
