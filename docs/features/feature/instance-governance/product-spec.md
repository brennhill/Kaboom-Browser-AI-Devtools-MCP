---
feature: instance-governance
status: implemented
doc_type: product-spec
feature_id: feature-instance-governance
last_reviewed: 2026-09-05
---

# Instance Governance — Product Spec

## What the operator gets

- Exactly one production Kaboom daemon per machine, no matter how many terminals, `--state-dir` overrides, git worktrees, or CI jobs invoke `kaboom`.
- `kaboom --instances` — lists every Kaboom daemon and bridge currently registered on the machine, with pid, role, ports, uptime, heartbeat freshness, and state directory.
- `kaboom --reap [--dry-run]` — reclaims dead registry entries, wedged (alive-but-not-heartbeating) processes, and over-cap test daemons/bridges, oldest first. `--dry-run` prints the same plan without terminating or removing anything.
- Idle daemons release their ports on their own: a production daemon exits after 2 hours with no connected client, extension, recording, or terminal session; a `--parallel` test daemon exits after 5 minutes idle, or after 30 minutes regardless of activity if the test run that started it never stopped it.
- Bridges (one stdio process per MCP client session) detect when their client process is gone — even if it was killed and never closed its pipe — and stand down instead of lingering.
- Captured artifacts (screenshots, recordings, performance traces, evidence, logs) are bounded per directory by file count, byte size, and age, swept hourly.
- A version upgrade takes over from an older daemon by asking it to stand down and then taking the freed lock — never by racing it for a port.

## Why this exists

Every guard that existed before this feature was scoped to a state directory, so `--parallel`, `--state-dir`, git worktrees, and CI each got a private universe in which "one daemon" was trivially true while the machine-wide count was unbounded. Measured on one developer machine before this feature shipped:

| Symptom | Measurement |
| --- | --- |
| Abandoned `--parallel` state dirs | 880 (29 MB, oldest 32 days) |
| Test residue in the real state root | 166,824 dirs / 191,923 files / 750 MB |
| Total state directory size | 1.0 GB |
| Captured artifacts | 5,975 screenshots, 5,317 recordings |
| Stale locks on ports outside the cleanup range | 7 (ports 7904, 19023–19310) |
| Lock records naming recycled PIDs | 3 (TextInputSwitcher, 2× Adobe Creative Cloud) |
| Daemon uptime with no client attached | 2 days 13 hours |
| Bridges alive after their session ended | 2, at 31 hours and ~24 MB each |

A pid-only liveness check (`kill(pid, 0)`) cannot tell "does some process hold this pid" from "is my process still running" — on this machine, lock records written 8 Aug named pids that the OS had since reassigned to unrelated user processes (TextInputSwitcher, two Adobe Creative Cloud helpers), so those entries could never expire and a pid-only reaper could have signalled the wrong process.

## Requirements this satisfies

- One production daemon per machine, enforced by a mechanism the kernel — not application heuristics — guarantees is mutually exclusive.
- Isolation for tests and worktrees must isolate *data* (state directories) without isolating *accounting* (the machine-wide census), so a test run can never make the singleton trivially true again.
- Liveness must be identity-checked (pid + start time + command), never pid-only, so a recycled pid cannot be mistaken for a live Kaboom process.
- An operator must be able to see every Kaboom process on the machine and reclaim only what is provably reclaimable, without ever risking a healthy daemon someone is using.
- Disk usage from captured artifacts must have a ceiling that is enforced automatically, not only when someone notices.

## What it deliberately does NOT do

- **Never terminates a healthy, in-cap production daemon.** The one-daemon cap is enforced at admission (the late arrival defers to the incumbent); the reaper's over-cap reclamation applies only to parallel test daemons and bridges, explicitly excluding production daemons, so a newly starting daemon can never kill the one a developer is actively using.
- **Does not treat an unreadable signal as safe to act on.** An unresolvable heartbeat, an unparseable start time, or a missing busy-probe input is always treated as "still alive" / "still busy" / "not the eviction victim" — never as licence to reclaim or evict.
- **Does not race an incumbent for a port on upgrade.** A newer build asks the incumbent to stand down over HTTP and waits (bounded by a timeout) for the kernel lock to free, rather than trying to bind alongside it.
- **Registry isolation is not configurable per test the way state-dir isolation is.** Tests must set `KABOOM_REGISTRY_DIR` explicitly; there is no fallback that would let a forgetful test write into a developer's real registry.

## What it replaced

Singleton admission previously lived in `cmd/browser-agent/internal/daemonlife` as a per-state-directory lock file with pid-based liveness, a startup grace window, and its own takeover policy. That package now owns only crash-loop restart throttling; the lock file, the startup grace window, and the pid-only liveness check it used are gone. The version-upgrade takeover and the equal-version install-epoch tiebreaker moved into `instancegov.shouldRequestHandoff`, so an upgrade still supersedes an incumbent — by asking it to stand down and then taking the freed lock, never by racing it.

## See also

- [./index.md](./index.md)
- [./tech-spec.md](./tech-spec.md)
- [./qa-plan.md](./qa-plan.md)
