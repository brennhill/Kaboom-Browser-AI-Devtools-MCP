---
doc_type: feature_index
feature_id: feature-page-structure-detection
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-07
code_paths:
  - src/background/commands/analyze.ts
  - cmd/browser-agent/internal/toolanalyze/navigation.go
  - cmd/browser-agent/internal/toolanalyze/analyzedispatch/dispatcher.go
  - internal/schema/analyze.go
test_paths:
  - cmd/browser-agent/internal/toolanalyze/handlers_coverage_test.go
  - cmd/browser-agent/internal/toolanalyze/analyzedispatch/dispatcher_test.go
  - internal/schema/invariants_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Page Structure Detection

## TL;DR
- Status: proposed
- Tool: `analyze`
- Mode: `page_structure`

## Specs
- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Design Reference: [design-spec.md](./design-spec.md)
