---
feature: memory-snapshot
status: proposed
version: null
tool: analyze
mode: memory_snapshot
authors: []
created: 2026-03-06
updated: 2026-03-06
doc_type: product-spec
feature_id: feature-memory-snapshot
last_reviewed: 2026-07-05
---

# Memory Snapshot

> Capture a heap snapshot via CDP and return structured memory analysis for AI consumption — with progressive disclosure from summary to targeted analysis to full graph.

## Problem

Memory leaks are one of the hardest bugs for AI agents to diagnose. Kaboom currently has no visibility into JavaScript heap state — the agent can observe symptoms (growing memory in task manager, degraded performance over time) but cannot identify which objects are leaking, what's retaining them, or how large the retained trees are.

Chrome DevTools MCP exposes `take_memory_snapshot` which captures a `.heapsnapshot` file. However, raw heap snapshots are 50-500MB and incomprehensible without Chrome DevTools' heap snapshot viewer. An AI agent needs structured, actionable data — not a raw binary dump.

## Solution

Add `analyze(what="memory_snapshot")` which captures a heap snapshot via CDP's `HeapProfiler` domain and returns a structured summary: total heap size, object counts by constructor, top retainers, potential leak indicators, and detached DOM nodes.

The key design principle: **capture once, analyze many ways.** The snapshot is captured via CDP and cached by `snapshot_id`. The AI starts with a token-efficient summary and can run targeted analysis queries against the same cached snapshot — all computed server-side in Go, returning only conclusions, not raw data.

This is where Kaboom's architecture pays off. Graph traversal, aggregation, diffing, and pattern matching are O(n) in Go but would consume the LLM's entire context window if done via raw data. **Everything the LLM would do by scanning rows of data, the daemon does in a single pass and returns just the conclusion.**

## MCP Interface

**Tool:** `analyze`
**Mode:** `memory_snapshot`

### Request

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "memory_snapshot",
    "include_detached_dom": true
  }
}
```

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `what` | string | yes | -- | Must be `"memory_snapshot"` |
| `detail` | string | no | `"summary"` | Analysis mode. See detail modes table below. |
| `snapshot_id` | string | no | null | Re-query a previously captured snapshot (avoids re-capturing) |
| `constructor` | string | no | null | Filter by constructor name (for `retainers`, `instances` modes) |
| `compare_to` | string | no | null | Second `snapshot_id` for diff-based modes (`leak_suspects`, `growth_report`) |
| `include_detached_dom` | boolean | no | `true` | Include detached DOM node analysis |
| `save_path` | string | no | null | Optionally save raw `.heapsnapshot` to this file path |
| `top_n` | number | no | `20` | Number of results to include |

### Detail Modes

Each mode runs a specific analysis server-side and returns only the conclusion. The snapshot is captured once; all modes query the same cached data.

| Mode | ~Response Size | What It Does | When to Use |
|------|---:|---|---|
| **`summary`** | ~3-5KB | Heap overview, top objects by retained size, detached DOM, leak indicators with actionable suggestions | **Start here.** Identifies the problem in most cases. |
| **`dom_leaks`** | ~2-4KB | Only detached DOM nodes: retainer chains, component attribution, count and retained size | The #1 thing devs look for. Focused, minimal tokens. |
| **`retainers`** | ~2-5KB | "What's keeping X alive?" Walks the retainer graph for a specific `constructor` and returns top retainer chains (BFS/DFS in Go, milliseconds) | When summary shows a suspicious constructor and you need to trace ownership |
| **`leak_suspects`** | ~3-6KB | Diff two snapshots (`snapshot_id` vs `compare_to`). Objects that grew significantly between snapshots are likely leaks. Returns deltas by constructor. | The classic memory leak workflow: snapshot, interact, snapshot, diff |
| **`growth_report`** | ~4-8KB | Full comparison of two snapshots: new allocations, growing objects, shrinking objects, net delta by constructor | Comprehensive before/after analysis |
| **`strings`** | ~2-4KB | Top strings by retained size with deduplication count. Unbounded log buffers, cached API responses, growing state. | When summary shows high string retention |
| **`closures`** | ~2-4KB | Closures with disproportionate retained-vs-shallow size ratios. Identifies closures retaining large scopes. | Classic JS leak pattern: closures capturing more than intended |
| **`full`** | ~50-500KB | Complete parsed heap graph: all constructor groups, all retainer chains, edge types, node-level detail | When targeted modes aren't enough and the AI needs to reason about the graph |
| **`raw`** | resource URI | Full `.heapsnapshot` JSON. Writes to `save_path` or returns a resource URI. | Manual inspection in Chrome DevTools, or custom external analysis |

### Progressive Disclosure Workflow

```
# Step 1: Capture and get summary (~3KB)
analyze(what="memory_snapshot")
# -> snapshot_id: "snap-1", summary with leak indicators

# Step 2: Summary says "47 detached DOM nodes" — drill in (~2KB)
analyze(what="memory_snapshot", snapshot_id="snap-1", detail="dom_leaks")
# -> detached nodes with retainer chains and component attribution

# Step 3: Summary says "EventListenerInfo growing" — trace retainers (~3KB)
analyze(what="memory_snapshot", snapshot_id="snap-1", detail="retainers", constructor="EventListenerInfo")
# -> top retainer chains for EventListenerInfo objects

# Step 4: After fixing, take another snapshot and diff (~4KB)
analyze(what="memory_snapshot")
# -> snapshot_id: "snap-2"
analyze(what="memory_snapshot", snapshot_id="snap-2", detail="leak_suspects", compare_to="snap-1")
# -> objects that grew between snap-1 and snap-2

# Step 5: Full growth report for the PR description (~6KB)
analyze(what="memory_snapshot", snapshot_id="snap-2", detail="growth_report", compare_to="snap-1")
# -> complete before/after comparison
```

Total tokens for a full memory leak investigation: **~15-20KB across 5 targeted queries**. An LLM trying to reason about the same data from raw heap dumps would need 500KB+ of context — if it could parse the format at all.

### Response (summary)

```json
{
  "memory_snapshot": {
    "snapshot_id": "snap-abc123",
    "detail": "summary",
    "url": "https://example.com",
    "timestamp": "2026-03-06T14:30:00Z",
    "heap_summary": {
      "total_size_bytes": 52428800,
      "total_size_human": "50.0 MB",
      "total_objects": 284000,
      "gc_roots": 1200
    },
    "top_by_retained_size": [
      {
        "constructor": "Array",
        "count": 12400,
        "shallow_size_bytes": 1200000,
        "retained_size_bytes": 8500000,
        "retained_size_human": "8.1 MB"
      },
      {
        "constructor": "EventListenerInfo",
        "count": 3200,
        "shallow_size_bytes": 256000,
        "retained_size_bytes": 4200000,
        "retained_size_human": "4.0 MB"
      }
    ],
    "detached_dom": {
      "count": 47,
      "total_retained_bytes": 2100000,
      "total_retained_human": "2.0 MB",
      "examples": [
        {
          "node_type": "HTMLDivElement",
          "retained_bytes": 450000,
          "retainer_chain": "Window > app > componentCache > Map > HTMLDivElement"
        }
      ]
    },
    "leak_indicators": [
      {
        "indicator": "high_detached_dom",
        "severity": "high",
        "message": "47 detached DOM nodes retaining 2.0 MB. Common cause: removed components still referenced in caches, event handlers, or closures.",
        "suggestion": "Check component unmount lifecycle -- ensure event listeners are removed and references cleared."
      },
      {
        "indicator": "high_event_listeners",
        "severity": "moderate",
        "message": "3,200 EventListenerInfo objects retaining 4.0 MB. This may indicate listeners not being cleaned up on component removal.",
        "suggestion": "Use AbortController or explicit removeEventListener in cleanup/unmount handlers."
      }
    ],
    "available_detail_modes": ["dom_leaks", "retainers", "strings", "closures", "full", "raw"],
    "duration_ms": 3200
  }
}
```

### Response (leak_suspects)

```json
{
  "memory_snapshot": {
    "snapshot_id": "snap-def456",
    "detail": "leak_suspects",
    "compared_to": "snap-abc123",
    "time_between_ms": 30000,
    "heap_delta": {
      "before_bytes": 52428800,
      "after_bytes": 68157440,
      "delta_bytes": 15728640,
      "delta_human": "+15.0 MB"
    },
    "suspects": [
      {
        "constructor": "HTMLDivElement (detached)",
        "before_count": 47,
        "after_count": 142,
        "delta_count": "+95",
        "before_retained_bytes": 2100000,
        "after_retained_bytes": 6800000,
        "delta_retained_human": "+4.5 MB",
        "confidence": "high",
        "reason": "Detached DOM node count tripled. Strong indicator of component leak on unmount."
      },
      {
        "constructor": "EventListenerInfo",
        "before_count": 3200,
        "after_count": 4800,
        "delta_count": "+1600",
        "before_retained_bytes": 4200000,
        "after_retained_bytes": 6300000,
        "delta_retained_human": "+2.0 MB",
        "confidence": "high",
        "reason": "Event listeners growing proportionally with detached DOM. Likely same root cause."
      }
    ],
    "shrunk": [
      {
        "constructor": "InternalNode",
        "delta_count": "-200",
        "delta_retained_human": "-0.3 MB",
        "note": "Normal GC activity"
      }
    ],
    "verdict": "15 MB heap growth in 30s. Primary suspects: detached DOM nodes (+95) and event listeners (+1,600). These are likely the same leak — components being removed from the DOM without cleanup."
  }
}
```

### Response (retainers)

```json
{
  "memory_snapshot": {
    "snapshot_id": "snap-abc123",
    "detail": "retainers",
    "constructor": "EventListenerInfo",
    "total_instances": 3200,
    "total_retained_human": "4.0 MB",
    "top_retainer_chains": [
      {
        "chain": "Window > app > store > subscriptions > Array > EventListenerInfo",
        "instances_via_chain": 1800,
        "retained_human": "2.2 MB",
        "suggestion": "Store subscriptions array is accumulating listeners. Check if subscribe() has a matching unsubscribe() on component unmount."
      },
      {
        "chain": "Window > __zone_symbol__eventListeners > Map > EventListenerInfo",
        "instances_via_chain": 900,
        "retained_human": "1.1 MB",
        "suggestion": "Zone.js event listener tracking. If using Angular, ensure ngOnDestroy cleans up manual event listeners."
      }
    ],
    "common_retaining_edges": [
      {"edge_type": "property", "name": "subscriptions", "frequency": 56},
      {"edge_type": "element", "name": "Array[*]", "frequency": 44}
    ]
  }
}
```

## Implementation Approach

### Extension Side (CDP)

Uses the existing `chrome.debugger` infrastructure:

1. **Attach** debugger to tracked tab
2. **`HeapProfiler.enable`**
3. **`HeapProfiler.takeHeapSnapshot`** -- collects heap data via `HeapProfiler.addHeapSnapshotChunk` events
4. **Reassemble** snapshot chunks into complete JSON
5. **Send** raw parsed snapshot to daemon via WebSocket (chunked transfer for large snapshots)
6. **`HeapProfiler.disable`**, detach debugger

### Go Side (Daemon) — Where the Analysis Lives

The daemon receives the parsed snapshot and performs all analysis in Go. This is the key architectural decision: **analysis runs where compute is cheap (Go), not where tokens are expensive (LLM).**

- **Snapshot ingestion:** Parse `.heapsnapshot` JSON into an in-memory graph (nodes, edges, strings arrays)
- **Cache:** Store parsed graph keyed by `snapshot_id`. Max 2 snapshots cached.
- **`summary`:** Single pass: aggregate by constructor, find detached DOM (nodes with HTMLElement constructors not reachable from document root), compute leak heuristics
- **`dom_leaks`:** Filter to detached DOM nodes, walk retainer edges backwards (BFS), build retainer chains. O(detached_count * avg_chain_depth)
- **`retainers`:** Filter nodes by constructor, BFS backwards through edges, group by retainer path. O(filtered_count * graph_depth)
- **`leak_suspects`:** Map diff between two cached snapshots by constructor. O(n) where n = distinct constructors. Trivial.
- **`growth_report`:** Same as leak_suspects but includes shrunk objects, new constructors, and net deltas
- **`strings`:** Aggregate string nodes by value, sort by retained size, deduplicate. O(string_count)
- **`closures`:** Filter nodes with closure constructors, compute retained/shallow ratio, sort by disproportion. O(closure_count)
- **`full`:** Return the complete parsed graph as structured JSON
- **`raw`:** Write cached snapshot to `save_path` or return as resource URI

All analysis modes are single-pass or bounded-depth graph traversals. Even on a 50MB heap (284K objects), these complete in <100ms in Go.

### Snapshot Cache Lifecycle

- Cached snapshots are keyed by `snapshot_id` (generated on capture)
- Cache holds at most 2 snapshots (for comparison workflows like `leak_suspects` and `growth_report`)
- Oldest snapshot evicted when a third is captured
- Cache cleared on tab navigation or session end
- `snapshot_id` returned in every response so the AI can reference it

### Performance Considerations

- Heap snapshots can be large (50-500MB raw)
- Extension-to-daemon transfer: chunked via WebSocket, 5-15 seconds for large heaps
- Go parsing of `.heapsnapshot` JSON: 1-3 seconds for 50MB
- Analysis queries against parsed graph: <100ms each
- Summary response: ~3-5KB. Targeted modes: ~2-8KB. Full: ~50-500KB.
- Must use async command pattern with appropriate timeout (60s for capture, analysis queries are synchronous)

### Token Efficiency Analysis

| Approach | Tokens for Full Leak Investigation |
|---|---:|
| Raw `.heapsnapshot` in context | Impossible (50-500MB) |
| Chrome DevTools MCP (file path only) | N/A (agent can't read the file) |
| `detail="full"` in context | ~50,000-500,000 (often exceeds context window) |
| **Kaboom targeted modes** (5 queries) | **~15,000-20,000** |

The targeted mode approach uses **10-30x fewer tokens** than dumping the full graph, while providing more actionable information because the analysis is already done.

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Capture heap snapshot via CDP HeapProfiler | must |
| R2 | Return top object types by retained size | must |
| R3 | Identify detached DOM nodes with retainer chains | must |
| R4 | Provide leak indicator heuristics with actionable suggestions | must |
| R5 | Return total heap size and object count | must |
| R6 | Support all detail modes: summary, dom_leaks, retainers, leak_suspects, growth_report, strings, closures, full, raw | must |
| R7 | Cache snapshots by `snapshot_id` for re-query without re-capture | must |
| R8 | Support two-snapshot comparison via `compare_to` | must |
| R9 | Configurable `top_n` for result size control | should |
| R10 | Constructor filtering via `constructor` parameter | should |
| R11 | Optionally save raw `.heapsnapshot` to file | should |
| R12 | Token-efficient summary response (<5KB typical) | must |
| R13 | Async execution with 60s timeout for capture | must |
| R14 | Synchronous response for analysis queries on cached snapshots | must |
| R15 | Clear error when debugger cannot attach | must |
| R16 | Include `available_detail_modes` in summary response for discoverability | must |

## Non-Goals

- Not a continuous memory monitor. This is a point-in-time snapshot.
- Not a memory timeline. For tracking memory growth over time, the AI takes multiple snapshots and compares (the 2-snapshot cache and `leak_suspects`/`growth_report` modes support this workflow).

## Dependencies

- `debugger` permission (already in manifest)
- CDP `HeapProfiler` domain
- `cdp-dispatch.ts` attach/detach lifecycle (shipped)
- Async command infrastructure (shipped)
- WebSocket chunked transfer for large payloads (shipped)

## Competitive Context

Chrome DevTools MCP's `take_memory_snapshot` saves a raw `.heapsnapshot` file -- useful for manual inspection in DevTools but not directly actionable by an AI agent (binary format, 50-500MB, no analysis).

Kaboom's `analyze(what="memory_snapshot")` is a strict improvement:
- **Default:** Returns analyzed, token-efficient summary (~3-5KB) with leak indicators and actionable suggestions
- **Targeted analysis:** 7 server-side analysis modes that run graph traversals in Go and return only conclusions -- the LLM gets answers, not data to process
- **Snapshot comparison:** Built-in two-snapshot diffing for leak detection workflows
- **Progressive disclosure:** Start with summary, drill into dom_leaks/retainers/strings/closures, escalate to full/raw only if needed
- **10-30x token savings:** Full leak investigation in ~15-20K tokens vs. 500K+ for raw graph analysis
