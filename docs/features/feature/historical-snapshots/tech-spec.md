---
status: proposed
scope: feature/historical-snapshots
ai-priority: medium
tags: [v7, persistence]
relates-to: [product-spec.md, ../normalized-log-schema/tech-spec.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-historical-snapshots
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Historical Snapshots — Technical Specification

## Architecture

### Snapshot Pipeline
```
Periodic Timer (every N minutes)
    ↓
Serialize Event Store
    ├─ All events, logs, network
    ├─ All indexes
    ├─ Metadata (git, feature flags)
    ↓
Snapshot Writer (JSON or SQLite)
    ↓
Disk Storage (~/.kaboom/snapshots/)
    ↓
Cleanup/Retention Policy (>7 days)
```

### Components
1. **Snapshot Manager** (`server/snapshots/manager.go`)
   - Periodic timer, starts snapshot process
   - Collects metadata (git commit, feature flags)
   - Triggers serialization

2. **Snapshot Serializer** (`server/snapshots/serializer.go`)
   - Serialize all events to JSON
   - Build indexes for fast querying
   - Calculate snapshot size and stats

3. **Snapshot Writer** (`server/snapshots/writer.go`)
   - Write to JSON files (MVP) or SQLite (Phase 2)
   - Metadata storage
   - Retention policy enforcement

4. **Snapshot Loader** (`server/snapshots/loader.go`)
   - Load snapshot from disk
   - Reconstruct event store
   - Enable querying as if live

5. **Snapshot Differ** (`server/snapshots/differ.go`)
   - Compare two snapshots
   - Identify added/removed events
   - Calculate statistics

## Implementation Plan

### Phase 1: JSON Snapshots (Week 1-2)
1. Implement snapshot manager with periodic timer
2. Implement JSON serializer
3. Implement JSON file writer with metadata
4. Implement loader and querying on loaded snapshots
5. Implement basic retention policy (simple date-based cleanup)

### Phase 2: Metadata & Diff (Week 2-3)
1. Collect Git metadata (commit, branch, dirty status)
2. Collect feature flag state
3. Implement snapshot differ
4. Support querying snapshots by metadata

### Phase 3: SQLite (Week 3-4)
1. Implement SQLite writer as alternative to JSON
2. Migrate JSON snapshots to SQLite on startup
3. Index snapshots for efficient querying
4. Performance optimization

## API Changes

### Snapshot Structure (JSON)
```json
{
  "version": 1,
  "metadata": {
    "id": "snap-20260131-101523",
    "timestamp": "2026-01-31T10:15:23.456Z",
    "git_commit": "a7f8e3d",
    "git_branch": "feature/payment",
    "git_dirty": false,
    "feature_flags": { ... },
    "environment": { ... },
    "stats": { ... }
  },
  "events": [ ... ],
  "indexes": { ... }
}
```

### MCP Query Handlers
```go
// List snapshots
type SnapshotsRequest struct {
    Since      *time.Time
    Until      *time.Time
    GitCommit  string
    GitBranch  string
    TestRunID  string
    Limit      int
    Cursor     string
}

// Query snapshot data
type SnapshotDataRequest struct {
    SnapshotID string
    Query      *ObservableQuery  // Same as observe() query
}

// Diff snapshots
type SnapshotDiffRequest struct {
    SnapshotAID string
    SnapshotBID string
}
```

## Code References
- **Snapshot manager:** `/Users/brenn/dev/kaboom/server/snapshots/manager.go` (new)
- **Serializer:** `/Users/brenn/dev/kaboom/server/snapshots/serializer.go` (new)
- **Writers:** `/Users/brenn/dev/kaboom/server/snapshots/writer.go` (new)
- **Loader:** `/Users/brenn/dev/kaboom/server/snapshots/loader.go` (new)
- **Differ:** `/Users/brenn/dev/kaboom/server/snapshots/differ.go` (new)
- **Config:** Config file for snapshot settings (modified)

## Performance Requirements
- **Snapshot save:** <1s for typical session (10K events)
- **Snapshot load:** <500ms
- **Snapshot size:** <50MB per snapshot (10K events)
- **Disk usage:** <1GB per day (configurable retention)
- **Query on snapshot:** Same as live queries (<100ms)

## Testing Strategy

### Unit Tests
- Serialization accuracy (round-trip test)
- Metadata collection
- Retention policy (date-based cleanup)
- Snapshot differ logic

### Integration Tests
- Save snapshot, load, query
- Verify all events present
- Verify indexes rebuilt correctly
- Performance under load (large snapshots)

### E2E Tests
- Save snapshots during test run
- Query saved snapshots
- Compare snapshots
- Replay snapshot and verify queries work

## Dependencies
- **JSON:** Go stdlib
- **SQLite:** (Phase 2, external library)
- **Git:** For commit/branch info

## Risks & Mitigation
1. **Disk space exhaustion**
   - Mitigation: Configurable retention, size limits, warnings
2. **Slow snapshot save blocking operations**
   - Mitigation: Async save, compression
3. **Memory overhead during snapshot**
   - Mitigation: Stream serialization if needed
4. **Stale data in loaded snapshots**
   - Mitigation: Timestamp displayed prominently in UI

## Storage Layout
```
~/.kaboom/snapshots/
├── snap-20260131-090000.json
├── snap-20260131-100000.json
├── snap-20260131-110000.json
└── snapshots.db (SQLite, Phase 2)
```

## Retention Policy
- Default: Keep 7 days
- Configurable: 1-30 days
- Automatic cleanup: Run at startup and periodically
- Disk quota: Optional hard limit (evict oldest first)

## Backward Compatibility
- Snapshots are opt-in (enabled by default, can disable)
- No impact on live queries
- Snapshot format versioned for future changes
- Can migrate JSON to SQLite without user action
