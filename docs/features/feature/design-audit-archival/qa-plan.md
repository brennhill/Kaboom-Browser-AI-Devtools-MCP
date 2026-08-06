---
status: proposed
version-applies-to: v6.6
scope: feature/design-audit-archival/qa
ai-priority: medium
tags: [qa, testing, design-audit, archival, placeholder]
relates-to: [product-spec.md, tech-spec.md, feature-tracking.md]
last-verified: 2026-03-05
incomplete: true
doc_type: qa-plan
feature_id: feature-design-audit-archival
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Design Audit & Archival

**Status:** PLACEHOLDER — Waiting for Phase 1 implementation
**Target Version:** v6.6
**Based on:** feature-tracking.md

---

## Overview

QA plan for design audit and archival feature. Test scenarios will be expanded once Phase 1 implementation begins.

---

## Planned Test Categories

### 1. **Capture & Storage Tests** (HIGH PRIORITY)

Scenarios to cover:
- ✓ Single viewport capture (desktop)
- ✓ Multiple viewport capture (desktop, tablet, mobile in parallel)
- ✓ Screenshot format validation (PNG/WebP)
- ✓ File storage in correct directory structure
- ✓ Metadata stored correctly in database
- ✓ Concurrent captures (multiple agents simultaneously)
- ✓ Large screenshot handling (1MB+ files)

### 2. **Query Tests**

Scenarios to cover:
- ✓ Query by component name
- ✓ Query by component + variant
- ✓ Query by viewport
- ✓ Query by URL
- ✓ Query by date range
- ✓ Query with limit and offset
- ✓ Query response includes correct metadata
- ✓ Large result set pagination

### 3. **Cleanup Tests** (HIGH PRIORITY)

Scenarios to cover:
- ✓ Age-based cleanup (delete older than N days)
- ✓ Space-based cleanup (delete when exceeding 5GB)
- ✓ Orphan cleanup (remove files without DB entries)
- ✓ Cleanup doesn't delete too recent files
- ✓ Cleanup preserves database integrity
- ✓ Cleanup handles corrupted files gracefully
- ✓ Recovery from cleanup interruption

### 4. **Database Index Tests**

Scenarios to cover:
- ✓ Index correctness for component + variant queries
- ✓ Index correctness for URL + timestamp queries
- ✓ Index performance (< 100ms for typical queries)
- ✓ No duplicate screenshots stored
- ✓ Referential integrity (no orphaned DB entries)

### 5. **Performance Tests**

Scenarios to cover:
- ✓ Capture latency < 1 second (3 viewports parallel)
- ✓ Query latency < 500ms (10K screenshots)
- ✓ Memory usage stable during 1000+ captures
- ✓ No performance regression in other MCP tools
- ✓ Disk I/O doesn't block observe() calls

### 6. **Edge Cases**

Scenarios to cover:
- ✓ Empty database queries
- ✓ Invalid component/variant names (special chars)
- ✓ Metadata exceeding 5KB (should truncate or error)
- ✓ Disk space exhaustion (other processes)
- ✓ Database corruption recovery
- ✓ Symlink attack in file storage path
- ✓ Storage limit edge cases (exactly at 5GB)

### 7. **Integration Tests**

Scenarios to cover:
- ✓ Capture + upload + query workflow
- ✓ Multiple agents querying same data
- ✓ Design regression detection (compare 2 screenshots)
- ✓ Cleanup during active queries (concurrency)

---

## Regression Tests

All existing MCP tools must work normally:
- ✓ observe() modes unchanged
- ✓ configure() commands work
- ✓ No impact on logs, network, actions, websocket capture
- ✓ No degradation in performance < 5%

---

## Load Tests

- ✓ Store 10,000+ screenshots
- ✓ Query performance still < 500ms
- ✓ Cleanup completes within 5 seconds
- ✓ Database file < 500MB (with 10K screenshots)

---

## TODO: Complete This QA Plan

Once Phase 1 implementation begins:

1. Define specific test data (component names, viewports, dates)
2. Create Playwright test scenarios for capture
3. Create SQL queries to validate database state
4. Define performance baselines and thresholds
5. Establish automated test suite
6. Create manual testing checklist for release

---

## Related Documents

- **product-spec.md** — Feature requirements
- **tech-spec.md** — Technical implementation
- **feature-tracking.md** — Phase breakdown
- **../../../core/quality/uat-v5.3-checklist.md** — UAT patterns for reference

---

## Success Criteria

The feature is ready for release when:
- [ ] All 50+ test scenarios pass
- [ ] No regressions in existing MCP tools
- [ ] Performance targets met (capture < 1s, query < 500ms)
- [ ] Cleanup verified accurate (no orphans, respects quotas)
- [ ] Database integrity verified (no duplicates, referential integrity)
