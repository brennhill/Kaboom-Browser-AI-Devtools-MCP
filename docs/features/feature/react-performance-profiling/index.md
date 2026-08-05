---
doc_type: feature_index
feature_id: feature-react-performance-profiling
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-05
code_paths:
  - src/lib/analysis/react-profiler.ts
  - src/inject/api.ts
  - src/background/commands/analyze.ts
  - cmd/browser-agent/internal/toolanalyze/analyzedispatch/react_profile.go
  - internal/schema/analyze.go
test_paths:
  - tests/extension/performance/react-profiler.test.js
  - cmd/browser-agent/internal/toolanalyze/analyzedispatch/react_profile_test.go
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# React Performance Profiling

## TL;DR

- Status: shipped
- Tool: `analyze`
- Mode: `react_profile`
- Lifecycle: explicit `start`, exercise the page, then `stop`

## Specs

- [Product specification](./product-spec.md)
- [Technical specification](./tech-spec.md)
- [QA plan](./qa-plan.md)

The profiler is off unless explicitly started. It records bounded commit and
component timing evidence through React's DevTools hook and restores the
original hook when stopped. It records changed property names, never values.
Restoration is ownership-aware, so a profiler installed later is never
overwritten. Navigation to a page with a new DevTools hook preserves that new
hook while the profiler restores only the old page callback it still owns.
Reported `actualDuration` values are explicitly labeled as subtree-inclusive
rather than exclusive component CPU time.
