---
doc_type: qa-plan
feature_id: feature-hook-eval-rig
status: proposed
owners: []
last_reviewed: 2026-07-05
links:
  index: ./index.md
  product: ./product-spec.md
  tech: ./tech-spec.md
  fixtures: ./eval-fixtures.md
relates-to: [product-spec.md, tech-spec.md, eval-fixtures.md]
tags: [testing, qa, hooks, eval]
---

# QA Plan: Hook Eval Rig

> QA plan for the Hook Eval Rig — the deterministic framework that measures whether `kaboom-hooks` actually improves AI coding sessions. The rig itself is test tooling, so this plan focuses on correctness of the runner, determinism, fixture coverage, and the guardrails that keep eval code out of production binaries. Tier 1 (unit evals) is implemented; Tier 2 (integration) and Tier 3 (live metrics) are aspirational and tracked here as forward-looking scenarios.

---

## 1. Data Leak Analysis

**Goal:** The eval rig executes real hook logic against the Kaboom codebase. Verify it neither transmits data off-host nor ships into a production binary.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Eval code linked into shipped binaries | A lint invariant (check 16 in `scripts/lint-hardening.sh`) forbids `internal/hook/eval` from being imported by production binaries. Verify the invariant exists and fails the build if violated. | critical |
| DL-2 | Network access during eval | Tier 1 unit evals run fully offline against fixtures and the local repo. Verify no fixture triggers a network call. | high |
| DL-3 | Temp session dirs leaking | `RunFixture` creates `os.MkdirTemp` session dirs and defers `os.RemoveAll`. Verify no temp dir is left behind after a run. | medium |
| DL-4 | Fixture content exposing secrets | Fixtures contain synthetic code snippets. Verify none embed real credentials or private paths. | medium |
| DL-5 | Repo-root resolution escaping the repo | `REPO_ROOT` resolves via `findRepoRoot` walking up for `go.mod`. Verify it cannot resolve to a directory outside the repository under test. | medium |

### Negative Tests (must NOT leak)

- [ ] Building any production binary fails if it imports `internal/hook/eval`.
- [ ] A full `go test ./internal/hook/eval/` run makes no outbound network connections.
- [ ] Temp session directories are removed after each fixture (no `/tmp/eval-session-*` residue).

---

## 2. LLM Clarity Assessment

**Goal:** The rig's report output is read by engineers and CI, not by an in-session model. Verify the report is unambiguous and the fixture contract is self-documenting.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Per-hook report lines | `FormatReport` prints `hook: passed/total (avg Xms, max Yms)` per hook. Verify counts and latency are correct. | [ ] |
| CL-2 | Pass/fail totals | The trailing `All evals: P/T passed.` matches the aggregate. | [ ] |
| CL-3 | Failure messages | `validate` emits specific failures: `output missing "..."`, `output should not contain "..."`, `~N tokens exceeds budget`, `latency Nms exceeds budget`. | [ ] |
| CL-4 | Aspirational vs regression | Fixtures or descriptions tagged `ASPIRATIONAL` are skipped, not failed, so the report distinguishes capability gaps from regressions. | [ ] |
| CL-5 | Fixture schema documented | `eval-fixtures.md` documents every field (`has_output`, `contains`, `not_contains`, `max_tokens`, `max_latency_ms`, `project_root`, `session_state`). | [ ] |

### Common Misinterpretation Risks

- [ ] A reviewer treats an aspirational failure as a regression — verify the skip path keeps them out of the failed count.
- [ ] A reviewer assumes `max_tokens` is exact — document that it is estimated as `len(output)/4`.
- [ ] A reviewer reads green CI as "hooks are accurate" — clarify the rig measures injected output, not whether the model obeys it.

---

## 3. Simplicity Assessment

**Goal:** Count steps to author a fixture and run the rig.

**Complexity Score:** Low (run) / Medium (author a new integration scenario).

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Run all unit evals | 1 step: `go test ./internal/hook/eval/` | No |
| Add a fixture | 1 step: drop a JSON file in the hook's `testdata/<hook>/` dir | No — auto-discovered by `LoadFixtures` |
| Pre-seed session state | 1 step: add `session_state.touches[]` to the fixture | No |
| Generate a report | 1 step: `go test -run TestEval_Report -v` | No |

### Default Behavior Verification

- [ ] `LoadFixtures` auto-discovers every `*.json` under the known hook and principle subdirectories.
- [ ] A fixture with no `session_state` runs without session pre-seeding.
- [ ] `project_root: "REPO_ROOT"` resolves to the repository root automatically.

---

## 4. Code Test Plan

### 4.1 Unit Tests (runner correctness)

Source under test: `internal/hook/eval/eval.go`. Driver: `internal/hook/eval/eval_test.go`.

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Load all fixtures | `testdata/` tree | Non-empty fixture list spanning all hook + principle dirs | must |
| UT-2 | `contains` assertion | Fixture expecting a substring present | Pass when present, fail with `output missing` when absent | must |
| UT-3 | `not_contains` assertion | Fixture forbidding a substring | Fail with `output should not contain` when present | must |
| UT-4 | `has_output: false` | Fixture expecting silence | Fail if any output produced | must |
| UT-5 | `max_tokens` budget | Output longer than budget | Fail with `tokens exceeds budget` | should |
| UT-6 | `max_latency_ms` budget | Slow hook | Fail with `latency exceeds budget` — except under `-race` | should |
| UT-7 | Race-detector latency skip | Build with `-race` | Latency budget is not enforced (`raceDetectorActive`) | must |
| UT-8 | Relative path resolution | `tool_input.file_path` relative | Resolved against `projectRoot` to an absolute path | must |
| UT-9 | Session pre-seed | Fixture with `session_state.touches` | Touches appended to a temp session dir before the hook runs | must |
| UT-10 | Hook dispatch | `hook` field per fixture | Routed to the correct `RunX`/`CompressOutput` function | must |
| UT-11 | Aspirational skip | Description/path contains `ASPIRATIONAL` | Skipped, excluded from totals | must |
| UT-12 | Aggregate math | Mixed pass/fail results | `Report` totals and per-hook averages correct | should |

### 4.2 Fixture Coverage (regression suite)

Fixtures live under `internal/hook/eval/testdata/`. Two families:

- **Hook infrastructure** — `quality-gate` (13), `compress-output` (10), `session-track` (9), `blast-radius` (9), `decision-guard` (10).
- **Universal principles** — `u01`..`u10` (≈49 fixtures) exercising the discover → suggest → enforce → migrate cycle.

| # | Coverage Check | Expected Behavior | Priority |
|---|----------------|-------------------|----------|
| FC-1 | Every regression fixture passes | A failure means a hook regressed | must |
| FC-2 | Aspirational fixtures are skipped, not failed | They mark capability gaps, not breakage | must |
| FC-3 | New hooks ship with a `testdata/<hook>/` dir | Coverage grows with the surface | should |
| FC-4 | `eval-fixtures.md` stays in sync with fixture counts | Doc reflects current pass/fail tallies | should |

### 4.3 Edge Case Tests

| # | Edge Case | Scenario | Expected Behavior | Priority |
|---|-----------|----------|-------------------|----------|
| EC-1 | Missing hook subdir | A configured dir does not exist | `LoadFixtures` skips it (treats `IsNotExist` as empty) | should |
| EC-2 | Malformed fixture JSON | Corrupt fixture file | `LoadFixtures` returns a parse error naming the file | must |
| EC-3 | Unknown hook name | `hook` value not in dispatch table | `runHook` returns empty string | should |
| EC-4 | Empty `tool_input` | Fixture omits file_path | Path resolution is a no-op; hook handles absence | should |
| EC-5 | Repo root not found | Run from a tree with no `go.mod` | Test fails fast with a clear message | should |

### 4.4 Aspirational Scenarios (Tier 2 / Tier 3, not yet implemented)

> These are documented so they are not mistaken for regressions. They define the eval rig's roadmap.

- A/B comparison (`hooks on` vs `hooks off`) producing net-token-impact deltas per scenario.
- Integration scenarios replayed from JSONL session sequences against pinned codebases.
- Live session metrics aggregated into `~/.kaboom/stats/lifetime.json`.
- A `kaboom-hooks eval` subcommand wrapping Tier 1/Tier 2 from the CLI (today the eval is test-only).
- CI regression gates on latency p99 (> 20%) and net savings (> 10%).

---

## 5. UAT Checklist (Engineer)

### Prerequisites

- [ ] Go toolchain installed
- [ ] Repository checked out with `go.mod` at the root

### Step-by-Step Verification

| # | Step | Expected Result | Pass |
|---|------|-----------------|------|
| UAT-1 | `go test ./internal/hook/eval/ -run TestEval_AllFixtures -count=1` | All regression fixtures pass; aspirational ones report as skipped | [ ] |
| UAT-2 | `go test ./internal/hook/eval/ -run TestEval_Report -v -count=1` | Per-hook summary and pass/fail totals print | [ ] |
| UAT-3 | Add a passing fixture to a hook's `testdata/` dir, rerun | New fixture is auto-discovered and passes | [ ] |
| UAT-4 | Introduce a deliberate regression in a hook, rerun | The corresponding fixture fails with a specific message | [ ] |
| UAT-5 | `go test -race ./internal/hook/eval/` | Suite passes; latency budgets are not enforced under race | [ ] |
| UAT-6 | Attempt to import `internal/hook/eval` from a production package and build | Build/lint fails per the hardening invariant | [ ] |

### Regression Checks

- [ ] Eval runtime stays well within the suite budget (Tier 1 < 10s on a typical machine).
- [ ] No temp session directories remain after the run.
- [ ] Production binaries do not link eval code.

---

## Sign-Off

| Area | Tester | Date | Pass/Fail |
|------|--------|------|-----------|
| Data Leak Analysis | | | |
| Clarity | | | |
| Simplicity | | | |
| Code Tests | | | |
| UAT | | | |
| **Overall** | | | |
