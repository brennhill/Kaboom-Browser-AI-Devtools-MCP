---
status: proposed
scope: feature/design-audit-archival/tracking
ai-priority: medium
tags: [tracking]
relates-to: [product-spec.md]
last-verified: 2026-03-05
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Design Audit & Archival — Feature Tracking

**Roadmap Tier:** v6.6 — Specialized Audits & Analytics
**Status:** ✅ APPROVED (Spec review complete)
**Effort:** Phase 1 = 2 weeks (server + extension + tests)
**Phase 2:** Design diff visualization, regression detection, external storage backends

---

## Overview

Screenshot archival and queryable design system compliance tool that allows LLMs to:
- Capture page screenshots across responsive breakpoints (desktop, tablet, mobile)
- Archive with semantic metadata (component, variant, viewport, date, URL)
- Query historical screenshots for design regression analysis
- Monitor disk usage with automatic space-based cleanup (5GB default, unlimited age)

---

## Core Documents

| Document | Link | Purpose |
|----------|------|---------|
| **Design Spec** | [screenshot-archival-and-query.md](./index.md) | Feature specification, API design, data model, configuration |
| **Spec Review** | [screenshot-archival-and-query-review.md](./index.md) | Principal engineer review, critical issues, implementation guidance |

---

## Phase 1 Deliverables (Weeks 2-3)

### Server Implementation (Week 2)
- [ ] SQLite schema with composite indexes
  - `idx_component_variant_timestamp`
  - `idx_url_timestamp`
  - `idx_viewport`, `idx_timestamp`, `idx_filepath`
- [ ] `ScreenshotStore` interface (direct SQLite impl, no abstraction yet)
  - `Store(screenshot *Screenshot) error` — atomic file writes + batch inserts
  - `Query(query *QueryParams) ([]*Screenshot, error)` — execute queries
- [ ] HTTP handlers
  - `POST /screenshots` — batch upload with metadata validation
  - Extend `observe({what: 'screenshots'})` query support
- [ ] Cleanup job
  - Daily 2 AM UTC cleanup
  - Age-based: if `max_age_days` set
  - Space-based: delete oldest if exceeding `max_disk_bytes` (5GB default)
  - Disk usage monitoring + warnings in responses
- [ ] `RebuildScreenshotIndex()` CLI tool for corruption recovery

### Extension Implementation (Week 2-3)
- [ ] Parallel viewport capture
  - `Promise.all()` for 3 viewports simultaneously
  - Temp file storage during capture
- [ ] Batch upload
  - Single `POST /screenshots` with all viewports + shared metadata
  - Metadata validation (5KB cap, field name sanitization)
- [ ] Integration with pending queries
  - Follow existing focused `sync/server.js` and `sync/batchers.js` patterns
  - Handle disk warnings in responses

### Testing (Week 3)
- [ ] Unit tests (800 LOC)
  - Filename sanitization (path traversal, injection)
  - Metadata validation (size, field names)
  - Query filter logic
  - Index coverage verification
- [ ] Integration tests (1000 LOC)
  - Capture → store → query → delete lifecycle
  - Concurrent viewport uploads
  - All 7 query patterns from spec
  - TTL + space-based cleanup
  - DB corruption recovery
- [ ] Security tests (400 LOC)
  - SQL injection fuzzing
  - Path traversal attempts
  - Metadata injection
  - Payload size limits

#### Total: 2200+ LOC (tests), 1200+ LOC (implementation)

---

## Phase 1 Success Criteria

- ✅ Screenshots auto-captured in parallel across 3 viewports
- ✅ Batch uploads reduce total latency to ~1000-1500ms
- ✅ Atomic file writes prevent orphaned screenshots
- ✅ SQLite composite indexes enable fast queries (<500ms for 10K+ screenshots)
- ✅ LLM can query by: component, viewport, date range, URL, variant, custom metadata
- ✅ Query limits enforced (default 10, max 100 results)
- ✅ Disk usage warnings in all responses (when > 80% of limit)
- ✅ Auto-cleanup via daily job (space + age constraints)
- ✅ Recovery tool for index corruption
- ✅ Zero external dependencies

---

## Phase 2 Enhancements (Future)

| Feature | Effort | Priority |
|---------|--------|----------|
| Diff visualization (generate visual diffs for PRs) | 4-6h | Medium |
| Regression detection (auto-detect unexpected changes) | 6-8h | Medium |
| Pattern matching queries (`component: 'button*'`) | 2-3h | Low |
| Generalize `latest_per_variant` → `groupBy` parameter | 3-4h | Low |
| External storage (S3 + Postgres) | 20-30h | Low (enterprise) |
| Image compression (quality tuning, WebP) | 4-6h | Low |
| Advanced cleanup strategies (pluggable, least-used, by-component) | 6-8h | Low |

---

## Implementation Notes

### Critical Points from Review

1. **Metadata size limit:** 5KB per screenshot, validated at server
2. **Path traversal:** All fields (component, variant, viewport, sitename) sanitized before filename construction
3. **Atomic writes:** Temp file → rename pattern with recovery tool
4. **Batch uploads:** Extension captures all viewports in parallel, single POST transaction
5. **Separate mutex:** `screenshotMu` separate from `Capture.mu` to avoid blocking observers
6. **Query limits:** Default 10, max 100 results, response includes `total_available` for pagination
7. **Cleanup config:** Age-based (null = unlimited), space-based (5GB default), daily job, 80% warning threshold

### Key Files to Modify

- **cmd/browser-agent/main.go** — HTTP routes, screenshot handler
- **cmd/browser-agent/types.go** — Screenshot types, constants
- **cmd/browser-agent/queries.go** — Query handlers
- **extension/background/sync/server.js** and **batchers.js** — Parallel capture transport and batch upload
- **.claude/refs/architecture.md** — Document SQLite layer, concurrency model

---

## Roadmap Integration

**v6.6 (Post-Thesis, Tier 4):** Specialized Audits & Analytics
- Complements Performance, A11y, SEO audits
- Extends "Annotated Screenshots" with archival + queries
- Can ship in parallel with v6.5 (token efficiency) and other Tier 4 features
- Non-blocking for critical path (v6.0-6.2 thesis)

### Marketing Value:
- "Design regression testing with AI: auto-audit design system compliance across responsive variants"
- "Screenshot archives queryable by component, viewport, date — build design regression test suites automatically"

---

## Known Unknowns

1. **Image compression:** Start with JPEG baseline, optimize in Phase 2 if needed
2. **Diff visualization:** Phase 2 feature pending LLM demand
3. **External storage:** Enterprise-only (Phase 2), keep embedded SQLite for v6.6
4. **Metadata schema versioning:** Phase 2 concern when schema changes needed

---

## Success Metrics

- **Performance:** 3-viewport capture in <1500ms, query latency <500ms (10K screenshots)
- **Reliability:** Zero orphaned screenshots, successful recovery from DB corruption
- **Security:** All tests pass (path traversal, injection, metadata validation)
- **UX:** LLM receives clear disk usage warnings, can adjust capture frequency
- **Adoption:** Teams using for design regression testing in CI/test workflows

---

## Last Updated

**2026-01-30** — Spec approved, ready for implementation kick-off.
