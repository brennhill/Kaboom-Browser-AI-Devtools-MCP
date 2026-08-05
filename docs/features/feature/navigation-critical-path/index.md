---
doc_type: feature_index
feature_id: feature-navigation-critical-path
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-05
code_paths:
  - internal/performance/navigation/critical_path.go
  - internal/tools/observe/session/session.go
test_paths:
  - internal/performance/navigation/critical_path_test.go
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# Navigation Critical Path

## TL;DR

`analyze({what:"performance"})` returns an ordered timeline from navigation
TTFB through auth, application requests, state updates, React commits, FCP, and
LCP. Missing browser or application evidence is explicitly unavailable, never
reported as a synthetic zero.
Candidates must occur after the preceding available phase; unrelated slow work
outside that monotonic chain is not promoted into the critical path.

## Specs

- [Product specification](./product-spec.md)
- [Technical specification](./tech-spec.md)
- [QA plan](./qa-plan.md)
