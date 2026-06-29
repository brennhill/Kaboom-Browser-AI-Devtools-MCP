---
doc_type: qa-plan
feature_id: feature-blast-radius
status: proposed
scope: feature/blast-radius/qa
ai-priority: medium
tags: [testing, qa, hooks, blast-radius]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-06-29
last_reviewed: 2026-06-29
---

# QA Plan: Blast Radius

> Verifies the blast-radius PostToolUse hook: when the artificial intelligence (AI) edits a file that exports symbols, it scans the project's reverse import graph and injects a warning listing the files that import the edited file, highlighting those already read this session. Covers data-leak analysis, agent clarity, simplicity, code-level tests, and step-by-step user acceptance testing (UAT).

---

## 1. Data Leak Analysis

**Goal:** Confirm the hook reads only local project source to build the import graph, writes only to the session directory, and never transmits anything off the machine.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Cached graph location | Verify the import graph is written only to the session directory as `import_graph.json` | high |
| DL-2 | File-content exposure | Verify the hook extracts import paths only; it never includes file bodies in the injected context | high |
| DL-3 | Paths outside the project | Verify edits whose path resolves outside the project root (`..` prefix) are ignored, never scanned | medium |
| DL-4 | Large-file handling | Verify files above the scan size limit are skipped, avoiding accidental capture of generated or vendored blobs | low |
| DL-5 | No network transmission | Verify the hook makes no network calls; output goes only to stdout as `additionalContext` | critical |
| DL-6 | Session annotation source | Verify in-session highlighting reads only the local session touch log, never external state | medium |

### Negative Tests (must NOT leak)
- [ ] The injected context contains import-path warnings only, never file contents
- [ ] Edits resolving outside the project root produce no output
- [ ] The hook performs no network I/O
- [ ] The graph cache stays inside the session directory

---

## 2. Agent Clarity Assessment

**Goal:** Confirm the injected warning is unambiguous and actionable for an AI agent.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Graduated severity | The agent distinguishes plain (1-5 importers), WARNING (6-15), and CRITICAL (16+) bands | [ ] |
| CL-2 | In-session vs not-yet-reviewed | "Already in session (already read)" is clearly separate from "Not yet reviewed" | [ ] |
| CL-3 | Truncation marker | "... and N more" clearly signals the list was capped at ten per group | [ ] |
| CL-4 | Relative paths | Importer paths are project-relative and resolvable by the agent | [ ] |
| CL-5 | Silence on internal edits | The agent understands that no warning means the edit did not touch an exported symbol | [ ] |

### Common Agent Misinterpretation Risks
- [ ] Agent reads the importer list as files it must edit, rather than files to verify for compatibility
- [ ] Agent assumes the absence of a warning means there are no dependents (it may mean an internal-only edit)
- [ ] Agent ignores the CRITICAL band on wide-impact modules

---

## 3. Simplicity Assessment

**Goal:** Confirm the hook is fully automatic and requires no agent action.

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Get blast-radius warning on edit | 0 steps: fires automatically on Edit/Write | No — fully automatic |
| Highlight already-visited dependents | 0 steps: reads the session log if present | No — automatic when session tracking is installed |
| Run standalone (no session tracking) | 0 steps: degrades gracefully | No — works without session tracking |

### Default Behavior Verification
- [ ] The hook fires only on edit-class tools (Edit, Write, and the Gemini equivalents)
- [ ] Internal-only edits (no exported symbol touched) produce no noise
- [ ] Without session tracking, all importers list as not-yet-reviewed

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Non-edit tool ignored | `tool_name` = "Read" | Returns nil (no warning) | must |
| UT-2 | Edit-tool detection | "Edit", "Write", "write_file", "replace_in_file", "edit_file" | All treated as edits | must |
| UT-3 | Path outside project | file resolves to `../foo` | Returns nil | must |
| UT-4 | Internal-only Go edit | `new_string` with no exported symbol | Returns nil (looksExported false) | must |
| UT-5 | Exported Go symbol | `new_string` with `func Exported` | Proceeds to graph lookup | must |
| UT-6 | Exported TS symbol | `new_string` with `export function` | Proceeds | must |
| UT-7 | Exported Python symbol | `def public` (not `def _private`) | Proceeds; dunder/underscore skipped | must |
| UT-8 | Exported Rust symbol | `pub fn` | Proceeds | should |
| UT-9 | No importers | Edited file has zero dependents | Returns nil | must |
| UT-10 | Plain band | 3 importers | "imported by 3 file(s)" | must |
| UT-11 | WARNING band | 8 importers | "WARNING: ... imported by 8 files" | must |
| UT-12 | CRITICAL band | 20 importers | "CRITICAL: ... imported by 20 files" | must |
| UT-13 | In-session highlighting | Some importers already read | Split into "Already in session" / "Not yet reviewed" | must |
| UT-14 | Importer cap | 15 not-yet-reviewed | First ten shown, then "... and 5 more" | must |
| UT-15 | Cache freshness | `import_graph.json` older than five minutes | Rebuilt, not reused | should |
| UT-16 | File-count cap | Project with more than the scan limit | Walk stops at the cap | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Build and cache graph | Walk -> extract imports -> save cache | Reverse edges built; cache written to session dir | must |
| IT-2 | Reuse warm cache | Second invocation within five minutes | Loads cache, no rebuild | must |
| IT-3 | Go module resolution | Local module imports | Resolved to package `.go` files (excluding `_test.go`) | must |
| IT-4 | TS relative resolution | `./` and `../` imports | Resolved through index and extension fallbacks; bare specifiers skipped | must |
| IT-5 | Cross-hook session read | Session tracking installed | Already-read importers highlighted | should |
| IT-6 | Standalone operation | No session directory | All importers list as not-yet-reviewed | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Warm lookup | Cached graph load + lookup | < 50ms | must |
| PT-2 | Cold build | Walk + regex scan of the project | < 250ms | must |
| PT-3 | Export detection | Regex match on `new_string` | < 5ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Scenario | Expected Behavior | Priority |
|---|-----------|----------|-------------------|----------|
| EC-1 | Empty new_string | Cannot tell if exported | Assume exported, proceed | should |
| EC-2 | Unknown language | Unsupported extension | Assume exported, proceed | should |
| EC-3 | Skipped directories | `.git`, `node_modules`, `vendor`, hidden | Excluded from the walk | must |
| EC-4 | Corrupt cache file | Unparseable `import_graph.json` | Rebuilt from scratch | should |
| EC-5 | Missing project root | Empty project root | Returns nil | must |

---

## 5. UAT Checklist (Human + AI)

> The AI edits files; the human confirms the injected warning content.

### Prerequisites
- [ ] `kaboom-hooks` installed and wired as a PostToolUse hook on Edit and Write
- [ ] A multi-file project where several files import a shared module
- [ ] Optionally, session tracking installed for in-session highlighting

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | Edit an exported function in a widely imported module | Hook output | Warning lists importer files with the correct band | [ ] |
| UAT-2 | Edit an internal (unexported) helper | Hook output | No warning injected | [ ] |
| UAT-3 | Edit a module imported by 6+ files | Hook output | WARNING band header | [ ] |
| UAT-4 | Edit a module imported by 16+ files | Hook output | CRITICAL band header | [ ] |
| UAT-5 | Read one importer, then edit the module | Hook output | That importer shown under "Already in session" | [ ] |
| UAT-6 | Edit a module with 15+ importers | Hook output | First ten shown, then "... and N more" | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | No file contents leaked | Inspect the injected context | Import-path warnings only | [ ] |
| DL-UAT-2 | Cache stays local | Locate `import_graph.json` | Inside the session directory only | [ ] |
| DL-UAT-3 | No network traffic | Monitor during edits | No outbound requests | [ ] |

### Regression Checks
- [ ] Read, Bash, and other non-edit tools never trigger a warning
- [ ] Session-tracking output is unaffected by blast-radius
- [ ] The hook completes within its timeout on a large project

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
