---
doc_type: architecture_invariants
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Invariants

These are non-negotiable system behaviors. Any change that violates one requires a new ADR and explicit approval.

| ID | Invariant | Critical Paths | Minimum Checks |
| --- | --- | --- | --- |
| `ARCH_INV_001` | Async queue-and-poll flow remains intact for browser commands (`create -> poll -> execute -> result -> retrieve`). | `internal/queries/dispatcher_queries.go`, `internal/capture/query_dispatcher.go`, `internal/capture/sync.go` | `go test -v ./internal/capture -run TestAsyncQueueIntegration`, `./scripts/validate-architecture.sh` |
| `ARCH_INV_002` | Observe/interact command-status handlers keep real queue-backed behavior and never regress to stubs. | `cmd/browser-agent/tools_observe.go`, `cmd/browser-agent/tools_interact.go`, `cmd/browser-agent/tools_core.go` | `./scripts/validate-architecture.sh`, `go test ./cmd/browser-agent/...` |
| `ARCH_INV_003` | MCP request/response envelope remains valid JSON-RPC 2.0. | `cmd/browser-agent/handler.go`, `internal/mcp/` | `go test ./cmd/browser-agent/...` |
| `ARCH_INV_004` | Public MCP tool schemas remain backward compatible unless there is an explicit, versioned breaking-change plan. | `internal/schema/` | `go test ./cmd/browser-agent/...`, docs update in `interfaces.md` |
| `ARCH_INV_005` | Go wire types and TypeScript wire types stay in sync. | `internal/types/wire_*.go`, `src/types/wire/wire-*.ts` | `make check-wire-drift` |
| `ARCH_INV_006` | Extension/server sync protocol changes are coordinated in one change set. | `internal/capture/sync.go`, `internal/capture/handlers.go`, `src/background/sync/sync-client.ts` | `go test ./internal/capture/...` and extension test target |
| `ARCH_INV_007` | Sensitive data handling preserves redaction guarantees in logs and surfaced outputs. | `internal/redaction/`, `cmd/browser-agent/tools_observe.go`, export paths | `go test ./internal/redaction/...` plus relevant tool tests |
| `ARCH_INV_008` | Bridge, daemon, and CLI modes remain operable; bridge respawn behavior is preserved. | `cmd/browser-agent/bridge.go`, `cmd/browser-agent/main_connection_mcp.go`, `cmd/browser-agent/cli.go` | `go test ./cmd/browser-agent/...` |
| `ARCH_INV_009` | Local-first operation remains default, with no unplanned external data egress. | Network/export code paths, docs for any new external endpoint | Feature-level review plus targeted tests for new egress path |

## P0 Path Rule

Changes touching any `P0` path in [module-map.md](module-map.md) must include:

1. Completed [SAFE_EDIT_CHECKLIST.md](SAFE_EDIT_CHECKLIST.md) in the PR body.
2. Explicit rollback plan in the PR body.
3. The minimum checks listed for every affected invariant.

## Change Process for Invariants

1. Propose change with an ADR in `docs/architecture/`.
2. Update this file and linked docs in the same PR.
3. Add/adjust tests that prove new invariant behavior.
4. Include migration + rollback notes in PR description.
