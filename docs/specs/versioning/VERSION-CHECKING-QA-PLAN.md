---
feature: Version Checking & Update Notifications
status: reference
last_reviewed: 2026-02-16
---

# QA Plan: Version Checking & Update Notifications

> How to test version checking functionality. Includes automated tests + human UAT.

## Testing Strategy

### Code Testing (Automated)

#### Unit Tests: Semver Parsing

**File**: `src/lib/version.ts`

- [ ] `parseVersion("5.2.5")` returns `{ major: 5, minor: 2, patch: 5 }`
- [ ] `parseVersion("0.0.1")` returns `{ major: 0, minor: 0, patch: 1 }`
- [ ] `parseVersion("10.20.300")` returns `{ major: 10, minor: 20, patch: 300 }`
- [ ] `parseVersion("5.2")` returns null (missing patch)
- [ ] `parseVersion("5.2.x")` returns null (invalid patch)
- [ ] `parseVersion("")` returns null (empty string)
- [ ] `parseVersion("5.2.5-beta")` returns `{ major: 5, minor: 2, patch: 5 }` (ignores suffix)

#### Unit Tests: Version Comparison

**File**: `src/lib/version.ts`

- [ ] `compareVersions("5.2.6", "5.2.5")` returns `1` (newer)
- [ ] `compareVersions("5.2.5", "5.2.6")` returns `-1` (older)
- [ ] `compareVersions("5.2.5", "5.2.5")` returns `0` (equal)
- [ ] `compareVersions("6.0.0", "5.9.9")` returns `1` (major version)
- [ ] `compareVersions("5.3.0", "5.2.9")` returns `1` (minor version)
- [ ] `compareVersions("5.2.10", "5.2.9")` returns `1` (patch version)
- [ ] `compareVersions("invalid", "5.2.5")` returns null (invalid input)

#### Unit Tests: Version Comparison Helpers

**File**: `src/lib/version.ts`

- [ ] `isVersionNewer("5.2.6", "5.2.5")` returns true
- [ ] `isVersionNewer("5.2.5", "5.2.6")` returns false
- [ ] `isVersionNewer("5.2.5", "5.2.5")` returns false
- [ ] `isVersionSameOrNewer("5.2.5", "5.2.5")` returns true
- [ ] `isVersionSameOrNewer("5.2.6", "5.2.5")` returns true
- [ ] `isVersionSameOrNewer("5.2.4", "5.2.5")` returns false

#### Integration Tests: Version Check State

**File**: `src/background/sync/version-check.ts`

**Setup**: Mock `chrome.runtime.getManifest()` to return version "5.2.5"

- [ ] `getExtensionVersion()` returns version from manifest
- [ ] `isNewVersionAvailable()` returns false initially
- [ ] `getLastServerVersion()` returns null initially
- [ ] After `checkServerVersion()` with server="5.2.6", `isNewVersionAvailable()` returns true
- [ ] After `checkServerVersion()` with server="5.2.5", `isNewVersionAvailable()` returns false
- [ ] `resetVersionCheck()` clears all state

#### Integration Tests: Version Check Rate Limiting

**File**: `src/background/sync/version-check.ts`

**Setup**: Mock fetch and Date.now()

- [ ] First call to `checkServerVersion()` fetches `/health`
- [ ] Second call within 6 hours returns without fetching (rate limited)
- [ ] Call after 6 hours fetches again (rate limit expired)
- [ ] Rate limit timer per server URL (different URLs have separate limits)

#### Integration Tests: HTTP Request Headers

**File**: `src/background/sync/server.ts`

**Setup**: Spy on fetch calls

- [ ] `sendLogsToServer()` includes `X-Kaboom-Extension-Version` header
- [ ] `postSettings()` includes version header
- [ ] `pollPendingQueries()` includes version header plus session/pilot headers
- [ ] All headers are preserved (no overwrites)
- [ ] Version header is exactly from `getExtensionVersion()`

#### Integration Tests: Badge Updates

**File**: `src/background/sync/version-check.ts`

**Setup**: Mock `chrome.action` APIs

- [ ] `updateVersionBadge()` with `newVersionAvailable=true` sets badge text "⬆"
- [ ] Badge background color is blue (#0969da)
- [ ] Badge tooltip shows version (e.g., "Kaboom: New version available (v5.2.6)")
- [ ] `updateVersionBadge()` with `newVersionAvailable=false` clears badge
- [ ] Tooltip without update available says "Kaboom"

#### Integration Tests: Error Handling

**File**: `src/background/sync/version-check.ts`

**Setup**: Mock fetch with various failures

- [ ] HTTP error (500) is caught, state preserved, no crash
- [ ] Network timeout is caught, state preserved, no crash
- [ ] Invalid JSON response is caught, state preserved, no crash
- [ ] Missing version field in response is handled gracefully
- [ ] Debug log called on all errors

#### Edge Case Tests: Version Comparison Logic

**File**: `src/lib/version.ts`

- [ ] "0.0.0" vs "0.0.1" (patch increment)
- [ ] "0.1.0" vs "0.0.9" (minor vs patch)
- [ ] "1.0.0" vs "0.9.9" (major increment)
- [ ] "10.0.0" vs "9.9.9" (double-digit major)
- [ ] Same version: "5.2.5" vs "5.2.5"

### Security/Compliance Testing

#### Data Leak Tests
- [ ] Version numbers never logged to public endpoints
- [ ] Version header not included in response body
- [ ] Sensitive data (auth tokens, user IDs) never mixed with version info

#### Permission Tests
- [ ] Extension can read own manifest.json (no elevated permissions needed)
- [ ] Extension can call `chrome.action.*` (existing permission)
- [ ] Version check doesn't require new permissions

---

## Human UAT Walkthrough

### Scenario 1: New Server Version Available (Happy Path)

**Setup**:
- Install extension v5.2.5
- Start server v5.2.6
- Configure extension to connect to server

**Steps**:
1. [ ] Open extension popup, verify "Connected"
2. [ ] Wait for version check (within 30 minutes, or modify interval to 10s for testing)
3. [ ] Check extension icon - verify blue "⬆" badge appears
4. [ ] Hover over badge - verify tooltip shows "Kaboom: New version available (v5.2.6)"
5. [ ] Open Chrome DevTools → Application → Local Storage
6. [ ] Verify no version state leaked (version checking is in-memory only)

**Expected Result**:
- Badge appears within polling interval
- Tooltip text is accurate
- No errors in console

**Verification**:
```bash
# In browser console (DevTools)
chrome.storage.local.get(['aiWebPilotEnabled'], result => console.log(result));
# Should show NO version state (it's in-memory)
```

---

### Scenario 2: Versions Match (Alternative Path)

**Setup**:
- Install extension v5.2.5
- Start server v5.2.5
- Configure extension to connect

**Steps**:
1. [ ] Open extension popup, verify "Connected"
2. [ ] Wait for version check (or trigger with modified interval)
3. [ ] Check extension icon - verify NO "⬆" badge
4. [ ] Hover over icon - verify tooltip says "Kaboom" (no version info)
5. [ ] Open DevTools Console, check for any errors

**Expected Result**:
- No badge shown
- Tooltip is generic
- No console errors

---

### Scenario 3: Extension Newer Than Server (Edge Case)

**Setup**:
- Install extension v5.3.0
- Start server v5.2.5
- Configure extension

**Steps**:
1. [ ] Open extension popup, verify "Connected"
2. [ ] Wait for version check
3. [ ] Check extension icon - verify NO badge
4. [ ] Verify no "update available" messages

**Expected Result**:
- No badge (extension is newer, not older)
- System works normally

---

### Scenario 4: Server Unreachable / Network Error (Error Path)

**Setup**:
- Install extension v5.2.5
- Server not running / unreachable
- Extension configured with invalid server URL

**Steps**:
1. [ ] Extension tries to connect (should fail gracefully)
2. [ ] Version check still runs (but fails silently)
3. [ ] Wait for version check interval
4. [ ] Open DevTools Console → Check for "Version check failed" debug log
5. [ ] Verify extension continues working (no crash)
6. [ ] Manually restart server
7. [ ] Wait for next version check
8. [ ] Verify extension reconnects and badge updates correctly

**Expected Result**:
- No UI crash or hang
- Debug log shows version check attempt
- Extension functionality unaffected
- Recovery happens automatically on next check

---

### Scenario 5: Multiple Extension Instances

**Setup**:
- Open same site in 2 tabs
- Both tabs running extension v5.2.5
- Server is v5.2.6

**Steps**:
1. [ ] Wait for version check
2. [ ] Both tabs should show badge independently
3. [ ] Each tab sends version header with requests

**Expected Result**:
- Both tabs get version check results
- Both update independently
- No conflicts or race conditions

---

### Scenario 6: Header Verification (Network Tab)

**Setup**:
- Open DevTools → Network tab
- Connect extension to server
- Send some requests (logs, settings, etc.)

**Steps**:
1. [ ] Make a request (e.g., POST /settings)
2. [ ] Click request in Network tab
3. [ ] Go to "Headers" → "Request Headers"
4. [ ] Scroll to find `X-Kaboom-Extension-Version`
5. [ ] Verify header value matches manifest version
6. [ ] Repeat for other endpoints (/logs, /pending-queries, etc.)

**Expected Result**:
- Header present on all requests
- Value is correct semver
- No auth/sensitive data in header

---

### Scenario 7: Continuous Integration / CI Build

**Setup**:
- Run full build pipeline

**Steps**:
1. [ ] `make compile-ts` - TypeScript compiles without errors
2. [ ] `go build ./cmd/browser-agent` - Server builds successfully
3. [ ] Check for version flag: `go build -ldflags "-X main.version=5.2.5"`
4. [ ] Verify version appears in `/health` response

**Expected Result**:
- No compilation errors
- No type errors
- Version embedded correctly

---

## Regression Testing

### Existing Features That Might Break

**Connection Status**:
- Version check added to `checkConnectionAndUpdate()`
- Verify: Connection status still updates correctly, badge still shows connection status

**Polling Loops**:
- New version check loop added alongside existing loops
- Verify: Other polling (query, settings, logs) unaffected
- Verify: `stopAllPolling()` stops all loops including version check

**HTTP Headers**:
- New header added to request headers
- Verify: Existing headers still present (session, pilot, content-type, etc.)
- Verify: Backward compat - server still accepts requests without header

**Storage**:
- No new storage used (version checking is in-memory only)
- Verify: `chrome.storage.local` unaffected
- Verify: Settings/pilot state unchanged

---

## Performance/Load Testing

### Version Check Performance

**Test**: Single version check operation
```
Setup: Mock /health endpoint with realistic response
Action: Call checkServerVersion()
Measure: Time taken
Expectation: < 100ms total (fetch + parse + state update)
```

**Test**: Polling loop overhead
```
Setup: Enable version check polling
Measure: CPU/memory during 30-minute cycle
Expectation: < 1% CPU, < 1MB memory per check
```

**Test**: Concurrent requests with version header
```
Setup: Extension sending logs + settings + queries simultaneously
Measure: Network waterfall, header overhead
Expectation: Header adds < 100 bytes per request, < 0.1ms overhead
```

### Badge Update Performance
```
Setup: Trigger badge update (updateVersionBadge())
Measure: Time to reflect in UI
Expectation: < 50ms (synchronous Chrome API)
```

---

## Manual Testing Checklist

### Pre-Release Checklist

**Code Quality**:
- [ ] TypeScript compiles without warnings
- [ ] Go builds without errors
- [ ] No console errors in DevTools
- [ ] All unit tests pass (when added)
- [ ] All integration tests pass (when added)

**Version Functionality**:
- [ ] Badge appears when server > extension
- [ ] Badge disappears when server == extension
- [ ] Header present on all requests
- [ ] Server logs version mismatch (check stderr)
- [ ] Rate limiting works (no spam requests)

**Edge Cases**:
- [ ] Network failure handled gracefully
- [ ] Invalid version format handled
- [ ] Missing /health endpoint handled
- [ ] Extension ahead of server works
- [ ] Versions don't match but system functional

**Backward Compatibility**:
- [ ] Old extension without header still works
- [ ] Old server without version in /health still works
- [ ] Graceful degradation if /health unavailable

**Browser Compatibility**:
- [ ] Chrome 90+ (manifest v3)
- [ ] Badge APIs available
- [ ] Network requests work

### Release Validation

- [ ] Version numbers correct in manifest.json and server build
- [ ] Documentation up to date
- [ ] No breaking changes to APIs
- [ ] Telemetry unaffected
- [ ] Performance acceptable

---

## Notes

- Interval tuning: For testing, change `VERSION_CHECK_INTERVAL_MS` in `src/background/polling.ts:28` from `30 * 60 * 1000` to `10 * 1000` for 10-second checks
- Mock `/health` responses in test: Use `fetchMock` or Sinon to stub `/health` endpoint
- Badge testing: Mock `chrome.action` APIs to verify calls without UI
- Rate limiting test: Use `jest.useFakeTimers()` to advance time in tests
