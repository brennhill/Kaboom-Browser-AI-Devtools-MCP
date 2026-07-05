---
status: proposed
scope: feature/push-regression/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-push-regression
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-push-regression.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Push Regression Review](push-regression-review.md).

# Technical Spec: Push Notification on Regression

## Purpose

Today, the AI agent must explicitly call `check_performance` to learn whether its code changes caused a performance regression. This creates a gap in the feedback loop: the agent writes code, triggers a page reload, and then... does nothing with the performance data unless it remembers to ask. Most agents move on to the next task without checking.

Push notification on regression closes this gap by embedding performance regression alerts directly into the `get_changes_since` response — the polling endpoint the agent already uses to watch for browser state changes. If the AI is watching for changes (which it should be after modifying code), regressions surface automatically without an extra tool call.

This transforms the AI's development workflow from "write code → hope it works" to "write code → immediately learn if it broke something."

---

## Opportunity & Business Value

**Thesis-critical feedback loop**: This is the single most important feature for the "AI writes code → Kaboom detects impact → AI adjusts" cycle. Without it, the loop has a human-dependent step ("remember to check performance"). With it, the loop is fully automatic.

**Token efficiency**: Instead of the agent calling `check_performance` after every code change (expensive — returns full baseline data, resource breakdowns, etc.), the regression alert is a lightweight addition to the diff stream the agent is already consuming. No extra round-trip, no extra tokens for a "looks fine" response.

**Agentic workflow compatibility**: Agents that use `get_changes_since` for their observe loop (watching for console errors, new network requests, etc.) get performance regression detection for free. The performance signal rides alongside other state changes.

**Error attribution**: When the alert says "your last reload regressed load time by 800ms", the agent can immediately correlate this with its most recent code change. The causal link is clear because the timing is tight: the regression is reported on the first `get_changes_since` call after the reload.

---

## How It Works

### Detection Trigger

After the extension sends a new performance snapshot (which happens automatically 2 seconds after each page load), the server compares it against the stored baseline for that URL path. If any metric exceeds the regression thresholds, the server stores a "pending regression alert" that will be included in the next `get_changes_since` response.

The regression thresholds are the same ones used by `check_performance`:
- Load time: >20% regression from baseline
- FCP: >20% regression
- LCP: >20% regression
- TTFB: >50% regression (more tolerance for network variance)
- CLS: >0.1 absolute increase
- Total transfer size: >25% increase

### Delivery Mechanism

The `get_changes_since` response already returns a structured object with sections for console changes, network changes, WebSocket changes, etc. A new section is added:

```
"performance_alerts": [
  {
    "type": "regression",
    "url": "/dashboard",
    "detected_at": "2026-01-24T10:30:05Z",
    "summary": "Load time regressed by 847ms (1.2s → 2.1s) after reload",
    "metrics": {
      "load": { "baseline": 1200, "current": 2047, "delta_ms": 847, "delta_pct": 70.6 },
      "lcp": { "baseline": 800, "current": 1450, "delta_ms": 650, "delta_pct": 81.3 },
      "transfer_bytes": { "baseline": 245000, "current": 389000, "delta": 144000, "delta_pct": 58.8 }
    },
    "recommendation": "Check recently added scripts or stylesheets. Use causal_diff for resource-level comparison."
  }
]
```

Only regressed metrics are included (metrics within threshold are omitted to keep the response focused).

### Alert Lifecycle

1. Performance snapshot arrives from extension
2. Server compares against baseline → finds regressions
3. Server stores the alert with a unique ID and the checkpoint ID at detection time
4. Agent calls `get_changes_since` with a checkpoint before the detection → alert is included in the response
5. Alert is included in responses until the agent acknowledges it (by calling `get_changes_since` with a checkpoint AFTER the alert was created — meaning it's already seen it)
6. Alert is automatically cleared if the next snapshot for the same URL shows the regression resolved (self-healing code changes)

This means the alert appears exactly once in the agent's diff stream — on the first poll after the regression is detected. Subsequent polls with newer checkpoints won't repeat it.

### Multiple Regressions

If the agent reloads multiple pages or the same page multiple times before polling, all pending alerts accumulate. The `performance_alerts` array can contain multiple entries. Each has its own URL and timestamp.

Alerts are capped at 10 pending entries. Beyond that, the oldest are dropped (the agent clearly isn't watching).

---

## Data Model

### Pending Alert

Stored in the server's `PerformanceStore`:
- Alert ID (monotonic counter)
- URL path that regressed
- Detection timestamp
- Checkpoint ID at detection time (for lifecycle management)
- Metric deltas (only regressed metrics)
- Whether it's been "seen" (checkpoint has advanced past it)
- Whether it's been "resolved" (subsequent snapshot for same URL is within threshold)

### Server State

Added to the performance subsystem:
- `pendingAlerts`: Slice of pending regression alerts (max 10)
- `alertCounter`: Monotonic ID for alerts
- `lastSnapshotTime`: Per-URL timestamp of the most recent snapshot (for detecting reloads)

---

## Integration with get_changes_since

The `get_changes_since` handler checks for pending alerts whose checkpoint ID is greater than or equal to the requested checkpoint. It includes them in the response under the `performance_alerts` key.

After including alerts in a response, the server marks them as "delivered at checkpoint X". On the next call with a checkpoint > X, the alert is no longer included.

This piggybacks on the existing checkpoint-based diffing system — no new infrastructure needed.

---

## Integration with check_performance

The `check_performance` tool is unchanged. It still returns full baseline comparisons, resource breakdowns, and regression details. The push alert is a lightweight "heads up" — if the agent wants full details, it calls `check_performance` after receiving an alert.

The alert's `recommendation` field suggests this: "Use `check_performance` for full details" or "Use `causal_diff` for resource-level comparison."

---

## Edge Cases

- **No baseline exists yet**: No alert is generated. The first snapshot for a URL creates the baseline; alerts only fire on subsequent snapshots.
- **Agent never polls `get_changes_since`**: Alerts accumulate up to 10, then oldest are dropped. No memory leak.
- **Multiple tabs loading the same URL**: Each snapshot is compared independently. If tab A loads fast and tab B loads slow, the slow one generates an alert.
- **Baseline is stale** (from a previous session, loaded via persistent memory): Alerts still fire. The baseline represents "known good" — if the current session is slower, that's still a regression worth reporting.
- **Regression resolves itself** (developer fixes the code, reloads): The next snapshot clears the alert. The agent sees the alert once, then it disappears.
- **Server restart between snapshot and poll**: Pending alerts are lost (they're in-memory only). This is acceptable — the agent can always call `check_performance` explicitly.
- **Performance snapshot arrives during `get_changes_since` handling**: The alert is stored after the snapshot is processed. If the checkpoint-based query is already in flight, the alert appears on the next poll.

---

## Performance Constraints

- Regression detection per snapshot: under 0.5ms (compare ~6 numeric values against baseline)
- Alert inclusion in `get_changes_since`: under 0.1ms (scan max 10 pending alerts)
- Memory for 10 pending alerts: under 5KB
- No additional HTTP requests or goroutines

---

## Test Scenarios

1. Snapshot within threshold → no alert generated
2. Snapshot with load time 30% over baseline → alert generated with correct delta
3. Alert appears in `get_changes_since` response under `performance_alerts`
4. Alert not repeated in subsequent `get_changes_since` with newer checkpoint
5. Multiple regressions → multiple alerts in response
6. Max 10 pending alerts → oldest dropped when 11th arrives
7. Subsequent snapshot resolves regression → alert cleared before delivery
8. No baseline for URL → no alert (first snapshot creates baseline, doesn't alert)
9. Alert includes only regressed metrics (within-threshold metrics omitted)
10. `recommendation` field suggests next action
11. Alert checkpoint tracking: alert created at checkpoint 5, query from checkpoint 3 → included
12. Alert checkpoint tracking: alert created at checkpoint 5, query from checkpoint 6 → not included
13. CLS regression (absolute increase > 0.1) → alert generated
14. TTFB regression under 50% → no alert (higher tolerance for network variance)
15. Server restart → pending alerts lost, no crash

---

## File Locations

Server implementation: Changes to `cmd/browser-agent/performance.go` (alert detection and storage) and `cmd/browser-agent/ai_checkpoint.go` (alert inclusion in `get_changes_since` response).

Tests: `cmd/browser-agent/performance_test.go` (alert detection) and `cmd/browser-agent/ai_checkpoint_test.go` (delivery lifecycle).
