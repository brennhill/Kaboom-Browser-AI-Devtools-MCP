---
feature: self-healing-tests
status: proposed
doc_type: tech-spec
feature_id: feature-self-healing-tests
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Self-Healing Tests

> Plain language only. No code. Describes HOW the self-healing tests feature works at a high level.

## Architecture Overview

Self-healing tests add two new modes to existing Kaboom MCP tools:
1. **test_diagnosis** (observe tool) — Correlates test failure against captured browser telemetry to produce structured diagnosis
2. **test_fix** (generate tool) — Generates targeted fix proposal from diagnosis

These modes enable AI agents to orchestrate a multi-step workflow:
```
Test fails → Agent calls observe(test_diagnosis) → Diagnosis returned
→ Agent calls generate(test_fix) → Fix proposal returned
→ Agent applies fix → Re-runs test
```

This is NOT an autonomous loop within Kaboom. The AI agent controls the workflow. Kaboom provides observation and generation primitives.

## Key Components

### 1. Test Diagnosis Engine (observe tool, mode: test_diagnosis)

#### Inputs:
- Failure message (error text from test runner)
- Optional stack trace
- Optional test name
- Optional time window (since timestamp)
- Optional test file path

#### Processing:
1. Parse failure message to extract failure type (element not found, timeout, assertion failed, etc.)
2. Query ring buffers for telemetry in the time window (default: last 60 seconds)
3. Correlate failure against:
   - Console errors (timestamps, messages, stack traces)
   - Network failures (4xx/5xx responses, timeouts)
   - DOM state (query for expected element, find candidates)
   - Recent changes (from causal diffing data)
4. Classify failure into category (selector_stale, api_contract_changed, timing_issue, etc.)
5. Compute confidence based on evidence strength
6. Return structured diagnosis with category, evidence, root cause explanation, recommended action

#### Outputs:
- Diagnosis object with:
  - category (enum: selector_stale, api_contract_changed, timing_issue, element_removed, network_failure, js_error, true_regression, flaky, unknown)
  - confidence (high, medium, low)
  - summary (human-readable explanation)
  - evidence (DOM data, console errors, network failures, timing metrics)
  - root_cause (plain-English explanation)
  - recommended_action (fix_test, fix_code, mark_flaky, investigate)
  - related_changes (state changes observed in telemetry window)

### 2. Test Fix Generator (generate tool, format: test_fix)

#### Inputs:
- Diagnosis object (from observe test_diagnosis)
- Optional test file path
- Optional framework (playwright, cypress, puppeteer, generic)
- Optional fix strategy override

#### Processing:
1. Read diagnosis category and evidence
2. Determine fix strategy based on category:
   - selector_stale → selector_update (find candidate, propose replacement)
   - api_contract_changed → mock_update (show old vs new response shape)
   - timing_issue → wait_adjustment (add/adjust wait conditions)
   - element_removed → manual_investigation_needed
   - network_failure → check_endpoint_or_mock
   - js_error → fix_code_not_test
   - true_regression → fix_code_not_test
3. Generate specific changes:
   - For selector_update: extract old selector from diagnosis, propose new selector from candidates
   - For wait_adjustment: suggest explicit waits (waitForSelector, waitForNetworkIdle)
   - For mock_update: diff old vs new response shapes, suggest fixture updates
4. Generate framework-specific hints (if framework specified)
5. Add confidence level and rationale for each change
6. Return structured fix proposal

#### Outputs:
- Fix object with:
  - strategy (enum: selector_update, wait_adjustment, api_mock_update, code_fix_needed, mark_flaky, no_fix_available)
  - description (human-readable summary)
  - changes array (ordered list of proposed changes)
  - framework_hint (framework-specific code suggestion)
  - warnings (caveats, e.g., multiple candidates found)
  - recommended_action (same as diagnosis)

### 3. Selector Candidate Finder

**Purpose:** For selector_stale failures, find similar elements in current DOM.

#### Processing:
1. Extract expected selector from failure message or test file
2. Query DOM for elements matching selector (should return 0 matches)
3. Use similarity search to find candidates:
   - Text similarity (Levenshtein distance on selector strings)
   - Structural similarity (if DOM fingerprinting available — same tag, similar attributes, similar position)
   - Role similarity (same aria-role or element type)
4. Rank candidates by similarity score
5. Return top 3-5 candidates with scores and context (tag, text, attributes, bounding box)

#### Integration:
- Uses existing analyze tool infrastructure (`what:"dom"` mode)
- Leverages DOM fingerprinting feature (when available) for structural matching
- Falls back to string similarity if fingerprinting not available

### 4. API Contract Differ

**Purpose:** For api_contract_changed failures, identify what changed in response shape.

#### Processing:
1. Extract API endpoint from failure context or network waterfall
2. Query network_bodies buffer for recent responses from that endpoint
3. If historical data available (from previous passing runs), compare old vs new
4. Diff response shapes:
   - Field added (present in new, missing in old)
   - Field removed (present in old, missing in new)
   - Field type changed (string → number)
   - Field renamed (similar name, different position)
5. Identify which field the test was trying to access
6. Return diff with old shape, new shape, and affected field

#### Integration:
- Uses existing network_bodies buffer
- Leverages causal diffing (when available) to identify "before" state
- Falls back to single response shape analysis if no historical data

## Data Flows

### Diagnosis Flow
```
Test failure output → AI agent → MCP: observe({what: "test_diagnosis", failure: {...}})
→ Server receives request
→ Parse failure message (extract failure type, expected element/assertion)
→ Query ring buffers:
  - console errors (match timestamps, messages)
  - network bodies (check for 4xx/5xx, response shape changes)
  - changes (recent DOM/network state changes)
  - error_clusters (known error patterns)
→ Correlate evidence:
  - If element not found: analyze({what: "dom"}) for candidates
  - If API error: check network_bodies for endpoint status
  - If JS error: check console errors for stack traces
→ Classify failure category based on evidence
→ Compute confidence (high if strong evidence, low if weak)
→ Build diagnosis response with category, evidence, root cause, action
→ Return to AI agent
```

### Fix Generation Flow
```
AI agent receives diagnosis → MCP: generate({format: "test_fix", diagnosis: {...}})
→ Server receives request
→ Read diagnosis category and evidence
→ Determine fix strategy:
  - selector_stale → extract candidate from evidence, propose selector_replace change
  - api_contract_changed → extract diff from evidence, propose mock_update change
  - timing_issue → analyze timing metrics, propose wait_add or wait_adjust change
→ Build change objects with old_value, new_value, confidence, rationale
→ Add framework-specific hint (if framework provided)
→ Add warnings if applicable (multiple candidates, manual review needed)
→ Return fix proposal to AI agent
```

### Selector Candidate Flow (Sub-flow of Diagnosis)
```
Failure: "Element not found: [data-testid='submit-btn']"
→ Diagnosis engine extracts selector: "[data-testid='submit-btn']"
→ Call analyze({what: "dom"}) with selector (should return 0 matches)
→ Call analyze({what: "dom"}) with universal selector "*" (get all elements)
→ Filter elements by similarity:
  - Find buttons (tag match)
  - Find elements with data-testid attribute (attribute match)
  - Find elements with similar text ("Submit", "submit-button")
  - Compute Levenshtein distance on selector strings
→ Rank by combined score
→ Return top 3: [data-testid='submit-button'] (0.92), [data-testid='submit'] (0.85), button.submit (0.78)
→ Include in diagnosis.evidence.dom.candidates
```

## Implementation Strategy

### Phase 1: Diagnosis Core
1. Add "test_diagnosis" mode to observe tool schema
2. Implement diagnosis engine in server (new module: test_diagnosis.go or similar)
3. Implement failure message parser (extract selector, assertion, error type)
4. Implement evidence correlator (query ring buffers, match timestamps)
5. Implement category classifier (heuristics for each category)
6. Return structured diagnosis response

### Phase 2: Selector Candidate Finder
1. Implement selector extraction from failure message
2. Implement DOM query for candidates (use existing analyze({what: "dom"}))
3. Implement string similarity scoring (Levenshtein or similar)
4. Integrate with DOM fingerprinting (when available) for structural similarity
5. Return top candidates with scores

### Phase 3: Fix Generator
1. Add "test_fix" format to generate tool schema
2. Implement fix strategy selector (map category to strategy)
3. Implement change builders:
   - selector_replace builder
   - wait_adjustment builder
   - mock_update builder
4. Implement framework-specific hints (templates for Playwright, Cypress, Puppeteer)
5. Return structured fix proposal

### Phase 4: API Contract Differ
1. Implement endpoint extraction from failure context
2. Implement response shape analysis (JSON schema extraction)
3. Implement shape diff (compare old vs new fields)
4. Integrate with causal diffing (when available)
5. Return diff in fix proposal

### Phase 5: Integration with Existing Features
1. Leverage error_clusters for known error patterns
2. Leverage changes (causal diffing) for recent state changes
3. Leverage DOM fingerprinting for structural similarity
4. Ensure test_diagnosis and test_fix respect existing redaction rules

## Edge Cases & Assumptions

### Edge Case 1: No Telemetry in Time Window
**Handling:** Return category: "unknown", confidence: "low", with message explaining no telemetry found. Suggest agent verify Kaboom is capturing (check observe({what: "page"})).

### Edge Case 2: Multiple Candidate Selectors
**Handling:** Return all candidates sorted by score. Fix proposal uses highest-scoring candidate but includes warning: "Multiple candidates found, manual review recommended".

### Edge Case 3: Failure Message Ambiguous
**Handling:** Parser extracts best guess (partial selector, error type). If parsing fails, return category: "unknown" with raw failure message in evidence.

### Edge Case 4: Test Failure Outside Kaboom Window
**Handling:** If test ran before Kaboom started capturing, no telemetry available. Return category: "unknown" with note about timing.

### Edge Case 5: Diagnosis Disagrees with Fix
**Handling:** Diagnosis says "selector_stale", but no candidates found. Fix proposal escalates to "element_removed" and recommends manual investigation.

### Assumption 1: Kaboom Is Capturing During Test
We assume the browser extension is active and capturing telemetry when the test runs. Without this, diagnosis returns "unknown".

### Assumption 2: Test Failure Output Available
We assume the AI agent has access to test runner output (error message, stack trace). This is passed as input to test_diagnosis.

### Assumption 3: Single Tab for Tests
We assume tests run in a single tracked tab. Multi-tab tests (Playwright parallel workers) may require tab correlation (future enhancement).

### Assumption 4: Test Failures Are Browser-Side
We assume failures occur in the browser (DOM, network, JS errors). Non-browser failures (unit tests, API tests) cannot be diagnosed.

## Risks & Mitigations

### Risk 1: False Positives in Diagnosis
**Mitigation:** Use confidence levels (high, medium, low). AI agent can treat low-confidence diagnoses as suggestions, not facts. Include evidence in response so agent can verify.

### Risk 2: Fix Proposals Break Tests Further
**Mitigation:** Fix proposals are suggestions, not automatically applied. AI agent reviews and applies. Include rationale for each change so agent can evaluate.

### Risk 3: Selector Candidate Finder Misses True Match
**Mitigation:** Return multiple candidates, not just top one. AI agent can review all candidates. If all candidates wrong, recommended_action escalates to "investigate".

### Risk 4: API Contract Differ Without Historical Data
**Mitigation:** If no historical data, show current response shape and note that comparison not available. Fix proposal suggests manual fixture update.

### Risk 5: Performance Degradation from Correlation
**Mitigation:** Diagnosis queries ring buffers (in-memory, fast). Default time window is 60 seconds (limits scan). Target < 500ms response time.

## Dependencies

### Depends On (Existing Features)
- **observe({what: "errors"})** — Console error data
- **observe({what: "network_waterfall"})** — Network request metadata
- **observe({what: "network_bodies"})** — Network response bodies
- **observe({what: "changes"})** — Recent state changes
- **observe({what: "error_clusters"})** — Error pattern grouping
- **analyze({what: "dom"})** — DOM queries for candidates
- **generate({format: "test"})** — Existing test generation infrastructure (shared templates)

### Optionally Leverages (In-Progress Features)
- **DOM fingerprinting** — Structural similarity for selector candidates (improves matching quality)
- **Causal diffing** — Before/after state comparison for API contract changes (enables historical comparison)

### Depended On By (Proposed Features)
- **Agentic E2E Repair** — Specializes self-healing for API contract drift
- **Agentic CI/CD** — Orchestrates self-healing in CI pipelines
- **Kaboom CI** — Provides headless capture for CI environments

## Performance Considerations

- Diagnosis response time: < 500ms (in-memory buffer queries)
- Fix generation response time: < 200ms (operates on diagnosis data, no new queries)
- Memory impact: < 2MB transient (copies telemetry for analysis, releases after)
- DOM candidate search: < 100ms (uses existing analyze({what: "dom"}) infrastructure)
- Time window scan: default 60s (configurable, max 5 minutes recommended)

## Security Considerations

- **No new data capture:** Reads existing ring buffers, no new capture mechanisms
- **Failure messages may contain sensitive data:** Test output could include API keys, tokens, PII. Apply existing redaction rules to diagnosis response.
- **No code execution:** test_fix generates text proposals (selectors, waits, mocks). Does not execute JavaScript or modify files. AI agent handles application.
- **Test file paths informational only:** test_file parameter used for context. Kaboom does not read or write files on disk.

## Test Plan Reference

See qa-plan.md for detailed testing strategy. Key test scenarios:
1. Diagnosis correctly identifies selector_stale failure
2. Diagnosis correctly identifies api_contract_changed failure
3. Diagnosis correctly identifies timing_issue failure
4. Fix proposal suggests correct selector replacement
5. Fix proposal suggests correct wait adjustment
6. Fix proposal includes framework-specific hints
7. No telemetry → diagnosis returns "unknown" with clear message
8. Multiple candidates → fix proposal includes warning
