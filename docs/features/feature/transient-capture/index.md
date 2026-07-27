---
doc_type: feature_index
feature_id: feature-transient-capture
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-27
code_paths:
  - cmd/browser-agent/tools_async_formatting.go
  - cmd/browser-agent/internal/asyncresult/transients.go
  - src/lib/page/transient-capture.ts
test_paths:
  - cmd/browser-agent/tools_async_transient_test.go
  - internal/tools/observe/session_transients_test.go
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
