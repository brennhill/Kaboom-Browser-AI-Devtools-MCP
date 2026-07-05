---
status: shipped
scope: feature/persistent-memory/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-persistent-memory
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-persistent-memory.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Persistent Memory Review](persistent-memory-review.md).

# Technical Spec: Persistent Cross-Session Memory

## Purpose

Without persistence, every time the Kaboom server restarts, the agent starts from zero — no noise rules, no baselines, no knowledge of previous errors. In an AI-driven workflow where agents work in sessions (start, code, stop), this means re-discovering the same noise patterns, re-saving baselines, and losing error history every time.

Persistent memory gives the server a disk-backed store that survives restarts. When a new session starts, the agent can call `load_session_context` and immediately know: what baselines exist, what noise rules were configured, what errors were seen before, and how many sessions have been recorded for this project.

---

## How It Works

### Project-Local Storage

Persistent data lives in `.kaboom/` at the project root (the server's working directory), gitignored. No hashing, no indirection — the project directory IS the identity.

This follows the same pattern as `.vscode/`, `.next/`, `.turbo/`, and other tool-local state directories. The developer can inspect, delete, or back up the data trivially.

The server adds `.kaboom/` to `.gitignore` automatically on first use if not already present.

### Storage Layout

```
<project-root>/.kaboom/
├── meta.json              # Session metadata (count, timestamps)
├── baselines/             # Behavioral baselines
│   ├── login.json
│   └── dashboard.json
├── noise/                 # Noise configuration
│   └── config.json
├── api_schema/            # Inferred API schemas
│   └── schema.json
├── errors/                # Error history
│   └── history.json
└── performance/           # Performance baselines
    └── endpoints.json
```

Each namespace is a subdirectory. Each key is a JSON file within that directory.

### Session Lifecycle

1. **Server starts**: Creates a `SessionStore`, reads `meta.json` (or creates it fresh). Increments session count. Starts a background flush goroutine.
2. **Agent calls `load_session_context`**: Server reads summaries from all namespaces and returns a combined context object. Also applies any loaded configuration (e.g., restoring noise rules so they're immediately active).
3. **During the session**: Other features (baselines, noise, schema) call `Save` to persist their state. Writes happen immediately to disk.
4. **Server shuts down**: The shutdown handler persists any in-memory state that hasn't been saved yet (current noise config, baselines). Stops the background flush goroutine. Writes final meta.json.

### Background Flush

A background goroutine runs every 30 seconds, checking for "dirty" namespaces (ones with buffered changes that haven't been written). This is a safety net for features that buffer data in memory before persisting — mainly error history and performance metrics.

The flush goroutine exits cleanly on server shutdown.

---

## Data Model

### Project Metadata

Stored in `meta.json`:
- Project path (the CWD where the server runs)
- When persistence was first created for this project
- When the last session occurred
- Total session count

### Session Context (load_session_context response)

A combined summary of all persisted data:
- Project path and session count
- **Baselines**: count and list of names
- **API Schema**: number of known endpoints, last update time
- **Noise Config**: total rules, auto-detected count, manual count, lifetime entries filtered
- **Error History**: list of unresolved errors (fingerprint, first/last seen, occurrence count) and recently resolved errors (last 7 days)
- **Performance**: number of endpoints tracked, any degraded since last session

### Error History Entry

Tracks an error across sessions:
- Message fingerprint
- First-seen and last-seen timestamps
- Total occurrence count
- Whether it's been resolved
- When it was resolved (if applicable)

Error history is capped at 500 entries. Entries older than 30 days are evicted.

---

## Tool Interface

### `load_session_context`

**Parameters**: None

**Behavior**: Reads all namespace summaries from disk, applies loaded configuration (noise rules become active), and returns the combined session context. This is the first tool call an agent should make when starting work on a project.

If the store has never been used for this project, returns a fresh context with session_count=1 and no data in any namespace.

### `session_store`

A general-purpose key-value interface for storing/loading arbitrary data.

**Parameters**:
- `action` (required): "save", "load", "list", "delete", or "stats"
- `namespace`: Logical grouping (required for save/load/delete)
- `key`: Storage key (required for save/load/delete)
- `data`: JSON data to persist (for action "save")

**Behavior by action**:
- **save**: Writes data as JSON to `<namespace>/<key>.json`. Errors if data exceeds 1MB.
- **load**: Reads and returns the JSON from `<namespace>/<key>.json`. Errors if not found.
- **list**: Returns all keys in a namespace (file names without .json extension).
- **delete**: Removes the file.
- **stats**: Returns storage statistics: total bytes, entries per namespace, session count.

---

## Server Integration

The `V4Server` struct holds a reference to the `SessionStore`. Initialization creates the store and starts the flush goroutine. The shutdown path:

1. Persists current noise config to `noise/config.json`
2. Persists all in-memory baselines to `baselines/<name>.json`
3. Calls `SessionStore.Shutdown()` which flushes dirty data and saves meta.json

---

## Size Limits

- Max 1MB per individual file (checked on save)
- Max 10MB total for `.kaboom/` directory
- Error history capped at 500 entries
- Stale errors (>30 days old) are automatically evicted

---

## Security and Privacy

- **Sensitive data**: Same sanitization as in-memory data applies. Auth headers are stripped, passwords are redacted before they ever reach the store.
- **File permissions**: Files are created with 0644, directories with 0755 (user-readable by default).
- **Gitignored by default**: The server adds `.kaboom/` to `.gitignore` on first use. Persistent data never enters version control.
- **Concurrent instances**: If multiple Kaboom server instances run for the same project, the first one takes a file lock (`flock()`) on meta.json. The second instance operates in read-only mode for persistence.

---

## Edge Cases

- **Read-only filesystem**: If the storage directory can't be written to, save operations return errors but the server continues operating with in-memory-only data. No crash.
- **Corrupted JSON**: If a stored file can't be parsed, it's silently skipped on load. The feature starts fresh as if no data existed.
- **Missing directories**: The store creates directories on demand (MkdirAll).
- **Concurrent access**: All store operations are mutex-protected. Saves take a write lock, loads take a read lock.
- **Store not initialized**: If `load_session_context` is called before the store is ready, it returns a specific error message rather than crashing.

---

## Performance Constraints

- Creating the store (dirs + read meta): under 100ms
- Loading session context (all namespace summaries): under 200ms
- Single file save: under 50ms
- Single file load: under 20ms
- Shutdown (flush all dirty data): under 500ms
- Periodic flush: under 100ms
- Memory overhead: under 1MB (metadata + file handles)

---

## Test Scenarios

1. Store created at `.kaboom/` in working directory
2. `.kaboom/` added to `.gitignore` if not already present
3. Save then load returns identical data
4. Load nonexistent key → error
5. List returns all keys without .json extension
6. Delete removes file, subsequent load errors
7. Stats returns correct sizes and entry counts
8. File exceeding 1MB → error on save
9. Shutdown then restart → session count increments, last_session updates
10. Fresh store with no prior sessions → session_count=1, no data
11. Store with existing baselines/noise/errors → all summaries populated
12. Loading noise config → rules become active on the server immediately
13. Concurrent reads and writes → no race conditions or file corruption
14. Shutdown flushes all dirty data
15. Short flush interval → data written within interval
16. Read-only directory → errors returned, server continues in-memory
17. Error history at 500 entries → oldest evicted when 501st added
18. Error entries older than 30 days → removed on cleanup

---

## File Location

Implementation goes in `cmd/browser-agent/ai_persistence.go` with tests in `cmd/browser-agent/ai_persistence_test.go`.
