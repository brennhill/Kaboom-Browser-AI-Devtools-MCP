---
doc_type: feature_index
feature_id: feature-ai-web-pilot
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - cmd/browser-agent/internal/toolguard/guards.go
  - cmd/browser-agent/tools_core.go
  - cmd/browser-agent/internal/toolinteract/interact_evidence.go
test_paths:
  - cmd/browser-agent/tools_interact_gate_test.go
  - cmd/browser-agent/tools_coldstart_gate_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Ai Web Pilot

## TL;DR

- Status: shipped
- Tool: interact
- Mode/Action: navigate, execute_js, highlight
- Location: `docs/features/feature/ai-web-pilot`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_AI_WEB_PILOT_001
- FEATURE_AI_WEB_PILOT_002
- FEATURE_AI_WEB_PILOT_003

## Code and Tests

Pilot, extension-connectivity, tracked-tab, and CSP preconditions are owned by
`internal/toolguard`. Tool adapters receive those canonical guard methods
directly; package-main no longer carries a duplicate guard surface.
