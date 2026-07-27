---
doc_type: tech-spec
status: active
scope: core/mcp-contract
ai-priority: high
tags: [core, mcp, tech-spec, architecture, canonical]
relates-to: [product-spec.md, qa-spec.md, mcp-command-option-matrix.md]
last-verified: 2026-02-17
canonical: true
---

# Core MCP Technical Spec (TARGET)

## Architecture
1. MCP client sends JSON-RPC to `POST /mcp`.
2. `MCPHandler` parses and validates the request.
3. `ToolHandler` dispatches to tool-specific handlers.
4. Passive reads resolve from server/capture buffers.
5. Active commands queue extension work and then:
- return queued handle (background mode), or
- block for completion (sync-by-default), or
- require polling via `observe(command_result)`.

Primary components:
- Transport + MCP routing: `cmd/browser-agent/handler.go`
- Tool dispatch: `cmd/browser-agent/tools_core.go`
- Tool schemas: `internal/schema/schema.go`
- Queue/command lifecycle: `internal/capture/queries.go`
- Unified extension sync channel: `internal/capture/sync.go`
- Extension sync client: `src/background/sync/sync-client.ts`
- Extension command executor: `src/background/pending-queries.ts`

## End-to-End Data Flows

### 1. Passive read flow (`observe`)
1. Client calls `tools/call` with `name:"observe"`.
2. `toolObserve` dispatches by `what` in `cmd/browser-agent/tools_observe.go`.
3. Handler returns JSON payload from server buffers and metadata.

### 2. Active command flow (`analyze`/`interact`)
1. Tool handler creates pending query with correlation ID via `CreatePendingQueryWithTimeout`.
2. Query is stored in `QueryDispatcher`; command is registered as `pending`.
3. Extension polls `POST /sync` and receives queued commands.
4. Extension executes command (`src/background/pending-queries.ts`).
5. Extension posts command result on next `/sync` cycle (`command_results`).
6. Server maps result to `correlation_id` and calls `CompleteCommand`.
7. Tool either:
- returns inline completion (sync wait path), or
- returns queued/still_processing and caller fetches with `observe(command_result)`.

### 3. Telemetry ingestion flow
- Extension sends telemetry via `/sync` (`extension_logs`, settings, command results).
- Dedicated POST endpoints still support bulk telemetry (`/network-bodies`, `/enhanced-actions`, etc.).
- Observe modes read unified in-memory capture buffers.

## Command Lifecycle Semantics
- Queue cap: `maxPendingQueries=5` (oldest is dropped on overflow).
- Command statuses: `pending`, `complete`, `error`, `expired`, `timeout`.
- Completion retrieval: `GetCommandResult` / `WaitForCommand`.
- Expiration and disconnect handling:
- stale queries expire
- on extension disconnect, pending commands are marked `expired` with reason `extension_disconnected`.

## Sync-by-Default Behavior
`maybeWaitForCommand` in `cmd/browser-agent/tools_core.go` governs command waiting.
- Default wait budget: 15s.
- If still pending after wait: returns `status:"still_processing"` + `correlation_id`.
- If `background=true` or `sync=false`/`wait=false`: returns queued response immediately.

## Resource and Capability Surfaces
- MCP resources are served from `handler.go` through `resources/list/read`.
- Tool schemas and descriptions are server-owned in `internal/schema/`.

## Known Compatibility Surface
- `/sync` is the primary bidirectional extension channel.
- `/query-result` remains available as compatibility endpoint.

## Canonical Code Paths
- MCP route + methods: `cmd/browser-agent/handler.go`
- Tool dispatch switch: `cmd/browser-agent/tools_core.go`
- Observe dispatch map: `cmd/browser-agent/tools_observe.go`
- Analyze dispatch map: `cmd/browser-agent/tools_analyze.go`
- Configure dispatch map: `cmd/browser-agent/tools_configure.go`
- Interact dispatch map: `cmd/browser-agent/tools_interact.go`
- Generate dispatch map: `cmd/browser-agent/tools_generate.go`
- Query dispatcher and command tracker: `internal/capture/queries.go`
- Extension sync endpoint: `internal/capture/sync.go`
- Extension command execution: `src/background/pending-queries.ts`
