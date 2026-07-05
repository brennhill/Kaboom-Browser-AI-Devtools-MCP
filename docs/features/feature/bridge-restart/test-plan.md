---
doc_type: test_plan
feature_id: feature-bridge-restart
status: implemented
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Bridge Restart — Test Plan

**Status:** [x] Product Tests Defined | [x] Tech Tests Designed | [x] Tests Generated | [x] All Tests Passing

---

## Product Tests

### Valid State Tests

- **Test:** Happy path restart with responsive daemon
  - **Given:** Daemon is running and healthy on port
  - **When:** LLM calls `configure(action="restart")`
  - **Then:** Daemon PID changes, response has `restarted: true, status: ok`

- **Test:** Restart with hung (frozen) daemon
  - **Given:** Daemon is frozen via SIGSTOP (simulating deadlock)
  - **When:** LLM calls `configure(action="restart")`
  - **Then:** Frozen daemon is killed, fresh daemon spawns, PID changes

- **Test:** Daemon-side restart when responsive
  - **Given:** Daemon is healthy and processes requests normally
  - **When:** Request reaches daemon (not intercepted by bridge)
  - **Then:** Daemon sends self-SIGTERM, bridge auto-respawns

### Edge Case Tests

- **Test:** extractToolAction with non-configure tool
  - **Given:** A tools/call request for observe, interact, analyze, or generate
  - **When:** extractToolAction parses the request
  - **Then:** Returns the tool name but empty action string

- **Test:** extractToolAction with malformed JSON
  - **Given:** Various malformed inputs (nil, empty, invalid JSON, missing fields)
  - **When:** extractToolAction parses the request
  - **Then:** Returns empty strings without panicking

- **Test:** extractToolAction with non-tools/call method
  - **Given:** An initialize or ping request
  - **When:** extractToolAction parses the request
  - **Then:** Returns empty strings immediately

- **Test:** Configure without action field
  - **Given:** configure call with other params but no action
  - **When:** extractToolAction parses the request
  - **Then:** Returns tool="configure", action="" (does not trigger restart)

---

## Technical Tests

### Unit Tests

**Test File:** `cmd/browser-agent/bridge_test.go`

| Test | Status |
|------|--------|
| `TestExtractToolAction_ConfigureRestart` | Passing |
| `TestExtractToolAction_ConfigureHealth` | Passing |
| `TestExtractToolAction_NonConfigure` | Passing |
| `TestExtractToolAction_NonToolsCall` | Passing |
| `TestExtractToolAction_MalformedJSON` | Passing |
| `TestExtractToolAction_ConfigureNoAction` | Passing |

### Startup Convergence Tests

| Test | Status |
|------|--------|
| `TestBridgeStartupLock_SingleLeaderElection` | Passing |
| `TestWaitForPeerDaemon_ServerAppearsOnFirstRetry` | Passing |
| `TestBridgeStartupContention_AllClientsConverge` | Passing |
| `TestFastStart_ToolsCallWaitsForDaemonBoot` | Passing |

### Manual Tests

1. **Happy path:** Start kaboom, call `configure(action="restart")`, verify daemon PID changes
2. **Hung daemon:** `kill -STOP <daemon_pid>`, call restart, verify recovery with new PID
