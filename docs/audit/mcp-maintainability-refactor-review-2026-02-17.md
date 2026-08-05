# MCP Maintainability Refactor Review (Go + TypeScript)

Date: 2026-02-17  
Scope: `cmd/browser-agent`, `internal/*`, `src/*` (Go server + TypeScript extension/background)  
Constraint used during review: read-only analysis, no code edits

## Goal

Define how to refactor the current architecture to preserve behavior while making the system easier to maintain and safer for LLM-assisted changes.

## Executive Summary

The system works, but key contracts are duplicated across multiple layers and rely heavily on stringly-typed routing. This makes edits high-risk and encourages drift between schema, dispatch, and execution.  

The proper design is:

1. One canonical command/contract source.
2. Generated or centrally derived schema/types/validators/registries for Go and TS.
3. Registry-based dispatch with strict per-action parsing.
4. Runtime context injection instead of global mutable state in background orchestration.
5. Single transport path (`/sync`) as canonical.

## Findings (Ordered by Severity)

### 1. Critical: Contract duplication across layers creates drift risk

The same command/mode/action sets are represented separately in multiple places:

- Tool schemas: `cmd/browser-agent/tools_schema.go`
- Server dispatch: `cmd/browser-agent/tools_core.go`, `cmd/browser-agent/tools_observe.go`, `cmd/browser-agent/tools_analyze.go`, `cmd/browser-agent/tools_configure.go`, `cmd/browser-agent/tools_interact.go`
- Sync command types: `internal/capture/syncruntime/handler.go`
- Extension sync types: `src/background/sync/sync-client.ts`
- Extension execution router: `src/background/pending-queries.ts`

Impact:

1. A new action can be added in one place but missed in another.
2. Unknown params/actions are often surfaced late at runtime.
3. LLM edits require touching many files with hidden coupling.

### 2. Critical: Circular imports + global mutable state in background runtime

Examples:

- `src/background/index.ts` imports `pending-queries` and re-exports runtime state.
- `src/background/pending-queries.ts` imports `index` and reads/writes runtime behavior through globals.

Key globals:

- `serverUrl`, `connectionStatus`, `__aiWebPilotEnabledCache`, `extensionLogQueue`, etc. in `src/background/index.ts`.

Impact:

1. Order-dependent behavior and hidden side effects.
2. Difficult unit isolation.
3. LLM edits are fragile because dependencies are implicit.

### 3. High: Oversized orchestration files reduce edit safety

Hotspots:

- `src/background/pending-queries.ts` (~1299 lines)
- `cmd/browser-agent/tools_schema.go` (~727 lines)
- `cmd/browser-agent/tools_core.go` (~405 lines with broad cross-cutting wiring)
- `cmd/browser-agent/handler.go` (core request/response path)

Impact:

1. Mixed responsibilities (parsing, routing, side effects, response shaping).
2. Hard to reason about end-to-end behavior during changes.

### 4. High: Validation strategy is inconsistent and often lenient

Evidence:

- Lenient parser helper: `internal/mcp/response.go` (`LenientUnmarshal`)
- Tool-level unknown-arg warning based on top-level schema: `cmd/browser-agent/handler.go`
- Structured warning parser exists but is limited in use: `cmd/browser-agent/tools_validation.go`
- Many extension params parsed from `unknown`/string JSON at execution time: `src/background/pending-queries.ts`

Impact:

1. Typos and invalid combinations may pass too far before failure.
2. Error quality and behavior can differ by action/tool.

### 5. High: Legacy + canonical transport APIs coexist in exported surfaces

Evidence:

- Canonical path is `/sync`: `internal/capture/syncruntime/handler.go`, `src/background/sync/sync-client.ts`
- Legacy functions were still exported and visible through `src/background.ts`; sync ownership now lives in focused modules under `src/background/sync/`.

Impact:

1. Confuses maintainers and LLMs about the real path.
2. Increases chance of accidental use of deprecated flows.

### 6. Medium: Async status semantics are not strict end-to-end

Evidence:

- Extension sends statuses (`complete|error|timeout`), but many failures are encoded as `complete` with `error` payload.
- Server-side processing of command results largely keys off correlation/result presence, not strict status semantics.
- Relevant files: `src/background/pending-queries.ts`, `src/background/sync/sync-client.ts`, `internal/capture/syncruntime/handler.go`, `internal/capture/queries.go`.

Impact:

1. Harder to reason about success/failure transitions.
2. Polling clients must infer intent from mixed fields.

### 7. Medium: Some TS contract types are stale or weakly enforced

Evidence:

- `src/types/runtime/queries.ts` declares `BrowserActionParams` without newer action values such as `new_tab`.
- `QueryType` and runtime handling are not strongly connected to generated/runtime validators.

Impact:

1. False confidence from types.
2. Runtime behavior can diverge from declared models.

## Accuracy and Data-Flow Assessment

## End-to-end data flow (current)

1. MCP request enters via `/mcp`: `cmd/browser-agent/handler.go`.
2. Tool dispatch occurs in `ToolHandler.HandleToolCall`: `cmd/browser-agent/tools_core.go`.
3. Tool-specific handlers enqueue `queries.PendingQuery`: `cmd/browser-agent/tools_*.go`.
4. Pending queries are surfaced through `/sync`: `internal/capture/syncruntime/handler.go`.
5. Extension `SyncClient` receives commands: `src/background/sync/sync-client.ts`.
6. Extension executes in `handlePendingQuery`: `src/background/pending-queries.ts`.
7. Command results are sent back via `/sync` `command_results`.
8. Server correlates via query ID/correlation ID: `internal/capture/syncruntime/handler.go`, `internal/capture/queries.go`.
9. Tool response is post-processed (redaction/warnings/telemetry): `cmd/browser-agent/handler.go`, `cmd/browser-agent/telemetry_passive.go`.

## Current accuracy posture

- Tool-level mode/action sets are mostly aligned today between schema and dispatch.
- Alignment is convention-based, not compile-time guaranteed.
- Golden tests protect schema snapshots (`cmd/browser-agent/golden_test.go`) but do not fully enforce execution coverage for every option combination.

## Proper Target Design

### 1. Single canonical contract

Maintain one source of truth for:

- Tools (`observe`, `analyze`, `generate`, `configure`, `interact`)
- Modes/actions
- Query types (`dom`, `a11y`, `browser_action`, etc.)
- Per-action parameter schemas
- Per-action response envelopes

Generated artifacts:

1. Go enums + validators + handler registry skeleton.
2. TS discriminated unions + validators + execution registry skeleton.
3. MCP `tools/list` schema output.

### 2. Registry-based architecture (no giant switch chains)

Use explicit registries:

1. Parse phase: strict parser per action/mode.
2. Execute phase: handler implementation per action/mode.
3. Encode phase: standardized response formatting.

Every registry entry should include:

1. Name.
2. Parameter validator/parser.
3. Execution function.
4. Result encoder.
5. Contract test ID.

### 3. Context injection over globals (TS background)

Replace `index` global coupling with runtime context object:

1. `RuntimeContext` passed to handlers.
2. State modules expose read/write APIs, not mutable exports.
3. Remove circular import between `index.ts` and `pending-queries.ts`.

### 4. Strict validation at boundaries

Enforce validation:

1. MCP boundary: validate tool args against tool + mode/action schema.
2. Sync boundary: validate command payload against query-type schema.
3. Extension execution boundary: do not parse unknown JSON ad hoc per branch.

### 5. Canonical transport surface

Keep `/sync` as the single command transport path.  
Move legacy APIs to an explicitly deprecated compatibility module and stop exporting them from default communication facades.

### 6. Standard async result model

Define one status contract:

1. `status=complete` must represent successful execution.
2. `status=error|timeout` must represent failures.
3. Error payload format must be stable.

## Refactor Plan (Behavior-Preserving Sequence)

### Phase 0: Guardrails

1. Add/strengthen contract parity tests for schema vs dispatch vs execution.
2. Add command matrix tests proving all commands/options pass through Go -> `/sync` -> TS -> result.

### Phase 1: Contract extraction

1. Introduce a contract definition package/folder (tool/action/query metadata).
2. Generate schema/types/registries from this contract.
3. Keep existing handlers; wire generated definitions as adapters.

### Phase 2: Extension runtime cleanup

1. Split `pending-queries.ts` by query domain:
   - `query-router.ts`
   - `query-dom.ts`
   - `query-a11y.ts`
   - `query-browser-actions.ts`
   - `query-state.ts`
   - `query-recording.ts`
2. Introduce `RuntimeContext`.
3. Remove `index <-> pending-queries` circular dependency.

### Phase 3: Go tool modularization

1. Keep `ToolHandler` minimal and move mode handlers into smaller modules with strict parsers.
2. Replace map-of-anonymous-functions with typed registry entries.
3. Derive tool schemas from registry metadata.

### Phase 4: Transport/API hardening

1. Restrict default exports to `/sync`-first APIs.
2. Move legacy helper exports behind an explicit compatibility namespace.
3. Enforce strict async result semantics.

### Phase 5: CI enforcement

Block merges when any of the following fail:

1. Contract parity (schema/registry/types).
2. Full command-option matrix coverage.
3. End-to-end command execution for each tool action/mode.

## Proposed Success Criteria

1. Adding a new mode/action requires edits in one contract definition plus one handler file.
2. Schema, Go parser, TS parser, and tests are generated or mechanically linked.
3. No runtime circular imports in background execution path.
4. Unknown parameters fail fast with consistent structured errors.
5. `/sync` is the only default command transport path.

## Practical Next Step

Start with Phase 0 + Phase 1 in a small PR:

1. Contract extraction for one tool (`interact`) only.
2. Generated registry and schema for `interact`.
3. Command matrix tests for `interact` actions.
4. No behavioral changes.
