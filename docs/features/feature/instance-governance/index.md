---
doc_type: feature_index
feature_id: feature-instance-governance
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-08-30
code_paths:
  - internal/proclock/proclock.go
  - internal/proclock/proclock_unix.go
  - internal/proclock/proclock_windows.go
  - internal/procidentity/procidentity.go
  - internal/procidentity/procidentity_unix.go
  - internal/procidentity/procidentity_windows.go
  - internal/instancereg/registry.go
  - internal/instancereg/prune.go
  - internal/instancegov/instancegov.go
  - internal/instancegov/policy.go
  - internal/instancegov/cpu.go
  - internal/instancegov/runtime.go
  - internal/idlewatch/idlewatch.go
  - internal/procwatch/procwatch.go
  - internal/retention/retention.go
  - internal/reaper/reaper.go
  - internal/reaper/census.go
  - internal/reaper/staledirs.go
  - internal/semver/semver.go
  - internal/state/paths.go
  - cmd/browser-agent/daemon_governance.go
  - cmd/browser-agent/retention_policy.go
  - cmd/browser-agent/census_command.go
  - cmd/browser-agent/config.go
  - cmd/browser-agent/main_connection_mcp.go
  - cmd/browser-agent/internal/bridge/bridge_governance.go
  - cmd/browser-agent/internal/bridge/bridge_startup.go
  - cmd/browser-agent/internal/procctl/terminate.go
  - cmd/browser-agent/internal/runtimeflags/flags.go
  - cmd/browser-agent/internal/startupconfig/help.go
test_paths:
  - internal/proclock/proclock_test.go
  - internal/procidentity/procidentity_test.go
  - internal/instancereg/registry_test.go
  - internal/instancegov/policy_test.go
  - internal/instancegov/instancegov_test.go
  - internal/idlewatch/idlewatch_test.go
  - internal/procwatch/procwatch_test.go
  - internal/retention/retention_test.go
  - internal/reaper/reaper_test.go
  - internal/reaper/census_test.go
  - internal/reaper/staledirs_test.go
  - internal/semver/semver_test.go
  - cmd/browser-agent/daemon_governance_test.go
  - cmd/browser-agent/retention_policy_test.go
  - cmd/browser-agent/internal/bridge/bridge_governance_test.go
  - cmd/browser-agent/internal/integrationtest/instance_cap_test.go
---

# Instance Governance

Bounds how many Kaboom processes, ports, and bytes a single machine may hold.

## The defect this replaces

Every previous guard was scoped to a **state directory**. `--parallel`,
`--state-dir`, git worktrees, and CI each got a private universe in which
"one daemon" was trivially true and the machine-wide count was unbounded.

Measured on one developer machine before this feature:

| Symptom | Measurement |
| --- | --- |
| Abandoned `--parallel` state dirs | 880 (29MB, oldest 32 days) |
| Test residue in the real state root | 166,824 dirs / 191,923 files / 750MB |
| Total state directory | 1.0 GB |
| Captured artifacts | 5,975 screenshots, 5,317 recordings |
| Stale locks on ports outside the cleanup range | 7 (ports 7904, 19023–19310) |
| Lock records naming recycled PIDs | 3 (TextInputSwitcher, 2× Adobe Creative Cloud) |
| Daemon uptime with no client attached | 2 days 13 hours |
| Bridges alive after their session | 2, at 31 hours and ~24MB each |

## Design

1. **Kernel-held singleton** (`internal/proclock`). `flock`/`LockFileEx` held for
   the process lifetime. Mutual exclusion is guaranteed by the kernel and released
   on any death including SIGKILL and OOM, so there are no stale-lock heuristics,
   no grace windows, and no read-decide-write race.
2. **Machine-wide registry** (`internal/instancereg`). Its location ignores
   `KABOOM_STATE_DIR`: isolation isolates *data*, never *accounting*. Under
   `go test` an explicit `KABOOM_REGISTRY_DIR` is required, so a test can never
   write into a developer's home — the defect that produced the 750MB above.
3. **Identity-checked liveness** (`internal/procidentity`). Liveness compares pid
   **plus start time plus command**, because `kill(pid, 0)` answers "does some
   process hold this pid", not "is my process still running".
4. **Admission with caps** (`instancegov.Admit`, `instancegov.Surplus`). One
   production daemon per machine; test daemons capped at `min(4, max(2, cores/4))`,
   evicting oldest-first. Admission and the reaper call ONE cap function, differing
   only in whether a candidate is joining (`incoming` 1 or 0), and ONE wedged
   predicate (`instancegov.IsWedged`) — those pairs had already drifted apart once,
   disagreeing about a record whose heartbeat could not be parsed.
5. **Idle exit** (`internal/idlewatch`). A daemon serving nobody releases its
   ports. Any unreadable work signal counts as busy, so it never exits mid-recording.
6. **Bridge parent-death** (`internal/procwatch`). stdin EOF covers a clean client
   exit; this covers a client that was killed and never closed the pipe.
7. **Reaper and census** (`internal/reaper`, `--instances`, `--reap`).
8. **Retention budgets** (`internal/retention`). Count, bytes, and age per
   directory, applied hourly with single-pass eviction.
9. **Cross-process proof** (`instance_cap_test.go`). Eight daemons launch
   simultaneously; exactly one binds and seven exit 0.

## What this replaced

Singleton admission previously lived in `cmd/browser-agent/internal/daemonlife` as a
per-state-directory lock file with PID-based liveness, a startup grace window, and
its own takeover policy. That package shrank from 2,369 to 1,116 lines when the
kernel lock took over: `lock_file.go`, `EnforceStartupPolicy`, `PersistCurrentLock`,
`RemoveLockIfOwned`, `Deferral`, and their tests were deleted rather than left
standing beside the new authority. Its `Deps` went from thirteen seams to five —
every process and port primitive is gone, including the pid-only `IsProcessAlive`
that could signal a process whose pid had been recycled. daemonlife now owns one
thing: crash-loop restart throttling.

The version-upgrade takeover and the equal-version install-epoch tiebreaker were
migrated into `instancegov.shouldRequestHandoff`, so an upgrade still supersedes an
incumbent — by asking it to stand down and then taking the freed lock, never by
racing it.

## Operating it

```bash
kaboom --instances          # every daemon and bridge on this machine
kaboom --reap --dry-run     # what would be reclaimed
kaboom --reap               # reclaim it
```

`--reap` never terminates a healthy, in-cap daemon. Over-cap **production**
daemons are handled at admission (the late arrival defers), never by the reaper,
so a starting daemon can never kill the one a developer is using.
