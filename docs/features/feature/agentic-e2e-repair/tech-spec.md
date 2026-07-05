---
feature: agentic-e2e-repair
status: proposed
doc_type: tech-spec
feature_id: feature-agentic-e2e-repair
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Agentic E2E Repair

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Agentic E2E Repair is an **agent workflow pattern** that orchestrates existing Kaboom MCP tools to diagnose and fix broken end-to-end tests. It adds no new MCP tools or modes. The agent follows a structured diagnosis-and-repair loop using `observe`, `configure`, `generate`, and `interact` tools in a specific sequence.

The workflow assumes an E2E test has failed and the agent has access to test runner output. The agent re-runs the failing test with Kaboom capturing browser telemetry, diagnoses the root cause category, generates a corrected test file, verifies the fix, and reports the change.

## Key Components

The workflow has seven sequential phases:

### Phase 1: Detect
The agent learns that an E2E test failed through one of three mechanisms:
- **Test runner stdout parsing**: Agent reads test framework output (Playwright, Cypress, Selenium) and identifies which test failed with what error message.
- **CI webhook**: External CI system sends failure notification to agent.
- **Developer instruction**: Human tells the agent "test X failed."

The agent extracts: test file path, test name, error message, and optionally the stack trace.

### Phase 2: Re-run with Capture
The agent re-runs the failing test with Kaboom capturing browser telemetry. Two strategies:

#### Strategy A: Agent invokes test runner directly
```
bash: npx playwright test tests/checkout.spec.js --headed
```
The `--headed` flag ensures the test runs in a visible browser window that the Kaboom extension can track.

#### Strategy B: Agent navigates manually and simulates the test flow
If test runner integration is difficult, the agent reads the test source code, navigates to the tested URL via `interact({action: "navigate"})`, and executes the test steps manually via `interact({action: "execute_js"})`. Kaboom captures telemetry as the agent drives the browser.

During this phase, Kaboom passively captures: console errors, network requests/responses, DOM state snapshots, WebSocket events, performance timing.

### Phase 3: Observe
The agent uses Kaboom's `observe` tool to inspect what the browser saw:

```
observe({what: "errors"})
observe({what: "network_waterfall"})
observe({what: "network_bodies", url_filter: "/api/..."})
analyze({what: "dom", selector: ".submit-btn"})
configure({action: "validate_api", operation: "analyze"})
observe({what: "error_clusters"})
observe({what: "changes"})
```

This provides the evidence needed for diagnosis: actual console errors, actual API responses, actual DOM state, error groupings, and what changed since the last known-good run.

### Phase 4: Diagnose
The agent correlates the test's expected behavior with observed browser state to classify the failure into one of five root cause categories:

#### Category 1: Selector Drift
- Diagnostic signals: Test fails with "element not found" or "locator timeout." `analyze({what: "dom"})` with test's selector returns empty. `analyze({what: "dom"})` with relaxed selector (text content, role, partial class) finds a matching element. `observe({what: "changes"})` shows DOM structure changed recently.
- Decision: The element was renamed or restructured, not removed. Update the selector.

#### Category 2: API Contract Drift
- Diagnostic signals: Test assertions on response body fields fail. `observe({what: "network_bodies"})` shows actual response has different field names or structure. `configure({action: "validate_api"})` reports `shape_change`, `type_change`, or `new_field` violations. Inferred API schema differs from test's expectations.
- Decision: API contract changed. Update test assertions and mocks to match new shape.

#### Category 3: Timing Fragility
- Diagnostic signals: Test fails intermittently (passes on retry). `observe({what: "network_waterfall"})` shows slow responses overlapping with test action. Test uses hard-coded waits or no waits. Errors related to incomplete loading ("Cannot read property of null").
- Decision: Add explicit wait conditions (`waitForResponse`, `waitForSelector`, `waitForLoadState`).

#### Category 4: Mock/Fixture Staleness
- Diagnostic signals: Test mocks return data not matching actual API shape. `observe({what: "network_bodies"})` shows real responses differ from mock data. `validate_api` shows discrepancies. Test passes with mocks but fails against real backend.
- Decision: Update mock data to match current API response shape.

#### Category 5: True Regression
- Diagnostic signals: Test's assertions match documented API contract, but application behavior changed. `observe({what: "errors"})` shows application-level errors (not test errors). `observe({what: "error_clusters"})` shows a new cluster. Failure correlates with recent code change.
- Decision: Do NOT fix the test. Report the regression with diagnostic evidence.

The agent uses error clustering to avoid investigating the same root cause multiple times when multiple tests fail from a single change.

### Phase 5: Generate Fix
Based on diagnosis category, the agent uses `generate` tool to produce corrected test file:

**For selector drift**:
```
generate({format: "test", include_fixtures: false})
```
Agent provides context: "Update selector from `.submit-btn` to `.btn-submit` based on observed DOM."

**For API contract drift**:
```
generate({format: "test", include_fixtures: true})
```
Agent provides context: "Update assertions to expect `user_name` instead of `userName`. Update mock fixture to return new shape."

**For timing fragility**:
```
generate({format: "test", include_fixtures: false})
```
Agent provides context: "Add `await page.waitForResponse('/api/data')` before assertion."

**For mock staleness**:
```
generate({format: "test", include_fixtures: true})
```
Agent provides context: "Update mock fixture to match observed API response shape from `network_bodies`."

**For true regression**:
```
generate({format: "reproduction"})
```
Agent generates reproduction script instead of test fix.

The `generate` tool produces framework-appropriate test code (Playwright, Cypress, Selenium) based on detected framework.

### Phase 6: Verify
The agent re-runs the test with the proposed fix to confirm it passes:

#### Strategy A: Write fix to file and invoke test runner
```
Write: tests/checkout.spec.js (updated test code)
bash: npx playwright test tests/checkout.spec.js
```

#### Strategy B: Run at least twice to catch flakes
If the test passes on first run, run again. If it passes both times, consider it fixed. If it passes intermittently, classify as a flake and report rather than claiming a fix.

**Circuit breaker**: Maximum 3 fix attempts per test. If third attempt fails, escalate to human review.

### Phase 7: Report
The agent provides a structured explanation:

```
generate({format: "pr_summary"})
```

Output includes: test file path and test name, root cause category (selector drift, API contract drift, timing fragility, mock staleness, or true regression), what changed (specific field names, selector patterns, wait conditions), why the fix is correct (based on observed browser state), and whether fix was verified (test now passes).

For batch repairs (multiple tests broken by same root cause), agent reports once with list of affected tests.

## Data Flows

```
Test fails (test runner output or human notification)
  |
  v
Agent re-runs test with Kaboom capture
  -> bash: npx playwright test --headed
  OR
  -> interact({action: "navigate"}) + execute_js
  |
  v
Kaboom captures telemetry
  -> Console errors, network, DOM, WebSocket, performance
  |
  v
Agent observes browser state
  -> observe({what: "errors"})
  -> observe({what: "network_bodies"})
  -> analyze({what: "dom"})
  -> configure({action: "validate_api"})
  -> observe({what: "error_clusters"})
  |
  v
Agent diagnoses root cause
  -> Selector drift / API drift / Timing / Mock stale / True regression
  |
  v
Agent generates fix (or reproduction if regression)
  -> generate({format: "test", include_fixtures: true/false})
  OR
  -> generate({format: "reproduction"})
  |
  v
Agent verifies fix
  -> Write updated test file
  -> bash: npx playwright test
  -> Check exit code
  |
  v
Agent reports outcome
  -> generate({format: "pr_summary"})
```

## Implementation Strategy

**No Kaboom changes required.** This feature is entirely agent-side logic. Implementation:

1. **Skill definition file** (if using Claude Code skills): Encodes the seven-phase workflow, diagnosis heuristics, and circuit breaker rules.
2. **Agent prompt enhancement**: Instructions for agent on how to diagnose and repair E2E test failures.
3. **Test runner integration**: Agent must be able to invoke test runner CLI (Playwright, Cypress, Selenium) and parse stdout/stderr.

Trade-off: No server overhead, but requires agent reasoning to classify failures and generate appropriate fixes. Agent must have access to test source code for reading and writing.

## Edge Cases & Assumptions

### Edge Cases

- **No Kaboom data captured**: Extension not connected or not tracking the test browser tab. Agent cannot diagnose without observations. Reports "insufficient data" and suggests re-running with Kaboom capture enabled.

- **Test fails with no browser errors and no network failures**: Agent queries DOM to check selector validity. If selectors valid and API responses correct, classifies as potential timing issue and examines wait conditions.

- **Multiple root causes in single test**: Agent addresses root causes in dependency order (API contract first, then selectors, then timing). Generates single fix addressing all causes.

- **Test framework not recognized**: Agent observes browser state but generates generic JavaScript fixes rather than framework-specific ones. `generate({format: "test"})` handles framework detection.

- **Extension disconnected during test run**: Observation data partial or missing. Agent falls back to test runner output only and reports Kaboom capture was incomplete.

- **Buffer overflow during long test suite**: Kaboom's ring buffers evict oldest entries. Agent clears buffers between test runs using `configure({action: "clear"})` to ensure relevant data available.

- **Test was already flaky before agent's change**: Agent runs test at least twice after fixing. If passes intermittently, classifies as flake and reports rather than claiming fix.

- **Fix breaks different test**: Circuit breaker activates after 3 attempts. Agent reports "fix introduced regression in test X" and escalates to human.

- **API returns errors (500) rather than changed data**: Agent distinguishes "API contract changed" (fix test) from "API broken" (true regression). 500 response with error body is regression; 200 response with different fields is contract drift.

### Assumptions

- A1: Kaboom server running and extension tracking the tab where E2E test executes.
- A2: Test runner produces machine-readable output agent can parse to identify which test failed with what error.
- A3: Network body capture enabled for test run (required for API contract drift detection).
- A4: Test repository accessible to agent for reading test source and writing fixes.
- A5: Agent has context on project's test framework (Playwright, Cypress, etc.) to generate framework-appropriate fixes.
- A6: Application under test runs on localhost or URL accessible from Kaboom-instrumented browser.

## Risks & Mitigations

### Risk 1: Agent modifies wrong test file
- **Description**: Agent misidentifies test file or test name from runner output.
- **Mitigation**: Agent confirms file path and test name match exactly before writing. Circuit breaker prevents runaway modifications.

### Risk 2: Fix introduces new failures
- **Description**: Updated selector or assertion works for failing test but breaks other tests.
- **Mitigation**: Agent re-runs full test suite after fix (if time budget allows) or at minimum runs related tests. Circuit breaker catches cascading failures.

### Risk 3: Agent cannot classify failure
- **Description**: Failure doesn't match any of the five root cause categories.
- **Mitigation**: Agent reports "unable to classify — manual investigation required" with full diagnostic evidence (errors, network, DOM state). Does not attempt blind fix.

### Risk 4: Test framework syntax errors
- **Description**: Generated test code has syntax errors or uses wrong framework APIs.
- **Mitigation**: `generate({format: "test"})` produces valid framework-specific code based on detected framework. Agent verifies fix by running test — syntax errors caught immediately.

### Risk 5: Sensitive data in test fixtures
- **Description**: Agent generates fixtures containing real user data from observed API responses.
- **Mitigation**: Kaboom strips auth headers and redacts sensitive fields. Agent applies additional redaction when generating fixtures (emails, tokens, passwords replaced with placeholder values).

## Dependencies

### Depends on:
- `observe` tool: `errors`, `network_waterfall`, `network_bodies`, `api`, `error_clusters`, `changes` modes (shipped)
- `generate` tool: `test`, `reproduction` types (shipped)
- `analyze` tool: `what:"dom"` mode (shipped)
- `configure` tool: `diff_sessions`, `clear` actions (shipped)
- `interact` tool: `navigate`, `refresh`, `execute_js`, `save_state`, `load_state` actions (shipped)
- Error Clustering feature (shipped)
- API Schema Inference feature (shipped)
- Reproduction Enhancements feature (shipped)
- DOM Fingerprinting feature (in-progress, enhances selector drift detection but not strictly required)

### Depended on by:
- Kaboom CI integration (runs this workflow in CI pipelines)
- Self-Healing Tests (may invoke E2E Repair as sub-workflow for individual test fixes)

## Performance Considerations

Per-operation Kaboom overhead is minimal:

| Operation | Kaboom overhead | Notes |
|-----------|------------------|-------|
| Test re-run observation | < 50ms startup | Actual test duration depends on test complexity |
| Observe errors/network | < 10ms per call | Reading from buffers |
| Query DOM | < 100ms | DOM traversal |
| Validate API | < 20ms | Schema comparison |
| Generate test fix | < 300ms | Test code generation |
| Full workflow | < 5 minutes | Includes test re-runs (2-3x) |

Dominant costs are test execution time (outside Kaboom control) and agent reasoning time.

## Security Considerations

**No new attack surface**: Uses existing MCP tools with existing security boundaries (localhost-only, opt-in body capture, header stripping).

**Test code, not production code**: Agent modifies test files, not application source. If determines true regression, reports rather than fixes.

**Human review gate**: Agent proposes fixes (via commit, PR, or direct output) but does not auto-merge. Human or CI gate must approve changes.

**Credential safety**: Test fixtures generated from observed API traffic must not include auth tokens, session cookies, or API keys. Existing header stripping and redaction patterns apply.

**Circuit breaker**: Agent stops after 3 failed fix attempts to prevent infinite fix-break loops.

**Audit trail**: All Kaboom tool calls logged via enterprise audit feature. Agent's diagnostic reasoning included in commit message or PR description for traceability.
