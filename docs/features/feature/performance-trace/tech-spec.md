---
feature: performance-trace
status: shipped
doc_type: tech-spec
feature_id: feature-performance-trace
last_reviewed: 2026-08-04
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# Technical Spec: Performance Trace

## Architecture

The analyze dispatcher queues `performance_trace` through the normal generation-aware command
path. The background controller owns one trace lifecycle and one Chrome debugger attachment. It
uses `Tracing.start` in `ReportEvents` mode with the same timeline and V8 CPU profiler categories
used by Chrome's Performance tooling. `Tracing.dataCollected` events are serialized into batches
below the local request bound and uploaded in sequence.

The daemon's performance-trace manager opens a mode-0600 partial file beneath Kaboom's state
directory. It validates the trace identifier, sequence number, and every event before an
append-only write. `Tracing.tracingComplete` means Chrome has delivered all pending batches; the
extension then waits for its ordered upload chain, asks the daemon to append the closing JSON,
syncs and closes the file, and atomically renames it from `.partial` to `.json`.

The MCP result contains metadata and the local path, never the raw event array. Four
extension-only HTTP endpoints own the artifact lifecycle:

- `/performance-trace/start`
- `/performance-trace/chunk`
- `/performance-trace/finish`
- `/performance-trace/abort`

## Failure behavior

- A second start is rejected without disturbing the active capture.
- A stop for another tab is rejected because mixing targets would make the artifact dishonest.
- Chrome detach, reported data loss, upload rejection, serialization error, or completion timeout
  fails the command and requests partial-artifact cleanup.
- Chunk writes are ordered and each batch is validated before any event in it is written.
- Daemon state-path resolution failures are logged and use process-local temporary storage so
  capture remains available with a visible diagnostic trail.

## Privacy and performance

The extension talks only to its configured local Kaboom daemon, under extension-only HTTP guards.
Artifacts never enter Cloudflare telemetry. Event processing is append-only and incremental;
memory is bounded to Chrome's current event delivery plus one queued upload batch rather than the
complete trace.

Trace completion has a 15-second bounded wait. Capturing itself may last as long as the caller
chooses between explicit start and stop actions.
