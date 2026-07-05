---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Dynamic Binary Upgrade

## Problem

When users install a new kaboom binary (npm, pip, or manual copy), the old daemon keeps running because the installer didn't kill it or the user doesn't know how to restart. The existing version mismatch recovery only triggers when a **new bridge process** connects. If no new bridge connects, the old daemon runs indefinitely.

## User Stories

1. **As a user upgrading kaboom-mcp**, I want the daemon to detect the new binary on disk and auto-restart, so I always get the latest version without manual intervention.

2. **As a user**, I want to see a notice in tool responses when an upgrade is detected, so I know a restart is imminent.

3. **As a user**, I want to see confirmation after an upgrade completes, so I know the new version is active.

## Requirements

### Functional

1. **Binary Watching**
   - Daemon polls its own executable path every 30 seconds via `os.Stat()`
   - Compares modtime and file size against cached values
   - Only proceeds to version verification on change detection

2. **Version Verification**
   - Runs `<binary> --version` with 5-second timeout
   - Parses output matching "kaboom v0.8.1" or bare "0.8.1"
   - Only triggers upgrade for strictly newer semver (not same, not older)

3. **Graceful Shutdown**
   - After detecting a newer version, waits 5 seconds (grace period)
   - Sends SIGTERM to self, reusing the existing shutdown path
   - Bridge's recovery logic respawns from `os.Executable()` (now pointing to new binary)

4. **Upgrade Notification**
   - Tool responses include a NOTICE when upgrade is pending
   - After restart, first tool responses include "Upgraded from vX to vY"
   - Health endpoint reports `upgrade_pending` when detected

5. **Upgrade Marker**
   - Before shutdown, writes `~/.kaboom/run/last-upgrade.json`
   - New daemon reads and clears the marker on startup
   - Marker contains `from_version`, `to_version`, `timestamp`

### Non-Functional

- Poll interval: 30 seconds (not configurable to prevent abuse)
- Version check timeout: 5 seconds
- Grace period: 5 seconds
- Zero additional dependencies
- Thread-safe state access

## Configuration

- `KABOOM_NO_AUTO_UPGRADE=1` disables the watcher entirely

## Edge Cases

1. **Binary deleted**: Stat fails silently, watcher continues polling
2. **Binary replaced with older version**: Detected but not treated as upgrade
3. **Binary replaced with same version**: No action taken
4. **--version times out**: Treated as verification failure, no upgrade
5. **--version returns garbage**: Parsed as invalid, no upgrade
6. **Marker file corrupt**: Silently discarded on read
7. **Multiple rapid replacements**: Only first detection triggers restart
8. **Context cancelled during grace period**: Clean goroutine exit

## Design: Graceful Shutdown, Not syscall.Exec

The daemon detects the upgrade, notifies via tool response piggyback, then sends itself SIGTERM. The bridge's existing recovery logic detects the connection loss and respawns from `os.Executable()` (which now points to the new binary). This reuses the well-tested shutdown path and avoids the complexity of in-process replacement.
