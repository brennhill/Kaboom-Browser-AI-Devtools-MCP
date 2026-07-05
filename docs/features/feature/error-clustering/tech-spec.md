---
status: proposed
scope: feature/error-clustering/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-error-clustering
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-error-clustering.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Error Clustering Review](error-clustering-review.md).

# Technical Spec: Error Clustering

## Purpose

Browser applications generate errors. Many of those errors are related — the same root cause manifests as multiple stack traces across different components, different user actions, and different sessions. Today, the AI sees each error as an independent event. If a broken API response causes 4 different components to throw, the AI sees 4 separate errors and might investigate each independently. That wastes tokens, wastes time, and misses the root cause.

Error clustering groups related errors into clusters based on shared characteristics: common stack frames, shared error messages, temporal proximity, and causal chains. Instead of "here are 12 errors," the AI sees "here are 3 root causes, each with their related errors." This makes the AI dramatically more effective at diagnosing and fixing problems.

---

## How It Works

### Cluster Formation

When a new error arrives, the server compares it against existing clusters using three signals:

1. **Stack frame similarity**: Errors sharing 2+ stack frames in the same call path are likely related. The comparison ignores framework internals (React, Vue, Angular stack frames) and focuses on application code.

2. **Message similarity**: After stripping variable content (IDs, URLs, timestamps), errors with the same normalized message template belong together. "Cannot read property 'name' of undefined" from different call sites is likely the same bug.

3. **Temporal proximity**: Errors occurring within 2 seconds of each other are candidates for causal grouping — one error often triggers downstream errors quickly.

A new error joins an existing cluster if it matches on any two of these three signals. If it matches only one, it starts a new cluster (single-signal matches produce too many false positives).

### Cluster Structure

Each cluster has:
- A representative error (the first or most informative instance)
- A root cause hypothesis (the deepest common stack frame across instances)
- A count of instances
- The time range (first seen to last seen)
- A list of affected components (extracted from stack traces)
- A severity (inherited from the highest-severity instance)

### Root Cause Inference

The server identifies the likely root cause by finding the deepest application-code stack frame that appears in the majority (>50%) of cluster instances. This is typically the function that threw the original error, before it propagated up through event handlers and framework code.

If no common frame exists (errors are related by message only), the root cause is listed as the normalized error message itself.

### Message Normalization

Error messages are normalized by replacing variable content with placeholders:
- UUIDs → `{uuid}`
- Numeric IDs → `{id}`
- URLs → `{url}`
- File paths → `{path}`
- Timestamps → `{timestamp}`
- Quoted strings longer than 20 chars → `{string}`

This produces a template like: "Failed to fetch {url}: {id} not found" that matches across instances.

### Cluster Lifecycle

Clusters are ephemeral — they exist only while the server is running (consistent with Kaboom's session-scoped model). On restart, clustering starts fresh. This is intentional: stale clusters from yesterday's bugs shouldn't influence today's diagnosis.

A cluster is considered "resolved" and removed when no new instances arrive for 5 minutes. This prevents old clusters from accumulating indefinitely during a long session.

### MCP Interface

Error clusters are exposed through the existing `analyze` composite tool:

```
analyze(target: "errors")
```

Response:
```json
{
  "clusters": [
    {
      "id": "cluster_1",
      "representative_error": "TypeError: Cannot read property 'name' of undefined",
      "root_cause": "UserProfile.render (user-profile.js:42)",
      "instance_count": 4,
      "first_seen": "2026-01-24T15:30:00Z",
      "last_seen": "2026-01-24T15:30:02Z",
      "affected_components": ["UserProfile", "Dashboard", "Sidebar"],
      "severity": "error",
      "instances": [
        { "message": "Cannot read property 'name' of undefined", "source": "user-profile.js:42", "timestamp": "..." },
        { "message": "Cannot read property 'name' of undefined", "source": "dashboard.js:108", "timestamp": "..." }
      ]
    }
  ],
  "unclustered_errors": 2,
  "total_errors": 14,
  "summary": "3 error clusters identified. Primary root cause: null user object in UserProfile.render (4 instances). 2 unclustered errors."
}
```

Individual errors that don't match any cluster are still available through `observe(what: "errors")` as before.

### Integration with Existing Alerts

When a new cluster forms (3+ instances of a related error within 10 seconds), it generates a compound alert through the existing alert piggyback system:

```json
{
  "level": "error",
  "type": "error_cluster",
  "message": "Error cluster detected: 4 related errors from UserProfile.render",
  "cluster_id": "cluster_1",
  "instance_count": 4,
  "root_cause": "UserProfile.render (user-profile.js:42)"
}
```

This alert appears in the next `observe` response, drawing the AI's attention to the cluster rather than individual errors.

---

## Data Model

### Error Instance (input)

Each error arriving from the extension contains:
- Message (string)
- Stack trace (string, newline-separated frames)
- Source file and line (extracted from first app-code frame)
- Timestamp
- Error type (TypeError, ReferenceError, etc.)
- Severity (error, warning)

### Cluster (computed)

- ID (auto-generated, session-unique)
- Representative error (the instance with the most informative stack trace)
- Normalized message template
- Common stack frames (frames appearing in >50% of instances)
- Root cause frame (deepest common app-code frame)
- Instance list (capped at 20 — after that, only count increments)
- First/last seen timestamps
- Affected components (unique source files from instances)
- Instance count

### Stack Frame Parsing

Stack frames are parsed into structured form:
- Function name (or `<anonymous>`)
- Source file (relative path)
- Line number
- Column number

Framework frames are identified by path patterns: `node_modules/react`, `node_modules/vue`, `node_modules/@angular`, `webpack/bootstrap`, `zone.js`, etc. These are excluded from similarity comparison but retained in the display.

---

## Edge Cases

- **Single error**: Never clustered. Clusters require 2+ instances.
- **Errors without stack traces**: Clustered by message similarity only (single-signal clustering allowed when no stack is available, since it's the only signal).
- **Minified stack traces**: Source maps are not available to the server. Clustering works on the raw frames — minified frames like `a.js:1:2345` are still comparable across instances from the same bundle.
- **Very high error rate** (100+ errors/second): Clustering continues but instance storage is capped at 20 per cluster. Count still increments. This prevents memory exhaustion during error storms.
- **Errors from different tabs/pages**: Clustered together if they match. The root cause is often shared (same broken API, same bad deploy).
- **Cluster splits**: If a cluster accumulates errors that are later determined to be unrelated (the matching was too broad), there's no split mechanism. The cluster expires after 5 minutes of inactivity and new errors form tighter clusters. Simplicity over perfection.
- **Memory pressure**: Under memory pressure (existing memory enforcement system), clusters are the first candidates for eviction. Individual errors in the buffer are preserved; clusters are recomputable.

---

## Performance Constraints

- Cluster matching per new error: under 1ms (compare against max 50 active clusters)
- Stack frame parsing: under 0.5ms per error (regex-based, cached)
- Message normalization: under 0.1ms (regex replacements)
- Memory per cluster: under 5KB (20 instances × ~250 bytes each)
- Maximum active clusters: 50 (oldest evicted when exceeded)
- Maximum instances per cluster: 20 (count continues, list capped)

---

## Test Scenarios

1. Two errors with shared stack frames → clustered together
2. Two errors with same normalized message → clustered together
3. Temporal proximity alone → not clustered (needs second signal)
4. Stack similarity + temporal proximity → clustered
5. Message similarity + stack similarity → clustered
6. Single error → not clustered
7. Error without stack trace → clustered by message only
8. Framework frames excluded from similarity comparison
9. Root cause identified as deepest common app-code frame
10. Cluster alert generated at 3+ instances
11. Cluster expires after 5 minutes of inactivity
12. Instance cap at 20 (count continues incrementing)
13. Active cluster cap at 50 (oldest evicted)
14. `analyze(target: "errors")` returns all clusters
15. Unclustered errors counted separately
16. Message normalization replaces UUIDs, IDs, URLs
17. High error rate doesn't cause memory exhaustion
18. Server restart clears all clusters

---

## File Locations

Server implementation: `cmd/browser-agent/clustering.go` (cluster formation, matching, lifecycle, MCP tool handler).

Tests: `cmd/browser-agent/clustering_test.go`.
