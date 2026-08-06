---
status: proposed
scope: feature/behavioral-baselines/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-behavioral-baselines
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-behavioral-baselines.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Behavioral Baselines Review](./index.md).

# Technical Spec: Behavioral Baselines

## Purpose

In vibe-coded projects with no test suite, the agent has no way to know what "correct" looks like after making changes. Behavioral baselines solve this by letting the agent snapshot the current browser state when things are working, then compare against that snapshot later to detect regressions.

A baseline captures what the API endpoints are returning, what console errors exist (if any), which WebSocket connections are open, and how fast things are. It's not a pixel-perfect screenshot — it's a behavioral fingerprint that says "when this feature works, the network returns these shapes at these speeds with these status codes."

---

## How It Works

### Saving a Baseline

When the agent calls `save_baseline`, the server reads the current state of all buffers and extracts a behavioral fingerprint:

- **Network**: Groups all observed network bodies by method + normalized path. For each endpoint, records the status code, response JSON shape (top-level field names and types), content type, average latency, and observation count.
- **Console**: Counts current errors and warnings. Records fingerprints of all known error messages (so they can be recognized as "expected" later).
- **WebSocket**: Records which WebSocket URLs are currently connected and whether they're expected to be open.
- **Timing**: Computes P50, P95, and max latency across all network requests.

The baseline is stored in memory (keyed by name) and persisted to disk at `~/.kaboom/baselines/<name>.json`.

### Comparing Against a Baseline

When the agent calls `compare_baseline`, the server reads the current buffer state and compares each dimension against the saved baseline:

- **Network regressions**: An endpoint that used to return 2xx now returns 4xx/5xx.
- **Latency regressions**: An endpoint's current latency exceeds the baseline by more than the timing tolerance factor (default: 3x).
- **Console regressions**: New console errors that weren't in the baseline's known error list.
- **WebSocket regressions**: A connection that was expected to be open is now closed.
- **Improvements**: An endpoint that was failing now succeeds (noted but not flagged as regression).

The comparison result includes a status ("match", "regression", or "improved"), a list of specific regressions with severity and description, and a summary string.

---

## Data Model

### Baseline

A baseline contains:
- Name, description, creation/update timestamps, version number
- URL scope (which page/path this baseline applies to)
- Network baselines: a list of endpoint records, each with method, parameterized path pattern, expected status, response shape (field→type map), content type, average latency, and observation count
- Console baseline: error count, warning count, and a list of fingerprinted known messages
- WebSocket baseline: list of expected connections with URL pattern and open/closed state
- Timing baseline: P50, P95, and max latency values

### Path Normalization

URLs are normalized before storage: UUIDs in paths become `{uuid}`, numeric IDs become `{id}`. This means `/api/users/550e8400.../posts` and `/api/users/7f8a9b0c.../posts` are treated as the same endpoint pattern.

### Response Shape Extraction

The server parses JSON response bodies and records only the top-level field names and their types (string, number, boolean, array, object, null). Values are never stored. This means shape comparison catches structural changes (field removed, type changed) but ignores value differences.

### Tolerance Configuration

Comparisons accept a tolerance config:
- `timing_factor`: How many times slower an endpoint can be before flagging (default: 3.0)
- `allow_additional_network`: Whether new endpoints not in the baseline are OK (default: true)
- `allow_additional_console_info`: Whether new info-level console entries are OK (default: true)
- `ignore_dynamic_values`: Whether to compare shapes rather than exact values (default: true)

---

## Tool Interface

### `save_baseline`

**Parameters**:
- `name` (required): Descriptive name like "login-flow" or "dashboard-load"
- `description`: What this baseline represents
- `url_scope`: URL pattern this baseline applies to
- `overwrite`: Whether to replace an existing baseline with the same name (default: false)

**Behavior**: Captures current state, stores in memory and on disk. Returns the name, version, endpoint count, and URL scope. Errors if the name exists and overwrite is false. Errors if max baselines (50) reached.

### `compare_baseline`

**Parameters**:
- `name` (required): Name of the baseline to compare against
- `tolerance`: Object with timing_factor, allow_additional_network, allow_additional_console_info

**Behavior**: Reads current state, compares against the named baseline, returns the comparison result. Errors if the baseline name doesn't exist.

### `list_baselines`

**Parameters**: None

**Returns**: Array of baseline summaries (name, description, URL scope, version, timestamps, endpoint count, has-websocket flag), total count, and storage bytes used.

### `delete_baseline`

**Parameters**:
- `name` (required): Name of the baseline to delete

**Behavior**: Removes from memory and deletes the disk file.

---

## Persistence

Baselines persist across server restarts. On save, the baseline is written as JSON to `~/.kaboom/baselines/<name>.json`. On server startup, all `.json` files in that directory are loaded into memory.

Size limits:
- Max 50 baselines
- Max 100KB per baseline file
- Max 5MB total baseline storage

If a baseline exceeds the size limit, the save fails with an error and the file is not written.

---

## Edge Cases

- **Endpoint not observed**: If a baseline endpoint isn't seen in the current session, it's not flagged as a regression (the user might not have navigated there yet). Only status code changes on observed endpoints are reported.
- **Known errors**: Errors that were present when the baseline was saved are recognized by fingerprint and not flagged as regressions. Only new, unknown errors trigger a regression.
- **Version tracking**: Each overwrite increments the version number, providing a history of how many times the baseline was updated.
- **Non-JSON responses**: If a response body isn't valid JSON, the shape is nil (no shape comparison performed for that endpoint).
- **Concurrent access**: Baselines have their own RWMutex. Saves take a write lock. Compares take a read lock on baselines and a read lock on the buffer.

---

## Performance Constraints

- Capturing a baseline: under 30ms
- Comparing against a baseline: under 20ms
- Persisting to disk: under 50ms
- Loading all baselines at startup: under 200ms for 50 baselines
- Memory per baseline: under 100KB
- Total baseline memory: under 5MB

---

## Test Scenarios

1. Save with 5 network bodies, 2 errors, 1 WS connection → baseline has all three sections
2. Save when name exists and overwrite=false → error message
3. Save with overwrite=true → version increments
4. Save at max baselines (50) → error message
5. Compare with no changes → status "match"
6. Endpoint was 200, now 500 → regression with category "network", severity "error"
7. Endpoint latency 100ms baseline, now 400ms (>3x) → timing regression
8. Endpoint latency 100ms baseline, now 250ms (<3x) → no regression
9. New console errors not in baseline → console regression
10. Known errors still present → not flagged (recognized by fingerprint)
11. WebSocket was open, now closed → websocket regression
12. Endpoint was failing, now succeeds → noted as improvement
13. Custom tolerance: timing_factor=2.0 makes 250ms a regression (>200)
14. Baseline name doesn't exist → error "not found"
15. List with no baselines → empty array, count 0
16. List with baselines → correct metadata for each
17. Delete removes from memory and disk
18. JSON shape extraction: {"id": 1, "name": "test"} → {"id": "number", "name": "string"}
19. Non-JSON body → shape is nil
20. Path normalization: UUIDs → {uuid}, numbers → {id}
21. Persist writes to disk, load reads back correctly
22. Baseline exceeding 100KB → error, not written
23. Concurrent save and compare → no race conditions

---

## File Location

Implementation goes in `cmd/browser-agent/ai_baselines.go` with tests in `cmd/browser-agent/ai_baselines_test.go`.
