---
feature: agentic-e2e-repair
status: proposed
version: null
tool: observe, generate, configure, interact
mode: multi-mode orchestration
authors: []
created: 2026-01-28
updated: 2026-01-28
doc_type: product-spec
feature_id: feature-agentic-e2e-repair
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Agentic E2E Repair

> AI agents automatically detect, diagnose, and fix broken end-to-end tests by observing browser state through Kaboom, identifying root causes, and generating targeted repairs.

## Problem

End-to-end tests are the most fragile tests in any test suite. They break frequently for reasons that are not bugs in the application:

1. **Selector drift.** A developer renames a CSS class or restructures a component. The test uses `.submit-btn` but the button is now `.btn-submit`. The application works correctly; the test is wrong.

2. **API contract drift.** A backend team renames a field from `userName` to `user_name`. The frontend adapts, but the E2E test's assertions or mocks still reference the old field name. The test fails even though the real application works.

3. **Timing fragility.** A test clicks a button before an async operation completes. It worked before because the operation was fast; a backend change made it slower. The test needs a wait condition, not a code fix.

4. **Mock/fixture staleness.** Test mocks return data in a shape the API no longer produces. The test's mock says `{status: "active"}` but the real API now returns `{state: "active"}`. The test passes with mocks but fails against the real backend.

5. **Environment drift.** The test depends on a specific database state, a feature flag setting, or a third-party service response that changed between environments.

Today, when an E2E test fails, a human developer must:
- Read the test output to understand the failure
- Open DevTools to inspect browser state
- Compare what the test expected vs. what the browser shows
- Determine whether the test, the application, or both need updating
- Make the fix and re-run

This is exactly the kind of diagnostic work an AI agent excels at -- if it has visibility into browser state. Kaboom provides that visibility. Agentic E2E Repair defines the workflow for an AI agent to use Kaboom's observation, generation, and interaction capabilities to close the loop from "test failed" to "test fixed."

## Solution

Agentic E2E Repair is an **agent workflow pattern** that orchestrates existing Kaboom MCP tools, not a new MCP tool itself. The AI agent follows a structured diagnosis-and-repair loop:

1. **Detect** -- The agent learns that an E2E test failed (from test runner output, CI notification, or developer instruction).
2. **Re-run with capture** -- The agent re-runs the failing test with Kaboom capturing browser telemetry (console logs, network requests, DOM state, errors).
3. **Observe** -- The agent uses Kaboom's `observe` tool to inspect what the browser saw: errors, network responses, DOM state, API schemas, error clusters.
4. **Diagnose** -- The agent correlates the test's expected behavior with the observed browser state to identify the root cause category (selector drift, API change, timing issue, mock staleness, or true regression).
5. **Generate fix** -- The agent uses Kaboom's `generate` tool to produce a targeted repair (updated selector, corrected assertion, added wait condition, refreshed mock data).
6. **Verify** -- The agent re-runs the test to confirm the fix resolves the failure without introducing new failures.
7. **Report** -- The agent provides a structured explanation of what broke, why, and what was changed.

Kaboom's role is to provide the **observation primitives** that make steps 3 and 4 possible. Without Kaboom, the agent sees only "test failed with exit code 1." With Kaboom, the agent sees the actual DOM, the actual API responses, the actual console errors -- the same evidence a human developer would use in DevTools.

## User Stories

- As an AI coding agent, I want to observe the browser state during a failing E2E test so that I can diagnose why the test failed.
- As an AI coding agent, I want to compare the test's expected DOM selectors against the actual DOM so that I can detect selector drift.
- As an AI coding agent, I want to compare the test's expected API response shape against the actual network traffic so that I can detect API contract drift.
- As an AI coding agent, I want to see error clusters from the test run so that I can identify root causes rather than investigating each error independently.
- As an AI coding agent, I want to generate a corrected test file with updated selectors, assertions, or wait conditions so that the test passes against the current application state.
- As an AI coding agent, I want to re-run the test after applying a fix so that I can verify the repair worked.
- As a developer, I want the AI to explain what it changed and why so that I can review the fix with confidence.
- As a developer, I want E2E test failures to be automatically triaged into categories (test bug vs. app bug) so that I know whether to review a test fix or investigate a regression.

## MCP Interface

Agentic E2E Repair does not introduce new MCP tools or modes. It composes existing tools in a specific sequence. The following table maps each step of the repair workflow to the Kaboom tools involved.

### Workflow-to-Tool Mapping

| Workflow Step | Tool | Mode/Action | Purpose |
|---|---|---|---|
| Re-run with capture | `interact` | `navigate` | Open the application URL the test targets |
| Observe errors | `observe` | `errors` | Console errors and unhandled exceptions during test |
| Observe network | `observe` | `network_waterfall` | HTTP requests/responses, status codes, timing |
| Observe API responses | `observe` | `network_bodies` | Actual response payloads for comparison |
| Observe API schema | `observe` | `api` | Inferred schema from traffic vs. expected contract |
| Observe DOM | `analyze` | `dom` | Current DOM state for selector verification |
| Observe error clusters | `observe` | `error_clusters` | Grouped errors pointing to single root cause |
| Observe changes | `observe` | `changes` | What changed since last known good state |
| Validate API contract | `configure` | `validate_api` | Detect shape_change, type_change, new_field, null_field violations |
| Compare sessions | `configure` | `diff_sessions` | Compare passing vs. failing run states |
| Generate test fix | `generate` | `test` | Produce corrected Playwright/Cypress test code |
| Generate reproduction | `generate` | `reproduction` | Steps-to-reproduce for the failure |
| Execute verification | `interact` | `execute_js` | Run JavaScript in page to verify DOM state |
| Navigate for re-run | `interact` | `navigate`, `refresh` | Re-run test scenario |
| Save/load state | `interact` | `save_state`, `load_state` | Snapshot browser state before/after fix |
| Clear for re-run | `configure` | `clear` | Reset Kaboom buffers between test runs |

### Example Diagnosis Sequence

Step 1: Agent observes errors from the failing test run.
```
observe({ what: "errors" })
```

Step 2: Agent observes network traffic to see what the API actually returned.
```
observe({ what: "network_bodies", url_filter: "/api/users" })
```

Step 3: Agent validates the API contract against what the test expected.
```
configure({ action: "validate_api", operation: "analyze" })
```

Step 4: Agent queries the DOM to check whether the expected selector exists.
```
analyze({what: "dom", selector: ".submit-btn" })
```

Step 5: Agent generates a corrected test based on observations.
```
generate({format: "test", include_fixtures: true })
```

## Requirements

| # | Requirement | Priority |
|---|---|---|
| R1 | The agent must be able to observe console errors, network traffic, and DOM state during a failing E2E test run via existing Kaboom tools. | must |
| R2 | The agent must be able to classify the failure into a root cause category: selector drift, API contract drift, timing fragility, mock staleness, or true regression. | must |
| R3 | The agent must be able to generate a corrected test file using the `generate` tool with `format: "test"`. | must |
| R4 | The agent must verify the fix by re-running the test and confirming it passes. | must |
| R5 | The agent must provide a structured explanation of the diagnosis and fix (which root cause category, what changed, why the fix is correct). | must |
| R6 | The agent must use error clustering to avoid investigating the same root cause multiple times when multiple tests fail from a single change. | should |
| R7 | The agent must compare API schema from observed traffic against the test's expectations using `validate_api`. | should |
| R8 | The agent must query the DOM to verify selector existence before proposing selector updates. | should |
| R9 | The agent must use `diff_sessions` to compare a known-good run against the failing run when session data is available. | should |
| R10 | The agent must respect a circuit breaker: maximum 3 fix attempts per test before escalating to a human. | should |
| R11 | The agent should be able to batch-repair related failures (multiple tests broken by the same API change) rather than fixing each independently. | could |
| R12 | The agent should generate a PR summary or commit message that explains the batch of fixes. | could |

## Non-Goals

- This feature does NOT modify the Kaboom MCP server or extension. It is a workflow pattern using existing tools.
- This feature does NOT create a new MCP tool. The 5-tool maximum is strictly preserved.
- This feature does NOT auto-commit fixes without human review. The agent proposes fixes; a human (or a CI approval gate) decides to merge.
- This feature does NOT handle non-browser tests (unit tests, API tests, integration tests without a browser). It requires Kaboom's browser observation layer.
- This feature does NOT replace CI infrastructure. It assumes a test runner (Playwright, Cypress, Selenium) already exists and can be invoked by the agent.
- This feature does NOT perform proactive test maintenance. It is reactive: it activates when a test fails. Proactive maintenance belongs to the Self-Healing Tests feature.
- Out of scope: fixing application code. If the root cause is a true regression in the application (not a stale test), the agent reports the regression but does not attempt to fix production code.

## Diagnosis Framework

The agent classifies each failure into one of five root cause categories. Each category has distinct diagnostic signals and fix strategies.

### Category 1: Selector Drift

#### Diagnostic signals:
- Test fails with "element not found" or "locator timeout"
- `analyze({what: "dom"})` with the test's selector returns empty
- `analyze({what: "dom"})` with a relaxed selector (text content, role, partial class) finds a matching element
- `observe({ what: "changes" })` shows DOM structure changed recently

#### Fix strategy:
- Update the selector to match the current DOM structure
- Prefer stable selectors: `data-testid`, ARIA roles, text content over CSS classes or XPath
- If the element was removed (not renamed), escalate as a true regression

### Category 2: API Contract Drift

#### Diagnostic signals:
- Test assertions on response body fields fail
- `observe({ what: "network_bodies" })` shows the actual response has different field names or structure
- `configure({ action: "validate_api", operation: "analyze" })` reports `shape_change`, `type_change`, or `new_field` violations
- `observe({ what: "api" })` shows the inferred schema differs from the test's expectations

#### Fix strategy:
- Update test assertions to match the new API shape
- Update test mocks/fixtures to return the new shape
- If both test and application code reference the old shape, prefer updating the test to match observed reality (the browser works; the test is wrong)
- If the API change appears unintentional (error responses, missing data), escalate as a true regression

### Category 3: Timing Fragility

#### Diagnostic signals:
- Test fails intermittently (passes on retry)
- `observe({ what: "network_waterfall" })` shows slow responses overlapping with the test's action
- The test uses hard-coded waits or no waits at all
- `observe({ what: "errors" })` shows errors related to incomplete loading ("Cannot read property of null" on data that should exist)

#### Fix strategy:
- Add explicit wait conditions (`waitForResponse`, `waitForSelector`, `waitForLoadState`)
- Replace hard-coded `setTimeout` with condition-based waits
- Add retry logic for flaky network conditions

### Category 4: Mock/Fixture Staleness

#### Diagnostic signals:
- Test mocks return data that does not match the actual API shape
- `observe({ what: "network_bodies" })` shows real responses differ from mock data
- `configure({ action: "validate_api" })` shows discrepancies between learned schema and test expectations
- The test passes with mocks but fails against the real backend

#### Fix strategy:
- Update mock data to match the current API response shape
- Update fixture files with the correct field names, types, and structures
- If the test suite uses a shared mock factory, update the factory rather than individual mocks

### Category 5: True Regression

#### Diagnostic signals:
- The test's assertions match the documented API contract, but the application behavior changed
- `observe({ what: "errors" })` shows application-level errors (not test errors)
- `observe({ what: "error_clusters" })` shows a new cluster that did not exist in the last known-good run
- The failure correlates with a recent code change (not a test change)

#### Fix strategy:
- Do NOT fix the test. The test is correctly detecting a regression.
- Report the regression with full diagnostic evidence: error cluster, network diff, DOM state
- Generate a reproduction script via `generate({format: "reproduction" })`
- Escalate to the developer who made the recent change

## Batch Repair Workflow

When multiple E2E tests fail simultaneously, the agent should avoid fixing each independently. Instead:

1. Run all failing tests with Kaboom capture.
2. Collect error clusters across all test runs.
3. Group failures by root cause (same API change, same selector rename, same timing issue).
4. Fix the root cause once (update the shared mock, update the shared selector pattern).
5. Re-run all affected tests to verify the single fix resolves the batch.

This prevents the agent from making redundant changes (updating the same field name in 15 different test files one at a time) and ensures consistency.

## Relationship to Other Features

### vs. Self-Healing Tests

Self-Healing Tests is **proactive**: it monitors test health over time, detects flake patterns, and hardens tests before they break. Agentic E2E Repair is **reactive**: it activates after a test has already failed and produces a specific fix.

They are complementary. Self-Healing Tests reduces the frequency of failures. Agentic E2E Repair reduces the time to resolve failures that still occur.

### vs. Self-Testing

Self-Testing enables AI to run Kaboom's own test suite (UAT) against itself. Agentic E2E Repair applies to the user's application tests, not Kaboom's. However, both use the same underlying MCP tools for observation and verification.

### vs. Reproduction Enhancements

Reproduction Enhancements generate Playwright scripts that reproduce a bug. Agentic E2E Repair consumes the same observation data but generates **fixes** rather than reproductions. When the agent determines a failure is a true regression (not a test bug), it falls back to generating a reproduction script using the reproduction feature.

### vs. Error Clustering

Error Clustering is a dependency. The agent uses error clusters to group related failures and identify root causes. Without clustering, the agent would investigate each error independently, wasting tokens and time.

### vs. DOM Fingerprinting

DOM Fingerprinting provides stable element identification across DOM changes. The agent uses DOM fingerprinting (when available) to map the test's stale selectors to their current equivalents. This is the mechanism behind selector drift detection in Category 1.

### vs. API Schema Inference / validate_api

API Schema Inference and validate_api are dependencies. The agent uses the inferred API schema and contract validation to detect API drift (Category 2). Without these, the agent could only compare raw JSON -- with them, it understands structural changes.

## Performance SLOs

| Metric | Target |
|---|---|
| Diagnosis time per failing test | < 30 seconds (observation + analysis) |
| Fix generation time | < 10 seconds (generate tool call) |
| Verification time | Determined by test runner (outside Kaboom's control) |
| Server memory impact | Zero additional (uses existing buffers) |
| Token efficiency | Batch repair should use < 2x tokens of single repair for N related failures |

Note: Diagnosis and fix generation times refer to the Kaboom tool calls, not the AI's reasoning time. The AI's reasoning is outside Kaboom's control.

## Security Considerations

- **No new attack surface.** This feature uses existing MCP tools with existing security boundaries (localhost-only, opt-in body capture, header stripping).
- **Test code, not production code.** The agent modifies test files, not application source code. If the agent determines a true regression, it reports rather than fixes.
- **Human review gate.** The agent proposes fixes (via commit, PR, or direct output) but does not auto-merge. A human or CI gate must approve changes.
- **Credential safety.** Test fixtures generated from observed API traffic must not include authentication tokens, session cookies, or API keys. The existing header stripping and redaction patterns apply.
- **Circuit breaker.** The agent stops after 3 failed fix attempts to prevent infinite fix-break loops.
- **Audit trail.** All Kaboom tool calls are logged via the enterprise audit feature. The agent's diagnostic reasoning should be included in the commit message or PR description for traceability.

## Edge Cases

- **No Kaboom data captured during test run.** Expected behavior: The agent cannot diagnose without observations. It reports "insufficient data" and suggests re-running with Kaboom capture enabled. Precondition: Kaboom server must be running and the extension must be tracking the test browser tab.

- **Test fails with no browser errors and no network failures.** Expected behavior: The agent queries the DOM to check selector validity. If selectors are valid and API responses are correct, the agent classifies this as a potential timing issue and examines the test's wait conditions.

- **Multiple root causes in a single test failure.** Expected behavior: The agent addresses root causes in dependency order (API contract first, then selectors, then timing). It generates a single fix that addresses all causes.

- **Test framework not recognized.** Expected behavior: The agent can still observe browser state but generates generic JavaScript fixes rather than framework-specific ones (Playwright vs. Cypress vs. Selenium). The `generate({format: "test" })` tool handles framework detection.

- **Kaboom extension disconnected during test run.** Expected behavior: Observation data is partial or missing. The agent falls back to test runner output only and reports that Kaboom capture was incomplete.

- **Buffer overflow during long test suite.** Expected behavior: Kaboom's ring buffers evict oldest entries. The agent should clear buffers between test runs using `configure({ action: "clear" })` to ensure relevant data is available.

- **Test was already flaky before the agent's change.** Expected behavior: The agent should run the test at least twice after fixing. If it passes intermittently, classify as a flake and report rather than claiming a fix.

- **Fix breaks a different test.** Expected behavior: Circuit breaker activates after 3 attempts. The agent reports "fix introduced a regression in test X" and escalates to human review.

- **API returns errors (500) rather than changed data.** Expected behavior: The agent distinguishes between "the API contract changed" (fix the test) and "the API is broken" (true regression). A 500 response with an error body is a regression; a 200 response with different fields is contract drift.

## Dependencies

- **Depends on:**
  - `observe` tool (errors, network_waterfall, network_bodies, api, error_clusters, changes) -- shipped
  - `generate` tool (test, reproduction) -- shipped
  - `analyze` tool (`what:"dom"`) -- shipped
  - `configure` tool (`diff_sessions`, `clear`) -- shipped
  - `interact` tool (navigate, refresh, execute_js, save_state, load_state) -- shipped
  - Error Clustering feature -- shipped
  - API Schema Inference feature -- shipped
  - Reproduction Enhancements feature -- shipped
  - DOM Fingerprinting feature -- in-progress (enhances selector drift detection but not strictly required)
  - A test runner (Playwright, Cypress, Selenium) -- external dependency, not Kaboom's responsibility

- **Depended on by:**
  - Kaboom CI integration (runs this workflow in CI pipelines)
  - Self-Healing Tests (may invoke E2E Repair as a sub-workflow for individual test fixes)

## Assumptions

- A1: The Kaboom server is running and the browser extension is tracking the tab where the E2E test executes.
- A2: The test runner produces machine-readable output that the agent can parse to identify which test failed and with what error message.
- A3: Network body capture is enabled for the test run (required for API contract drift detection).
- A4: The test repository is accessible to the agent for reading test source code and writing fixes.
- A5: The agent has context on the project's test framework (Playwright, Cypress, etc.) to generate framework-appropriate fixes.
- A6: The application under test runs on localhost or a URL accessible from the Kaboom-instrumented browser.

## Open Items

| # | Item | Status | Notes |
|---|---|---|---|
| OI-1 | How does the agent receive test failure notifications? | open | Options: (a) agent parses test runner stdout, (b) CI webhook, (c) developer manually invokes the agent. The product spec should not mandate one approach -- all three should be supported. |
| OI-2 | Should the agent attempt to fix tests in frameworks it does not recognize? | open | Risk of generating syntactically invalid test code. May be safer to only generate fixes for recognized frameworks (Playwright, Cypress, Selenium) and report-only for others. |
| OI-3 | What is the maximum number of files the agent should modify in a single repair? | open | The principal engineer review flagged unbounded codebase search. A threshold of 10 files before requiring human approval is proposed but not finalized. |
| OI-4 | How does the agent decide between "update test" vs. "update application code" when an API contract change is detected? | open | Default: always update the test. If the API change was unintentional (error responses, missing data), escalate rather than fix. Decision tree needs refinement based on real-world usage. |
| OI-5 | Should the agent track fix success rates over time to improve its diagnosis heuristics? | open | Useful for learning which fix strategies work for a given codebase, but introduces statefulness beyond the session-scoped model. Could use Kaboom's persistent memory (configure store/load). |
| OI-6 | How does the agent handle tests that require a specific database state or third-party service? | open | Kaboom observes the browser, not the database. The agent can infer data dependencies from API responses but cannot directly provision test data. May depend on Reproduction Enhancements' fixture generation. |
| OI-7 | Should this workflow be packaged as a Claude Code skill file? | open | The legacy spec proposed a `.claude/skills/self-heal.yaml` format. Whether this becomes a formal skill or remains a documented workflow pattern depends on Claude Code's skill system maturity. |
| OI-8 | How does the agent handle parallel test runners (multiple Playwright workers)? | open | Multiple workers POSTing to the same Kaboom server could cause cross-contamination of telemetry. The principal engineer review recommended scoping contract tracking to test_id. This requires the test runner to tag each request with a test identifier. |
