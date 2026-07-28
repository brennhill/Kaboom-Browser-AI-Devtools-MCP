---
feature: deployment-watchdog
status: proposed
doc_type: tech-spec
feature_id: feature-deployment-watchdog
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Deployment Watchdog

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Deployment Watchdog is a **time-bounded monitoring session** implemented as a new action under the existing `configure` MCP tool and a new mode under the `observe` tool. It captures a pre-deployment baseline, then continuously compares incoming telemetry against that baseline for a configurable duration (1-60 minutes). If regressions are detected, alerts are surfaced via the existing `observe({what: "changes"})` polling mechanism.

The watchdog is server-side state (in-memory session) with no extension changes required. It composes three existing Kaboom primitives: session snapshots (`diff_sessions`), behavioral baselines (`save_baseline`/`compare_baseline`), and push regression alerts (`observe({what: "changes"})`).

## Key Components

**Watchdog Manager**: Server-side component that maintains active watchdog sessions. Each session is an in-memory struct containing: deployment label, baseline snapshot, start time, duration, thresholds, current status (capturing_baseline, monitoring, alert, completed, stopped), alerts fired, check count, and last check time.

**Baseline Capture**: When watchdog starts, captures current browser state using same mechanism as `diff_sessions({session_action: "capture"})`. Baseline includes: console error fingerprints (so pre-existing errors not flagged as regressions), network endpoint status codes and latencies, performance metrics (load time, LCP, FCP, CLS, TTFB), WebSocket connection state.

If a named behavioral baseline exists for the URL scope, watchdog also loads it for richer comparison (API response shapes, per-endpoint timing tolerances, WebSocket state expectations).

**Regression Checker**: Background goroutine that evaluates incoming telemetry against baseline every 30 seconds during monitoring window. Checks four dimensions:

1. **Error rate**: Counts new console errors not present in baseline (by fingerprint matching). Threshold: `max_new_errors` (default: 0).
2. **Latency**: Compares current page load time, LCP, TTFB against baseline values. Threshold: `latency_factor` (default: 1.5x).
3. **Network failures**: Tracks rate of 4xx/5xx responses. Threshold: `max_network_error_rate` (default: 5%). Also flags specific endpoints that changed from success to failure.
4. **Web Vitals**: Compares CLS, LCP, FCP against baseline. Threshold: `vitals_regression_pct` (default: 20% regression).

**Alert Generator**: When a threshold is breached, generates alert struct containing: alert ID, severity (high/medium), type (new_errors, latency_regression, network_failure, vitals_regression), detected timestamp, summary, details, and recommendation.

**Alert Delivery**: Alerts delivered through two channels:
1. **Active polling**: Agent calls `configure({what: "watchdog", watchdog_action: "status"})` to get current state including pending alerts.
2. **Passive embedding**: Alerts embedded in `observe({what: "changes"})` response under `deployment_alerts` key. Alert appears once when first detected, not repeated on subsequent polls (same pattern as push regression alerts).

## Data Flows

```
AI calls configure({what: "watchdog", watchdog_action: "start", deployment_label: "v2.3.0", duration_minutes: 15, thresholds: {...}})
  |
  v
Watchdog Manager creates new session
  -> Generates watchdog_id (e.g., "wd-1706400000-1")
  -> Sets status: "capturing_baseline"
  |
  v
Baseline Capture executes
  -> Captures current console errors, network state, performance metrics, WebSocket state
  -> Saves as session snapshot with name "wd-baseline-{label}"
  -> Optionally loads behavioral baseline if available
  -> Sets status: "monitoring"
  -> Records monitoring_until timestamp
  |
  v
Regression Checker goroutine starts
  -> Every 30 seconds, evaluates telemetry against baseline
  -> Checks: new errors, latency regression, network failure rate, vitals regression
  -> If threshold breached, generates alert and sets status: "alert"
  -> Alerts stored in session.alerts array
  |
  v
AI polls deployment status
  -> configure({what: "watchdog", watchdog_action: "status"})
  OR
  -> observe({what: "deployment_status"})
  OR
  -> observe({what: "changes"}) [includes deployment_alerts]
  |
  v
On alert detection, AI investigates
  -> observe({what: "errors"}) for full error context
  -> observe({what: "performance"}) for detailed metrics
  -> Decides: rollback or accept
  |
  v
AI stops watchdog (or duration elapses)
  -> configure({what: "watchdog", watchdog_action: "stop", reason: "rollback_triggered"})
  -> Watchdog Manager generates completion summary
  -> Session removed from active list, added to recent_completions (kept for 1 hour)
```

## Implementation Strategy

**Server changes** (files affected):

`cmd/browser-agent/types.go`:
- Add `WatchdogSession` struct containing session state
- Add `WatchdogAlert` struct containing alert details
- Add `WatchdogThresholds` struct containing configurable thresholds

`cmd/browser-agent/configure.go`:
- Add handler for `action: "watchdog"` with sub-actions: `start`, `status`, `stop`
- Start creates session, captures baseline, spawns checker goroutine
- Status returns current session state with alerts
- Stop terminates session, generates summary

`cmd/browser-agent/observe.go`:
- Add handler for `what: "deployment_status"` returning active watchdogs and recent completions
- Modify `what: "changes"` to include `deployment_alerts` field from active watchdogs

`cmd/browser-agent/watchdog.go` (new file):
- `WatchdogManager` struct with mutex-protected map of active sessions
- `captureBaseline()` function: captures current telemetry snapshot
- `regressionChecker()` goroutine: evaluates telemetry every 30s
- `checkErrorRate()`, `checkLatency()`, `checkNetworkFailures()`, `checkWebVitals()` functions
- `generateAlert()` function: creates alert struct with severity and recommendations
- `generateCompletionSummary()` function: produces summary when watchdog stops

**Extension changes**: None. Watchdog consumes existing telemetry buffers (logs, network, performance) without changes to extension.

**Trade-offs**:
- In-memory session state means watchdog sessions lost on server restart (acceptable for short-lived sessions, max 60 min).
- 30-second check interval balances responsiveness with CPU overhead (lower interval increases CPU, higher interval delays alert detection).
- Maximum 3 concurrent watchdogs to prevent memory exhaustion (each session ~200KB).

## Edge Cases & Assumptions

### Edge Cases

- **No telemetry arrives during monitoring**: Browser tab closed or navigated away. Watchdog continues running but cannot detect regressions. When duration elapses, reports `verdict: "inconclusive"` with note "insufficient data received."

- **Extension disconnects during monitoring**: Same as above. Watchdog cannot detect regressions without telemetry. Reports inconclusive when stops.

- **Server restarts during monitoring**: Watchdog session lost (in-memory only). `observe({what: "deployment_status"})` returns empty list. AI can start new session after restart.

- **Multiple deployments overlap**: AI starts second watchdog while one active. Both run independently with separate baselines and thresholds. Alerts tagged with watchdog_id so AI can distinguish.

- **Baseline captured during error state**: Baseline already has errors. Those errors recognized as pre-existing (by fingerprint) and not flagged as regressions. Only truly new errors trigger alerts.

- **Rapid consecutive deployments**: AI deploys, starts watchdog, then deploys again within soak period. First watchdog's baseline becomes stale. AI should stop first watchdog and start new one. Watchdog does not auto-detect re-deployments.

- **Regression self-resolves**: Latency spike triggers alert but subsequent checks show normal latency. Watchdog transitions from `alert` back to `monitoring`. Alert remains in history for final summary. Final verdict accounts for both alert and recovery.

- **All threshold dimensions disabled**: AI passes `null` for every threshold. Watchdog runs but never fires alerts. Still produces summary comparing baseline to final state.

- **Duration of 0 or negative**: Rejected with error. Minimum duration is 1 minute, maximum is 60 minutes.

- **Watchdog started but no deployment happens**: Watchdog doesn't know or care about deployment mechanism. Just compares baseline to current state. If nothing changes, reports `verdict: "unchanged"` at completion.

### Assumptions

- A1: Extension connected and tracking tab before watchdog started.
- A2: AI captures baseline BEFORE triggering deployment (watchdog does not auto-detect deployments).
- A3: Browser tab remains open and navigable during monitoring period (watchdog passively observes; does not drive browser).
- A4: Server remains running for full duration of watchdog session.
- A5: At least one page load or navigation occurs after deployment completes, generating fresh telemetry for comparison.
- A6: Ring buffers not full-flushed between baseline capture and regression checks (monitoring period short enough that telemetry retained).

## Risks & Mitigations

### Risk 1: False positive from expected changes
- **Description**: Deployment intentionally changes behavior (new feature adds API calls), watchdog flags as regression.
- **Mitigation**: AI sets appropriate thresholds for expected changes. For example, if new feature adds network calls, increase `max_network_error_rate` threshold or disable network checks (`null`). Threshold configuration is per-session.

### Risk 2: Session persists after server restart
- **Description**: Watchdog session lost on restart, AI doesn't know monitoring stopped.
- **Mitigation**: `observe({what: "deployment_status"})` returns empty list when no sessions exist. AI detects absence and can restart session. Documenting that sessions are ephemeral sets correct expectations.

### Risk 3: Memory exhaustion from many sessions
- **Description**: Many concurrent watchdogs consume excessive memory.
- **Mitigation**: Hard cap of 3 concurrent watchdogs. Fourth start attempt rejected with error. Each session ~200KB, so max 600KB total.

### Risk 4: Baseline staleness not detected
- **Description**: AI starts watchdog with baseline captured hours ago, false positives from drift.
- **Mitigation**: Watchdog records baseline capture timestamp. Status response includes baseline age. AI can check age and decide whether to recapture before starting.

### Risk 5: Alert spam from transient issues
- **Description**: Network hiccup causes temporary latency spike, fires alert, resolves immediately.
- **Mitigation**: Regression checker requires condition to persist across 2 consecutive checks (60 seconds) before firing alert. Single-check spikes ignored. This reduces noise from transient conditions.

## Dependencies

### Depends on:
- `diff_sessions` (shipped): Snapshot capture and comparison primitives.
- `observe({what: "changes"})` (shipped): Alert delivery through changes polling stream.
- Push regression alerts (shipped): Alert delivery pattern and suppression coordination.
- Web vitals (shipped): CLS, LCP, FCP metrics.
- Performance data capture (shipped): Load time, TTFB metrics.

### Optionally composes with:
- Behavioral baselines (in-progress): Richer comparison with API shape and per-endpoint tolerances.
- Performance budget (shipped): Budget thresholds as additional regression signals.

### Depended on by:
- Agentic CI/CD (proposed): Deployment watchdog is building block for fully autonomous CI/CD workflows.

## Performance Considerations

| Metric | Target | Implementation notes |
|--------|--------|---------------------|
| Baseline capture time | < 50ms | Reads current telemetry buffers, no network calls |
| Per-check evaluation time | < 5ms | Lightweight comparison of numeric values and string sets |
| Alert generation and storage | < 1ms | Struct allocation and append to alerts array |
| Memory per watchdog session | < 200KB | Baseline snapshot + alerts array + session metadata |
| Maximum concurrent watchdogs | 3 | Hard limit to prevent memory exhaustion |
| Status query response time | < 10ms | Read from in-memory session map |

30-second check interval ensures watchdog never contributes to CPU contention even if 3 sessions active simultaneously.

## Security Considerations

**Baseline data**: Baseline snapshot contains same data types as `diff_sessions` snapshots (error messages, URLs, performance numbers). No response bodies captured. URLs may contain path parameters but query strings with sensitive values already stripped by extension's privacy layer.

**Deployment labels**: `deployment_label` is freeform string. Stored in memory only, appears in MCP responses. Should not contain secrets.

**No credential storage**: Watchdog does not store or require deployment API credentials. Rollback decisions made by AI and executed through external mechanisms.

**Audit trail**: Watchdog start, stop, and alert events recorded in audit trail (`configure({what: "audit_log"})`) so there's record of what AI monitored and what it was told.

**Snapshot redaction**: Baseline snapshot inherits whatever redaction rules active on server (header stripping, URL scrubbing). No additional redaction needed.
