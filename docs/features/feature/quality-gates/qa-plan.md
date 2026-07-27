---
doc_type: qa-plan
feature_id: feature-quality-gates
status: proposed
owners: []
last_reviewed: 2026-07-27
links:
  index: ./index.md
  product: ./product-spec.md
  tech: ./tech-spec.md
  setup_guide: ./setup-guide.md
relates-to: [product-spec.md, tech-spec.md]
tags: [testing, qa, hooks]
---

# QA Plan: Quality Gates

> QA plan for the quality gates feature: one-command setup, the quality-gate enforcement hook, output compression, and token tracking. Covers data leak analysis, LLM clarity, simplicity, code-level testing, and step-by-step UAT.

---

## 1. Data Leak Analysis

**Goal:** The hooks read source files and command output and inject them into the agent's context, while the compression hook talks to the local daemon. Verify everything stays local and nothing pollutes the host protocol.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Source/output transmitted off-host | Quality-gate reads local files; compress-output posts only to `127.0.0.1`. Verify no external network calls. | critical |
| DL-2 | Secrets in injected standards/examples | The standards doc and convention examples are injected verbatim. Verify the scaffolded standards carry no secrets and example lines are trimmed (120 runes). | high |
| DL-3 | Stdout protocol pollution | Hooks must emit only the `additionalContext` JSON envelope. Any stray print breaks Claude/Gemini/Codex. Verify silence on bad input. | critical |
| DL-4 | Setup writing outside the project | `target_dir` is validated to be within the project root. Verify path-traversal attempts are rejected with `ErrPathNotAllowed`. | high |
| DL-5 | Overwriting user files | Setup must never overwrite an existing `.kaboom.json` or standards doc. Verify existing files are preserved. | high |
| DL-6 | Token-savings post leaking content | The post sends only `{category, tokens_before, tokens_after}`. Verify no command text or output is included. | medium |
| DL-7 | Compressed output dropping real failures | Compression must preserve failure lines. Verify failures are never silently discarded (a correctness-adjacent leak of signal). | high |

### Negative Tests (must NOT leak)

- [ ] Quality-gate and compress-output make no external (non-localhost) network connections.
- [ ] On malformed stdin, hooks exit silently with no output.
- [ ] Setup rejects a `target_dir` outside the project root.
- [ ] Setup does not overwrite an existing `.kaboom.json` or `kaboom-code-standards.md`.
- [ ] The token-savings POST body contains only counts and a category, never command text.

---

## 2. LLM Clarity Assessment

**Goal:** Verify the injected context is unambiguous so the agent fixes violations and reads compressed output correctly.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Standards block | Standards appear between `=== PROJECT CODE STANDARDS ===` and `=== END STANDARDS ===`. | [ ] |
| CL-2 | File-size signal | Above the limit reads `WARNING ... must be split`; near the limit reads an approaching-limit `NOTE`. | [ ] |
| CL-3 | Convention summary vs examples | The always-on `PROJECT CONVENTIONS` block is distinct from the matched `CODEBASE CONVENTIONS` examples. | [ ] |
| CL-4 | Helper suggestion | At 2+ instances the `SUGGESTION: Consider extracting a shared helper` note appears with the file count. | [ ] |
| CL-5 | Review instruction | The trailing `QUALITY GATE: Review your change ... Fix any violations before proceeding.` is present. | [ ] |
| CL-6 | Compressed output labeling | Compression output identifies the tool (e.g. go test summary) and shows failures, so the agent does not assume the run passed. | [ ] |
| CL-7 | Non-blocking semantics | The agent should treat findings as guidance to revise, not as a build failure. | [ ] |

### Common LLM Misinterpretation Risks

- [ ] The agent treats compressed output as the full log and misses that passing detail was elided — verify the summary states counts.
- [ ] The agent ignores the convention summary because it lacks examples — verify the always-on block is clearly framed as align-to guidance.
- [ ] The agent reads a near-limit `NOTE` as a hard failure — verify wording distinguishes NOTE from WARNING.
- [ ] The agent edits the standards doc to silence a rule instead of fixing code — document that standards changes should reflect real policy changes.

---

## 3. Simplicity Assessment

**Goal:** Count steps and cognitive load.

**Complexity Score:** Low — one setup command; enforcement is automatic.

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Turn on quality gates | 1 step: `configure(what="setup_quality_gates")` | No |
| Point at an existing standards doc | 1 step: set `code_standards` in `.kaboom.json` | No |
| Tune thresholds | 1 step: edit `file_size_limit` / `duplicate_threshold` | No |
| Compress test output | 0 steps: automatic on Bash | No |
| Add cheap AI review | 1 step: add a Haiku prompt hook | No |

### Default Behavior Verification

- [ ] Setup creates both files with sensible defaults and installs all hook entries.
- [ ] Setup is idempotent — re-running does not duplicate hooks or overwrite files.
- [ ] Quality-gate fires on Edit and Write, not on Read.
- [ ] Compression fires only when Bash output exceeds the line threshold.

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Source / Input | Expected Output | Priority |
|---|-----------|----------------|-----------------|----------|
| UT-1 | Non-edit ignored | `quality_gate.go`; tool_name Read | Returns nil | must |
| UT-2 | Standards injection | Edit with `.kaboom.json` + standards present | `PROJECT CODE STANDARDS` block injected | must |
| UT-3 | File over limit | File above `file_size_limit` | `WARNING ... must be split` | must |
| UT-4 | File near limit | File at 90–100% of limit | Approaching-limit `NOTE` | should |
| UT-5 | Missing project root | Edit outside any `.kaboom.json` | Returns nil | must |
| UT-6 | Malformed config | Corrupt `.kaboom.json` | Falls back to defaults | must |
| UT-7 | Write fallback content | Write with empty content field | Reads the written file from disk | should |
| UT-8 | Convention summary injected | Any edit in a discovered project | `PROJECT CONVENTIONS` block present | must |
| UT-9 | Matched examples + helper suggestion | Edit with `http.Client{` used 2+ times | Examples + helper suggestion | must |
| UT-10 | Compress go test pass | `compress_output.go`; 50+ pass lines | `N passed, 0 failed` summary | must |
| UT-11 | Compress go test fail | passes + 1 failure | Failure name and message preserved | must |
| UT-12 | Short output untouched | < threshold lines | No compression | must |
| UT-13 | Non-Bash ignored | Read with tool_response | No compression | must |
| UT-14 | Agent-adapted envelope | `protocol.go`; Gemini env set | Output nested under `hookSpecificOutput` | should |
| UT-15 | Empty context → no output | `WriteOutput("")` | Writes nothing | must |

### 4.2 Setup Tests

Source: `cmd/browser-agent/internal/toolconfigure/qualitygates/handler.go`. Tests: `qualitygates/handler_test.go` and `tools_configure_quality_gates_test.go`.

| # | Test Case | Input | Expected Behavior | Priority |
|---|-----------|-------|-------------------|----------|
| ST-1 | Fresh setup | Empty project | `.kaboom.json` + standards + hooks created | must |
| ST-2 | Existing config preserved | `.kaboom.json` present | Not overwritten; standards path read from it | must |
| ST-3 | Custom standards path | `code_standards` points elsewhere | Default standards file not created; suggestion to ensure it exists | should |
| ST-4 | Idempotent hook install | Hooks already present | `hooks_installed: false`, no duplication | must |
| ST-5 | Path-traversal guard | `target_dir` outside project | `ErrPathNotAllowed` | must |
| ST-6 | Missing active codebase | No active codebase set | `ErrNotInitialized` with guidance | must |
| ST-7 | Hook matrix correct | Fresh install | Edit/Write, Read, and Bash matchers wired to the right commands | must |

### 4.3 Integration / Tracking Tests

| # | Test Case | Components | Expected Behavior | Priority |
|---|-----------|-----------|-------------------|----------|
| IT-1 | End-to-end quality-gate | `kaboom-hooks quality-gate` via stdin | Valid envelope on a real edit | must |
| IT-2 | End-to-end compression | `kaboom-hooks compress-output` via stdin | Compressed envelope + best-effort stats post | must |
| IT-3 | Token tracking | `internal/tracking/` | Savings aggregated per session; lifetime persisted | should |
| IT-4 | Install contract | `scripts/release/install-upgrade-regression.contract.test.mjs`, `scripts/test-install-hooks-only.sh` | Install/upgrade preserves user settings and updates managed entries | must |
| IT-5 | Eval fixtures | `internal/hook/eval/testdata/quality-gate/`, `compress-output/` | Regression fixtures pass | must |

### 4.4 Edge Case Tests

| # | Edge Case | Scenario | Expected Behavior | Priority |
|---|-----------|----------|-------------------|----------|
| EC-1 | Daemon unreachable | Compression with no daemon | Compression still returns; stats post fails silently | should |
| EC-2 | Huge repo | Quality-gate on a 500+ file project | Stays within latency budget via caps/cache | should |
| EC-3 | File deleted before hook | Edit then delete | Gate bails on the stat check | should |
| EC-4 | Unknown Bash tool | Output with no recognized pattern | Generic head+tail compression above threshold | should |
| EC-5 | Bad stdin | Non-JSON input | Silent exit, return 0 | must |

---

## 5. UAT Checklist (Human + AI)

### Prerequisites

- [ ] `kaboom-hooks` on PATH (`kaboom-hooks --version`)
- [ ] An active codebase set in the MCP server
- [ ] A project with at least 3 files sharing a known pattern and one oversized file

### Step-by-Step Verification

| # | Step | Expected Result | Pass |
|---|------|-----------------|------|
| UAT-1 | Run `configure(what="setup_quality_gates")` | `.kaboom.json` + `kaboom-code-standards.md` created; hooks installed to `.claude/settings.json` | [ ] |
| UAT-2 | Re-run setup | No duplicate hooks; existing files preserved | [ ] |
| UAT-3 | Edit a source file | Standards + convention summary + review instruction injected | [ ] |
| UAT-4 | Edit a file over the size limit | `WARNING ... must be split` appears | [ ] |
| UAT-5 | Edit introducing `http.Client{` (already used 2+ times) | Existing examples + helper-extraction suggestion | [ ] |
| UAT-6 | Run a verbose `go test` via Bash | Output compressed to a summary plus failures | [ ] |
| UAT-7 | Run a failing test | Failure name and message preserved in the compressed output | [ ] |
| UAT-8 | Check token savings | Session savings logged on daemon shutdown; lifetime stats updated | [ ] |
| UAT-9 | Edit with Gemini/Codex env set | Output envelope adapts to the agent | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Local-only traffic | Monitor network during a hook run | Only `127.0.0.1` (token-savings) traffic | [ ] |
| DL-UAT-2 | No stdout noise | Inspect hook stdout | Only the JSON envelope | [ ] |
| DL-UAT-3 | Setup respects boundaries | Attempt setup with an out-of-project `target_dir` | Rejected | [ ] |

### Regression Checks

- [ ] Existing `.claude/settings.json` entries are preserved after install.
- [ ] Read-tool edits do not trigger the quality gate.
- [ ] Short Bash output is left uncompressed.
- [ ] Other hooks (session-track, blast-radius, decision-guard) still fire alongside quality-gate.

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
