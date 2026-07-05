---
doc_type: qa-plan
feature_id: feature-decision-guard
status: proposed
owners: []
last_reviewed: 2026-07-05
links:
  index: ./index.md
  product: ./product-spec.md
  tech: ./tech-spec.md
relates-to: [product-spec.md, tech-spec.md]
tags: [testing, qa, hooks]
---

# QA Plan: Decision Guard

> QA plan for the Decision Guard PostToolUse hook. Covers data leak analysis, LLM clarity, simplicity, code-level testing, and step-by-step UAT verification. Decision Guard is a standalone `kaboom-hooks` subcommand that reads `.kaboom/decisions.json` and injects matching architectural decisions on Edit/Write.

---

## 1. Data Leak Analysis

**Goal:** Verify the hook does not transmit, log, or expose code or decision content beyond the local process. The hook reads from stdin, reads one project file, and writes to stdout. It must make no network calls.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Edited code transmitted off-host | The hook receives the full `new_string`/`content` on stdin. Verify it is only matched against patterns in memory and never written to a network socket or temp file. | critical |
| DL-2 | Decision rules leaking secrets | `.kaboom/decisions.json` is committed to the repository. Verify the format encourages rules and patterns, not credentials. Document that secrets must never be placed in `rule`, `reason`, or `pattern` fields. | high |
| DL-3 | Stdout pollution breaking the host agent | The hook must write only the `additionalContext` JSON envelope to stdout. Any stray print breaks the Claude Code/Gemini/Codex hook protocol. | high |
| DL-4 | Reading files outside the project root | Verify `loadDecisions` only reads `<projectRoot>/.kaboom/decisions.json`, where `projectRoot` is found by walking up for `.kaboom.json`. No traversal outside that root. | medium |
| DL-5 | Regex denial-of-service | A malicious or accidental catastrophic-backtracking regex in a decision could hang the hook. Verify invalid regex is skipped (`regexp.Compile` error returns no match) and the hook stays within its latency budget. | medium |
| DL-6 | Error output noise | On bad stdin the hook must exit silently (return 0, no stdout). Verify malformed input never echoes the raw payload to stderr or stdout. | medium |

### Negative Tests (must NOT leak)

- [ ] The hook makes zero network connections during execution (verify with a syscall trace or offline run).
- [ ] On unparseable `decisions.json`, the hook returns no output rather than echoing file contents.
- [ ] On non-Edit/non-Write tool input, the hook is silent (no decision content emitted).
- [ ] Invalid regex in a decision is skipped, not surfaced as an error containing the pattern.

---

## 2. LLM Clarity Assessment

**Goal:** Verify the injected `additionalContext` is unambiguous so the calling model revises its edit correctly.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Block delimiters | Output is wrapped in `=== ARCHITECTURAL DECISIONS (do not violate) ===` ... `=== END DECISIONS ===` so the model can isolate the section. | [ ] |
| CL-2 | Decision identity | Each decision shows `[ID] rule`, optionally `Reason:` and `Enforced:`. The model can cite the ID when explaining its change. | [ ] |
| CL-3 | Actionable closing line | The trailing `DECISION GUARD: Your edit matches a locked architectural decision. Revise to comply.` tells the model what to do. | [ ] |
| CL-4 | Multiple matches | When several decisions fire, each appears on its own line. The model must understand all of them apply, not just the first. | [ ] |
| CL-5 | Non-blocking semantics | The hook injects context; it does not reject the edit. The model should treat the decision as guidance to revise, not a hard failure. | [ ] |

### Common LLM Misinterpretation Risks

- [ ] The model treats a decision match as a build failure and aborts instead of revising — verify wording frames it as a revise-and-continue instruction.
- [ ] The model ignores additional decisions after the first — verify each is listed explicitly.
- [ ] The model edits `decisions.json` to silence the rule instead of fixing the code — document that updating decisions is allowed only when the decision is genuinely outdated.

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for adopting and operating the feature.

**Complexity Score:** Low — one JSON file, one hook entry.

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Lock a decision by hand | 1 step: add an object to `.kaboom/decisions.json` | No — single file edit |
| Have the AI lock a decision | 1 step: ask it to append to `.kaboom/decisions.json` | No — plain JSON, no API |
| Enforce on every edit | 0 steps after install: PostToolUse hook fires automatically | No |
| Expire a temporary rule | 1 step: add `expires` (YYYY-MM-DD) | No |

### Default Behavior Verification

- [ ] With no `.kaboom/decisions.json`, the hook is silent and adds zero tokens.
- [ ] A decision with only a literal `pattern` works without a regex.
- [ ] A decision with `regex` or a `re:`-prefixed `pattern` matches correctly.
- [ ] Expired decisions are skipped automatically.

---

## 4. Code Test Plan

### 4.1 Unit Tests

Source under test: `internal/hook/decision_guard.go`. Tests live in `internal/hook/decision_guard_test.go`.

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Literal pattern match | Edit introduces `require (`; decision pattern `require (` | Output contains the decision ID and `ARCHITECTURAL DECISIONS` | must |
| UT-2 | No match | Edit with unrelated code | Returns nil (no output) | must |
| UT-3 | Regex match via `regex` field | Edit `errors.New("lower")`; regex `errors\.New\("[a-z]` | Matches, output contains decision ID | must |
| UT-4 | Regex match via `re:` prefix | Pattern `re:fmt\.Print` | Matches `fmt.Println(` | must |
| UT-5 | Invalid regex skipped | Decision with malformed regex | No match, no panic | must |
| UT-6 | Expired decision skipped | Decision with `expires` in the past | Returns nil | must |
| UT-7 | Non-expired decision honored | Decision with `expires` in the future | Matches normally | should |
| UT-8 | Non-edit tool ignored | `tool_name: Read` | Returns nil | must |
| UT-9 | Empty new content ignored | Edit with empty `new_string` and no `content` | Returns nil | must |
| UT-10 | Multiple decisions match | Edit matching two decisions | Both IDs present in output | must |
| UT-11 | Only matching decision fires | Two decisions, one matches | Output contains the matching ID, not the other | must |
| UT-12 | Missing decisions file | No `.kaboom/decisions.json` | Returns nil silently | must |
| UT-13 | Malformed decisions JSON | Corrupt file | Returns nil (load returns empty) | must |
| UT-14 | Reason and Enforced rendered | Decision with `reason` and `enforced` | Output includes `Reason:` and `Enforced:` lines | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | End-to-end via binary | `cmd/hooks/main.go:runDecisionGuard` reading stdin, writing stdout | Valid hook JSON envelope emitted on match | must |
| IT-2 | Project root discovery | `FindProjectRoot` walks up from edited file for `.kaboom.json` | Decisions loaded from the correct root | must |
| IT-3 | Agent-aware output | `WriteOutput` adapts envelope for Claude vs Gemini | Claude gets flat `additionalContext`; Gemini gets nested `hookSpecificOutput` | should |
| IT-4 | Eval fixtures | `internal/hook/eval/testdata/decision-guard/` (10 fixtures) | Regression fixtures pass; aspirational fixtures documented | must |

### 4.3 Edge Case Tests

| # | Edge Case | Scenario | Expected Behavior | Priority |
|---|-----------|----------|-------------------|----------|
| EC-1 | Pattern inside a comment | Edit adds `// fmt.Println` | Known gap: literal/regex match fires even in comments. Documented in eval fixture `008-false-positive-in-comment`. | should |
| EC-2 | Pattern inside a string literal | Edit adds `"...fmt.Println..."` | Known gap: matches inside strings. See `009-false-positive-in-string`. | should |
| EC-3 | Test-file exemption | Edit to `*_test.go` with otherwise-banned pattern | Known gap: decisions apply uniformly; no path-based exemption yet. See `010-test-file-exemption`. | should |
| EC-4 | Near-miss literal | `require` without the open paren | No match (literal precision). | should |
| EC-5 | Properly formatted error | `errors.New("Uppercase: ...")` vs lowercase regex | No match — uppercase first char. | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant.

### Prerequisites

- [ ] `kaboom-hooks` binary on PATH (`kaboom-hooks --version`)
- [ ] Project has `.kaboom.json` at the root
- [ ] `.claude/settings.json` registers `kaboom-hooks decision-guard` on the `Edit|Write` matcher
- [ ] `.kaboom/decisions.json` contains at least one decision

### Step-by-Step Verification

| # | Step | Expected Result | Pass |
|---|------|-----------------|------|
| UAT-1 | Add a decision with literal pattern `http.Client{`, then have the AI edit a file introducing `http.Client{` | The AI sees the decision in its context and revises to use the shared client | [ ] |
| UAT-2 | Edit a file that does not match any pattern | No decision context injected | [ ] |
| UAT-3 | Add a decision with a `re:` regex, edit matching code | Decision fires; output shows the ID | [ ] |
| UAT-4 | Add a decision with `expires` set to yesterday, edit matching code | Decision is skipped (no output) | [ ] |
| UAT-5 | Remove `.kaboom/decisions.json`, edit any file | Hook is silent; no errors | [ ] |
| UAT-6 | Add two decisions both matching one edit | Both IDs appear in the injected context | [ ] |

### Regression Checks

- [ ] Other hooks (quality-gate, blast-radius, session-track) still fire on the same Edit/Write.
- [ ] Hook latency stays within the < 15ms budget for a typical edit and decision list.
- [ ] No stdout noise outside the JSON envelope (host agent protocol intact).

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
