---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
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

### 2. Startup Cleanup (To Implement)

Before spawning a new daemon, check and clean:

```go
func cleanupStaleServer(port int) error {
    // Read PID file
    pidFile := getPIDFile(port)
    if !fileExists(pidFile) {
        return nil // No PID file, we're good
    }

    // Check if process is alive
    pid := readPID(pidFile)
    if !processExists(pid) {
        // Stale PID file, remove it
        os.Remove(pidFile)
        return nil
    }

    // Process exists, try graceful stop
    syscall.Kill(pid, syscall.SIGTERM)
    time.Sleep(2 * time.Second)

    // Force kill if still alive
    if processExists(pid) {
        syscall.Kill(pid, syscall.SIGKILL)
    }

    os.Remove(pidFile)
    return nil
}
```

### 3. Port Conflict Fast-Fail (To Implement)

```go
func checkPortAvailable(port int) error {
    ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
    if err != nil {
        return fmt.Errorf("port %d in use by another process", port)
    }
    ln.Close()
    return nil
}
```

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
