---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Version Checking & Update Notifications

Kaboom includes automatic version checking to notify users when newer versions are available on GitHub. This document describes how the system works and how to use it.

## Overview

The version checking system has three components:

1. **Server → GitHub**: Server checks GitHub API daily for the latest release version
2. **Server → Extension**: Server exposes current version and available version in `/health` endpoint response
3. **Extension UI**: Extension reads `/health` response and displays a badge if an update is available

The server handles all GitHub API polling. The extension simply reads the cached result from `/health` and displays a notification badge. No configuration needed — everything runs automatically.

## User Experience

### When a New Version is Available

1. **Server checks GitHub** daily (once per 24 hours, on startup + periodic polling)
2. **Server caches result** for 6 hours (avoids GitHub API rate limiting)
3. **Extension polls `/health`** regularly as part of normal connectivity checks
4. **If available > current version**:
   - A **blue "⬆" badge** appears on the extension icon
   - Extension tooltip shows: `"Kaboom: New version available (v5.2.6)"`
5. **User can click extension icon** to see update info and download link
6. **Popup displays**:
   - Current version (e.g., "5.2.5")
   - Available version (e.g., "5.2.6")
   - Download link to GitHub releases

### Version Format

Versions follow semantic versioning: `X.Y.Z` (e.g., `5.2.5`)

- **Major version** (X): Breaking changes
- **Minor version** (Y): New features, backward compatible
- **Patch version** (Z): Bug fixes

### Example Scenarios

| Server Version | Available | Extension | Result |
|---|---|---|---|
| 5.2.5 | 5.2.5 | 5.2.5 | ✅ No badge |
| 5.2.5 | 5.2.6 | 5.2.5 | 🔵 Blue "⬆" badge (patch update available) |
| 5.2.5 | 5.3.0 | 5.2.5 | 🔵 Blue "⬆" badge (minor update available) |
| 5.2.5 | 5.2.5 | 5.3.0 | ✅ No badge (extension newer than available) |

## How It Works

### Architecture Diagram

```
┌──────────────────────────────────────────────────────────┐
│                      GitHub API                          │
│  /repos/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/...         │
└────────────────────────┬─────────────────────────────────┘
                         │ (daily check, 6h cache)
                         ↓
                    ┌─────────────┐
                    │   Server    │
                    │  (Go)       │
                    └──────┬──────┘
                           │ /health: { version, availableVersion }
                           ↓
                    ┌─────────────────────┐
                    │   Browser           │
                    │   Extension         │
                    │  (TypeScript)       │
                    │                     │
                    │  updateVersionBadge │
                    │  showNotification   │
                    └─────────────────────┘
                           │
                           ↓
                       User sees badge
```

### Server Side (Go)

#### Version Check Flow

1. **Server starts**: `startVersionCheckLoop()` is called during initialization
2. **Immediate check**: `checkGitHubVersion()` fetches the latest release from GitHub (if no cached value)
3. **GitHub API call**: Fetches `https://api.github.com/repos/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/latest`
4. **Extract version**: Parses `tag_name` field (e.g., "v5.2.6" → "5.2.6")
5. **Cache result**: Stores in memory with 6-hour TTL
6. **Periodic polling**: Checks again every 24 hours (or if cache expired)
7. **Expose in `/health`**: Returns `availableVersion` in response alongside `version`

#### Rate Limiting & Caching

- **GitHub API limit**: 60 requests/hour (unauthenticated)
- **Server strategy**: Daily checks (1 request/day) with 6-hour cache
- **Fallback**: If GitHub unreachable, keeps previous cached value (or server version if no previous check)
- **Location**: [cmd/browser-agent/main.go](../../cmd/browser-agent/main.go#L1542)

#### `/health` Response Example

```json
{
  "status": "ok",
  "version": "5.2.5",
  "availableVersion": "5.2.6",
  "logs": {
    "entries": 42,
    "maxEntries": 1000,
    "logFile": "/home/user/kaboom-logs.jsonl",
    "logFileSize": 102400
  },
  "buffers": { ... },
  "extension": { ... },
  "circuit": { ... }
}
```

**Note**: `availableVersion` field is only present if a version check has been completed. If the server hasn't checked GitHub yet, the field will be omitted.

### Extension Side (TypeScript)

#### Version Check Flow

1. **Extension polls `/health`** periodically as part of normal connectivity checks (no separate version-specific polling)
2. **Receives response** with `version` and optional `availableVersion` fields
3. **Calls `updateVersionFromHealth()`** with the response data
4. **Compares versions**: Uses semver comparison (e.g., "5.2.6" > "5.2.5")
5. **Updates state**: Sets `newVersionAvailable` flag based on comparison
6. **Updates badge**: Calls `updateVersionBadge()` to show/hide the "⬆" icon
7. **Updates tooltip**: Shows "Kaboom: New version available (v5.2.6)" or just "Kaboom"

#### Version Header

Every request to the server includes:

```
X-Kaboom-Extension-Version: 5.2.5
```

This header is automatically added to all API requests for diagnostics:
- `/logs` - Console logs
- `/websocket-events` - WebSocket events
- `/network-bodies` - Network requests
- `/network-waterfall` - Network waterfall
- `/enhanced-actions` - User actions
- `/settings` - Settings sync
- `/pending-queries` - Query polling
- `/extension-logs` - Extension logs
- `/api/extension-status` - Extension status
- `/performance-snapshots` - Performance metrics
- Plus other endpoints

The server logs version mismatches (e.g., when extension and server versions differ).

#### Badge Updates

```typescript
// When availableVersion > extensionVersion:
- Badge text: "⬆"
- Badge color: Blue (#0969da)
- Tooltip: "Kaboom: New version available (v5.2.6)"

// When equal or no update available:
- Badge cleared (no "⬆")
- Tooltip: "Kaboom"
```

## Configuration

### Server GitHub Check Interval

Edit [cmd/browser-agent/main.go:42-52](../../cmd/browser-agent/main.go#L42-L52):

```go
const (
	githubAPIURL         = "https://api.github.com/repos/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/latest"
	versionCheckCacheTTL = 6 * time.Hour    // Cache validity period
	versionCheckInterval = 24 * time.Hour   // Polling frequency
	httpClientTimeout    = 10 * time.Second // GitHub API timeout
)
```

**Recommended values**:
- `versionCheckInterval`: 24 hours (daily) - avoids GitHub rate limits
- `versionCheckCacheTTL`: 6 hours - allows manual refresh within 6h window
- `httpClientTimeout`: 10 seconds - prevents hanging on network issues

### Custom GitHub Repository

To use a different GitHub repository (fork), edit [cmd/browser-agent/main.go:47](../../cmd/browser-agent/main.go#L47):

```go
const githubAPIURL = "https://api.github.com/repos/YOUR-ORG/YOUR-REPO/releases/latest"
```

**Requirements**:
- GitHub releases must use semver tags: `vX.Y.Z` (e.g., `v5.2.6`)
- Tag format must include `v` prefix
- API endpoint must be public (no authentication)

### Setting Server Version at Build Time

```bash
# Build with custom version
go build -ldflags "-X main.version=5.2.6" ./cmd/browser-agent
```

If no `-ldflags` provided, defaults to `5.2.5` (see [cmd/browser-agent/main.go:30](../../cmd/browser-agent/main.go#L30)).

## Troubleshooting

### Badge Not Showing

**Symptoms**: GitHub has newer version but no "⬆" badge appears

**Solutions**:
1. Check GitHub has the newer version:
   ```bash
   curl -s https://api.github.com/repos/brennhill/kaboom-agentic-browser-devtools-mcp/releases/latest | jq .tag_name
   ```

2. Server hasn't checked GitHub yet
   - Server checks on startup, so restart the server: `killall kaboom && kaboom`
   - Or wait for next daily check

3. Check server version was fetched by extension
   - Open extension popup → should show server version under "Connected"
   - Or check DevTools Network tab → `/health` response should include `availableVersion` field

4. Check browser DevTools for errors
   - Open DevTools → Console → look for version check errors
   - DevTools → Network → click `/health` request → Response tab

5. Verify GitHub API is accessible from your network
   - Try: `curl https://api.github.com/repos/brennhill/kaboom-agentic-browser-devtools-mcp/releases/latest`

### GitHub API Unreachable

**Symptom**: Server logs show "GitHub version check failed"

```
[kaboom] GitHub version check failed: connection refused
```

**Causes**:
1. Network/firewall blocking GitHub API (`api.github.com`)
2. No internet connection
3. GitHub API down

**Solutions**:
- Check connectivity: `curl https://api.github.com`
- Check firewall allows `api.github.com`
- Retry next day (checks happen daily)
- Version checking is non-critical, doesn't block functionality

### Version Mismatch in Server Logs

**Symptom**: Server stderr shows version mismatch

```
[kaboom] Version mismatch: server=5.2.5 extension=5.2.6
```

**Why**: Extension and server have different versions (may be intentional)

**What to do**:
- This is informational only, not an error
- Server logs this for diagnostics (allows debugging version-related issues)
- If incompatibility issues occur, rebuild extension to match server version
- Check [RELEASE.md](./release.md) for upgrade notes

## API Reference

### Server Functions (Go)

#### `checkGitHubVersion()`

Fetches the latest version from GitHub. Called automatically on startup and every 24 hours.

**Behavior**:
- Checks cache first (6-hour TTL)
- Fetches GitHub API if cache expired
- Updates `availableVersion` global variable
- Non-blocking; silently fails if GitHub unreachable

**Implementation**: [cmd/browser-agent/main.go#L1542](../../cmd/browser-agent/main.go#L1542)

#### `startVersionCheckLoop()`

Starts the periodic version checking loop. Called once during server initialization.

**Behavior**:
- Immediately calls `checkGitHubVersion()` on startup
- Schedules periodic checks every 24 hours
- Runs in background goroutine

**Implementation**: [cmd/browser-agent/main.go#L1588](../../cmd/browser-agent/main.go#L1588)

#### `GET /health` Response

Returns server health including version information.

**Fields**:
- `status`: "ok"
- `version`: Current server version (e.g., "5.2.5")
- `availableVersion`: Latest GitHub release version if known (e.g., "5.2.6")
  - Omitted if no version check completed yet
  - Omitted if GitHub check failed
- `logs`: Log file statistics
- `buffers`: Capture buffer counts
- `extension`: Extension connection status
- `circuit`: Circuit breaker status

**Example**:
```json
{
  "status": "ok",
  "version": "5.2.5",
  "availableVersion": "5.2.6",
  ...
}
```

### Extension Functions (TypeScript)

#### `updateVersionFromHealth(healthResponse, debugLogFn?)`

Updates version information from server health response. Called when `/health` is received.

**Parameters**:
- `healthResponse`: Object with `version` and `availableVersion` fields
- `debugLogFn`: Optional debug logging function

**Example**:
```typescript
import { updateVersionFromHealth, debugLog } from 'kaboom';

updateVersionFromHealth({
  version: "5.2.5",
  availableVersion: "5.2.6"
}, debugLog);
```

#### `isNewVersionAvailable(): boolean`

Returns `true` if a newer version is available based on last `/health` response.

**Example**:
```typescript
if (isNewVersionAvailable()) {
  console.log("Update available!");
}
```

#### `getAvailableVersion(): string | null`

Returns the latest version from last `/health` response, or `null` if not yet fetched.

**Example**:
```typescript
const availVer = getAvailableVersion(); // "5.2.6" or null
```

#### `getUpdateInfo(): UpdateInfo`

Get update information for display in UI.

**Returns**:
```typescript
{
  available: boolean;           // True if update is available
  currentVersion: string;       // Current extension version
  availableVersion: string | null; // Latest available version
  downloadUrl: string;          // GitHub releases URL
}
```

**Example**:
```typescript
import { getUpdateInfo } from 'kaboom';

const info = getUpdateInfo();
if (info.available) {
  console.log(`Update: ${info.currentVersion} → ${info.availableVersion}`);
  console.log(`Get it: ${info.downloadUrl}`);
}
```

#### `updateVersionBadge(): void`

Manually update the extension badge based on current version state.

**Example**:
```typescript
import { updateVersionBadge } from 'kaboom';

updateVersionBadge();
```

#### `getExtensionVersion(): string`

Returns the extension version from `manifest.json`.

**Example**:
```typescript
import { getExtensionVersion } from 'kaboom';

console.log(getExtensionVersion()); // "5.2.5"
```

#### `resetVersionCheck(): void`

Reset version checking state to initial values. **For testing only.**

**Example**:
```typescript
import { resetVersionCheck } from 'kaboom';

resetVersionCheck();
```

## Files Involved

### Server (Go)
- [cmd/browser-agent/main.go](../../cmd/browser-agent/main.go) - GitHub checking, `/health` endpoint

### Extension (TypeScript)
- [src/lib/version.ts](../src/lib/version.ts) - Semver parsing & comparison
- [src/background/sync/version-check.ts](../src/background/sync/version-check.ts) - Version state management
- [src/background/sync/server.ts](../src/background/sync/server.ts) - HTTP header injection
- [connection-monitor.ts](../src/background/orchestration/connection-monitor.ts) - Integration with `/health` polling

## Data Flow Summary

```
┌─────────────────────────────────────────────────────────┐
│  Server Startup                                         │
│  1. Read version from manifest / ldflags                │
│  2. Call startVersionCheckLoop()                        │
│  3. Immediately call checkGitHubVersion()               │
│  4. Start periodic timer for daily checks               │
└──────────────────┬──────────────────────────────────────┘
                   │
                   ↓
         ┌─────────────────────┐
         │  GitHub API Check   │
         │  (6h cache TTL)     │
         │                     │
         │ If cached:          │
         │   Return immediately│
         │ Else:               │
         │   Fetch & cache     │
         └──────────┬──────────┘
                    │
                    ↓ Update availableVersion global var
         ┌─────────────────────┐
         │  /health endpoint   │
         │  Returns:           │
         │  - version          │
         │  - availableVersion │
         │  - logs, buffers... │
         └──────────┬──────────┘
                    │
                    ↓ Extension polls /health
         ┌──────────────────────────┐
         │  updateVersionFromHealth │
         │  - Compare versions      │
         │  - Set flags             │
         │  - Update badge          │
         │  - Show notification     │
         └──────────────────────────┘
                    │
                    ↓
         ┌──────────────────────────┐
         │  User sees "⬆" badge     │
         │  Clicks → Download link  │
         └──────────────────────────┘
```

## See Also

- [RELEASE.md](./release.md) - Version history and upgrade notes
- [README.md](../../../README.md) - Installation and setup
- [Plugin-server communications](./README.md) - Full protocol spec
