---
status: draft
priority: tier-1
phase: v5.4-foundation
relates_to: [PRODUCT_SPEC.md, TECH_SPEC_KABOOM_RUN.md, TECH_SPEC_LOCAL_TAILER.md, TECH_SPEC_SSH_TAILER.md]
last-updated: 2026-01-31
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Backend Log Ingestion: Daemon Integration — Technical Specification

**Goal:** Extend Kaboom daemon to ingest backend logs (kaboom-run, file tailer, SSH tailer) and merge with FE telemetry for unified request tracing.

---

## Overview

### Current v5.3 Daemon

```
HTTP Server (localhost:7890)
├─ POST /event ← Browser extension sends FE events
├─ GET /buffers/timeline ← LLM queries
├─ GET /daemon/status ← Health check
└─ Ring Buffers ← Stores all FE events

MCP Server (stdio)
├─ observe() ← LLM queries ring buffers
├─ generate() ← LLM generates tests
└─ → Reads from Ring Buffers
```

### Extended v5.4+ Daemon

```
HTTP Server (localhost:7890)
├─ POST /event ← Single events (extension, file tailer, SSH tailer)
├─ POST /events ← Batch events (kaboom-run wrapper)
├─ GET /buffers/timeline ← LLM queries (merged FE+BE)
├─ GET /daemon/status ← Health check (includes clock skew)
├─ GET /ingest/stats ← Backend ingestion stats
└─ Ring Buffers ← Stores FE + BE events (merged by timestamp)

Log Ingestion (Concurrent goroutines)
├─ HTTP listener (POST /event, /events)
├─ Event normalization (extract timestamp, correlation_id, level, message)
├─ Ring buffer routing (add to appropriate buffer)
└─ Stats tracking (lines read, lines failed, last error)

MCP Server (stdio)
├─ observe() ← LLM queries (FE+BE merged)
├─ generate() ← LLM generates tests
└─ → Reads from Ring Buffers
```

---

## Architectural Decisions

### Decision 1: Single Ring Buffer vs Multiple Buffers

**Option A (Chosen):** Single unified ring buffer

- All events (FE + BE) stored in one ring buffer
- Sorted by timestamp (not receive time)
- Tagged with source (extension, stdout, file, ssh)
- Query filters by source or shows all

**Option B (Rejected):** Separate `backend_logs` ring buffer

- Would require separate buffer implementation
- Query would need to merge results from two buffers
- Harder to correlate by timestamp across buffers
- More complex for correlation_id matching

**Why Option A:** Simpler, natural merging by timestamp, correlation_id matching works automatically.

### Decision 2: Event Normalization

#### All events normalized to common schema:

```go
type NormalizedEvent struct {
  EventID       string            // UUID, assigned by daemon
  Timestamp     int64             // ms since epoch (SOURCE time, not receive time)
  Level         string            // debug, info, warn, error, critical
  Source        string            // extension, stdout, stderr, file, ssh
  Message       string            // Log message text
  CorrelationID string            // Trace ID (optional, "" if not found)
  Metadata      map[string]any    // Additional fields (client_skew_ms, inode, path, etc.)
  Tags          []string          // Source tags (e.g., ["service:auth", "env:dev"])
  ReceiveTime   int64             // When daemon received event (for latency tracking)
}
```

#### Benefits:
- Consistent query interface regardless of source
- Timestamp comes from event source, not daemon (accurate correlation)
- Metadata bag for source-specific fields (client_skew_ms, SSH host, file path)
- Tags enable source filtering without special columns

### Decision 3: Ring Buffer Capacity

#### Ring buffer sizing strategy:

```
Existing v5.3 buffer: 1M events (configurable)
New v5.4 with BE logs: 2M events (50% increase)

Rationale:
- FE events (from extension): ~50-100 events/sec max
- BE events (from tailers): ~100-500 events/sec (varies by log volume)
- 24-hour retention: 86,400 seconds
- At 500 events/sec: ~43M events/day
- Buffer size: 2M events = ~4 hours of max load (reasonable for interactive debugging)
- Operator can increase if needed (config: buffer_capacity: 5000000)
```

### Decision 4: Timestamp Source

#### Use source timestamp, NOT daemon receive time:

- FE events from extension: Contains browser timestamp (set by browser)
- FE requests with X-Client-Time: Contains user's machine time
- BE logs: Contain server timestamp (extracted by parser)
- BE events cached on disk before sending: Use original timestamp

**Why:** Enables accurate correlation across FE/BE timeline. Receiver time only matters for latency tracking, not for timeline ordering.

### Decision 5: Correlation ID Matching

#### Correlation strategy (in priority order):

1. **Exact match by correlation_id:**
   - FE sends `X-Trace-ID: 550e8400-e29b-41d4` in request
   - BE logs contain `[trace:550e8400-e29b-41d4]`
   - Parser extracts correlation_id from both
   - Daemon matches by exact ID

2. **Fuzzy timestamp match (within 100ms):**
   - If no correlation_id found in BE logs
   - Match FE request timestamp (e.g., 12:00:00.100) with BE log timestamp (e.g., 12:00:00.115)
   - Heuristic: Accept if delta < 100ms and same endpoint
   - Flag as "inferred" (for AI context)

3. **Per-Request ID (stateful correlation):**
   - Maintain request counter per correlation_id
   - E.g., correlation_id "abc123" → request #1, #2, #3
   - Match by correlation_id + request order
   - Falls back to fuzzy matching if mismatch

**Why:** Handles imperfect instrumentation (some logs missing trace IDs) while prioritizing exact correlation when available.

---

## Implementation Details

### HTTP Event Ingestion Handler

#### Endpoint 1: POST /event (Single)

```go
// Handle both extension events and tailer events
func handleEvent(w http.ResponseWriter, r *http.Request) {
  var rawEvent map[string]any
  json.NewDecoder(r.Body).Decode(&rawEvent)

  normalized := normalizeEvent(rawEvent, "backend")  // source inferred from Content
  normalized.EventID = uuid.New().String()
  normalized.ReceiveTime = time.Now().UnixMilli()

  ringBuf.Add(normalized)

  w.Header().Set("Content-Type", "application/json")
  json.NewEncoder(w).Encode(map[string]any{
    "status": "ok",
    "event_id": normalized.EventID,
  })
}

func normalizeEvent(raw map[string]any, defaultSource string) NormalizedEvent {
  ne := NormalizedEvent{
    Timestamp: extractTimestamp(raw),
    Level: normalizeLevel(getString(raw, "level")),
    Source: getString(raw, "source", defaultSource),
    Message: getString(raw, "message"),
    Metadata: make(map[string]any),
    Tags: getStringArray(raw, "tags"),
  }

  // Extract correlation_id from multiple possible locations
  ne.CorrelationID = extractCorrelationID(raw)

  // Preserve all other fields in metadata
  for k, v := range raw {
    if !isStandardField(k) {
      ne.Metadata[k] = v
    }
  }

  return ne
}
```

#### Endpoint 2: POST /events (Batch)

```go
func handleEvents(w http.ResponseWriter, r *http.Request) {
  var batch struct {
    Events []map[string]any `json:"events"`
  }
  json.NewDecoder(r.Body).Decode(&batch)

  eventIDs := []string{}
  failed := []string{}

  for _, rawEvent := range batch.Events {
    normalized := normalizeEvent(rawEvent, "backend")
    normalized.EventID = uuid.New().String()
    normalized.ReceiveTime = time.Now().UnixMilli()

    if err := ringBuf.Add(normalized); err != nil {
      failed = append(failed, normalized.EventID)
    } else {
      eventIDs = append(eventIDs, normalized.EventID)
    }
  }

  w.Header().Set("Content-Type", "application/json")
  json.NewEncoder(w).Encode(map[string]any{
    "status": "ok",
    "count": len(eventIDs),
    "event_ids": eventIDs,
    "failed": failed,
  })
}
```

### Timestamp Extraction

```go
func extractTimestamp(raw map[string]any) int64 {
  // Try multiple timestamp field names
  for _, fieldName := range []string{"timestamp", "ts", "time", "date", "@timestamp"} {
    if val, ok := raw[fieldName]; ok {
      if ts := parseTimestamp(val); ts > 0 {
        return ts
      }
    }
  }

  // Fallback: current time
  return time.Now().UnixMilli()
}

func parseTimestamp(val any) int64 {
  switch v := val.(type) {
  case float64:
    // Already ms since epoch
    if v > 1000000000000 { // > 2001
      return int64(v)
    }
    // Might be seconds since epoch
    return int64(v * 1000)

  case string:
    // Try RFC3339 (ISO 8601)
    if t, err := time.Parse(time.RFC3339, v); err == nil {
      return t.UnixMilli()
    }
    // Try RFC3339 without timezone
    if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
      return t.UnixMilli()
    }
    // Try Unix timestamp string
    if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
      if ts > 1000000000000 { // ms
        return ts
      }
      return ts * 1000 // seconds
    }
  }

  return 0  // Unparseable, fallback to current time
}
```

### Correlation ID Extraction

```go
func extractCorrelationID(raw map[string]any) string {
  // Check direct field
  if val, ok := raw["correlation_id"]; ok {
    if id := toString(val); id != "" {
      return id
    }
  }

  // Check metadata object
  if metadata, ok := raw["metadata"].(map[string]any); ok {
    for _, fieldName := range []string{"correlation_id", "trace_id", "request_id", "trace-id", "x-trace-id"} {
      if val, ok := metadata[fieldName]; ok {
        if id := toString(val); id != "" {
          return id
        }
      }
    }
  }

  // Check message for embedded trace ID
  if msg, ok := raw["message"].(string); ok {
    if id := extractTraceIDFromMessage(msg); id != "" {
      return id
    }
  }

  return ""  // No correlation_id found
}

func extractTraceIDFromMessage(msg string) string {
  // Pre-compiled patterns (same as log parser)
  patterns := []string{
    `[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`,  // UUID
    `1-[a-f0-9]{8}-[a-f0-9]{24}`,  // AWS X-Ray
    `req[_:]?([a-zA-Z0-9\-]+)`,  // req:xyz
    `trace[_]?id[_:]?([a-zA-Z0-9\-]+)`,  // trace_id:xyz
  }

  for _, pattern := range patterns {
    if re := compiledPatterns[pattern]; re != nil {
      if match := re.FindStringSubmatch(msg); len(match) > 0 {
        return match[1]
      }
    }
  }

  return ""
}
```

---

## Query Interface Changes

### Endpoint: GET /buffers/timeline

#### Query parameters (new):

```
?correlation_id=550e8400-e29b-41d4    // Filter by trace ID
?source=backend                        // Filter by source (extension, stdout, file, ssh)
?service=auth                          // Filter by tag (requires events to be tagged)
?level=error                           // Filter by log level
?start_time=1704067200000             // Timestamp range
?end_time=1704067260000
```

#### Response format (backward compatible):

```json
{
  "events": [
    {
      "timestamp": 1704067200000,
      "level": "info",
      "source": "extension",
      "message": "User clicked login",
      "correlation_id": "550e8400-e29b-41d4",
      "tags": ["dev"]
    },
    {
      "timestamp": 1704067200005,
      "level": "info",
      "source": "stdout",
      "message": "POST /api/auth received",
      "correlation_id": "550e8400-e29b-41d4",
      "metadata": {
        "client_skew_ms": 3
      }
    }
  ]
}
```

### Endpoint: GET /ingest/stats (New)

#### Response:

```json
{
  "kaboom_run": {
    "host": "localhost",
    "lines_read": 12500,
    "lines_failed": 0,
    "last_read_time": "2024-01-01T12:05:00Z",
    "last_error": null
  },
  "file_tailer_app_log": {
    "path": "/var/log/app.log",
    "lines_read": 8900,
    "lines_failed": 2,
    "last_read_time": "2024-01-01T12:04:59Z",
    "last_error": "File rotated"
  },
  "ssh_tailer_prod": {
    "host": "prod.example.com",
    "lines_read": 5100,
    "lines_failed": 15,
    "last_read_time": "2024-01-01T12:04:55Z",
    "last_error": "Connection timeout",
    "circuit_breaker": {
      "state": "open",
      "failures": 12,
      "next_retry": "2024-01-01T12:06:00Z"
    }
  }
}
```

---

## Clock Skew Tracking

### Clock Skew Sample Collection

```go
type ClockSkewTracker struct {
  samples []int64  // Last 30 skew samples
  mu      sync.RWMutex
}

func (cst *ClockSkewTracker) RecordSample(skewMS int64) {
  cst.mu.Lock()
  defer cst.mu.Unlock()

  cst.samples = append(cst.samples, skewMS)
  if len(cst.samples) > 30 {
    cst.samples = cst.samples[1:]  // Keep last 30
  }
}

func (cst *ClockSkewTracker) GetStats() ClockSkewStats {
  cst.mu.RLock()
  defer cst.mu.RUnlock()

  if len(cst.samples) == 0 {
    return ClockSkewStats{}
  }

  // Calculate percentiles
  sorted := make([]int64, len(cst.samples))
  copy(sorted, cst.samples)
  sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

  median := sorted[len(sorted)/2]
  p95 := sorted[int(float64(len(sorted))*0.95)]
  p99 := sorted[int(float64(len(sorted))*0.99)]

  // Calculate std dev
  var sum, sumSq int64
  for _, s := range sorted {
    sum += s
    sumSq += s * s
  }
  mean := float64(sum) / float64(len(sorted))
  variance := float64(sumSq)/float64(len(sorted)) - mean*mean
  stdDev := math.Sqrt(variance)

  return ClockSkewStats{
    SamplesCollected: len(cst.samples),
    Median:           median,
    P95:              p95,
    P99:              p99,
    StdDev:           stdDev,
    // ... status determination logic ...
  }
}
```

---

## Backward Compatibility

### v5.3 Clients

#### Existing queries still work:

```
GET /buffers/timeline              # Returns all events (FE only in v5.3, FE+BE in v5.4)
```

#### Behavior:
- v5.3 client queries, v5.4 daemon: Returns FE + BE merged by timestamp
- v5.3 client doesn't filter by source, so gets all
- Timestamp ordering ensures chronological flow

### Ring Buffer Sizing

**If buffer capacity increased** (1M → 2M):

- v5.3 behavior: TTL + buffer limit apply equally
- v5.4 behavior: More events retained due to larger buffer
- No breaking change, just better retention

### New Endpoints

#### Backward compatible:

```
POST /event             # Already exists for extension, now also used by tailers
POST /events            # New, optional (only used by kaboom-run)
GET /ingest/stats       # New, optional endpoint
GET /daemon/status      # Existing, adds clock_skew field
```

---

## Testing Strategy

### Unit Tests

1. Event normalization (various formats, missing fields)
2. Timestamp parsing (RFC3339, Unix seconds, milliseconds, ISO 8601)
3. Correlation ID extraction (UUID, X-Ray, Jaeger, W3C, regex patterns)
4. Clock skew calculation (median, percentiles, std dev)

### Integration Tests

1. FE + BE event merging by timestamp
2. Correlation ID matching (exact + fuzzy)
3. Query filtering (by source, level, service tag)
4. Batch POST /events endpoint
5. Single POST /event endpoint
6. Ring buffer capacity limits and eviction

### End-to-End Tests

1. Run kaboom-run with Node server
2. Simulate FE events via extension
3. Verify merged timeline in `/buffers/timeline`
4. Verify correlation by trace ID
5. Check clock skew detection

---

## Monitoring & Debugging

### Health Check

```bash
curl http://localhost:7890/daemon/status
```

Includes:
- Ring buffer capacity and usage
- Clock skew status (median, std dev, recommendation)
- Connection errors or recent alerts

### Ingestion Stats

```bash
curl http://localhost:7890/ingest/stats
```

Shows:
- Lines read from each source (kaboom-run, file tailers, SSH tailers)
- Error rates and last errors
- Circuit breaker state for SSH connections

### Debug Logging

```
[daemon] POST /event received from extension: correlation_id=550e8400-e29b-41d4
[daemon] POST /events received batch of 100 lines from kaboom-run
[daemon] Clock skew updated: median=5ms, std_dev=3ms, status=synchronized
[daemon] Query: /buffers/timeline?correlation_id=550e8400-e29b-41d4 → 12 events
```

---

## Future Extensions (v6.0+)

### Auto-Correction of Clock Skew

- Automatic timestamp adjustment based on median skew
- User opt-out via config: `clock_skew_auto_correct: false`

### Log Aggregation Integrations

- Export events to external platforms (Datadog, Splunk, BigQuery)
- Long-term retention beyond ring buffer TTL

### Advanced Correlation

- Distributed tracing protocol support (Jaeger gRPC, OTel OTLP)
- Automatic propagation of trace context across service boundaries

---

## Related Documents

- **Product Spec:** [PRODUCT_SPEC.md](PRODUCT_SPEC.md)
- **kaboom-run:** [TECH_SPEC_KABOOM_RUN.md](TECH_SPEC_KABOOM_RUN.md)
- **Local Tailer:** [TECH_SPEC_LOCAL_TAILER.md](TECH_SPEC_LOCAL_TAILER.md)
- **SSH Tailer:** [TECH_SPEC_SSH_TAILER.md](TECH_SPEC_SSH_TAILER.md)

---

**Status:** Ready for implementation review

**Estimated Effort:** 5 days (includes testing and integration)

**Dependencies:** Existing daemon HTTP server, ring buffer implementation

