---
doc_type: architecture_interfaces
status: active
last_reviewed: 2026-03-05
owners: []
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Interfaces

This file defines externally meaningful contracts and the compatibility policy for safe edits.

## Compatibility Policy

- Prefer additive changes.
- Avoid removing or renaming existing fields, actions, endpoints, or enum values without versioning.
- If a breaking change is unavoidable, require ADR + migration plan + explicit version signal.

## Interface Inventory

| Interface | Source of Truth | Compatibility Rule | Validation |
| --- | --- | --- | --- |
| MCP tool schemas (`observe`, `analyze`, `generate`, `configure`, `interact`) | `internal/schema/` | No breaking parameter/result changes without versioned plan | `go test ./cmd/browser-agent/...` |
| MCP JSON-RPC request/response envelope | `cmd/browser-agent/handler.go`, `internal/mcp/` | Must remain valid JSON-RPC 2.0 | MCP handler tests |
| Extension sync protocol (`/sync` and related result posts) | `internal/capture/sync.go`, `internal/capture/httpingest/handlers.go`, `src/background/sync/sync-client.ts` | Server/extension changes are coordinated in same PR | Go + extension tests |
| Go/TypeScript wire types | `internal/types/wire_*.go`, `src/types/wire/wire-*.ts` | Must stay generated/synchronized | `make check-wire-drift` |
| Extension message protocol (background <-> content/popup) | `src/background/message-handlers.ts`, `src/content.ts`, docs in `docs/core/extension-message-protocol.md` | Preserve existing message semantics unless versioned | Extension tests |
| CLI mode behavior | `cmd/browser-agent/cli.go`, `cli_commands.go` | Existing command behavior/output should not regress silently | CLI-focused Go tests |

## Breaking Change Checklist

Use this when an interface change cannot be additive:

1. Create ADR in `docs/architecture/` with rationale and alternatives.
2. Define migration path (old -> new) and rollback plan.
3. Gate release with targeted compatibility tests.
4. Update docs that reference the changed interface in same PR.
