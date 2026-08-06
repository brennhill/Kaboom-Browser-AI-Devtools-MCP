---
status: draft
priority: tier-1
phase: v6.0-sprint-1
relates-to: [ring-buffer, normalized-event-schema]
last-updated: 2026-01-31
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Browser Extension Enhancement — Technical Specification

**Goal:** Implement performance timing, DOM snapshots, and accessibility capture as normalized events for v6.0.

---

## Architecture Overview

```
Content Script (page context)
├─ PerformanceObserver (Web Vitals)
├─ MutationObserver (DOM changes)
├─ PerformanceEntries (user timing)
└─ Action Tracker (click, type, navigate)
    ├─ Capture timing
    ├─ Capture DOM snapshot
    ├─ Capture accessibility context
    └─ Emit NormalizedEvent

      ↓
Background Script (extension context)
├─ Message handler (receive events from content script)
├─ Ring buffer queue
├─ Normalize event schema
└─ Send to MCP bridge (localhost:7890/event)

      ↓
MCP Bridge (v5.3+)
├─ HTTP POST /event
├─ Route to ring buffer
└─ Make available to LLM via MCP observe()
```

---

## Implementation Plan

### Phase 1: Action Timing (Week 1, Days 1-2)

**File:** `extension/src/content-script/action-timer.ts`

```typescript
interface ActionTimingRecord {
  action_id: string;
  action_type: 'click' | 'type' | 'navigate' | 'scroll' | 'focus';
  start_time: number;
  end_time?: number;

  // Timing measurements
  interaction_latency_ms?: number;  // JS handler execution
  dom_latency_ms?: number;           // First DOM mutation
  network_latency_ms?: number;       // Longest network request
  total_latency_ms?: number;         // Start → complete

  // Network context
  network_requests: Array<{
    method: string;
    url: string;
    start_time: number;
    end_time: number;
    status?: number;
    latency_ms: number;
  }>;

  // Element context
  element_selector: string;
  element_coordinates?: [number, number];  // x, y
}

class ActionTimer {
  private timings = new Map<string, ActionTimingRecord>();
  private mutationObserver: MutationObserver;
  private networkObserver: PerformanceObserver;

  constructor() {
    this.setupMutationTracking();
    this.setupNetworkTracking();
    this.setupActionTracking();
  }

  private setupMutationTracking() {
    this.mutationObserver = new MutationObserver((mutations) => {
      // Find associated action_id from most recent action
      // Record timing to first mutation
    });

    this.mutationObserver.observe(document.documentElement, {
      subtree: true,
      attributes: true,
      characterData: true,
      childList: true,
    });
  }

  private setupNetworkTracking() {
    // Track fetch() and XMLHttpRequest via PerformanceObserver
    this.networkObserver = new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        // Link network request to action_id
        // Record timing
      }
    });

    this.networkObserver.observe({
      entryTypes: ['resource', 'navigation'],
      buffered: true,
    });
  }

  private setupActionTracking() {
    // Listen for click, type, navigate
    document.addEventListener('click', (e) => {
      this.recordActionStart('click', e.target as HTMLElement);
    });

    // Type tracking via input event
    document.addEventListener('input', (e) => {
      this.recordActionStart('type', e.target as HTMLElement);
    });
  }

  recordActionStart(type: string, element: HTMLElement) {
    const action_id = crypto.randomUUID();
    const record: ActionTimingRecord = {
      action_id,
      action_type: type as any,
      start_time: performance.now(),
      network_requests: [],
      element_selector: this.generateSelector(element),
      element_coordinates: this.getElementCoordinates(element),
    };

    this.timings.set(action_id, record);

    // Schedule end (with timeout for network requests)
    setTimeout(() => this.recordActionEnd(action_id), 1000);

    return action_id;
  }

  recordActionEnd(action_id: string) {
    const record = this.timings.get(action_id);
    if (!record) return;

    record.end_time = performance.now();
    record.total_latency_ms = record.end_time - record.start_time;

    // Emit as normalized event
    window.postMessage({
      type: 'KABOOM_ACTION_TIMING',
      payload: record,
    }, '*');

    this.timings.delete(action_id);
  }

  private generateSelector(element: HTMLElement): string {
    // Smart selector generation (see Phase 4)
    return 'button.add-to-cart'; // placeholder
  }

  private getElementCoordinates(element: HTMLElement): [number, number] {
    const rect = element.getBoundingClientRect();
    return [rect.left + rect.width / 2, rect.top + rect.height / 2];
  }
}

// Export singleton
export const actionTimer = new ActionTimer();
```

#### Integration with v5.3:
- Add to content script initialization
- Events sent via postMessage (already trusted channel to background)
- No breaking changes to existing telemetry

#### Tests:
- [ ] Click button, timing recorded
- [ ] Type in input, timing recorded
- [ ] Navigation tracked
- [ ] Network requests associated with action
- [ ] Timing accurate within ±20ms

---

### Phase 2: DOM Snapshots (Week 1, Days 3-4)

**File:** `extension/src/content-script/dom-snapshot.ts`

```typescript
interface DOMSnapshot {
  snapshot_id: string;
  action_id: string;
  timestamp: number;
  direction: 'before' | 'after';

  // Full HTML (compressed)
  html: string;  // max 10KB

  // Metadata
  element_count: number;
  checksum: string;

  // Diff if comparing to previous
  diff?: {
    type: 'added' | 'removed' | 'modified' | 'attribute_changed' | 'text_changed';
    selector: string;
    before_value?: string;
    after_value?: string;
  }[];
}

class DOMSnapshotCapture {
  private lastSnapshot?: DOMSnapshot;

  captureBeforeAction(action_id: string, element: HTMLElement): DOMSnapshot {
    const snapshot = this.createSnapshot(action_id, 'before', element);
    this.lastSnapshot = snapshot;
    return snapshot;
  }

  captureAfterAction(action_id: string, element: HTMLElement): DOMSnapshot {
    const snapshot = this.createSnapshot(action_id, 'after', element);

    if (this.lastSnapshot && this.lastSnapshot.action_id === action_id) {
      snapshot.diff = this.computeDiff(this.lastSnapshot, snapshot);
    }

    return snapshot;
  }

  private createSnapshot(
    action_id: string,
    direction: 'before' | 'after',
    element: HTMLElement
  ): DOMSnapshot {
    // Clone element to avoid modifying live DOM
    const clone = element.cloneNode(true) as HTMLElement;

    // Remove script tags (security)
    clone.querySelectorAll('script').forEach(el => el.remove());

    // Compress HTML
    let html = clone.innerHTML
      .replace(/>\s+</g, '><')  // Remove whitespace between tags
      .replace(/<!--[\s\S]*?-->/g, '');  // Remove comments

    // Truncate if too large
    if (html.length > 10000) {
      html = html.substring(0, 10000) + '...[truncated]';
    }

    return {
      snapshot_id: crypto.randomUUID(),
      action_id,
      timestamp: performance.now(),
      direction,
      html,
      element_count: clone.querySelectorAll('*').length,
      checksum: this.checksum(html),
    };
  }

  private computeDiff(before: DOMSnapshot, after: DOMSnapshot): any[] {
    // Simple diff: compare element counts, checksums
    // For v6.0, just basic detection; v6.1 can add granular diffing

    const changes = [];

    if (before.checksum !== after.checksum) {
      if (before.element_count < after.element_count) {
        changes.push({
          type: 'added',
          count: after.element_count - before.element_count,
        });
      } else if (before.element_count > after.element_count) {
        changes.push({
          type: 'removed',
          count: before.element_count - after.element_count,
        });
      } else {
        changes.push({
          type: 'modified',
          description: 'HTML changed but element count same',
        });
      }
    }

    return changes;
  }

  private checksum(html: string): string {
    // Simple hash for comparison
    let hash = 0;
    for (let i = 0; i < html.length; i++) {
      const char = html.charCodeAt(i);
      hash = ((hash << 5) - hash) + char;
      hash = hash & hash;  // Convert to 32-bit integer
    }
    return hash.toString(36);
  }
}

export const domSnapshot = new DOMSnapshotCapture();
```

#### Integration:
- Call `captureBeforeAction()` before emitting action start
- Call `captureAfterAction()` after action completes
- Emit as normalized event

#### Tests:
- [ ] Snapshot captures HTML
- [ ] HTML compressed correctly
- [ ] Diff detects added/removed elements
- [ ] Diff detects text changes
- [ ] Snapshot < 5KB

---

### Phase 3: Accessibility Context (Week 1, Days 5)

**File:** `extension/src/content-script/accessibility-context.ts`

```typescript
interface AccessibilityContext {
  context_id: string;
  action_id: string;
  timestamp: number;

  // Element semantics
  element_selector: string;
  element_role: string;
  element_aria_label?: string;
  element_aria_description?: string;
  element_aria_disabled?: boolean;
  element_aria_hidden?: boolean;
  element_aria_live?: string;

  // Visible text
  visible_text: string;
  placeholder?: string;

  // Form context
  form_name?: string;
  form_method?: string;
  form_action?: string;

  // Navigation context
  href?: string;
  target?: string;

  // Violations (basic WCAG checks)
  violations: Array<{
    criterion: string;  // e.g., "1.4.3"
    level: 'A' | 'AA' | 'AAA';
    severity: 'critical' | 'major' | 'minor';
    message: string;
  }>;
}

class AccessibilityContextCapture {
  captureForElement(action_id: string, element: HTMLElement): AccessibilityContext {
    const context: AccessibilityContext = {
      context_id: crypto.randomUUID(),
      action_id,
      timestamp: performance.now(),
      element_selector: this.generateSelector(element),
      element_role: element.getAttribute('role') || element.tagName.toLowerCase(),
      element_aria_label: element.getAttribute('aria-label') || undefined,
      element_aria_description: element.getAttribute('aria-description') || undefined,
      element_aria_disabled: element.hasAttribute('aria-disabled'),
      element_aria_hidden: element.hasAttribute('aria-hidden'),
      element_aria_live: element.getAttribute('aria-live') || undefined,
      visible_text: element.textContent?.trim() || '',
      placeholder: (element as HTMLInputElement).placeholder || undefined,
      violations: this.detectViolations(element),
    };

    // Additional context based on element type
    if (element.tagName === 'A') {
      context.href = element.getAttribute('href') || undefined;
      context.target = element.getAttribute('target') || undefined;
    }

    if (element.tagName === 'BUTTON' || element.tagName === 'INPUT') {
      const form = element.closest('form');
      if (form) {
        context.form_name = form.name || undefined;
        context.form_method = form.method || 'GET';
        context.form_action = form.action || undefined;
      }
    }

    return context;
  }

  private detectViolations(element: HTMLElement): any[] {
    const violations = [];

    // Check 1: Images without alt text
    if (element.tagName === 'IMG' && !element.hasAttribute('alt')) {
      violations.push({
        criterion: '1.1.1',
        level: 'A',
        severity: 'critical',
        message: 'Image missing alt text',
      });
    }

    // Check 2: Button without text or aria-label
    if (
      element.tagName === 'BUTTON' &&
      !element.textContent?.trim() &&
      !element.hasAttribute('aria-label')
    ) {
      violations.push({
        criterion: '1.1.1',
        level: 'A',
        severity: 'major',
        message: 'Button has no accessible text or aria-label',
      });
    }

    // Check 3: Form input without label
    if (element.tagName === 'INPUT') {
      const label = document.querySelector(`label[for="${element.id}"]`);
      if (!label && !element.hasAttribute('aria-label')) {
        violations.push({
          criterion: '1.3.1',
          level: 'A',
          severity: 'major',
          message: 'Input field has no associated label',
        });
      }
    }

    // Check 4: Color contrast (simplified)
    const style = window.getComputedStyle(element);
    const bgColor = style.backgroundColor;
    const fgColor = style.color;
    const contrast = this.estimateContrast(fgColor, bgColor);

    if (contrast < 4.5 && element.textContent?.trim()) {
      violations.push({
        criterion: '1.4.3',
        level: 'AA',
        severity: 'major',
        message: `Color contrast low: ${contrast.toFixed(2)}:1 (need ≥4.5:1)`,
      });
    }

    return violations;
  }

  private estimateContrast(fgColor: string, bgColor: string): number {
    // Simplified contrast calculation (v6.1 can use full WCAG formula)
    // For now, return placeholder
    return 5.0;
  }

  private generateSelector(element: HTMLElement): string {
    // Placeholder - see Phase 4
    return 'button.add-to-cart';
  }
}

export const a11yContext = new AccessibilityContextCapture();
```

#### Integration:
- Call on every interaction
- Emit as normalized event
- Use for AI semantic understanding

#### Tests:
- [ ] aria-label captured
- [ ] role detected
- [ ] Form context extracted
- [ ] Violations detected
- [ ] Contrast calculation works

---

### Phase 4: Smart Selector Generation (Week 1, Day 5)

**File:** `extension/src/content-script/smart-selector.ts`

```typescript
interface SmartSelector {
  primary: string;         // Most specific, data-testid if available
  fallback: string;        // Text or class-based
  semantic: string;        // Human-readable
  confidence_pct: number;  // Likelihood selector survives changes
}

class SmartSelectorGenerator {
  generate(element: HTMLElement): SmartSelector {
    const selectors = {
      primary: this.getPrimarySelector(element),
      fallback: this.getFallbackSelector(element),
      semantic: this.getSemanticSelector(element),
    };

    const confidence = this.estimateConfidence(element, selectors.primary);

    return {
      ...selectors,
      confidence_pct: confidence,
    };
  }

  private getPrimarySelector(element: HTMLElement): string {
    // Priority order:
    // 1. data-testid
    // 2. id
    // 3. aria-label
    // 4. class combination
    // 5. tag[nth-child]

    const testId = element.getAttribute('data-testid');
    if (testId) return `[data-testid="${testId}"]`;

    const id = element.id;
    if (id && this.isValidCSSIdentifier(id)) {
      return `#${id}`;
    }

    const ariaLabel = element.getAttribute('aria-label');
    if (ariaLabel) {
      return `[aria-label="${ariaLabel}"]`;
    }

    // Class combination
    const classes = element.className;
    if (classes && classes.trim()) {
      const classList = classes.split(/\s+/).filter(c => !c.startsWith('ng-'));
      if (classList.length > 0) {
        return `${element.tagName.toLowerCase()}.${classList.join('.')}`;
      }
    }

    // Fallback: nth-child
    const parent = element.parentElement;
    if (parent) {
      const index = Array.from(parent.children).indexOf(element) + 1;
      return `${element.tagName.toLowerCase()}:nth-child(${index})`;
    }

    return element.tagName.toLowerCase();
  }

  private getFallbackSelector(element: HTMLElement): string {
    // CSS selector with text matching (Playwright syntax)
    const text = element.textContent?.trim().substring(0, 50);

    if (text) {
      // Playwright selector with text
      return `${element.tagName.toLowerCase()}:has-text("${text}")`;
    }

    // If no text, use class
    return this.getPrimarySelector(element);
  }

  private getSemanticSelector(element: HTMLElement): string {
    // Human-readable description
    const tag = element.tagName.toLowerCase();
    const text = element.textContent?.trim().substring(0, 30);
    const aria = element.getAttribute('aria-label');

    if (aria) {
      return `${tag} "${aria}"`;
    }

    if (text) {
      return `${tag} with text "${text}"`;
    }

    const classes = element.className.split(/\s+/).filter(c => !c.startsWith('ng-'));
    if (classes.length > 0) {
      return `${tag}.${classes.join('.')}`;
    }

    return tag;
  }

  private estimateConfidence(element: HTMLElement, selector: string): number {
    // Confidence based on selector specificity
    // data-testid: 95% (likely stable)
    // id: 90% (might change)
    // aria-label: 85% (text can change)
    // class: 70% (classes often change)
    // nth-child: 30% (fragile)

    if (selector.includes('data-testid')) return 95;
    if (selector.includes('#')) return 90;
    if (selector.includes('aria-label')) return 85;
    if (selector.includes('.')) return 70;
    if (selector.includes('nth-child')) return 30;

    return 50;
  }

  private isValidCSSIdentifier(id: string): boolean {
    return /^[a-zA-Z_-][a-zA-Z0-9_-]*$/.test(id);
  }
}

export const selectorGenerator = new SmartSelectorGenerator();
```

#### Integration:
- Generate selector for every action
- Include in action metadata
- Used by LLM for test generation

#### Tests:
- [ ] data-testid used as primary
- [ ] Fallback to class if no testid
- [ ] Semantic text generated
- [ ] Confidence calculated
- [ ] Selector works against live DOM

---

## Normalized Event Format

All v6.0 browser events emit as:

```typescript
interface NormalizedBrowserEvent {
  id: string;
  timestamp: number;
  source: 'browser';
  level: 'debug' | 'info' | 'warn' | 'error' | 'critical';
  correlation_id?: string;
  message: string;
  metadata: {
    // Base event properties
    event_type: 'action' | 'log' | 'network' | 'exception' | 'snapshot' | 'accessibility';

    // For actions
    action?: {
      type: 'click' | 'type' | 'navigate' | 'scroll' | 'focus';
      element_selector: string;
      smart_selector?: SmartSelector;
      timing?: ActionTimingRecord;
      dom_snapshot?: DOMSnapshot;
      accessibility_context?: AccessibilityContext;
    };

    // For logs (v5.3)
    console_level?: 'log' | 'info' | 'warn' | 'error';
    console_args?: any[];

    // For network (v5.3)
    request?: {method: string; url: string; status: number};
    response_body?: string;
  };
  tags: string[];
}
```

---

## Performance Budget

### Browser Extension Per-Event Overhead:
- Action timing calculation: <0.1ms
- DOM snapshot: <50ms (async, not blocking)
- Accessibility scan: <10ms
- Selector generation: <5ms
- Total per action: <65ms (non-blocking)

### Memory Footprint:
- Timing records: ~1KB per 100 actions (maps cleared)
- Last snapshot: ~10KB
- Pending snapshots: ~20KB (max 2 concurrent)
- Selectors cache: ~5KB
- Total additional: ~40KB

---

## Testing Strategy

### Unit Tests (v6.0):
- [ ] Action timer accuracy (±5ms)
- [ ] DOM snapshot compression
- [ ] Diff algorithm correctness
- [ ] Selector generation for common patterns
- [ ] Accessibility violation detection

### Integration Tests (v6.0):
- [ ] Full action cycle: click → timing → snapshot → a11y → event
- [ ] Multiple concurrent actions
- [ ] Network request association
- [ ] Event emission to background script

### E2E Tests (v6.0):
- [ ] Run on ShopBroken checkout flow
- [ ] Verify all timing, snapshots, selectors captured
- [ ] Performance acceptable
- [ ] No lost events

---

## Rollout Plan

### Phase 1 (v6.0 sprint 1):
- Implement action timing, DOM snapshots, a11y capture
- Include in beta v6.0.0-beta.1
- Test on ShopBroken

### Phase 2 (v6.0 sprint 2):
- Refine selector generation based on feedback
- Add more accessibility checks
- Release v6.0.0

### Phase 3 (v6.1):
- Add video recording of actions
- Add pixel-level visual regression detection
- Improve WCAG scanning

---

## Related Documents

- **Product Spec:** [PRODUCT_SPEC.md](PRODUCT_SPEC.md)
- **QA Plan:** [QA_PLAN.md](QA_PLAN.md)
- **Architecture:** [360-observability-architecture.md](../../../core/product/README.md#ingestion-layer)

---

**Status:** Ready for implementation
**Estimated Effort:** 1 week (5 days)
**Dependencies:** None (builds on v5.3)
