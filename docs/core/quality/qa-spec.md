---
doc_type: qa-plan
status: active
scope: core/mcp-contract/qa
ai-priority: high
tags: [core, qa, mcp, contract, regression]
relates-to: [product-spec.md, tech-spec.md, mcp-command-option-matrix.md]
last-verified: 2026-02-17
canonical: true
---

# Core MCP QA Spec (TARGET)

## Goal
Ensure every MCP tool command and documented option is valid end-to-end: schema -> server dispatch -> extension execution (where required) -> observable result.

## Test Gates
1. Schema parity gate
- Verify every enum in `tools_schema.go` has a matching dispatch handler.
- Verify every required key (`what`, `action`, `format`) enforces missing-param errors.

2. End-to-end command gate
- For extension-backed commands, verify queue + `/sync` + completion + `observe(command_result)`.
- For server-side commands, verify direct response path and error behavior.

3. Option handling gate
- Verify filter and pagination options alter returned data as documented.
- Verify async control flags (`sync`, `wait`, `background`) produce expected status transitions.

4. Safety gate
- Verify rate limit enforcement.
- Verify redaction on tool output.
- Verify extension-disconnect behavior for queued commands.

## Required Automated Coverage
- MCP handler and tool call routing:
- `cmd/browser-agent/handler_consistency_test.go`
- Observe coverage and mode handlers:
- `cmd/browser-agent/internal/toolobserve/dispatcher_commands_test.go`
- `internal/tools/observe/logs/logs_edge_test.go`
- `internal/tools/observe/network/network_test.go`
- `internal/tools/observe/page/page_readiness_test.go`
- `internal/tools/observe/session/session_test.go`
- `internal/tools/observe/timeline/correlation_test.go`
- Interact async and command result behavior:
- `cmd/browser-agent/tools_interact_rich_test.go`
- Analyze command paths:
- `cmd/browser-agent/tools_analyze_*test.go`
- Sync endpoint + queue lifecycle:
- `internal/capture/syncruntime/sync_test.go`
- `internal/capture/async_queue_integration_test.go`
- `internal/capture/correlation_tracking_test.go`

## Manual UAT Checklist (Contract-level)
1. Run `configure(what:"health")` and confirm extension/tracking status fields.
2. Run one passive `observe` mode and confirm metadata + cursor fields.
3. Run one extension-backed analyze command with `background:true`, then poll `observe(command_result)`.
4. Run one sync-by-default interact action and verify inline completion.
5. Force extension disconnect and verify queued command expiration behavior.
6. Verify `interact({what:"screenshot"})` is rejected and
   `observe({what:"screenshot"})` remains the sole screenshot API.
7. Verify `analyze(what:"dom")` path for DOM queries.

## Exit Criteria
- No schema/dispatch mismatches.
- No missing command-result retrieval path for extension-backed commands.
- No documented option that is silently ignored without explicit warning or pass-through semantics.
- Canonical docs updated when schema changes.
