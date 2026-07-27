---
status: active
scope: features/navigation
ai-priority: high
tags: [features, navigation, canonical]
last-verified: 2026-03-05
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Features Docs Guide

## Start Here
- Feature index: `feature-index.md`
- Core TARGET specs:
- `../core/product-spec.md`
- `../core/tech-spec.md`
- `../core/qa-spec.md`
- Command/option traceability:
- `../core/mcp-command-option-matrix.md`

## Feature Folder Contract
Each feature under `docs/features/feature/<feature-name>/` should keep:
- `index.md` (entrypoint and status)
- `product-spec.md` (behavioral contract)
- `tech-spec.md` (implementation contract)
- `qa-plan.md` (verification contract)

## Canonical MCP References
- Tool schemas: `internal/schema/schema.go`
- Tool dispatch: `cmd/browser-agent/tools_core.go`
- Observe handlers: `cmd/browser-agent/internal/toolobserve/dispatcher.go`
- Analyze handlers: `cmd/browser-agent/tools_analyze.go`
- Configure handlers: `cmd/browser-agent/tools_configure.go`
- Interact handlers: `cmd/browser-agent/tools_interact.go`
- Generate handlers: `cmd/browser-agent/internal/toolgenerate/dispatcher.go`

## Update Rule
When schema/handler behavior changes:
1. Update `docs/core/mcp-command-option-matrix.md`.
2. Update affected feature `product-spec.md`, `tech-spec.md`, `qa-plan.md`.
3. Refresh `last-verified` in touched docs.
