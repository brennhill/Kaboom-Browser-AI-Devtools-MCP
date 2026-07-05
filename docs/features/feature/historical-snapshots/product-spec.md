---
status: proposed
scope: feature/historical-snapshots
ai-priority: medium
tags: [v7, persistence, analysis, eyes]
relates-to: [../normalized-log-schema.md, ../../core/architecture.md]
last-verified: 2026-03-05
doc_type: product-spec
feature_id: feature-historical-snapshots
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Historical Snapshots

## Overview
Historical Snapshots periodically saves complete system state (all events, logs, network activity, variable values) to disk, allowing developers to "rewind time" and examine what the system looked like at any point in the past. When a bug is discovered, developers can replay the exact sequence of events that led to it, including state at each step. Snapshots are indexed by timestamp and tagged with metadata (test run ID, Git commit, feature flag state), enabling rich queries like "Show me all snapshots where feature X was enabled and we had errors."

## Problem
Ephemeral event storage means once a developer closes Kaboom, all observation data is lost. If a bug manifests hours later, there's no way to examine what happened when the bug was introduced. "Let me just run the test again" often doesn't reproduce intermittent issues. Long-running observations (overnight test suites, extended debugging sessions) can't save intermediate results.

## Solution
Historical Snapshots:
1. **Periodic Saving:** Snapshot entire system state every N minutes (configurable, default 1 minute)
2. **Metadata Tagging:** Each snapshot includes timestamp, test run ID, git commit, feature flags
3. **Selective Storage:** Save to disk (SQLite or JSON) with configurable retention (default 7 days)
4. **Replay Capability:** Load snapshot and query it as if Kaboom is running on that point in time
5. **Comparison:** Compare two snapshots to see what changed

## User Stories
- As a developer debugging flaky tests, I want to save snapshots when failures occur so that I can replay the exact sequence of events later
- As a QA engineer running overnight test suites, I want snapshots saved periodically so that I can analyze failures the next morning
- As a DevOps engineer investigating a production incident, I want to compare snapshots from before/after the incident to identify what changed
- As a platform engineer, I want to archive snapshots for compliance auditing

## Acceptance Criteria
- [ ] Snapshot entire system state (events, logs, network, variables) every N minutes
- [ ] Store snapshots to disk (SQLite by default, JSON option)
- [ ] Metadata per snapshot: timestamp, test_run_id, git_commit, feature_flags, environment
- [ ] Query saved snapshots: `observe({what: 'snapshots', since: timestamp, git_commit: 'abc123'})`
- [ ] Load and replay snapshot: all queries work on snapshots as if live
- [ ] Compare two snapshots: show what events were added/removed
- [ ] Configurable retention policy (default 7 days, max 30 days)
- [ ] Performance: save snapshot <1s, load snapshot <500ms, disk usage <1GB per day

## Not In Scope
- Automatic snapshot uploads to cloud
- Encryption at rest (user responsibility)
- Compression (can be added later)
- Remote storage (local disk only)

## Data Structures

### Snapshot Metadata
```json
{
  "snapshot_id": "snap-20260131-101523",
  "timestamp": "2026-01-31T10:15:23.456Z",
  "test_run_id": "test-xyz",
  "git_commit": "a7f8e3d",
  "git_branch": "feature/payment",
  "git_dirty": false,
  "feature_flags": {
    "new_checkout": true,
    "beta_api": false
  },
  "environment": {
    "NODE_ENV": "test",
    "DEBUG": "*"
  },
  "stats": {
    "total_events": 1250,
    "total_logs": 3450,
    "total_requests": 45,
    "error_count": 3,
    "sessions": 2
  }
}
```

### Snapshot Storage Format
```json
{
  "metadata": { ... },
  "events": [
    { type: "network", ... },
    { type: "log", ... },
    ...
  ],
  "indexes": {
    "by_request_id": { "req-123": [0, 1, 5, ...] },
    "by_session_id": { "session-xyz": [0, 2, 3, ...] },
    "by_timestamp": [...]
  }
}
```

## Examples

### Example 1: Save Snapshot on Test Failure
#### Developer runs flaky test:
```
[10:15:23] Test starts
[10:15:30] Test fails (assertion error)
[10:15:31] Kaboom automatically saves snapshot with tag "test-failure-xyz"
```

#### Developer investigates later:
```
observe({what: 'snapshots', tag: 'test-failure-xyz'})
// → Shows snapshot from 10:15:31
// → Can replay, query, analyze like live session
```

### Example 2: Compare Before/After Incident
```
observe({
  what: 'snapshot-diff',
  snapshot_a: 'snap-20260131-100000',  // Before incident
  snapshot_b: 'snap-20260131-101500'   // After incident
})

// Returns:
// - 45 new error events
// - 3 services with increased latency
// - Feature flag "rate_limiter" was toggled at 10:05:30
```

## MCP Changes
```javascript
// List snapshots
observe({
  what: 'snapshots',
  since: timestamp,
  git_commit: 'abc123',
  test_run_id: 'test-xyz',
  limit: 10
})

// Load and query snapshot
observe({
  what: 'snapshot-data',
  snapshot_id: 'snap-20260131-101523',
  query: { what: 'network', level: 'ERROR' }
})

// Compare snapshots
analyze({
  what: 'snapshot-diff',
  snapshot_a_id: 'snap-a',
  snapshot_b_id: 'snap-b'
})
```

## Configuration
```yaml
snapshots:
  enabled: true
  interval_minutes: 1          # Save every minute
  storage: sqlite              # or "json"
  retention_days: 7            # Keep 7 days
  max_disk_usage_gb: 1         # Max 1GB per day
  auto_save_on_failure: true   # Save on test failure
  auto_save_on_error: true     # Save on backend error

  # Per-session limits
  max_snapshot_size_mb: 50
  max_events_per_snapshot: 10000
```

## Implementation Strategy
Phase 1 (MVP): Save to JSON files, query via in-memory load
Phase 2: SQLite backend for better querying
Phase 3: Differential snapshots (only save changes from previous)
Phase 4: Compression and archival policies
