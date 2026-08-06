---
status: proposed
scope: feature/compressed-diffs/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-compressed-diffs
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-compressed-diffs.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Compressed Diffs Review](./index.md).

# Technical Spec: Compressed State Diffs

## Purpose

The `get_changes_since` tool gives an AI agent a token-efficient way to check what happened in the browser since it last looked. Instead of reading the full console log, network history, and WebSocket buffer (which can be thousands of entries and tens of thousands of tokens), the agent gets a compressed diff — just the new errors, failures, and disconnections that matter.

This is the foundation for the agent feedback loop: edit code, wait a moment, call `get_changes_since`, see if anything broke.

---

## How It Works

The server maintains **checkpoints** — snapshots of where the agent was in each buffer the last time it checked. A checkpoint records the current index position in each ring buffer (console logs, network bodies, WebSocket events, user actions) along with a timestamp.

When the agent calls `get_changes_since`, the server:

1. Resolves which checkpoint to compare against (the last auto-checkpoint by default, or a named/timestamped one if specified).
2. Reads all entries in each buffer that came after the checkpoint position.
3. Compresses them into a diff: deduplicating repeated messages, highlighting only failures and new endpoints, and summarizing rather than dumping raw data.
4. Advances the auto-checkpoint to "now" so the next call only sees truly new events.
5. Returns a small JSON response with a severity level ("clean", "warning", "error") and a one-line summary.

The result is typically under 2KB — roughly 500 tokens — compared to 30KB+ for raw buffer reads.

---

## Data Model

### Checkpoint

A checkpoint stores:
- An optional name (for named checkpoints like "before_refactor")
- A timestamp of when it was created
- An index position into each of the four buffers: logs, network bodies, WebSocket events, and enhanced actions
- The URL the page was on at checkpoint time
- A map of known endpoints and their last-seen status codes (for detecting new failures vs. already-failing endpoints)

The server holds up to 20 named checkpoints plus one auto-checkpoint (the most recent call). Named checkpoints are created explicitly by the agent and do not auto-advance.

### Diff Response

The diff response contains four optional sections (one per buffer category):

- **Console diff**: Lists new errors and new warnings, deduplicated by fingerprint. Each entry has the message text (truncated to 200 chars), the source file/line, and a repeat count. Also reports the total number of new entries.
- **Network diff**: Lists new failures (endpoints that now return 4xx/5xx but previously returned success), newly-seen endpoints (never observed before), and degraded endpoints (latency more than 3x baseline). Also reports total new request count.
- **WebSocket diff**: Lists new disconnections, new connections, and error messages. Reports total new event count.
- **Actions diff**: Lists user actions (clicks, navigation, typing) that occurred since the checkpoint. Reports total count.

The response also includes:
- The checkpoint timestamps (from/to) and duration in milliseconds
- An overall severity: "error" if any new console errors or network failures, "warning" if only warnings or disconnections, "clean" if nothing notable
- A one-line human-readable summary like "2 new console error(s), 1 network failure(s)"
- An estimated token count (JSON byte length / 4)

---

## Tool Interface

**Tool name**: `get_changes_since`

**Parameters** (all optional):
- `checkpoint`: A named checkpoint, an ISO 8601 timestamp, or omit for auto (last call position)
- `include`: Array of categories to include: "console", "network", "websocket", "actions". Defaults to all.
- `severity`: Minimum severity filter: "all" (default), "warnings", or "errors_only"

**Returns**: The diff response object described above.

---

## Behavior

### Auto-checkpoint advancement

Every time `get_changes_since` is called without specifying a named checkpoint, the auto-checkpoint advances to the current buffer positions. This means the next call will only see events that occurred after this one. Named checkpoint queries do NOT advance the auto-checkpoint.

### First call (no prior checkpoint)

If no auto-checkpoint exists yet (first call since server start), the server creates an initial checkpoint at the beginning of all buffers and returns everything currently in the buffers. After this call, the auto-checkpoint is set to the current position.

### Message fingerprinting

Console messages are deduplicated by fingerprint. The fingerprinting process normalizes dynamic content: UUIDs become `{uuid}`, numbers of 4+ digits become `{n}`, and ISO timestamps become `{ts}`. This means "Error loading user abc123-..." and "Error loading user def456-..." collapse into a single entry with count=2.

### Endpoint normalization

Network URLs are reduced to their path (no query parameters) for the purpose of identifying "the same endpoint." This means `/api/users?page=1` and `/api/users?page=2` are treated as the same endpoint when tracking status code changes.

### Severity hierarchy

The overall severity is determined by scanning all diff categories. Console errors or network failures → "error". WebSocket disconnections or console warnings → "warning". Nothing notable → "clean".

---

## Edge Cases

- **Buffer overflow**: If the checkpoint references an index that no longer exists in the ring buffer (entries were evicted), the server falls back to returning all currently-available entries. No panic, no error — just best-effort.
- **Concurrent access**: Checkpoint reads/writes use their own mutex separate from the buffer mutex. Diff computation acquires a read lock on the buffer. Lock ordering is always checkpoint lock first, buffer lock second.
- **Empty buffers**: Returns severity "clean" with all diff sections empty and summary "No significant changes."
- **Category filtering**: If `include` is specified, omitted categories are nil in the response (not empty objects), saving tokens.
- **Max entries per category**: Each diff section is capped at 50 entries to prevent blowup on noisy pages.

---

## Performance Constraints

- Resolving a checkpoint: under 1ms (map lookup or time parse)
- Console diff for 1000 entries: under 10ms
- Network diff for 100 entries: under 5ms
- WebSocket diff for 500 entries: under 5ms
- Actions diff for 50 entries: under 2ms
- Total tool response time: under 25ms
- Typical response size: under 2KB
- Memory for checkpoints: under 100KB (20 checkpoints at ~5KB each)

---

## Integration Points

- **Noise Filtering**: The console diff skips entries that match active noise rules (browser extensions, HMR, analytics). Only real application signals show up in the diff.
- **Behavioral Baselines**: A checkpoint can reference a baseline name, comparing against the baseline's expected state rather than the last call.
- **get_session_health**: The health check tool is essentially a simplified wrapper around `get_changes_since` with preset options.
- **Persistent Memory**: Named checkpoints can persist across sessions so the agent can compare against known-good states.

---

## Test Scenarios

1. Empty server → severity "clean", all diffs empty, summary "No significant changes"
2. Three new error logs after checkpoint → console diff has 3 entries, severity "error"
3. Same error message 5 times → deduplicated to 1 entry with count=5
4. Endpoint was returning 200, now returns 500 → network failures entry with previous_status=200
5. New endpoint never seen before → appears in new_endpoints list
6. WebSocket close event after checkpoint → appears in disconnections
7. Auto-checkpoint advances: first call sees errors, second call sees nothing new
8. Named checkpoint: always returns same window regardless of other calls
9. Severity filtering: errors_only excludes warnings
10. Include filtering: ["console", "network"] → websocket and actions are nil
11. Buffer wrapped past checkpoint index → best-effort from available entries
12. Timestamp as checkpoint reference → entries after that time
13. Token count approximation is close to JSON length / 4
14. 100 errors after checkpoint → capped at 50 in response
15. Concurrent reads and writes → no race conditions (verified with -race flag)
16. UUID/number normalization in fingerprinting works correctly
17. URL path extraction strips query parameters
18. Severity hierarchy: error > warning > clean
19. Summary formatting: "2 new console error(s), 1 network failure(s)"

---

## Checkpoint Naming Conventions

Agents should follow consistent naming so humans (and future sessions) can understand the checkpoint history at a glance.

**Required checkpoints** (agents should always create these):
- `session_start` — created at the beginning of every coding session, before any edits. This is the "known state" anchor for post-session auditing.
- `pre_commit` — created just before the agent commits code. Allows comparing pre-commit health against post-deploy health.

**Recommended checkpoints** (for multi-step work):
- `before_{action}` — before a risky operation (e.g., `before_refactor`, `before_migration`, `before_dependency_upgrade`)
- `after_{action}` — after completing a phase, to mark it as a known-good point
- `baseline_{feature}` — when a feature is confirmed working, for future regression comparison

### Naming rules:
- Lowercase, underscores for spaces
- Max 50 characters
- Descriptive enough that a human reading the checkpoint list can reconstruct the session timeline
- No timestamps in names (the checkpoint already stores its creation time)

### Example session timeline:
```
session_start          → clean
before_auth_refactor   → clean
after_auth_refactor    → 1 warning (new deprecation notice, acceptable)
before_commit          → clean
```

A human reviewing this can immediately see: the agent started clean, did an auth refactor that introduced a minor warning, then resolved it before committing.

---

## Future: Human-Friendly Diff Viewer

The compressed diff format is optimized for AI token efficiency, but humans overseeing agent sessions need a way to audit what happened too. A human-readable diff viewer should present the same checkpoint-based data in a format designed for quick human scanning — not JSON payloads.

Use cases:
- **Post-session review**: "The agent said it fixed the bug — did the browser actually stop erroring?"
- **Regression auditing**: "Before the agent's PR, was the app healthy? After?"
- **Trust building**: Gives humans confidence that the agent's self-reported status matches reality

Implementation is TBD. Could be a CLI output mode, a browser extension panel, a web dashboard served by the Go binary, or an MCP resource that renders markdown. The key requirement is that the same checkpoint/diff data powering the agent's feedback loop is also inspectable by a human without parsing JSON.

---

## File Location

Implementation goes in `cmd/browser-agent/ai_checkpoint.go` with tests in `cmd/browser-agent/ai_checkpoint_test.go`.
