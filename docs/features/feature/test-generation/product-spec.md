---
feature: test-generation
status: proposed
tool: generate
mode: [test_from_context, test_heal, test_classify]
version: 0.7.12
doc_type: product-spec
feature_id: feature-test-generation
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Test Generation from Context

## Problem Statement

Developers using AI coding assistants need automated test generation that validates code changes. Currently:

1. **TestSprite** (competitor) offers AI-driven test generation but requires cloud, costs $29-99/month, and starts "blind" without runtime context
2. Kaboom captures comprehensive error context but cannot generate tests from it
3. Manual test writing is slow and error-prone
4. Generated tests break when selectors change (no self-healing)

This creates a gap: Kaboom has the best error context, but developers must use a separate tool (TestSprite) for test generation.

## Solution

Extend Kaboom's `generate` tool with three new modes for the complete validation loop:

1. **`test_from_context`** — Generate tests from captured error/interaction context
2. **`test_heal`** — Auto-repair broken selectors in existing tests
3. **`test_classify`** — Classify test failures (real bug vs flaky vs environment)

**Key advantage over TestSprite:** Kaboom already has the error context (console, network, DOM, state). Tests are generated from facts, not guesses.

## Requirements

### Functional Requirements

#### 1. Test Generation Mode (`test_from_context`)

Generate Playwright/Vitest tests from captured Kaboom context.

##### Actions:
- `test_from_context.error` — Generate test that reproduces a captured error
- `test_from_context.interaction` — Generate test from recorded user interaction
- `test_from_context.regression` — Generate regression test from analyze baseline

##### Request:
```json
{
  "type": "test_from_context",
  "context": "error",
  "error_id": "err_abc123",
  "framework": "playwright",
  "output_format": "file"
}
```

##### Response:
```json
{
  "status": "success",
  "test": {
    "framework": "playwright",
    "filename": "error-form-validation.spec.ts",
    "content": "import { test, expect } from '@playwright/test';\n\ntest('reproduces form validation error', async ({ page }) => {\n  await page.goto('/signup');\n  await page.fill('#email', 'invalid');\n  await page.click('button[type=submit]');\n  await expect(page.locator('.error')).toContainText('Invalid email');\n});",
    "selectors": ["#email", "button[type=submit]", ".error"],
    "assertions": 1,
    "coverage": {
      "error_reproduced": true,
      "network_mocked": false,
      "state_captured": true
    }
  },
  "metadata": {
    "source_error": "err_abc123",
    "generated_at": "2026-01-29T10:30:00Z",
    "context_used": ["console", "dom", "network"]
  }
}
```

##### Supported Frameworks:
- `playwright` (default) — Full browser testing
- `vitest` — Unit/integration tests
- `jest` — Unit tests

##### Context Sources:
- Error context from `observe` (console errors, network failures)
- Interaction recordings from `interact` (user actions)
- Regression baselines from `analyze` (before/after states)

#### 2. Test Healing Mode (`test_heal`)

Auto-repair broken selectors in existing tests.

##### Actions:
- `test_heal.analyze` — Identify broken selectors in test file
- `test_heal.repair` — Generate fixed selectors using current DOM
- `test_heal.batch` — Heal all broken tests in directory

##### Request:
```json
{
  "type": "test_heal",
  "action": "repair",
  "test_file": "tests/login.spec.ts",
  "broken_selectors": ["#old-login-btn", ".deprecated-form"]
}
```

##### Response:
```json
{
  "status": "success",
  "healed": [
    {
      "old_selector": "#old-login-btn",
      "new_selector": "button[data-testid='login']",
      "confidence": 0.95,
      "strategy": "testid_fallback",
      "line_number": 12
    },
    {
      "old_selector": ".deprecated-form",
      "new_selector": "form[action='/auth/login']",
      "confidence": 0.87,
      "strategy": "attribute_match",
      "line_number": 8
    }
  ],
  "unhealed": [],
  "updated_content": "...(full test file with fixes)..."
}
```

##### Healing Strategies (priority order):
1. `testid_match` — Look for data-testid/data-test attributes
2. `aria_match` — Match by aria-label, role
3. `text_match` — Match by visible text content
4. `attribute_match` — Match by other stable attributes (name, action, href)
5. `structural_match` — Match by DOM position (least reliable)

##### Confidence Thresholds:
- `>= 0.9` — Auto-apply fix
- `0.7-0.9` — Suggest fix, require confirmation
- `< 0.7` — Report as unhealed, manual review required

#### 3. Test Classification Mode (`test_classify`)

Classify test failures to reduce noise and prioritize real bugs.

##### Actions:
- `test_classify.failure` — Classify a specific test failure
- `test_classify.batch` — Classify all failures in test run

##### Request:
```json
{
  "type": "test_classify",
  "action": "failure",
  "failure": {
    "test_name": "should submit form successfully",
    "error": "Timeout waiting for selector '#submit-btn'",
    "screenshot": "base64...",
    "trace": "..."
  }
}
```

##### Response:
```json
{
  "status": "success",
  "classification": {
    "category": "selector_broken",
    "confidence": 0.92,
    "evidence": [
      "Selector '#submit-btn' not found in DOM",
      "Similar element found: 'button[type=submit]'",
      "Element was renamed in recent commit"
    ],
    "recommended_action": "heal",
    "is_real_bug": false,
    "is_flaky": false,
    "is_environment": false
  },
  "suggested_fix": {
    "type": "selector_update",
    "old": "#submit-btn",
    "new": "button[type=submit]"
  }
}
```

##### Classification Categories:
- `real_bug` — Actual application bug (highest priority)
- `selector_broken` — DOM changed, selector outdated
- `timing_flaky` — Race condition, needs wait/retry
- `network_flaky` — Network timeout, external dependency
- `environment` — CI/local environment difference
- `test_bug` — Bug in test logic itself

### Non-Functional Requirements

1. **Performance**
   - Test generation < 3 seconds
   - Selector healing < 1 second per selector
   - Classification < 2 seconds per failure

2. **Privacy**
   - All processing local (no cloud)
   - No test content sent externally
   - Secrets in tests flagged and redacted

3. **Quality**
   - Generated tests must be syntactically valid
   - Selectors prefer stable attributes (data-testid, aria-label)
   - Tests include meaningful assertions (not just "page loads")

### Out of Scope

- Running tests (use Playwright/Vitest directly)
- Visual regression testing (screenshot comparison)
- Cross-browser test matrix management
- Test parallelization/orchestration

## Success Criteria

1. **AI can generate tests from errors**
   - Captures error via `observe`
   - Generates test via `generate {format: "test_from_context"}`
   - Test reproduces the error when run

2. **Broken tests can be auto-healed**
   - AI identifies broken selector
   - Runs `generate {format: "test_heal"}`
   - Fixed test passes

3. **Failures are correctly classified**
   - Test fails with ambiguous error
   - Classification identifies root cause
   - Developer fixes right thing (not chasing flaky tests)

## User Workflow

### Generate Test from Error

```
Developer: "This form throws an error when I submit"
AI: [Uses observe to capture error context]
AI: [Runs generate({what: "test_from_context", context: "error"})]
Result: Playwright test that reproduces the error
Developer: Runs test, confirms it fails
Developer: Fixes bug
Developer: Runs test, confirms it passes
```

### Heal Broken Tests

```
CI: "5 tests failed after UI refactor"
AI: [Runs generate({what: "test_heal", test_file: "tests/*.spec.ts"})]
Result: "3 selectors healed with high confidence, 2 need manual review"
AI: Applies high-confidence fixes
Developer: Reviews and approves low-confidence fixes
CI: All tests pass
```

### Classify Test Failure

```
CI: "Test 'checkout flow' failed"
Developer: "Is this a real bug or flaky test?"
AI: [Runs generate({what: "test_classify", failure: {...}})]
Result: "Classification: timing_flaky (confidence 0.88). Element appears after animation, test doesn't wait."
AI: Suggests adding explicit wait
Developer: Applies fix, test becomes stable
```

## Relationship to Other Tools

| Tool | Role in Test Workflow |
|------|----------------------|
| `observe` | Capture error context for test generation |
| `analyze` | Create regression baselines, detect issues to test |
| `generate` | **Generate tests, heal selectors, classify failures** |
| `interact` | Record user interactions for test generation |
| `configure` | Set test generation preferences |

## Competitive Positioning

| Feature | TestSprite | Kaboom |
|---------|-----------|----------|
| Test generation | From PRD (blind) | From captured context (informed) |
| Self-healing | Yes (cloud) | Yes (local) |
| Failure classification | Yes | Yes |
| Privacy | Cloud-based | 100% local |
| Cost | $29-99/month | Free |
| Context quality | Limited | Comprehensive (console, network, DOM, state) |
| Framework support | Many | Playwright, Vitest, Jest |

## Notes

- Extends existing `generate` tool (no new tool needed)
- Leverages Kaboom's captured context advantage
- Local-only processing (privacy-first)
- Complements existing `analyze` regression detection
- AI decides when to generate vs heal vs classify based on context
