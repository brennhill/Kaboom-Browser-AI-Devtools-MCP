---
feature: analyze-tool
type: review
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Spec Review: `analyze` Tool

## Review Metadata

| Field | Value |
|-------|-------|
| **Review Date** | 2026-01-29 |
| **Reviewer** | Principal Engineer Agent (Claude Opus 4.5) |
| **Spec Version** | v7.0 (proposed) |
| **Files Reviewed** | product-spec.md, tech-spec.md, qa-plan.md, migration.md, architecture.md |

---

## 1. Performance

### Critical Issues

#### P1-1: axe-core lazy loading strategy is underspecified
- TECH_SPEC says "Lazy-load only when analyze called" but doesn't specify:
  - Where the loaded state is tracked (content.js or inject.js)
  - How to avoid re-injection if already loaded
  - Memory cleanup strategy after analysis completes
- **Resolution required**: Add explicit loading state machine

#### P1-2: Lighthouse integration via chrome.debugger is blocking
- Using `chrome.debugger` API requires attaching to the tab, which can block other DevTools operations
- TECH_SPEC mentions "simulate throttling" but this still involves substantial main thread work
- **Resolution required**: Clarify that Lighthouse runs in a worker or background context, or accept this limitation with clear documentation

### Medium Issues

#### P2-1: 30-second timeout may be insufficient for full audits
- PRODUCT_SPEC says "Audits complete in < 5 seconds for typical pages"
- TECH_SPEC specifies 30-second timeout
- axe-core on complex pages (1000+ elements) can take 10-15 seconds
- Full Lighthouse audit takes 20-40 seconds even with throttling
- **Suggestion**: Consider per-action timeouts (5s for accessibility, 30s for Lighthouse)

#### P2-2: Memory snapshot via performance.memory is deprecated
- `performance.memory` is a non-standard Chrome-only API that may be removed
- **Suggestion**: Document as Chrome-only feature, add fallback detection

### Minor Issues

#### P3-1: Caching strategy (10 seconds) may cause stale results
- If user fixes an issue and immediately re-runs audit, cached result won't reflect the fix
- **Suggestion**: Add `force_refresh` parameter

---

## 2. Concurrency

### Critical Issues

#### C1-1: No goroutine lifecycle management specified for analysis workers
- Current pilot.go pattern uses pending queries with WaitForResult (blocking)
- execute_js uses async pattern with correlation IDs (non-blocking)
- TECH_SPEC doesn't specify which pattern `analyze` will use
- **Resolution required**: Explicitly choose:
  - Blocking (WaitForResult) for quick operations (< 5s)
  - Async (correlation_id) for long operations (Lighthouse)

### Medium Issues

#### C2-1: Multiple concurrent analyses not addressed
- QA_PLAN mentions "Multiple concurrent analyses" test case but TECH_SPEC doesn't specify:
  - Maximum concurrent analyses per tab
  - How to handle conflicts (e.g., two memory snapshots)
- **Suggestion**: Implement per-tab analysis mutex or queue

#### C2-2: Race condition possible during page navigation
- TECH_SPEC says "Page navigates during analysis → Cancel analysis, return partial results"
- But inject.js may have already started axe-core which continues running
- **Suggestion**: Add navigation listener that sends abort signal to inject.js

---

## 3. Data Contracts

### Critical Issues

#### D1-1: Response format not fully specified
- PRODUCT_SPEC shows sample response but doesn't define:
  - Complete schema for each action type
  - Error response format (should use existing StructuredError pattern from tools_core.go)
  - Partial result format on timeout
- **Resolution required**: Define JSON schema for all response types, aligned with existing mcpJSONResponse/mcpStructuredError patterns

### Medium Issues

#### D2-1: Severity levels inconsistent with existing security_audit
- PRODUCT_SPEC uses: `critical`, `high`, `medium`, `low`
- Existing security_audit in observe tool uses: `critical`, `high`, `medium`, `low`, `info`
- **Suggestion**: Align severity enums across all security-related features

#### D2-2: Memory mode response schema missing
- PRODUCT_SPEC lists "Response includes" but no sample JSON
- **Suggestion**: Add explicit response schema for memory.snapshot, memory.compare, memory.leaks

### Minor Issues

#### D3-1: Bundle mode may not be implementable as specified
- "Duplicate dependencies" and "Unused exports" require source maps or build-time analysis
- Browser cannot detect these from runtime inspection alone
- **Suggestion**: Either scope to what's observable (loaded chunk sizes) or document as "requires build integration"

---

## 4. Error Handling

### Critical Issues

#### E1-1: CSP blocking scenario not fully addressed
- TECH_SPEC says "Fallback: Return error with guidance to adjust CSP"
- But this leaves user with no analysis capability
- **Resolution required**: Implement isolated context injection via chrome.debugger as primary fallback

### Medium Issues

#### E2-1: React DevTools fallback is too coarse
- TECH_SPEC says "Return 'React DevTools required for render analysis'"
- This message doesn't help Vue/Svelte users
- **Suggestion**: Detect framework first, then provide framework-specific guidance

#### E2-2: Error codes not specified
- Need to define new error codes for analyze-specific failures:
  - `axe_injection_failed`
  - `lighthouse_unavailable`
  - `analysis_timeout`
  - `framework_not_detected`
- **Suggestion**: Add to tools_core.go error code constants

---

## 5. Security

### Critical Issues

#### S1-1: AI Web Pilot toggle requirement needs enforcement path
- TECH_SPEC says "Requires AI Web Pilot toggle enabled"
- But PRODUCT_SPEC describes analyze as "read-only (unlike interact's execute_js)"
- If analyze is read-only, does it really need the same high-trust toggle?
- **Resolution required**: Clarify security model:
  - Option A: Require AI Web Pilot (consistent with other extension-controlled tools)
  - Option B: Create separate "Analysis" toggle (lower trust than execute_js)

### Medium Issues

#### S2-1: Cookie value redaction mentioned but not specified
- TECH_SPEC says "Sanitized output: Strip sensitive data from findings"
- But no redaction patterns specified
- **Suggestion**: Reuse existing redaction_test.go patterns; document what gets redacted

#### S2-2: Storage audit may expose sensitive data
- security.storage "Audit localStorage/sessionStorage for sensitive data"
- Returning detected patterns could itself leak sensitive info to AI
- **Suggestion**: Return only keys and pattern matches, not values

---

## 6. Maintainability

### Critical Issues

#### M1-1: Extension code organization unclear
- migration.md says create `extension/lib/analyze.js`
- But current extension has modular structure (console.js, network.js, etc.)
- Need to decide: one analyze.js or per-mode files (audit.js, memory.js, etc.)
- **Resolution required**: Define file structure that matches existing patterns

### Medium Issues

#### M2-1: Test file locations inconsistent
- QA_PLAN specifies:
  - `cmd/browser-agent/analyze_test.go` (matches existing pattern)
  - `tests/extension/analyze.test.js` (correct)
  - `tests/integration/analyze_test.go` (no integration test folder exists currently)
- **Suggestion**: Clarify integration test location or create the folder

#### M2-2: Version bump rationale is weak
- migration.md says "minor version bump (v6.x → v7.0)"
- But current version in main.go is 5.2.0
- **Suggestion**: Clarify version numbering rationale

### Minor Issues

#### M3-1: Documentation updates incomplete
- migration.md lists files to update but misses:
  - `/docs/core/UAT-TEST-PLAN.md` (needs analyze scenarios)
  - Extension manifest.json (may need new permissions for chrome.debugger)
  - CHANGELOG.md pattern

---

## 7. Architectural Constraint Change

### Observation

The change from 4-tool to 5-tool maximum is **well-reasoned**:
- The semantic boundary argument is valid: "what happened" (observe) vs "what's wrong" (analyze)
- 5 tools remains minimal compared to competitors
- The constraint language changed appropriately: "5-tool maximum" not "5 tools"

**Approved with note**: Future tool additions should require architecture review with this same rigor.

---

## Implementation Roadmap

### Phase 0: Pre-Implementation
1. Resolve all Critical issues (P1-1, P1-2, C1-1, D1-1, E1-1, S1-1, M1-1)
2. Update TECH_SPEC with resolutions
3. Re-review updated spec

### Phase 1: Core Infrastructure
1. Add `analyze` tool registration to `cmd/browser-agent/tools_core.go`
2. Create `cmd/browser-agent/analyze.go`
3. Create endpoint `/analyze-result` in `main.go`
4. Write unit tests: `cmd/browser-agent/analyze_test.go`

### Phase 2: Extension Infrastructure
1. Create `extension/lib/analyze.js` (dispatcher)
2. Update `extension/content.js` (message handling)
3. Update `extension/background.js` (routing)
4. Write extension tests: `tests/extension/analyze.test.js`

### Phase 3: Audit Mode
1. Implement axe-core integration with lazy loading
2. Implement Lighthouse integration (or document as future work)
3. Implement accessibility/performance audit
4. Write audit-specific tests

### Phase 4: Memory & Security
1. Implement memory.snapshot/compare
2. Implement security.headers/cookies/storage
3. Skip bundle mode (requires build-time info) or document limitations

### Phase 5: Regression Mode
1. Implement baseline capture
2. Implement compare with stored baseline

### Phase 6: Documentation & UAT
1. Update UAT-TEST-PLAN.md
2. Update CHANGELOG.md
3. Run full UAT suite

---

## Sign-Off

| Status | Details |
|--------|---------|
| **NEEDS REVISION** | Resolve 7 Critical issues before implementation |

### Critical Issues Requiring Resolution

| ID | Issue | Resolution Path |
|----|-------|-----------------|
| P1-1 | axe-core lazy loading strategy | Add loading state machine to TECH_SPEC |
| P1-2 | Lighthouse blocking behavior | Document limitation or specify worker context |
| C1-1 | Blocking vs async pattern | Choose pattern per operation type |
| D1-1 | Response schemas | Define JSON schema for all response types |
| E1-1 | CSP fallback implementation | Specify chrome.debugger isolated context path |
| S1-1 | Security toggle requirement | Decide: AI Web Pilot or separate toggle |
| M1-1 | Extension file organization | Define file structure matching existing patterns |

Once these are addressed in an updated TECH_SPEC, re-submit for final approval.

---

## Key Implementation Files

- `cmd/browser-agent/tools_core.go` — Tool registration pattern
- `cmd/browser-agent/pilot.go` — Pending query pattern for extension communication
- `cmd/browser-agent/types.go` — PendingQuery struct
- `extension/content.js` — Message bridge
- `extension/lib/axe.min.js` — Already bundled (527KB)
