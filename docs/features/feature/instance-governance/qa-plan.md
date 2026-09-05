---
feature: instance-governance
status: implemented
doc_type: qa-plan
feature_id: feature-instance-governance
last_reviewed: 2026-09-05
---

# Instance Governance — QA Plan

## Behaviours under test

| Behaviour | Test file |
| --- | --- |
| Acquire grants a free lock; a second acquire in the same process is refused; release is idempotent and nil-safe; the kernel releases the lock when the holder is killed (a subprocess helper holds the lock, is killed, and a fresh acquire succeeds). | `internal/proclock/proclock_test.go` (150 lines) |
| A process snapshot includes this process itself; an impossible pid is omitted; a recycled pid (same pid, different start/command) is rejected as a match; an empty identity on either side never matches; `Self()` returns this process's own identity. | `internal/procidentity/procidentity_test.go` (83 lines) |
| Registry directory resolution ignores a `KABOOM_STATE_DIR` override; refuses the real home directory when `testing.Testing()` is true and no `KABOOM_REGISTRY_DIR` is set; register/list/deregister round-trips; a heartbeat call advances only `heartbeat_at`, never `started_at`; `Prune` removes dead and pid-recycled records but keeps a live-but-wedged one; `List` skips unparseable/corrupt entries instead of failing the whole listing. | `internal/instancereg/registry_test.go` (211 lines) |
| `autoParallelCap` is bounded to [2, 4] by core count; `DefaultPolicy` allows exactly one production daemon; `Surplus` counts the incoming candidate, evicts oldest-first and only the excess, clamps an impossible (< 1) cap to 1, and never returns more victims than there are members; `oldestFirst` sorts unreadable start times last and does not mutate its input; `Daemons`/`Bridges` select strictly by role/kind; `IsWedged` falls back to start time when the heartbeat is unreadable. | `internal/instancegov/policy_test.go` (169 lines) |
| First production daemon proceeds and registers; a second defers to the lock holder; a newer version requests handoff and proceeds; an older version defers to a newer incumbent; a parallel daemon over cap evicts the oldest; parallel daemons never take the singleton lock; `Release` deregisters and unlocks; a failed handoff does not let the caller proceed; a deferral names the incumbent even when it just acquired the lock; a deferral waits briefly for the incumbent to publish; a newer install epoch supersedes at the same version; an equal-or-older install defers at the same version; an unknown install epoch never supersedes. | `internal/instancegov/instancegov_test.go` (400 lines, 13 test functions) |
| An idle daemon exits after the idle window; a busy daemon never exits; activity resets a partially-elapsed idle window; `MaxLifetime` exits even while busy; a zero `MaxLifetime` means unlimited; a zero `IdleAfter` disables idle exit; a nil busy probe is treated as busy. | `internal/idlewatch/idlewatch_test.go` (131 lines, 7 tests) |
| `parentGone` detects reparenting; `Watch` fires when the parent disappears; `Watch` stops cleanly on context cancellation; `Watch` is a no-op when the process was already orphaned at startup. | `internal/procwatch/procwatch_test.go` (110 lines, 4 tests) |
| `Enforce` removes the oldest files until the file-count budget fits; until the byte budget fits; removes files past `MaxAge`; a zero budget removes nothing; a missing directory is not an error; subdirectories are never removed; all three budget dimensions apply in one pass. | `internal/retention/retention_test.go` (157 lines, 7 tests) |
| A healthy daemon is never touched by `Plan`; a dead record is pruned, not killed; a wedged instance is killed; over-cap parallel daemons are killed oldest-first; a production daemon is never selected as an over-cap victim; `DryRun` performs no side effects; `Apply` terminates and removes; a termination failure is reported (not swallowed). | `internal/reaper/reaper_test.go` (203 lines, 8 tests) |
| `FormatCensus` names each instance; marks a stale heartbeat; reports correctly when the registry is empty. | `internal/reaper/census_test.go` (59 lines, 3 tests) |
| Stale parallel directories are removed; a live (claimed) parallel directory is never removed; `--dry-run` reports without removing; only directories matching the generated-run-dir name shape are eligible; a missing parallel root is not an error. | `internal/reaper/staledirs_test.go` (117 lines, 5 tests) |
| Version part parsing; malformed input never yields a negative comparison; `IsNewer` ordering; equal versions are neither newer than each other; `Same`. | `internal/semver/semver_test.go` (106 lines, 5 tests) |
| `busyProbe` reports busy for each of the four work kinds independently; `busyProbe` treats any missing signal as busy; `idleConfigFor` applies a hard lifetime bound only to parallel daemons. | `cmd/browser-agent/daemon_governance_test.go` (108 lines, 3 tests) |
| Every capture budget is internally bounded (non-zero, sane ceilings); every capture budget's directory resolves without error. | `cmd/browser-agent/retention_policy_test.go` (44 lines, 2 tests) |
| `governBridge` registers on start and deregisters on release; a bridge stands down when its parent (client) process disappears, even simulated via a polled pid rather than a real kill; a bridge continues serving when the registry itself is unavailable. | `cmd/browser-agent/internal/bridge/bridge_governance_test.go` (95 lines, 3 tests) |
| End-to-end: 8 real daemon processes launched simultaneously against one shared registry register exactly one production daemon; the survivor is verified alive and holding at least one port; the losing daemons exit 0 (not crash, not linger); deferring daemons leave no registry entries behind. | `cmd/browser-agent/internal/integrationtest/instance_cap_test.go` (212 lines; `TestOnlyOneProductionDaemonSurvivesAConcurrentLaunch`, `TestDeferringDaemonsLeaveNoRegistryEntries`) |

Platform-specific lock/identity code (`proclock_unix.go`/`proclock_windows.go`, `procidentity_unix.go`/`procidentity_windows.go`) is gated by `//go:build !windows` / `//go:build windows`; the same test files (`proclock_test.go`, `procidentity_test.go`) exercise whichever implementation matches the CI runner's OS. Whether Windows CI actually runs this test suite was not verified while writing this document.

## Manual verification

- The disk-usage measurements in `product-spec.md` (880 abandoned dirs, 750 MB test residue, 1.0 GB total) were gathered by hand on one developer machine before this feature shipped, to characterize the defect — they are not reproduced by an automated test.
- `kaboom --instances` and `kaboom --reap --dry-run` output readability (column alignment, human-facing wording) is not asserted beyond `FormatCensus`'s three unit tests; a full CLI run against a real populated registry has not been captured here as a golden-file test.

## Not covered today

- No test exercises `requestSelfShutdown`'s actual `SIGTERM`-to-self path, or verifies that idle shutdown produces the same observable teardown sequence as Ctrl+C or `--stop` — `daemon_governance_test.go` covers `busyProbe` and `idleConfigFor` only.
- No test in the listed suite drives `startRetentionSweeper`'s goroutine/ticker loop end-to-end, including the `retention_dir_unresolved` and `retention_sweep_failed` logging branches in `sweepCaptureBudgets` — `retention_policy_test.go` only checks that `captureBudgets()` values are bounded and that each budget's directory resolves.
- No test exercises `startDaemonGovernance`/`startGovernanceLoops`/`startClientReaper` wired together as the daemon actually runs them; coverage stops at the individual pieces (`busyProbe`, `idleConfigFor`) each function composes.
- No listed test calls `admitDaemon`/`admitDaemonOrDefer` in `cmd/browser-agent` directly, so the real `RequestShutdown` closure — which calls `server.daemonRecovery.RequestShutdown(incumbentPort)` over HTTP — is untested; `instancegov_test.go` exercises `shouldRequestHandoff`/`requestHandoff` only against fake `RequestShutdown` closures supplied by the test.
- `TestGovernBridgeStandsDownWhenTheClientDies` simulates the parent disappearing via a polled function value (`atomic.Int64`), not an actual killed OS process; the real `os.Getppid()`-based reparenting path it stands in for is exercised indirectly by `procwatch_test.go`'s own unit tests, not by an end-to-end bridge test with a real child/parent process pair.

## See also

- [./index.md](./index.md)
- [./product-spec.md](./product-spec.md)
- [./tech-spec.md](./tech-spec.md)
