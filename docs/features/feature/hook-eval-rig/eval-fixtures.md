# Eval Fixtures Reference

> 51 fixtures, 28 pass / 23 fail (55%). All fixtures run against the kaboom codebase itself.
>
> **Regression fixtures** validate existing capabilities. All must pass — a failure means a hook broke.
> **Aspirational fixtures** define improvement targets. Each failure is a concrete capability gap.

---

## Quality Gate (6/13)

Source: `internal/hook/hook_policy.go` (195 LOC), `internal/hook/conventions.go` (251 LOC)

The quality gate fires on every Edit/Write. It finds `.kaboom.json` in the parent chain, loads the project's code standards, checks file size, and detects convention drift.

### Passing

**001 — Standards injection** `quality-gate/001-standards-injection.json`

Edits `internal/hook/hook_policy.go`. The hook finds `.kaboom.json` at the repo root, reads `kaboom-code-standards.md`, and injects the first 150 lines into the agent's context. Expects `PROJECT CODE STANDARDS` and `Max 800 lines`. This is the core loop: every edit, the agent sees the project's rules.

**002 — File size warning** `quality-gate/002-file-size-warning.json`

Edits `tools_analyze_annotations_test.go` (1937 lines). With a limit of 800, the hook emits `WARNING: ... must be split`. Tests that the line counter works and the threshold math is right.

**003 — Non-edit ignored** `quality-gate/003-non-edit-ignored.json`

Read tool on `protocol.go`. Quality gate only fires on Edit/Write. Expects no output. Guards against the hook wasting tokens on reads.

**004 — Convention detection** `quality-gate/004-convention-detection.json`

Edits `session_store.go` with `json.NewEncoder(w).Encode(result)`. Convention detection finds existing usage of `json.NewEncoder(` in `protocol.go` and shows it. Expects `CODEBASE CONVENTIONS` and `json.NewEncoder(`. This prevents convention drift — the agent sees how the project already does it.

**005 — Write tool triggers** `quality-gate/005-write-tool-triggers.json`

Uses the Write tool (not Edit) on `protocol.go`. Verifies that Write also triggers the quality gate, not just Edit.

**008 — Helper extraction** `quality-gate/008-duplicate-pattern-detection.json`

Edits `session_store.go` with `os.ReadFile(path)`. Convention detection finds 2+ existing usages and outputs `SUGGESTION: Consider extracting a shared helper`. Tests the duplication detection threshold.

### Failing

**006 — Error format violation** `quality-gate/006-error-format-violation.json`

Edits with `errors.New("something broke")` — a lowercase, unstructured error. The standards doc says errors must use `{OPERATION}: {ROOT_CAUSE}. {RECOVERY_ACTION}`. Expects the gate to flag `"something broke"` specifically as violating the format. **Why it fails:** The quality gate injects the standards doc (which mentions error format) but doesn't parse the edit to check compliance. It's a reference injector, not a linter.

**007 — Missing file header** `quality-gate/007-missing-file-header.json`

Writes a Go file without the required `// filename.go — Purpose` header. Expects the gate to flag it. **Why it fails:** Same reason — the gate injects standards that say "file headers required" but doesn't inspect the edit for missing headers.

**009 — TypeScript `any` detection** `quality-gate/009-typescript-any-detection.json`

Edits a `.ts` file with `data: any`. Expects the gate to flag the specific `any` usage as a strict mode violation. **Why it fails:** The gate injects standards mentioning "No `any`" but doesn't scan the edit for TypeScript-specific patterns. The convention detection only looks for probes like `fetch(`, not language violations.

**010 — God function detection** `quality-gate/010-god-function-detection.json`

Edits with a 60-line function body. Expects the gate to warn about function length. **Why it fails:** The gate checks file-level line count, not function-level. It has no concept of function boundaries.

**011 — Unchecked error return** `quality-gate/011-unchecked-error-return.json`

Edits with `data, _ := os.ReadFile(path)` and `os.WriteFile(out, data, 0644)` (error ignored). Expects the gate to flag the unchecked error on `os.WriteFile`. **Why it fails:** Convention detection fires on `os.ReadFile(` and `os.WriteFile(` but only to show existing usage — it doesn't analyze error handling patterns.

**012 — JSON camelCase violation** `quality-gate/012-json-camelcase-violation.json`

Edits with ``json:"userName"`` tag. Expects the gate to flag `userName` as a snake_case violation. **Why it fails:** Convention detection checks for `type X struct` declarations (duplicate detection) but doesn't parse JSON struct tags.

**013 — Dead code detection** `quality-gate/013-dead-code-detection.json`

Edits with 3 lines of commented-out code. Expects the gate to flag it. **Why it fails:** No comment analysis exists. The gate doesn't distinguish code from comments.

---

## Compress Output (6/10)

Source: `internal/hook/compress_output.go` (426 LOC)

Compresses verbose Bash output (test runners, build tools) to summaries. Preserves errors, strips noise. Fires when Bash output exceeds 50 lines.

### Passing

**001 — Go test pass compression** `compress-output/001-go-test-pass.json`

25 passing tests (50+ lines). Compresses to `go test summary: 25 passed, 0 failed`. Verifies the `=== RUN` noise is stripped. The agent sees a 2-line summary instead of 50 lines.

**002 — Go test fail compression** `compress-output/002-go-test-fail.json`

24 passes + 1 fail (TestHandler). Compresses but preserves the failure name and error message (`expected 200 but got 500`). The agent sees what broke without wading through 24 passing tests.

**003 — Short output no-compress** `compress-output/003-short-output-no-compress.json`

`echo hello` produces 1 line. Below 50-line threshold, no compression. Guards against mangling short output.

**004 — Non-Bash ignored** `compress-output/004-non-bash-ignored.json`

Read tool with tool_response. Compression only fires on Bash. Expects no output.

**005 — Boundary 49 lines** `compress-output/005-boundary-49-lines-no-compress.json`

24 test pairs = 48 content lines + trailing newline = 49 split elements. `49 < 50` threshold not reached. No compression. Tests the exact boundary.

**006 — Boundary 50 lines** `compress-output/006-boundary-50-lines-compress.json`

24 test pairs + 1 summary = 49 content lines + trailing newline = 50 split elements. `50 >= 50` threshold reached. Expects `24 passed, 0 failed`. Tests the other side of the boundary.

### Failing

**007 — Stack trace preservation** `compress-output/007-preserves-stack-trace.json`

TestPanic panics with `index out of range [5] with length 3` and a goroutine stack trace (`handlers.go:87`). Expects the compressed output to include both `index out of range` and `handlers.go:87`. **Why it fails:** The compressor keeps only the first error line per failed test (`pendingErrors[0]`). The stack trace after line 0 is lost.

**008 — ESLint compression** `compress-output/008-eslint-compression.json`

58 lines of ESLint output with 5 errors and 35 warnings across 10 files. Expects eslint-specific compression (summary counts, drop repetitive rule names). **Why it fails:** No eslint compressor exists. The output false-matches the pytest compressor because `\d+ error` matches ESLint's `45:10 error`. It gets categorized as `pytest summary:` — wrong tool, wrong format. The fixture's `not_contains: ["pytest"]` catches this.

**009 — npm install compression** `compress-output/009-npm-install-compression.json`

50+ lines of `npm warn deprecated` spam. Expects compression to package count and vulnerability summary. **Why it fails:** No npm compressor exists. The fixture expects semantic compression — extracting the `847 packages` and vulnerability counts, not generic head/tail truncation.

**010 — Sub-test hierarchy** `compress-output/010-subtests-preserved.json`

Go test with `TestHandler/nil_body` failing inside parent `TestHandler`. Expects compressed output to show the sub-test path and its error, while stripping passing sub-tests of unrelated parents. **Why it fails:** The `pendingErrors` logic collects errors per `currentRun`, but sub-test runs reset `currentRun`. The parent test's failure detail doesn't link correctly to the sub-test's error context.

---

## Session Track (5/9)

Source: `internal/hook/session.go` (130 LOC), `internal/hook/session.go` (278 LOC)

Records every tool use in an append-only JSONL session log. Detects redundant reads and injects session summaries on edits.

### Passing

**001 — First read** `session-track/001-first-read.json`

First Read of a file with no session history. No redundant read, no session summary (reads don't trigger summaries). Expects no output. Verifies the hook doesn't over-inject.

**002 — Redundant read** `session-track/002-redundant-read.json`

Session state has a prior Read of the same file. Reading it again triggers `[Session] You read this file N sec ago. No edits since.` Tests the core redundancy detection.

**003 — Edit summary** `session-track/003-edit-summary.json`

Session has 2 reads + 1 bash command. Editing a file triggers `[Session] 2 files read, 1 edited, 1 commands.` Tests the session summary injection on edits.

**004 — Redundant read with edit** `session-track/004-redundant-read-with-edit.json`

Session has a Read, then an Edit of the same file. Reading it again triggers `You read this file ... ago. You edited it ... ago.` Tests that edit-awareness is included in redundant read warnings.

**005 — High-volume compact** `session-track/005-high-volume-compact.json`

Session with 19 touches (10 reads, 6 edits, 3 bashes). Editing another file triggers a session summary. Expects `max_tokens: 50` — the summary must stay compact regardless of session size. Tests that the one-liner format scales.

### Failing

**006 — Bash `cat` as read** `session-track/006-bash-not-tracked-as-read.json`

Session has `Bash: cat /p/handlers.go`. Then a Read of that same file. Expects the hook to recognize the Bash `cat` as a prior read and warn about redundancy. **Why it fails:** Session track classifies Bash as action `"bash"`, not `"read"`. It records the command summary but doesn't parse the command to extract file paths. `WasFileRead` only looks for entries with `action == "read"`.

**007 — `grep` as read** `session-track/007-grep-as-read.json`

Same idea: session has `Bash: grep -n 'WriteOutput' /project/internal/hook/hook_policy.go`. Then a Read of that file. **Why it fails:** Same root cause — Bash commands aren't parsed for file references.

**008 — Edit without tests** `session-track/008-edit-without-tests-warning.json`

Session has 6 consecutive edits across 5 files with zero Bash/test commands. Editing a 7th file should warn to run tests. **Why it fails:** `SessionSummary` reports counts (`6 edited, 0 commands`) but doesn't interpret them. No logic says "you've edited many files without testing."

**009 — Circular edit warning** `session-track/009-circular-edit-warning.json`

Session has: read A, edit A, read B, edit B, test fails, edit A again. Should warn about re-editing. **Why it fails:** The hook records all touches but doesn't detect revisiting. The edit action triggers `SessionSummary`, which counts totals but doesn't flag that `handler.go` was already edited earlier in the session.

---

## Blast Radius (4/9)

Source: `internal/hook/blast_radius.go` (529 LOC)

Builds a per-language import graph, detects whether the edit touches exported symbols, and lists files that import the edited file.

### Passing

**001 — Exported function edit** `blast-radius/001-edit-exported-function.json`

Edits `protocol.go` with `func WriteOutput(...)` — uppercase Go function = exported. The import graph finds files that import `internal/hook`. Output shows `[Blast Radius] ... imported by N file(s)`. Core functionality works.

**002 — Unexported function edit** `blast-radius/002-edit-internal-only.json`

Edits `protocol.go` with `func parseInternal()` — lowercase = unexported. `looksExported` returns false, hook returns nil. No blast radius warning for internal changes.

**003 — Read ignored** `blast-radius/003-read-ignored.json`

Read tool on `protocol.go`. `isEditTool("Read")` returns false. No blast radius on reads.

**004 — File path in output** `blast-radius/004-output-contains-filepath.json`

Same edit as 001 but specifically checks that `internal/hook/hook_policy.go` (the relative path) appears in the output, and that importers are listed under `Not yet reviewed`. Tests the formatting, not just existence of output.

### Failing

**005 — Session-aware importers** `blast-radius/005-session-aware-importers.json`

Session state has a prior Read of `cmd/hooks/main.go` (an importer of `protocol.go`). Edits `protocol.go`. Expects the output to show `main.go` under `Already in session` instead of `Not yet reviewed`. **Why it fails:** The session touch records the file path as `REPO_ROOT/cmd/hooks/main.go` (from the fixture's session_state). But `WasFileRead` compares against the absolute path `filepath.Join(projectRoot, imp)`. The fixture path and the resolved path don't match — the fixture uses the literal string `REPO_ROOT/...`, which is not resolved to the actual repo root path.

**006 — TypeScript import resolution** `blast-radius/006-cross-language-ts-import.json`

Edits `src/content/window-message-listener.ts` with an `export function`. `content.ts` imports it via `from './content/window-message-listener.js'`. **Why it fails:** The import resolver tries `candidate + ""` where candidate = `src/content/window-message-listener.js`. The `.js` file doesn't exist (it's TypeScript source). Then it tries `candidate + ".ts"` = `window-message-listener.js.ts` — wrong. It never strips `.js` and tries `.ts`. Standard TS/ESM pattern is to import with `.js` extension but the source is `.ts`.

**007 — Test file awareness** `blast-radius/007-test-file-awareness.json`

Edits `quality_gate.go` and expects the output to mention `quality_gate_test.go`. **Why it fails:** `buildImportGraph` skips `_test.go` files in `findGoFilesInDir`. Test files are excluded from the import graph. No separate mechanism lists co-located test files.

**008 — Function signature change** `blast-radius/008-function-signature-change.json`

Edits `WriteOutput` adding a third parameter. Expects the output to flag this as a breaking signature change. **Why it fails:** The blast radius knows the file is imported but doesn't compare old vs new function signatures. It has no concept of "this specific function changed" — only "this file changed and it's imported."

**009 — Interface change** `blast-radius/009-interface-change.json`

Edits to add a method to an interface. Expects warning about implementors. **Why it fails:** No interface analysis exists. The import graph tracks file-level imports, not type-level relationships. Go interfaces are implicitly satisfied — there's no `implements` keyword to grep for.

---

## Decision Guard (7/10)

Source: `internal/hook/hook_policy.go` (156 LOC)

Checks edited code against locked architectural decisions in `.kaboom/decisions.json`. Supports literal patterns, regex, and expiry dates.

### Passing

**001 — Pattern match** `decision-guard/001-pattern-match.json`

Edit with `require (\n\t"github.com/some/dep" v1.0.0\n)`. Matches DECISION-001's literal pattern `require (`. Output shows `ARCHITECTURAL DECISIONS`, `DECISION-001`, `zero production dependencies`, `DECISION GUARD`.

**002 — No match** `decision-guard/002-no-match.json`

Edit with `func HandleNewEndpoint() error { return nil }`. No decision patterns match. No output.

**003 — Regex match** `decision-guard/003-regex-match.json`

Edit with `errors.New("something went wrong")`. Matches DECISION-002's regex `errors\.New\("[a-z]` (lowercase first char after `"`). Output shows `DECISION-002`, `structured error format`. Also verifies `not_contains: ["DECISION-001"]` — only the matching decision fires.

**004 — Expired decision** `decision-guard/004-expired-decision.json`

Edit with `EXPIRED_DECISION_SENTINEL_VALUE`. This matches DECISION-EVAL-EXPIRED's pattern, but the decision has `"expires": "2025-06-01"` — already past. `isExpired()` returns true, the decision is skipped. No output. Tests the expiry mechanism.

**005 — Multi-decision match** `decision-guard/005-multi-decision-match.json`

Edit with `fmt.Println(errors.New("bad error message"))`. Matches DECISION-002 (lowercase error) AND DECISION-003 (`fmt.Print`). Output contains both IDs but not DECISION-001. Tests that multiple decisions fire independently.

**006 — Near-miss no trigger** `decision-guard/006-near-miss-no-trigger.json`

Edit with `// This module does not require any external dependencies`. Contains `require` but NOT `require (` (missing the open paren). DECISION-001's pattern is literal `require (` — no match. Tests precision of literal matching.

**007 — Uppercase error OK** `decision-guard/007-uppercase-error-ok.json`

Edit with `errors.New("ReadInput: cannot read stdin. Check file descriptor")`. DECISION-002's regex is `errors\.New\("[a-z]` — the char after `"` must be lowercase. `R` is uppercase, so no match. Tests that properly formatted errors aren't flagged.

### Failing

**008 — False positive in comment** `decision-guard/008-false-positive-in-comment.json`

Edit with `// NOTE: We avoid fmt.Println here because WriteOutput handles agent detection.` The regex `fmt\.Print` matches `fmt.Println` even though it's inside a comment. Expects no output. **Why it fails:** `matchesDecision` runs the regex against the entire `newContent` string with no comment stripping. It can't distinguish `// fmt.Println` from `fmt.Println`.

**009 — False positive in string** `decision-guard/009-false-positive-in-string.json`

Edit with `helpText := "Do not use fmt.Println directly."` Same issue — `fmt.Println` inside a string literal matches the regex. **Why it fails:** Same root cause. No string-literal awareness in the matcher.

**010 — Test file exemption** `decision-guard/010-test-file-exemption.json`

Edit of `protocol_test.go` with `fmt.Println("debug output")`. Test files should be exempt from DECISION-003 (direct stdout is fine in tests). **Why it fails:** The decision guard doesn't check the file path. It applies all decisions uniformly regardless of whether the file is production code or test code.

---

## Fixture File Format

Each fixture is a JSON file in `internal/hook/eval/testdata/<hook-name>/`:

```json
{
  "description": "Human-readable test description",
  "hook": "quality-gate",
  "project_root": "REPO_ROOT",
  "session_state": {
    "touches": [
      {"t": "2026-03-07T14:00:00Z", "tool": "Read", "file": "/path", "action": "read"}
    ]
  },
  "input": {
    "tool_name": "Edit",
    "tool_input": {"file_path": "relative/to/repo.go", "new_string": "..."},
    "tool_response": "..."
  },
  "expect": {
    "has_output": true,
    "contains": ["must appear in output"],
    "not_contains": ["must NOT appear"],
    "max_tokens": 50,
    "max_latency_ms": 500
  }
}
```

| Field | Purpose |
|-------|---------|
| `project_root: "REPO_ROOT"` | Resolves to the kaboom repo root at test time |
| `session_state` | Pre-populates the session store before running the hook |
| `has_output` | `true` = hook must produce output, `false` = must be silent |
| `contains` | Every string must appear in the output |
| `not_contains` | None of these strings may appear in the output |
| `max_tokens` | Output must not exceed this (estimated as `len/4`) |
| `max_latency_ms` | Hook must complete within this budget |

## Running the Eval

```bash
go test ./internal/hook/eval/ -v -run TestEval_Report -count=1
```

The eval is test-only code. A lint invariant (check 16 in `scripts/quality/verification/lint-hardening.sh`) prevents `hook/eval` from being imported by production binaries.
