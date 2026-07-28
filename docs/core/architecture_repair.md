# Architecture Repair Plan

## Current State

The kaboom codebase has two major architectural pain points: a monolithic Go backend (`cmd/browser-agent/`) and a flat Chrome extension (`src/`). Both suffer from poor modularity, but for different reasons.

---

## Phase 1: Naming Convention Discipline (Current — Option C)

Keep each MCP tool's package-main surface concentrated in its dispatch boundary,
with implementations delegated to focused internal packages:

```
internal/toolobserve/dispatcher.go            # observe routing and thin result adapters
internal/toolanalyze/analyzedispatch/dispatcher.go   # analyze routing
tools_configure.go          # configure routing and thin adapters
tools_interact_dispatch.go  # interact routing and boundary glue
internal/tool*/             # feature implementations
...
```

Shared test helpers are consolidated in `tools_test_helpers_test.go`. The
cross-tool queue-to-result protocol lives in
`internal/asynccommand/handler.go`; the composition root injects its functions
directly into feature owners.

**Status**: In progress. Root adapters are being removed or colocated with their
registry while behavior moves behind narrow internal-package contracts.

---

## Phase 2: Internal Sub-Packages (Next — Option A)

Break `cmd/browser-agent/` from a 207-file `package main` into focused internal packages:

```
cmd/browser-agent/
  main.go                    # CLI entry, wiring
  tools.go                   # ToolHandler + HandleToolCall dispatch

internal/
  tools/
    observe/
      observe.go             # Handler(ctx, args) -> Response
      schema.go              # Schema() -> MCPTool
      analysis.go            # sub-handlers
    analyze/
      analyze.go
      schema.go
      security.go
    generate/
      ...
    configure/
      ...
    interact/
      ...
    shared/
      queries.go             # executeA11yQuery, cross-tool helpers
      types.go               # shared request/response types
```

### Design Challenges (Go)

1. **ToolHandler dependency**: Every tool method is on `*ToolHandler` (199 methods). Sub-packages can't reference `main.ToolHandler`. Need to extract a `ToolContext` interface that sub-packages accept.

2. **Capture god object**: `*capture.Capture` has 149 methods across 22 files and is imported 65 times from `cmd/browser-agent/`. Tools need it for state access. Solution: define narrow interfaces per tool (`ObserveState`, `ConfigureState`) so each package depends only on what it uses.

3. **Server access**: Tools read from `*Server` (log entries, audit). Need to extract a read interface.

4. **Cross-tool dispatch**: Some handlers dispatch to other tools (e.g., observe calls analyze for a11y). These cross-cuts need to go through interfaces, not direct function calls.

5. **Circular dependency risk**: `capture` depends on `performance`, `queries`, `state`; tools depend on `capture`. Decomposing requires inverting these dependencies through interfaces in the `types` package.

### Migration Strategy

1. Start with the simplest tool (generate — fewest dependencies)
2. Extract the `ToolContext` interface from what generate needs
3. Move generate to `internal/tools/generate/`
4. Iterate: observe, analyze, configure, interact (increasing complexity)
5. Each step verified by `make test`

---

## Extension Design Challenges

The Chrome extension (`src/`) has different constraints than the Go backend.

### Platform-Imposed Constraints (Can't Fix)

1. **Execution context isolation**: Chrome MV3 runs code in 3+ isolated contexts (background service worker, content scripts, inject scripts). Each needs its own message handlers. This duplication is required by Chrome's security model.

2. **No dynamic imports in service worker**: Background scripts can't lazy-load. Everything must be statically imported at startup.

3. **Content/inject scripts must be bundled**: MV3 requires single-file bundles for injected scripts. No code splitting possible.

### Self-Inflicted Issues (Can Fix)

1. **Large files**:
   - `pending-queries.ts` (1,299 LOC) — async query queue/sync mechanism
   - `dom-primitives.ts` (1,096 LOC) — DOM manipulation primitives
   - `recording.ts` (730 LOC) — recording lifecycle
   - `event-listeners.ts` (558 LOC) — browser event capture
   - `snapshots.ts` (543 LOC) — page snapshot management

2. **Flat background directory**: 12 files in `src/background/` with no sub-structure. State flows through imports without interfaces.

3. **Message protocol complexity**: `runtime-messages.ts` (513 LOC) defines the typed protocol between all contexts. Changes ripple across background, content, and inject code.

4. **State-module growth**: focused cache owners (`cache-limits.ts`, `error-groups.ts`, `snapshots.ts`, and `debug-log.ts`) can still grow without explicit per-module budgets.

### Extension Repair Strategy

1. **Split large files**: Break `pending-queries.ts` into queue management, response coordination, and timeout handling. Break `dom-primitives.ts` by DOM operation category.

2. **Extract shared message types**: Move message type definitions to `src/shared/messages/` with per-context re-exports.

3. **Introduce feature modules in background**: Group related functionality (recording, snapshots, queries) into sub-directories under `src/background/`.

4. **Keep message handler duplication**: This is required by Chrome. Document it clearly so future contributors don't try to "DRY" it.

---

## Priority Order

1. **Phase 1** (now): Naming conventions + test helper consolidation
2. **Phase 2** (next): Go sub-packages starting with generate
3. **Extension large files**: Split files exceeding 800 LOC
4. **Extension modules**: Group background features into sub-directories
