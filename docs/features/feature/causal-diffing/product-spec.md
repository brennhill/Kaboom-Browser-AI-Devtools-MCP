---
feature: causal-diffing
status: proposed
tool: configure
mode: causal_diff
version: 0.7.12
doc_type: product-spec
feature_id: feature-causal-diffing
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Causal Diffing

## Problem Statement

When a performance regression is detected, the AI knows THAT something got slower but not WHY. A 500ms load time increase could be caused by:
- A new 300KB bundle added to the page
- A slow API response (backend regression)
- A render-blocking stylesheet
- An unoptimized image
- Increased DOM complexity

Without causal attribution, the AI has no actionable recourse. It can only report "performance regressed by 500ms" and wait for a human to open DevTools, manually compare resource waterfalls, and identify the culprit. This defeats the purpose of autonomous performance monitoring.

**The gap:** Performance budget monitor detects regressions. Push notifications deliver alerts. But neither explains WHY performance changed. The AI receives a symptom without a diagnosis.

## Solution

Add `configure({what: "causal_diff"})` -- a new action that performs resource-level comparison between the baseline state and the current state. It identifies what's new, what's gone, what got bigger, and what got slower, then summarizes the probable cause in a structured format the AI can act on directly.

Causal diffing answers: "Why did performance change?" by comparing resource waterfalls and attributing regression to specific resources (scripts, stylesheets, API calls, images, fonts).

## Requirements

- R1: Compare resource lists between baseline and current snapshots (added, removed, resized, retimed resources)
- R2: Detect render-blocking resources (scripts without async/defer, stylesheets in head)
- R3: Detect critical-path resources (resources initiated before FCP)
- R4: Compute probable cause summary explaining regression
- R5: Generate actionable recommendations (lazy-load, investigate API, review imports)
- R6: Support explicit baseline comparison (specify baseline_id) or automatic (use most recent)
- R7: Normalize URLs for comparison (strip cache-busters, preserve hash fragments)
- R8: Handle large resource lists (>200 entries) by storing top 50 by transfer size
- R9: Group dynamic API paths by prefix (e.g., /api/user/123 and /api/user/456 treated as same resource)
- R10: Report when no resource changes detected (probable cause: backend, DOM complexity, or throttling)

## Out of Scope

- This feature does NOT perform the initial baseline capture. That's handled by existing performance budget and diff_sessions features.
- This feature does NOT fix performance issues. It diagnoses them. The AI uses the diagnosis to propose code changes.
- This feature does NOT monitor performance continuously. It's invoked on-demand when a regression is detected.
- Out of scope: Visual regression detection (pixel-level comparison). Causal diffing is resource-level only.
- Out of scope: Memory profiling or JavaScript heap analysis. This is network/resource-focused.

## Success Criteria

- AI can call causal_diff after receiving a performance regression alert
- Response includes categorized resource changes (added, removed, resized, retimed)
- Response includes render-blocking and critical-path resource identification
- Probable cause summary is actionable (e.g., "chart-library.js added 412KB and is render-blocking")
- Recommendations array contains specific next steps (e.g., "Consider lazy-loading X")
- Resource fingerprinting adds < 50KB per baseline
- Diff computation completes in < 5ms

## User Workflow

1. AI receives performance regression alert via `observe({what: "changes"})`
2. Alert says "load time regressed by 500ms" with recommendation: "Use causal_diff for resource-level analysis"
3. AI calls `configure({what: "causal_diff", url: "/dashboard"})`
4. Server compares current resource waterfall against baseline waterfall
5. Server returns structured diff: added/removed/resized/retimed resources, probable cause, recommendations
6. AI uses diagnosis to propose fix (e.g., "Add dynamic import for chart-library.js to reduce blocking time")
7. Developer reviews and applies fix
8. AI watches `observe({what: "changes"})` for next snapshot confirming fix

## Examples

### Example 1: New render-blocking script added

Request:
```json
{
  "tool": "configure",
  "arguments": {
    "action": "causal_diff",
    "url": "/dashboard"
  }
}
```

Response:
```json
{
  "url": "/dashboard",
  "timing_delta": {
    "load_ms": 847,
    "fcp_ms": 320,
    "lcp_ms": 650
  },
  "resource_changes": {
    "added": [
      {
        "url": "/static/js/chart-library.js",
        "type": "script",
        "size_bytes": 412000,
        "duration_ms": 250,
        "render_blocking": true
      }
    ],
    "removed": [],
    "resized": [],
    "retimed": []
  },
  "probable_cause": "Added 412KB in new scripts (chart-library.js is render-blocking). Total payload increased by 35%.",
  "recommendations": [
    "Consider lazy-loading chart-library.js (412KB, render-blocking)",
    "Use dynamic import to load chart library only when needed"
  ]
}
```

### Example 2: API response slowed down

Response:
```json
{
  "url": "/dashboard",
  "timing_delta": {
    "load_ms": 340,
    "fcp_ms": 15,
    "lcp_ms": 340
  },
  "resource_changes": {
    "added": [],
    "removed": [],
    "resized": [],
    "retimed": [
      {
        "url": "/api/dashboard/data",
        "baseline_ms": 80,
        "current_ms": 340,
        "delta_ms": 260
      }
    ]
  },
  "probable_cause": "API response /api/dashboard/data slowed by 260ms. No frontend changes detected.",
  "recommendations": [
    "Investigate backend performance on /api/dashboard/data",
    "Check database query performance or cache invalidation"
  ]
}
```

### Example 3: Bundle size increased

Response:
```json
{
  "url": "/app",
  "timing_delta": {
    "load_ms": 180,
    "fcp_ms": 120,
    "lcp_ms": 150
  },
  "resource_changes": {
    "added": [],
    "removed": [],
    "resized": [
      {
        "url": "/static/js/main.chunk.js",
        "baseline_bytes": 145000,
        "current_bytes": 198000,
        "delta_bytes": 53000
      }
    ],
    "retimed": []
  },
  "probable_cause": "main.chunk.js grew by 53KB (37% increase). Total payload increased by 12%.",
  "recommendations": [
    "Review recent imports in main.chunk.js",
    "Check for unintentionally bundled dependencies",
    "Run bundle analyzer to identify large additions"
  ]
}
```

---

## Notes

- Integrates with Performance Budget (detects regressions) and Push Notifications (delivers alerts)
- Resource fingerprint stored alongside each performance baseline (incremental storage, no duplicate network calls)
- Query params stripped for URL normalization but hash fragments preserved (they indicate code-split chunks)
- Small resources (<1KB) aggregated in summary to reduce noise
