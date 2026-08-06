---
status: proposed
scope: feature/agentic-cicd/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-agentic-cicd
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-agentic-cicd.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Agentic Cicd Review](./index.md).

# Tech Spec: Agentic CI/CD Loop

**Status:** Draft
**Phase:** 8
**Features:** 33-36

## Important: Architecture Clarification

**Phase 8 features are NOT Kaboom MCP tools.** They are:

1. **Claude Code skills** — Workflow definitions that orchestrate existing Kaboom tools
2. **CI integration scripts** — GitHub Actions / GitLab CI that spawn agents
3. **Webhook handlers** — Receive events and dispatch agents

Kaboom provides the **observation primitives**. Phase 8 composes them into autonomous workflows.

| Layer | Responsibility | Implementation |
|-------|----------------|----------------|
| Kaboom | Observe browser state | MCP tools (existing) |
| CI Infrastructure | Run tests with capture | `/snapshot`, `/clear`, `kaboom-ci.js` |
| Claude Code Skills | Orchestrate diagnosis + fix | `.claude/skills/*.md` |
| CI Integration | Trigger skills on failure | GitHub Actions, webhooks |

---

## Overview

Phase 8 features enable AI to **close the feedback loop autonomously**. Unlike traditional observability tools that surface problems for humans to fix, these features let AI agents observe, diagnose, and repair issues without human intervention.

This is the key differentiator: Kaboom doesn't just tell you what's wrong — it enables AI to fix it.

## Thesis

> AI will be the driving force in development.

Current AI coding assistants are reactive: they wait for humans to describe problems, then suggest fixes. Agentic CI/CD inverts this — AI proactively discovers problems through observation and autonomously implements solutions.

### Traditional flow:
```
Human notices bug → Human describes to AI → AI suggests fix → Human applies
```

### Agentic flow:
```
AI observes via Kaboom → AI diagnoses root cause → AI implements fix → AI verifies fix worked
```

## Features

### 33. Self-Healing Tests

**Problem:** Test failures in CI block deployments. Developers context-switch to investigate failures that are often:
- Flaky tests (timing, ordering)
- Stale selectors (UI changed)
- Contract changes (API responses evolved)

**Solution:** AI agent observes test failure, uses Kaboom to capture browser state during failure, diagnoses root cause, and fixes the test or underlying code.

**Implementation:** Claude Code skill, NOT a Kaboom tool.

#### Prerequisites:
- ✅ Tab targeting (Phase 0) — completed
- ✅ Verification loop (`verify_fix`) — completed
- ⚠️ Kaboom CI infrastructure — required for headless capture

#### CI Dependency Check:

The skill must verify Kaboom is running before attempting diagnosis:

```yaml
# In skill definition
preconditions:
  - check: http://127.0.0.1:7890/health
    expect: status 200
    on_fail: "Kaboom server not running. Start with: npx kaboom-mcp"
```

## Workflow:
```
1. CI runs tests, one fails
2. Agent receives failure notification (webhook or poll)
3. Agent checks Kaboom is running (GET /health)
4. Agent re-runs failing test with Kaboom capture enabled
5. Agent calls /snapshot to get browser state at failure
6. Agent observes: console errors, network requests, DOM state
7. Agent diagnoses: "Selector '.submit-btn' not found, button now has class '.btn-submit'"
8. Agent uses verify_fix {action: "start"} to capture baseline
9. Agent fixes: Updates test selector
10. Agent verifies: Re-runs test, calls verify_fix {action: "compare"}
11. Agent commits fix with explanation
```

## Kaboom tools used:
- `GET /snapshot` — All browser state at failure point (CI endpoint)
- `observe {what: "errors"}` — Console errors during test
- `observe {what: "network"}` — API responses
- `analyze({what: "dom"})` — Current DOM state
- `verify_fix` — Before/after comparison to confirm fix worked
- `analyze {target: "changes"}` — What changed since last passing run

**Key insight:** Kaboom's DOM observation and network capture give AI the same visibility a developer would have in DevTools, but programmatically accessible.

## Edge Cases:

| Case | Handling |
|------|----------|
| True regression (not test bug) | Check if code changed. If feature removed intentionally, flag for human review. |
| Cascading failures (10 tests fail from 1 cause) | Group by error signature before fix attempts. |
| Flaky test (passes on retry) | Track flake rate. If >10%, mark as flaky, don't "fix." |
| Protected files | Respect `.gitignore`, file permissions. Report instead of fail. |
| Infinite loop (fix breaks something else) | Circuit breaker: max 3 fix attempts per test per run. |

## Security:
- AI must not commit credentials (pre-commit hook required)
- Human approval gate for security-related test changes
- Audit trail of all AI-generated commits

---

### 34. Agentic E2E Repair

**Problem:** API contracts drift over time. Backend changes a field name, frontend tests break. Manual fix requires:
1. Finding which field changed
2. Updating all affected tests
3. Updating any mocks/fixtures

**Solution:** Specialized self-healing focused on API contract mismatches.

#### Workflow:
```
1. E2E test fails with "Cannot read property 'userName' of undefined"
2. Agent captures network response: {user_name: "Alice"}
3. Agent compares to test expectation: response.userName
4. Agent diagnoses: "API changed 'userName' to 'user_name' (snake_case migration)"
5. Agent searches codebase for all 'userName' references
6. Agent proposes: Update frontend to use 'user_name', OR update test mocks
7. Agent implements chosen fix
8. Agent verifies all affected tests pass
```

#### Kaboom tools used:
- `observe {what: "network"}` — Actual API response
- `analyze {target: "api"}` — Inferred API schema from traffic
- `validate_api` — Compare expected vs actual contract

**Key insight:** Kaboom's API schema inference from observed traffic means AI knows what the API *actually* returns, not what documentation *claims* it returns.

---

### 35. PR Preview Exploration

**Problem:** Code review catches logic errors but misses runtime bugs. Preview deployments exist but require manual testing.

**Solution:** When a PR is opened, AI agent autonomously:
1. Opens the preview URL
2. Explores the app (clicks, fills forms, navigates)
3. Observes for errors, regressions, accessibility issues
4. Reports findings or proposes fixes directly on the PR

#### Workflow:
```
1. PR opened → preview deployed at preview-123.example.com
2. Agent receives webhook notification
3. Agent opens preview URL in browser (via tab targeting)
4. Agent explores: clicks buttons, submits forms, navigates pages
5. Agent observes: console errors, failed requests, broken layouts
6. Agent captures: "TypeError on /checkout when cart is empty"
7. Agent diagnoses: Missing null check in CartTotal component
8. Agent comments on PR with fix, or pushes fix commit
```

#### Kaboom tools used:
- `browser_action {action: "navigate", url: "..."}` — Open preview
- `execute_javascript` — Interact with page
- `observe {what: "errors"}` — Runtime errors
- `analyze {target: "accessibility"}` — A11y issues
- `generate {format: "reproduction"}` — Steps to reproduce

**Key insight:** Tab targeting (Phase 0) enables this — AI can open a new tab and explore without disrupting the developer's work.

---

### 36. Deployment Watchdog

**Problem:** Production issues discovered hours/days after deploy when users complain. Monitoring tools alert on metrics but can't diagnose or fix.

**Solution:** Post-deployment, AI agent:
1. Takes a baseline snapshot before deploy
2. Takes a comparison snapshot after deploy
3. Continuously monitors for regressions
4. If regression detected: diagnoses, optionally triggers rollback

#### Workflow:
```
1. Pre-deploy: Agent captures session snapshot "pre-v2.3.0"
2. Deploy completes
3. Post-deploy: Agent captures session snapshot "post-v2.3.0"
4. Agent compares: diff_sessions("pre-v2.3.0", "post-v2.3.0")
5. Agent detects: "New console error: 'PaymentService is not defined'"
6. Agent diagnoses: "Bundle splitting broke PaymentService import"
7. Agent decides: Severity HIGH → trigger rollback
8. Agent executes: Calls deployment API to rollback
9. Agent notifies: Posts to Slack with diagnosis
```

#### Kaboom tools used:
- `diff_sessions` — Compare before/after state
- `observe {what: "errors"}` — New errors post-deploy
- `analyze {target: "performance"}` — Performance regressions
- `security_audit` — New security issues

**Key insight:** Session comparison (Phase 2) makes this possible — AI can objectively measure "did this deploy make things worse?"

---

## Dependencies

| Feature | Depends On |
|---------|------------|
| 33. Self-Healing Tests | Tab targeting (0), Verification loop (3) |
| 34. Agentic E2E Repair | API contract validation (2) |
| 35. PR Preview Exploration | Tab targeting (0), Browser actions |
| 36. Deployment Watchdog | Session comparison (4), Context streaming (5) |

Phase 8 features are **high-level orchestrations** built on Phase 0-7 primitives. They don't require new Kaboom capture mechanisms — they compose existing tools into autonomous workflows.

## Implementation Notes

### Not Kaboom features — Agent patterns

These features aren't MCP tools. They're **agent behavior patterns** that use existing Kaboom tools. Implementation lives in:

1. **Claude Code skills** — `/self-heal`, `/watch-deploy`
2. **CI integration scripts** — GitHub Actions that spawn agents
3. **Webhook handlers** — Receive events, dispatch agents

### Kaboom's role

Kaboom provides the **observation layer** that makes these workflows possible:

| Without Kaboom | With Kaboom |
|------------------|---------------|
| AI sees: "Test failed" | AI sees: Console error, network 404, DOM state |
| AI guesses at cause | AI diagnoses from evidence |
| AI can't verify fix worked | AI re-runs and observes success |

### Example: Self-Healing Test Skill

```yaml
# .claude/skills/self-heal.yaml
name: self-heal
description: Diagnose and fix failing tests
triggers:
  - webhook: ci/test-failure
workflow:
  - run_test_with_capture:
      test: ${{ trigger.test_name }}
      kaboom: true
  - observe_failure:
      tools: [errors, network, dom]
  - diagnose:
      prompt: "Analyze this test failure and identify root cause"
  - fix:
      prompt: "Implement a fix for the diagnosed issue"
  - verify:
      rerun_test: true
      require_pass: true
  - commit:
      message: "fix(test): ${{ diagnosis.summary }}"
```

## Success Metrics

| Metric | Target |
|--------|--------|
| Test failures auto-fixed | >50% of selector/contract issues |
| PR bugs caught pre-merge | >30% of runtime errors |
| MTTR for deploy regressions | <10 minutes (vs hours) |
| Developer interrupts avoided | >5 per week per team |

## Risks

1. **Over-fixing:** AI fixes symptoms not causes (e.g., updating selector when component was removed intentionally)
   - Mitigation: Require human approval for structural changes

2. **Flaky verification:** AI thinks fix worked but test is flaky
   - Mitigation: Run verification multiple times

3. **Infinite loops:** Fix breaks something else, AI fixes that, breaks another thing
   - Mitigation: Circuit breaker after N fix attempts

4. **Security:** AI has deploy/rollback access
   - Mitigation: Scoped permissions, audit trail, approval gates for production

## Competitive Landscape

| Tool | Observation | Diagnosis | Repair |
|------|-------------|-----------|--------|
| Sentry | Errors only | Stack trace | ❌ |
| Datadog | Metrics | Anomaly detection | ❌ |
| LaunchDarkly | Feature flags | ❌ | Rollback only |
| **Kaboom + AI** | Full browser state | AI reasoning | AI implements fix |

No existing tool combines deep browser observation with AI-driven repair. This is Kaboom's moat.

## Timeline

Phase 8 depends on Phases 0-7. Recommended order:

1. **Tab targeting (Phase 0)** — Required for 33, 35
2. **Session comparison (Phase 2)** — Required for 36
3. **API validation (Phase 1)** — Required for 34
4. **Self-Healing Tests (33)** — Highest impact, validates pattern
5. **PR Preview (35)** — Natural extension of 33
6. **E2E Repair (34)** — Specialized variant
7. **Deployment Watchdog (36)** — Production-grade, requires maturity
