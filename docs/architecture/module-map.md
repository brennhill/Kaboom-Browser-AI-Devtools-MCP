---
doc_type: architecture_module_map
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Module Map

Use this as the first-pass ownership map for safe edits.
Current ownership is based on commit history and can be split into teams later.

| Module Path | Primary Owner | Risk Tier | Owns | High-Risk Change Examples | Baseline Verification |
| --- | --- | --- | --- | --- | --- |
| `cmd/browser-agent/` | Brenn | P0 | MCP entrypoints, bridge/daemon runtime, tool dispatch | JSON-RPC shape drift, daemon lifecycle regressions | `go test ./cmd/browser-agent/...` |
| `internal/capture/` | Brenn | P0 | Extension state, buffers, async command queue | Broken queue/poll flow, stale state reads | `go test ./internal/capture/...` |
| `internal/tools/` | Brenn | P1 | Tool-specific read/analysis/generation logic | Incorrect tool behavior or output schema | `go test ./internal/tools/...` |
| `internal/types/` + `src/types/` | Brenn | P0 | Go/TS wire type contracts | Request/response incompatibility between extension and server | `make check-wire-drift` |
| `src/background/` | Brenn | P0 | Extension control plane and sync client | Lost command execution or telemetry ingest | `npm test` or extension test target |
| `src/content/` | Brenn | P1 | In-page capture and action execution hooks | DOM/network/websocket capture regressions | `npm test` or extension test target |
| `src/popup/` | Brenn | P2 | User-facing extension controls | Incorrect pilot state/toggle behavior | `npm test` or extension test target |
| `server.go` + `internal/capture/*` | Brenn | P0 | HTTP endpoint registration and payload handling | Endpoint mismatch, protocol breakage | `go test ./cmd/browser-agent/... ./internal/capture/...` |
| `docs/architecture/` | Brenn | P1 | Architecture contract and invariants | Missing/incorrect constraints for future edits | Documentation review in PR |

## Owner Notes

- If a change touches any `P0` module, include a completed [SAFE_EDIT_CHECKLIST.md](SAFE_EDIT_CHECKLIST.md) in the PR.
- If a change touches 3+ high-risk modules, require explicit scope and rollback in the edit request.
