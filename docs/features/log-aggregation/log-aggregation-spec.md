---
doc_type: product_spec
feature_id: feature-log-aggregation
status: proposed
issue: "#89"
tool: observe
mode: summarized_logs
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# LLM-Native Log Aggregation --- Product Spec

## Problem

A typical browser session produces hundreds of console log entries. The majority are repetitive noise: WebSocket heartbeat acknowledgments, polling status checks, framework lifecycle messages, analytics callbacks, and timer-driven telemetry pings. When an LLM agent calls `observe(what="logs")`, it receives every individual entry. This has three compounding costs:

1. **Token waste.** A session with 200 log entries where 150 are heartbeat variants burns ~3,000 tokens on messages the agent will never act on. Over a multi-turn debugging session this compounds to tens of thousands of wasted tokens.

2. **Signal burial.** The 5 entries that actually matter (a warning about a failed API retry, a deprecation notice, an unexpected state transition) are buried in pages of identical noise. The agent must scan the entire list to find them, and may miss them entirely.

3. **Redundant tool calls.** To manage the volume, agents paginate through logs across multiple calls, adding latency and further token cost for each round-trip.

Existing mitigations are insufficient:

- `configure(what="noise_rule")` suppresses entries entirely. This is useful for entries that are never relevant (extension noise, favicon 404s), but destructive for entries that matter in aggregate. "200 heartbeats in 30 seconds" is useful context; "0 heartbeats" is misleading.
- `observe(what="logs", level="error")` filters by level, but repetitive noise spans all levels.
- `observe(what="error_bundles")` solves a different problem (assembling context around errors, not summarizing volume).

## Solution

Add `observe(what="summarized_logs")` --- a new observe mode that returns **aggregated log groups** instead of individual entries. The server performs single-pass grouping of log entries by normalized message fingerprint, then returns one entry per group with count, time range, and level breakdown. Anomalies (entries that do not belong to any high-frequency group) are surfaced prominently.

This is the log equivalent of what `error_clusters` does for errors: collapse volume into structure.

## User Experience

### Before (raw logs)

```
observe({what: "logs", limit: 100})
```

Returns 100 entries. 82 are WebSocket heartbeats. 11 are polling status messages. 3 are React re-render traces. 4 are actual signal: a retry warning, a deprecation notice, a state assertion, and an unexpected null.

The agent must read all 100 to find the 4 that matter. Cost: ~2,000 tokens.

### After (summarized logs)

```
observe({what: "summarized_logs"})
```

Returns:

```json
{
  "groups": [
    {
      "fingerprint": "ws_heartbeat_ack",
      "sample_message": "WebSocket heartbeat acknowledged: connection_id=abc123",
      "count": 82,
      "level_breakdown": {"log": 82},
      "first_seen": "2026-02-20T10:00:01Z",
      "last_seen": "2026-02-20T10:04:58Z",
      "is_periodic": true,
      "period_seconds": 3.6,
      "source": "ws-client.js"
    },
    {
      "fingerprint": "poll_status_ok",
      "sample_message": "Poll status: {\"ready\":true,\"queue\":0}",
      "count": 11,
      "level_breakdown": {"info": 11},
      "first_seen": "2026-02-20T10:00:05Z",
      "last_seen": "2026-02-20T10:04:55Z",
      "is_periodic": true,
      "period_seconds": 27.2,
      "source": "poller.js"
    },
    {
      "fingerprint": "react_rerender",
      "sample_message": "Re-rendering Dashboard component (props changed)",
      "count": 3,
      "level_breakdown": {"debug": 3},
      "first_seen": "2026-02-20T10:01:12Z",
      "last_seen": "2026-02-20T10:03:44Z",
      "is_periodic": false,
      "source": "react-dom.js"
    }
  ],
  "anomalies": [
    {
      "level": "warn",
      "message": "API retry attempt 3/3 for /users/profile: timeout after 5000ms",
      "source": "api-client.js",
      "timestamp": "2026-02-20T10:02:33Z"
    },
    {
      "level": "warn",
      "message": "navigator.geolocation is deprecated in insecure contexts",
      "source": "location.js",
      "timestamp": "2026-02-20T10:01:05Z"
    },
    {
      "level": "log",
      "message": "State assertion failed: expected user.role to be 'admin', got 'viewer'",
      "source": "auth-guard.js",
      "timestamp": "2026-02-20T10:03:01Z"
    },
    {
      "level": "warn",
      "message": "Unexpected null in response.data.preferences, using defaults",
      "source": "settings.js",
      "timestamp": "2026-02-20T10:03:55Z"
    }
  ],
  "summary": {
    "total_entries": 100,
    "groups": 3,
    "anomalies": 4,
    "compression_ratio": 0.93,
    "time_range": {
      "start": "2026-02-20T10:00:01Z",
      "end": "2026-02-20T10:04:58Z"
    }
  },
  "metadata": { }
}
```

The agent reads 3 groups + 4 anomalies = 7 items instead of 100. Cost: ~300 tokens. The 4 signal entries are immediately visible in `anomalies`. The 3 groups give aggregate context without listing every instance.

---

## API Design

### Invocation

```json
{"tool": "observe", "arguments": {"what": "summarized_logs"}}
```

### Parameters

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `what` | string | (required) | `"summarized_logs"` |
| `limit` | int | 100 | Max number of **source entries** to consider for aggregation (from the tail of the buffer). Max 1000. |
| `min_level` | string | `""` | Minimum log level to include: debug, log, info, warn, error. Empty means all levels. |
| `level` | string | `""` | Exact log level filter. Only entries at this level are considered. |
| `source` | string | `""` | Only aggregate entries from this source. |
| `url` | string | `""` | Only aggregate entries matching this URL substring. |
| `scope` | string | `"current_page"` | `"current_page"` (tracked tab only) or `"all"`. |
| `min_group_size` | int | 2 | Minimum occurrences for an entry to form a group. Entries below this threshold appear in `anomalies`. |
| `since_cursor` | string | `""` | Only aggregate entries newer than this cursor. |
| `after_cursor` | string | `""` | Backward pagination cursor. |
| `restart_on_eviction` | bool | false | Auto-restart if cursor expired. |

### Response Format

```json
{
  "groups": [
    {
      "fingerprint": "string",
      "sample_message": "string",
      "count": 82,
      "level_breakdown": {"log": 80, "info": 2},
      "first_seen": "2026-02-20T10:00:01Z",
      "last_seen": "2026-02-20T10:04:58Z",
      "is_periodic": true,
      "period_seconds": 3.6,
      "source": "ws-client.js",
      "sources": ["ws-client.js", "ws-fallback.js"],
      "noise_rule_match": "builtin_ws_heartbeat"
    }
  ],
  "anomalies": [
    {
      "level": "warn",
      "message": "Actual error or interesting event",
      "source": "app.js",
      "url": "https://example.com",
      "line": 42,
      "column": 15,
      "timestamp": "2026-02-20T10:02:33Z",
      "tab_id": 12345
    }
  ],
  "summary": {
    "total_entries": 100,
    "groups": 3,
    "anomalies": 4,
    "noise_suppressed": 12,
    "compression_ratio": 0.93,
    "time_range": {
      "start": "2026-02-20T10:00:01Z",
      "end": "2026-02-20T10:04:58Z"
    }
  },
  "metadata": {
    "extension_connected": true,
    "tracked_tab": 12345,
    "scope": "current_page",
    "pagination": {}
  }
}
```

### Response Fields

**groups[]**:

| Field | Type | Description |
|-------|------|-------------|
| `fingerprint` | string | Stable identifier for the group, derived from the normalized message template. Human-readable slug format (e.g., `ws_heartbeat_ack`, `poll_status_ok`). |
| `sample_message` | string | One representative raw message from the group (the most recent instance). |
| `count` | int | Number of entries in this group. |
| `level_breakdown` | object | Map of log level to count within this group (e.g., `{"log": 80, "warn": 2}`). |
| `first_seen` | string | RFC3339 timestamp of the earliest entry in the group. |
| `last_seen` | string | RFC3339 timestamp of the most recent entry in the group. |
| `is_periodic` | bool | Whether the entries arrive at regular intervals (standard deviation of inter-arrival times < 20% of mean, with 3+ observations). |
| `period_seconds` | float | Mean interval between entries in seconds. Only present when `is_periodic` is true. |
| `source` | string | Primary source file for the group (the most common source across entries). |
| `sources` | []string | All unique source files that produced entries in this group. Only present when more than one source exists. |
| `noise_rule_match` | string | If entries in this group match an existing noise rule, the rule ID is included. Omitted when no rule matches. This helps the agent understand that the group is already known noise. |

**anomalies[]**:

Anomalies use the same schema as `observe(what="logs")` individual entries. These are entries that appeared fewer than `min_group_size` times (default: fewer than 2 times, i.e., unique entries). They represent the signal --- the entries that are most likely to be actionable.

| Field | Type | Description |
|-------|------|-------------|
| `level` | string | Log level (debug, log, info, warn, error). |
| `message` | string | Raw message text. |
| `source` | string | Source file. |
| `url` | string | Page URL. |
| `line` | int | Line number. |
| `column` | int | Column number. |
| `timestamp` | string | RFC3339 timestamp. |
| `tab_id` | int | Browser tab ID. |

**summary**:

| Field | Type | Description |
|-------|------|-------------|
| `total_entries` | int | Total number of log entries considered (before grouping). |
| `groups` | int | Number of groups formed. |
| `anomalies` | int | Number of anomaly entries (ungrouped). |
| `noise_suppressed` | int | Number of entries suppressed by existing noise rules (not included in groups or anomalies). |
| `compression_ratio` | float | `1 - (groups + anomalies) / total_entries`. Higher means more repetition was collapsed. 0.93 means the response is 93% smaller than raw. |
| `time_range.start` | string | Earliest timestamp across all considered entries. |
| `time_range.end` | string | Latest timestamp across all considered entries. |

---

## Aggregation Algorithm

### Overview

The algorithm runs in a single pass over the log buffer (newest to oldest), building groups incrementally. It is designed for the Go backend with zero allocations on the hot path beyond the group map itself.

### Step 1: Message Fingerprinting

Each log message is normalized into a fingerprint by replacing variable content with placeholders. This is the same normalization strategy used by error clustering, extended for general log messages.

**Normalization rules** (applied in order):

1. Strip ANSI color codes
2. Replace UUIDs (`[0-9a-f]{8}-[0-9a-f]{4}-...`) with `{uuid}`
3. Replace hex hashes (8+ hex chars) with `{hash}`
4. Replace numeric sequences (3+ digits) with `{n}`
5. Replace quoted strings longer than 20 chars with `{string}`
6. Replace URL-like patterns (`https?://...`) with `{url}`
7. Replace ISO 8601 timestamps with `{timestamp}`
8. Replace file paths (`/foo/bar/baz.js`) with `{path}`
9. Collapse whitespace

**Example:**

```
"WebSocket heartbeat acknowledged: connection_id=abc12345-def6-7890-abcd-ef1234567890 at 2026-02-20T10:00:01Z"
  -> "WebSocket heartbeat acknowledged: connection_id={uuid} at {timestamp}"
  -> fingerprint: "ws_heartbeat_acknowledged_connection_id"
```

The fingerprint is then slugified: lowercased, non-alphanumeric replaced with `_`, consecutive underscores collapsed, truncated to 64 chars.

### Step 2: Grouping

Iterate the buffer tail-to-head (newest first), up to `limit` entries:

1. Apply existing filters (scope, level, min_level, source, url, noise rules)
2. Entries matching a noise rule are counted in `noise_suppressed` and skipped
3. Compute the fingerprint for the entry's message
4. If a group with this fingerprint exists: increment count, update `first_seen`, append level to breakdown, add source to sources set
5. If no group exists and the entry is the first with this fingerprint: create a tentative entry (held in a "pending singles" map)
6. If a second entry with the same fingerprint arrives: promote the pending single to a full group

After the scan completes:

- Groups with `count >= min_group_size` go into `groups[]`
- Entries that remained in the pending singles map (count < min_group_size) go into `anomalies[]`

### Step 3: Periodicity Detection

For each group with 3+ entries:

1. Compute inter-arrival times between consecutive entries (sorted by timestamp)
2. Compute mean and standard deviation of intervals
3. If `stddev / mean < 0.20` (less than 20% jitter), mark `is_periodic = true` and set `period_seconds = mean`

This reuses the same periodicity heuristic from `configure(what="noise_rule", noise_action="auto_detect")` but applies it per-group rather than per-URL.

### Step 4: Sorting

- `groups[]` sorted by `count` descending (highest-frequency groups first)
- `anomalies[]` sorted by timestamp descending (newest first), with error/warn levels promoted above log/info/debug

### Complexity

- Time: O(n) where n = number of entries scanned. Single pass with O(1) map lookups per entry.
- Space: O(g + a) where g = number of unique fingerprints (groups) and a = number of anomalies. In practice, g is small (10-50 unique fingerprints in a typical session).

---

## Interaction with Existing Features

### Noise Rules (`configure(what="noise_rule")`)

Noise rules and log aggregation are **complementary, not competing**:

- **Noise rules suppress entries entirely** --- they disappear from all observe responses. Use for entries that are never useful (extension warnings, favicon 404s).
- **Log aggregation groups entries** --- they still appear, but collapsed. Use when the aggregate count/pattern matters but individual instances do not.

In `summarized_logs`, entries matching existing noise rules are excluded from both groups and anomalies. Their count is reported in `summary.noise_suppressed`. If a group of entries all match a noise rule, the group does not appear.

If only some entries in a would-be group match a noise rule, the non-matching entries form the group normally. This can happen when a noise rule has a narrow regex that catches most but not all variants.

The `noise_rule_match` field on groups tells the agent "this group also matches noise rule X". The agent can then decide whether to suppress the group via noise rules in future calls, or keep it visible in summarized form.

### Error Bundles (`observe(what="error_bundles")`)

Error bundles assemble context **around** individual errors. Summarized logs aggregate **across** the log stream. They solve different problems and do not overlap:

- Use `error_bundles` when investigating a specific error ("what happened around this crash?")
- Use `summarized_logs` when surveying the session ("what's going on? what should I investigate?")

### Error Clustering (`analyze(what="error_clusters")`)

Error clustering groups related **errors** by stack frame and message similarity. Summarized logs groups **all log levels** by message fingerprint. There is some conceptual overlap for error-level entries, but:

- Error clusters use stack trace analysis --- summarized logs do not (console logs rarely have stacks)
- Error clusters infer root causes --- summarized logs just count
- Summarized logs include debug/info/warn levels that error clusters ignore

### Pagination

`summarized_logs` supports `since_cursor` and `after_cursor` for incremental use. The cursor points into the underlying log buffer (same cursor system as `observe(what="logs")`). The aggregation runs over the slice of the buffer selected by the cursor range.

This allows the agent to call `summarized_logs` once to get the baseline, then call it again with `since_cursor` to get only new entries aggregated. New entries that match existing group fingerprints will form new groups in the incremental response --- the server does not maintain cross-call group state.

---

## Anomaly Detection

The primary anomaly signal is **uniqueness**: entries that appear only once (or fewer than `min_group_size` times) in the observation window are anomalies. This is a deliberately simple heuristic:

- It requires no ML, no training data, no statistical models
- It naturally surfaces errors, warnings, and unusual events because those tend to be unique
- It automatically filters out repetitive noise because noise repeats

Additional anomaly signals (applied as annotations, not filtering):

1. **Level escalation**: A group that historically contained only `log`-level entries suddenly includes a `warn` or `error` entry. The anomaly entry is pulled out of the group and placed in `anomalies[]`.
2. **Frequency spike**: A group's rate in the last 10 seconds is 3x higher than its overall rate. Annotated on the group with `"frequency_spike": true`.
3. **New fingerprint**: A fingerprint that has never been seen in previous calls to `summarized_logs` in this session. Annotated on the group or anomaly with `"new": true` (requires session-scoped fingerprint tracking in the daemon).

These secondary signals are optional enhancements for v2. The initial implementation relies solely on uniqueness (the `min_group_size` threshold).

---

## Privacy Considerations

- All aggregation happens locally in the Go daemon. No data leaves the machine.
- Fingerprints are derived from message content but strip identifying information (UUIDs, IDs, URLs) as part of normalization.
- The `sample_message` field contains one raw message per group. If the agent needs to avoid passing sensitive message content to a cloud LLM, it should use the `fingerprint` and `count` fields instead.
- No persistent storage. Groups are computed per-call and discarded. The daemon holds no aggregation state between calls (stateless aggregation).

---

## Performance Constraints

| Metric | Target | Rationale |
|--------|--------|-----------|
| Aggregation of 1000 entries | < 5ms | Must feel instant. Single-pass O(n). |
| Fingerprinting per message | < 0.05ms | 7 regex replacements, pre-compiled. |
| Memory for 100 groups | < 50KB | Each group is ~500 bytes (fingerprint + sample + counters + timestamps). |
| Response size vs raw logs | > 80% reduction | Typical sessions have 10-20 unique fingerprints from 100+ entries. |

All regex patterns used for fingerprinting are pre-compiled at daemon startup, not per-call. The fingerprinting function is a pure function with no allocations beyond the output string.

---

## Edge Cases

| Edge Case | Resolution |
|-----------|------------|
| Empty log buffer | Return `{groups: [], anomalies: [], summary: {total_entries: 0, groups: 0, anomalies: 0}}` |
| All entries are unique (no repetition) | All entries appear in `anomalies[]`, `groups[]` is empty, `compression_ratio` is 0 |
| All entries are identical | Single group with count = total, `anomalies[]` is empty, `compression_ratio` approaches 1 |
| Mixed structured/unstructured messages | Fingerprinting operates on the string representation. JSON messages are fingerprinted as strings. |
| Very long messages (>1000 chars) | Truncate to 1000 chars before fingerprinting. `sample_message` is also truncated. |
| High cardinality (every message slightly different) | Fingerprinting strips variable content, so `"User 123 logged in"` and `"User 456 logged in"` share the fingerprint `"User {n} logged in"`. If normalization cannot collapse messages, they become anomalies. |
| Entries from multiple tabs (scope="all") | Groups span tabs. The `sources` field may contain entries from different tab contexts. |
| Cursor-based incremental call | Aggregation runs on the buffer slice selected by the cursor. Groups are computed fresh each call. A group may have count=1 in an incremental call if only one new entry arrived. |
| min_group_size=1 | Every entry forms a group, `anomalies[]` is empty. This is valid but defeats the purpose. |
| Entries with missing timestamps | Entries without timestamps are assigned the current time for grouping purposes. They sort last in anomalies. |
| Entries already suppressed by noise rules | Counted in `noise_suppressed`, excluded from groups and anomalies. |
| Rapid log spam (1000+ entries in 1 second) | The `limit` parameter caps processing. Default 100. Agent can increase to 1000. Grouping handles this in O(n). |

---

## Success Criteria

1. An LLM agent calling `observe(what="summarized_logs")` on a session with 100 log entries (80% repetitive) receives fewer than 15 items total (groups + anomalies).
2. Actual errors and warnings that appear only once are always present in `anomalies[]`.
3. Response latency is under 10ms for 1000 entries.
4. The feature works without any extension changes (pure Go daemon logic).
5. Existing `observe(what="logs")` is unaffected --- agents can still get raw logs.
6. Noise rules continue to work. Entries suppressed by noise rules do not appear in summarized output.

---

## Implementation Approach

### Files Changed

| File | Change |
|------|--------|
| `internal/tools/observe/summarized_logs.go` | **NEW.** Handler for `summarized_logs` mode. Fingerprinting, grouping, periodicity detection, response assembly. ~250 lines. |
| `internal/tools/observe/summarized_logs_test.go` | **NEW.** Unit tests. ~300 lines. |
| `internal/tools/observe/handlers.go` | Add `"summarized_logs": GetSummarizedLogs` to `Handlers` map. ~1 line. |
| `internal/tools/observe/schema.go` | Add `"summarized_logs"` to the `what` enum. ~1 line. |
| `cmd/browser-agent/tools_observe.go` | Add `"summarized_logs"` to `observeHandlers` map, delegating to `observe.GetSummarizedLogs`. ~3 lines. |
| `cmd/browser-agent/tools_schema.go` | Add `"summarized_logs"` to tool description. ~1 line. |
| `cmd/browser-agent/testdata/mcp-tools-list.golden.json` | Regenerate (includes new enum value). |

### No Extension Changes

This is a pure Go-side feature. The extension already sends console log entries to the daemon via the existing sync protocol. The daemon ring buffer already holds all the data. Summarized logs is a new read path over existing data.

### No Protocol Changes

No new MCP methods, no new wire types, no new HTTP endpoints. This is a new `what` value for the existing `observe` tool.

---

## Effort Estimate

2-3 days. One new Go file (~250 lines) for the handler and fingerprinting logic. One new test file (~300 lines). Five existing files changed by 1-3 lines each.

---

## Future Enhancements (Not in Scope)

- **Cross-call group persistence**: Track fingerprints across calls to annotate new vs. recurring groups.
- **Frequency spike detection**: Alert when a group's rate suddenly increases.
- **Level escalation detection**: Alert when a normally-quiet group starts producing warnings/errors.
- **Auto-promote to noise rule**: Suggest converting high-count periodic groups into permanent noise rules.
- **Structured log parsing**: Parse JSON log messages and group by structured fields rather than string fingerprint.
- **WebSocket message aggregation**: Apply the same grouping to `websocket_events`.

---

## See Also

- [Noise Filtering Tech Spec](../feature/noise-filtering/tech-spec.md)
- [Error Clustering Tech Spec](../feature/error-clustering/tech-spec.md)
- [Error Bundling Tech Spec](../feature/error-bundling/tech-spec.md)
- [Pagination Tech Spec](../feature/pagination/tech-spec.md)
