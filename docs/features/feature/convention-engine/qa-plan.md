---
doc_type: qa-plan
feature_id: feature-convention-engine
status: proposed
owners: []
last_reviewed: 2026-07-05
links:
  index: ./index.md
  product: ./product-spec.md
  tech: ./tech-spec.md
relates-to: [product-spec.md, tech-spec.md]
tags: [testing, qa, hooks, conventions]
---

# QA Plan: Convention Engine

> QA plan for Phase 1 of the convention engine — automatic call-site discovery and per-edit convention injection inside the `kaboom-hooks` quality gate. Covers data leak analysis, LLM clarity, simplicity, code-level testing, and UAT.

---

## 1. Data Leak Analysis

**Goal:** The engine reads source files across the project and injects snippets into the agent's context. Verify it stays local and does not expose data beyond the host process.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Source content transmitted off-host | Discovery and detection read files only from disk and emit text to stdout via the hook envelope. Verify no network calls. | critical |
| DL-2 | Secrets surfaced in examples | `searchProject` returns matching lines verbatim. A pattern that coincidentally matches a line containing a secret would inject it. Verify probes target call-site shapes, not values, and that example lines are trimmed to 120 runes. | high |
| DL-3 | Scanning outside the project root | Walks start at `FindProjectRoot` (nearest `.kaboom.json`). Verify `skipDirs` excludes `.git`, `vendor`, `node_modules`, `dist`, `build`, `.claude`, and hidden dirs. | medium |
| DL-4 | Reading oversized/generated blobs | Verify `maxFileSizeForScan` (100KB) and `isGenerated` skip bundled/minified files that may contain embedded data. | medium |
| DL-5 | Stdout protocol integrity | All output flows through the quality gate's `additionalContext`. Verify the engine never prints directly. | high |

### Negative Tests (must NOT leak)

- [ ] Discovery and detection make zero network connections.
- [ ] Example lines are truncated to 120 runes with an ellipsis.
- [ ] `node_modules`, `vendor`, `.git`, and hidden directories are never scanned.
- [ ] Files above 100KB and generated files (`.bundled.`, `.min.`, `.map`) are skipped.

---

## 2. LLM Clarity Assessment

**Goal:** The injected convention context must be unambiguous so the model aligns new code without confusion.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Summary block delimiters | The always-on summary is wrapped in `=== PROJECT CONVENTIONS (auto-discovered) ===` ... `=== END PROJECT CONVENTIONS ===`. | [ ] |
| CL-2 | Frequency signal | Each summarized pattern shows `pattern (N files)` so the model can weigh how established it is. | [ ] |
| CL-3 | Example block | Matched patterns appear under `=== CODEBASE CONVENTIONS (match existing patterns) ===` with `path:line: content` examples. | [ ] |
| CL-4 | Helper suggestion | When a pattern exists in 2+ files, the `SUGGESTION: Consider extracting a shared helper` note appears with the file count. | [ ] |
| CL-5 | Convention vs decision | Convention context is advisory ("align"), distinct from the blocking tone of decision guard. The model should treat it as guidance. | [ ] |

### Common LLM Misinterpretation Risks

- [ ] The model treats a discovered pattern as a mandatory rule rather than a strong default — verify wording says "align," not "must."
- [ ] The model copies an example line verbatim instead of adapting it — examples are evidence of the convention, not snippets to paste.
- [ ] The model conflates the auto-discovered summary with the project standards doc — verify the two blocks are clearly delimited.

---

## 3. Simplicity Assessment

**Goal:** Count steps and cognitive load.

**Complexity Score:** Low — zero configuration; runs inside the existing quality gate.

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Get conventions on every edit | 0 steps after quality-gate install | No |
| Limit which patterns surface | 0 steps — frequency + noise filter handle it | No |
| Tune duplicate threshold | 1 step: set `duplicate_threshold` in `.kaboom.json` | No |

### Default Behavior Verification

- [ ] With no configuration, discovery runs automatically on the first edit in a project.
- [ ] Patterns appearing in fewer than 3 files do not surface.
- [ ] The summary is capped at 10 patterns; detection reports at most 3 matched patterns.
- [ ] Universal noise patterns never appear as conventions.

---

## 4. Code Test Plan

### 4.1 Unit Tests

Source under test: `internal/hook/convention_discover.go`, `internal/hook/convention_detect.go`. Tests: `convention_discover_test.go`, `convention_detect_test.go`.

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Discover repeated Go call-site | Tree where `pkg.Foo(` appears in 3+ files | Pattern returned with correct file count | must |
| UT-2 | Below-threshold pattern dropped | Pattern in only 2 files | Not returned | must |
| UT-3 | Go noise filtered | `fmt.Sprintf(`, `strings.Contains(` widespread | Excluded from results | must |
| UT-4 | TS noise filtered | `console.log(`, `JSON.stringify(` widespread | Excluded from results | must |
| UT-5 | Frequency sort | Patterns with varying counts | Sorted by file count descending | must |
| UT-6 | Probe cap | More than 20 qualifying patterns | Truncated to 20 | should |
| UT-7 | Cache hit within TTL | Two calls within 5 minutes | Second returns cached result without re-walk | should |
| UT-8 | Language family resolution | `.tsx` file | Scans `.ts/.tsx/.js/.jsx` | must |
| UT-9 | Static probe match | Edit adds `http.Client{` | Detection reports it with examples | must |
| UT-10 | Type-declaration probe | Edit adds `type Foo struct` | `type Foo struct` searched as a duplicate probe | must |
| UT-11 | Helper suggestion threshold | Pattern in 2+ files | Suggestion text appended | must |
| UT-12 | No examples → no match | Probe present in edit but absent elsewhere | Pattern omitted from results | should |
| UT-13 | Example line truncation | Matching line > 120 runes | Trimmed with ellipsis | should |
| UT-14 | Self-file excluded | Edited file itself contains the pattern | Edited file not listed as an example | should |
| UT-15 | Skip dirs honored | Pattern only inside `node_modules` | Not surfaced | must |

### 4.2 Integration Tests

| # | Test Case | Components | Expected Behavior | Priority |
|---|-----------|-----------|-------------------|----------|
| IT-1 | Quality gate injects summary | `RunQualityGate` on an edit | `PROJECT CONVENTIONS` block present | must |
| IT-2 | Quality gate injects examples | Edit containing a known probe | `CODEBASE CONVENTIONS` block with examples | must |
| IT-3 | Discovery feeds detection | `DiscoveredProbes` merged with static probes | Discovered patterns drive example search | must |
| IT-4 | Eval fixtures on real repo | `internal/hook/eval/testdata/quality-gate/` and `u01..u10` | Regression fixtures pass against the Kaboom codebase | must |

### 4.3 Edge Case Tests

| # | Edge Case | Scenario | Expected Behavior | Priority |
|---|-----------|----------|-------------------|----------|
| EC-1 | Empty project | No source files | No conventions; no summary | should |
| EC-2 | No `.kaboom.json` | Edit outside a configured project | Quality gate (and engine) silent | must |
| EC-3 | Oversized files only | All candidates > 100KB | Skipped; no conventions | should |
| EC-4 | Mixed-language repo | Go + TS files | Each edit uses its own language family and noise set | should |
| EC-5 | Stale cache after refactor | Patterns change within 5 minutes | Old result served until TTL expires, then refreshed | should |
| EC-6 | Scan cap reached | Repo > 500 candidate files | Discovery stops at the cap (best-effort) | should |

---

## 5. UAT Checklist (Human + AI)

### Prerequisites

- [ ] `kaboom-hooks` installed and registered on `Edit|Write`
- [ ] Project has `.kaboom.json` at the root
- [ ] Project contains at least 3 files sharing a non-trivial call-site pattern

### Step-by-Step Verification

| # | Step | Expected Result | Pass |
|---|------|-----------------|------|
| UAT-1 | Edit any source file | Context includes the `PROJECT CONVENTIONS (auto-discovered)` block | [ ] |
| UAT-2 | Edit introducing `http.Client{` in a repo that already uses it | Context shows existing usages and a helper-extraction suggestion | [ ] |
| UAT-3 | Edit introducing a brand-new pattern absent elsewhere | No example block for that pattern | [ ] |
| UAT-4 | Add `type Widget struct` where a `Widget` type already exists | Duplicate-type probe surfaces the existing declaration | [ ] |
| UAT-5 | Edit a `.ts` file | Conventions reflect the TS/JS family, not Go | [ ] |
| UAT-6 | Edit twice within 5 minutes | Second edit reuses cached discovery (no perceptible slowdown) | [ ] |

### Regression Checks

- [ ] Standards-doc injection, file-size warnings, and the review instruction still appear.
- [ ] Quality-gate latency remains within budget on a large repo.
- [ ] Universal noise patterns never appear in the summary.

---

## Sign-Off

| Area | Tester | Date | Pass/Fail |
|------|--------|------|-----------|
| Data Leak Analysis | | | |
| LLM Clarity | | | |
| Simplicity | | | |
| Code Tests | | | |
| UAT | | | |
| **Overall** | | | |
