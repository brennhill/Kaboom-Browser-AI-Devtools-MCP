---
status: proposed
scope: feature/timeline-search
ai-priority: high
tags: [v7, testing, debugging]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-timeline-search
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Timeline & Search — QA Plan

## Test Scenarios

### Scenario 1: View Timeline for Correlation ID
**Objective:** Verify all events for correlation_id appear on timeline

#### Setup:
- Multiple events from different layers with same correlation_id: "test-payment-001"
  - User action (frontend)
  - Network request (network)
  - Backend logs (backend)
  - Error response (network)
  - Error message displayed (frontend)

#### Steps:
1. Call `observe({what: "timeline", correlation_id: "test-payment-001"})`
2. Verify all 5 events returned
3. Verify events in chronological order
4. Verify timestamps have microsecond precision
5. Verify each event has correct layer and type

#### Expected Result:
- All events present
- Chronological order
- No missing events
- Each event tagged correctly

#### Acceptance Criteria:
- [ ] All events retrieved
- [ ] Timestamps ordered
- [ ] Event types correct
- [ ] No duplicates
- [ ] Query <200ms

---

### Scenario 2: Timeline with Causality Chains
**Objective:** Verify causality chains are built correctly

#### Setup:
- Events representing clear causality:
  1. User clicks button (user_action)
  2. Frontend sends POST request (network_request)
  3. Backend receives and processes (backend_log INFO)
  4. Backend times out (backend_log ERROR)
  5. Error response returned (network_request response)
  6. Frontend shows error message (user_action frontend response)

#### Steps:
1. Call `observe({what: "timeline", correlation_id: "test-payment-001", include_causality: true})`
2. Verify causality_chains in response
3. Verify chain shows: "User clicked → Request sent → Backend timeout → Error displayed"
4. Verify each step linked correctly
5. Verify no broken chains

#### Expected Result:
- Causality chains built
- Chain reflects actual causality
- All steps present
- Chain is actionable/readable

#### Acceptance Criteria:
- [ ] Causality chains generated
- [ ] Chains accurate
- [ ] All steps present
- [ ] Readable by humans

---

### Scenario 3: Search by Severity
**Objective:** Verify filtering by severity works

#### Setup:
- Timeline with mixed severities:
  - 5 info events
  - 3 warning events
  - 2 error events

#### Steps:
1. Call `observe({what: "timeline_search", query: "severity:error"})`
2. Verify only 2 error events returned
3. Call with `severity:warning`, verify 3 warnings
4. Call with `severity:info`, verify 5 info events
5. Call with all severities: verify all 10 events

#### Expected Result:
- Severity filter works
- Only matching events returned
- No false positives/negatives

#### Acceptance Criteria:
- [ ] Filter works correctly
- [ ] No cross-contamination
- [ ] Query fast (<100ms)

---

### Scenario 4: Search by Duration
**Objective:** Verify filtering by duration works

#### Setup:
- Events with durations:
  - 3 fast requests (<100ms)
  - 5 medium requests (100-1000ms)
  - 2 slow requests (>5000ms)

#### Steps:
1. Call `observe({what: "timeline_search", query: "duration_ms:>5000"})`
2. Verify only 2 slow requests returned
3. Call with `duration_ms:<100`, verify 3 fast requests
4. Call with `duration_ms:[100,1000]`, verify 5 medium

#### Expected Result:
- Duration filtering works
- Correct events returned
- No false matches

#### Acceptance Criteria:
- [ ] Duration filter accurate
- [ ] Comparisons work (>, <, =, range)
- [ ] No rounding errors

---

### Scenario 5: Search by Layer
**Objective:** Verify filtering by layer works

#### Setup:
- Timeline with events from all layers:
  - 13 frontend events
  - 8 network events
  - 28 backend events
  - 2 code modification events
  - 1 infrastructure change event

#### Steps:
1. Call `observe({what: "timeline_search", query: "layer:frontend"})`
2. Verify 13 frontend events
3. Call with `layer:backend`, verify 28 events
4. Call with `layer:network`, verify 8 events
5. Call with multiple: `layer:backend layer:network`

#### Expected Result:
- Each layer isolated correctly
- Combined queries work
- No cross-contamination

#### Acceptance Criteria:
- [ ] Layer filtering works
- [ ] All layers supported
- [ ] Combinations work

---

### Scenario 6: Time Range Query
**Objective:** Verify time range filtering works

#### Setup:
- Events spanning 1 hour: 10:00-11:00
- 50 events distributed throughout
- Want to filter to specific 10-minute window

#### Steps:
1. Call `observe({what: "timeline", time_range: ["2026-01-31T10:15:00Z", "2026-01-31T10:25:00Z"]})`
2. Verify only events in 10:15-10:25 window returned
3. Verify events outside window not included
4. Verify timestamps all within range (±1s tolerance)

#### Expected Result:
- Time range respected
- Boundary events correct
- No missing/extra events

#### Acceptance Criteria:
- [ ] Time filtering accurate
- [ ] Boundaries respected
- [ ] No off-by-one errors

---

### Scenario 7: Search by User ID
**Objective:** Verify filtering by user_id works

#### Setup:
- Events for multiple users:
  - User 987: 23 events
  - User 654: 15 events
  - User 321: 9 events

#### Steps:
1. Call `observe({what: "timeline_search", query: "user_id:987"})`
2. Verify 23 events for user 987
3. Verify no events from users 654 or 321
4. Call with user 654, verify 15 events
5. Call with user 321, verify 9 events

#### Expected Result:
- User filtering works
- No cross-user contamination

#### Acceptance Criteria:
- [ ] User filtering accurate
- [ ] No data leakage
- [ ] Query fast

---

### Scenario 8: Trace Event Causality
**Objective:** Verify related_events links work

#### Setup:
- Event with related_events: [evt2, evt3, evt4]

#### Steps:
1. Query event: returns related_events array
2. For each related event, query individually
3. Verify each related event exists and is accessible
4. Verify reverse link exists (evt2 links back to evt1)

#### Expected Result:
- Related events accessible
- Bidirectional links work
- No broken references

#### Acceptance Criteria:
- [ ] Links valid
- [ ] Bidirectional
- [ ] No orphaned events

---

### Scenario 9: Export Timeline
**Objective:** Verify timeline can be exported to file

#### Setup:
- Timeline with 47 events for correlation_id

#### Steps:
1. Call `interact({what: "timeline_export", correlation_id: "test-payment-001"})`
2. Verify response: export_id, file_path
3. Verify file exists at path
4. Read file: parse JSON
5. Verify JSON structure: events array, metadata
6. Verify all 47 events in export
7. Verify timestamps preserved
8. Verify file compressed (gzip)

#### Expected Result:
- Export created successfully
- File accessible
- JSON valid
- All data preserved
- Compression applied

#### Acceptance Criteria:
- [ ] Export file created
- [ ] JSON structure valid
- [ ] All events exported
- [ ] Compression applied
- [ ] File size reasonable

---

### Scenario 10: Timeline Statistics
**Objective:** Verify timeline stats are accurate

#### Setup:
- Timeline with 47 events, mixed types/layers/severities

#### Steps:
1. Call `observe({what: "timeline_stats", correlation_id: "test-payment-001"})`
2. Verify total_events: 47
3. Verify events_by_type breakdown:
   - user_action: 5
   - network_request: 8
   - backend_log: 28
   - etc.
4. Verify events_by_layer breakdown
5. Verify error count matches actual errors
6. Verify duration_ms is longest chain duration

#### Expected Result:
- Stats accurate
- All counts match
- Calculations correct

#### Acceptance Criteria:
- [ ] Event counts accurate
- [ ] Type distribution correct
- [ ] Layer distribution correct
- [ ] Error count correct

---

### Scenario 11: Combined Filters
**Objective:** Verify multiple filters work together

#### Setup:
- Complex timeline: many events, multiple types/layers/severities

#### Steps:
1. Query: `severity:error layer:backend`
2. Verify only backend errors returned
3. Query: `severity:error duration_ms:>1000 layer:network`
4. Verify network errors that took >1s
5. Query: `event_type:network_request user_id:987`
6. Verify network requests for user 987 only

#### Expected Result:
- Filters combine with AND logic
- Only matching events returned
- No false positives

#### Acceptance Criteria:
- [ ] Multiple filters work
- [ ] AND logic correct
- [ ] No unexpected results

---

### Scenario 12: Performance: Large Timeline
**Objective:** Verify performance on large timelines (1000+ events)

#### Setup:
- Simulate 1000 events in correlation ID

#### Steps:
1. Call `observe({what: "timeline", correlation_id: "large-test"})`
2. Record query time
3. Verify completes <200ms
4. Verify all 1000 events returned (or paginated)
5. Call search: `severity:error`, record time
6. Verify search <100ms

#### Expected Result:
- Large timelines query quickly
- No timeouts
- Results complete

#### Acceptance Criteria:
- [ ] Timeline <200ms
- [ ] Search <100ms
- [ ] Results complete
- [ ] No pagination required for typical windows

---

### Scenario 13: Microsecond Precision
**Objective:** Verify timestamps have microsecond precision

#### Setup:
- Events with timestamps at microsecond resolution

#### Steps:
1. Create 5 events with microsecond differences
2. Query timeline for correlation_id
3. Verify timestamps include microseconds
4. Verify order preserved at microsecond level
5. Verify no rounding/truncation

#### Expected Result:
- Microsecond timestamps preserved
- Order correct to microsecond
- No precision loss

#### Acceptance Criteria:
- [ ] Microseconds present in timestamps
- [ ] Order accurate
- [ ] No truncation

---

### Scenario 14: Event Deduplication
**Objective:** Verify duplicate events removed

#### Setup:
- Same event logged twice (duplicate event_id)

#### Steps:
1. Manually inject duplicate events
2. Query timeline
3. Verify only one copy of duplicate event appears
4. Verify deduplication happens transparently

#### Expected Result:
- Duplicates removed
- No confusion from duplicate events

#### Acceptance Criteria:
- [ ] Duplicates detected
- [ ] Only one copy kept
- [ ] Transparent to user

---

### Scenario 15: Cross-Layer Correlation
**Objective:** Verify events from all layers are correlated

#### Setup:
- Complex flow:
  - User clicks button (frontend)
  - Browser sends request (network)
  - Backend receives, processes (backend)
  - Database query happens (backend)
  - Response sent (network)
  - Frontend updates UI (frontend)
  - Code gets logged (code layer)

#### Steps:
1. Trace through all layers for single user action
2. Verify all 7 events have same correlation_id
3. Verify causality chain includes all layers
4. Verify timestamps consistent across layers
5. Verify no layer missed

#### Expected Result:
- Full stack visible
- All layers correlated
- Causality clear

#### Acceptance Criteria:
- [ ] All layers present
- [ ] Correlation complete
- [ ] Causality accurate
- [ ] Timestamps consistent

---

## Acceptance Criteria (Overall)
- [ ] All scenarios pass on Linux, macOS, Windows
- [ ] Timeline query: <200ms for 1-hour windows
- [ ] Search query: <100ms typical
- [ ] Events in microsecond-precise order
- [ ] All layers correlated correctly
- [ ] Causality chains accurate
- [ ] Export/import preserves all data
- [ ] No event loss
- [ ] Deduplication works
- [ ] Performance scales to 10K events

## Test Data

### Sample Event Stream (1 hour, 1250 events)
```
[10:00:00] User 987 signs in (5 events)
[10:05:00] User browses products (50 events)
[10:15:00] User adds item to cart (15 events)
[10:20:00] User attempts checkout (30 events)
[10:20:15] Payment timeout occurs (40 events)
[10:20:30] Error displayed to user (10 events)
[10:21:00] Dev modifies code (3 events)
[10:21:30] Service restarts (8 events)
[10:22:00] User retries checkout (25 events)
[10:22:30] Checkout succeeds (20 events)
... more throughout hour
```

## Regression Tests

**Critical:** After each change, verify:
1. No event loss on queries
2. Timestamp ordering never violated
3. Causality chains never broken
4. Duplicates never appear in results
5. Filters never leak between queries
6. Performance never degrades (track baseline)
7. Microsecond precision maintained
8. Export/import round-trip works
9. Correlation IDs preserved
10. Cross-layer visibility maintained

## Performance Baseline

| Operation | Target | Measured | Status |
|-----------|--------|----------|--------|
| timeline query (1K events) | <200ms | _ | _ |
| search query (1K events) | <100ms | _ | _ |
| export creation | <500ms | _ | _ |
| event indexing (per event) | <5ms | _ | _ |
| causality analysis (1K events) | <50ms | _ | _ |

## Known Limitations

- [ ] No temporal filtering (complex time expressions)
- [ ] No ML-based anomaly detection
- [ ] No cross-session correlation
- [ ] No predictive root cause
- [ ] Simple query language (no complex expressions)
- [ ] No visualization UI (text results only)
- [ ] Export limited to 10K events (compression)
