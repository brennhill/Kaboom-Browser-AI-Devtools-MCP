---
feature: Version Checking & Update Notifications
status: shipped
last_reviewed: 2026-02-16
---

# Tech Spec: Version Checking & Update Notifications

> Plain language only. Describes HOW the implementation works.

## Architecture Overview

The version checking system has two independent components that work together:

**Extension-to-Server**: Extension includes its version in every request header
**Server-to-Extension**: Extension periodically polls server version and updates badge

Both operate asynchronously without blocking main functionality.

## Key Components

### Extension Side

**`src/lib/version.ts`** - Semver Utilities
- Pure functions for parsing and comparing semantic versions
- No dependencies, no side effects
- Supports X.Y.Z format (e.g., "5.2.5")
- Functions: `parseVersion()`, `compareVersions()`, `isVersionNewer()`

**`src/background/sync/version-check.ts`** - Version Check Logic
- Manages version check state (lastCheckedVersion, newVersionAvailable flag)
- `checkServerVersion()`: Polls `/health`, updates state
- `updateVersionBadge()`: Updates extension badge based on state
- `getExtensionVersion()`: Reads from `manifest.json`
- Rate limiting: Only checks every 6 hours per server

**`src/background/polling.ts`** - Polling Loop Management
- `startVersionCheck()`: Starts 30-minute polling interval
- `stopVersionCheck()`: Stops polling when disconnected
- Calls version check function immediately on start, then every 30 minutes

**`src/background/sync/server.ts`** - HTTP Request Headers
- `getRequestHeaders()`: Injects `X-Kaboom-Extension-Version` header
- Applied to all API requests (logs, WS events, actions, etc.)
- 10+ endpoints updated

**`src/background/orchestration/connection-monitor.ts`** - Integration
- Starts version check in `checkConnectionAndUpdate()`
- Uses dynamic import to avoid circular dependencies
- Exports public API for version functions

### Server Side

**`cmd/browser-agent/main.go`** - Version Header Processing
- Extracts `X-Kaboom-Extension-Version` from request headers
- Compares with server version
- Logs mismatch to stderr for diagnostics
- Already exposes version in `/health` endpoint

## Data Flows

### Flow 1: Periodic Version Check (Extension → Server)

```
Extension connection established
    ↓
checkConnectionAndUpdate() called
    ↓
startVersionCheck() starts polling
    ↓
Every 30 minutes:
  1. checkServerVersion() called
  2. Rate limit check: Skip if checked within 6 hours
  3. Fetch GET /health
  4. Extract version from response
  5. Compare: server version vs extension version
  6. updateVersionBadge() updates UI
  7. Log result to debug log
```

### Flow 2: Every API Request (Extension → Server)

```
Extension makes any API request (POST /logs, POST /settings, etc.)
    ↓
getRequestHeaders() called
    ↓
Inject X-Kaboom-Extension-Version header
    ↓
Send request with header
    ↓
Server receives request
    ↓
Extract extension version header
    ↓
Compare with server version
    ↓
Log mismatch if versions differ (stderr)
```

### Flow 3: Version Comparison

```
Server version: "5.2.6"
Extension version: "5.2.5"
    ↓
parseVersion("5.2.6") → { major: 5, minor: 2, patch: 6 }
parseVersion("5.2.5") → { major: 5, minor: 2, patch: 5 }
    ↓
compareVersions() logic:
  - Major equal? (5 == 5) ✓
  - Minor equal? (2 == 2) ✓
  - Patch compare? (6 > 5) → return 1 (newer)
    ↓
isVersionNewer(server, extension) → true
    ↓
updateVersionBadge():
  - Set badge text: "⬆"
  - Set badge color: blue (#0969da)
  - Set tooltip: "Kaboom: New version available (v5.2.6)"
```

## Implementation Strategy

### Approach: Modular, Non-Blocking

1. **Separation of Concerns**
   - Version utilities in `version.ts` (pure, testable)
   - Version check logic in `version-check.ts` (state management)
   - Polling in `polling.ts` (interval management)
   - Headers in `server.ts` (HTTP integration)

2. **Integration Pattern**
   - Lazy dynamic import in `index.ts` to avoid circular deps
   - Plugs into existing polling loop infrastructure
   - Uses existing `debugLog` for diagnostics

3. **No Breaking Changes**
   - Extension header is optional (old servers ignore it)
   - Version check doesn't block main connection
   - Badge update is idempotent

## Edge Cases & Assumptions

### Edge Case 1: Server Unreachable
**Description**: `/health` endpoint fails or server is down
**Handling**: Silently fail, preserve current state, retry next interval

### Edge Case 2: Invalid Version Format
**Description**: Server returns `{ version: "5.2-beta" }` or null
**Handling**: `parseVersion()` returns null, comparison skips, no badge update

### Edge Case 3: Rapid Polling
**Description**: Multiple version checks requested in quick succession
**Handling**: Rate limit check (6-hour interval) prevents redundant fetches

### Edge Case 4: Extension Ahead of Server
**Description**: Extension v5.3.0, Server v5.2.5
**Handling**: `isVersionNewer()` returns false, no badge shown

### Edge Case 5: Badge Already Shown
**Description**: User clicks extension icon while badge is showing
**Handling**: Tooltip still visible, badge remains until versions match

### Assumption 1
Extension can reach `/health` endpoint (no CORS/firewall issues)

### Assumption 2
Server version is properly set at build time via `-ldflags`

### Assumption 3
Version format is strictly X.Y.Z (enforced by regex)

## Risks & Mitigations

### Risk 1: Rate Limit Too Aggressive
**Description**: 6-hour server-side rate limit causes stale version info
**Mitigation**: 30-minute extension polling interval is reasonable trade-off; users typically keep browser open

### Risk 2: Badge Conflicts with Error Count
**Description**: Badge might show both "⬆" and error count
**Mitigation**: Badge only shows "⬆" when update available, error badge shown otherwise (mutually exclusive)

### Risk 3: Header Stripping by Proxy
**Description**: Custom proxy strips or modifies `X-Kaboom-*` headers
**Mitigation**: Header is informational only (not required for functionality), graceful degradation

### Risk 4: Performance Impact on Startup
**Description**: Dynamic import of version-check adds latency
**Mitigation**: Lazy import happens after connection established, no critical path impact

### Risk 5: Memory Leak from Interval
**Description**: `setInterval` not cleared properly on extension reload
**Mitigation**: `stopVersionCheck()` called in `stopAllPolling()` cleanup

## Dependencies

- Existing `/health` endpoint (already present)
- Existing polling loop infrastructure
- Chrome API: `chrome.runtime.getManifest()`, `chrome.action.setBadgeText()`
- Go standard library (fmt, strings, time)

## Performance Considerations

### Extension Side
- **Polling overhead**: One HTTP request every 30 minutes (negligible)
- **CPU usage**: Semver comparison is O(1) (3 integer comparisons)
- **Memory**: ~100 bytes state (two strings + one boolean)
- **Badge updates**: <1ms (synchronous Chrome API calls)

### Server Side
- **Header extraction**: O(1) string lookup in request headers
- **Logging**: Conditional write to stderr only on mismatch
- **No database queries or heavy lifting**

### Total Impact: Negligible
- No interference with extension's main telemetry capture
- No impact on server's MCP tool execution
- Gracefully degrades if `/health` is slow

## Security Considerations

### Data Exposure
- Version numbers are non-sensitive
- No auth tokens or personal data in headers
- Header value is exactly version string from manifest

### Attack Surface
- Header injection: Limited (version string is from manifest, not user input)
- Spoofing: Server can't be tricked into thinking extension is newer (comparison is one-way)
- DoS: Rate limiting prevents version check spam

### Privacy
- Version check only reaches own server (localhost or configured URL)
- No third-party requests
- No analytics or tracking

## Testing Surface Areas

### Unit Tests (Pure Functions)
- `parseVersion()`: Valid/invalid formats, edge cases
- `compareVersions()`: All comparison outcomes (-1, 0, 1)
- `isVersionNewer()`: Major/minor/patch differences

### Integration Tests
- `checkServerVersion()`: Mocked `/health`, state updates
- Badge update: State change triggers badge update
- Rate limiting: Multiple calls within limit don't fetch again

### End-to-End Tests
- Extension sends header on every request
- Server receives and logs header
- Badge appears/disappears correctly in extension popup

### Edge Cases
- Missing version in `/health` response
- Network timeout on `/health` request
- Invalid semver in response
- Extension header missing (backward compat)
