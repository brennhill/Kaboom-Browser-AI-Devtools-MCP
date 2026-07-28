---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Sync Endpoint Protocol Specification

## Overview

The `/sync` endpoint consolidates multiple polling loops into a single bidirectional sync. This simplifies the extension-server communication, makes connection state binary (success/fail), and is self-healing for MV3 service worker lifecycle.

## Current Architecture (Before)

| Loop | Interval | Direction | Endpoint |
|------|----------|-----------|----------|
| Query polling | 1s | GET | /pending-queries |
| Settings heartbeat | 2s | POST | /settings |
| Status ping | 30s | POST | /api/extension-status |
| Extension logs | 5s | POST | /extension-logs |
| Waterfall posting | 10s | POST | /network-waterfall |
| Version check | 24h | GET | GitHub API |

## New Architecture (After)

| Loop | Interval | Direction | Endpoint |
|------|----------|-----------|----------|
| **Sync** | 1s | POST | **/sync** |
| Waterfall posting | 10s | POST | /network-waterfall |
| Version check | 24h | GET | GitHub API |

## Protocol

### Request

```typescript
POST /sync
Content-Type: application/json
X-Kaboom-Extension-Version: 5.6.0

{
  // Session identification
  "session_id": "ext_abc123",

  // Extension settings (replaces /settings POST)
  "settings": {
    "pilot_enabled": true,
    "tracking_enabled": true,
    "tracked_tab_id": 123,
    "tracked_tab_url": "https://example.com",
    "capture_logs": true,
    "capture_network": true,
    "capture_websocket": true,
    "capture_actions": true
  },

  // Extension logs batch (replaces /extension-logs POST)
  "extension_logs": [
    {
      "timestamp": "2024-01-15T10:30:00.000Z",
      "level": "info",
      "message": "Polling started",
      "source": "background",
      "category": "connection"
    }
  ],

  // Ack last processed command ID (for reliable delivery)
  "last_command_ack": "cmd_42",

  // Command results batch (replaces multiple POST endpoints)
  "command_results": [
    {
      "id": "cmd_43",
      "correlation_id": "abc",
      "status": "complete",
      "result": { "html": "..." },
      "error": null
    }
  ]
}
```

### Response

```typescript
{
  // Server acknowledged the sync
  "ack": true,

  // Commands for extension to execute (replaces /pending-queries GET)
  "commands": [
    {
      "id": "cmd_44",
      "type": "dom",
      "params": { "selector": "#app" },
      "correlation_id": "def"
    }
  ],

  // Server-controlled poll interval (dynamic backoff)
  "next_poll_ms": 1000,

  // Server time for drift detection
  "server_time": "2024-01-15T10:30:01.000Z",

  // Optional: Server version for compatibility
  "server_version": "5.6.0",

  // Optional: Capture setting overrides from AI
  "capture_overrides": {
    "capture_logs": "error",
    "capture_network": "all"
  }
}
```

## State Management

### Extension State

```typescript
interface SyncState {
  // Connection status (binary)
  connected: boolean;

  // Last successful sync timestamp
  lastSyncAt: number;

  // Consecutive failures for backoff
  consecutiveFailures: number;

  // Current backoff (calculated from failures)
  currentBackoffMs: number;

  // Last acknowledged command ID
  lastCommandAck: string | null;

  // Pending command results to send
  pendingResults: CommandResult[];

  // Buffered extension logs
  bufferedLogs: ExtensionLog[];
}
```

### Backoff Strategy

```typescript
// Simple exponential backoff, no circuit breaker needed
function calculateBackoff(failures: number): number {
  if (failures === 0) return 0;
  const base = 1000; // 1 second
  const max = 30000; // 30 seconds
  return Math.min(base * Math.pow(2, failures - 1), max);
}

// Reset on success
function onSyncSuccess(state: SyncState): void {
  state.connected = true;
  state.lastSyncAt = Date.now();
  state.consecutiveFailures = 0;
  state.currentBackoffMs = 0;
}

// Backoff on failure
function onSyncFailure(state: SyncState): void {
  state.connected = false;
  state.consecutiveFailures++;
  state.currentBackoffMs = calculateBackoff(state.consecutiveFailures);
}
```

## Server Implementation

### Handler Pseudocode

```go
func (h *SyncHandler) HandleSync(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        w.WriteHeader(http.StatusMethodNotAllowed)
        return
    }

    var req SyncRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", 400)
        return
    }

    c.mu.Lock()

    // Update last poll time (for health endpoint)
    c.lastPollAt = time.Now()

    // Store session settings
    if req.SessionID != "" {
        c.sessionID = req.SessionID
    }
    if req.Settings != nil {
        c.pilotEnabled = req.Settings.PilotEnabled
        c.trackingEnabled = req.Settings.TrackingEnabled
        c.trackedTabID = req.Settings.TrackedTabID
        c.trackedTabURL = req.Settings.TrackedTabURL
    }

    // Buffer extension logs
    for _, log := range req.ExtensionLogs {
        c.extensionLogs = append(c.extensionLogs, log)
    }

    // Process command results
    for _, result := range req.CommandResults {
        c.processCommandResult(result)
    }

    // Acknowledge processed commands
    if req.LastCommandAck != "" {
        c.acknowledgeCommand(req.LastCommandAck)
    }

    // Get pending commands
    commands := c.getPendingCommands()

    c.mu.Unlock()

    // Build response
    resp := SyncResponse{
        Ack:             true,
        Commands:        commands,
        NextPollMs:      1000,
        ServerTime:      time.Now().Format(time.RFC3339),
        ServerVersion:   version.Version,
        CaptureOverrides: c.getCaptureOverrides(),
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}
```

## Migration Path

1. **Phase 1**: Add `/sync` endpoint to server (backward compatible)
2. **Phase 2**: Update extension to use `/sync`, keep old endpoints as fallback
3. **Phase 3**: Remove fallback after verification
4. **Phase 4**: Deprecate old endpoints in next major version

## Benefits

1. **Simpler state**: Connection is binary (success/fail), no circuit breaker states
2. **Self-healing**: Each sync is independent, no accumulated state to corrupt
3. **MV3 compatible**: Service worker can die and restart without issues
4. **Lower overhead**: One HTTP request instead of 4 every second
5. **Reliable delivery**: Command ack ensures no missed commands
6. **Dynamic backoff**: Server can control poll interval based on load

## Files to Modify

| File | Changes |
|------|---------|
| `internal/capture/sync.go` | NEW - HandleSync implementation |
| `cmd/browser-agent/main.go` | Register /sync route |
| `src/background/sync.ts` | NEW - Sync client implementation |
| `src/background/polling.ts` | Remove consolidated polling loops |
| `src/background/index.ts` | Switch to sync client |
