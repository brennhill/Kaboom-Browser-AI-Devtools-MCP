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

# Test Generation Feature — Open Questions

## Questions for User Review

### Q1: Error ID Format
The TECH_SPEC mentions `error_id` parameter but doesn't specify how errors are identified in the system. Currently, console errors don't have unique IDs.

#### Options:
1. Generate IDs when errors are captured (e.g., `err_<timestamp>_<hash>`)
2. Use timestamp + message hash as implicit ID
3. Reference errors by index (most recent = 0)

**Current Decision:** Will use option 2 (timestamp + message hash) for now. Can revisit if user prefers explicit IDs.

---

### Q2: Test Framework Defaults
PRODUCT_SPEC says Playwright is default. Should we also support:
- Vitest for unit tests (different from Playwright E2E)?
- Jest (older but still popular)?

**Current Decision:** Implementing Playwright first (Phase 1), adding Vitest/Jest support in later phases.

---

### Q3: Self-Healing Confidence Thresholds
TECH_SPEC specifies:
- `>= 0.9` → Auto-apply
- `0.7-0.9` → Suggest, require confirmation
- `< 0.7` → Report as unhealed

**Question:** Should auto_apply=true override confidence thresholds, or should we always require confirmation for low confidence?

**Current Decision:** Respecting confidence thresholds even with auto_apply=true. User can manually apply low-confidence fixes.

---

### Q4: Test File Parsing
For selector healing, we need to parse test files to find selectors.

#### Options:
1. Simple regex-based parsing (fast, may miss edge cases)
2. AST parsing with a TypeScript/JavaScript parser (accurate, requires dependency)

**Current Decision:** Using regex for Phase 1 (matches `page.locator('...')`, `getByTestId('...')`, etc.). Can add AST parsing later if needed.

---

### Q5: Secret Detection Patterns
TECH_SPEC lists patterns to detect. Should we:
1. Only warn (comment in test)?
2. Auto-redact and replace with placeholder?
3. Both (redact + warn)?

**Current Decision:** Auto-redact with placeholder `'[REDACTED]'` and add comment explaining redaction.

---

## Decisions Made Without User Input

1. **Using existing codegen.go infrastructure** for test generation base
2. **DOM queries via existing pilot.go** pattern for selector healing
3. **Blocking pattern for quick ops, async for batch** per TECH_SPEC concurrency model
4. **File structure: modular** (testgen.go, testgen_heal.go, testgen_classify.go)

---

## Implementation Notes

- Phase 1 (test_from_context.error) can start immediately
- Phase 2 (test_heal) needs DOM query infrastructure (already exists)
- Phase 3 (test_classify) is pure server-side pattern matching

No blockers identified - proceeding with implementation.
