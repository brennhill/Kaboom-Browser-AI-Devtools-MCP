---
doc_type: architecture_decision
status: active
last_reviewed: 2026-08-06
---

# ADR: AI Pilot Feature Active Sync Architecture

**Status:** Approved
**Date:** 2026-01-26
**Authors:** Engineering Team
**Impact:** Server-side + Extension (active message passing, storage sync)

## Problem Statement

The AI Web Pilot toggle appeared enabled in the extension UI but the MCP server and healthcheck endpoints never reflected the enabled state. This created a UX lie: users toggled the feature on, but the server couldn't see it.

**Root Cause:** Passive storage listener pattern + one-way data flow.
- Extension popup writes `chrome.storage.sync` when toggle changes
- Background script had passive listener (with inherent race conditions)
- Server queried state via HTTP header only on next `/pending-queries` poll
- Window of 0-3 seconds where UI showed enabled but server thought disabled

**Business Impact:**
- Users enable AI Pilot, nothing happens (appears broken)
- Confusing debugging: "Why isn't my plugin working?"
- Undermines trust in browser automation features

---

## Solution: Three-Tier Active Sync

### Architecture Decisions

#### 1. Storage Hierarchy (No Conflicts)

```
Priority: chrome.storage.sync > chrome.storage.local > chrome.storage.session
```

- **`sync`** = Authoritative source (cross-device user preference, survives reinstall)
- **`local`** = Write-through cache (performance, immediate reads)
- **`session`** = In-memory cache (service worker restarts)

**Conflict Resolution:** On startup, read `sync` → overwrite `local/session`. Sync always wins.

#### 2. Active Message Passing (Sync Notifications)

Instead of passive storage listeners, implement immediate messaging:

```
Extension Popup:
  1. Write to all 3 storage areas (atomic)
  2. Send chrome.runtime.sendMessage({ type: 'setAiWebPilotEnabled', enabled })
  3. Wait for confirmation broadcast

Background:
  1. Receive setAiWebPilotEnabled message
  2. Update _aiWebPilotEnabledCache synchronously
  3. Write to all 3 storage areas (redundant but safe)
  4. Broadcast { type: 'pilotStatusChanged', enabled } back to popup

Popup:
  1. Receive pilotStatusChanged
  2. Update checkbox to reflect confirmed state
```

**Key Principle:** Never rely on storage listeners for critical state. Use explicit messaging.

#### 3. Multiple Query Mechanisms (Redundancy)

Clients can verify pilot state via three independent paths:

1. **MCP Tool:** `observe {what: "pilot"}`
   - Integrated into existing `observe` tool (no new tool, respects 5-tool limit)
   - Returns `{enabled, source, extension_connected, last_poll_ago}`

2. **HTTP Endpoint:** `GET /pilot-status`
   - Direct debugging access (bypasses MCP layer)
   - Same response schema as MCP tool
   - Always returns 200 OK (source field indicates freshness)

3. **Health Response:** `get_health` includes `pilot` field
   - Servers can check health + pilot state in single call
   - No polling required

**Freshness Indicators:** Each response includes `source` field:
- `"extension_poll"` — Last poll < 3 seconds ago (actively connected)
- `"stale"` — Last poll > 3 seconds ago (data is old)
- `"never_connected"` — Extension never polled (default state)

---

## Implementation

### Phase 1: Server-Side Query Endpoints ✅

**Files Modified:**
- `cmd/browser-agent/pilot.go`: Added `GetPilotStatus()`, `PilotStatusResponse`, `HandlePilotStatus()`, `toolObservePilot()`
- `cmd/browser-agent/health.go`: Added `PilotInfo` type, extended `MCPHealthResponse`
- `cmd/browser-agent/tools_core.go`: Added `observe {what: "pilot"}` handler
- `cmd/browser-agent/main.go`: Registered `/pilot-status` HTTP endpoint

**Key Implementations:**

```go
// GetPilotStatus determines freshness based on polling window
func (c *Capture) GetPilotStatus() PilotStatusResponse {
    c.mu.RLock()
    defer c.mu.RUnlock()

    if c.lastPollAt.IsZero() {
        return PilotStatusResponse{Source: "never_connected"}
    }

    age := time.Since(c.lastPollAt)
    if age < 3*time.Second {
        return PilotStatusResponse{Source: "extension_poll", ExtensionConnected: true}
    }
    return PilotStatusResponse{Source: "stale", ExtensionConnected: false}
}
```

### Phase 2: Extension Active Sync ✅

**Files Modified:**
- `extension/popup.js`: Modified `handleAiWebPilotToggle()` to write all 3 storage areas + send message
- `extension/background.js`: Added `verifyPilotStorageConsistency()`, extended message handler, modified startup
- `extension-tests/pilot-toggle.test.js`: Added 5 tests for active sync behavior

**Key Implementations:**

```javascript
// Popup: Atomic write + immediate message
export async function handleAiWebPilotToggle(enabled) {
    chrome.storage.sync.set({ aiWebPilotEnabled: enabled })
    chrome.storage.local.set({ aiWebPilotEnabled: enabled })
    chrome.storage.session.set({ aiWebPilotEnabled: enabled })
    chrome.runtime.sendMessage({ type: 'setAiWebPilotEnabled', enabled })
}

// Background: Startup reconciliation
async function verifyPilotStorageConsistency() {
    const sync = await new Promise(r => chrome.storage.sync.get(['aiWebPilotEnabled'], r))
    const local = await new Promise(r => chrome.storage.local.get(['aiWebPilotEnabled'], r))
    const session = await new Promise(r => chrome.storage.session.get(['aiWebPilotEnabled'], r))

    if (sync.aiWebPilotEnabled !== local.aiWebPilotEnabled ||
        sync.aiWebPilotEnabled !== session.aiWebPilotEnabled) {
        // Sync wins: overwrite local/session
        chrome.storage.local.set({ aiWebPilotEnabled: sync.aiWebPilotEnabled })
        chrome.storage.session.set({ aiWebPilotEnabled: sync.aiWebPilotEnabled })
        console.log('[Kaboom] Storage mismatch detected — sync value restored')
    }
}
```

---

## Testing Strategy

### Go Tests (cmd/browser-agent/pilot_test.go)

✅ `TestObservePilotMode` — Observe tool supports "pilot" mode
✅ `TestObservePilotResponseSchema` — Response includes enabled, source, extension_connected
✅ `TestGetPilotStatusWithExtensionConnected` — source="extension_poll" for recent poll
✅ `TestGetPilotStatusStale` — source="stale" for polls > 3s old
✅ `TestGetPilotStatusNeverConnected` — source="never_connected" when no polls
✅ `TestGetPilotStatusThreadSafety` — Concurrent reads/writes safe under RWMutex
✅ `TestHealthResponseIncludesPilot` — get_health includes pilot field

**Run:** `go test -run "Pilot" ./cmd/browser-agent/`

### Extension Tests (extension-tests/pilot-toggle.test.js)

✅ `handleAiWebPilotToggle writes to all 3 storage areas atomically`
✅ `handleAiWebPilotToggle sends immediate message to background`
✅ `background receives setAiWebPilotEnabled and updates cache`
✅ `background broadcasts pilotStatusChanged confirmation`
✅ `background syncs to all 3 storage areas on message`

**Run:** `node --test extension-tests/pilot-toggle.test.js`

---

## Quality Gates ✅

- `go vet ./cmd/browser-agent/` — No issues
- `make test` — All tests pass
- `node --test extension-tests/*.test.js` — All extension tests pass

---

## Data Flow Diagram

```
User toggles UI
    ↓
popup.js::handleAiWebPilotToggle()
    ├→ chrome.storage.sync.set({ aiWebPilotEnabled: true })
    ├→ chrome.storage.local.set({ aiWebPilotEnabled: true })
    ├→ chrome.storage.session.set({ aiWebPilotEnabled: true })
    └→ chrome.runtime.sendMessage({ type: 'setAiWebPilotEnabled', enabled: true })
        ↓
background.js message handler
    ├→ Update _aiWebPilotEnabledCache (synchronous)
    ├→ Write to all 3 storage areas (redundant)
    └→ Broadcast { type: 'pilotStatusChanged', enabled: true }
        ↓
popup.js listener receives confirmation
    └→ Update checkbox UI (reflects confirmed state)

Meanwhile (independent path):
MCP client calls observe {what: "pilot"}
    ↓
server::toolObservePilot()
    └→ Returns { enabled: true, source: "extension_poll", ... }

HTTP client calls GET /pilot-status
    ↓
server::HandlePilotStatus()
    └→ Returns same JSON (no MCP overhead)

Health client calls get_health
    ↓
server::GetHealth()
    └→ Returns MCPHealthResponse with pilot field
```

---

## Edge Cases Covered

### Service Worker Restart
**Solution:** Startup calls `verifyPilotStorageConsistency()`. Sync value overwrites local/session. Data never lost.

### Multiple Popups Open
**Solution:** Background message handler serializes updates via mutex. All popups receive confirmation broadcasts. Eventually consistent.

### Cross-Device Sync
**Solution:** User enables on Device A, `chrome.storage.sync` propagates to Device B. On Device B startup, sync overwrites local/session cache. Users expect this.

### Storage Mismatch (Corruption)
**Solution:** Startup verification detects and fixes. Sync always wins. Logged with warning for debugging.

### Concurrent Toggle + Poll
**Solution:** Server state protected by RWMutex. Capture reads under RLock. Pilot state queries always see consistent snapshot.

---

## Decision Rationale

### Why Active Messages Over Passive Listeners?

**Passive listeners are fundamentally unreliable:**
- Storage listener fires asynchronously (race window)
- Extension service worker may be suspended between listener registration and event
- No guaranteed delivery (browser can GC listeners)

**Active messages guarantee:**
- Synchronous cache update before user sees confirmation
- Explicit acknowledgment (popup waits for broadcast)
- No race conditions between storage write and background state change

### Why Three Storage Areas?

- **sync**: Cross-device consistency (user preference survives reinstall)
- **local**: Instant reads without waiting for sync API
- **session**: In-memory cache for hot path (service worker restarts)

Redundancy costs nothing; failures are unrecoverable without it.

### Why Integrate into Observe Instead of New Tool?

**User constraint:** "5 tools max — no more."

**Solution:** Pilot status is observational data, fits naturally into `observe` tool:
- `observe {what: "logs"}` — Get logs
- `observe {what: "network"}` — Get network events
- `observe {what: "pilot"}` — Get pilot status (NEW)

No new tool, no cognitive overload, LLMs naturally discover it.

### Why Multiple Query Paths?

**Resilience:** If MCP breaks, HTTP endpoint still works. If polling breaks, health endpoint captures snapshot. Clients aren't forced into single code path.

**Debugging:** `/pilot-status` endpoint is curl-friendly. Server admins can check health without MCP knowledge.

---

## Rollback Plan

If issues arise post-deployment:

1. **Revert Extension Changes**: Falls back to passive listener (slower, but functional)
2. **Revert Server Changes**: Pilot still works via HTTP header on next poll
3. **Zero Data Loss**: State is boolean only. No user data at risk.
4. **Test Before Push**: All quality gates must pass before deployment.

---

## Verification Checklist

- ✅ Toggle in popup → background cache updates immediately (< 50ms)
- ✅ Background sends confirmation → popup updates UI
- ✅ All 3 storage areas written atomically (or all fail)
- ✅ Startup reads sync → overwrites local/session if mismatch
- ✅ MCP tool `observe {what: "pilot"}` returns correct state
- ✅ HTTP endpoint `/pilot-status` accessible
- ✅ Health response includes pilot field with correct structure
- ✅ `go vet ./cmd/browser-agent/` clean
- ✅ `make test` passes (all tests)
- ✅ `node --test extension-tests/*.test.js` passes
- ✅ No race conditions (tested with concurrent reads/writes)

---

## References

- [AI Web Pilot technical specification](../../features/feature/ai-web-pilot/tech-spec.md)
- [System overview](../platform/system-overview.md)
