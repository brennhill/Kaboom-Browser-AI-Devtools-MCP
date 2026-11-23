// Purpose: Package server — core server log storage, handler state, and thread-safe log accessor APIs.
// Why: Centralizes server-side log persistence and exposes read-only log snapshots for analysis.
// Docs: docs/features/feature/backend-log-streaming/index.md

/*
Package server implements core server-side log storage and provides thread-safe
accessor APIs for querying captured log data.

Key types:
  - Server: manages file-backed log storage with configurable buffer capacity.
  - LogAccessor: provides thread-safe read-only access to log buffer snapshots.

Key functions:
  - NewServer: creates a server with the specified log file and buffer size.
  - GetLogEntries: returns a snapshot of all log entries.
  - GetLogEntriesSince: returns log entries newer than a given timestamp.
*/
package server
