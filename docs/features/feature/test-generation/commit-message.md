---
status: proposed
scope: feature/test-generation
ai-priority: medium
tags: [documentation]
last-verified: 2026-03-05
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Suggested Commit Message

```
feat(test-generation): Implement TestSprite-parity test generation with WebSocket advantage

Implements comprehensive test generation, self-healing, and failure classification
to achieve competitive parity with TestSprite, plus unique WebSocket monitoring.

## What's New

### 7 new modes for the `generate` tool:

1. test_from_context.error — Generate Playwright tests from console errors
2. test_from_context.interaction — Generate tests from user actions
3. test_from_context.regression — Generate regression tests from baselines
4. test_heal.analyze — Find selectors in test files
5. test_heal.repair — Heal broken selectors with confidence scoring
6. test_heal.batch — Heal entire test directories (max 20 files)
7. test_classify.failure — Classify test failures by pattern
8. test_classify.batch — Classify multiple failures at once

### Key capabilities:

- Automatic Playwright test generation from captured errors
- Self-healing tests with confidence-based selector repair
- Pattern-based failure classification (6 categories)
- Batch operations with safety limits
- Security: path validation, selector injection prevention
- Response format: mcpJSONResponse pattern

### Unique advantage over TestSprite:

WebSocket frame monitoring enables test generation for real-time apps
that TestSprite cannot handle (chat, multiplayer games, collaboration tools).

## Implementation

- Files: testgen.go (1,693 lines), testgen_test.go (2,996 lines)
- Tests: 77 comprehensive tests (all passing)
- Coverage: unit, integration, security, batch, error handling
- Security: validateTestFilePath(), validateSelector()
- Batch limits: 20 files, 500KB/file, 5MB total

## Documentation

Created comprehensive docs in docs/features/feature/test-generation/:
- product-spec.md — Feature requirements and competitive positioning
- tech-spec.md v1.1 — Technical implementation (10 critical issues resolved)
- review.md — Principal engineer review
- qa-plan.md — ~100 test cases
- uat-guide.md — Human testing scenarios
- validation-guide.md — Hands-on validation using demo site
- competitive-advantage.md — WebSocket advantage analysis
- status.md — Implementation status
- wake-up.md — Quick overview

## Validation Status

Implementation complete and tested (77/77 tests passing).
Ready for hands-on validation using ~/dev/kaboom-demos (34 intentional bugs).

See validation-guide.md for step-by-step validation plan (~2 hours).

## Breaking Changes

None. New modes extend existing `generate` tool using format parameter.

## Next Steps

1. Run validation (validation-guide.md)
2. Document validation results
3. Optional: Wire up DOM queries, error ID assignment, file writing
4. Ship v5.3.0

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>
```
