# Common Patterns (Required)

This file defines the default implementation patterns for extension and MCP changes.
Use this as a hard checklist during design, coding, and review.

## 0) 0.8 Helper Inventory (Use These First)

- Server dispatch and mode resolution:
  - `cmd/browser-agent/tool_dispatch_helpers.go`
  - `cmd/browser-agent/tools_observe.go`
  - `cmd/browser-agent/tools_configure.go`
- Shared async query path:
  - `cmd/browser-agent/tools_async_completion.go`
- Interact response shaping:
  - `cmd/browser-agent/tools_interact_dispatch.go`
  - `cmd/browser-agent/internal/toolinteract/interact_evidence.go`
- Recording helper seams:
  - `cmd/browser-agent/internal/toolrecording/helpers.go`
- Extension command routing:
  - `src/background/commands/registry.ts`
  - `src/background/commands/helpers.ts`
- Frame-target handling:
  - `src/background/exec/frame-targeting.ts`
- Shared test helpers:
  - `internal/pagination/test_helpers_test.go`
  - `internal/capture/sync_test_helpers_test.go`

## 1) Shared State Access

- Use feature helpers/modules for shared keys instead of new inline `chrome.storage.local` logic.
- For tab tracking, route through tab-state helpers and keep key usage centralized.
- For recording/pending-intent state, keep reads/writes in recording modules and avoid copy/paste storage flows in unrelated files.

## 2) Multi-Entry-Point Actions

- If behavior is reachable from keyboard, context menu, popup, and MCP, implement one shared toggle/start-stop helper.
- Entry points should only do minimal input mapping and call the shared helper.
- Do not duplicate stop/start branching logic per entry point.

## 3) Cross-Context Message Contracts

- Define message contracts in `src/types/runtime-messages.ts` first.
- Keep names, payload shape, and response semantics consistent across popup/background/content/offscreen.
- If a message crosses Go/TS boundary, update wire/schema definitions in the same change.

## 4) User-Facing Recording UX

- Use shared label/toast/badge helpers so wording and truncation stay consistent.
- Do not hardcode new recording status text in multiple modules.
- When replacing UX mechanisms (example: watermark -> badge), remove old behavior and align tests immediately.

## 5) Duplicate Code Policy

- Run:
  - `npx jscpd src/background src/popup --min-lines 8 --min-tokens 60`
- For each non-trivial clone:
  - Extract to a helper, or
  - Keep intentionally and add a short comment explaining why extraction is worse (performance, isolation, sandbox constraints, etc.).

## 6) Tests for End-to-End Data Passing

- Any cross-context flow change must include:
  - producer-side unit coverage,
  - consumer-side unit coverage,
  - one end-to-end/smoke assertion of payload shape and behavior.
- If behavior changes, update/remove stale tests in the same PR; do not leave failing legacy assertions.

## 7) Tool Dispatch + Registry Pattern

- Top-level tool entrypoints (`toolObserve`, `toolConfigure`, `toolInteract`, `toolAnalyze`, `toolGenerate`) should delegate through `dispatchTool(...)`.
- Mode/action registration belongs in the tool registry files (`tools_*_registry.go`), not ad-hoc `switch` blocks in entrypoint files.
- Alias handling (`action`/`mode` fallback) must stay in shared mode-resolution helpers.

## 8) Pending Query + Async Command Pattern

- Always enqueue extension work via `enqueuePendingQuery(...)`.
- Do not write one-off queueing logic inside individual tool handlers.
- Keep queue saturation and timeout behavior standardized through shared enqueue response paths.

## 9) Frame and Target Normalization Pattern

- Normalize `frame` parameters with `normalizeFrameArg(...)`.
- Resolve explicit frame IDs with `resolveMatchedFrameIds(...)`.
- Keep target tab/context enrichment centralized through command helpers (`resolveTargetTab`, `withTargetContext`).

## 10) Extension Command Routing Pattern

- Register pending-query handlers in `src/background/commands/registry.ts`.
- Reuse `src/background/commands/helpers.ts` for parsing, target resolution, action toasts, and result envelopes.
- Avoid reintroducing monolithic `if/else` router logic in `pending-queries.ts`.

## 11) MCP Stdout Boundary

- Reserve browser-agent stdout exclusively for MCP JSON-RPC payloads emitted through `writeMCPPayload(...)`.
- Route help, stop/doctor status, CLI rendering, warnings, and diagnostics through the canonical `internal/diag` sink.
- Do not write connect-mode responses directly to `os.Stdout`; use the shared MCP framing path.
- Keep `stdout_protocol_boundary_test.go` green when adding new output.

## 11) Response Shaping Pattern

- Keep composable response enrichment (`include_screenshot`, `include_interactive`, content/metadata shaping) in shared response helper files.
- Preserve stable response envelopes and metadata keys; avoid per-handler custom shapes for equivalent outcomes.
- If a schema/output shape changes, update docs examples and smoke assertions in the same change.

## 12) Shared Test Utility Pattern

- Before adding repeated assertions/setup in tests, extend shared helpers first.
- Use pagination and sync helper suites for cursor/transport assertions to avoid drift between modules.
- Contract changes must be reflected in smoke/UAT modules that validate the same behavior.

## 13) Atomic Migration Pattern

- Do not add or retain compatibility facades, pass-through wrappers, alias-only modules, transitional shims, or duplicate old/new entry points.
- A migration must update every production caller, test, schema, and documentation reference to the canonical API and delete the obsolete surface in the same change.
- If an end-to-end migration cannot be completed safely in the current change, leave the existing design intact and plan the complete migration instead.
- A re-export used only to avoid updating callers is a failed migration, not a module boundary.
- Public wire-protocol compatibility is maintained in the canonical implementation; it does not justify a second internal API surface.

## Review Checklist

- [ ] Storage access follows helper/module boundaries.
- [ ] Multi-entry-point behavior uses a shared helper path.
- [ ] Runtime message contract is typed and synchronized.
- [ ] UX labels/toasts/badges come from shared utilities.
- [ ] Tool mode dispatch and alias handling stay in shared registry/dispatch helpers.
- [ ] Async extension work uses shared enqueue helpers (no one-off queue paths).
- [ ] Frame/tab normalization uses shared targeting helpers.
- [ ] Response shape changes are reflected in docs/examples/smoke checks.
- [ ] `jscpd` run completed and clones were resolved or documented.
- [ ] Unit + e2e/smoke tests reflect current behavior and pass.
- [ ] Migrations are complete: no compatibility facade, alias-only module, old caller, stale test, or stale documentation remains.
