---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Timestamp Standard - Kaboom Project

**Version:** v5.3+
**Last Updated:** 2026-01-30

---

## Problem

Different buffer types use different timestamp representations internally:

| Buffer Type | Internal Storage | Format Example |
|-------------|------------------|----------------|
| Logs | `Timestamp string` | `"2026-01-30T10:15:23.456Z"` (RFC3339) |
| WebSocket Events | `Timestamp string` | `"2026-01-30T10:15:23.456Z"` (RFC3339) |
| **Actions** | `Timestamp int64` | `1738238123456` (Unix milliseconds) |
| **NetworkWaterfall** | `Timestamp time.Time` | Go native type |
| NetworkBodies | `Timestamp string` | `"2026-01-30T10:15:23.456Z"` (RFC3339) |

This inconsistency causes issues for:
- Cursor-based pagination (v5.4+)
- Time-based filtering
- LLM temporal reasoning
- Cross-buffer correlation

---

## Standard

**ALL timestamps in MCP responses MUST be RFC3339 strings (millisecond precision, UTC timezone).**

```
Format: "2026-01-30T10:15:23.456Z"
Spec: RFC3339 with millisecond precision
Timezone: ALWAYS UTC (Z suffix required)
```

### Implementation

**Normalize at response serialization time** (in `cmd/browser-agent/tools_core.go`):

```go
// For Actions (int64 → RFC3339)
func serializeAction(action EnhancedAction) map[string]interface{} {
    return map[string]interface{}{
        "type": action.Type,
        "target": action.Target,
        // Convert Unix milliseconds to RFC3339
        "timestamp": time.UnixMilli(action.Timestamp).Format(time.RFC3339),
        "sequence": action.Sequence,
    }
}

// For NetworkWaterfall (time.Time → RFC3339)
func serializeNetworkWaterfallEntry(entry NetworkWaterfallEntry) map[string]interface{} {
    return map[string]interface{}{
        "url": entry.URL,
        "method": entry.Method,
        // Convert Go time.Time to RFC3339
        "timestamp": entry.Timestamp.Format(time.RFC3339),
    }
}

// For Logs/WebSocket (already RFC3339 string, pass through)
func serializeLogEntry(entry LogEntry) map[string]interface{} {
    return map[string]interface{}{
        "message": entry.Message,
        "level": entry.Level,
        // Already RFC3339 string, no conversion needed
        "timestamp": entry.Timestamp,
    }
}
```

---

## Rationale

### Why RFC3339?
- **ISO standard** - Universally recognized format
- **Human-readable** - LLMs can understand "2026-01-30T10:15:23Z" means "January 30, 2026 at 10:15am UTC"
- **Sortable** - Lexicographic sort = chronological sort
- **JSON-compatible** - Serializes as string, no precision loss

### Why Millisecond Precision?
- **JavaScript compatibility** - `new Date().toISOString()` produces millisecond precision
- **Sufficient granularity** - Distinguishes events within same second
- **Avoids collisions** - Most browser events happen >1ms apart
- **Nanosecond unnecessary** - Browser timestamps don't have nanosecond precision

### Why UTC?
- **No timezone ambiguity** - All timestamps comparable without conversion
- **Server-side consistency** - Kaboom server runs in single timezone
- **Avoids DST issues** - Daylight saving time doesn't affect UTC

### Why Normalize at Response Time (Not Storage)?
- **Preserves internal types** - Actions can keep int64 (efficient), NetworkWaterfall keeps time.Time (Go native)
- **Zero migration cost** - No changes to existing storage structures
- **Performance** - Conversion happens once per API call, not every buffer write
- **Backward compatible** - Old code continues to work with internal types

---

## Edge Cases

### Empty Timestamps
If an entry has no timestamp (e.g., `Timestamp: ""`), the response serializer MUST:
1. **Option A (Strict):** Reject the entry at ingestion time (fail fast)
2. **Option B (Fallback):** Generate server-side timestamp: `time.Now().Format(time.RFC3339)`

**Recommendation:** Option A (strict) - prevents timestamp drift and ensures data integrity.

### Clock Skew
Browser and server clocks may differ. **Solution:**
- Use **server-side timestamps** whenever possible (when log arrives at Kaboom)
- For browser-generated timestamps, accept as-is (don't adjust) to preserve client-side temporal relationships

### Non-UTC Timestamps from Browser
If browser sends timestamp with non-UTC timezone (e.g., `"2026-01-30T10:15:23+05:00"`):
1. **Parse with timezone** - `time.Parse(time.RFC3339, timestamp)`
2. **Convert to UTC** - `parsedTime.UTC()`
3. **Format as UTC** - `utcTime.Format(time.RFC3339)`

Example:
```go
// Input: "2026-01-30T10:15:23+05:00" (IST)
parsedTime, _ := time.Parse(time.RFC3339, timestamp)
utcTime := parsedTime.UTC()
normalized := utcTime.Format(time.RFC3339)
// Output: "2026-01-30T05:15:23Z" (UTC)
```

---

## Verification

**Unit test template:**

```go
func TestTimestampNormalization(t *testing.T) {
    tests := []struct {
        name     string
        internal interface{}
        expected string
    }{
        {
            name:     "Actions int64",
            internal: int64(1738238123456),
            expected: "2026-01-30T10:15:23Z",
        },
        {
            name:     "NetworkWaterfall time.Time",
            internal: time.Date(2026, 1, 30, 10, 15, 23, 456000000, time.UTC),
            expected: "2026-01-30T10:15:23Z",
        },
        {
            name:     "Logs string (already RFC3339)",
            internal: "2026-01-30T10:15:23.456Z",
            expected: "2026-01-30T10:15:23.456Z",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            normalized := normalizeTimestamp(tt.internal)
            assert.Equal(t, tt.expected, normalized)
        })
    }
}
```

---

## Related Documents

- [Cursor Pagination Proposal](../../features/feature/cursor-pagination/feature-proposal.md) - Uses composite `"timestamp:sequence"` cursors
- [Pagination Product Spec](../../features/feature/pagination/product-spec.md) - Offset-based pagination (v5.3)
- [v6.0 Roadmap](../roadmap.md) - Future filtering and time-range queries

---

## Approval

**Product:** ✅ Approved
**Engineering:** ✅ Approved
**Target:** v5.3 (normalization at response time)
**Target:** v5.4 (cursor pagination using normalized timestamps)
