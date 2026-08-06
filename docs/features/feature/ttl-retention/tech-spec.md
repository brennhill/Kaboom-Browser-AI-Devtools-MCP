---
status: shipped
scope: feature/ttl-retention/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-ttl-retention
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-ttl-retention.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [TTL Retention Review](./index.md).

# Technical Spec: TTL-Based Retention (Feature 16)

## Overview

TTL-based retention adds configurable time-to-live for captured browser telemetry so buffers automatically evict old entries. This prevents stale data from accumulating in long-running sessions while giving the AI agent control over how much historical context to retain per buffer type.

---

## Requirements

### Must Have

1. Per-buffer TTL configuration (console, network, websocket, actions)
2. Global TTL as default when per-buffer TTL is not set
3. Runtime configuration via MCP `configure` tool
4. Startup configuration via CLI flags
5. Read-time filtering (entries older than TTL excluded from responses)
6. TTL=0 means unlimited retention (no filtering)
7. Minimum TTL of 1 minute to prevent misconfiguration
8. TTL status visible in health/diagnostics output

### Should Have

1. Memory pressure interaction (TTL can trigger early eviction under pressure)
2. Per-buffer statistics showing filtered vs retained entry counts
3. Ability to clear specific buffer while preserving others

### Nice to Have

1. Dynamic TTL adjustment based on memory pressure
2. TTL presets for common scenarios (debug session, CI run, long monitoring)

---

## Data Model

### Go Structs

```go
// TTLConfig holds per-buffer TTL settings.
// Zero values mean "use global default" for buffers, or "unlimited" for global.
type TTLConfig struct {
    Global    time.Duration `json:"global"`    // Default TTL for all buffers
    Console   time.Duration `json:"console"`   // Console logs (Server.entries)
    Network   time.Duration `json:"network"`   // Network bodies
    WebSocket time.Duration `json:"websocket"` // WebSocket events
    Actions   time.Duration `json:"actions"`   // Enhanced user actions
}

// TTLStats tracks per-buffer retention statistics.
type TTLStats struct {
    Buffer           string    `json:"buffer"`            // "console", "network", "websocket", "actions"
    TTL              string    `json:"ttl"`               // Human-readable TTL ("5m", "1h", "unlimited")
    TotalEntries     int       `json:"total_entries"`     // Entries currently in buffer
    EntriesInTTL     int       `json:"entries_in_ttl"`    // Entries within TTL window
    FilteredByTTL    int       `json:"filtered_by_ttl"`   // Entries that would be filtered
    OldestEntry      time.Time `json:"oldest_entry"`      // Timestamp of oldest entry
    NewestEntry      time.Time `json:"newest_entry"`      // Timestamp of newest entry
    EffectiveTTL     string    `json:"effective_ttl"`     // Resolved TTL (buffer-specific or global)
}

// TTLConfigResponse is returned when querying or updating TTL settings.
type TTLConfigResponse struct {
    Config TTLConfig  `json:"config"`
    Stats  []TTLStats `json:"stats"`
}
```

### JSON Schemas

#### TTL Configuration (MCP Tool Input)

```json
{
  "type": "object",
  "properties": {
    "global": {
      "type": "string",
      "description": "Default TTL for all buffers (e.g., '15m', '1h', '' for unlimited)",
      "pattern": "^([0-9]+[hms])+$|^$"
    },
    "console": {
      "type": "string",
      "description": "TTL for console log entries"
    },
    "network": {
      "type": "string",
      "description": "TTL for network body captures"
    },
    "websocket": {
      "type": "string",
      "description": "TTL for WebSocket events"
    },
    "actions": {
      "type": "string",
      "description": "TTL for enhanced user actions"
    }
  }
}
```

#### TTL Status Response

```json
{
  "config": {
    "global": "15m",
    "console": "",
    "network": "5m",
    "websocket": "",
    "actions": "30m"
  },
  "stats": [
    {
      "buffer": "console",
      "ttl": "15m",
      "total_entries": 150,
      "entries_in_ttl": 120,
      "filtered_by_ttl": 30,
      "oldest_entry": "2025-01-20T14:15:00Z",
      "newest_entry": "2025-01-20T14:30:00Z",
      "effective_ttl": "15m (global)"
    },
    {
      "buffer": "network",
      "ttl": "5m",
      "total_entries": 80,
      "entries_in_ttl": 45,
      "filtered_by_ttl": 35,
      "oldest_entry": "2025-01-20T14:20:00Z",
      "newest_entry": "2025-01-20T14:30:00Z",
      "effective_ttl": "5m (buffer-specific)"
    }
  ]
}
```

---

## API Surface

### MCP Tool: `configure` with TTL Action

Extends the existing `configure` tool with a new `ttl` action.

```json
{
  "name": "configure",
  "description": "Configure the session: ..., manage TTL-based retention settings, ...",
  "inputSchema": {
    "type": "object",
    "properties": {
      "action": {
        "type": "string",
        "enum": ["store", "load", "noise_rule", "dismiss", "clear", "ttl"],
        "description": "Configuration action to perform"
      },
      "ttl_action": {
        "type": "string",
        "enum": ["get", "set", "reset"],
        "description": "TTL sub-action: get current settings, set new values, reset to defaults"
      },
      "ttl_config": {
        "type": "object",
        "description": "TTL configuration (for 'set' action)",
        "properties": {
          "global": { "type": "string" },
          "console": { "type": "string" },
          "network": { "type": "string" },
          "websocket": { "type": "string" },
          "actions": { "type": "string" }
        }
      }
    }
  }
}
```

#### Examples

##### Get current TTL settings:
```json
{
  "action": "ttl",
  "ttl_action": "get"
}
```

##### Set global TTL:
```json
{
  "action": "ttl",
  "ttl_action": "set",
  "ttl_config": {
    "global": "15m"
  }
}
```

##### Set per-buffer TTL:
```json
{
  "action": "ttl",
  "ttl_action": "set",
  "ttl_config": {
    "global": "30m",
    "network": "5m",
    "websocket": "1h"
  }
}
```

##### Reset to defaults (unlimited):
```json
{
  "action": "ttl",
  "ttl_action": "reset"
}
```

### HTTP Endpoint

The existing `/v4/health` endpoint is extended to include TTL information:

```
GET /v4/health
```

Response includes TTL section:

```json
{
  "status": "ok",
  "uptime_seconds": 3600,
  "memory": { ... },
  "buffers": { ... },
  "ttl": {
    "config": {
      "global": "15m",
      "console": "",
      "network": "5m",
      "websocket": "",
      "actions": ""
    },
    "effective": {
      "console": "15m",
      "network": "5m",
      "websocket": "15m",
      "actions": "15m"
    }
  }
}
```

### Startup Flags

```
--ttl <duration>           Global TTL for all buffers (default: unlimited)
--ttl-console <duration>   TTL for console log entries
--ttl-network <duration>   TTL for network body captures
--ttl-websocket <duration> TTL for WebSocket events
--ttl-actions <duration>   TTL for enhanced user actions
```

Examples:
```bash
# 15-minute global TTL
kaboom --ttl 15m

# Different TTL per buffer
kaboom --ttl 30m --ttl-network 5m --ttl-websocket 1h

# Unlimited (default)
kaboom
```

---

## Implementation Details

### TTL Resolution

When reading entries, resolve the effective TTL for each buffer:

```go
func (c *TTLConfig) EffectiveTTL(buffer string) time.Duration {
    var bufferTTL time.Duration
    switch buffer {
    case "console":
        bufferTTL = c.Console
    case "network":
        bufferTTL = c.Network
    case "websocket":
        bufferTTL = c.WebSocket
    case "actions":
        bufferTTL = c.Actions
    }

    // Buffer-specific TTL takes precedence
    if bufferTTL > 0 {
        return bufferTTL
    }
    // Fall back to global (0 means unlimited)
    return c.Global
}
```

### Read-Time Filtering

TTL filtering happens at read time, not write time. This preserves the full ring buffer behavior while hiding expired entries from API responses:

```go
func (c *Capture) GetWebSocketEvents(filter WebSocketEventFilter) []WebSocketEvent {
    c.mu.RLock()
    defer c.mu.RUnlock()

    effectiveTTL := c.ttlConfig.EffectiveTTL("websocket")
    cutoff := time.Time{}
    if effectiveTTL > 0 {
        cutoff = time.Now().Add(-effectiveTTL)
    }

    var result []WebSocketEvent
    for i, event := range c.wsEvents {
        // TTL filter
        if effectiveTTL > 0 && c.wsAddedAt[i].Before(cutoff) {
            continue
        }
        // Other filters...
        if filter.ConnectionID != "" && event.ID != filter.ConnectionID {
            continue
        }
        result = append(result, event)
    }
    return result
}
```

### Eviction Strategy: Time-Based (Not LRU)

Kaboom uses **time-based eviction** via TTL, not LRU. The reasoning:

1. **Predictability**: Time-based eviction is deterministic. AI agents can reason about what data is available based on elapsed time.

2. **Fairness**: LRU would favor frequently-accessed buffers over rarely-accessed ones. A network body captured once but never queried shouldn't expire faster than a frequently-read WebSocket event.

3. **Simplicity**: Ring buffers already provide capacity-based eviction. TTL adds age-based eviction. Combining two orthogonal strategies is simpler than hybrid LRU.

4. **Memory-pressure compatibility**: The existing memory enforcement (spec: `tech-spec-memory-enforcement.md`) handles capacity-based eviction. TTL handles age-based filtering. They compose cleanly.

### Memory Pressure Interaction

When memory pressure is detected (above soft limit), the server can accelerate TTL eviction:

```go
const (
    normalTTLMultiplier   = 1.0
    pressureTTLMultiplier = 0.5  // Halve effective TTL under pressure
    criticalTTLMultiplier = 0.25 // Quarter effective TTL at critical pressure
)

func (c *Capture) getEffectiveTTLWithPressure(buffer string) time.Duration {
    baseTTL := c.ttlConfig.EffectiveTTL(buffer)
    if baseTTL == 0 {
        return 0 // Unlimited stays unlimited even under pressure
    }

    switch c.mem.PressureLevel() {
    case PressureNormal:
        return baseTTL
    case PressureSoft:
        return time.Duration(float64(baseTTL) * pressureTTLMultiplier)
    case PressureHard, PressureCritical:
        return time.Duration(float64(baseTTL) * criticalTTLMultiplier)
    }
    return baseTTL
}
```

This interaction is **optional** and disabled by default. Enable via `--ttl-pressure-aware` flag.

### Storage Location

TTL state is stored in the existing `Capture` struct:

```go
type Capture struct {
    // ... existing fields ...

    // TTL configuration
    ttlConfig TTLConfig

    // Per-buffer timestamps (already exist)
    wsAddedAt      []time.Time
    networkAddedAt []time.Time
    actionAddedAt  []time.Time
}
```

The `Server` struct already has `logAddedAt` for console entries.

### Thread Safety

TTL configuration changes are protected by the existing `Capture.mu` mutex:

```go
func (c *Capture) SetTTLConfig(config TTLConfig) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.ttlConfig = config
}

func (c *Capture) GetTTLConfig() TTLConfig {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.ttlConfig
}
```

---

## Configuration Options

### CLI Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--ttl` | duration | 0 (unlimited) | Global TTL for all buffers |
| `--ttl-console` | duration | 0 (use global) | TTL for console logs |
| `--ttl-network` | duration | 0 (use global) | TTL for network bodies |
| `--ttl-websocket` | duration | 0 (use global) | TTL for WebSocket events |
| `--ttl-actions` | duration | 0 (use global) | TTL for user actions |
| `--ttl-pressure-aware` | bool | false | Reduce TTL under memory pressure |

### Environment Variables

As an alternative to CLI flags:

| Variable | Equivalent Flag |
|----------|-----------------|
| `KABOOM_TTL` | `--ttl` |
| `KABOOM_TTL_CONSOLE` | `--ttl-console` |
| `KABOOM_TTL_NETWORK` | `--ttl-network` |
| `KABOOM_TTL_WEBSOCKET` | `--ttl-websocket` |
| `KABOOM_TTL_ACTIONS` | `--ttl-actions` |

### Presets (via MCP)

Common TTL configurations can be applied via presets:

```json
{
  "action": "ttl",
  "ttl_action": "set",
  "preset": "debug"
}
```

| Preset | Global | Network | WebSocket | Actions | Use Case |
|--------|--------|---------|-----------|---------|----------|
| `debug` | 15m | 5m | 30m | 15m | Active debugging session |
| `ci` | 5m | 2m | 5m | 5m | CI pipeline run |
| `monitor` | 1h | 30m | 1h | 1h | Long-running monitoring |
| `unlimited` | 0 | 0 | 0 | 0 | Full history (default) |

---

## Testing Strategy

### Unit Tests

1. **TTL parsing**
   - Valid durations: "1m", "15m", "1h", "2h30m"
   - Invalid durations: "abc", "0", negative values
   - Minimum enforcement: durations < 1m rejected
   - Empty string means unlimited

2. **TTL resolution**
   - Buffer-specific TTL takes precedence over global
   - Global TTL used when buffer-specific is 0
   - Both 0 means unlimited

3. **Read-time filtering**
   - Entries older than TTL are excluded
   - Entries within TTL are included
   - Boundary case: entry exactly at TTL age is excluded
   - TTL=0 returns all entries

4. **Per-buffer independence**
   - Different TTL per buffer works correctly
   - Changing one buffer's TTL doesn't affect others

5. **Memory pressure interaction**
   - Normal pressure: TTL unchanged
   - Soft pressure: TTL halved (if enabled)
   - Critical pressure: TTL quartered (if enabled)
   - Unlimited stays unlimited under any pressure

### Integration Tests

6. **MCP tool integration**
   - `configure` with `action: "ttl"` works
   - Get returns current config
   - Set updates config
   - Reset clears to defaults

7. **CLI flag integration**
   - `--ttl` sets global TTL
   - Per-buffer flags override global
   - Environment variables work

8. **Health endpoint**
   - TTL config appears in `/v4/health` response
   - Effective TTL calculated correctly

### Edge Cases

9. **Empty buffer with TTL**
   - No crash, returns empty array

10. **Ring buffer wrap with TTL**
    - Oldest entries may be TTL-expired before ring eviction
    - Ring eviction still works correctly

11. **TTL change during read**
    - Concurrent TTL update doesn't cause race
    - Read uses consistent TTL for entire query

12. **Stats calculation**
    - Correct count of entries in/out of TTL window
    - Works with empty buffers

### Test File Location

Tests go in `cmd/browser-agent/ttl_test.go` (extending existing file).

---

## Migration / Backwards Compatibility

### Existing TTL Field

The existing `Capture.TTL` field becomes `ttlConfig.Global`:

```go
// Before
type Capture struct {
    TTL time.Duration
}

// After
type Capture struct {
    ttlConfig TTLConfig // ttlConfig.Global replaces TTL
}

// Compatibility: SetTTL still works
func (c *Capture) SetTTL(ttl time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.ttlConfig.Global = ttl
}
```

### Existing CLI Flag

The existing `--ttl` flag behavior is unchanged. It sets the global TTL.

### Existing Tests

All existing TTL tests continue to pass. They test global TTL behavior, which is preserved.

### API Compatibility

No breaking changes to existing API:
- `--ttl` flag still works
- `Capture.SetTTL()` still works
- Existing MCP tools that read buffers still work (they get TTL-filtered results)

New capabilities are additive:
- Per-buffer TTL via new flags
- TTL configuration via `configure` tool
- TTL stats in health endpoint

---

## File Locations

| File | Purpose |
|------|---------|
| `cmd/browser-agent/ttl.go` | TTL types, parsing, resolution logic |
| `cmd/browser-agent/ttl_test.go` | TTL tests |
| `cmd/browser-agent/types.go` | TTLConfig struct definition |
| `cmd/browser-agent/tools_core.go` | MCP tool schema update |
| `cmd/browser-agent/configure.go` | TTL action handler |
| `cmd/browser-agent/health.go` | Health endpoint TTL section |
| `cmd/browser-agent/main.go` | CLI flag parsing |

---

## Limits

| Constraint | Value | Reason |
|------------|-------|--------|
| Minimum TTL | 1 minute | Prevent accidental data loss from tiny TTLs |
| Maximum TTL | 24 hours | Prevent memory growth from very long TTLs |
| Per-buffer TTL resolution | 1 second | Sufficient granularity for filtering |
| Stats calculation frequency | On-demand | No background overhead |

---

## Non-Goals

- **Persistent TTL configuration**: TTL resets to defaults on server restart. Use CLI flags or startup scripts for persistence.
- **Automatic TTL tuning**: The AI agent decides appropriate TTL values, not the server.
- **Write-time eviction**: Entries are always written to buffers. TTL only filters reads.
- **TTL for performance/a11y caches**: These have their own eviction strategies. TTL applies only to the four main buffers.
