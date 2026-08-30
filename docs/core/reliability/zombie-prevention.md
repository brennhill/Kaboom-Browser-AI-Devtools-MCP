---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-08-30
---

# Zombie Process Prevention

## Problem

Multiple kaboom daemon processes can accumulate from:
- Multiple installation sources (npm, npx, dev builds)
- Testing and development spawning many instances
- Port conflicts causing silent failures
- No cleanup on version upgrades

## Solutions

### 1. npm Lifecycle Cleanup ✅

**package.json scripts:**
```json
{
  "preinstall": "npm uninstall -g kaboom-mcp (kills old version)",
  "preuninstall": "pkill -9 kaboom (kills all running servers)"
}
```

### 2. Machine-Wide Instance Governance (Implemented)

Superseded the sketches that used to sit here. The `cleanupStaleServer` sketch
below relied on `processExists(pid)`, which is exactly the defect that let lock
records written on 8 August claim pids since reassigned to TextInputSwitcher and
two Adobe Creative Cloud helpers — a pid-only check called all three alive, so
those state dirs could never be reclaimed and an unrelated user process was one
branch away from being SIGTERMed.

See `docs/features/feature/instance-governance/index.md` for the full design.
In short:

- **`internal/proclock`** — the singleton is a kernel-held `flock`, so there is no
  stale-lock detection to get wrong and no read-decide-write race to lose.
- **`internal/procidentity`** — liveness is pid **plus start time plus command**.
- **`internal/instancereg`** — a machine-wide census whose location ignores
  `KABOOM_STATE_DIR`, so `--parallel` and `--state-dir` can no longer create
  private universes with private (and therefore meaningless) singletons.
- **`internal/idlewatch`** — a daemon serving nobody exits instead of holding two
  ports indefinitely.
- **`internal/procwatch`** — a bridge whose MCP client died stands down.
- **`internal/reaper`** — `kaboom --instances` and `kaboom --reap`.
- **`internal/retention`** — count/bytes/age budgets per capture directory.

### 3. Port Conflict Fast-Fail (Implemented)

Port availability is checked before binding (`ensurePortAvailable`), and the
machine census records which ports each instance holds so `--instances` can
report a conflict rather than leaving an operator to guess.

### 4. The Launcher Must Not Outlive Its Child (Implemented)

The npm `bin` entries are POSIX `sh` exec shims. In MCP server mode the shim
`exec`s the Go binary, replacing its own process image:

```sh
exec "$candidate" "$@"
```

The process the MCP client spawned *is* the server, so there is no launcher left
to leak. Do not reintroduce a Node launcher here.

The Node launcher this replaced blocked in `execFileSync`, which is what made the
leak permanent:

- `execFileSync` does not return until the child exits, so the launcher could not
  act on `SIGTERM`/`SIGINT` aimed at the process group.
- The real teardown was worse than a missed signal: when the client ended a
  session it killed `npx`, and **no signal reached the launcher at all**. The
  launcher and the Go binary below it were orphaned and reparented to PID 1.
- Nothing else unwound them. The Go bridge exits on stdin `io.EOF`
  (`cmd/browser-agent/internal/bridge/bridge.go`), but the client held its end of
  the stdin pipe open, so EOF never arrived.

Signal forwarding alone would not have fixed this — an orphaned process receives
no signal to forward. Removing the extra process is what closes the hole.

Windows has no `exec()`, so `bin/*.cmd` invokes the binary from `cmd.exe`, which
remains a thin parent and exits with its child. No Node runtime sits in either
chain.

Enforced by `tests/cli/launcher/launcher-shim.contract.test.cjs`, which spawns
the shim and asserts the PID it spawned is the binary itself, with no descendants.

### 4b. The Process Census (Implemented)

Cleanup is not verification. `cleanup-test-daemons.sh` sweeps, but a sweep that
silently matches nothing looks exactly like a clean run — which is how twelve
daemons stayed alive for twenty hours behind green suites, and how ~6,900 orphaned
launchers accumulated unnoticed. Both were invisible because nothing ever counted.

`scripts/tests/framework/process-census.sh` counts, and a leak fails the run:

| Assertion | Question it answers |
| --- | --- |
| `assert_no_process_growth <label>` | Did this unit of work leave more kaboom processes than it found? |
| `assert_no_duplicate_daemons <label>` | Are two processes serving the same port? |
| `assert_no_launcher_processes <label>` | Has a Node launcher reappeared in front of a binary? |

Wired into `test-all-tools-comprehensive.sh` after **every category** and
`smoke-test.sh` after **every module**, and both gate the exit code. A category
whose tests all pass still turns the run red if it leaks.

Rules the guard follows, each of which cost something to learn:

- **Count before sweeping.** The census runs before cleanup, so it reports what
  the work left behind rather than what cleanup managed to hide.
- **Baseline-relative.** A developer legitimately has their own daemon running.
  The census measures growth caused by the suite, never absolute presence, and
  the production daemon (`~/.kaboom/bin/...`, port 7890) matches no pattern.
- **Settle, then fail.** Shutdown is not instantaneous, so the growth assertion
  polls for `KABOOM_CENSUS_SETTLE_SECONDS` (default 15) before declaring a leak.
  Waiting is the deliberate cost of never missing one.
- **Duplicates are checked by port**, not by count, so the check still holds where
  one long-lived daemon is intentional (smoke keeps one alive between modules).
- **A launcher is never acceptable at any count.** The bin entries are exec shims;
  the only way such a process exists is a regression.

`tests/cli/uat-assertions/process-census.test.cjs` checks every assertion in both
directions — that it fires on the real leak and stays quiet on the thing that
only resembles one. An assertion that cannot fail buys confidence it did not earn.

### 5. npx Cache Cleanup

Regular cleanup command:
```bash
# Clear npx cache older than 7 days
find ~/.npm/_npx -type d -mtime +7 -exec rm -rf {} \;

# Or clear all:
rm -rf ~/.npm/_npx
```

### 6. Development Best Practices

**Use one installation method at a time:**

**Development:** Point MCP config to dev build
```json
{
  "mcpServers": {
    "kaboom-browser-devtools": {
      "command": "/path/to/dev/kaboom/dist/kaboom-darwin-arm64"
    }
  }
}
```

**Production:** Use npm global
```json
{
  "mcpServers": {
    "kaboom-browser-devtools": {
      "command": "kaboom-mcp"
    }
  }
}
```

**Before switching contexts:**
```bash
# Kill all kaboom processes
pkill -9 kaboom

# Verify port is free
lsof -ti :7890 || echo "Port free"
```

## Manual Cleanup Commands

```bash
# Kill all kaboom processes
pkill -9 kaboom

# Remove PID files
rm -f /tmp/kaboom-*.pid

# Clear npx cache
rm -rf ~/.npm/_npx

# Uninstall npm global
npm uninstall -g kaboom-mcp

# Check for remaining processes
ps aux | grep kaboom | grep -v grep
```

## Monitoring

Check for zombie processes:
```bash
# Count kaboom processes
ps aux | grep -c kaboom | grep -v grep

# List all with ports
lsof -nP -iTCP -sTCP:LISTEN | grep kaboom
```
