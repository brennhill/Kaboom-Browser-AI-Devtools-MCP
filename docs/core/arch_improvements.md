---
doc_type: architecture-roadmap
status: proposed
scope: core/architecture-improvements
ai-priority: high
tags: [core, architecture, roadmap, extensibility, cleanup]
relates-to:
  [
    tech-spec.md,
    server-architecture.md,
    extension-architecture.md,
    architecture_repair.md,
    mcp-correctness.md,
  ]
last-verified: 2026-02-19
canonical: false
---

# Architecture Improvements Roadmap

## Why this document exists

Kaboom is shipping quickly and already has strong core architecture patterns (zero-dependency Go server, async queue model, extension command pipeline, drift checks for wire types). The next step is to make the system easier to evolve without regressions as tool surface area grows.

This roadmap defines concrete architecture improvements with:

- clear outcomes and success criteria
- proposed target structures and interfaces
- migration sequencing to avoid breakage
- explicit compatibility constraints
- CI guardrails to prevent backsliding

This is intentionally implementation-oriented and file-path specific.

## Current strengths

1. Clear runtime split between server and extension.
2. Existing contract drift checks (`make check-wire-drift`).
3. Incremental extraction already started (`src/background/dom/dom-dispatch.ts`, `src/background/dom/dom-types.ts`, `src/background/dom/primitives/dom-frame-probe.ts`).
4. Existing invariants framework (`scripts/check-sync-invariants.sh`).
5. Good extension and server architecture docs in place.

## Current architecture pain points

1. Tool contracts are still split across schema files, handler code, runtime enums, and docs.
2. Cross-layer boundaries are mostly conventional, not enforced.
3. Async command lifecycle semantics exist but are not centralized as a single typed model used by all tools.
4. Injected DOM code is now generated, but the same generation model is not yet systematized for other self-contained executeScript payloads.
5. Go tool composition still has substantial centralized logic in `cmd/browser-agent/` and can become harder to extend safely.
6. Integration tests are strong in places but not yet fully contract-driven across Go + TS boundaries.

## North-star outcomes

1. Add a new tool action in one place and generate most of the rest.
2. Layering violations fail CI quickly.
3. Every async result follows one canonical envelope and state machine.
4. Generated files are always reproducible and checked.
5. High-risk behavior changes require contract tests and integration tests before merge.

## Design principles

1. Compatibility first: no breaking MCP wire changes without explicit versioning.
2. Single source of truth for schema/contract decisions.
3. Codegen where duplication creates drift risk.
4. Explicit boundaries over tribal knowledge.
5. Prefer additive migration and shims over large rewrites.
6. Enforce architecture via automated checks, not review discipline alone.

## Constraints to preserve

1. MV3 extension context isolation (background/content/inject) is mandatory.
2. ExecuteScript functions must remain self-contained.
3. No new server runtime dependencies unless justified and approved.
4. Existing tool names and common parameter shape must remain backward-compatible by default.

## Improvement Track A: Contracts as Code

### Goal

Define one canonical tool contract model and generate downstream artifacts.

### Target

Introduce a canonical contract source file (or package) that can generate:

1. Go schema declarations (`tools_*_schema.go` outputs).
2. Go enum/action registry helpers used by handlers.
3. TS action/option types used by extension dispatch.
4. Documentation snippets/tables for user-visible tool options.
5. Contract parity tests.

### Proposed structure

1. Canonical contract source:
   - `internal/contracts/tools_contract.yaml` (or JSON/TOML)
2. Generator:
   - `scripts/generate-tool-contracts.js`
3. Generated outputs:
   - `cmd/browser-agent/generated_tools_schema.go`
   - `cmd/browser-agent/generated_tools_actions.go`
   - `src/types/generated/tool-contracts.ts`
   - `docs/core/generated/tool-contract-matrix.md`

### Incremental migration plan

1. Start with one tool (`interact`) as pilot.
2. Generate schema enums only, leave handlers manual.
3. Add parity tests that compare generated actions vs dispatch maps.
4. Expand to other tools once parity is stable.

### Success criteria

1. Contract drift tests fail if any enum/action is unsupported.
2. Adding a new action requires one contract change and one handler implementation.
3. Docs matrix is generated, not manually edited.

## Improvement Track B: Extension Layering Enforcement

### Goal

Convert implicit layering into explicit, machine-enforced rules.

### Current direction (already started)

1. `dom-dispatch` extracted from `dom-primitives`.
2. `dom-types` extracted for shared contracts.
3. `dom-frame-probe` extracted as an isolated self-contained injection function.

### Target layer model

1. `src/background/platform/`
   - `chrome.*` wrappers, storage/runtime adapters, tabs/scripting adapters
2. `src/background/orchestration/`
   - query lifecycle, tool coordination, retries, sync scheduling
3. `src/background/executors/`
   - dom/upload/recording/analyze execution modules
4. `src/background/contracts/`
   - shared result and command types

### Allowed dependency rules (example)

1. `platform` imports none of orchestration/executors.
2. `orchestration` may import `platform` + `contracts`.
3. `executors` may import `platform` + `contracts` only.
4. `contracts` imports no runtime modules.

### Enforcement mechanism

1. Add script:
   - `scripts/check-layer-boundaries.sh`
2. Rule source:
   - `docs/core/architecture/layer-rules.json`
3. CI integration:
   - include in `check-invariants` and GitHub architecture workflow.

### Success criteria

1. Violating imports fail local and CI checks.
2. New modules include declared layer placement.
3. Refactors do not rely on hidden cross-layer state.

## Improvement Track C: Unified Async Command State Machine

### Goal

Make async lifecycle semantics explicit and shared across Go and TS.

### Canonical lifecycle

One state machine shared across Go + TS:

1. `queued`
2. `running`
3. terminal:
   - `complete`
   - `error`
   - `timeout`
   - `cancelled`

Allowed transitions:

1. `queued -> running`
2. `queued -> cancelled` (cancel before start)
3. `running -> complete|error|timeout|cancelled`

### Canonical result envelope

Define a standard payload model used by `analyze`, `interact`, and future async actions:

1. `correlation_id`
2. `status`
3. `result` (tool-specific)
4. `error` (typed)
5. `timing` (queued_at, started_at, completed_at, duration_ms)
6. `effective_context`:
   - `effective_tab_id`
   - `effective_url`
   - `effective_title`
7. `diagnostics` (optional)

### Implementation targets

1. Go shared types:
   - `internal/types/async_result.go`
2. TS shared types:
   - `src/types/async-result.ts`
3. Mapping layer in handlers:
   - consolidate result wrapping in `tools_core.go` helpers.

### Compatibility strategy

1. Add fields; do not remove old fields initially.
2. Keep old consumers working with shim mapping.
3. Map legacy terminal aliases (for example `expired`) into canonical terminal states before response serialization.
4. Gate removal of legacy fields/states on version marker and release notes.

### Success criteria

1. All async tool responses include the standard envelope.
2. Consumers can rely on stable `status` semantics.
3. Timeout behavior is consistent between server and extension.

## Improvement Track D: Codegen for Self-contained Injection Payloads

### Goal

Eliminate manual duplication/drift in injected executeScript helpers.

### Current status

1. `src/background/dom/primitives/dom-primitives.ts` is generated.
2. Template source exists: `scripts/templates/dom-primitives.ts.tpl`.
3. Generator exists with `--check`: `scripts/build/generate-dom-primitives.js`.
4. Invariant check added in `scripts/check-sync-invariants.sh`.

### Next step

Generalize this pattern for other injected self-contained functions:

1. analysis probes
2. upload helpers
3. future specialized executeScript payloads

### Proposed framework

1. Template directory convention:
   - `scripts/templates/injected/*.tpl`
2. Unified generator:
   - `scripts/generate-injected-code.js`
3. Manifest-driven generation:
   - input list of template -> output mappings
4. Generated file headers include source template and generator path.

### Success criteria

1. No duplicate selector engines maintained manually in multiple files.
2. Every generated file has deterministic output and `--check` mode.
3. Reviewers can audit source templates rather than generated blobs.

## Improvement Track E: Go Tool Module Plugin Pattern

### Goal

Reduce central dispatch complexity and make tool extension safer.

### Target interface

Introduce a lightweight module contract for each tool domain:

1. `Validate(input)`:
   - normalize and reject invalid combinations early
2. `Execute(ctx, input)`:
   - run tool behavior with no registry coupling
3. `Describe()`:
   - return tool metadata, action matrix, and contract pointers used by docs/schema plumbing
4. `Examples()`:
   - return curated examples for docs/help tooling and test fixtures

Optional compatibility methods during migration:

1. `Name() string` (if needed by old registry code)
2. `Schema() MCPTool` (until schema generation fully owns this path)

### Proposed packaging

1. `internal/tools/<tool>/module.go`
2. `internal/tools/<tool>/validation.go`, `execute.go`, `describe.go`, `examples.go`
3. `cmd/browser-agent/internal/toolmodule/registry.go` for canonical module registration and dispatch
4. Keep `ToolHandler` as integration root during migration.

### Registry policy

1. Central registry only wires modules and shared infra (queue, bridge, telemetry, clocks).
2. No tool-specific branching in core handlers beyond module lookup + common envelope handling.
3. Tool behavior lives behind module boundaries, including action-specific validation.

### Migration plan

1. Add module interface + adapter without changing behavior.
2. Pilot with one medium-coupling tool (`configure` or `observe`) to validate ergonomics.
3. Move high-churn tools (`interact`, `analyze`) after adapter and tests are stable.
4. Keep compatibility layer in `tools_core.go` until migration complete.

### Success criteria

1. New tool actions added without modifying large central switches/maps.
2. Per-tool unit tests run without full server setup.
3. Central `cmd/browser-agent/` files trend downward in complexity.

## Improvement Track F: Contract-driven Testing

### Goal

Validate behavior at boundaries, not just units.

### Test layers

1. Unit tests
   - parser/validator/state transitions
2. Contract tests
   - schema enum values have handlers
   - required fields and response envelopes present
   - module `Describe()` and `Examples()` outputs are schema-valid
3. Cross-runtime integration tests
   - Go handler -> queued command -> extension executor -> result normalization
   - same fixture runs through:
     - Go handler
     - TS executor
     - expected MCP response shape assertions
4. Regression tests for high-risk interactions
   - `wait_for` semantics
   - frame targeting (including iframe and shadow edge cases)
   - async completion and timeout/cancel paths
   - upload flow and form submit escalation
   - recording start/stop/playback lifecycle

### Concrete additions

1. Extend `cmd/browser-agent/tools_schema_parity_test.go` style across all tools.
2. Add fixture-based boundary tests under `tests/integration/contracts/`:
   - each fixture includes request, extension mock output, and expected MCP envelope
3. Add shared lifecycle transition fixtures and expected envelopes:
   - `queued -> running -> complete`
   - `queued -> running -> timeout`
   - `queued -> running -> error`
   - `queued -> cancelled`

### Success criteria

1. Breaking schema/dispatch mismatches fail fast in CI.
2. Async lifecycle regressions are captured by dedicated tests.
3. Test failures point to specific contract diffs.

## Improvement Track G: Architecture Guardrails in CI

### Goal

Make architectural quality non-optional.

### Guardrails

1. Contract drift checks.
2. Generated-file drift checks.
3. Layer boundary checks.
4. File-size guard with explicit justified exceptions.
5. Invariants checks for known regressions.

### CI shape

1. Fast checks on every PR:
   - schema/codegen/layer/invariants
2. Heavier integration suite on merge queue/nightly.
3. Required status checks for protected branches.

### Success criteria

1. Architecture regressions blocked before merge.
2. Review load shifts from mechanical checks to design choices.

## Sequencing Plan (Suggested)

## Phase 0 (already in progress)

1. DOM primitive generation and invariants.
2. Shared DOM contract extraction.
3. Frame probe extraction.

## Phase 1 (1-2 weeks)

1. Implement canonical lifecycle enum + transition validator in Go and TS.
2. Route async responses through one shared envelope helper.
3. Add lifecycle contract tests for `interact` + `analyze`.

Exit criteria:

1. `queued -> running -> complete|error|timeout|cancelled` is enforced in both runtimes.
2. Lifecycle fixtures fail on transition drift.

## Phase 2 (2-4 weeks)

1. Introduce plugin-style module interface (`Validate`, `Execute`, `Describe`, `Examples`).
2. Add module registry assembly with no tool-specific logic in core handlers.
3. Pilot migration for one medium-coupling tool.

Exit criteria:

1. One tool runs fully through module interface.
2. Core handler path is registry-only for the migrated tool.

## Phase 3 (3-6 weeks)

1. Expand module migration to high-risk paths (`interact`, upload, recording).
2. Add boundary fixture harness for Go handler + TS executor + MCP response shape.
3. Add layer boundary checker and enforce rules.

Exit criteria:

1. High-risk paths have fixture coverage on boundary contracts.
2. Layer violations fail CI.

## Phase 4 (ongoing)

1. Complete tool module migration.
2. Introduce/expand contract generation where duplication still causes drift.
3. Remove compatibility shims and deprecated fields.
4. Continue shrinking high-complexity files.

Exit criteria:

1. Central orchestration files no longer absorb tool-specific logic.
2. Architecture checks are stable and low-noise.

## Risk register and mitigations

1. Risk: codegen churn causes noisy diffs.
   - Mitigation: deterministic sort/order, stable formatting, generated header metadata.
2. Risk: migration breaks extension runtime behavior.
   - Mitigation: phased rollout, parity tests, keep wrapper shims until validated.
3. Risk: over-abstracting too early.
   - Mitigation: pilot one tool first, expand only on proven reduction in toil.
4. Risk: CI friction from over-strict guardrails.
   - Mitigation: start in warning mode, then ratchet to required checks.

## Metrics to track

1. Number of schema/handler drift incidents per release.
2. Number of generated vs hand-maintained contract files.
3. Mean time to add a new action end-to-end.
4. Count of layer-boundary violations caught pre-merge.
5. Test pass rate for contract/integration suites.
6. File complexity/size trends in top 10 largest files.

## Immediate next actions

1. Land canonical lifecycle types and transition checks in both Go + TS (`queued -> running -> complete|error|timeout|cancelled`).
2. Add `ToolModule` interface and registry-only wiring path (`Validate`, `Execute`, `Describe`, `Examples`) behind compatibility adapters.
3. Add boundary fixtures for high-risk paths first:
   - `wait_for`
   - frame targeting
   - async completion/timeout/cancel
   - upload
   - recording
4. Add a dedicated architecture CI job combining:
   - `check-sync-invariants.sh`
   - lifecycle contract tests
   - contract drift checks
   - layer boundary checks
5. Document contribution rules for generated files and module boundaries in `CONTRIBUTING.md`.

## Definition of done for this roadmap

This roadmap is complete when:

1. contract drift is structurally impossible without failing checks
2. tool behavior changes are validated by contract and integration tests
3. layer boundaries are enforced automatically
4. major high-risk duplication areas are generated or modularized
5. adding features is measurably faster with lower regression risk
