---
status: proposed
scope: feature/historical-snapshots
ai-priority: medium
tags: [v7, testing]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-historical-snapshots
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Historical Snapshots — QA Plan

## Test Scenarios

### Scenario 1: Periodic Snapshot Creation
#### Setup:
- Kaboom running with snapshot interval = 1 minute
- Events being generated

#### Steps:
1. Let Kaboom run for 5 minutes
2. Check disk: ~/.kaboom/snapshots/
3. Verify snapshots created at each interval

#### Expected Result:
- Snapshots created every 1 minute
- 5 snapshots total (at 0, 1, 2, 3, 4 minutes)
- Each has metadata with correct timestamp

#### Acceptance Criteria:
- [ ] Snapshots created on schedule
- [ ] Correct number created
- [ ] Timestamps accurate
- [ ] Metadata complete

---

### Scenario 2: Snapshot Content Verification
#### Setup:
- Snapshot created with known events

#### Steps:
1. Generate 100 events over 1 minute
2. Snapshot created automatically
3. Load snapshot and query it
4. Verify all 100 events present

#### Expected Result:
- All events in snapshot
- No events lost or duplicated
- Events in correct order
- Indexes rebuilt correctly

#### Acceptance Criteria:
- [ ] All 100 events in snapshot
- [ ] 0% data loss
- [ ] Order preserved
- [ ] Indexes accurate

---

### Scenario 3: Query on Loaded Snapshot
#### Setup:
- Snapshot from 10 minutes ago loaded

#### Steps:
1. Load snapshot with query: `{what: 'network', level: 'ERROR'}`
2. Verify results identical to live query at that time

#### Expected Result:
- Query works on loaded snapshot
- Same results as original query
- No differences in output format
- Performance <500ms

#### Acceptance Criteria:
- [ ] Query works on snapshot
- [ ] Results match original
- [ ] Performance acceptable

---

### Scenario 4: Metadata Tagging
#### Setup:
- Test run with Git commit and feature flags

#### Steps:
1. Start test on commit abc1234
2. Feature flag 'new_ui' enabled
3. Snapshot created during test
4. Query snapshots with git_commit filter

#### Expected Result:
- Snapshot metadata includes:
  - git_commit: "abc1234"
  - git_branch: "feature/x"
  - feature_flags: {new_ui: true}
- Can filter snapshots by these metadata

#### Acceptance Criteria:
- [ ] Metadata collected correctly
- [ ] Snapshot has git_commit
- [ ] Snapshot has feature flags
- [ ] Metadata queryable

---

### Scenario 5: Snapshot on Test Failure
#### Setup:
- Test fails, auto-save-on-failure enabled

#### Steps:
1. Run test, test fails
2. Kaboom automatically saves snapshot with tag
3. Developer queries: `observe({what: 'snapshots', test_run_id: 'xyz'})`

#### Expected Result:
- Snapshot saved immediately on failure
- Tagged with test_run_id
- Queryable
- Contains all events up to failure

#### Acceptance Criteria:
- [ ] Snapshot saved on failure
- [ ] Tagged with test ID
- [ ] Accessible via query
- [ ] Complete event history

---

### Scenario 6: Snapshot Retention Policy
#### Setup:
- Retention set to 2 days
- Snapshots exist from 1, 2, 3 days ago

#### Steps:
1. Run cleanup (or wait for scheduled cleanup)
2. List snapshots
3. Verify old snapshots deleted

#### Expected Result:
- Snapshots >2 days old deleted
- Snapshots <2 days old retained
- Disk space reclaimed

#### Acceptance Criteria:
- [ ] Old snapshots deleted
- [ ] Recent snapshots kept
- [ ] Cleanup runs automatically
- [ ] No manual intervention needed

---

### Scenario 7: Snapshot Differ
#### Setup:
- Two snapshots from different times
- System state changed between them

#### Steps:
1. Load snapshot A (10:00)
2. Load snapshot B (10:05)
3. Call: `analyze({what: 'snapshot-diff', snapshot_a: 'A', snapshot_b: 'B'})`

#### Expected Result:
- Diff shows:
  - New events added between snapshots
  - Events removed (if any)
  - Statistics: event count changes, error count changes

#### Acceptance Criteria:
- [ ] Diff identifies new events
- [ ] Statistics accurate
- [ ] Added/removed counts correct

---

### Scenario 8: Snapshot with Large Event Count
#### Setup:
- 10K events in session over 1 minute
- Snapshot created with all events

#### Steps:
1. Generate 10K events
2. Snapshot created
3. Snapshot file size measured
4. Load and query snapshot

#### Expected Result:
- Snapshot <50MB
- All 10K events preserved
- Load time <500ms
- Query latency <100ms

#### Acceptance Criteria:
- [ ] Snapshot <50MB
- [ ] No data loss
- [ ] Load <500ms
- [ ] Query responsive

---

### Scenario 9: Disk Usage & Cleanup
#### Setup:
- Continuous snapshot creation for 1 week
- Disk usage monitored

#### Steps:
1. Snapshots created every minute for 7 days
2. Monitor disk usage
3. Verify doesn't exceed limits
4. Run cleanup, verify old snapshots removed

#### Expected Result:
- Disk usage stable (not growing unbounded)
- Cleanup runs as scheduled
- Old snapshots removed
- Disk limit respected

#### Acceptance Criteria:
- [ ] Disk growth controlled
- [ ] Cleanup effective
- [ ] Retention policy enforced
- [ ] Memory pressure avoided

---

### Scenario 10: Snapshot Round-Trip
#### Setup:
- Original session with known events
- Snapshot created and loaded

#### Steps:
1. Create session, generate 500 events
2. Create snapshot
3. Delete live session data (simulate restart)
4. Load snapshot
5. Query snapshot for specific event
6. Compare with original

#### Expected Result:
- All events preserved in snapshot
- Query results identical to original
- No data corruption
- Indexes rebuilt correctly

#### Acceptance Criteria:
- [ ] Perfect round-trip (no data loss)
- [ ] Query results identical
- [ ] Indexes functional
- [ ] No corruption

---

## Acceptance Criteria (Overall)
- [ ] All 10 scenarios pass
- [ ] Snapshots created periodically
- [ ] Snapshots contain all events
- [ ] Queries work on snapshots
- [ ] Retention policy enforced
- [ ] Metadata collected and queryable
- [ ] Performance targets met

## Test Data

### Fixture: Sample Snapshot Metadata
```json
{
  "snapshot_id": "snap-20260131-101523",
  "timestamp": "2026-01-31T10:15:23.456Z",
  "test_run_id": "test-xyz",
  "git_commit": "a7f8e3d9c1b2e4f6",
  "git_branch": "feature/payment",
  "git_dirty": false,
  "feature_flags": {
    "new_checkout": true,
    "beta_api": false
  },
  "stats": {
    "total_events": 1250,
    "total_logs": 3450,
    "total_requests": 45,
    "error_count": 3
  }
}
```

## Regression Tests
- [ ] Snapshots don't interfere with live queries
- [ ] Retention policy doesn't delete recent snapshots
- [ ] Snapshot size stable over time
- [ ] Diff algorithm correct
- [ ] Metadata collection reliable
- [ ] Disk cleanup doesn't corrupt remaining snapshots
