---
doc_type: feature_index
feature_id: feature-transient-capture
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-11-21
code_paths:
  - internal/tools/asynccommand/handler.go
  - internal/tools/asyncresult/transients.go
  - src/lib/page/transient-capture.ts
test_paths:
  - internal/tools/asynccommand/transient_test.go
  - internal/tools/observe/session/session_transients_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Transient Capture

## TL;DR
- Status: shipped
- Scope: capture short-lived UI state for debugging/observe workflows

## Specs
- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Canonical Note
Transient capture should prefer event/timeline-safe snapshots and avoid introducing observer-induced UI mutations.
Completed command enrichment is owned by `internal/tools/asynccommand.Handler`, which
reads the canonical capture timeline and delegates filtering and bounds to
`internal/tools/asyncresult`.
