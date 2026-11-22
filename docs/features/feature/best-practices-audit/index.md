---
doc_type: feature_index
feature_id: feature-best-practices-audit
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-11-21
code_paths:
  - internal/composition/toolruntime/tools_core.go
  - internal/tools/analyze/handler/deps.go
  - internal/tools/analyze/handler/combinedaudit/handler.go
  - internal/tools/analyze/handler/analyzedispatch/dispatcher.go
  - internal/tools/analyze/handler/audit_scoring.go
test_paths:
  - scripts/contracts/goarchitecturetests/contracts_test.go
  - internal/tools/analyze/handler/combinedaudit/handler_test.go
  - internal/tools/analyze/handler/toolanalyze_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Best Practices Audit

## TL;DR

- Status: proposed
- Tool: generate
- Mode/Action: best_practices_audit
- Location: `docs/features/feature/best-practices-audit`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_BEST_PRACTICES_AUDIT_001
- FEATURE_BEST_PRACTICES_AUDIT_002
- FEATURE_BEST_PRACTICES_AUDIT_003

## Code and Tests

Combined audit explicitly names its analyze and observe dependency groups.
It no longer embeds a catch-all host interface, so category changes remain
coupled to their owning feature boundary.
