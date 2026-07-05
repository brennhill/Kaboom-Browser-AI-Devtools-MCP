---
status: proposed
scope: feature/workflow-integration/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-workflow-integration
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Workflow Integration

> QA plan for the Workflow Integration feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Workflow Integration generates session summaries, PR annotations, and git hook output. These artifacts flow into PR descriptions, commit messages, and persistent files on disk -- all of which may be visible to team members, CI systems, and public repositories.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | PR summary exposes sensitive URLs | Verify that `generate_pr_summary` markdown output does not include full URLs with query parameters containing API keys, tokens, or session IDs | critical |
| DL-2 | Session summary file on disk contains secrets | Verify that `.kaboom/sessions/latest.json` and archive files do not contain raw network body content, auth headers, or credentials | critical |
| DL-3 | Error messages in PR summary expose internal paths | Verify that error messages in the "Errors" section of the PR summary (e.g., `TypeError at dashboard.js:142`) do not expose absolute server-side file paths or database connection strings | high |
| DL-4 | Resource change list reveals internal infrastructure | Verify that the "Resource Changes" section (`Added: chart-library.js`) does not expose internal package names, private registry URLs, or proprietary library names that should not be public | medium |
| DL-5 | Git hook one-liner leaks performance data to public repos | Verify that the one-liner format (`[perf: +200ms load, +45KB bundle]`) appended to commit messages does not reveal sensitive performance baselines that competitors could use | low |
| DL-6 | HTTP endpoint `/v4/session-summary` accessible beyond localhost | Verify that the endpoint is bound to 127.0.0.1 only and not accessible from external network interfaces | critical |
| DL-7 | Session archive files accumulate sensitive data over time | Verify that the 20-entry archive cap (FIFO) properly deletes old session files and that deleted files are not recoverable from the `.kaboom/` directory | medium |
| DL-8 | Git context in session metadata exposes branch names with secrets | Verify that `git_branch` and `git_commit` metadata captured in session summaries do not expose branch names that contain secrets (e.g., `fix/api-key-ABC123`) | medium |
| DL-9 | Bundle size analysis reveals internal dependency graph | Verify that resource additions/removals in the summary do not reveal the full dependency tree or internal package versions | low |
| DL-10 | Concurrent session summaries leak cross-session data | Verify that two concurrent server instances produce independent summaries and do not merge data from different sessions | medium |

### Negative Tests (must NOT leak)
- [ ] Auth header values must not appear in PR summary markdown
- [ ] API keys in URL query strings must not appear in session summary JSON files
- [ ] `.kaboom/sessions/latest.json` must not contain raw request/response bodies with tokens
- [ ] The `/v4/session-summary` endpoint must reject connections from non-localhost origins
- [ ] Deleted archive files (beyond 20-entry cap) must not remain on disk
- [ ] Error stack traces in PR summaries must not include server-side file system paths
- [ ] Git hook output must not include full URLs -- only relative paths and metric deltas

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Performance delta direction clarity | Verify that `+200ms (+17%)` clearly indicates degradation (slower) and `-200ms (-17%)` indicates improvement (faster), not ambiguous "change" | [ ] |
| CL-2 | Bundle size delta meaning | Verify that `+45KB (+13%)` clearly means the bundle grew, and the AI understands this is typically a regression unless intentional (new feature added) | [ ] |
| CL-3 | Error "fixed" vs "new" vs "persistent" | Verify that the three error categories are unambiguous: "fixed" = present at start, gone at end; "new" = not at start, appeared during session; "persistent" = present throughout | [ ] |
| CL-4 | A11y "new violations" vs "existing" | Verify the AI can distinguish new accessibility issues introduced during the session from pre-existing ones that were already present | [ ] |
| CL-5 | "No performance data collected" meaning | Verify the AI understands this means no snapshots were taken (not that performance is zero or perfect), and suggests the developer reload the page | [ ] |
| CL-6 | "Insufficient data" for short sessions | Verify that the AI understands "insufficient data (< 2 snapshots)" means the comparison is unreliable, not that there were no issues | [ ] |
| CL-7 | Per-URL vs session-wide deltas | Verify that when multiple URLs are reported, the AI understands each row is a per-URL delta, not an aggregate of all URLs | [ ] |
| CL-8 | Transfer size vs decoded size | Verify that "Bundle Size" is clearly labeled as either `transferSize` or `decodedSize` so the AI reports the correct metric to the developer | [ ] |
| CL-9 | One-liner suppression logic | Verify that when the one-liner has no meaningful data (`[perf: - | errors: - | a11y: -]`), the output is empty or clearly marked as "no changes to report" | [ ] |
| CL-10 | Session metadata interpretation | Verify that "12 performance samples across 8 page loads" is clear (12 snapshots taken, 8 distinct page navigations) and the AI does not confuse samples with page loads | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI might interpret positive delta (+200ms) as improvement rather than degradation -- verify the sign convention is explicit
- [ ] AI might report "Bundle Size +45KB" without context that 45KB is net of additions and removals -- verify net calculation is shown
- [ ] AI might claim "errors fixed" when errors simply stopped occurring (page not loaded, not actually fixed) -- verify the fix designation requires the same page to be observed error-free
- [ ] AI might interpret empty a11y section as "all a11y checks passed" when it actually means "no a11y checks were run" -- verify the distinction
- [ ] AI might use the first-vs-last snapshot delta without realizing the first snapshot is a cold cache load -- verify cold-cache warnings

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Generate PR summary | 1 step: `generate_pr_summary` (no parameters) | No -- already zero-config |
| View current session summary | 1 step: `GET /v4/session-summary` | No -- already minimal |
| Include summary in PR | 2 steps: (1) call `generate_pr_summary`, (2) paste into `gh pr create --body` | No -- the two steps are inherently separate (generate, then use) |
| Configure git hook | 3 steps: (1) create hook script, (2) make executable, (3) verify it works | Yes -- could provide `kaboom install-hook` CLI command |
| Load previous session context | 1 step: `load_session_context` | No -- already minimal |
| View session archive history | 1 step: read `.kaboom/sessions/archive/` directory | Yes -- could provide MCP tool for session history query |

### Default Behavior Verification
- [ ] Session summary generates automatically on server shutdown (no explicit call needed)
- [ ] PR summary includes all sections (performance, errors, a11y, resources) by default
- [ ] One-liner format is generated alongside full summary automatically
- [ ] Archive cap at 20 entries applies without configuration
- [ ] Previous session summary is loadable via `load_session_context` without explicit save step
- [ ] Running summary available via HTTP endpoint during active session without explicit activation

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Session with 2 snapshots produces correct delta | Snapshot 1: load 1200ms, LCP 800ms; Snapshot 2: load 1400ms, LCP 900ms | Delta: +200ms (+17%) load, +100ms (+13%) LCP | must |
| UT-2 | Session with resolved error | Error "TypeError" at start, absent at end | Summary lists error under "fixed" category | must |
| UT-3 | Session with new error | No errors at start, "ReferenceError" appears | Summary lists error under "new" category | must |
| UT-4 | Session with persistent error | Error present at start and end | Summary lists error under "persistent" category | must |
| UT-5 | New a11y violation detection | No violations at start, color contrast violation appears | Summary includes violation under "new_violations" | must |
| UT-6 | PR summary produces valid markdown table | 2 performance snapshots with errors | Output contains valid markdown table with correct column alignment | must |
| UT-7 | One-liner format correctness | Load +200ms, bundle +45KB, 1 error fixed | Output: `[perf: +200ms load, +45KB bundle | errors: -1 fixed | a11y: clean]` | must |
| UT-8 | One-liner suppression when no changes | No performance changes, no errors, no a11y | Output is empty string or omitted | should |
| UT-9 | No snapshots returns "no performance data" | Session with 0 snapshots | Summary says "No performance data collected" | must |
| UT-10 | Single snapshot returns "insufficient data" | Session with 1 snapshot | Summary marked as "insufficient data" | must |
| UT-11 | Multiple URLs: top 5 by reload count | 8 URLs, varying reload counts | Summary includes top 5 URLs sorted by reload count | must |
| UT-12 | URLs with 1 snapshot excluded | URL visited once (1 snapshot) | URL excluded from delta report (no delta possible) | must |
| UT-13 | Shutdown generates `.kaboom/sessions/latest.json` | Server clean shutdown | File exists with valid JSON session summary | must |
| UT-14 | Archive capped at 20 entries | 25 sessions completed | Only most recent 20 exist in archive directory | must |
| UT-15 | Resource changes: added/removed detection | Script `chart-library.js` added, `legacy-charts.js` removed | Summary shows both under correct categories with byte sizes | must |
| UT-16 | Bundle size net calculation | Added 98KB + 12KB, removed 65KB | Net: +45KB | must |
| UT-17 | HTTP endpoint returns in-progress summary | Active session, call `GET /v4/session-summary` | Returns current accumulated summary, not empty | must |
| UT-18 | Session duration and reload count | Session runs 45 minutes, 8 page reloads | `duration_s: 2700`, `reload_count: 8` | must |
| UT-19 | Previous session loaded via `load_session_context` | Previous session saved in `latest.json` | Returns the saved session summary with all fields | should |
| UT-20 | Format parameter: JSON output | `generate_pr_summary` with `format: "json"` | Returns structured JSON object (not markdown string) | should |
| UT-21 | Format parameter: markdown output | `generate_pr_summary` with `format: "markdown"` | Returns markdown-formatted string | should |
| UT-22 | Format parameter: one_liner output | `generate_pr_summary` with `format: "one_liner"` | Returns one-liner string | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | End-to-end: capture -> shutdown -> summary | Extension capture + server shutdown + file write | Session summary file written with correct deltas from captured performance snapshots | must |
| IT-2 | PR summary uses same data as session summary | `generate_pr_summary` + `.kaboom/sessions/latest.json` | Both contain identical performance deltas, error counts, and resource changes | must |
| IT-3 | HTTP endpoint consistency with MCP tool | `GET /v4/session-summary` + `generate_pr_summary` MCP call | Both return the same data (one as HTTP JSON, one as MCP response) | must |
| IT-4 | Archive rotation | Generate 25 sessions sequentially | Archive contains exactly 20 files, oldest 5 deleted | must |
| IT-5 | Git hook integration | `prepare-commit-msg` hook calls `/v4/session-summary` | Commit message has one-liner appended correctly | should |
| IT-6 | Session summary survives partial crash | Kill server with SIGKILL after 5 minutes of capture | Periodic flush ensures `.kaboom/sessions/latest.json` has data (at most 60s stale) | should |
| IT-7 | Concurrent server instances | Two servers running, both shut down | Each produces its own summary in archive with unique session ID filename | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Session summary generation speed | Wall clock time for summary computation | < 50ms | must |
| PT-2 | PR summary markdown generation | Wall clock time for `generate_pr_summary` | < 10ms | must |
| PT-3 | HTTP endpoint response time | Time for `GET /v4/session-summary` | < 20ms | must |
| PT-4 | Archive total storage | Disk space for 20 archived sessions | < 500KB | must |
| PT-5 | Periodic flush overhead | CPU overhead of 60-second flush goroutine | < 1ms per flush | should |
| PT-6 | Summary generation with 100+ errors | Summary computation with 100 error entries | < 100ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | No performance snapshots in session | Server started, no pages loaded | Summary reports "No performance data collected", no delta table | must |
| EC-2 | Server crash (no clean shutdown) | Server killed with SIGKILL | Periodic flush saved most recent data; next session starts fresh | must |
| EC-3 | Very short session (< 10 seconds) | Server started and stopped within 10 seconds, 1 snapshot | Summary marked "insufficient data" | must |
| EC-4 | Multiple URLs observed | Developer navigates to 8 different pages | Top 5 URLs by reload count included in summary | must |
| EC-5 | Agent never calls generate_pr_summary | Session ends without PR summary request | Session summary still saved to disk, available next session | must |
| EC-6 | Concurrent sessions (two server instances) | Two servers write summaries simultaneously | Each writes unique session ID file, no data corruption | should |
| EC-7 | First snapshot is cold load | First page load has empty cache, subsequent are cached | Delta excludes cold-load bias (uses second snapshot as baseline) | should |
| EC-8 | Very large session (thousands of snapshots) | 8-hour development session with many reloads | Summary generation still under 50ms due to incremental computation | should |
| EC-9 | `.kaboom/sessions/` directory does not exist | Fresh project, no previous sessions | Directory created on first summary write | must |
| EC-10 | Disk full during summary write | Disk has no free space | Write fails gracefully, server does not crash, error logged | should |
| EC-11 | Session summary with all zeros | All metrics identical between first and last snapshot | Delta shows "0ms (0%)" for all metrics, one-liner suppressed | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web application loaded and tracked (e.g., `http://localhost:3000`)
- [ ] Application has been loaded at least twice to produce performance snapshots

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | Human loads app page, AI captures performance: `{"tool":"observe","arguments":{"what":"performance"}}` | Page loaded in browser | Performance snapshot captured with load time, LCP, CLS values | [ ] |
| UAT-2 | Human makes a code change that increases bundle size (add a large library), reloads page | Browser shows updated app | New performance snapshot captured | [ ] |
| UAT-3 | AI generates PR summary: `{"tool":"generate","arguments":{"type":"pr_summary"}}` | N/A | Markdown table showing performance delta (increased load time, bundle size) | [ ] |
| UAT-4 | AI verifies summary contains all sections | Read returned markdown | Performance Impact table, Resource Changes, Errors, Accessibility sections all present | [ ] |
| UAT-5 | Human introduces a console error, reloads page | Error visible in DevTools | Error captured by Kaboom | [ ] |
| UAT-6 | AI generates PR summary again | N/A | Summary now includes the new error under "New" errors section | [ ] |
| UAT-7 | Human fixes the error, reloads page | Error no longer in DevTools | Error resolved | [ ] |
| UAT-8 | AI generates PR summary again | N/A | Summary shows the error under "Fixed" errors section | [ ] |
| UAT-9 | AI queries HTTP endpoint: `GET http://127.0.0.1:7890/v4/session-summary` | N/A | JSON response with running summary matching MCP tool output | [ ] |
| UAT-10 | Human gracefully stops server (Ctrl+C) | Server exits cleanly | `.kaboom/sessions/latest.json` file created with full session summary | [ ] |
| UAT-11 | Human restarts server, AI loads previous context: `{"tool":"configure","arguments":{"action":"load_session_context"}}` | Server running again | Previous session summary loaded and available | [ ] |
| UAT-12 | Verify archive file | Check `.kaboom/sessions/archive/` | At least one timestamped JSON file present | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | PR summary does not contain auth headers | Make authenticated API calls, generate PR summary | No Authorization/Cookie header values in markdown | [ ] |
| DL-UAT-2 | Session JSON file has no raw bodies | Inspect `.kaboom/sessions/latest.json` | No request/response body content with secrets | [ ] |
| DL-UAT-3 | HTTP endpoint rejects external access | Attempt `curl http://<machine-external-ip>:7890/v4/session-summary` | Connection refused or no response | [ ] |
| DL-UAT-4 | Error messages are sanitized | Trigger error with file path, check PR summary | Error message included as-is (console output), but no additional server-side paths added by Kaboom | [ ] |

### Regression Checks
- [ ] Existing `observe` tools work independently of workflow integration
- [ ] Existing `generate` tools (reproduction, test, HAR, SARIF) are unaffected
- [ ] Server shutdown behavior is unchanged (summary is additive, not replacing existing shutdown logic)
- [ ] Extension capture performance is not degraded by incremental summary computation
- [ ] MCP tool response schema is unchanged for all existing tools

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
