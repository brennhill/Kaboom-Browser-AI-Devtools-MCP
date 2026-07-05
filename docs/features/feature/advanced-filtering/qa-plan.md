---
status: proposed
scope: feature/advanced-filtering/qa
ai-priority: medium
tags: [qa, testing, advanced-filtering, placeholder]
relates-to: [feature-proposal.md, tech-spec.md]
version-applies-to: v6.0+
last-verified: 2026-03-05
incomplete: true
doc_type: qa-plan
feature_id: feature-advanced-filtering
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Advanced Filtering for Signal-to-Noise

**Status:** PLACEHOLDER — Waiting for detailed spec review
**Based on:** feature-proposal.md
**Target Version:** v6.0+

---

## Overview

This is a placeholder QA plan. Test scenarios will be defined once the technical specification (tech-spec.md) is finalized.

---

## Planned Test Categories

### 1. **Domain Filtering Tests** (HIGH PRIORITY)

Scenarios to cover:
- ✓ Single domain blocked
- ✓ Multiple domains blocked (comma-separated, array)
- ✓ Exact match vs. wildcard/regex behavior
- ✓ Case sensitivity
- ✓ Subdomain handling (should `example.com` block `analytics.example.com`?)
- ✓ Port numbers in domains (should `example.com:8080` be treated differently?)

### 2. **Whitelist vs. Blacklist Behavior**

Scenarios to cover:
- ✓ `blocked_domains` only
- ✓ `allowed_domains` only
- ✓ Both specified (what takes precedence?)
- ✓ Empty blocklist/allowlist (should this be an error or no-op?)

### 3. **Data Integrity Tests**

Scenarios to cover:
- ✓ Filtered requests are not returned in responses
- ✓ Filtered requests are not counted in metadata
- ✓ Total request count is accurate (do we report "10 out of 20 shown" or just "10"?)
- ✓ Pagination works correctly with filtering (does cursor account for filtered items?)

### 4. **Performance Tests**

Scenarios to cover:
- ✓ Filter overhead with small blocklist (< 10 domains)
- ✓ Filter overhead with large blocklist (1000+ domains)
- ✓ Memory usage with complex regex patterns
- ✓ No performance regression for non-filtering requests

### 5. **Edge Cases**

Scenarios to cover:
- ✓ Empty domain string (`""`)
- ✓ Malformed domains (`@#$%`)
- ✓ Localhost/127.0.0.1 filtering
- ✓ Ports and paths in domain filters
- ✓ Unicode/IDN domains

### 6. **Backward Compatibility**

Scenarios to cover:
- ✓ Old clients without filtering params work normally
- ✓ New params ignored by old clients gracefully
- ✓ Defaults to no filtering (backward compatible)

---

## Regression Tests

All existing pagination and buffer-clearing tests must pass:
- ✓ Cursor-based pagination still works with filtering
- ✓ Buffer-clearing doesn't interfere with filtering
- ✓ Multiple concurrent filters work correctly

---

## TODO: Complete This QA Plan

To complete this QA plan, once tech-spec.md is finalized:

1. Define specific test data (URLs, domains, expected outcomes)
2. Specify test tools (manual, automated Playwright, API testing)
3. Set performance targets (max filter latency)
4. Define acceptance criteria for each test category
5. Create test cases with step-by-step procedures
6. Establish baseline metrics for regression testing

---

## Related Documents

- **feature-proposal.md** — Feature rationale and requirements
- **tech-spec.md** — Technical implementation details
- **ADR-advanced-filtering.md** — Architecture decisions
- **../uat-v5.3-checklist.md** — Current UAT patterns for reference

---

## Success Criteria

The feature is ready for release when:
- [ ] All 40+ test scenarios pass
- [ ] No regressions in existing pagination/buffer-clearing
- [ ] Performance overhead < 5ms per request
- [ ] 100% backward compatibility verified
