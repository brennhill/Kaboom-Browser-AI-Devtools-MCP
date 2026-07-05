---
status: proposed
scope: feature/causality-analysis
ai-priority: medium
tags: [v7, analysis, debugging, eyes]
relates-to: [../request-session-correlation.md, ../../core/architecture.md]
last-verified: 2026-03-05
doc_type: product-spec
feature_id: feature-causality-analysis
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Causality Analysis

## Overview
Causality Analysis automatically detects cause-and-effect relationships in system events. When a test fails, Kaboom asks "What changed right before the failure?" and highlights the most likely causes. When a user reports "Payment declined," Kaboom identifies whether the failure was due to network latency, backend error, feature flag state, or user input. By analyzing event timestamps, state changes, error propagation, and dependencies, Kaboom builds a causal graph showing which events directly caused which outcomes.

## Problem
Current debugging requires manual hypothesis testing: "Is the failure due to the network request? The backend? The database query? The feature flag?" Developers must examine logs in sequence, trace dependencies manually, and guess at causality based on timestamps. This is slow and error-prone, especially with latency: a network request at timestamp X could cause an effect at timestamp X+500ms.

## Solution
Causality Analysis:
1. **Temporal Analysis:** Identify events in rapid sequence (cause precedes effect)
2. **State Change Detection:** Track state changes (feature flags, environment variables)
3. **Dependency Inference:** Connect events via shared context (request ID, variable name)
4. **Failure Attribution:** Propagate errors backward to their source
5. **Causal Graph:** Visualize cause-and-effect chains

## User Stories
- As a developer, I want to see "this error was caused by this feature flag toggle" so that I can instantly identify the root cause
- As a QA engineer, I want to know "this test failed because of this network timeout" so that I can create a concise bug report
- As a DevOps engineer, I want to trace a cascade failure back to the triggering event so that I can implement circuit breaker logic

## Acceptance Criteria
- [ ] Detect causal events: error → previous network request, assertion failure → missing element
- [ ] Identify feature flag/environment changes as triggers for behavior changes
- [ ] Propagate errors backward: "This error in service A caused this timeout in service B"
- [ ] Generate causal graph: UI shows arrows connecting causes to effects
- [ ] API: `analyze({what: 'causality', event_id: 'xyz'})` returns cause candidates
- [ ] Configurable confidence threshold: only show likely causes

## Not In Scope
- Machine learning causality inference (rule-based only)
- Cross-session causality
- Distributed tracing (use W3C trace context)

## Data Structures

### Causal Graph
```json
{
  "event_id": "assertion-failed-123",
  "event": "assertion failed: expect(button).visible",
  "timestamp": "2026-01-31T10:15:30.456Z",
  "likely_causes": [
    {
      "event_id": "network-req-456",
      "event": "XHR GET /api/button failed (404)",
      "timestamp": "2026-01-31T10:15:25.200Z",
      "confidence": 0.95,
      "reason": "404 response precedes missing element by 5s"
    },
    {
      "event_id": "feature-toggle-789",
      "event": "Feature flag 'show_button' changed to false",
      "timestamp": "2026-01-31T10:15:20.100Z",
      "confidence": 0.80,
      "reason": "Flag change precedes missing element"
    }
  ]
}
```

## Examples

### Example 1: Network Failure Causes Test Failure
```
[10:15:20] XHR GET /api/products → timeout
[10:15:25] Page still shows "Loading..."
[10:15:30] Assertion: expect(page).toHaveSelector('.product-list') → FAILED

Causality: "Your test failed because the products API call timed out"
```

### Example 2: Feature Flag Causes Behavior Change
```
[10:15:10] Feature flag 'checkout_redesign' toggled to TRUE
[10:15:15] User navigates to /checkout
[10:15:20] Page shows new UI (v2)
[10:15:30] User input validation changes

Causality: "Behavior changed because of feature flag toggle"
```

## MCP Changes
```javascript
analyze({
  what: 'causality',
  event_id: 'assertion-failed-123',
  confidence_threshold: 0.70  // Only show likely causes
})
```

Returns causal graph with ranked causes.
