---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Agent Assignment: manage_state

**Branch:** `feature/pilot-state`
**Worktree:** `../kaboom-pilot-state`
**Priority:** P4 Phase 2 (parallel — requires Phase 1 complete)
**Dependency:** Merge `feature/pilot-toggle` first

---

## Objective

Implement `manage_state` MCP tool that saves and restores browser state (localStorage, sessionStorage, cookies) for faster reproduction workflows.

---

## Deliverables

### 1. Inject.js Handler

**File:** `extension/inject.js`

Add in AI Web Pilot section:
```javascript
// ============================================================================
// AI WEB PILOT: STATE MANAGEMENT
// ============================================================================

function captureState() {
  const state = {
    url: window.location.href,
    timestamp: Date.now(),
    localStorage: {},
    sessionStorage: {},
    cookies: document.cookie
  }

  for (let i = 0; i < localStorage.length; i++) {
    const key = localStorage.key(i)
    state.localStorage[key] = localStorage.getItem(key)
  }

  for (let i = 0; i < sessionStorage.length; i++) {
    const key = sessionStorage.key(i)
    state.sessionStorage[key] = sessionStorage.getItem(key)
  }

  return state
}

function restoreState(state, includeUrl = true) {
  // Clear existing
  localStorage.clear()
  sessionStorage.clear()

  // Restore localStorage
  for (const [key, value] of Object.entries(state.localStorage || {})) {
    localStorage.setItem(key, value)
  }

  // Restore sessionStorage
  for (const [key, value] of Object.entries(state.sessionStorage || {})) {
    sessionStorage.setItem(key, value)
  }

  // Restore cookies (clear then set)
  document.cookie.split(';').forEach(c => {
    const name = c.split('=')[0].trim()
    if (name) {
      document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`
    }
  })

  if (state.cookies) {
    state.cookies.split(';').forEach(c => {
      document.cookie = c.trim()
    })
  }

  const restored = {
    localStorage: Object.keys(state.localStorage || {}).length,
    sessionStorage: Object.keys(state.sessionStorage || {}).length,
    cookies: (state.cookies || '').split(';').filter(c => c.trim()).length
  }

  // Navigate if requested
  if (includeUrl && state.url && state.url !== window.location.href) {
    window.location.href = state.url
  }

  return { success: true, restored }
}
```

### 2. Background Script Storage

**File:** `extension/background.js`

Snapshots stored in `chrome.storage.local`:
```javascript
const SNAPSHOT_KEY = 'kaboom_state_snapshots'

async function saveStateSnapshot(name, state) {
  const { [SNAPSHOT_KEY]: snapshots = {} } = await chrome.storage.local.get(SNAPSHOT_KEY)
  snapshots[name] = {
    ...state,
    name,
    size_bytes: JSON.stringify(state).length
  }
  await chrome.storage.local.set({ [SNAPSHOT_KEY]: snapshots })
  return { success: true, snapshot_name: name, size_bytes: snapshots[name].size_bytes }
}

async function loadStateSnapshot(name) {
  const { [SNAPSHOT_KEY]: snapshots = {} } = await chrome.storage.local.get(SNAPSHOT_KEY)
  return snapshots[name] || null
}

async function listStateSnapshots() {
  const { [SNAPSHOT_KEY]: snapshots = {} } = await chrome.storage.local.get(SNAPSHOT_KEY)
  return Object.values(snapshots).map(s => ({
    name: s.name,
    url: s.url,
    timestamp: s.timestamp,
    size_bytes: s.size_bytes
  }))
}

async function deleteStateSnapshot(name) {
  const { [SNAPSHOT_KEY]: snapshots = {} } = await chrome.storage.local.get(SNAPSHOT_KEY)
  delete snapshots[name]
  await chrome.storage.local.set({ [SNAPSHOT_KEY]: snapshots })
  return { success: true, deleted: name }
}
```

### 3. MCP Tool Handler

**File:** `cmd/browser-agent/pilot.go`

```go
func (v *Capture) handleManageState(params map[string]any) (any, error) {
    action, _ := params["action"].(string)
    snapshotName, _ := params["snapshot_name"].(string)
    includeUrl := true
    if iu, ok := params["include_url"].(bool); ok {
        includeUrl = iu
    }

    switch action {
    case "save":
        if snapshotName == "" {
            return nil, errors.New("snapshot_name required for save")
        }
        return v.sendPilotCommand("state_save", map[string]any{"snapshot_name": snapshotName})

    case "load":
        if snapshotName == "" {
            return nil, errors.New("snapshot_name required for load")
        }
        return v.sendPilotCommand("state_load", map[string]any{
            "snapshot_name": snapshotName,
            "include_url": includeUrl,
        })

    case "list":
        return v.sendPilotCommand("state_list", nil)

    case "delete":
        if snapshotName == "" {
            return nil, errors.New("snapshot_name required for delete")
        }
        return v.sendPilotCommand("state_delete", map[string]any{"snapshot_name": snapshotName})

    default:
        return nil, errors.New("action must be save|load|list|delete")
    }
}
```

---

## Tests

**File:** `extension-tests/pilot-state.test.js` (new)

1. Save captures localStorage, sessionStorage, cookies
2. Load restores all three storage types
3. Load clears existing state before restoring
4. List returns snapshot metadata
5. Delete removes snapshot
6. Round-trip: save → clear → load → verify restored
7. include_url=false skips navigation

---

## Verification

```bash
node --test extension-tests/pilot-state.test.js
go test -v ./cmd/browser-agent/ -run ManageState
```

---

## Files Modified

| File | Change |
|------|--------|
| `extension/inject.js` | `captureState()`, `restoreState()` |
| `extension/background.js` | Snapshot CRUD in chrome.storage.local |
| `extension/content.js` | Forward state messages |
| `cmd/browser-agent/pilot.go` | `handleManageState()` |
| `extension-tests/pilot-state.test.js` | New file |
