---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Daemon Cleanup & Version Management

## Overview

Kaboom is a persistent daemon server. To ensure clean upgrades and prevent version conflicts, we've implemented comprehensive daemon cleanup and version validation.

## Issues Fixed (Feb 10, 2026)

### 🔴 Critical: v5.8.2 Pinned in optionalDependencies

**Problem:** The npm package's `optionalDependencies` were hard-coded to v5.8.2 instead of v6.0.0, causing users to receive the old binary even after upgrading.

**File:** `npm/kaboom-mcp/package.json`

**Fix:** Updated all optionalDependencies to `6.0.0`:
```json
"optionalDependencies": {
  "@brennhill/kaboom-darwin-arm64": "6.0.0",
  "@brennhill/kaboom-darwin-x64": "6.0.0",
  "@brennhill/kaboom-linux-arm64": "6.0.0",
  "@brennhill/kaboom-linux-x64": "6.0.0",
  "@brennhill/kaboom-win32-x64": "6.0.0"
}
```

### 🟡 Missing Daemon Cleanup on Install

**Problem:** The `preinstall` hook only tried to uninstall the old npm package but didn't kill the running daemon, leaving old processes alive.

**Fix:** Enhanced `preinstall` hook to kill all running kaboom processes:
```json
"preinstall": "node -e \"const cp = require('child_process'); try { cp.execSync('npm uninstall -g kaboom-mcp', {stdio:'ignore'}); } catch(e) {} try { const cmd = process.platform === 'win32' ? 'taskkill /F /IM kaboom.exe 2>nul || true' : 'pkill -9 kaboom 2>/dev/null || true'; cp.execSync(cmd, {stdio:'ignore', shell:true}); } catch(e) {}\""
```

Also updated `preuninstall` to use the same cross-platform cleanup.

### 🟡 No Force Cleanup for Users

**Problem:** Users had no way to force-kill old daemons if installation failed or if they needed to manually recover.

**Fixes:**
1. Added `--force` flag to Go binary
2. Created user-accessible cleanup script
3. Version validation in CI/CD

## Usage

### Option 1: Automatic Cleanup During Install (Recommended)

When you run `npm install -g kaboom-mcp`, the `preinstall` hook automatically:
- Uninstalls old global version
- Kills all running daemons
- Cleans up stale PID files

```bash
npm install -g kaboom-mcp@latest
```

No additional action needed!

### Option 2: Manual Cleanup Before Install

If you want to ensure daemons are cleaned up before installing:

```bash
# Using the Go binary
kaboom --force

# Or using the shell script
./scripts/maintenance/clean-old-daemons.sh
```

### Option 3: Stop Specific Server

To stop only the server on a specific port:

```bash
kaboom --stop --port 7890
```

## Version Synchronization

Since npm's `package.json` doesn't support variable interpolution, we use:

1. **Manual sync via Makefile:** The `sync-version` target updates all version references
   ```bash
   make sync-version
   ```

2. **Validation:** The `validate-deps-versions` check ensures optionalDependencies match the main version
   ```bash
   make validate-deps-versions
   ```

3. **CI Integration:** The quality gate includes version validation
   ```bash
   make quality-gate
   ```

### How It Works

**File:** `npm/kaboom-mcp/lib/validate-versions.js`

This script checks that all optionalDependencies match the main package version. If they don't match:

```
❌ Version mismatch in optionalDependencies:

  @brennhill/kaboom-darwin-arm64: "5.8.2" (expected "6.0.0")
  @brennhill/kaboom-darwin-x64: "5.8.2" (expected "6.0.0")
  ...

Fix by running: make sync-version
```

## Implementation Details

### Go Binary: `--force` Flag

**Files:** `cmd/browser-agent/config.go` and `cmd/browser-agent/internal/procctl/stop.go`

The `--force` flag invokes `procctl.ForceCleanup()` which:

1. **Finds all kaboom processes** using:
   - `lsof -c kaboom` (Unix-like systems)
   - `taskkill` (Windows)

2. **Kills them gracefully:**
   - Sends SIGTERM first (graceful shutdown)
   - Waits 100ms for process to exit
   - Sends SIGKILL if needed

3. **Cleans up PID files** for ports 7890-7910

4. **Logs to lifecycle events** for debugging

### Package Hooks

**Files:**
- `npm/kaboom-mcp/package.json` - preinstall/preuninstall scripts

**preinstall:**
- Runs before npm install
- Uninstalls old global version
- Kills all running daemons
- Cross-platform (macOS/Linux/Windows)

**preuninstall:**
- Runs before npm uninstall
- Kills all running daemons

### Shell Script: `clean-old-daemons.sh`

**File:** `scripts/maintenance/clean-old-daemons.sh`

User-accessible cleanup script for manual daemon cleanup.

Platform detection:
- **macOS:** Uses `lsof` + `pkill`
- **Linux:** Uses `pgrep` + `pkill`
- **Windows:** Uses `taskkill`

## Testing

### Verify No Version Drift

```bash
make validate-deps-versions
```

Should output:
```
✓ All optionalDependencies match version 6.0.0
```

### Verify Daemon Cleanup Works

```bash
# Start a daemon
kaboom

# In another terminal, force cleanup
kaboom --force

# Verify it's gone
lsof -ti :7890  # Should return nothing
```

### Verify Install Process

```bash
# Simulate old version
npm install -g kaboom-mcp@5.8.2

# Verify 5.8.2 is running
kaboom --version  # Should show 5.8.2

# Upgrade to 6.0.0
npm install -g kaboom-mcp@latest

# Verify 6.0.0 is running and daemon was cleaned up
kaboom --version  # Should show 6.0.0
```

## Troubleshooting

### "Still have v5.8 after upgrade"

This shouldn't happen anymore, but if it does:

1. **Force cleanup:**
   ```bash
   kaboom --force
   ```

2. **Uninstall completely:**
   ```bash
   npm uninstall -g kaboom-mcp
   ```

3. **Reinstall:**
   ```bash
   npm install -g kaboom-mcp@latest
   ```

### "Multiple kaboom processes still running"

Check for zombie processes:

```bash
# Find all kaboom processes
ps aux | grep kaboom

# Kill everything
kaboom --force

# Or manually
pkill -9 kaboom
```

### "PID file stale, can't start new daemon"

```bash
# Clean up stale PID files
rm -f ~/.kaboom-*.pid

# Then start fresh
kaboom
```

## Release Checklist

Before releasing a new version:

1. **Update VERSION file** to new semantic version
2. **Run sync-version:**
   ```bash
   make sync-version
   ```
3. **Validate versions match:**
   ```bash
   make validate-deps-versions
   ```
4. **Run full quality gate:**
   ```bash
   make quality-gate
   ```
5. **Commit and tag:**
   ```bash
   git commit -am "v{VERSION}"
   git tag v{VERSION}
   ```
6. **Push and release**

The sync-version Makefile target will automatically update:
- `npm/kaboom-mcp/package.json` - optionalDependencies
- `npm/*/package.json` - all platform packages
- `pypi/*/pyproject.toml` - all PyPI packages
- And more (see Makefile sync-version target)

## Related Files

- `npm/kaboom-mcp/package.json` - Main package with daemon cleanup hooks
- `npm/kaboom-mcp/lib/validate-versions.js` - Version validation script
- `cmd/browser-agent/config.go` - `--force` flag definition and dispatch
- `cmd/browser-agent/internal/procctl/stop.go` - daemon stop and forced cleanup implementation
- `scripts/maintenance/clean-old-daemons.sh` - User-friendly cleanup script
- `Makefile` - sync-version and validate-deps-versions targets
