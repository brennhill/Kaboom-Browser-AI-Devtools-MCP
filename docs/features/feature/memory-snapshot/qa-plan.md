---
status: proposed
scope: feature/memory-snapshot/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-06-29
doc_type: qa-plan
feature_id: feature-memory-snapshot
last_reviewed: 2026-06-29
last_verified_version: 0.8.4
last_verified_date: 2026-06-29
---

# QA Plan: Memory Snapshot

> QA plan for the Memory Snapshot feature. Covers data leak analysis, LLM clarity, simplicity
> assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. A JavaScript heap snapshot can
contain in-memory user data, so this is the highest-sensitivity feature in the analyze tool.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Raw heap strings in default response | Verify the default `summary` mode returns aggregates (counts, sizes, constructors), not raw in-memory string contents. | critical |
| DL-2 | `strings` mode exposure | The `strings` mode surfaces string values by retained size. Verify the response stays on localhost and the agent is informed the content may be sensitive. | high |
| DL-3 | Saved `.heapsnapshot` location | Verify `save_path` and `raw` write only to the user-specified local path. | high |
| DL-4 | External transmission of snapshot | Verify the snapshot is streamed only to the local daemon over `127.0.0.1`, never to an external host. | critical |
| DL-5 | Cache lifetime | Verify cached snapshots clear on tab navigation and session end so stale memory contents do not linger. | medium |
| DL-6 | Retainer chains revealing tokens | Verify retainer-chain labels show property and constructor names, not raw credential values. | high |

### Negative Tests (must NOT leak)
- [ ] `summary` mode contains no raw heap string contents
- [ ] No snapshot bytes are transmitted to external servers
- [ ] Cached snapshots are cleared on navigation
- [ ] `save_path` writes only to the requested local path

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the responses can act on them without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Detail-mode discoverability | The `summary` response lists `available_detail_modes` so the agent knows how to drill in. | [ ] |
| CL-2 | `snapshot_id` reuse | The agent understands it can re-query the same `snapshot_id` without re-capturing. | [ ] |
| CL-3 | Retained versus shallow size | The agent distinguishes retained size (whole subtree) from shallow size (object itself). | [ ] |
| CL-4 | Detached DOM meaning | The agent understands detached DOM nodes are removed-but-retained elements, a leak signal. | [ ] |
| CL-5 | Diff direction in `leak_suspects` | The agent understands which snapshot is the baseline (`compare_to`) and which is current. | [ ] |
| CL-6 | Leak-indicator suggestions | The agent can act on the actionable suggestion attached to each leak indicator. | [ ] |

### Common LLM Misinterpretation Risks
- [ ] Agent re-captures instead of re-querying a cached `snapshot_id`
- [ ] Agent confuses retained size with shallow size
- [ ] Agent reverses the baseline and current snapshots in a diff

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load.

**Complexity Score:** Low (summary), Medium (full leak investigation across modes)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Capture and summarize | 1 step: `analyze({what: "memory_snapshot"})` | No -- already minimal |
| Drill into detached DOM | 1 step: add `detail: "dom_leaks"` with `snapshot_id` | No -- single parameter |
| Trace retainers | 1 step: add `detail: "retainers", constructor: "X"` | No |
| Leak diff | 2 captures + 1 diff: `detail: "leak_suspects", compare_to` | Inherent to before/after workflow |

### Default Behavior Verification
- [ ] Works with zero parameters (captures and returns `summary`)
- [ ] `snapshot_id` is always returned for follow-up queries
- [ ] `include_detached_dom` defaults to true

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Snapshot parser builds graph | A `.heapsnapshot` JSON fixture | Node/edge/strings graph | must |
| UT-2 | Summary aggregates by constructor | Parsed graph | Top objects by retained size | must |
| UT-3 | Detached DOM detection | Graph with detached HTMLElement nodes | Detached nodes with retained size | must |
| UT-4 | Leak indicators computed | Graph with high detached-DOM count | Indicator with severity and suggestion | must |
| UT-5 | Retainer BFS for a constructor | Graph + `constructor` filter | Top retainer chains | must |
| UT-6 | Two-snapshot diff | Two cached graphs | Constructor deltas in `leak_suspects` | must |
| UT-7 | Strings dedup and rank | Graph with duplicate strings | Top strings by retained size with counts | should |
| UT-8 | Closure disproportion ranking | Graph with closures | Closures ranked by retained/shallow ratio | should |
| UT-9 | Cache eviction at three captures | Capture three snapshots | Oldest evicted | must |
| UT-10 | `top_n` bounds result size | `top_n: 5` | At most five results | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Capture round trip | Daemon -> async command -> extension HeapProfiler -> chunked transfer -> parser | `summary` returned via MCP | must |
| IT-2 | Re-query cached snapshot | `snapshot_id` reused for `dom_leaks` | Synchronous result, no re-capture | must |
| IT-3 | Two-snapshot leak workflow | Capture, interact, capture, `leak_suspects` | Growth deltas surfaced | must |
| IT-4 | Debugger attach conflict | DevTools open | Recovery-action error | must |
| IT-5 | Navigation clears cache | Navigate, then re-query old `snapshot_id` | "no longer cached" error | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Parse 50MB heap | Parse time | 1-3s | should |
| PT-2 | Detail-mode query | Traversal time | < 100ms | must |
| PT-3 | Summary response size | Bytes | 3-5KB | must |
| PT-4 | Chunked transfer of large heap | Transfer time | 5-15s | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Internal browser page | Tracked tab on `chrome://` | Cannot-attach error | must |
| EC-2 | Oversized heap | Heap exceeds configured limit | Clear size-limit error | must |
| EC-3 | `compare_to` evicted | Diff against missing snapshot | Error naming the missing id | must |
| EC-4 | Unknown constructor in `retainers` | `constructor: "DoesNotExist"` | Empty result with note | should |
| EC-5 | Unwritable `save_path` | `save_path` to a read-only dir | Filesystem error, capture intact | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web page with a reproducible memory leak loaded and tracked (for example, a page that
      mounts and unmounts components without cleanup)

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `analyze({what: "memory_snapshot"})` | Page is being tracked | `summary` with heap size, top objects, leak indicators, `snapshot_id` | [ ] |
| UAT-2 | `analyze({what: "memory_snapshot", snapshot_id: "...", detail: "dom_leaks"})` | -- | Detached DOM nodes with retainer chains, no re-capture | [ ] |
| UAT-3 | `analyze({what: "memory_snapshot", snapshot_id: "...", detail: "retainers", constructor: "EventListenerInfo"})` | -- | Top retainer chains for that constructor | [ ] |
| UAT-4 | Interact to trigger the leak, then capture again and run `detail: "leak_suspects", compare_to` | Memory grows | Suspects with positive deltas and a verdict | [ ] |
| UAT-5 | `analyze({what: "memory_snapshot", snapshot_id: "...", detail: "raw", save_path: "/tmp/heap.heapsnapshot"})` | File written locally | `.heapsnapshot` saved to the path | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Localhost-only transfer | Monitor network during capture | Only `127.0.0.1:7890` traffic | [ ] |
| DL-UAT-2 | Summary hides raw strings | Inspect `summary` output | No raw in-memory string contents | [ ] |
| DL-UAT-3 | Cache clears on navigation | Navigate, re-query old id | "no longer cached" error | [ ] |

### Regression Checks
- [ ] Existing `analyze({what: "performance"})` still works
- [ ] CDP-based interact actions still work after a capture detaches the debugger
- [ ] Extension performance is not degraded by the HeapProfiler capture path

---

## Sign-Off

| Area | Tester | Date | Pass/Fail |
|------|--------|------|-----------|
| Data Leak Analysis | | | |
| LLM Clarity | | | |
| Simplicity | | | |
| Code Tests | | | |
| UAT | | | |
| **Overall** | | | |
