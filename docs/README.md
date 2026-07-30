---
doc_type: docs-index
status: active
scope: documentation/navigation
ai-priority: high
tags: [docs, index, canonical]
last-verified: 2026-02-17
---

# Kaboom Docs Index

## Quick Links
- Project rules and commands: `../claude.md`
- Architecture reference: `../.claude/refs/architecture.md`
- Feature index: `features/feature-index.md`
- Core product spec (TARGET): `core/product-spec.md`
- Core tech spec (TARGET): `core/tech-spec.md`
- Core QA spec (TARGET): `core/qa-spec.md`
- Command/option matrix: `core/mcp-command-option-matrix.md`
- Release process: `core/release.md`
- Known issues: `core/known-issues.md`

## Structure
- `docs/core/`: canonical cross-feature product/tech/qa and release docs
- `docs/features/`: feature-level product/tech/qa docs
- `docs/adrs/`: architecture decision records
- `docs/templates/`: spec templates

## Canonical Code Anchors
- MCP request handling: `cmd/browser-agent/handler.go`
- Tool schemas: `internal/schema/schema.go`
- Tool dispatch: `cmd/browser-agent/tools_core.go`
- Extension startup composition: `src/background/init.ts`
- Extension connection runtime: `src/background/orchestration/connection-monitor.ts`
- Extension telemetry runtime: `src/background/orchestration/stream-runtime.ts`
- Extension sync client: `src/background/sync/sync-client.ts`

## Notes
- The codebase is the ground truth.
- If docs conflict with code, update docs and refresh `last-verified`.
- Use `docs/features/feature-index.md` as the feature navigation entrypoint.
