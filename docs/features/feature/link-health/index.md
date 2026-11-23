---
doc_type: feature_index
feature_id: feature-link-health
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-11-21
code_paths:
  - internal/tools/asynccommand/handler.go
  - internal/tools/analyze/handler/navigation.go
  - internal/tools/analyze/handler/analyzedispatch/dispatcher.go
  - internal/tools/analyze/handler/linkvalidation/handler.go
  - src/lib/analysis/link-health.ts
  - src/background/pending-queries.ts
  - src/content/message-handlers.ts
  - src/inject/message-handlers.ts
test_paths:
  - internal/tools/asynccommand/handler_test.go
  - internal/tools/analyze/handler/handlers_coverage_test.go
  - internal/tools/analyze/handler/linkvalidation/handler_test.go
  - scripts/tests/browser/cat-19-link-health.sh
  - scripts/smoke-tests/links/link-health-smoke.sh
  - tests/cli/contracts/uat-harness-regressions.test.cjs
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Link Health

## TL;DR
- Status: shipped
- Tool: `analyze`
- Modes: `what:"link_health"` and `what:"link_validation"`

## Specs
- Product: `product-spec.md`
- Tech: `tech-spec.md`
- QA: `qa-plan.md`
