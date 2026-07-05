---
feature: analyze-tool
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# `analyze` Tool — User Acceptance Testing Guide

## Overview

This guide provides step-by-step instructions for human testers to verify the `analyze` tool works correctly in real-world development workflows.

**Estimated Time:** 45-60 minutes

### Required Setup:
- Kaboom extension installed
- Kaboom server running
- Claude or MCP-compatible AI client
- Test pages with known issues

---

## Prerequisites & Setup

### Environment Checklist

- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and shows "Connected" in popup
- [ ] AI Web Pilot toggle is ON
- [ ] MCP client (Claude Desktop, Cursor, etc.) connected
- [ ] Test page with known accessibility issues loaded

### Test Pages Needed

1. **Page with accessibility issues:**
   - Missing alt text on images
   - Low contrast text
   - Missing form labels

2. **Page with security issues:**
   - Missing CSP header
   - Cookies without Secure flag
   - Sensitive-looking localStorage keys

3. **Simple clean page:** about:blank or minimal HTML

4. **Complex page:** Amazon.com, GitHub.com, or other heavy site

---

## Happy Path Tests

### HP-1: Basic Accessibility Audit

**Objective:** Verify AI can request and receive accessibility findings

#### Steps:
1. Navigate to a page with known accessibility issues
2. Ask AI: "Check this page for accessibility issues"

#### Expected AI Actions:
- AI calls `analyze({action: 'audit', scope: 'accessibility'})`

#### Verification:
- [ ] Response arrives within 15 seconds
- [ ] Response includes `findings` array
- [ ] Each finding has `severity`, `category`, `message`
- [ ] Each finding has `affected` elements with selectors
- [ ] Summary shows counts by severity
- [ ] AI can describe the issues in natural language

---

### HP-2: Scoped Accessibility Audit

**Objective:** Verify selector scoping works

#### Steps:
1. Navigate to a page with a form element
2. Ask AI: "Check just the signup form for accessibility"

#### Expected AI Actions:
- AI calls `analyze({action: 'audit', scope: 'accessibility', selector: 'form'})`

#### Verification:
- [ ] Response only includes issues within the form
- [ ] Issues outside the form are NOT reported
- [ ] Duration is faster than full-page audit

---

### HP-3: Security Headers Audit

**Objective:** Verify security header checking works

#### Steps:
1. Navigate to a page without CSP (most dev servers)
2. Ask AI: "Check this page for security issues"

#### Expected AI Actions:
- AI calls `analyze({action: 'security'})`

#### Verification:
- [ ] Response includes `findings` for missing headers
- [ ] CSP missing is flagged as high severity
- [ ] Cookie audit included

---

### HP-4: Security Storage Audit

**Objective:** Verify localStorage/sessionStorage audit

#### Steps:
1. Open DevTools on any page
2. Add: `localStorage.setItem('auth_token', 'secret123')`
3. Ask AI: "Check this page for security issues"

#### Verification:
- [ ] Response flags `auth_token` as sensitive key
- [ ] Response does NOT include the value `secret123`
- [ ] Guidance suggests using httpOnly cookies

---

### HP-5: Memory Snapshot

**Objective:** Verify memory analysis works (Chrome only)

#### Steps:
1. Navigate to any page
2. Ask AI: "Take a memory snapshot of this page"

#### Expected AI Actions:
- AI calls `analyze({action: 'memory', scope: 'snapshot'})`

#### Verification:
- [ ] Response includes `heap_used_mb` and `heap_total_mb`
- [ ] Response includes `dom_node_count`
- [ ] Response includes `detached_nodes` count
- [ ] Response includes timestamp

---

### HP-6: Memory Comparison

**Objective:** Verify memory leak detection workflow

#### Steps:
1. Navigate to a SPA (e.g., React app)
2. Ask AI: "Take a memory baseline"
3. Perform actions that might leak memory (open/close modals, navigate routes)
4. Ask AI: "Check for memory leaks"

#### Expected AI Actions:
- First: `analyze({action: 'memory', scope: 'snapshot'})` (baseline)
- Second: `analyze({action: 'memory', scope: 'compare'})`

#### Verification:
- [ ] Compare response shows `baseline` and `current` states
- [ ] Delta values calculated (heap_growth_mb, heap_growth_pct)
- [ ] Detached nodes delta shown
- [ ] If growth significant, findings include guidance

---

### HP-7: Regression Baseline

**Objective:** Verify regression baseline capture

#### Steps:
1. Navigate to your app's main page
2. Ask AI: "Save a baseline of this page's state"

#### Expected AI Actions:
- AI calls `analyze({action: 'regression', scope: 'baseline'})`

#### Verification:
- [ ] Response confirms baseline captured
- [ ] Response includes timestamp and URL

---

### HP-8: Regression Comparison

**Objective:** Verify regression detection

#### Steps:
1. After HP-7, make a change to the page
2. Ask AI: "Check for regressions against the baseline"

#### Expected AI Actions:
- AI calls `analyze({action: 'regression', scope: 'compare'})`

#### Verification:
- [ ] Response compares accessibility, performance, security
- [ ] New issues flagged as regressions
- [ ] Performance deltas calculated
- [ ] Clear verdict: "regression_detected" or "no_regression"

---

### HP-9: Force Refresh Bypass Cache

**Objective:** Verify cache bypass works

#### Steps:
1. Run an accessibility audit
2. Within 10 seconds, ask AI: "Audit accessibility again, fresh"

#### Expected AI Actions:
- AI calls `analyze({action: 'audit', scope: 'accessibility', force_refresh: true})`

#### Verification:
- [ ] New audit runs (check duration is > 1s)
- [ ] `cached: false` in response

---

## Error Path Tests

### EP-1: AI Web Pilot Disabled

**Objective:** Verify correct error when pilot is OFF

#### Steps:
1. Turn OFF AI Web Pilot toggle in extension popup
2. Ask AI to run any analysis

#### Verification:
- [ ] AI receives error: `ai_web_pilot_disabled`
- [ ] AI explains the error to the user
- [ ] Turn toggle back ON, analysis works

---

### EP-2: Analysis Timeout

**Objective:** Verify timeout handling on very complex pages

#### Steps:
1. Navigate to an extremely complex page
2. Request a full audit

#### Verification:
- [ ] If timeout occurs, response includes `status: partial` or error
- [ ] Warning indicates timeout
- [ ] Server does not hang

---

### EP-3: Invalid Selector

**Objective:** Verify invalid selector handling

#### Steps:
1. Ask AI: "Check accessibility of element with selector '[[[invalid'"

#### Verification:
- [ ] Error response with `error: invalid_selector`
- [ ] No crash or hang

---

### EP-4: Selector Matches Nothing

**Objective:** Verify empty results handling

#### Steps:
1. Ask AI: "Check accessibility of #nonexistent-element-xyz"

#### Verification:
- [ ] Audit completes successfully
- [ ] Summary shows all zeros (no issues found)

---

### EP-5: Memory API Unavailable

**Objective:** Verify graceful degradation on non-Chrome browsers

#### Steps:
1. In Firefox (or simulate), request memory analysis

#### Verification:
- [ ] Error: `memory_api_unavailable`
- [ ] Message explains browser limitation

---

### EP-6: No Regression Baseline

**Objective:** Verify compare without baseline handling

#### Steps:
1. Clear any existing baselines
2. Ask AI: "Check for regressions"

#### Verification:
- [ ] Error: `no_baseline`
- [ ] AI suggests capturing a baseline first

---

### EP-7: Extension Not Connected

**Objective:** Verify extension timeout handling

#### Steps:
1. Disable the extension via chrome://extensions
2. Ask AI to run analysis

#### Verification:
- [ ] Error: `extension_timeout` (after ~10s)
- [ ] Re-enable extension, analysis works

---

### EP-8: CSP Blocking (Tier 1 Failure)

**Objective:** Verify CSP fallback behavior

#### Steps:
1. Navigate to a page with strict CSP
2. Ensure DevTools is CLOSED
3. Request accessibility audit

#### Verification:
- [ ] Either audit succeeds (via Tier 2 fallback)
- [ ] Or clear error explaining CSP blocked

---

### EP-9: CSP + DevTools Open

**Objective:** Verify error when Tier 2 cannot be used

#### Steps:
1. Navigate to page with strict CSP
2. Open Chrome DevTools
3. Request accessibility audit

#### Verification:
- [ ] Error: `axe_csp_blocked_devtools_open`
- [ ] Message explains DevTools conflict

---

## Edge Case Tests

### EC-1: Very Large DOM

**Objective:** Verify handling of complex pages

#### Steps:
1. Navigate to a very heavy page (Amazon product page)
2. Request accessibility audit

#### Verification:
- [ ] Audit completes (may take up to 15-30s)
- [ ] No browser crash or hang
- [ ] Main thread stays responsive

---

### EC-2: Large localStorage

**Objective:** Verify handling of large storage

#### Steps:
1. Populate localStorage with 5MB of data:
   ```javascript
   localStorage.setItem('big', 'x'.repeat(5*1024*1024))
   ```
2. Request security audit

#### Verification:
- [ ] Audit completes
- [ ] Response includes localStorage key names only

---

### EC-3: Page Navigation During Analysis

**Objective:** Verify partial results on navigation

#### Steps:
1. Request a full audit (takes several seconds)
2. Quickly navigate to another page before completion

#### Verification:
- [ ] Response indicates `status: partial`
- [ ] Warning: `page_navigated`

---

### EC-4: Tab Closed During Analysis

**Objective:** Verify tab closure handling

#### Steps:
1. Request analysis
2. Close the tab before completion

#### Verification:
- [ ] Error returned (not hang)
- [ ] Server continues operating

---

### EC-5: Rapid Successive Audits

**Objective:** Verify concurrent request handling

#### Steps:
1. Rapidly request 5 audits in quick succession

#### Verification:
- [ ] First audit completes
- [ ] Subsequent audits either complete or queue
- [ ] No memory leak (check extension memory)

---

### EC-6: About:blank Page

**Objective:** Verify minimal page handling

#### Steps:
1. Navigate to about:blank
2. Request accessibility audit

#### Verification:
- [ ] Audit completes quickly
- [ ] Summary shows minimal/zero issues

---

### EC-7: Cached Result Verification

**Objective:** Verify caching works correctly

#### Steps:
1. Request accessibility audit
2. Note the duration
3. Immediately request same audit again

#### Verification:
- [ ] Second request returns much faster (<100ms)
- [ ] Results are identical

---

## Performance Validation

### PV-1: Audit Timing Benchmark

#### Test Matrix:

| Page Type | Expected Time | Actual Time | Pass? |
|-----------|---------------|-------------|-------|
| Simple (50 elements) | < 3s | _____ | [ ] |
| Medium (500 elements) | < 5s | _____ | [ ] |
| Complex (5000+ elements) | < 15s | _____ | [ ] |

---

### PV-2: Memory Footprint

#### Steps:
1. Open Chrome Task Manager (Shift+Esc)
2. Note extension memory before any audit
3. Run 10 audits
4. Note extension memory after

#### Verification:
- [ ] Memory increase < 10MB
- [ ] Memory stabilizes (doesn't keep growing)

---

### PV-3: Main Thread Blocking

#### Steps:
1. Open a page with animations or video
2. Request a full audit
3. Observe the page during analysis

#### Verification:
- [ ] Animations continue smoothly
- [ ] Page scrolling remains responsive

---

## Real-World Workflow Tests

### WF-1: Fix Accessibility Issue Workflow

**Objective:** Verify end-to-end fix workflow

#### Steps:
1. Navigate to page with accessibility issues
2. Ask AI: "Find and fix accessibility issues on this page"
3. Let AI:
   - Run analysis
   - Identify issues
   - Apply fixes via interact
   - Re-run analysis to confirm

#### Verification:
- [ ] AI runs initial analysis
- [ ] AI identifies specific issues
- [ ] AI applies fixes (if interact enabled)
- [ ] AI re-runs analysis
- [ ] AI confirms issues resolved

---

### WF-2: Security Hardening Workflow

**Objective:** Verify security improvement workflow

#### Steps:
1. Navigate to a page you control
2. Ask AI: "Help me improve the security of this page"

#### Verification:
- [ ] AI provides actionable recommendations
- [ ] Recommendations reference specific findings
- [ ] AI can generate code/config for fixes

---

### WF-3: Memory Leak Investigation

**Objective:** Verify memory debugging workflow

#### Steps:
1. Navigate to a SPA
2. Ask AI: "Help me find if there are memory leaks"

#### Verification:
- [ ] AI takes meaningful baseline
- [ ] AI interprets memory changes
- [ ] AI identifies potential leak patterns

---

### WF-4: Pre-Deployment Regression Check

**Objective:** Verify regression detection workflow

#### Steps:
1. On current production page, ask AI: "Capture baseline for regression testing"
2. Deploy a change (or simulate)
3. Ask AI: "Check for regressions"

#### Verification:
- [ ] Baseline captured with version/timestamp
- [ ] Comparison identifies real changes
- [ ] Verdict is accurate

---

## Sign-Off Checklist

### Tester Information

| Field | Value |
|-------|-------|
| Tester Name | |
| Date | |
| Browser/Version | |
| Extension Version | |
| Server Version | |

### Test Results Summary

| Category | Passed | Failed | Blocked |
|----------|--------|--------|---------|
| Happy Path (HP-1 to HP-9) | | | |
| Error Path (EP-1 to EP-9) | | | |
| Edge Cases (EC-1 to EC-7) | | | |
| Performance (PV-1 to PV-3) | | | |
| Workflows (WF-1 to WF-4) | | | |

### Failed Tests

| Test ID | Issue Description | Severity |
|---------|-------------------|----------|
| | | |

### Blocked Tests

| Test ID | Blocking Reason |
|---------|-----------------|
| | |

### Notes

_Additional observations, edge cases discovered, or suggestions:_

---

### Final Verdict

- [ ] **PASS** - All critical tests pass, ready for release
- [ ] **CONDITIONAL PASS** - Minor issues, can release with known issues documented
- [ ] **FAIL** - Critical issues found, must fix before release

**Approved By:** __________________________ **Date:** __________
