---
feature: Version Checking & Update Notifications
status: shipped
tool: configure
mode: background
version: 0.7.12
last_reviewed: 2026-02-16
---

# Product Spec: Version Checking & Update Notifications

## Problem Statement

Users don't know when a newer version of Kaboom is available. They may be running outdated software with bugs or missing features. There's no visibility into version compatibility between the extension and server.

## Solution

Implement automatic periodic version checking where:
1. Extension checks if server has a newer version (every 30 minutes)
2. If newer version available, show visual badge ("⬆") on extension icon
3. Extension sends its version with every request, allowing server to log mismatches
4. Badge disappears when versions match

## Requirements

### Must Have
- Extension reads its version from `manifest.json`
- Extension periodically polls `/health` endpoint for server version
- Visual badge ("⬆") appears on extension icon when new version available
- Badge includes tooltip showing available version (e.g., "Kaboom: New version available (v5.2.6)")
- All extension requests include `X-Kaboom-Extension-Version` header
- Server logs version mismatches to stderr for diagnostics
- Version comparison uses semantic versioning (X.Y.Z format)
- Rate limiting prevents excessive polling (30-minute intervals)

### Nice to Have
- Clickable badge linking to download/release notes
- Persistent storage of last checked version
- Version compatibility warnings (e.g., major version mismatch)

### Out of Scope
- Auto-update functionality (manual update via Web Store/binary replacement)
- Minimum version enforcement (server blocking old extensions)
- Pre-release version handling (alpha, beta, rc)
- Non-semver versions (git hashes, custom formats)

## Success Criteria

✅ Extension badge appears when server version > extension version
✅ Badge disappears when versions match
✅ All API requests include version header
✅ Server logs version mismatches
✅ Polling respects rate limiting (no spam)
✅ Graceful handling when `/health` unavailable
✅ TypeScript compilation passes
✅ Go build succeeds

## User Workflow

### Happy Path: New Version Available

1. User has Kaboom extension v5.2.5 installed
2. Server is updated to v5.2.6
3. Extension connects to server, version check starts
4. Within 30 minutes, periodic check runs:
   - Polls `/health` endpoint
   - Receives `{ version: "5.2.6", ... }`
   - Compares: 5.2.6 > 5.2.5 ✓
   - Updates state: `newVersionAvailable = true`
5. Badge "⬆" appears on extension icon (blue background)
6. Hovering over badge shows: "Kaboom: New version available (v5.2.6)"
7. User clicks badge or visits Web Store to update
8. User installs new extension (v5.2.6)
9. Next version check: versions match, badge disappears

### Alternative Path: Versions Match

1. User has extension v5.2.5, server is v5.2.5
2. Version check runs normally
3. Comparison: 5.2.5 == 5.2.5 (no newer version)
4. State: `newVersionAvailable = false`
5. No badge shown
6. Extension title: "Kaboom"

### Error Path: Server Unreachable

1. Server is down or unreachable
2. Version check fetch fails
3. Error is logged (debug log only, no user-facing error)
4. Current state preserved (badge unchanged)
5. Check retried on next interval (no disruption to functionality)

## Examples

### Example 1: Patch Update Available
```
Server: 5.2.6
Extension: 5.2.5
Badge: ⬆ (shows v5.2.6)
Action: User downloads patch via Web Store
```

### Example 2: Minor Feature Update
```
Server: 5.3.0
Extension: 5.2.5
Badge: ⬆ (shows v5.3.0)
Action: User updates for new features
```

### Example 3: Extension Ahead of Server
```
Server: 5.2.5
Extension: 5.3.0
Badge: None (no newer version)
Action: User can continue using new extension with older server
```

## Metrics & Monitoring

- Server logs: Version mismatches in stderr for diagnostics
- Extension logs: Version check results in debug log
- Badge state: Visible indicator of update availability
- Check frequency: 30-minute intervals (configurable)

---

## Notes & References

- Uses semantic versioning (semver) X.Y.Z format
- Similar to browser extension auto-update, but user-initiated
- Version info already available in `/health` endpoint
- No breaking changes to existing APIs
- Backward compatible: old extensions without header still work
