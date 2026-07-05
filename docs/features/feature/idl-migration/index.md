---
doc_type: feature_index
feature_id: feature-idl-migration
status: draft
feature_type: feature
owners: []
last_reviewed: 2026-07-05
code_paths:
  - scripts/generate-wire-types.js
  - scripts/check-wire-drift.js
  - internal/types/wire_enhanced_action.go
  - internal/types/wire_network.go
  - internal/schema/interact.go
test_paths:
  - internal/schema/invariants_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# IDL Migration

## TL;DR
- Status: draft
- Scope: unify Go/TS boundary contracts under a single schema source

## Specs
- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Design Reference: [design-spec.md](./design-spec.md)
