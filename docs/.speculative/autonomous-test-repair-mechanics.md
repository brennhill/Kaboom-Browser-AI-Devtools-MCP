---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Autonomous Test Repair Mechanics

**Status:** Design Document for v6.0 Wave 1 (Self-Healing Tests feature) — **Minimal MVP**

**Purpose:** This document describes how Claude uses Kaboom's v5.1 telemetry capabilities to autonomously diagnose failing E2E tests, propose fixes, apply them, and verify success.

**Key Insight:** v5.1 already has all required infrastructure. v6.0 is **validation that Claude can use it autonomously.** Zero new code required for MVP.

**Scope:** Prove the thesis with manual tests. Add automation in v6.1 if needed.

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [What v5.1 Provides](#what-v51-provides)
4. [Claude's Workflow](#claudes-workflow)
5. [Example: Test Timeout](#example-test-timeout)
6. [Integration Points](#integration-points)
7. [Performance Budgets](#performance-budgets)

---

## Overview

### The Problem

E2E test failures in CI require manual debugging (15-60 minutes):

```
Test fails
  ↓
Developer reproduces locally
  ↓
Developer inspects logs/network/DOM
  ↓
Developer guesses root cause
  ↓
Developer applies fix
  ↓
Developer verifies
```

### The Solution

Claude uses v5.1's existing telemetry to diagnose and fix autonomously (1-2 minutes):

```
Test fails
  ↓
Kaboom (v5.1) captured: timeline, network, logs, DOM
  ↓
Claude reads timeline via MCP observe()
  ↓
Claude diagnoses root cause
  ↓
Claude applies fix (git)
  ↓
Claude verifies (3 test runs)
  ↓
Claude reports results
```

### What's New in v6.0?

Not capture or correlation (v5.1 does that). The **autonomous workflow**:
- Test fails → Claude reads data → Claude diagnoses → Claude fixes → Claude verifies → Done

---

## Architecture

### System Components

```
┌──────────────────────────────────────────────────────────┐
│                    CI/CD Pipeline                        │
│                    npm test:e2e                          │
└──────────────┬───────────────────────────────────────────┘
               ↓
┌──────────────────────────────────────────────────────────┐
│              Kaboom Server v5.1 (Go)                   │
│                                                          │
│  Existing Capabilities (proven, in production):          │
│                                                          │
│  Browser Extension (Capture)                            │
│  ├─ Actions: clicks, navigations, inputs               │
│  ├─ Network: HTTP requests & responses                 │
│  ├─ Console: logs, errors, warnings                    │
│  ├─ DOM: page structure & state                        │
│  └─ Timestamps: microsecond precision                  │
│           ↓                                              │
│  MCP Endpoints (observe modes):                         │
│  ├─ observe({what: 'timeline'})                        │
│  │  └─ Merges & sorts actions+network+console by time │
│  ├─ observe({what: 'network_bodies'})                  │
│  │  └─ Full HTTP request/response bodies               │
│  ├─ observe({what: 'logs'})                            │
│  │  └─ Console output with levels & timestamps         │
│  └─ observe({what: 'page'})                            │
│     └─ DOM snapshots & page metadata                   │
│                                                          │
└──────────────┬───────────────────────────────────────────┘
               ↓
┌──────────────────────────────────────────────────────────┐
│           Claude AI Agent (v6.0 NEW)                     │
│                                                          │
│  Workflow:                                              │
│  1. Read test failure output                            │
│  2. Call observe({what: 'timeline'})                    │
│  3. Analyze timeline to diagnose                        │
│  4. Propose fix (git patch)                             │
│  5. Apply fix (git operations)                          │
│  6. Verify (spawn subagent: run test 3x)               │
│  7. Report results to PR                                │
│                                                          │
└──────────────┬───────────────────────────────────────────┘
               ↓
         GitHub PR Updated
```

### Separation of Concerns

| Component | Responsibility | Status |
|-----------|---|---|
| **Kaboom** | Capture telemetry, expose via MCP | v5.1 ✅ |
| **Claude** | Diagnose, fix, verify | v6.0 NEW |
| **GitHub** | Report results | v6.0 NEW |

---

## What v5.1 Provides

### observe({what: 'timeline'})

**Already exists. Merges and sorts events.**

```javascript
// Kaboom v5.1 implementation (codegen.go:256-384)
// Merges three sources:
// 1. Actions (clicks, navigations, inputs)
// 2. Network (HTTP requests/responses)
// 3. Logs (console output)

// Returns: Chronologically sorted timeline
Timeline = [
  { timestamp: 0, kind: 'action', type: 'navigate', url: '/checkout' },
  { timestamp: 245, kind: 'network', method: 'POST', url: '/api/checkout', status: null },
  { timestamp: 1200, kind: 'network', method: 'POST', url: '/api/checkout', status: 200, duration: 955 },
  { timestamp: 1210, kind: 'action', type: 'dom_mutation', selector: '.confirmation', action: 'inserted' },
  { timestamp: 1210, kind: 'action', type: 'dom_mutation', selector: '.spinner', action: 'removed' },
  { timestamp: 5000, kind: 'console', level: 'error', message: 'Timeout assertion failed' }
]

// All timestamps normalized to unix milliseconds
// Timestamps are from browser's performance.now() at capture time
// Reliable correlation: all events on same timeline
```

**Why timeline works:**
- Browser extension timestamps all events at capture (performance.now())
- Timestamps are globally consistent (same clock)
- Sorting by timestamp reveals causal sequence
- No guessing about "what happened when"

---

### observe({what: 'network_bodies'})

**Already exists. Full HTTP payloads.**

```javascript
// Kaboom v5.1 implementation (network.go)

NetworkCapture = {
  url: 'https://api.example.com/checkout',
  method: 'POST',
  requestBody: { email: 'test@example.com', items: [123, 456] },
  responseBody: { confirmationId: 'CH-12345', processingTime: 955 },
  statusCode: 200,

  // Metadata
  startTime: 245,      // ms from test start
  endTime: 1200,       // ms from test start
  duration: 955,
  contentType: 'application/json',
  transferSize: 245,
  decompressedSize: 451,
  compressionRatio: 0.54
}
```

---

### observe({what: 'logs'})

**Already exists. Console output with timestamps.**

```javascript
// Kaboom v5.1 implementation (tools.go)

ConsoleLogs = [
  { timestamp: 0, level: 'log', message: 'Loading checkout page' },
  { timestamp: 245, level: 'log', message: 'Submitting form' },
  { timestamp: 1200, level: 'log', message: 'Order confirmed' },
  { timestamp: 5000, level: 'error', message: 'Assertion timeout' }
]

// Includes: log, warn, error, debug
// Has noise filtering (can ignore known noisy logs)
// Tab-specific filtering (can isolate one test tab)
```

---

### observe({what: 'page'})

**Already exists. DOM snapshots & metadata.**

```javascript
// Kaboom v5.1 implementation (queries.go)

PageSnapshot = {
  title: 'Checkout',
  url: 'https://localhost:3000/checkout',

  // DOM state at failure
  html: '<form class="checkout">...</form>',

  // What elements exist/don't exist
  selectors: {
    '.loading-spinner': false,     // Not in DOM
    '.confirmation': true,         // In DOM
    '[data-testid="submit"]': true
  },

  // Accessibility tree
  a11y: {
    role: 'main',
    children: [
      { role: 'form', name: 'Checkout form' },
      { role: 'button', name: 'Submit' }
    ]
  }
}
```

---

## Claude's Workflow

### Step 1: Test Fails

```
npm test:e2e
 FAIL  src/__tests__/checkout.test.ts
  ✕ should complete checkout flow
    Error: Timeout waiting for selector ".loading-spinner"
    at test/checkout.test.ts:18
```

Kaboom (v5.1) captured everything during the test run. Data is in memory.

### Step 2: Claude Reads Timeline

```javascript
Claude calls:
observe({ what: 'timeline', limit: 100 })

Kaboom returns:
[
  { t: 0, kind: 'action', type: 'navigate', url: '/checkout' },
  { t: 245, kind: 'network', method: 'POST', status: null },
  { t: 1200, kind: 'network', method: 'POST', status: 200, duration: 955 },
  { t: 1210, kind: 'action', type: 'dom_mutation', selector: '.spinner', action: 'removed' },
  { t: 1210, kind: 'action', type: 'dom_mutation', selector: '.confirmation', action: 'inserted' },
  { t: 5000, kind: 'console', level: 'error', message: 'Timeout assertion failed' }
]
```

### Step 3: Claude Diagnoses

Claude reads the test error + timeline and reasons:

```
Test error: "Timeout waiting for .loading-spinner"

Timeline shows:
- T+245ms: POST /checkout starts
- T+1200ms: POST response (955ms duration, acceptable)
- T+1210ms: .spinner REMOVED, .confirmation INSERTED
- T+5000ms: Test assertion ran (too late!)

Root cause: Test assumption issue
- Code flow is correct: submit → wait for API → hide spinner
- Spinner was present T+1210ms to T+5000ms... no wait
- Spinner was removed at T+1210ms, test checked at T+5000ms
- Spinner only existed ~11ms? No, timeline shows:
  - T+1860ms: spinner inserted (from page load?)
  - T+1210ms: spinner removed

Wait, T+1210 < T+1860? Timestamp order issue? Check again.

Actually looking at timeline:
- Spinner is removed at T+1210ms
- Test checks for it at T+5000ms (3.8 seconds later)
- Spinner was removed BEFORE test checked

This is a test assumption error: test assumes spinner exists for 5+ seconds,
but code hides it immediately after API response.

Fix: Check for final state (.confirmation) not transient indicator (.spinner)
```

### Step 4: Claude Applies Fix

```bash
git checkout -b fix/checkout-timeout-$(date +%s)

# Edit test/checkout.test.ts
# Change: await expect(.spinner).toBeVisible()
# To:     await expect(.confirmation).toBeVisible()

git add test/checkout.test.ts
git commit -m "fix: Update assertion from spinner to confirmation

Timeline analysis showed spinner removed at T+1210ms, test checked
at T+5000ms. This is a test assumption error, not a code bug."

git push origin fix/checkout-timeout-...
```

### Step 5: Claude Verifies (3 Runs)

```bash
# Run test 3 times to ensure it's stable
for i in {1..3}; do npm test -- --testNamePattern="checkout"; done

Results:
Run 1: ✅ PASS (1865ms)
Run 2: ✅ PASS (1870ms)
Run 3: ✅ PASS (1852ms)

Success rate: 3/3 = 100%
Verdict: ✅ VERIFIED
```

### Step 6: Claude Reports

```bash
gh pr comment <PR_ID> -b "✅ **Kaboom Auto-Fix: VERIFIED**

Root Cause: Test assumption error
- Spinner removed at T+1210ms, test expected it at T+5000ms
- This is not a code bug; it's an incorrect test expectation

Fix Applied:
- Changed assertion from .spinner to .confirmation

Verification: 3 consecutive test runs
- All passed
- Average duration: 1862ms

Commit: [abc123](link)

Ready to merge!"
```

---

## Example: Test Timeout

Complete walkthrough of a single failure:

### The Failure

```
Error: Timeout waiting for selector ".loading-spinner"
```

### What Happened (from timeline)

```
T+0ms        Navigate to /checkout
T+245ms      POST /checkout initiated
T+1200ms     POST response (200 OK, 955ms duration)
T+1210ms     DOM: .spinner removed
T+1210ms     DOM: .confirmation inserted
T+5000ms     Test assertion fails (spinner not found)
```

### Why It Failed (root cause)

Test expected `.spinner` to exist. Code hides it immediately after API response.
This is not a bug; it's a test assumption error.

### How Claude Fixes It

1. Read timeline → See spinner removed at T+1210ms
2. Diagnose → Test checks for transient indicator, not final state
3. Propose → Change assertion to `.confirmation` (actual final state)
4. Apply → Commit fix to branch
5. Verify → Run 3 times, all pass
6. Report → "Root cause identified, fix applied, verified"

---

## Integration Points

### MCP Endpoints (v5.1)

Claude calls these existing endpoints:

```javascript
observe({ what: 'timeline' })
observe({ what: 'network_bodies' })
observe({ what: 'logs' })
observe({ what: 'page' })
```

All return structured data. No new MCP work needed.

### Git Operations (v6.0)

Claude uses standard git commands (already supported):

```bash
git checkout -b fix/...
git add .
git commit
git push
```

### GitHub API (v6.0)

Claude posts results:

```bash
gh pr comment <PR> -b "..."
```

---

## Performance Budgets

### Time

| Phase | Target |
|-------|--------|
| Timeline capture (v5.1) | Real-time (passive) |
| Claude diagnosis | 30-60s |
| Fix application | 10-20s |
| Verification (3 runs) | 30-120s |
| PR comment | 5-10s |
| **Total** | **1-3 minutes** |

### Memory

Kaboom v5.1 uses ring buffers (bounded):

| Buffer | Capacity |
|--------|----------|
| Timeline | 200 events (~50KB) |
| Network bodies | 100 requests (8MB) |
| Console logs | 1,000 entries (~100KB) |
| DOM snapshots | On-demand |

---

## What v6.0 Adds (Minimal MVP)

**Not capture or correlation.** Those exist in v5.1.

v6.0 MVP adds:

**Nothing. Zero new code.**

v6.0 is proving the thesis by **demonstrating that Claude can use v5.1 existing capabilities autonomously.**

### What's Different From v5.1?

| v5.1 | v6.0 |
|-----|-----|
| Captures telemetry | Same |
| Exposes via MCP | Same |
| **Usage:** Human developer reads data | **Usage:** Claude reads data autonomously |

That's it. The feature is the **demonstration that AI can diagnose + fix without human intervention.**

### v6.1+ Optimizations (Future)

Once thesis is validated, add:
- System prompt (100 lines) — Make Claude more efficient
- CI hook (50 lines) — Automatic triggering
- Git automation (50 lines) — Better branching strategy

But these don't validate the thesis. They optimize it.

---

## Why v6.0 MVP Requires Zero Code

v5.1 already has everything:
- ✅ Capture telemetry (browser extension)
- ✅ Correlate by timestamp (timeline mode: codegen.go:256-384)
- ✅ Store bounded memory (ring buffers)
- ✅ Expose via MCP (25 observe modes)

Claude already has everything:
- ✅ Bash tool (run git commands, tests)
- ✅ MCP client (call observe())
- ✅ Reasoning ability (diagnose issues)
- ✅ Problem-solving (apply fixes)

**The "feature" is just proving they work together.**

v6.0 MVP = validation workflow, not engineering work.

Only add code if validation shows it's necessary.

---

## What v6.0 Does NOT Do

- ❌ Analyze root cause algorithmically (Claude does this)
- ❌ Generate multiple fix options (Claude picks strategy)
- ❌ Pre-compute observations (Claude asks for data)
- ❌ Diagnose with confidence scores (Claude reasons)

Kaboom provides **facts**. Claude provides **reasoning**.

---

## v6.0 MVP: Minimal Scope

This feature ships with **zero new code**.

### What's Required
- ✅ Kaboom v5.1 (already exists)
- ✅ MCP observe() endpoints (already exist)
- ✅ Claude with Bash tool (already exists)
- ✅ GitHub CLI (user installs if needed)

### What's NOT Required
- ❌ System prompt (Claude figures it out)
- ❌ CI integration hook (manual trigger first)
- ❌ Analysis engine (Claude does analysis)
- ❌ Diagnosis rules (Claude reasons)

### Validation Approach

1. **Manual test:** Have Claude fix 2-3 real failing tests by hand
   - Claude reads timeline via observe()
   - Claude diagnoses
   - Claude applies fix
   - Claude verifies

2. **If it works:** Declare v6.0 thesis validated
   - "AI closes feedback loop autonomously (with human triggering)"

3. **If it doesn't:** Add system prompt + CI hook in v6.1
   - Don't block v6.0 on optimization

### Ship Criteria

- [ ] Manual test: Claude successfully diagnoses 2+ test failures
- [ ] Manual test: Claude successfully applies fixes
- [ ] Manual test: Claude successfully verifies via 3 runs
- [ ] Timeline data is accurate and usable
- [ ] No bugs in v5.1 observe() modes

**Total shipping effort: ~1 day (validation only)**

**Total new code: 0 lines**

---

## References

- [Kaboom target architecture](../features/core/target-architecture.md)
- [v5.1 Completed Features](../roadmap.md#completed-features-canonical-list)
- [Test-generation handler](../../cmd/browser-agent/internal/testgenhandler/generate.go)
- [v6.0 Roadmap](../roadmap.md)
