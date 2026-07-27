---
doc_type: architecture
status: active
scope: features/core/target-architecture
ai-priority: high
tags: [core, architecture, target, canonical]
relates-to: [../../core/product-spec.md, ../../core/tech-spec.md, ../../core/qa-spec.md]
last-verified: 2026-03-05
canonical: true
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Target Architecture (Canonical Pointers)

This page is the architecture pointer for target behavior docs.

Canonical documents:
- Product contract: `docs/core/product-spec.md`
- Technical architecture and flows: `docs/core/tech-spec.md`
- QA contract and release gates: `docs/core/qa-spec.md`
- Command/option traceability: `docs/core/mcp-command-option-matrix.md`

Implementation anchors:
- MCP handler: `cmd/browser-agent/handler.go`
- Tool schemas: `internal/schema/schema.go`
- Tool dispatch: `cmd/browser-agent/tools_core.go`
- Query lifecycle: `internal/capture/queries.go`
- Unified sync endpoint: `internal/capture/sync.go`
- Extension sync executor: `src/background/sync/sync-client.ts`, `src/background/pending-queries.ts`
