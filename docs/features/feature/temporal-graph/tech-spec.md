---
status: proposed
scope: feature/temporal-graph/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-temporal-graph
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-temporal-graph.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Temporal Graph Review](temporal-graph-review.md).

# Technical Spec: Cross-Session Temporal Graph

## Purpose

Persistent memory (v1) stores key-value data: baselines, noise rules, API schemas. It knows *what* exists but not *when things happened* or *why they changed*. The AI can load session context and see "there are 3 baselines and 2 noise rules," but it can't answer questions like:

- "When did this error first appear?"
- "What changed right before the regression started?"
- "Has this error happened before? How was it fixed last time?"
- "Is this a recurring pattern — does it show up every time we deploy?"

The temporal graph extends persistent memory with a time-ordered event log that tracks causality. Events (errors, regressions, fixes, deploys) are recorded with timestamps and optional causal links ("error X appeared after change Y"). The AI can query this graph to understand the history of the project's browser behavior across sessions.

This makes the AI a collaborator with long-term memory — it remembers what happened last week and uses that context to diagnose today's problem faster.

---

## How It Works

### Event Recording

The server records significant events to the temporal graph automatically:

1. **Error events**: When a new error fingerprint is first seen (not every occurrence — just the first). Includes the error message, source location, and timestamp.

2. **Regression events**: When the performance monitor detects a regression (metric exceeds baseline by threshold). Includes the metric, the baseline value, and the regressed value.

3. **Resolution events**: When a previously-seen error stops occurring for 5+ minutes during active browsing. Marked as "possibly resolved."

4. **Baseline shift events**: When a performance baseline is updated (new steady-state detected). Records old and new values.

5. **Deploy events**: When the CI webhook receives a deploy notification, or when the resource list changes significantly (new scripts/stylesheets suggest a deploy).

The AI can also explicitly record events:

```
configure(what: "record_event", event: { type: "fix", description: "Fixed null user in UserProfile", related_to: "evt_error_123" })
```

### Causal Links

Events can reference other events, forming a directed graph:

- A regression event might reference a deploy event ("regression appeared after deploy X")
- A resolution event references the error event it resolves
- An AI-recorded fix event references the error it fixed

Links are optional. Most events stand alone. The graph is sparse — typically 10-20% of events have explicit links.

### Automatic Correlation

The server attempts to correlate events automatically using temporal proximity:

- If a regression is detected within 30 seconds of a resource change (new script loaded), the regression event gets a "possibly_caused_by" link to an implicit deploy event.
- If an error disappears within 60 seconds of a new resource list (page reload with new code), a resolution event is created with a "possibly_fixed_by" link.

These automatic correlations are labeled as "inferred" (vs. "explicit" for AI-recorded links) so the AI knows which links are hypotheses vs. confirmed.

### Query Interface

The temporal graph is queried through the `analyze` composite tool:

```
analyze(target: "history", query: { type: "error", since: "7d" })
```

Parameters for the history query:
- `type` (optional): Filter by event type (error, regression, resolution, baseline_shift, deploy, fix)
- `since` (optional): Time window ("1h", "1d", "7d", "30d"). Default: "7d"
- `related_to` (optional): Find events linked to a specific event ID
- `pattern` (optional): Search event descriptions by substring

Response:
```json
{
  "events": [
    {
      "id": "evt_1",
      "type": "error",
      "timestamp": "2026-01-20T10:00:00Z",
      "description": "TypeError: Cannot read property 'name' of undefined at UserProfile.render",
      "source": "user-profile.js:42",
      "status": "resolved",
      "links": [
        { "target": "evt_3", "relationship": "resolved_by", "confidence": "explicit" }
      ]
    },
    {
      "id": "evt_2",
      "type": "regression",
      "timestamp": "2026-01-21T14:00:00Z",
      "description": "LCP regressed: 1200ms → 2400ms on /dashboard",
      "metric": "lcp_ms",
      "links": [
        { "target": "evt_4", "relationship": "possibly_caused_by", "confidence": "inferred" }
      ]
    }
  ],
  "total_events": 12,
  "time_range": "7d",
  "patterns": [
    { "description": "Recurring: TypeError in UserProfile seen 3 times in 30 days, resolved each time within 1 day" }
  ],
  "summary": "12 events in last 7 days. 2 errors (1 resolved), 1 regression (linked to deploy), 1 fix."
}
```

### Pattern Detection

The server identifies recurring patterns across the event history:

- **Recurring errors**: Same error fingerprint appearing, being resolved, then reappearing. Suggests a flaky fix or an intermittent root cause.
- **Deploy-correlated regressions**: Regressions that consistently appear after deploys. Suggests missing performance tests in CI.
- **Time-of-day patterns**: Errors that appear at specific times (often related to cron jobs, scheduled tasks, or traffic patterns).

Patterns are computed on-demand when the history is queried (not continuously). They're included in the response as a separate `patterns` array.

### Storage

The temporal graph is stored in `.kaboom/history/events.jsonl` — one JSON object per line, append-only. This makes it trivially inspectable and streamable.

Events are appended immediately when they occur. The file is read sequentially on query. For the expected volume (10-50 events per session, ~500 events per month of daily use), sequential scan is fast enough.

### Retention

Events older than 90 days are evicted on server startup. The eviction reads the file, filters out old events, and rewrites it. This keeps the file bounded at a reasonable size (typically under 500KB for 90 days of use).

The retention period is configurable via `.kaboom.json`:
```json
{ "history_retention_days": 90 }
```

---

## Data Model

### Event Entry

Each event in the JSONL file:
- ID (auto-generated, format: `evt_{timestamp}_{random}`)
- Type (error, regression, resolution, baseline_shift, deploy, fix)
- Timestamp (ISO 8601)
- Description (human-readable summary)
- Source (file:line for errors, metric name for regressions)
- Origin ("system" or "agent") — distinguishes automatically-recorded events from AI-recorded ones
- Agent (MCP client name, only for origin="agent" events)
- Status (active, resolved, superseded)
- Metadata (type-specific: metric values, error fingerprints, resource lists)
- Links (array of `{ target: event_id, relationship: string, confidence: "explicit"|"inferred" }`)

System-origin events are generated by the server from real browser data (errors, regressions, resource changes). Agent-origin events are explicitly recorded by the AI via `configure(what: "record_event")`. This distinction lets future AI sessions weight system events more heavily — they're ground truth from the browser, while agent events are the AI's interpretation (which may have been wrong).

### Relationships

- `caused_by` / `possibly_caused_by`: This event was triggered by the target event
- `resolved_by` / `possibly_resolved_by`: This event was fixed by the target event
- `supersedes`: This event replaces the target (e.g., new baseline supersedes old)
- `related_to`: General association without causal direction

### Event Status

- `active`: The condition described by this event is still present
- `resolved`: The condition has been fixed or is no longer occurring
- `superseded`: A newer event replaces this one (e.g., baseline updated again)

---

## Edge Cases

- **No history file**: First query creates it. Empty response with "No history recorded yet."
- **Corrupted JSONL line**: Skip the line, log a warning, continue reading subsequent lines. JSONL is line-independent — one bad line doesn't corrupt the rest.
- **Very long session (1000+ events)**: Events are deduplicated by fingerprint — the same error appearing 100 times produces one event with an occurrence count, not 100 events. Regressions are only recorded when they cross the threshold, not on every measurement.
- **Concurrent server instances**: Same locking strategy as persistent memory — second instance reads only.
- **Clock skew across sessions**: Events are ordered by recorded timestamp. If the system clock changes between sessions, events may appear out of order. The query sorts by timestamp regardless.
- **Orphaned links**: If an event references a target that was evicted (>90 days), the link is preserved but marked "target_evicted" in query responses.
- **AI records invalid event**: Missing required fields → error response with valid fields listed. Invalid `related_to` ID → event recorded without the link, warning in response.
- **Large history file (>10MB)**: Unlikely at normal usage (~500 events/month × ~200 bytes = ~100KB/month). If it occurs, eviction runs immediately rather than waiting for startup.

---

## Performance Constraints

- Event append: under 1ms (JSON marshal + file append, no fsync per write)
- History query (full 90-day scan): under 50ms for 5000 events
- Pattern detection: under 100ms (computed during query, cached for 60s)
- Eviction on startup: under 200ms (read, filter, rewrite)
- Memory for query results: under 500KB (events are streamed, not all held in memory)
- Disk space: under 1MB for 90 days typical usage

---

## Test Scenarios

1. Error event recorded when new fingerprint first seen
2. Repeat error occurrence → count incremented, no new event
3. Regression event recorded when threshold exceeded
4. Resolution event when error absent for 5+ minutes
5. Baseline shift event on baseline update
6. Deploy event on CI webhook or resource change
7. AI-recorded fix event with explicit link
8. Automatic correlation: regression within 30s of resource change
9. Automatic correlation: error disappears after reload
10. Query with type filter returns only matching events
11. Query with since filter returns only recent events
12. Query with related_to returns linked events
13. Pattern detection: recurring error identified
14. 90-day eviction removes old events on startup
15. Corrupted JSONL line skipped gracefully
16. Empty history → empty response, no error
17. Event deduplication by fingerprint
18. Orphaned links marked in query response
19. Concurrent read/write → no corruption
20. `configure(what: "record_event")` with valid event → stored with origin="agent"
21. `configure(what: "record_event")` with invalid event → error
22. System events have origin="system", agent events have origin="agent"
23. Agent name recorded for agent-origin events

---

## File Locations

Server implementation: `cmd/browser-agent/temporal_graph.go` (event recording, query, pattern detection, eviction).

Storage: `.kaboom/history/events.jsonl` (project-local, gitignored).

Tests: `cmd/browser-agent/temporal_graph_test.go`.
