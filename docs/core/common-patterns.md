# Common Patterns (Required)

This file defines the default implementation patterns for extension and MCP changes.
Use this as a hard checklist during design, coding, and review.

## 0) 0.8 Helper Inventory (Use These First)

- Server dispatch and mode resolution:
  - `cmd/browser-agent/internal/toolrouting/routing.go`
  - `cmd/browser-agent/internal/toolobserve/dispatcher.go`
  - `cmd/browser-agent/tools_configure.go`
- Shared async query path:
  - `cmd/browser-agent/internal/asynccommand/handler.go`
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

- Top-level tool entrypoints delegate to their canonical dispatcher owners.
  Configure routes through `internal/toolconfigure.Dispatcher`; do not recreate
  package-main mode registries or forwarding methods.
- Mode/action registration belongs in the tool registry files (`tools_*_registry.go`), not ad-hoc `switch` blocks in entrypoint files.
- Tool routing is canonical-only: every top-level tool call uses `what`, and
  registries must not add alternate selector or mode names.
- Action-family owners expose one dispatcher method across package boundaries;
  individual `handle*` implementations remain private. AST contract tests
  should ratchet the allowed exported receiver surface.

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

## 14) Async Failure Evidence

- Unexpected promise rejections and caught exceptions must emit a redacted
  structured log or Doctor diagnostic through the feature owner.
- Never use an empty catch or `.catch(() => undefined)` for an unexpected
  condition.
- An intentionally unlogged expected absence or cancellation must include an
  adjacent `EXPECTED_ABSENCE:` comment explaining both why the condition is
  normal and why emitting a log would be misleading.
- This applies equally to promise rejection handlers and synchronous
  `try/catch` fallbacks. Comment-only promise catches and empty synchronous
  catches are rejected by the architecture suite unless they carry that
  explicit two-part rationale.
- Do not include raw errors when they may contain user state. Prefer stable
  operation names, error categories, correlation IDs, and redacted remediation.

## 15) Canonical Operational Incident Pattern

- Report an operational failure once through `internal/incident`; derive Doctor
  presentation and privacy-safe analytics from that canonical incident.
- Use only registry-owned incident codes. Feature callers provide correlation,
  numeric connection generation, and redacted local evidence; they do not author
  Doctor prose, analytics dimensions, or arbitrary outbound maps.
- Treat recovery as an explicit state machine: `detected → retrying → recovered`
  or `detected → retrying → exhausted`. Transitions must be idempotent and reject
  stale generations.
- Keep detailed evidence local. The analytics projection deliberately excludes
  correlation IDs, generations, paths, URLs, messages, fixes, and local evidence.
- Instrument ownership boundaries rather than helpers: daemon lifecycle,
  extension connection, tracking/readiness, command resolution, state recovery,
  and bounded queues.
- Bound incident history and storage. Capacity eviction is single-pass and every
  dropped incident increments an observable pressure counter.

## 16) Deterministic Lifecycle and Boundary Contracts

- Drive asynchronous lifecycle tests with injected clocks, schedulers,
  transports, storage, and randomness. Tests advance named events; they do not
  sleep and hope that a race occurs.
- Inject command execution, process discovery, and filesystem behavior through
  instance-owned runtimes. Never swap package variables to fake operating-system
  behavior; parallel tests must not share mutable seams.
- Every accepted operation has exactly one terminal outcome. Delivery retries
  must never execute an in-flight command twice, and timeout/cancellation must
  preserve the command ID, correlation ID, and connection generation.
- Every cross-context or cross-process boundary authenticates, validates,
  bounds, correlates, and reports. Boundary payloads have one canonical wire
  owner; handler-local transport types and manually synchronized enums are
  prohibited.
- Register important contract names in `.architecture-boundaries.json`; CI
  rejects declarations outside the configured canonical owner, preventing a
  local interface from silently drifting away from the runtime contract.
- Wire contracts flow in one direction: canonical Go structs generate
  TypeScript and OpenAPI schemas, shared fixtures prove both directions, and CI
  byte-compares downstream generated clients. Optionality overrides and parallel
  hand-maintained allowlists are prohibited.
- Every bounded resource declares its capacity, overflow policy, retry or
  recovery policy, dropped count, and Doctor visibility. Silent eviction and
  recovery that depends on unrelated new activity are prohibited.
- Prefer deterministic operation budgets (visited nodes, serialized bytes,
  transitions, retries, and retained entries) over wall-clock assertions.
  Hardware latency budgets run separately in an isolated CI lane.
- CI workflows invoke canonical Make targets. A workflow contract must fail if
  a required architecture, lifecycle, performance, coverage, or security gate
  is defined locally but omitted from hosted CI.
- `make check-go-architecture` rejects growth in mutable Go package state and
  exported declarations against the reviewed per-file baseline. New files have
  no allowance. After removing debt, run `make go-architecture-baseline-update`;
  it can only lower or delete allowances. Intentional public API growth requires
  a manual baseline edit in the same reviewed change, with its ownership and
  necessity documented in the relevant feature index. Immutable sentinel errors
  and compiled regular expressions are explicitly classified by the checker;
  do not broaden that classification merely to make a mutable registry pass.

## Review Checklist

- [ ] Storage access follows helper/module boundaries.
- [ ] Multi-entry-point behavior uses a shared helper path.
- [ ] Runtime message contract is typed and synchronized.
- [ ] Async behavior is covered by event-driven lifecycle tests with controlled time and transport.
- [ ] Every accepted operation has one correlated terminal outcome and cancellation reaches its owner.
- [ ] Boundary ingress is authenticated, runtime-validated, size-bounded, and locally diagnosable.
- [ ] Every bounded queue exposes pressure, loss, and autonomous recovery behavior.
- [ ] UX labels/toasts/badges come from shared utilities.
- [ ] Tool mode dispatch and alias handling stay in shared registry/dispatch helpers.
- [ ] Async extension work uses shared enqueue helpers (no one-off queue paths).
- [ ] Frame/tab normalization uses shared targeting helpers.
- [ ] Response shape changes are reflected in docs/examples/smoke checks.
- [ ] `jscpd` run completed and clones were resolved or documented.
- [ ] Unit + e2e/smoke tests reflect current behavior and pass.
- [ ] Migrations are complete: no compatibility facade, alias-only module, old caller, stale test, or stale documentation remains.
- [ ] Every catch leaves redacted evidence, or carries an explicit `EXPECTED_ABSENCE:` rationale.
- [ ] Operational failures use a registered incident code and canonical lifecycle projection rather than parallel log/Doctor/analytics calls.
- [ ] Mutable Go state and exported surfaces did not grow, or their reviewed ownership rationale accompanies the baseline change.

### Isolate wall-clock performance gates

Wall-clock SLO assertions must not run inside parallel unit-test shards, where
unrelated CPU contention makes a correct implementation fail nondeterministically.
Keep deterministic correctness, allocation, and race coverage in the normal
lane; run unchanged latency thresholds through `make test-performance`, which
sets both package and test parallelism to one. Do not “fix” load sensitivity by
raising a product SLO.
