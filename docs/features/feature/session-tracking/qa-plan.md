---
doc_type: qa-plan
feature_id: feature-session-tracking
status: proposed
scope: feature/session-tracking/qa
ai-priority: medium
tags: [testing, qa, hooks, session-tracking]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-06-29
last_reviewed: 2026-06-29
---

# QA Plan: Session Tracking

> Verifies the session-tracking PostToolUse hook: it records every file read, edit, write, and command to an append-only session log, detects redundant reads, and injects a session summary on edits. Other hooks read this log for cross-hook awareness. Covers data-leak analysis, agent clarity, simplicity, code-level tests, and step-by-step user acceptance testing (UAT).

---

## 1. Data Leak Analysis

**Goal:** Confirm the session log stays on disk under the user's home directory, holds only short summaries, and is cleaned up over time.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Log location | Verify entries append only to `~/.kaboom/sessions/<id>/touches.jsonl` | high |
| DL-2 | Summary truncation | Verify edit, write, and command summaries are truncated to the summary length limit (100 chars) | medium |
| DL-3 | Line length bound | Verify the reader bounds line length so an oversized entry cannot blow up memory | low |
| DL-4 | Session identifier derivation | Verify the identifier is a short hash of process identifier and working directory, or a truncated agent session identifier — never raw user data | medium |
| DL-5 | No network transmission | Verify the hook writes only to disk and stdout; no network calls | critical |
| DL-6 | Stale-session cleanup | Verify directories older than the stale age are removed, so logs do not accumulate forever | medium |
| DL-7 | Read-only cross-hook queries | Verify other hooks query the log read-only and cannot corrupt it | low |

### Negative Tests (must NOT leak)
- [ ] The log never leaves `~/.kaboom/sessions/`
- [ ] Command summaries are capped, not stored in full
- [ ] The hook performs no network I/O
- [ ] Stale session directories are removed on cleanup

---

## 2. Agent Clarity Assessment

**Goal:** Confirm the injected notices help the agent avoid redundant work without confusing it.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Redundant-read notice | "[Session] You read this file N ago. No edits since." is unambiguous | [ ] |
| CL-2 | Read-then-edited notice | "You read this file N ago. You edited it M ago." clearly conveys both events | [ ] |
| CL-3 | Session summary | "[Session] X files read, Y edited, Z commands." is clear | [ ] |
| CL-4 | Last-test outcome | "Last test: PASS/FAIL (cmd)" is derived from the last Bash summary and is clear | [ ] |
| CL-5 | Silence semantics | The agent understands that no notice means a first read or a non-redundant action | [ ] |

### Common Agent Misinterpretation Risks
- [ ] Agent treats a redundant-read notice as an error rather than an advisory
- [ ] Agent misreads the PASS/FAIL heuristic (substring match on the command summary) as authoritative test results
- [ ] Agent assumes the summary counts only the current file rather than the whole session

---

## 3. Simplicity Assessment

**Goal:** Confirm tracking is automatic and the hook is the foundation other hooks build on.

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Record a tool use | 0 steps: every Read/Edit/Write/Bash is recorded | No — automatic |
| Detect a redundant read | 0 steps: checked before recording | No — automatic |
| Provide cross-hook data | 0 steps: log is read-only queryable | No — shared package |

### Default Behavior Verification
- [ ] Every recognized tool use is recorded, even when nothing is injected
- [ ] Redundant-read detection runs before the new touch is appended
- [ ] Edits and writes trigger a session summary when entries exist

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Action classification | Read/Edit/Write/Bash and Gemini equivalents | Mapped to read/edit/write/bash | must |
| UT-2 | Append touch | One Edit | A JSONL line appended to `touches.jsonl` | must |
| UT-3 | Redundant read detection | Read a file already read | Redundant-read notice returned | must |
| UT-4 | Read-then-edit notice | Read after an intervening edit | Notice includes the edit time | must |
| UT-5 | First read is silent | Read a never-seen file | Returns nil, but records the touch | must |
| UT-6 | Summary on edit | Edit with prior entries | Session summary returned | must |
| UT-7 | Summary counts | Mixed reads/edits/commands | Correct read/edit/command counts | must |
| UT-8 | Last-test PASS | Last Bash summary contains "pass" | "Last test: PASS" appended | should |
| UT-9 | Last-test FAIL | Last Bash summary contains "fail" | "Last test: FAIL" appended | should |
| UT-10 | Summary truncation | Long `new_string` | Truncated to the summary limit | must |
| UT-11 | Session identifier from env | `GEMINI_SESSION_ID` set | Truncated agent identifier used | must |
| UT-12 | Session identifier fallback | No env identifier | Hash of process identifier + cwd | must |
| UT-13 | FilesEdited dedup | Same file edited twice | Listed once | should |
| UT-14 | WasFileEdited since | Edit after a given time | Returns true with the timestamp | must |
| UT-15 | Stale cleanup | Meta older than the stale age | Directory removed | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Full record-and-inject cycle | Hook -> session store | Touch recorded; correct notice injected | must |
| IT-2 | Cross-hook read | blast-radius reads the log | Already-read importers detected | must |
| IT-3 | Concurrent appends | Two hook processes append | Both lines present; no corruption (append-only) | must |
| IT-4 | Meta written on first access | First `SessionDir` call | `meta.json` created with start time, cwd, ppid | should |
| IT-5 | Newest-first ordering | Multiple entries | `ReadTouches` returns newest first | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Append entry | Single `O_APPEND` write | < 2ms | must |
| PT-2 | Read all touches | Scan the JSONL log | < 5ms | should |
| PT-3 | Session identifier | Hash of ppid + cwd | < 1ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Scenario | Expected Behavior | Priority |
|---|-----------|----------|-------------------|----------|
| EC-1 | Missing log | No `touches.jsonl` yet | Reads return empty, no error | must |
| EC-2 | Malformed line | A corrupt JSONL entry | Skipped during scan; others still parse | must |
| EC-3 | Bash with no file | `run_shell_command` | Recorded with empty file, command summary kept | should |
| EC-4 | Empty session | No entries | Summary returns empty string | must |
| EC-5 | I/O failure on append | Unwritable directory | Hook degrades silently (records nothing) | should |

---

## 5. UAT Checklist (Human + AI)

> The AI reads, edits, and runs commands; the human confirms the injected notices.

### Prerequisites
- [ ] `kaboom-hooks` installed and wired as a PostToolUse hook on Read, Edit, Write, and Bash
- [ ] A project the AI can read and edit

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | Read a file for the first time | Hook output | No notice; touch recorded | [ ] |
| UAT-2 | Read the same file again | Hook output | "You read this file N ago. No edits since." | [ ] |
| UAT-3 | Edit then re-read a file | Hook output | Notice mentions both read and edit times | [ ] |
| UAT-4 | Edit a file mid-session | Hook output | Session summary with read/edit/command counts | [ ] |
| UAT-5 | Run a passing test, then edit | Hook output | Summary appends "Last test: PASS" | [ ] |
| UAT-6 | Run a failing test, then edit | Hook output | Summary appends "Last test: FAIL" | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Log stays local | Locate `touches.jsonl` | Under `~/.kaboom/sessions/<id>/` | [ ] |
| DL-UAT-2 | Summaries truncated | Inspect log entries | None exceed the summary limit | [ ] |
| DL-UAT-3 | No network traffic | Monitor during tool use | No outbound requests | [ ] |

### Regression Checks
- [ ] blast-radius and decision-guard still read the session log correctly
- [ ] Recording continues even when nothing is injected
- [ ] Stale sessions are cleaned up without affecting the active session

---

## Sign-Off

| Area | Tester | Date | Pass/Fail |
|------|--------|------|-----------|
| Data Leak Analysis | | | |
| Agent Clarity | | | |
| Simplicity | | | |
| Code Tests | | | |
| UAT | | | |
| **Overall** | | | |
