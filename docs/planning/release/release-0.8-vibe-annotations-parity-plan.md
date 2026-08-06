# Release 0.8 — Vibe Annotations Parity-Plus Plan

Status: draft  
Owner: core maintainers  
Last updated: 2026-03-05

## Objective

Meet or exceed Vibe Annotations on annotation UX, data quality, MCP ergonomics, reliability, and testability, while preserving Kaboom's broader platform strengths.

## Evidence Baseline

This plan is based on:
- `tmp-vibe/vibe-annotations/README.md`
- `tmp-vibe/vibe-annotations/extension/content/content.js`
- `tmp-vibe/vibe-annotations/extension/content/modules/inspection-mode.js`
- `tmp-vibe/vibe-annotations/extension/content/modules/annotation-popover.js`
- `tmp-vibe/vibe-annotations/extension/content/modules/element-context.js`
- `tmp-vibe/vibe-annotations/annotations-server/lib/server.js`
- `docs/features/feature/annotated-screenshots/index.md`
- `docs/architecture/flow-maps/analyze-annotations-waiter-and-flush.md`
- `extension/content/draw-mode.js`
- `cmd/browser-agent/internal/toolanalyze/annotationanalysis/handler.go`

## Target Definition (Parity-Plus)

Kaboom is considered "better than Vibe" when all conditions are true:

1. Annotation flow success rate in smoke tests is >= 99% across React, Vue, Svelte, and Next fixtures (3 repeated runs each).
2. MCP annotation workflows are deterministic and bounded (no stuck waiters, no stale IDs after clear/reset, explicit terminal reasons).
3. LLM-facing annotation outputs are richer than Vibe (detail enrichment, project scoping, issue generation, and test generation).
4. Startup/connect behavior under contention converges in <= 2s for at least one daemon, with client retry paths that hide transient startup races from LLMs.
5. Annotation docs and flow maps are complete and cross-linked under `docs/features` and `docs/architecture/flow-maps`.

## Gap Map (Vibe -> Kaboom Actions)

### 1) Annotation UX polish and speed
- Improve quick-start in-page workflow:
  - Keep one-click "Annotate" and keyboard shortcut reliability.
  - Add explicit visual mode state, cancel/save affordances, and low-friction edit/delete.
- Add fast "annotation inbox" review path with clear status and "go to element" actions.
- Ensure overlay/input never hijacks unrelated interactions.

### 2) Annotation context quality
- Preserve and extend context payload:
  - robust selector candidates
  - computed styles
  - parent/sibling context
  - framework hints
  - viewport + rect fidelity
- Guarantee stable serialization contracts and cross-version compatibility.

### 3) MCP ergonomics for coding agents
- Keep current `analyze(annotations|annotation_detail)` and `generate(annotation_*)`.
- Add/verify high-signal helper flows:
  - project-scoped reads by URL/pattern
  - deterministic bulk resolution workflow
  - explicit guidance metadata for ambiguous project scope.
- Ensure no stdout/stderr protocol noise in tool paths.

### 4) Reliability under load/contention
- Enforce bounded retries and backoff for daemon startup and extension bridge connect.
- Remove sleep-based races in composable interact side effects.
- Eliminate global lock bottlenecks in upload/parallel action infrastructure that affect annotation-adjacent flows.

### 5) Test coverage and anti-flake hardening
- Expand deterministic regression suite around known annotation bugs.
- Add framework fixture matrix (React, Vue, Svelte, Next) for selector churn/hydration changes.
- Run each annotation scenario 3x in smoke for resilience, not single-pass luck.

## Workstreams and Order (TDD Required)

## WS0 — Benchmark + Acceptance Harness (first)

1. Define parity test scenarios from Vibe core flows:
   - create/edit/delete annotation
   - multi-page accumulation
   - project-scoped retrieval
   - screenshot/detail retrieval
   - bulk resolution cleanup
2. Implement red tests in:
   - extension unit tests
   - Go tool handler tests
   - smoke tests (`scripts/smoke-tests`, framework fixtures)
3. Freeze acceptance dashboard:
   - pass rate
   - p95 latency
   - startup convergence time

Exit: clear red baseline and measurable targets in CI output.

## WS1 — Correctness + Trust

1. Resolve annotation data correctness defects first (stale IDs, session clear semantics, cross-project leakage).
2. Tighten schema and response contracts:
   - explicit status/terminal_reason
   - stable field names
   - correlation-safe detail lookup
3. Add regression tests for each bug fixed.

Exit: no known data-integrity failures; all bug fixes protected by tests.

## WS2 — UX + Workflow Throughput

1. Refactor draw/inspection UI into smaller modules (state, rendering, event wiring, persistence).
2. Add consistent keyboard model and visible mode state.
3. Add high-signal popup/inbox workflow:
   - list, focus, filter, resolve
   - actionable empty/offline states
4. Keep local-only safety and privacy defaults explicit.

Exit: fewer clicks than current path for common annotation-to-fix workflow.

## WS3 — MCP/Agent Experience Advantage

1. Ensure annotation tools produce LLM-optimized payloads with compact+expanded modes.
2. Improve generated artifacts:
   - `annotation_report` for humans
   - `annotation_issues` for trackers
   - `visual_test` for regression automation
3. Add smoke checks for push/notification-assisted annotation workflows where applicable.

Exit: one-pass "read -> implement -> verify -> resolve" workflow succeeds without manual repair.

## WS4 — Reliability + Performance

1. Remove fixed sleeps in annotation-related orchestration and composable actions.
2. Enforce timeout/retry/backoff policy constants and document them.
3. Add contention stress tests:
   - multi-client daemon connect
   - concurrent command execution
4. Gate on latency budgets:
   - startup convergence
   - annotation retrieval p95
   - detail retrieval p95

Exit: resilient under contention; no flaky startup/tool-call behavior.

## WS5 — Docs, Architecture Hygiene, and Release Gates

1. For every change, update:
   - canonical flow map in `docs/architecture/flow-maps/`
   - feature flow pointer in `docs/features/feature/annotated-screenshots/flow-map.md`
   - feature index metadata (`last_reviewed`, code/test paths)
2. Keep bidirectional links between feature docs and canonical maps.
3. Add release gate checks:
   - docs link integrity
   - required-doc presence
   - annotation smoke matrix pass

Exit: docs are current and navigable for both humans and LLMs.

## Backlog Mapping (Immediate Priority)

Prioritize in this order:
1. Data/trust correctness bugs (stale IDs, invalidation, scope leakage, fragile extraction).
2. Selector + SPA resilience defects.
3. Connection/startup stability defects.
4. Modular decomposition (tool handler/capture decomposition without behavior drift).
5. UX polish and notification flows.

## Architecture Direction

1. Extract annotation domain package boundaries:
   - capture
   - storage/session lifecycle
   - retrieval/query shaping
   - generation/reporting
2. Keep thin entrypoints in tool handlers; push logic into testable services.
3. Avoid shared mutable globals across tabs/sessions; prefer scoped registries.
4. Standardize structured logging and correlation IDs across extension/daemon boundaries.

## Milestones

1. M1: Parity harness + failing tests committed.
2. M2: Data correctness and selector resilience complete.
3. M3: UX workflow and MCP payload improvements complete.
4. M4: Contention reliability and latency budgets met.
5. M5: Docs/flow maps complete + release gate green.

## Risks and Mitigations

1. Risk: framework-specific DOM churn breaks selectors.
   - Mitigation: framework fixture matrix + repeated-run smoke.
2. Risk: startup races produce transient MCP failures.
   - Mitigation: single-winner convergence + client retry shielding + stress tests.
3. Risk: refactors introduce hidden behavior drift.
   - Mitigation: strict TDD, golden response tests, and before/after smoke snapshots.

