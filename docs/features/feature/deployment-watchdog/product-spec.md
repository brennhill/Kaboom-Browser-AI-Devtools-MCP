---
feature: deployment-watchdog
status: proposed
version: null
tool: configure, observe
mode: watchdog, deployment_status
authors: []
created: 2026-01-28
updated: 2026-01-28
doc_type: product-spec
feature_id: feature-deployment-watchdog
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Deployment Watchdog

> Monitors a web application after deployment, comparing runtime behavior against pre-deployment baselines. Alerts the AI when regressions are detected, enabling automated diagnosis and rollback decisions.

## Problem

After deploying a web application, the AI coding agent has no systematic way to verify that the deployment succeeded at runtime. Today, the agent either:

1. Moves on to the next task without checking, hoping nothing broke.
2. Manually calls `observe({what: "errors"})` and `observe({what: "performance"})` after deploy, requiring the agent to remember and interpret raw data.
3. Uses `diff_sessions` to do a one-shot before/after comparison, but this is a single point-in-time check that misses delayed regressions (e.g., errors that surface after the cache expires, or latency that degrades under load).

None of these approaches provide **continuous, time-bounded monitoring** with **automatic regression alerting**. The AI needs a "set and watch" mechanism: start monitoring after deploy, get notified if something goes wrong, and stop when the soak period completes without incident.

This matters because deployment regressions often emerge minutes after deploy (not immediately), and the AI agent should be watching the application rather than relying on the developer to report "something broke."

## Solution

Deployment Watchdog is a **time-bounded monitoring session** that the AI starts after a deployment. It captures a pre-deployment baseline, then continuously compares incoming telemetry against that baseline for a configurable duration. If a regression is detected (new errors, performance degradation, network failures), the watchdog surfaces an alert that the AI receives through the existing changes polling mechanism.

The watchdog composes three existing Kaboom primitives:

1. **Session snapshots** (`diff_sessions`) -- for baseline capture and point-in-time comparison.
2. **Behavioral baselines** -- for structured behavioral fingerprinting (API shapes, error fingerprints, timing tolerances).
3. **Push regression alerts** -- for delivering regression notifications through the `observe({what: "changes"})` polling stream.

The watchdog adds a **temporal dimension** that none of these primitives provide individually: it monitors over a duration, evaluating each new telemetry event against the baseline, rather than requiring the AI to manually trigger comparisons.

## User Stories

- As an AI coding agent, I want to start a monitoring session after triggering a deployment so that I am automatically alerted if the deployment causes regressions.
- As an AI coding agent, I want to define what "regression" means for this deployment (error rate thresholds, latency budgets, specific endpoints to watch) so that I am not overwhelmed with false positives.
- As an AI coding agent, I want the watchdog to run for a defined soak period so that I can confidently declare the deployment healthy if no alerts fire.
- As an AI coding agent, I want to receive regression alerts through the same polling mechanism I already use (`observe({what: "changes"})`) so that I do not need to learn a new observation pattern.
- As an AI coding agent, I want to see a deployment summary when the watchdog completes so that I can report the outcome to the developer.
- As a developer using Kaboom, I want the AI to catch deployment regressions within minutes so that I can decide to roll back before users are impacted.

## MCP Interface

Deployment Watchdog uses two tools: `configure` to manage watchdog sessions, and `observe` to check deployment status.

### Starting a Watchdog Session

**Tool:** `configure`
**Action:** `watchdog`

#### Request: Start

```json
{
  "tool": "configure",
  "arguments": {
    "action": "watchdog",
    "watchdog_action": "start",
    "deployment_label": "v2.3.0",
    "duration_minutes": 15,
    "url_scope": "/app",
    "thresholds": {
      "max_new_errors": 0,
      "latency_factor": 1.5,
      "max_network_error_rate": 0.05,
      "vitals_regression_pct": 20
    }
  }
}
```

#### Response: Start

```json
{
  "action": "watchdog_started",
  "watchdog_id": "wd-1706400000-1",
  "deployment_label": "v2.3.0",
  "status": "capturing_baseline",
  "baseline": {
    "captured_at": "2026-01-28T10:00:00Z",
    "console_errors": 2,
    "network_error_rate": 0.01,
    "page_url": "/app/dashboard",
    "performance": {
      "load_time_ms": 1200,
      "lcp_ms": 800,
      "cls": 0.05
    }
  },
  "monitoring_until": "2026-01-28T10:15:00Z",
  "thresholds": {
    "max_new_errors": 0,
    "latency_factor": 1.5,
    "max_network_error_rate": 0.05,
    "vitals_regression_pct": 20
  }
}
```

The watchdog captures the current browser state as the pre-deployment baseline when `start` is called. The AI should call `start` **before** the deployment begins. The AI then triggers the deployment through whatever mechanism is appropriate (CI/CD API, Git push, etc.) and the watchdog monitors the application as it reloads with the new version.

#### Request: Status Check

```json
{
  "tool": "configure",
  "arguments": {
    "action": "watchdog",
    "watchdog_action": "status",
    "watchdog_id": "wd-1706400000-1"
  }
}
```

#### Response: Status (Monitoring, No Issues)

```json
{
  "action": "watchdog_status",
  "watchdog_id": "wd-1706400000-1",
  "deployment_label": "v2.3.0",
  "status": "monitoring",
  "elapsed_minutes": 7,
  "remaining_minutes": 8,
  "checks_completed": 14,
  "alerts": [],
  "current_health": {
    "error_rate": 0.0,
    "latency_vs_baseline": "+5%",
    "network_error_rate": 0.01,
    "verdict": "healthy"
  }
}
```

#### Response: Status (Alert Fired)

```json
{
  "action": "watchdog_status",
  "watchdog_id": "wd-1706400000-1",
  "deployment_label": "v2.3.0",
  "status": "alert",
  "elapsed_minutes": 3,
  "remaining_minutes": 12,
  "checks_completed": 6,
  "alerts": [
    {
      "alert_id": "wa-1",
      "severity": "high",
      "type": "new_errors",
      "detected_at": "2026-01-28T10:03:00Z",
      "summary": "2 new console errors detected post-deployment",
      "details": {
        "new_errors": [
          {"message": "TypeError: Cannot read property 'user' of undefined", "count": 5},
          {"message": "PaymentService is not defined", "count": 1}
        ]
      },
      "recommendation": "New console errors appeared after deployment. Use observe({what: 'errors'}) for full error context. Consider rollback if these affect critical paths."
    }
  ],
  "current_health": {
    "error_rate": 0.03,
    "latency_vs_baseline": "+12%",
    "network_error_rate": 0.01,
    "verdict": "degraded"
  }
}
```

#### Request: Stop (Manual)

```json
{
  "tool": "configure",
  "arguments": {
    "action": "watchdog",
    "watchdog_action": "stop",
    "watchdog_id": "wd-1706400000-1",
    "reason": "rollback_triggered"
  }
}
```

#### Response: Stop / Completion Summary

```json
{
  "action": "watchdog_stopped",
  "watchdog_id": "wd-1706400000-1",
  "deployment_label": "v2.3.0",
  "status": "stopped",
  "reason": "rollback_triggered",
  "summary": {
    "duration_minutes": 3,
    "total_checks": 6,
    "total_alerts": 1,
    "alert_breakdown": {
      "new_errors": 1,
      "latency_regression": 0,
      "network_failure": 0,
      "vitals_regression": 0
    },
    "verdict": "regressed",
    "baseline_vs_final": {
      "new_errors": 2,
      "resolved_errors": 0,
      "latency_change": "+12%",
      "network_error_rate_change": "+0%"
    }
  }
}
```

When the watchdog duration elapses without alerts, it auto-stops with `"reason": "soak_complete"` and `"verdict": "healthy"`.

### Observing Deployment Status (Passive Alerts)

**Tool:** `observe`
**Mode:** `deployment_status`

The AI can also receive watchdog alerts passively through the observe tool, without polling the configure tool.

#### Request

```json
{
  "tool": "observe",
  "arguments": {
    "what": "deployment_status"
  }
}
```

#### Response

```json
{
  "active_watchdogs": [
    {
      "watchdog_id": "wd-1706400000-1",
      "deployment_label": "v2.3.0",
      "status": "alert",
      "elapsed_minutes": 3,
      "alerts_count": 1,
      "latest_alert_summary": "2 new console errors detected post-deployment",
      "verdict": "degraded"
    }
  ],
  "recent_completions": [
    {
      "watchdog_id": "wd-1706399000-1",
      "deployment_label": "v2.2.9",
      "status": "completed",
      "verdict": "healthy",
      "completed_at": "2026-01-28T09:45:00Z"
    }
  ]
}
```

Additionally, watchdog alerts are embedded in the `observe({what: "changes"})` response under a `deployment_alerts` key, following the same delivery pattern as push regression alerts. This means the AI receives deployment regression notifications automatically if it is already polling for changes.

## Watchdog Lifecycle

The watchdog session progresses through these states:

```
start called
    |
    v
[capturing_baseline] -- baseline snapshot taken
    |
    v
[monitoring] -- continuous evaluation against baseline
    |                               |
    v                               v
(regression detected)        (duration elapses)
    |                               |
    v                               v
[alert] -- alert fired,       [completed] -- soak passed,
    |      still monitoring        verdict: "healthy"
    |
    v
(AI calls stop or
 duration elapses)
    |
    v
[stopped] -- summary generated
```

State transitions:
- `capturing_baseline` -> `monitoring`: Automatic, once baseline snapshot completes (sub-second).
- `monitoring` -> `alert`: Automatic, when a regression check exceeds a threshold.
- `monitoring` -> `completed`: Automatic, when `duration_minutes` elapses with no alerts.
- `alert` -> `monitoring`: Automatic, if the regression self-resolves (the condition clears on the next check).
- `alert` -> `stopped`: When the AI explicitly calls `stop`.
- `alert` -> `completed`: When `duration_minutes` elapses (the alert was noted but not acted upon).
- Any state -> `stopped`: When the AI explicitly calls `stop`.

## Baseline Capture

When the watchdog starts, it captures a baseline using the same mechanism as `diff_sessions({session_action: "capture"})`. The baseline includes:

- **Console errors**: All current error messages and their fingerprints (so pre-existing errors are not flagged as regressions).
- **Network state**: All observed endpoints, their status codes, and latencies.
- **Performance timing**: Load time, LCP, FCP, CLS, TTFB from the most recent performance snapshot.
- **WebSocket connections**: Which connections are open and expected.

If the behavioral baselines feature is available (a named baseline exists for the URL scope), the watchdog also loads the behavioral baseline for richer comparison (API response shapes, timing tolerance factors, etc.).

The baseline is stored in memory for the duration of the watchdog session. It is not persisted to disk. If the server restarts during a watchdog session, the session is lost.

## Regression Checks

The watchdog evaluates incoming telemetry against the baseline at a regular interval. The check frequency is determined by the server (not configurable by the AI) to avoid overloading the comparison logic.

**Check interval:** Every 30 seconds during the monitoring window.

Each check evaluates four dimensions:

### 1. Error Rate

- Counts new console errors not present in the baseline (by fingerprint matching).
- Threshold: `max_new_errors` (default: 0). Any new error triggers an alert.
- Severity: `high` if the new error count exceeds 5 in a single check, `medium` otherwise.

### 2. Latency

- Compares current page load time, LCP, and TTFB against baseline values.
- Threshold: `latency_factor` (default: 1.5). Current value must exceed `baseline * latency_factor` to trigger.
- Severity: `high` if regression exceeds 3x baseline, `medium` if between 1.5x-3x.

### 3. Network Failures

- Tracks the rate of 4xx/5xx responses across all observed network requests.
- Threshold: `max_network_error_rate` (default: 0.05, i.e., 5%).
- Also flags specific endpoints that changed from success (2xx) to failure (4xx/5xx).
- Severity: `high` if a previously-healthy endpoint now returns 5xx, `medium` for 4xx.

### 4. Web Vitals

- Compares CLS, LCP, and FCP against baseline values.
- Threshold: `vitals_regression_pct` (default: 20%). A metric must regress by more than this percentage.
- CLS uses absolute comparison: regression if current CLS exceeds baseline CLS + 0.1.
- Severity: `medium` for all vitals regressions (they indicate UX degradation, not breakage).

## Thresholds and Defaults

All thresholds are configurable per watchdog session. Defaults are designed to be conservative (low tolerance for regressions):

| Threshold | Default | Description |
|-----------|---------|-------------|
| `max_new_errors` | 0 | Maximum new console errors before alerting |
| `latency_factor` | 1.5 | Load time must exceed baseline * factor to alert |
| `max_network_error_rate` | 0.05 | Max fraction of requests returning 4xx/5xx |
| `vitals_regression_pct` | 20 | Percentage regression threshold for web vitals |
| `duration_minutes` | 10 | How long to monitor (1-60 range) |

The AI can override any threshold at start time. Passing `null` for a threshold dimension disables that check entirely (e.g., `"max_new_errors": null` skips error monitoring).

## Alert Delivery

Alerts are delivered through two channels:

### 1. Active Polling

The AI calls `configure({action: "watchdog", watchdog_action: "status"})` to get the current watchdog state, including any pending alerts.

### 2. Passive Embedding

Watchdog alerts are embedded in the `observe({what: "changes"})` response under a `deployment_alerts` key. This follows the same pattern as push regression alerts: the alert appears once in the changes stream when first detected, and is not repeated on subsequent polls with a newer checkpoint.

```json
{
  "changes_since": "...",
  "deployment_alerts": [
    {
      "watchdog_id": "wd-1706400000-1",
      "deployment_label": "v2.3.0",
      "alert_id": "wa-1",
      "severity": "high",
      "type": "new_errors",
      "summary": "2 new console errors detected post-deployment",
      "recommendation": "Check observe({what: 'errors'}) for details. Consider rollback."
    }
  ]
}
```

This means the AI receives deployment alerts automatically if it is already watching for changes, which is the recommended agentic workflow pattern.

## Composition with Existing Features

### Behavioral Baselines

If the AI has previously saved a behavioral baseline for the URL scope (via `configure({action: "save_baseline"})`), the watchdog automatically loads it for richer comparison. Behavioral baselines provide:

- API response shape comparison (detect structural changes in JSON responses)
- Per-endpoint timing tolerances (instead of a global latency factor)
- WebSocket connection state expectations

The watchdog uses the behavioral baseline as an additional signal alongside its own snapshot baseline. Behavioral baseline regressions are reported with `type: "behavioral_regression"` in alerts.

### Push Regression

The existing push regression system (which fires alerts when performance snapshots regress from stored baselines) continues to operate independently during a watchdog session. If both the watchdog and push regression detect the same latency regression, the watchdog alert takes precedence (it has deployment context) and the push regression alert is suppressed to avoid duplication.

### Performance Budget

If performance budgets are configured via `configure({action: "health"})`, the watchdog respects those budgets as additional thresholds. A budget violation during a watchdog session is reported as a watchdog alert with `type: "budget_violation"`.

### Session Diffing

The watchdog's final summary includes a `diff_sessions`-style comparison between the baseline snapshot and the final state at stop/completion time. The AI can also manually call `diff_sessions({session_action: "compare", compare_a: "wd-baseline-v2.3.0", compare_b: "current"})` to get a detailed diff at any point during monitoring.

The watchdog automatically saves its baseline as a named snapshot (`wd-baseline-{label}`) in the session manager, so the AI can use `diff_sessions` to compare against it later.

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Start a watchdog session with a deployment label and optional thresholds | must |
| R2 | Capture a baseline snapshot at start time | must |
| R3 | Continuously evaluate incoming telemetry against the baseline at 30s intervals | must |
| R4 | Detect new console errors not present in the baseline | must |
| R5 | Detect latency regressions exceeding the configured threshold | must |
| R6 | Detect network failure rate increases exceeding the configured threshold | must |
| R7 | Detect web vitals regressions exceeding the configured threshold | should |
| R8 | Fire alerts with severity classification when a threshold is breached | must |
| R9 | Deliver alerts through the `observe({what: "changes"})` polling stream | must |
| R10 | Auto-stop after the configured duration elapses | must |
| R11 | Support manual stop with a reason | must |
| R12 | Produce a deployment summary at stop/completion | must |
| R13 | Save the baseline as a named session snapshot for later diffing | should |
| R14 | Support observing active watchdog status via `observe({what: "deployment_status"})` | should |
| R15 | Suppress duplicate push regression alerts during an active watchdog | should |
| R16 | Load behavioral baselines if available for the URL scope | could |
| R17 | Respect performance budget thresholds if configured | could |
| R18 | Support concurrent watchdog sessions (max 3) | could |

## Non-Goals

- This feature does NOT execute rollbacks. Kaboom is an observation layer. The AI decides whether to rollback and uses whatever deployment mechanism is available (CI/CD API, Git revert, etc.). Kaboom tells the AI what it sees; the AI decides what to do.

- This feature does NOT integrate with external monitoring systems (Datadog, Sentry, PagerDuty). It monitors what the browser extension captures. External integration is out of scope.

- This feature does NOT persist watchdog sessions across server restarts. Watchdog sessions are ephemeral. If the server restarts, active sessions are lost. This is acceptable because the AI can start a new session.

- This feature does NOT perform synthetic load testing or traffic generation. It passively monitors real browser telemetry. The developer (or AI via the `interact` tool) must navigate the application to generate telemetry.

- This feature does NOT support multi-tab watchdog sessions. It monitors the active tracked tab. Monitoring multiple deployment environments simultaneously (staging + production) would require multiple Kaboom server instances.

- Out of scope: authorization tiers for rollback decisions. The original agentic CI/CD spec proposed human-approval gates for production rollbacks. This is the responsibility of the rollback mechanism, not Kaboom's observation layer.

## Performance SLOs

| Metric | Target |
|--------|--------|
| Baseline capture time | < 50ms |
| Per-check evaluation time | < 5ms |
| Alert generation and storage | < 1ms |
| Memory per watchdog session | < 200KB |
| Maximum concurrent watchdogs | 3 |
| Status query response time | < 10ms |

The 30-second check interval ensures the watchdog never contributes to CPU contention even if 3 sessions are active simultaneously. Each check is a lightweight comparison of numeric values and string sets.

## Security Considerations

- **Baseline data**: The baseline snapshot contains the same data types as `diff_sessions` snapshots (error messages, URLs, performance numbers). No response bodies are captured. URLs may contain path parameters but query strings with sensitive values should already be stripped by the extension's privacy layer.

- **Deployment labels**: The `deployment_label` is a freeform string. It is stored in memory only and appears in MCP responses. It should not contain secrets.

- **No credential storage**: The watchdog does not store or require deployment API credentials. Rollback decisions are made by the AI and executed through external mechanisms.

- **Audit trail**: Watchdog start, stop, and alert events are recorded in the audit trail (`configure({action: "audit_log"})`) so there is a record of what the AI monitored and what it was told.

- **Snapshot redaction**: The baseline snapshot inherits whatever redaction rules are active on the server (e.g., header stripping, URL scrubbing). No additional redaction is needed.

## Edge Cases

- **No telemetry arrives during monitoring**: If the browser tab is closed or navigated away, no new telemetry arrives. The watchdog continues running but cannot detect regressions. When duration elapses, it reports `"verdict": "inconclusive"` with a note that insufficient data was received.

- **Extension disconnects during monitoring**: Same as above. The watchdog cannot detect regressions without telemetry. It reports inconclusive when it stops.

- **Server restarts during monitoring**: The watchdog session is lost (in-memory only). The AI can start a new session after the server restarts. The `observe({what: "deployment_status"})` call returns an empty list.

- **Multiple deployments overlap**: If the AI starts a second watchdog while one is active, both run independently. Each has its own baseline and thresholds. Alerts are tagged with the watchdog ID so the AI can distinguish them.

- **Baseline captured during error state**: If the baseline already has errors, those errors are recognized as pre-existing (by fingerprint) and not flagged as regressions. Only truly new errors trigger alerts.

- **Rapid consecutive deployments**: If the AI deploys, starts a watchdog, then deploys again within the soak period, the first watchdog's baseline becomes stale. The AI should stop the first watchdog and start a new one. The watchdog does not auto-detect re-deployments.

- **Regression self-resolves**: If a latency spike triggers an alert but subsequent checks show normal latency, the watchdog transitions back from `alert` to `monitoring`. The alert remains in the history for the final summary. The final verdict accounts for both the alert and the recovery.

- **All threshold dimensions disabled**: If the AI passes `null` for every threshold, the watchdog runs but never fires alerts. It still produces a summary comparing baseline to final state.

- **Duration of 0 or negative**: Rejected with an error. Minimum duration is 1 minute, maximum is 60 minutes.

- **Watchdog started but no deployment happens**: The watchdog does not know or care about the deployment mechanism. It just compares baseline to current state. If nothing changes, it reports `"verdict": "unchanged"` at completion.

## Dependencies

- **Depends on:**
  - `diff_sessions` (shipped) -- Snapshot capture and comparison primitives.
  - `observe({what: "changes"})` (shipped) -- Alert delivery through the changes polling stream.
  - Push regression alerts (shipped) -- Alert delivery pattern and suppression coordination.
  - Web vitals (shipped) -- CLS, LCP, FCP metrics.
  - Performance data capture (shipped) -- Load time, TTFB metrics.

- **Optionally composes with:**
  - Behavioral baselines (in-progress) -- Richer comparison with API shape and per-endpoint tolerances.
  - Performance budget (shipped) -- Budget thresholds as additional regression signals.

- **Depended on by:**
  - Agentic CI/CD (proposed) -- The deployment watchdog is a building block for fully autonomous CI/CD workflows.

## Assumptions

- A1: The extension is connected and tracking a tab before the watchdog is started.
- A2: The AI captures the baseline BEFORE triggering the deployment (the watchdog does not auto-detect deployments).
- A3: The browser tab remains open and navigable during the monitoring period (the watchdog passively observes; it does not drive the browser).
- A4: The server remains running for the full duration of the watchdog session.
- A5: At least one page load or navigation occurs after the deployment completes, generating fresh telemetry for comparison.
- A6: The Kaboom server's ring buffers are not full-flushed between baseline capture and regression checks (i.e., the monitoring period is short enough that telemetry is retained).

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Should the watchdog auto-detect deployments (e.g., by watching for a full page reload or a version change in a meta tag)? | open | Auto-detection would remove A2 but adds complexity and brittleness. The current design requires the AI to explicitly start the watchdog. |
| OI-2 | Should the watchdog persist its baseline to disk via `configure({action: "store"})` so it survives server restarts? | open | The review flagged this as critical for production use. However, the watchdog is a short-lived session (max 60 min) and the probability of a server restart during that window is low for a localhost dev tool. |
| OI-3 | Should the watchdog support a "re-baseline" action that captures a new baseline mid-session (e.g., after a config change that is expected to alter behavior)? | open | This could prevent false positives from intentional changes during the soak period. |
| OI-4 | What is the correct interaction model when the watchdog fires an alert and the AI wants to investigate? Should the AI stop the watchdog first, or can it investigate while monitoring continues? | open | Current design: monitoring continues while the AI investigates. The AI stops the watchdog only when it decides to act (rollback or accept). |
| OI-5 | Should watchdog completion summaries be automatically stored in persistent memory (`configure({action: "store"})`) so the AI can reference past deployments? | open | Useful for trend analysis ("last 5 deployments all had latency increases") but adds storage complexity. |
| OI-6 | How should the watchdog interact with tab targeting? If the tracked tab changes mid-session, should the watchdog follow the new tab or stick to the original? | open | Current assumption: the watchdog monitors whatever tab is active. If the AI re-targets, the watchdog monitors the new target. |
| OI-7 | Should the watchdog check interval (30s) be configurable? | open | Fixed interval is simpler. Configurable interval adds a parameter but is useful for short-duration watchdogs (1-2 min) where 30s is too coarse. |
