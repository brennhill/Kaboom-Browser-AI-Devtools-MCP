---
status: proposed
scope: feature/perf-experimentation
ai-priority: high
tags: [performance, experimentation, vitals, closed-loop]
doc_type: product-spec
feature_id: feature-perf-experimentation
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Performance Experimentation

## Problem

AI agents can measure performance (vitals, network waterfall, performance snapshots) and they can make code changes. But running a before/after performance experiment requires ~7 manual tool calls and the AI has to interpret raw numbers itself. There's no first-class primitive for "measure, change, re-measure, compare."

Playwright can measure performance in scripted tests, but requires you to write the test first, runs against a synthetic browser, and can't act on the results. Kaboom already has the closed loop — it just needs better ergonomics for the experiment workflow.

## User Story

> "Lazy-load the hero image and check if LCP improves."

The AI should be able to: snapshot baseline → edit code → refresh → re-measure → return a structured comparison — with minimal tool calls and no manual number-crunching.

## Proposed API

### Option A: Snapshot + Compare on `observe`

```json
// Step 1: Baseline
observe({ what: "performance", snapshot: "before" })
// Returns: current vitals + saves snapshot labeled "before"

// Step 2: After code change + refresh
observe({ what: "performance", compare: "before" })
// Returns: current vitals + structured diff vs "before"
```

Response includes:
```json
{
  "current": { "lcp": 1.2, "fcp": 0.8, "cls": 0.01, "inp": 45 },
  "baseline": { "lcp": 2.8, "fcp": 0.9, "cls": 0.02, "inp": 52 },
  "diff": {
    "lcp": { "before": 2.8, "after": 1.2, "delta": -1.6, "percent": "-57%", "improved": true },
    "fcp": { "before": 0.9, "after": 0.8, "delta": -0.1, "percent": "-11%", "improved": true },
    "cls": { "before": 0.02, "after": 0.01, "delta": -0.01, "percent": "-50%", "improved": true },
    "inp": { "before": 52, "after": 45, "delta": -7, "percent": "-13%", "improved": true }
  },
  "summary": "All metrics improved. LCP improved 57% (2.8s → 1.2s)."
}
```

### Option B: Dedicated `generate` format

```json
generate({ what: "perf_comparison", baseline: "before" })
// Returns human-readable performance comparison report
```

### Recommendation

Option A — keeps it in `observe` where performance data already lives. The `snapshot` and `compare` parameters are additive (no breaking changes). The AI gets structured data it can reason about without parsing prose.

## What Gets Compared

| Metric | Source | Notes |
|--------|--------|-------|
| LCP | Web Vitals | Largest Contentful Paint |
| FCP | Web Vitals | First Contentful Paint |
| CLS | Web Vitals | Cumulative Layout Shift |
| INP | Web Vitals | Interaction to Next Paint |
| Resource count | Network waterfall | Total resources loaded |
| Total transfer size | Network waterfall | Bytes transferred |
| JS errors | Error buffer | Console error count |

## Workflow: 3 Calls Instead of 7+

| Step | Current (7+ calls) | Proposed (3 calls) |
|------|--------------------|--------------------|
| Baseline | observe(vitals) + observe(performance) + configure(store) | observe(performance, snapshot: "before") |
| Change | AI edits code | AI edits code |
| Refresh | interact(refresh) | interact(refresh) |
| Compare | observe(vitals) + observe(performance) + manual diff | observe(performance, compare: "before") |

## Scope

- Server-side only (Go). No extension changes needed.
- Snapshots stored in existing persistent store infrastructure.
- Snapshot auto-expires after 1 hour (configurable).
- Multiple named snapshots supported (e.g., "before-lazy-load", "before-code-split").

## Non-Goals

- Automated multi-run averaging (too complex for v1)
- Statistical significance testing
- CI integration (future: could feed into `generate(what: "pr_summary")`)

## Success Criteria

1. AI can run a complete before/after experiment in 3 tool calls
2. Structured diff response — no manual number parsing
3. Human-readable summary string included
4. Works with existing vitals + performance infrastructure
5. No new tools — extends existing `observe` tool
