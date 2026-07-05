---
feature: memory-snapshot
status: proposed
doc_type: tech-spec
feature_id: feature-memory-snapshot
last_reviewed: 2026-07-05
last_verified_version: 0.8.4
last_verified_date: 2026-06-29
---

# Tech Spec: Memory Snapshot

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Memory Snapshot adds a new analyze mode, `analyze({what: "memory_snapshot"})`, that captures a
JavaScript heap snapshot through the Chrome DevTools Protocol (CDP) `HeapProfiler` domain and
returns structured analysis. The mode registers in the analyze dispatch registry
(`analyzeHandlers` in `cmd/browser-agent/tools_analyze_dispatch.go`), and its hint plus optional
parameters register in `internal/tools/configure/mode_specs_analyze.go`.

The central architectural decision is that analysis runs in the Go daemon, not in the agent.
The extension captures the raw snapshot and transfers it to the daemon; the daemon parses it
into an in-memory object graph once, caches the graph by `snapshot_id`, and answers every
detail mode by traversing that cached graph. Each mode returns only its conclusion. This keeps
analysis where compute is cheap (Go, milliseconds per pass) rather than where tokens are
expensive (the agent's context window).

## Key Components

**CDP heap capture (extension)**: Using the existing Chrome debugger lifecycle in
`src/background/cdp-dispatch.ts` (`chrome.debugger.attach`, `sendCommand`, `detach`), the
extension enables the `HeapProfiler` domain, calls `HeapProfiler.takeHeapSnapshot`, accumulates
the chunked data emitted by `HeapProfiler.addHeapSnapshotChunk` events, reassembles the complete
JavaScript Object Notation (JSON) snapshot, disables the domain, and detaches.

**Chunked transfer (extension to daemon)**: Heap snapshots range from fifty to five hundred
megabytes. The extension streams the reassembled snapshot to the daemon over the existing
WebSocket channel in chunks to avoid loading the entire payload as one message.

**Snapshot ingestion and cache (Go)**: The daemon parses the `.heapsnapshot` JSON into an
in-memory graph of nodes, edges, and the strings table. It caches the parsed graph keyed by a
generated `snapshot_id`, holding at most two snapshots so comparison workflows
(`leak_suspects`, `growth_report`) can diff a before-and-after pair. The oldest snapshot is
evicted when a third is captured, and the cache clears on tab navigation or session end.

**Detail-mode analyzers (Go)**: Each detail mode is a single pass or a bounded-depth traversal
over the cached graph:
- `summary`: aggregate by constructor, identify detached DOM nodes, compute leak heuristics.
- `dom_leaks`: filter to detached DOM nodes and walk retainer edges backwards to build chains.
- `retainers`: filter by a constructor name and trace top retainer paths via breadth-first search.
- `leak_suspects`: diff two cached snapshots by constructor and surface significant growth.
- `growth_report`: full before-and-after comparison including shrunk objects and net deltas.
- `strings`: aggregate string nodes by value with deduplication, sorted by retained size.
- `closures`: rank closures by disproportionate retained-versus-shallow size ratio.
- `full`: return the complete parsed graph.
- `raw`: write the cached snapshot to a file path or return it as a resource Uniform Resource
  Identifier (URI).

**Asynchronous command plumbing**: Capture is long-running, so it follows the asynchronous
command pattern (`cmd/browser-agent/internal/toolinteract/interact_command_builder.go`) with a
sixty-second timeout and result polling through `observe({what: "command_result"})`. Analysis
queries against an already-cached snapshot are synchronous.

## Data Flows

```
AI calls analyze({what: "memory_snapshot"})
  |
  v
Daemon enqueues an asynchronous capture command (long-running)
  |
  v
Extension: chrome.debugger attach -> HeapProfiler.enable -> takeHeapSnapshot
  -> accumulate addHeapSnapshotChunk events -> reassemble JSON -> disable -> detach
  |
  v
Extension streams the snapshot to the daemon over WebSocket in chunks
  |
  v
Daemon parses JSON into a node/edge/strings graph, caches by snapshot_id
  |
  v
Daemon runs the requested detail mode (default: summary) and returns only the conclusion
  |
  v
Follow-up: analyze({what: "memory_snapshot", snapshot_id, detail: "dom_leaks" | "retainers" | ...})
  -> synchronous traversal of the cached graph, no re-capture
  |
  v
Comparison: analyze({..., detail: "leak_suspects", compare_to: <other snapshot_id>})
  -> diff two cached snapshots by constructor
```

## Implementation Strategy

**New server files**:
- A `memory_snapshot` handler wired into `analyzeHandlers` in
  `cmd/browser-agent/tools_analyze_dispatch.go`.
- A heap-snapshot parser that builds the node/edge/strings graph.
- A two-snapshot cache with `snapshot_id` keying and oldest-eviction.
- The detail-mode analyzers listed above.

**Modified server files**:
- `internal/tools/configure/mode_specs_analyze.go`: add the `memory_snapshot` mode hint and its
  optional parameters (`detail`, `snapshot_id`, `constructor`, `compare_to`,
  `include_detached_dom`, `save_path`, `top_n`).

**Extension files**:
- `src/background/cdp-dispatch.ts` and `src/background/commands/analyze.ts`: add the
  `HeapProfiler` capture sequence and chunked transfer to the daemon.

**Trade-offs**:
- Analysis in Go versus in the agent: parsing and traversing a fifty-megabyte heap in Go takes
  one to three seconds and returns kilobytes. Returning the same data to the agent would consume
  hundreds of thousands of tokens, often beyond the context window.
- Two-snapshot cache versus unbounded history: two snapshots cover the dominant
  snapshot-interact-snapshot-diff leak workflow while bounding daemon memory.

## Edge Cases & Assumptions

### Edge Cases

- **Debugger already attached**: return the existing attach-conflict error from
  `src/background/cdp-dispatch.ts` with a recovery action.
- **Very large heap (500MB)**: chunked transfer and streaming parse keep memory bounded; if the
  configured limit is exceeded, return a clear size-limit error.
- **`compare_to` references an evicted snapshot**: return an error naming the missing
  `snapshot_id` and suggesting a fresh capture.
- **`retainers` with an unknown constructor**: return an empty result set with a note rather than
  an error.
- **`snapshot_id` re-query after navigation**: the cache clears on navigation; re-querying a
  cleared snapshot returns a clear "snapshot no longer cached" error.
- **`raw` or `save_path` to an unwritable path**: return a filesystem error without crashing the
  capture.

### Assumptions

- A1: The extension is connected and tracking a tab.
- A2: The `debugger` permission is granted and the CDP lifecycle in
  `src/background/cdp-dispatch.ts` is the single point of debugger management.
- A3: The tracked tab hosts a real web page with a JavaScript heap, not an internal browser page.
- A4: The WebSocket channel supports chunked transfer for multi-megabyte payloads.
- A5: At most two snapshots are needed concurrently for comparison workflows.

## Risks & Mitigations

### Risk 1: Snapshot transfer overwhelms the channel
- **Description**: A five-hundred-megabyte snapshot could stall the WebSocket or exhaust memory.
- **Mitigation**: Stream in chunks; enforce a configurable maximum snapshot size and fail fast
  with a clear error when exceeded.

### Risk 2: Parse time blocks the daemon
- **Description**: Parsing a large heap JSON could take several seconds.
- **Mitigation**: Run capture as an asynchronous command with a sixty-second budget; parsing
  happens off the synchronous request path, and analysis queries hit the cached graph.

### Risk 3: Stale or mismatched snapshot comparisons
- **Description**: Diffing snapshots from different pages or after eviction yields misleading
  results.
- **Mitigation**: Tag each snapshot with its URL and timestamp; validate that `compare_to`
  exists and refer to both snapshots' metadata in the diff response.

### Risk 4: Cache memory pressure
- **Description**: Holding parsed graphs consumes daemon memory.
- **Mitigation**: Cap the cache at two snapshots with oldest-eviction and clear on navigation or
  session end.

## Dependencies

### Depends on:
- The analyze dispatch registry (`cmd/browser-agent/tools_analyze_dispatch.go`).
- The CDP attach/detach lifecycle and `HeapProfiler` domain (`src/background/cdp-dispatch.ts`).
- The asynchronous command pattern and command-result polling
  (`cmd/browser-agent/internal/toolinteract/interact_command_builder.go`,
  `observe({what: "command_result"})`).
- WebSocket chunked transfer for large payloads.

### Depended on by:
- AI agents diagnosing memory leaks that need detached-DOM attribution, retainer chains, and
  before-and-after diffs without parsing raw heap data.

## Performance Considerations

| Metric | Target | Implementation notes |
|--------|--------|---------------------|
| Capture duration | Seconds | Asynchronous command, 60s budget |
| Extension-to-daemon transfer | 5-15s for large heaps | Chunked over WebSocket |
| Daemon parse of 50MB heap | 1-3s | Single pass into node/edge graph |
| Detail-mode query | < 100ms | Single pass or bounded traversal of cached graph |
| Summary response size | 3-5KB | Conclusions only |
| Full response size | 50-500KB | Complete graph (opt-in) |

## Security Considerations

**Data captured**: A JavaScript heap snapshot can contain in-memory strings, including
user-visible text and application state. The snapshot reflects whatever the page holds in memory.

**Data path**: The snapshot and all derived analysis stay on localhost (`127.0.0.1`). Nothing is
transmitted externally. The `raw` and `save_path` options write only to user-specified local
paths.

**Redaction**: Default detail modes return aggregates (counts, sizes, constructor names,
retainer chains) rather than raw string contents. The `strings` mode surfaces string values by
retained size; the agent should treat its output as potentially sensitive, consistent with the
existing localhost-only privacy model.

**Attack surface**: The mode attaches the debugger (already granted) and enables `HeapProfiler`.
It reads memory; it does not execute page-supplied code or modify page state.
