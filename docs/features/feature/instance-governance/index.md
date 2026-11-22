---
doc_type: feature_index
feature_id: feature-instance-governance
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-11-21
code_paths:
  - internal/lifecycle/process/lock/proclock.go
  - internal/lifecycle/process/lock/proclock_unix.go
  - internal/lifecycle/process/lock/proclock_windows.go
  - internal/lifecycle/process/identity/procidentity.go
  - internal/lifecycle/process/identity/procidentity_unix.go
  - internal/lifecycle/process/identity/procidentity_windows.go
  - internal/lifecycle/process/identity/procidentity_info.go
  - internal/lifecycle/instance/registry/registry.go
  - internal/lifecycle/instance/registry/prune.go
  - internal/lifecycle/instance/gov/instancegov.go
  - internal/lifecycle/instance/gov/policy.go
  - internal/lifecycle/instance/gov/cpu.go
  - internal/lifecycle/instance/gov/runtime.go
  - internal/lifecycle/idlewatch/idlewatch.go
  - internal/lifecycle/process/watch/procwatch.go
  - internal/lifecycle/retention/retention.go
  - internal/lifecycle/instance/reaper/reaper.go
  - internal/lifecycle/instance/reaper/census.go
  - internal/lifecycle/instance/reaper/staledirs.go
  - internal/platform/semver/semver.go
  - internal/platform/state/paths/paths.go
  - cmd/browser-agent/daemon_governance.go
  - internal/lifecycle/retention/sweep/retention.go
  - internal/lifecycle/instance/censuscmd/census.go
  - cmd/browser-agent/config.go
  - cmd/browser-agent/main_connection_mcp.go
  - internal/lifecycle/bridge/bridge_governance.go
  - internal/lifecycle/bridge/bridge_startup.go
  - internal/lifecycle/procctl/terminate.go
  - internal/lifecycle/runtimeflags/flags.go
  - internal/lifecycle/startupconfig/help.go
test_paths:
  - internal/lifecycle/instance/censuscmd/census_test.go
  - internal/lifecycle/process/lock/proclock_test.go
  - internal/lifecycle/process/identity/procidentity_test.go
  - internal/lifecycle/instance/registry/registry_test.go
  - internal/lifecycle/instance/gov/policy_test.go
  - internal/lifecycle/instance/gov/instancegov_test.go
  - internal/lifecycle/idlewatch/idlewatch_test.go
  - internal/lifecycle/process/watch/procwatch_test.go
  - internal/lifecycle/retention/retention_test.go
  - internal/lifecycle/instance/reaper/reaper_test.go
  - internal/lifecycle/instance/reaper/census_test.go
  - internal/lifecycle/instance/reaper/staledirs_test.go
  - internal/platform/semver/semver_test.go
  - cmd/browser-agent/daemon_governance_test.go
  - internal/lifecycle/retention/sweep/retention_test.go
  - internal/lifecycle/bridge/bridge_governance_test.go
  - internal/composition/testsupport/integration/instance_cap_test.go
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

1. **Kernel-held singleton** (`internal/lifecycle/process/lock`). `flock`/`LockFileEx` held for
   the process lifetime. Mutual exclusion is guaranteed by the kernel and released
   on any death including SIGKILL and OOM, so there are no stale-lock heuristics,
   no grace windows, and no read-decide-write race.
2. **Machine-wide registry** (`internal/lifecycle/instance/registry`). Its location ignores
   `KABOOM_STATE_DIR`: isolation isolates *data*, never *accounting*. Under
   `go test` an explicit `KABOOM_REGISTRY_DIR` is required, so a test can never
   write into a developer's home — the defect that produced the 750MB above.
3. **Identity-checked liveness** (`internal/lifecycle/process/identity`). Liveness compares pid
   **plus start time plus command**, because `kill(pid, 0)` answers "does some
   process hold this pid", not "is my process still running".
4. **Admission with caps** (`instancegov.Admit`, `instancegov.Surplus`). One
   production daemon per machine; test daemons capped at `min(4, max(2, cores/4))`,
   evicting oldest-first. Admission and the reaper call ONE cap function, differing
   only in whether a candidate is joining (`incoming` 1 or 0), and ONE wedged
   predicate (`instancegov.IsWedged`) — those pairs had already drifted apart once,
   disagreeing about a record whose heartbeat could not be parsed.
5. **Idle exit** (`internal/lifecycle/idlewatch`). A daemon serving nobody releases its
   ports. Any unreadable work signal counts as busy, so it never exits mid-recording.
6. **Bridge parent-death** (`internal/lifecycle/process/watch`). stdin EOF covers a clean client
   exit; this covers a client that was killed and never closed the pipe.
7. **Reaper and census** (`internal/lifecycle/instance/reaper`, `--instances`, `--reap`).
8. **Retention budgets** (`internal/lifecycle/retention`). Count, bytes, and age per
   directory, applied hourly with single-pass eviction.
9. **Cross-process proof** (`instance_cap_test.go`). Eight daemons launch
   simultaneously; exactly one binds and seven exit 0.

## What this replaced

Singleton admission previously lived in `internal/lifecycle/daemon` as a
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

The operator surfaces are packages, not files in `main`. `--instances`/`--reap`
live in `internal/lifecycle/instance/censuscmd` (`Census`, `Reap`) and the hourly
capture-budget sweep in `internal/lifecycle/retention/sweep` (`Start`,
`Sweep`, `Budgets`). The sweeper takes a `LogLifecycle` function rather than the
whole `*Server`, so it depends on the one thing it uses instead of the composition
root, and `retention_test.go` moved with it.

## Operating it

```bash
kaboom --instances          # every daemon and bridge on this machine
kaboom --reap --dry-run     # what would be reclaimed
kaboom --reap               # reclaim it
```

`--reap` never terminates a healthy, in-cap daemon. Over-cap **production**
daemons are handled at admission (the late arrival defers), never by the reaper,
so a starting daemon can never kill the one a developer is using.
