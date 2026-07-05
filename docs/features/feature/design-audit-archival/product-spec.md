---
status: proposed
version-applies-to: v6.6
scope: feature/design-audit-archival
ai-priority: medium
tags: [screenshots, design-regression, archival, placeholder]
relates-to: [feature-tracking.md, tech-spec.md, qa-plan.md]
last-verified: 2026-03-05
incomplete: true
doc_type: product-spec
feature_id: feature-design-audit-archival
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Specification: Design Audit & Archival

**Status:** APPROVED (Spec review complete)
**Target Version:** v6.6 — Specialized Audits & Analytics
**Effort:** Phase 1 = 2 weeks
**Based on:** feature-tracking.md

---

## Overview

A screenshot archival and queryable design system compliance tool that allows LLMs to:
- Capture page screenshots across responsive breakpoints (desktop, tablet, mobile)
- Archive with semantic metadata (component, variant, viewport, date, URL)
- Query historical screenshots for design regression analysis
- Monitor disk usage with automatic space-based cleanup (5GB default)

---

## User Stories

### Story 1: Design Regression Detection
**As an** AI coding assistant
**I want to** capture screenshots across breakpoints and compare against historical versions
**So that** I can detect design regressions and verify responsive behavior

#### Acceptance Criteria:
- Can capture 3 viewports (desktop, tablet, mobile) in parallel
- Each screenshot tagged with component, variant, viewport size, timestamp
- Can query by component name to see historical screenshots
- Comparison highlights visual changes between versions

---

### Story 2: Design System Compliance
**As a** design engineer
**I want to** archive all design variants and their visual appearance
**So that** I can verify compliance with design system spec

#### Acceptance Criteria:
- Screenshots automatically tagged with component & variant metadata
- Full-page and component-level capture supported
- Queryable by component, variant, viewport, date range
- Disk space managed automatically (5GB cap, oldest files deleted first)

---

### Story 3: Storage Management
**As an** infrastructure owner
**I want to** control storage usage without manual cleanup
**So that** the system doesn't consume unlimited disk space

#### Acceptance Criteria:
- Default storage limit: 5GB
- Automatic cleanup job (daily, 2 AM UTC)
- Age-based deletion if `max_age_days` configured
- Space-based deletion if exceeding `max_disk_bytes`
- Warnings in API responses when disk usage high (>80% of limit)

---

## Core API

### Capture Endpoint
```
POST /observe
Body: {
  what: "screenshots",
  viewports: ["desktop", "tablet", "mobile"],
  metadata: {
    component: "UserCard",
    variant: "selected",
    context: "Dashboard page"
  }
}

Response: {
  status: "ok" | "warn",
  stored_count: 3,
  disk_usage_percent: 65,
  warning?: "Disk usage at 65% of limit (5GB)"
}
```

### Query Endpoint
```
GET /observe?what=screenshots&component=UserCard&viewport=desktop

Response: {
  screenshots: [
    {
      id: "uuid",
      filepath: "/data/kaboom/screenshots/...",
      metadata: {component, variant, viewport, ...},
      timestamp: "2026-01-30T10:15:23Z",
      size_bytes: 245600
    }
  ],
  total_stored: 5892,
  disk_usage_percent: 65
}
```

---

## Technical Requirements

See [tech-spec.md](tech-spec.md) for:
- Database schema (SQLite with indexing strategy)
- File storage layout and cleanup algorithms
- Metadata validation and sanitization
- Performance targets

---

## Success Criteria

- ✓ Phase 1 complete: server + extension + tests
- ✓ Screenshot capture latency < 1 second per viewport
- ✓ Query response time < 500ms for typical dataset (10K screenshots)
- ✓ Disk cleanup accurate (no orphaned files)
- ✓ No regression in other MCP tools

---

## Related Documents

- **feature-tracking.md** — Detailed phase breakdown and deliverables
- **tech-spec.md** — Technical implementation specification
- **qa-plan.md** — Test scenarios (placeholder)
- **screenshot-archival-and-query.md** — Full feature design specification (if separate)
- **screenshot-archival-and-query-review.md** — Principal engineer review (if separate)

---

## Next Steps

1. Review feature-tracking.md phase breakdown
2. Finalize tech-spec.md with database schema
3. Create detailed implementation plan with milestones
4. Update qa-plan.md with test scenarios
