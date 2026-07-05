---
status: proposed
version-applies-to: v6.6
scope: feature/design-audit-archival
ai-priority: medium
tags: [design-audit, archival, sqlite, storage, placeholder]
relates-to: [product-spec.md, feature-tracking.md, qa-plan.md]
last-verified: 2026-03-05
incomplete: true
doc_type: tech-spec
feature_id: feature-design-audit-archival
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Technical Specification: Design Audit & Archival

**Status:** APPROVED (Spec review complete)
**Target Version:** v6.6
**Effort:** Phase 1 = 2 weeks (server + extension + tests)
**Based on:** feature-tracking.md

---

## Overview

Technical implementation for screenshot archival, metadata storage, and queryable design system compliance.

---

## Database Schema (SQLite)

### Main Table: `screenshots`

```sql
CREATE TABLE screenshots (
  id TEXT PRIMARY KEY,                      -- UUID
  filepath TEXT UNIQUE NOT NULL,            -- /data/kaboom/screenshots/...

  -- Metadata
  component TEXT NOT NULL,                  -- e.g., "UserCard"
  variant TEXT,                             -- e.g., "selected", "disabled"
  viewport TEXT NOT NULL,                   -- "desktop" | "tablet" | "mobile"
  url TEXT NOT NULL,                        -- Page URL

  -- Capture context
  timestamp DATETIME NOT NULL,              -- ISO 8601
  capture_context TEXT,                     -- JSON metadata (5KB max)

  -- File metadata
  size_bytes INTEGER NOT NULL,
  format TEXT DEFAULT 'png',                -- png | webp

  -- Cleanup tracking
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  deleted_at DATETIME                       -- NULL = active, set for soft deletes
);

-- Composite indexes for common queries
CREATE INDEX idx_component_variant_timestamp
  ON screenshots(component, variant, timestamp DESC);

CREATE INDEX idx_url_timestamp
  ON screenshots(url, timestamp DESC);

CREATE INDEX idx_viewport ON screenshots(viewport);
CREATE INDEX idx_timestamp ON screenshots(timestamp DESC);
CREATE INDEX idx_filepath ON screenshots(filepath);
```

### Storage Configuration Table: `screenshot_config`

```sql
CREATE TABLE screenshot_config (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Insert defaults:
-- ("max_disk_bytes", "5368709120")     -- 5GB
-- ("max_age_days", NULL)               -- unlimited age
-- ("last_cleanup", "2026-01-31T02:00:00Z")
```

---

## File Storage Layout

```
/data/kaboom/screenshots/
├── 2026/01/30/
│   ├── component-UserCard/
│   │   ├── variant-selected/
│   │   │   ├── 10-15-23-desktop-a1b2c3d4.png
│   │   │   ├── 10-15-23-tablet-a1b2c3d4.png
│   │   │   ├── 10-15-23-mobile-a1b2c3d4.png
│   │   └── variant-disabled/
│   │       ├── 10-30-45-desktop-a1b2c3d4.png
│   │       └── ...
│   └── component-Dialog/
│       └── ...
├── 2026/01/29/
│   └── ...
└── orphaned/           -- For cleanup recovery
    └── ...
```

### Path components:
- `YYYY/MM/DD/` — Partition by capture date (aids cleanup)
- `component-{name}/` — Component name (sanitized, no /\)
- `variant-{name}/` — Variant name (sanitized, no /\)
- `{HH}-{MM}-{SS}-{viewport}-{uuid-short}.png` — Timestamp + viewport + short UUID

---

## Core Components

### ScreenshotStore Interface

```go
type ScreenshotStore interface {
  Store(screenshot *Screenshot) error
  Query(params *QueryParams) ([]*Screenshot, error)
  Delete(id string) error
  GetDiskUsage() (int64, error)
}

type Screenshot struct {
  ID              string
  Filepath        string
  Component       string
  Variant         string
  Viewport        string
  URL             string
  Timestamp       time.Time
  CaptureContext  string       // JSON (5KB max)
  SizeBytes       int64
  Format          string       // png, webp
}

type QueryParams struct {
  Component string
  Variant   string
  Viewport  string
  URL       string
  DateFrom  time.Time
  DateTo    time.Time
  Limit     int
  Offset    int
}
```

### Cleanup Job

```go
func (s *ScreenshotStore) RunCleanup(ctx context.Context) error {
  // 1. Age-based cleanup (if max_age_days configured)
  if maxAge := getConfigInt("max_age_days"); maxAge > 0 {
    cutoff := time.Now().AddDate(0, 0, -maxAge)
    s.DeleteBefore(ctx, cutoff)
  }

  // 2. Space-based cleanup (if exceeding max_disk_bytes)
  usage, _ := s.GetDiskUsage()
  maxBytes := getConfigInt("max_disk_bytes")
  if usage > maxBytes {
    // Delete oldest screenshots until under limit
    s.DeleteOldestUntil(ctx, maxBytes)
  }

  // 3. Orphan cleanup (remove files without DB entries)
  s.CleanupOrphans(ctx)
}
```

---

## API Changes

### New MCP Tool Mode

```
observe({
  what: "screenshots",
  component?: string,
  variant?: string,
  viewport?: "desktop" | "tablet" | "mobile",
  url?: string,
  date_from?: string (ISO 8601),
  date_to?: string (ISO 8601),
  limit?: number
})
```

### New configure() Command

```
configure({
  action: "screenshot_config",
  config_action: "set",
  key: "max_disk_bytes",
  value: "5368709120"
})
```

---

## Performance Requirements

- **Capture latency:** < 1 second per viewport (3 parallel)
- **Query latency:** < 500ms for typical dataset (10K screenshots)
- **Index efficiency:** < 100ms for component + timestamp lookup (10K rows)
- **Cleanup overhead:** < 5 seconds per GB scanned

---

## Security & Validation

- **Path traversal:** Sanitize component/variant names (remove `/\..`)
- **Metadata size:** Cap `capture_context` at 5KB
- **Disk quota:** Hard limit to prevent DoS
- **File permissions:** 0600 (read/write owner only)

---

## Testing Strategy

See [qa-plan.md](qa-plan.md) for detailed test scenarios.

Key areas:
- Capture parallelization
- Database index correctness
- Cleanup accuracy (no orphans)
- Query performance under load
- Storage limit enforcement

---

## Phase 1 Deliverables

### Week 2 (Server):
- [ ] SQLite schema with indexes
- [ ] ScreenshotStore implementation
- [ ] HTTP handlers (POST /screenshots)
- [ ] Cleanup job

### Week 2-3 (Extension):
- [ ] Parallel viewport capture
- [ ] Batch upload
- [ ] Integration with observe()

### Week 3 (Testing):
- [ ] Unit tests (storage, cleanup)
- [ ] Integration tests (capture + upload)
- [ ] Load tests (10K+ screenshots)
- [ ] Regression tests (other MCP tools)

---

## Related Documents

- **product-spec.md** — Feature requirements
- **feature-tracking.md** — Phase breakdown and deliverables
- **qa-plan.md** — Test scenarios
