---
doc_type: test_plan
feature_id: feature-log-aggregation
status: proposed
issue: "#89"
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# LLM-Native Log Aggregation --- Test Plan

**Status:** [x] Product Tests Defined | [x] Tech Tests Designed | [ ] Tests Generated | [ ] All Tests Passing

---

## Product Tests

### Valid State Tests

- **Test:** Basic aggregation of repetitive logs
  - **Given:** Log buffer contains 50 entries: 40 identical heartbeat messages, 5 identical polling messages, 5 unique messages
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** Response has 2 groups (heartbeat count=40, polling count=5) and 5 anomalies. `compression_ratio` is > 0.9.

- **Test:** All entries are unique (no repetition)
  - **Given:** Log buffer contains 20 entries, each with a completely unique message
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** Response has 0 groups and 20 anomalies. `compression_ratio` is 0.

- **Test:** All entries are identical
  - **Given:** Log buffer contains 30 copies of the same message
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** Response has 1 group with count=30 and 0 anomalies. `compression_ratio` approaches 1.

- **Test:** Variable content is collapsed into same fingerprint
  - **Given:** Log buffer contains 10 messages like "User 123 logged in", "User 456 logged in", "User 789 logged in" (different IDs each time)
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** All 10 entries form a single group with fingerprint containing `user_{n}_logged_in`. `sample_message` is one raw message.

- **Test:** Anomalies preserve full entry detail
  - **Given:** Log buffer contains 20 repetitive entries and 1 unique warning
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** The unique warning appears in `anomalies[]` with `level`, `message`, `source`, `url`, `line`, `column`, `timestamp`, and `tab_id` fields.

- **Test:** Level breakdown tracks per-group levels
  - **Given:** A group of 10 messages where 8 are level "log" and 2 are level "warn"
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** The group's `level_breakdown` is `{"log": 8, "warn": 2}`.

- **Test:** Periodicity detection for regular intervals
  - **Given:** 10 heartbeat messages arriving every 3 seconds (timestamps: T, T+3, T+6, ..., T+27)
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** The heartbeat group has `is_periodic: true` and `period_seconds` close to 3.0.

- **Test:** Non-periodic irregular entries
  - **Given:** 5 messages with the same fingerprint at irregular intervals (1s, 15s, 2s, 30s)
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** The group has `is_periodic: false` and no `period_seconds` field.

- **Test:** Scope filtering (current_page)
  - **Given:** Log buffer has entries from tab 100 and tab 200. Tab 100 is tracked.
  - **When:** Agent calls `observe(what="summarized_logs", scope="current_page")`
  - **Then:** Only entries from tab 100 are aggregated.

- **Test:** Scope filtering (all)
  - **Given:** Log buffer has entries from tab 100 and tab 200
  - **When:** Agent calls `observe(what="summarized_logs", scope="all")`
  - **Then:** Entries from both tabs are aggregated together.

- **Test:** Level filter (min_level)
  - **Given:** Log buffer has 10 debug, 10 info, 10 warn, 10 error entries
  - **When:** Agent calls `observe(what="summarized_logs", min_level="warn")`
  - **Then:** Only warn and error entries (20 total) are considered for aggregation.

- **Test:** Exact level filter
  - **Given:** Log buffer has entries at multiple levels
  - **When:** Agent calls `observe(what="summarized_logs", level="error")`
  - **Then:** Only error-level entries are considered.

- **Test:** Source filter
  - **Given:** Log buffer has entries from "app.js" and "vendor.js"
  - **When:** Agent calls `observe(what="summarized_logs", source="app.js")`
  - **Then:** Only entries from "app.js" are aggregated.

- **Test:** URL filter
  - **Given:** Log buffer has entries with URLs containing "example.com" and "other.com"
  - **When:** Agent calls `observe(what="summarized_logs", url="example.com")`
  - **Then:** Only entries with "example.com" in the URL are aggregated.

- **Test:** Limit parameter caps input entries
  - **Given:** Log buffer has 500 entries
  - **When:** Agent calls `observe(what="summarized_logs", limit=50)`
  - **Then:** Only the 50 most recent entries are aggregated. `summary.total_entries` is 50.

- **Test:** Custom min_group_size threshold
  - **Given:** Log buffer has 3 messages appearing 2 times each, and 5 messages appearing once
  - **When:** Agent calls `observe(what="summarized_logs", min_group_size=3)`
  - **Then:** All 3 two-count messages appear in `anomalies[]` (below threshold), and `groups[]` is empty.

- **Test:** Groups sorted by count descending
  - **Given:** Log buffer has group A (count=5), group B (count=20), group C (count=10)
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** Groups are ordered: B (20), C (10), A (5).

- **Test:** Anomalies sorted by timestamp descending with level promotion
  - **Given:** 3 anomalies: warn at T+1, log at T+3, error at T+2
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** Anomalies are ordered with error/warn first (error T+2, warn T+1), then log (T+3).

- **Test:** Multiple sources tracked per group
  - **Given:** 10 messages with the same fingerprint from "ws-client.js" (8 entries) and "ws-fallback.js" (2 entries)
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** Group has `source: "ws-client.js"` (most common) and `sources: ["ws-client.js", "ws-fallback.js"]`.

### Edge Case Tests (Negative)

- **Test:** Empty log buffer
  - **Given:** No log entries in the buffer
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** Returns `{groups: [], anomalies: [], summary: {total_entries: 0, groups: 0, anomalies: 0, compression_ratio: 0}}`.

- **Test:** Invalid scope parameter
  - **Given:** Agent provides an invalid scope
  - **When:** Agent calls `observe(what="summarized_logs", scope="invalid")`
  - **Then:** Returns structured error with param="scope" and hint about valid values.

- **Test:** Very long messages truncated for fingerprinting
  - **Given:** Two log messages that are identical in the first 1000 chars but differ after
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** Both entries are fingerprinted on the first 1000 chars and grouped together. `sample_message` is truncated.

- **Test:** Messages with only variable content
  - **Given:** Messages like "12345", "67890", "11111" (pure numbers)
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** All normalize to `{n}` and form a single group.

- **Test:** Entries with missing timestamps
  - **Given:** Log entries where some have no `ts` field
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** Entries without timestamps are still grouped by fingerprint. They sort last in anomalies.

- **Test:** Entries with missing messages
  - **Given:** A log entry with an empty or missing `message` field
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** Empty-message entries form their own group (fingerprint is empty string). They appear in anomalies if unique.

- **Test:** min_group_size=1 (every entry becomes a group)
  - **Given:** 10 entries, each unique
  - **When:** Agent calls `observe(what="summarized_logs", min_group_size=1)`
  - **Then:** 10 groups, 0 anomalies. All entries are grouped (each as count=1).

- **Test:** Limit exceeds buffer size
  - **Given:** Log buffer has 30 entries
  - **When:** Agent calls `observe(what="summarized_logs", limit=500)`
  - **Then:** All 30 entries are aggregated. `summary.total_entries` is 30.

- **Test:** Limit clamped to max (1000)
  - **Given:** Agent requests limit=5000
  - **When:** Agent calls `observe(what="summarized_logs", limit=5000)`
  - **Then:** Limit is clamped to 1000.

### Noise Rule Interaction Tests

- **Test:** Entries matching noise rules are excluded
  - **Given:** Log buffer has 10 entries matching a noise rule and 5 entries not matching
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** Only the 5 non-matching entries are considered. `summary.noise_suppressed` is 10.

- **Test:** noise_rule_match field populated on groups
  - **Given:** A group of 5 entries where the fingerprint overlaps with noise rule "builtin_hmr"
  - **When:** Noise rule does NOT match these specific entries (different pattern), but the group message is similar
  - **Then:** `noise_rule_match` is omitted (only set when entries actually match a rule).

- **Test:** Partial noise rule overlap
  - **Given:** 10 messages share a fingerprint. 7 match noise rule "user_123". 3 do not.
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** The 7 matching entries are suppressed. The 3 non-matching entries form a group with count=3.

- **Test:** Lifecycle/tracking/extension entries excluded
  - **Given:** Log buffer contains entries with type "lifecycle", "tracking", and "extension"
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** These entries are excluded (same as `observe(what="logs")` behavior).

### Concurrent/Race Condition Tests

- **Test:** Concurrent summarized_logs calls
  - **Given:** Log buffer is being written to while two agents call `summarized_logs` simultaneously
  - **When:** Both calls execute concurrently
  - **Then:** Both return valid responses. No panics, no data races. Results may differ slightly due to buffer changes between calls.

- **Test:** Buffer eviction during aggregation
  - **Given:** Ring buffer is full. New entries are evicting old ones during the aggregation scan
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** Returns a consistent snapshot (read lock prevents eviction during scan). No partial or corrupted entries.

### Cursor / Pagination Tests

- **Test:** since_cursor returns only new entries aggregated
  - **Given:** Agent calls `summarized_logs`, gets a cursor. 20 new entries arrive (15 repetitive, 5 unique).
  - **When:** Agent calls `observe(what="summarized_logs", since_cursor="<cursor>")`
  - **Then:** Only the 20 new entries are aggregated. Groups and anomalies reflect only the new entries.

- **Test:** Expired cursor with restart_on_eviction=false
  - **Given:** Cursor points to an evicted buffer position
  - **When:** Agent calls `observe(what="summarized_logs", since_cursor="<expired>")` without restart_on_eviction
  - **Then:** Returns structured error about expired cursor.

- **Test:** Expired cursor with restart_on_eviction=true
  - **Given:** Cursor points to an evicted buffer position
  - **When:** Agent calls `observe(what="summarized_logs", since_cursor="<expired>", restart_on_eviction=true)`
  - **Then:** Falls back to scanning from the start of the buffer. Response includes eviction metadata.

### Failure & Recovery Tests

- **Test:** Malformed JSON arguments
  - **Given:** Agent sends invalid JSON in the arguments
  - **When:** Server parses the request
  - **Then:** Returns structured error with recovery action.

- **Test:** Extension disconnected
  - **Given:** Extension is not connected. Buffer may have stale data.
  - **When:** Agent calls `observe(what="summarized_logs")`
  - **Then:** Response includes disconnect warning prepended to content (same as other observe modes). Aggregation still runs on stale data.

---

## Technical Tests

### Unit Tests

#### Coverage Areas:

**Fingerprinting:**
- `normalizeMessage("")` returns empty string
- `normalizeMessage("simple text")` returns unchanged
- `normalizeMessage("User 12345 logged in")` replaces numeric ID with `{n}`
- `normalizeMessage("Request abc12345-def6-7890-abcd-ef1234567890 failed")` replaces UUID with `{uuid}`
- `normalizeMessage("Fetched https://api.example.com/users")` replaces URL with `{url}`
- `normalizeMessage("Error at 2026-02-20T10:00:00Z")` replaces timestamp with `{timestamp}`
- `normalizeMessage("Hash: deadbeef1234")` replaces hex hash with `{hash}`
- `normalizeMessage("Path /var/log/app.js")` replaces file path with `{path}`
- `normalizeMessage("Long \"this is a very long quoted string here\"")` replaces long quoted string with `{string}`
- `normalizeMessage` with ANSI color codes strips them before processing
- Multiple replacements in a single message apply correctly
- `slugifyFingerprint("User {n} logged in")` returns `user_n_logged_in`
- Fingerprint truncation at 64 characters

**Grouping:**
- Single entry becomes anomaly (below min_group_size=2)
- Two identical entries form a group
- Two entries differing only in variable content form a group
- Two entries with completely different messages become two anomalies
- Group count increments correctly for 100 identical entries
- `first_seen` is earliest timestamp, `last_seen` is most recent
- `level_breakdown` accurately counts per-level entries
- `source` is set to the most common source
- `sources` includes all unique sources when more than one
- `sample_message` is the most recent raw message

**Periodicity:**
- 5 entries at exact 3-second intervals: `is_periodic=true`, `period_seconds=3.0`
- 5 entries at 3-second intervals with 10% jitter: `is_periodic=true`
- 5 entries at random intervals: `is_periodic=false`
- 2 entries: too few for periodicity detection, `is_periodic=false`
- 1 entry: not applicable (anomaly)

**Sorting:**
- Groups sorted by count descending
- Anomalies: error/warn sorted before log/info/debug, then by timestamp descending within priority band

**Summary calculation:**
- `compression_ratio = 1 - (groups + anomalies) / total_entries` computed correctly
- `compression_ratio = 0` when all entries are unique
- `compression_ratio` near 1 when all entries are identical
- `time_range.start` is earliest entry, `time_range.end` is latest entry

**Test File:** `internal/tools/observe/logs/summarized_logs_test.go`

### Integration Tests

#### Scenarios:
- End-to-end: extension sends log entries via sync protocol, agent calls `summarized_logs`, receives grouped response
- Filter chain: noise rules + level filter + source filter + scope filter all applied before aggregation
- Concurrent writes + reads: log entries arriving during aggregation produce consistent results
- Pagination: `since_cursor` incremental aggregation returns only new entries

**Test File:** `cmd/browser-agent/tools_observe_summarized_test.go`

### UAT/Acceptance Tests

**Framework:** Bash script + curl

#### Scenarios:
- Start kaboom, connect extension, navigate to a page with periodic console output
- Call `observe(what="summarized_logs")` via MCP JSON-RPC
- Verify response has `groups`, `anomalies`, `summary` keys
- Verify `summary.total_entries` > 0
- Verify `summary.compression_ratio` >= 0 and <= 1
- Verify each group has required fields: `fingerprint`, `sample_message`, `count`, `level_breakdown`, `first_seen`, `last_seen`, `is_periodic`, `source`
- Verify each anomaly has required fields: `level`, `message`, `source`, `timestamp`
- Call with `min_level=error` and verify only errors are aggregated
- Call with `limit=5` and verify `summary.total_entries` <= 5

**Test File:** Addition to `scripts/test-all-tools-comprehensive.sh`

### Manual Testing

#### Steps:
1. Open browser with extension enabled
2. Navigate to a page that produces console output (e.g., a React app with HMR, or a page with WebSocket connections)
3. Wait 30 seconds to accumulate log entries
4. Call `observe(what="summarized_logs")` via Claude or another MCP client
5. Verify that repetitive messages (heartbeats, polling, HMR updates) are grouped
6. Verify that unique warnings or errors appear in `anomalies[]`
7. Verify `summary.compression_ratio` shows meaningful compression
8. Compare with `observe(what="logs", limit=100)` to confirm the same entries are present in aggregated form
9. Call with `since_cursor` from the first response's metadata to verify incremental aggregation

---

## Test Status

### Links to generated test files (update as tests are created):

| Test Type | File | Status | Notes |
|-----------|------|--------|-------|
| Unit | `internal/tools/observe/logs/summarized_logs_test.go` | Pending | Awaiting implementation |
| Integration | `cmd/browser-agent/tools_observe_summarized_test.go` | Pending | Awaiting implementation |
| UAT | `scripts/test-all-tools-comprehensive.sh` | Pending | Addition to existing script |
| Manual | N/A | Pending | Awaiting implementation |

**Overall:** All product test scenarios must pass before feature is considered complete.
