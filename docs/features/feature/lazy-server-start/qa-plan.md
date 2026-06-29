---
doc_type: qa-plan
feature_id: feature-lazy-server-start
status: proposed
scope: feature/lazy-server-start/qa
ai-priority: high
tags: [testing, qa, bridge, lifecycle]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-06-29
last_reviewed: 2026-06-29
---

# QA Plan: Lazy Server Start

> Verifies the lazy server start contracts: tab tracking works offline, tool calls auto-start the daemon, the daemon recovers from failure, and the extension reconnects. Covers data-leak analysis, agent clarity, simplicity, code-level tests, and step-by-step user acceptance testing (UAT).

---

## 1. Data Leak Analysis

**Goal:** Confirm the startup and recovery machinery never transmits data off the machine and never leaks process or path details beyond the local lock files.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Daemon binds only localhost | Verify the daemon listens on `127.0.0.1:<port>`, never `0.0.0.0` or a public interface | critical |
| DL-2 | Lock file contents | Verify the daemon lock record holds only process identifier, port, state directory, version, and timestamp — no telemetry or page content | high |
| DL-3 | Tracked-tab state path | Verify tracked-tab state stays in `chrome.storage.local` and syncs only to the local daemon over `/sync` | critical |
| DL-4 | Launch-mode warning contents | Verify the `launch_mode_warning` string contains only the mode, reason, and suggested command — no environment dump | medium |
| DL-5 | Structured error contents | Verify retryable errors expose a message and flag, not full stack traces or absolute home paths | medium |
| DL-6 | Process detection shell-out | Verify `detectParentProcessName` runs only a fixed `ps -p <ppid> -o comm=` and never interpolates untrusted input | high |
| DL-7 | No external network on cold start | Verify daemon spawn and reconnection generate no traffic beyond `127.0.0.1` | critical |

### Negative Tests (must NOT leak)
- [ ] Daemon never opens a non-loopback listener
- [ ] Lock file never contains captured telemetry
- [ ] Cold start produces zero external network requests
- [ ] Parent-process detection cannot execute attacker-controlled commands

---

## 2. Agent Clarity Assessment

**Goal:** Confirm an AI client can interpret startup, waiting, and recovery states without misreading them as hard failures.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Starting vs failed | A `"starting"` status during the grace period is distinct from a fatal failure | [ ] |
| CL-2 | Retryable flag semantics | `retryable: true` clearly signals the client may retry the same call | [ ] |
| CL-3 | Fast-path transparency | `initialize` and `tools/list` results are identical whether or not the daemon is up | [ ] |
| CL-4 | Extension-required error | The readiness-guard error explains that the extension must be connected | [ ] |
| CL-5 | Launch-mode warning | The warning names the persistent-start command so the agent can relay it | [ ] |
| CL-6 | Non-retryable fatal error | A port-occupied or version-recycle failure is clearly non-retryable | [ ] |

### Common Agent Misinterpretation Risks
- [ ] Agent treats a `"starting"` wait as a permanent failure and aborts the session
- [ ] Agent retries a non-retryable fatal error in a tight loop
- [ ] Agent assumes `tools/list` succeeding means `tools/call` will succeed immediately

---

## 3. Simplicity Assessment

**Goal:** Confirm the user never performs an explicit start step.

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Track a tab before any AI session | 1 step: click "Track This Tab" | No — works offline already |
| Start the daemon | 0 steps: first `tools/call` triggers it | No — fully implicit |
| Recover from a daemon crash | 0 steps: next call respawns it | No — automatic |
| Reconnect the extension | 0 steps: one-second sync reconnects | No — automatic |

### Default Behavior Verification
- [ ] No "start server" button exists or is required
- [ ] Offline popup shows amber "Offline", never a red blocking error
- [ ] Track button stays enabled regardless of daemon state

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Spawn when none running | `tryConnectToExisting` with an unused port | Returns false, state not ready | must |
| UT-2 | Skip spawn when running | `tryConnectToExisting` against a live compatible daemon | Returns true, state ready | must |
| UT-3 | Respawn resets failure | Mark failed, then reset signals | `failed` and `err` cleared | must |
| UT-4 | Status during spawn | `checkDaemonStatus` with `tools/call` during grace period | Returns `"starting"` | must |
| UT-5 | Status when ready | `checkDaemonStatus` after `markReady` | Returns empty string | must |
| UT-6 | Fast-path initialize | `initialize` request | Handled without daemon (classified method-not-found at status layer) | must |
| UT-7 | Fast-path tools/list | `tools/list` request | `handleFastPath` returns handled with the tool list | must |
| UT-8 | tools/call waits | `tools/call` during `"starting"` | Forwarded (waited on), not instant-errored | must |
| UT-9 | Retryable on 5xx | Forwarded response status >= 500 | Structured error with `retryable: true` | must |
| UT-10 | Launch mode: daemon flag | `classifyLaunchMode` with `daemonMode` true | `persistent`, reason `daemon_flag_enabled` | must |
| UT-11 | Launch mode: supervisor | Supervisor env var set | `persistent`, reason `supervisor_detected` | must |
| UT-12 | Launch mode: interactive shell | TTY with `zsh` parent | `likely_transient` | must |
| UT-13 | Require persistent enforced | `KABOOM_REQUIRE_PERSISTENT` + transient | `enforcePersistentMode` returns an error | should |
| UT-14 | Extension readiness timeout | `ExtensionReadinessTimeout` value | Greater than zero (allows cold-start reconnect) | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Cold-start round trip | Bridge -> spawn -> daemon -> `tools/call` | Call succeeds after the daemon boots | must |
| IT-2 | Concurrent bridges | Two bridges, no daemon | Exactly one daemon spawned; one leader, one follower | must |
| IT-3 | Stale lock reclaim | Leader stalls, lock ages past stale threshold | Follower reclaims the lock and spawns | must |
| IT-4 | Daemon crash recovery | Kill daemon mid-session, issue a call | `respawnIfNeeded` re-launches and the call retries | must |
| IT-5 | Default takeover | Two daemons contend for the same state directory | Lock-to-PID-file match required before takeover | must |
| IT-6 | Non-Kaboom port occupant | Foreign listener on the port | Fatal, non-retryable error; foreign process untouched | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Fast-path latency | `initialize` / `tools/list` response time | < 50ms | must |
| PT-2 | tools/call cold wait | Time to first result | <= grace period (~2s) | must |
| PT-3 | Extension reconnect | Time from daemon start to reconnect | <= 1s | should |

### 4.4 Edge Case Tests

| # | Edge Case | Scenario | Expected Behavior | Priority |
|---|-----------|----------|-------------------|----------|
| EC-1 | Incompatible version | Old daemon on port | Stopped for upgrade, new daemon spawned | must |
| EC-2 | Version recycle fails | Old daemon cannot be recycled | Fatal, non-retryable error | should |
| EC-3 | Lock/PID mismatch, port serving | Mismatched lock with live port | Refuse takeover | must |
| EC-4 | Lock/PID mismatch, port idle | Mismatched lock, dead port | Reclaim lock | must |
| EC-5 | Notification with no id | JSON-RPC notification on stdio | Dropped, no response | should |

---

## 5. UAT Checklist (Human + AI)

> The AI executes MCP calls; the human watches the popup and process state.

### Prerequisites
- [ ] Chrome extension installed
- [ ] No Kaboom daemon currently running (`pkill` or fresh shell)
- [ ] An MCP client (Claude Code or similar) configured to launch Kaboom in bridge mode

### Step-by-Step Verification

| # | Step | Human Observes | Expected Result | Pass |
|---|------|----------------|-----------------|------|
| UAT-1 | Open the popup with no daemon running | Popup status | Amber "Offline", Track button enabled | [ ] |
| UAT-2 | Click "Track This Tab" while offline | Track toggles on | Tracking state saved locally; no error | [ ] |
| UAT-3 | Run the first MCP `tools/list` | Client receives the tool list | Instant response; daemon not yet required | [ ] |
| UAT-4 | Run the first `tools/call` (e.g. observe) | Client waits briefly, then result | Daemon auto-starts; call succeeds | [ ] |
| UAT-5 | Re-open the popup after the daemon starts | Popup status | "Connected", green; tracked tab synced | [ ] |
| UAT-6 | Kill the daemon, run another `tools/call` | Brief wait, then result | Daemon respawns; call retried and succeeds | [ ] |
| UAT-7 | Launch the bridge from an interactive shell without `--daemon` | Warning emitted | `launch_mode_warning` suggests a persistent start | [ ] |
| UAT-8 | Set `KABOOM_REQUIRE_PERSISTENT` and relaunch transiently | Hard error | Launch rejected with an actionable message | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Localhost-only listener | `lsof`/`netstat` on the daemon | Bound to `127.0.0.1` only | [ ] |
| DL-UAT-2 | No external traffic on cold start | Monitor network during spawn | Only loopback traffic | [ ] |
| DL-UAT-3 | Lock file contents | Inspect the daemon lock file | Process id, port, state dir, version only | [ ] |

### Regression Checks
- [ ] Existing MCP tools still function once the daemon is up
- [ ] Popup recording controls still work offline
- [ ] A persistently started daemon (`--daemon`) emits no transient warning

---

## Sign-Off

| Area | Tester | Date | Pass/Fail |
|------|--------|------|-----------|
| Data Leak Analysis | | | |
| Agent Clarity | | | |
| Simplicity | | | |
| Code Tests | | | |
| UAT | | | |
| **Overall** | | | |
