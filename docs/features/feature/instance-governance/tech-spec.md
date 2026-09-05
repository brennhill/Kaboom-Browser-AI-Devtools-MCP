---
feature: instance-governance
status: implemented
doc_type: tech-spec
feature_id: feature-instance-governance
last_reviewed: 2026-09-05
---

# Instance Governance — Tech Spec

## Components

| Component | File(s) | Responsibility |
| --- | --- | --- |
| Kernel lock | `internal/proclock/proclock.go`, `proclock_unix.go` (`//go:build !windows`, flock), `proclock_windows.go` (`//go:build windows`, LockFileEx) | `Acquire`/`Release`/`Write`/`ReadUnlocked` on an exclusive, process-lifetime lock file. Released by the kernel on any process death, including SIGKILL and OOM. |
| Process identity | `internal/procidentity/procidentity.go`, `procidentity_unix.go` (`ps` listing), `procidentity_windows.go` (`//go:build windows`, WMIC) | `Info{Start, Command}` compared as opaque strings; `Snapshot`/`Self`/`IsAlive`/`AliveIn` answer "is pid N still running as the process a record was written for," not "does some process hold pid N." |
| Machine registry | `internal/instancereg/registry.go`, `prune.go` | `Record` (snake_case JSON: `pid`, `role`, `ports`, `state_dir`, `version`, `install_epoch`, `parallel`, `started_at`, `heartbeat_at`, `identity`), `Register`/`Handle.Heartbeat`/`Handle.SetPorts`/`Handle.Close`, `List`, `Live` (identity-checked subset of `List`), `Prune`, `DecodeRecord`, `RecordJSON`, `SingletonLockPath`. |
| Admission policy | `internal/instancegov/instancegov.go`, `policy.go`, `cpu.go`, `runtime.go` | `Admit`/`Config`/`Result`, `Policy`/`DefaultPolicy`, `IsWedged`, `Surplus`, `oldestFirst`, `shouldRequestHandoff`/`requestHandoff`, `StartHeartbeat`/`StartIdleWatch`. |
| Idle exit | `internal/idlewatch/idlewatch.go` | `Watcher`/`Config{IdleAfter, MaxLifetime, Poll, Busy}`; a clock-driven `Tick`, so tests never sleep. |
| Bridge parent-death | `internal/procwatch/procwatch.go` | `Watch`/`Config{OriginalPPID, Poll, CurrentPPID}`; detects reparenting by comparing the live parent pid to the one observed at startup. |
| Disk retention | `internal/retention/retention.go` | `Budget{MaxFiles, MaxBytes, MaxAge}`, `Enforce` (single-pass, oldest-first). |
| Reclamation planning | `internal/reaper/reaper.go`, `census.go`, `staledirs.go` | `Plan`/`Apply` (pure function of a census → actions), `FormatCensus`, `SweepParallelDirs`. |
| Version comparison | `internal/semver/semver.go` | `Parts`, `IsNewer`, `Same` — optionally `v`-prefixed, stops at the first non-numeric segment. |
| State paths | `internal/state/paths.go` | `RootDir` (honours `KABOOM_STATE_DIR`), `ScreenshotsDir`, `RecordingsDir`, `PerformanceTracesDir`, `EvidenceDir`, `LogsDir`. |
| Daemon wiring | `cmd/browser-agent/daemon_governance.go` | `admitDaemon`/`admitDaemonOrDefer`, `startDaemonGovernance`/`startGovernanceLoops`, `busyProbe`, `idleConfigFor`, `requestSelfShutdown`. |
| Retention wiring | `cmd/browser-agent/retention_policy.go` | `captureBudgets()` (5 budgeted directories), `startRetentionSweeper`, `sweepCaptureBudgets`. |
| Operator surface | `cmd/browser-agent/census_command.go` | `runCensus` (`--instances`), `runReap` (`--reap`/`--dry-run`), `sweepAbandonedParallelDirs`. |
| CLI dispatch | `cmd/browser-agent/config.go`, `cmd/browser-agent/internal/runtimeflags/flags.go`, `cmd/browser-agent/internal/startupconfig/help.go` | `--instances`/`--reap`/`--dry-run`/`--parallel` flag parsing and help text; `handleEarlyExitModes` dispatches `--instances`/`--reap` before any daemon starts. |
| Daemon entry point | `cmd/browser-agent/main_connection_mcp.go` | `runMCPMode` calls `admitDaemonOrDefer` as the **outer** gate, before the per-state-dir `daemonlife` checks and before `ensurePortAvailable`. |
| Bridge governance | `cmd/browser-agent/internal/bridge/bridge_governance.go`, `bridge_startup.go` | `governBridge`/`bridgeGovernance`; `RunMode` calls it and defers `release()`. |
| Process termination | `cmd/browser-agent/internal/procctl/terminate.go` | `TerminatePID` — the one function in the codebase that ends another process. |

## Data and control flow

### Admission sequence (`instancegov.Admit`)

`Admit` dispatches on the candidate's role:

- **`admitSingleton`** — production daemons (`Role == RoleDaemon`, `Parallel == false`).
- **`admitCapped`** — parallel test daemons and all bridges (`Config.Parallel == true` or `Role == RoleBridge`).

**`admitSingleton`:**

1. `proclock.Acquire(cfg.LockPath)`. On success, `finish()` registers the instance and publishes its identity into the lock file's own bytes (`lock.Write(handle.RecordJSON())`), so the lock and the registry can never disagree about who holds it.
2. On `proclock.ErrLocked`, `findIncumbent(cfg)` identifies the current holder: `readIncumbentOnce` first tries to decode the lock file's own payload; if that fails (the winner acquired the lock microseconds ago and has not written its payload yet) it falls back to scanning `instancereg.Live()` for a non-parallel daemon record. This retries every `incumbentPollInterval` (25 ms) for up to `incumbentPublishWait` (500 ms) before giving up and reporting no incumbent.
3. `shouldRequestHandoff(cfg, incumbent)`: true only if `cfg.RequestShutdown` is non-nil and the incumbent is non-nil, and either (a) `semver.IsNewer(cfg.Version, incumbent.Version)` is strictly true, or (b) the versions are equal (`semver.Same`) and `cfg.InstallEpoch > incumbent.InstallEpoch` with both epochs known (`> 0`). Both comparisons are strict, so two equal builds cannot alternate evicting each other.
4. If handoff is not warranted, `Admit` returns `Result{Outcome: OutcomeDefer, DeferTo: incumbent}` and the caller exits 0.
5. If handoff is warranted, `requestHandoff` calls `cfg.RequestShutdown(*incumbent)` — in the real daemon this is `admitDaemon`'s closure, which calls `server.daemonRecovery.RequestShutdown(incumbentPort)` over HTTP — then polls `proclock.Acquire` every `handoffPollInterval` (50 ms) until it succeeds or `cfg.HandoffTimeout` (default `defaultHandoffTimeout` = 5 s) elapses. **A failed handoff is returned as an error, not folded into a `Defer` outcome that would let the process exit 0** — `runMCPMode` in `main_connection_mcp.go` treats a non-nil `admitErr` from `admitDaemonOrDefer` as a startup failure.

**`admitCapped`:**

1. Reads `instancereg.Live()`, filters out the candidate's own pid (`peersOf`).
2. Selects members of the candidate's own class: `instancegov.Daemons(peers, true)` for a parallel daemon, `instancegov.Bridges(peers)` for a bridge.
3. Computes `Surplus(members, cap, incoming=1)` — the candidate itself counts against the cap it is joining.
4. If nobody is over cap, `finish()` registers unconditionally: **a capped candidate is never refused outright**, so a test run can always start.
5. If eviction is needed, `evict()` calls `cfg.Terminate(victim.PID, false)` for each victim (oldest first). For a bridge, `bridge_governance.go` passes `Terminate: func(int, bool) error { return nil }` — a bridge over cap is left to the reaper, never killed by its own successor, because "killing another editor's live session to start your own would be worse than the leak this bounds."

### Registration (`instancereg.Register` / `finish`)

`Register` stamps `pid` (`os.Getpid()`), `ppid` (`os.Getppid()`), `identity` (`procidentity.Self()`), and both `started_at`/`heartbeat_at` (UTC RFC3339Nano) onto the caller-supplied `Record`, then writes it as `<registry-dir>/<pid>.json`. If `procidentity.Self()` cannot determine the calling process's own identity, registration fails outright. In `instancegov.finish`, a registration failure releases any lock already acquired rather than proceeding unregistered — "an instance the census cannot see is exactly the uncounted daemon this package exists to prevent."

### Heartbeat and idle exit (`runtime.go`, `daemon_governance.go`)

- `Result.StartHeartbeat(ctx, interval, onError)` runs a `time.Ticker`-driven goroutine calling `Handle.Heartbeat()` (rewrites only `heartbeat_at`, leaves `started_at` untouched) at `DefaultHeartbeatInterval` (10 s). A write failure invokes `onError` rather than being swallowed — `daemon_governance.go` logs `instance_heartbeat_failed`, because a silently-failed heartbeat makes a healthy daemon look wedged to the reaper.
- `Result.StartIdleWatch(ctx, cfg, onExit)` wraps `idlewatch.New(cfg, time.Now())` and its `Run` loop in a goroutine.
- `idleConfigFor` (daemon_governance.go): a production daemon gets `IdleAfter: 2h, Poll: 1m` and **no** `MaxLifetime` (an actively-used daemon must never be shut down purely on age). A parallel daemon gets `IdleAfter: 5m, MaxLifetime: 30m, Poll: 30s` — the hard lifetime exists because a test run that dies without stopping its own daemon leaves nothing else to reclaim it before the reaper runs.
- `busyProbe` (daemon_governance.go) resolves busy whenever any of its four signals — connected MCP client count, extension attached, active recording, live terminal session count — is non-nil and reports work, and **whenever any of the four probe functions is nil** ("a work signal is unavailable; assuming work in progress"). `idlewatch.Watcher.probe()` applies the same fail-safe default at the package level: a nil `Config.Busy` is always busy.
- On idle exit, `requestSelfShutdown` sends `SIGTERM` to the daemon's own pid — the same signal path as Ctrl+C or `--stop` — rather than a second, divergent teardown sequence.

### Bridge governance (`bridge_governance.go`, `bridge_startup.go`)

`governBridge(ctx, cfg, standDown)`:

1. Calls `instancegov.Admit` with `Role: RoleBridge`. A non-`OutcomeProceed` result (or an `Admit` error) still returns a callable no-op/release function — bridge admission failures are never fatal to serving the MCP client, because "the census is bookkeeping, and refusing to serve an MCP session because a bookkeeping file could not be written would turn a diagnostic failure into an outage."
2. On success, starts the heartbeat (`instancegov.DefaultHeartbeatInterval` = 10 s, no idle watch — a bridge's lifetime is governed by its parent, not by an idle timer) and launches `procwatch.Watch(ctx, cfg, onGone)` against the pid observed as the parent at startup (`cfg.OriginalPPID`, defaulting to `os.Getppid()` if zero).
3. `procwatch.Watch` polls `CurrentPPID()` every `Poll` (default 2 s); `parentGone` reports true whenever the current parent differs from the original **and** the original was `> 1` (a value of 1 or less means the process was already orphaned by design, e.g. a deliberately daemonized process, and watching is skipped entirely).
4. On parent death, `governBridge`'s wrapped callback calls `release()` (idempotent via `sync.Once`) **before** invoking `standDown`, so a bridge is never listed as live in the census while it is already on its way out. `bridge_startup.go`'s `RunMode` wires `standDown` to cancel its governance context and `os.Exit(0)`.

### Reclamation (`reaper.Plan` / `reaper.Apply`)

`Plan(Input{Live, All, Policy, HeartbeatTTL, Now})` computes three ordered passes into one `Report`:

1. **Dead entries** — every record in `All` whose pid is not in `Live` becomes `ActionPrune` (removes only the registry file; nothing is signalled).
2. **Wedged** — every remaining `Live` record for which `instancegov.IsWedged(rec, now, ttl)` is true becomes `ActionKill`. `IsWedged` first tries `rec.HeartbeatAge(now)`; if the heartbeat cannot be parsed, it falls back to `rec.Started()` — a record with **no readable liveness signal at all** that has existed longer than the TTL is wedged by any useful definition. This is the same function the census (`reaper/census.go`'s `heartbeatLabel`) uses to mark an entry `STALE`, so the operator-facing table and the reclamation plan can never disagree about one record.
3. **Over-cap** (`overCapActions`) — applies `instancegov.Surplus(members, cap, incoming=0)` (the same function admission uses, with `incoming=0` because nothing is joining, only being reclaimed) to parallel daemons and bridges only. **Production daemons are excluded from this pass entirely** — the `DaemonCap` is enforced at admission, where the late arrival defers; reclaiming it here instead would let a newly starting daemon kill the incumbent a developer is using.

`Apply(report, Deps{DryRun, Terminate, Remove})`: for each `ActionKill`, calls `Terminate` unless `DryRun`; if `Terminate` returns an error, that failure is appended to a joined error list and **the loop `continue`s before incrementing `Killed` or calling `Remove`** — a failed kill leaves the registry entry in place rather than falsely reporting the process reclaimed. For a successful kill or an `ActionPrune`, `Remove(action.Record.Path)` deletes the registry file (skipped in `DryRun`, and skipped if `Path` is empty).

### Disk retention (`retention.Enforce`, `retention_policy.go`)

`captureBudgets()` names five directories with independent ceilings:

| Directory | Max files | Max bytes | Max age |
| --- | --- | --- | --- |
| screenshots | 500 | 256 MiB | 7 days |
| recordings | 500 | 256 MiB | 30 days |
| performance-traces | 20 | 256 MiB | 7 days |
| evidence | 500 | 128 MiB | 7 days |
| logs | 50 | 128 MiB | 14 days |

`startRetentionSweeper` runs `sweepCaptureBudgets` immediately at startup (so a daemon that was down while disk accumulated does not wait an hour) and then every `retentionSweepInterval` (1 hour) until the context is cancelled. `retention.Enforce` scans the directory (non-recursive; subdirectories are never touched), sorts entries oldest-first, then makes a **single pass**: each file's removal decision is made from running totals (`remainingFiles`, `remainingBytes`) rather than re-scanning the directory after each deletion, per this repository's rule against loop-remove-recheck eviction. A file over `MaxAge` is removed regardless of the count/byte budgets; count and byte budgets are otherwise each evaluated independently against the running totals.

### Operator surface (`census_command.go`)

- `runCensus` (`--instances`): prunes dead entries first (`instancereg.Prune`) so the listing reflects the machine as it is now, then prints `reaper.FormatCensus(instancereg.List(), now)`.
- `runReap` (`--reap` / `--dry-run`): reads both `instancereg.List()` (all) and `instancereg.Live()` (alive), builds a `reaper.Plan`, prints each planned action's kind/pid/role/ports/reason, applies it via `reaper.Apply` with `procctl.TerminatePID` and `os.Remove`, then separately sweeps abandoned `--parallel` state directories older than `staleParallelDirMaxAge` (24 hours) that are not claimed by any live instance's `StateDir`.
- Both commands are dispatched by `handleEarlyExitModes` in `config.go`, before `runMCPMode` or any daemon start.

### Process termination (`procctl.TerminatePID`)

`TerminatePID(pid, force)`: on Windows, or when `force` is true, kills directly. Otherwise sends `SIGTERM`, sleeps `TerminateGracePeriod` (500 ms), checks `IsProcessAlive(pid)`, and escalates to `SIGKILL` only if the process is still alive. This is the one place in the codebase that ends another process — it replaced separate implementations that had existed in `daemonrecovery` and would have needed re-implementing again for the reaper.

## Invariants

| Invariant | Where enforced |
| --- | --- |
| The registry location ignores `KABOOM_STATE_DIR` always. | `instancereg.Dir()` resolves independently of `state.RootDir()`'s `KABOOM_STATE_DIR` override. |
| Under `go test`, the registry directory must be set explicitly via `KABOOM_REGISTRY_DIR`, or `Dir()` errors. | `instancereg.Dir()` calls `testing.Testing()` and refuses the real home directory. |
| `DaemonCap` is 1 in `DefaultPolicy()`. | `instancegov.DefaultPolicy()`; enforced in practice by the kernel-held singleton lock, not by counting. |
| `ParallelCap` is `clamp(cores/4, 2, 4)`. | `instancegov.autoParallelCap`. |
| `BridgeCap` is 8. | `instancegov.DefaultPolicy()`. |
| Admission and reclamation share one wedged predicate (`IsWedged`) and one eviction-selection function (`Surplus`). | `instancegov/policy.go`, called from both `instancegov.admitCapped` and `reaper.overCapActions`/`reaper.Plan`. |
| A record with an unparseable `started_at` sorts **last** in `oldestFirst`, so a corrupt timestamp is never the automatic eviction victim. | `instancegov.oldestFirst`. |
| A record with empty `procidentity.Info` on either side never matches as alive. | `procidentity.matches`. |
| Handoff supersession (version, then install epoch) is strict (`>`), never `>=`. | `instancegov.shouldRequestHandoff`. |
| Production daemons are never selected for over-cap eviction. | `reaper.overCapActions`'s class list contains only `"parallel daemon"` and `"bridge"`. |
| A failed `Terminate` in `reaper.Apply` skips both the kill count and the registry-file removal for that action. | `reaper.Apply`'s `continue` before `Remove`. |

## Failure modes

| Failure | Behaviour |
| --- | --- |
| Two daemons race to acquire the singleton lock. | The kernel grants exactly one `proclock.Acquire`; the other observes `ErrLocked` and defers (or requests handoff). Proven end-to-end by `TestOnlyOneProductionDaemonSurvivesAConcurrentLaunch` (8 real processes, one shared registry). |
| The lock's winner has not yet published its identity when a loser reads the lock file. | `findIncumbent` polls up to 500 ms (`incumbentPublishWait`) before falling back to a registry scan, then to reporting no incumbent (deferral message names no pid). |
| An incumbent is asked to stand down but never releases the lock within `HandoffTimeout` (default 5 s). | `requestHandoff` returns an error; `Admit` returns `Result{Outcome: Defer}` **and** a non-nil error; `runMCPMode` treats this as a hard startup failure (`return admitErr`), not a clean exit-0 defer. |
| `instancereg.Register` fails (identity unresolvable, or the write fails). | Any lock already acquired is released; `Admit` returns an error; the caller does not proceed unregistered. |
| A capped candidate (parallel daemon or bridge) needs eviction but `Config.Terminate` is nil. | `evict()` returns an explicit error ("no Terminate function was provided") rather than silently admitting over cap. |
| All four `busyInputs` probes are unavailable, or any one of them is nil. | `busyProbe` always reports busy; the daemon never idle-exits while its own instrumentation is broken. |
| A capture directory cannot be resolved (`state.ScreenshotsDir()` etc. errors). | Logged as `retention_dir_unresolved`; that budget is skipped for the sweep, other budgets still run. |
| `os.Remove` fails for a file `retention.Enforce` decided to evict. | The file is counted in `KeptFiles`/`KeptBytes` instead of `RemovedFiles`/`RemovedBytes`; the first such error is captured and returned/logged as `retention_sweep_failed`; the sweep continues through the remaining files rather than aborting. |
| `reaper.Apply`'s `Terminate` fails for one victim among several. | The failure is joined into the returned error; remaining actions in the plan are still applied. |
| A bridge's registry write fails at `governBridge` time. | `governBridge` returns a callable no-op release function; the bridge continues serving its MCP client, uncounted in the census, relying only on stdin EOF (or, if `OriginalPPID` was resolvable, `procwatch` is never started for this bridge either, since that only starts on the success branch). |
| A bridge's original parent was already `<= 1` (already orphaned/daemonized). | `procwatch.Watch` returns immediately without installing any monitoring — watching would be a guaranteed false positive. |
| A daemon is asked to hand off but its build's version or the incumbent's version is unparseable. | `semver.IsNewer`/`semver.Same` treat an uncomparable version as never newer; an unreadable version can never displace a running daemon. |

## See also

- [./index.md](./index.md)
- [./product-spec.md](./product-spec.md)
- [./qa-plan.md](./qa-plan.md)
