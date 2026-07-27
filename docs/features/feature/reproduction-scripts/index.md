---
doc_type: feature_index
feature_id: feature-reproduction-scripts
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-27
code_paths:
  - cmd/browser-agent/tools_generate.go
  - src/lib/page/reproduction.ts
test_paths:
  - cmd/browser-agent/reproduction_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Reproduction Scripts

## TL;DR

- Status: proposed
- Tool: generate
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/reproduction-scripts`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_REPRODUCTION_SCRIPTS_001
- FEATURE_REPRODUCTION_SCRIPTS_002
- FEATURE_REPRODUCTION_SCRIPTS_003

## Code and Tests

- MCP adapter and script formatting: `cmd/browser-agent/tools_generate.go`
- Page-side reproduction capture: `src/lib/page/reproduction.ts`
- Go behavior coverage: `cmd/browser-agent/reproduction_test.go`
