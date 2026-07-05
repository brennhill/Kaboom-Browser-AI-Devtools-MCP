---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Dynamic Binary Upgrade -- Test Plan

**Status:** [x] Product Tests Defined | [x] Tech Tests Designed | [x] Tests Generated | [ ] All Tests Passing

---

## Product Tests

### Valid State Tests

- **Test:** Daemon detects a newer binary on disk
  - **Given:** Daemon running v7.8.0
  - **When:** Binary is replaced with v0.8.1
  - **Then:** Watcher detects file change, verifies version, sets upgrade_pending

- **Test:** Daemon auto-restarts after grace period
  - **Given:** Upgrade detected (upgrade_pending = true)
  - **When:** 5-second grace period elapses
  - **Then:** Daemon sends SIGTERM to self, bridge respawns with new binary

- **Test:** Tool responses include upgrade notice
  - **Given:** Upgrade detected, daemon still running
  - **When:** Any tool call is made
  - **Then:** Response contains NOTICE with new version info

- **Test:** New daemon reports completed upgrade
  - **Given:** Daemon was auto-restarted due to upgrade
  - **When:** New daemon starts and reads upgrade marker
  - **Then:** First tool call includes "Upgraded from vX to vY" warning

- **Test:** Health endpoint reports upgrade state
  - **Given:** Upgrade detected
  - **When:** `/health` or `get_health` is called
  - **Then:** Response includes `upgrade_pending` with version info

### Edge Case Tests (Negative)

- **Test:** Binary replaced with older version
  - **Given:** Daemon running v7.8.0
  - **When:** Binary is replaced with v0.7.4
  - **Then:** No upgrade triggered

- **Test:** Binary replaced with same version
  - **Given:** Daemon running v7.8.0
  - **When:** Binary is replaced with v7.8.0 (same content or rebuilt)
  - **Then:** No upgrade triggered

- **Test:** Binary deleted
  - **Given:** Daemon running normally
  - **When:** Binary file is deleted from disk
  - **Then:** Stat error logged, watcher continues polling

- **Test:** --version times out
  - **Given:** Binary changed on disk
  - **When:** New binary hangs on --version
  - **Then:** Verification fails, no upgrade triggered

- **Test:** --version returns garbage
  - **Given:** Binary changed on disk
  - **When:** New binary outputs "not a version"
  - **Then:** Parse fails, no upgrade triggered

- **Test:** Feature disabled via env var
  - **Given:** `KABOOM_NO_AUTO_UPGRADE=1` set
  - **When:** Daemon starts
  - **Then:** Binary watcher is not started (returns nil)

### Concurrent/Race Condition Tests

- **Test:** Multiple goroutines reading upgrade state
  - **Given:** 10 concurrent goroutines
  - **When:** All call `UpgradeInfo()` simultaneously
  - **Then:** No data races (mutex-protected)

### Failure & Recovery Tests

- **Test:** Corrupt upgrade marker file
  - **Given:** Marker file contains invalid JSON
  - **When:** New daemon reads marker on startup
  - **Then:** Marker silently discarded, file cleaned up

- **Test:** Missing upgrade marker file
  - **Given:** No marker file exists
  - **When:** `readAndClearUpgradeMarker` called
  - **Then:** Returns nil, no error

---

## Technical Tests

### Unit Tests

#### Coverage Areas:
- `parseVersionParts()`: valid semver, v-prefix, malformed, empty
- `isNewerVersion()`: newer/older/same/prefix/empty/malformed
- `BinaryWatcherState.binaryChanged()`: modtime+size change, no change, missing file
- `verifyBinaryVersion()`: valid output, invalid output, timeout
- `checkForUpgrade()`: newer/older/same
- `writeUpgradeMarker()` / `readAndClearUpgradeMarker()`: round-trip, invalid JSON, missing file
- `maybeAddUpgradeWarning()`: with/without pending upgrade
- `buildUpgradeInfo()`: with/without pending upgrade
- `UpgradeMarkerFile()`: path correctness

**Test Files:**
- `cmd/browser-agent/version_compare_test.go`
- `cmd/browser-agent/binary_watcher_test.go`
- `cmd/browser-agent/handler_unit_test.go`
- `internal/state/paths_coverage_test.go`

### Integration Tests

#### Scenarios:
- Build two binaries with different versions via ldflags
- Start old daemon, replace binary with new
- Verify daemon restarts within ~35s (30s poll + 5s grace)

### Manual Testing

#### Steps:
1. `make dev` to start daemon
2. `go build -ldflags "-X main.version=99.0.0" -o $(which kaboom-mcp) ./cmd/browser-agent/`
3. Wait ~35 seconds
4. Verify daemon restarted via `curl localhost:9160/health`
5. Verify upgrade warning in next tool call

---

## Test Status

| Test Type | File | Status | Notes |
|-----------|------|--------|-------|
| Unit (version) | `cmd/browser-agent/version_compare_test.go` | Passing | 3 test functions |
| Unit (watcher) | `cmd/browser-agent/binary_watcher_test.go` | Passing | 14 test functions |
| Unit (handler) | `cmd/browser-agent/handler_unit_test.go` | Passing | 2 upgrade test functions |
| Unit (paths) | `internal/state/paths_coverage_test.go` | Passing | 2 test functions |
| Integration | Manual | Pending | Requires two-binary setup |
| Manual | N/A | Pending | Requires running daemon |

**Overall:** All unit tests pass. Integration and manual tests pending.
