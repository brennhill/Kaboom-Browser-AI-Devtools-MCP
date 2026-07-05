---
status: draft
priority: tier-1
phase: v5.4-foundation
effort: 2 weeks
relates-to: [layer1-be-observability.md, target-architecture.md]
blocks: [layer1-debugging, checkpoint-validation]
last-updated: 2026-01-31
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Backend Log Ingestion — Product Specification

**Goal:** Capture backend logs (local dev and production) and merge with FE telemetry for unified request tracing in Layer 1.

---

## Problem Statement

v5.3 provides FE observability:
- ✅ Console logs, network calls, exceptions
- ✅ User actions with DOM context
- ✅ WebSocket messages

Missing for Layer 1 debugging:
- ❌ Backend service logs (no visibility into what the server saw)
- ❌ Request correlation (no way to link FE event to BE event)
- ❌ Full request flow (UI → API → database → response)

**Impact:** AI can see "checkout failed" in UI but not "why" (DB timeout, validation error, auth failure, etc.)

---

## User Stories

### Story 1: Local Dev Debugging
**As:** Developer debugging locally
**I want:** To see backend logs merged with FE events automatically
**So that:** I understand what happened when a user action fails

```
Workflow:
1. Run: kaboom-run npm run dev
2. Browser opens: localhost:3000
3. User clicks: "Add to Cart"
4. Extension captures: Click event, network call
5. Stdout capture: "[2024-01-01T12:00:00Z] INFO [req:abc123] Cart updated"
6. Kaboom merges both
7. Query result shows:
   - FE: User clicked button (12:00:00.000)
   - FE: POST /api/cart sent (12:00:00.002)
   - BE: Received request (12:00:00.003) ← from stdout
   - BE: Database query (12:00:00.015) ← from stdout
   - FE: Response received 200 (12:00:00.025)
8. AI analyzes: "Cart updated in 25ms, no errors detected"
```

---

### Story 2: Production Debugging (SSH)
**As:** Developer debugging production
**I want:** To see production logs correlated with FE events without SSH-ing manually
**So that:** I can understand production issues faster

```
Workflow:
1. Configure: ~/.kaboom/config.yaml
   ```yaml
   backend:
     logs:
       - type: ssh
         host: prod.example.com
         user: ubuntu
         path: /var/log/app.log
   ```
2. Start: kaboom (daemon connects to prod via SSH)
3. Open: production website in browser
4. User sees: Error on checkout page
5. Extension captures: FE error, network 500
6. Kaboom tails: /var/log/app.log on prod server
7. Correlates: FE error (timestamp 12:00:00) ← → BE error (timestamp 12:00:00.050)
8. Query shows:
   - FE: User clicked "Checkout" (12:00:00.000)
   - FE: POST /api/checkout sent (12:00:00.005)
   - BE: Received request (12:00:00.006) ← from SSH
   - BE: Database connection timeout (12:00:00.050) ← from SSH
   - BE: 500 error response (12:00:00.055) ← from SSH
   - FE: Error displayed to user (12:00:00.060)
9. AI analyzes: "Database is down, checkout is failing"
```

---

### Story 3: Multi-Process Local Dev
**As:** Developer with microservices locally (compose)
**I want:** To see logs from all services merged in one timeline
**So that:** I can trace requests across service boundaries

```
Workflow:
1. docker-compose up
   - Service A (FE): localhost:3000
   - Service B (Auth): localhost:3001
   - Service C (API): localhost:3002
2. kaboom-run docker-compose logs --follow
   OR configure multiple log sources:
   ```yaml
   backend:
     logs:
       - type: file
         path: service-a.log
       - type: file
         path: service-b.log
       - type: file
         path: service-c.log
   ```
3. User clicks: Login button
4. Timeline shows:
   - FE: Click login (Service A stdout)
   - BE: POST /auth (Service B stdout)
   - BE: POST /api/validate (Service C stdout)
   - BE: Response 200 (Service B stdout)
   - FE: Redirect home (Service A stdout)
5. AI traces: Full request path through services
```

---

## Features to Implement

### Feature 1: kaboom-run Wrapper (Local Dev)
**What:** Wrapper command that intercepts stdout/stderr and streams to daemon

#### How:
```bash
kaboom-run npm run dev           # Node
kaboom-run python -m flask run   # Python
kaboom-run go run main.go        # Go
kaboom-run docker-compose up     # Docker
```

#### Data captured:
- Every line of stdout/stderr
- Real-time streaming to daemon
- Pass-through to user's terminal (logs still visible)
- Auto-correlate by request ID in logs

#### Success Criteria:
- [ ] Works with any child command
- [ ] Logs appear in daemon within 10ms of writing
- [ ] Zero performance impact on dev server
- [ ] Pass-through doesn't buffer output
- [ ] Works with piped output (e.g., `kaboom-run npm run dev | tee output.log`)

---

### Feature 2: Local File Log Tailer (Production Single-Server)
**What:** Tail a log file and stream new lines to daemon

#### How:
```yaml
backend:
  logs:
    - type: file
      path: /var/log/app.log
      poll_interval_ms: 100
```

#### Data captured:
- New lines added to file
- Polls every 100ms (configurable)
- Handles file rotation (if log file disappears, reopen)
- Extracts: timestamp, level, message, correlation_id

#### Success Criteria:
- [ ] Detects new lines within 100ms of being written
- [ ] Handles file rotation gracefully
- [ ] Memory efficient (doesn't buffer entire file)
- [ ] Continues from last position on restart
- [ ] Works with large log files (1GB+)

---

### Feature 3: SSH Remote Log Tailer (Production Distributed)
**What:** SSH into remote server and tail log file

#### How:
```yaml
backend:
  logs:
    - type: ssh
      host: prod.example.com
      user: ubuntu
      path: /var/log/app.log
      auth: ~/.ssh/id_rsa
```

#### Data captured:
- SSH connection to remote server
- Runs `tail -f` on remote server
- Streams back to local daemon
- Same log parsing as local file

#### Success Criteria:
- [ ] SSH connection established within 5 seconds
- [ ] Log lines stream within 500ms (network dependent)
- [ ] Reconnects on SSH timeout
- [ ] Continues tailing after brief network interruptions
- [ ] Supports key-based auth (no password)

---

### Feature 4: Multi-Log Source Configuration
**What:** Support multiple log sources simultaneously

#### How:
```yaml
backend:
  logs:
    # Service A
    - type: file
      path: /var/log/service-a.log
      tags: ["service:a"]

    # Service B (remote)
    - type: ssh
      host: service-b.prod.com
      path: /var/log/app.log
      tags: ["service:b"]

    # Service C (remote)
    - type: ssh
      host: service-c.prod.com
      path: /var/log/app.log
      tags: ["service:c"]
```

#### Data captured:
- All sources stream to same daemon
- Events tagged with source service
- Query can filter by service or aggregate across all

#### Success Criteria:
- [ ] All sources stream simultaneously
- [ ] No interference between sources
- [ ] Query can show "all logs" or "logs from service X"
- [ ] Supports 5+ concurrent log sources
- [ ] Graceful degradation if one source fails

---

### Feature 5: Log Parser (Multi-Format)
**What:** Parse logs in any format and extract key fields

#### Supported formats:
- JSON (auto-detect)
- Structured plaintext (regex patterns)
- Simple plaintext (fallback)

#### How it works:
1. **JSON:** Auto-extract standard fields
   ```json
   {
     "timestamp": "2024-01-01T12:00:00Z",
     "level": "info",
     "message": "Request processed",
     "trace_id": "abc123"
   }
   ```

2. **Structured:** User provides regex pattern
   ```
   Pattern: ^\[(.*?)\]\s+(\w+)\s+\[(.*?)\]\s+(.*)$
   Example: [2024-01-01T12:00:00Z] INFO [trace:abc123] Request processed
   Extract: timestamp, level, trace_id, message
   ```

3. **Simple:** No structure, just text
   ```
   "User logged in"
   → timestamp: now, level: info (guessed), message: "User logged in"
   ```

#### Data captured:
- Timestamp (auto-detected format)
- Log level (normalized: debug, info, warn, error, critical)
- Message (primary text)
- Correlation ID (extracted from: req, trace_id, request_id, correlation_id, etc.)
- Metadata (all other fields preserved)

#### Success Criteria:
- [ ] JSON logs parsed 100% correctly
- [ ] Structured logs with regex provided parsed correctly
- [ ] Simple logs never crash (graceful fallback)
- [ ] Correlation ID detected in 95% of logs
- [ ] Timestamp auto-detected in common formats
- [ ] Performance: Parse 10K log lines in <100ms

---

### Feature 6: Correlation & Merging
**What:** Link FE and BE events by correlation_id (trace ID)

#### How:
1. **FE generates trace ID:** UUID (e.g., `550e8400-e29b-41d4`)
2. **FE sends in request:** `X-Trace-ID: 550e8400-e29b-41d4`
3. **BE logs include trace ID:** `[trace:550e8400-e29b-41d4] Processing`
4. **Parser extracts:** `correlation_id: "550e8400-e29b-41d4"`
5. **Query merges:** All events with same `correlation_id`

#### Result:
```
Timeline (sorted by timestamp):
1. [FE] Click "Add to Cart" (12:00:00.000)
2. [FE] POST /api/cart (12:00:00.002)
3. [BE] Received request (12:00:00.003)
4. [BE] Inserted into database (12:00:00.015)
5. [FE] Response 200 (12:00:00.025)
```

#### Fallback (no trace ID):
- Match by timestamp + endpoint (less reliable)
- Flag as "inferred" (for AI context)

#### Success Criteria:
- [ ] All FE requests include X-Trace-ID
- [ ] All BE log entries include trace ID
- [ ] Query by correlation_id returns complete flow
- [ ] Fallback matching works for untagged logs

---

## Feature 7: Clock Skew Detection (v5.4) & Correction (v6.0+)

**What (v5.4):** Detect clock differences between FE and BE; report to operator

**What (v6.0+):** Automatically correct detected clock skew

### Why:
- Local dev: FE and BE are same machine, should have <10ms skew
- Production: FE (laptop) and BE (prod server) may differ by seconds
- Timeline order may be wrong without awareness of skew
- Correction is complex: requires algorithm refinement, testing

### Approach (v5.4 - Detection Only):

1. **FE sends local time:** `X-Client-Time: 1704067200000` in every request
2. **BE calculates offset:** `server_time - client_time = skew_ms`
3. **BE logs skew:** `"[timestamp] INFO [req:abc123] client_skew_ms:5 Processing"`
4. **Parser extracts:** `metadata: {client_skew_ms: 5}`
5. **Daemon accumulates:** Latest 30 skew samples from recent requests
6. **Reports status:** Via GET /daemon/status endpoint (operator reads, no auto-correction)

### Health Endpoint (v5.4):

```json
GET /daemon/status

{
  "clock_skew": {
    "samples_collected": 42,
    "median_skew_ms": 5,
    "p50_skew_ms": 5,
    "p95_skew_ms": 12,
    "p99_skew_ms": 25,
    "std_dev_ms": 3.2,
    "confidence_95_ms": "[2ms, 8ms]",
    "status": "synchronized",
    "algorithm": "Median of last 30 samples; outliers >3σ removed",
    "recommendation": "Same machine (FE and BE synchronized). No action needed.",
    "remediation_if_large": "If status is 'large_skew': (1) Check FE clock: `date`; (2) Check BE clock: SSH to server and run `date`; (3) Sync if needed: `sudo ntpdate -s time.nist.gov`"
  }
}
```

### Status Values (v5.4):

- `"synchronized"` — Median skew < 10ms AND std_dev < 5ms (same machine, NTP-aligned)
- `"detected"` — Median skew 10-100ms (acceptable, different machines or transient delays, report to user)
- `"large_skew"` — Median skew >100ms (warn operator, likely NTP issue or clock misconfiguration)

### Detection Algorithm (v5.4):

1. **Collect samples:** Extract `client_skew_ms` from BE logs for each FE request
2. **Filter outliers:** Remove samples where |sample - median| > 3 * std_dev
3. **Calculate statistics:**
   - Median (p50): Middle value of 30 most recent stable samples
   - p95, p99: Percentiles of recent samples
   - Std_dev: Standard deviation of recent samples
   - Confidence interval (95%): median ± 1.96 * std_dev
4. **Determine status:** Based on median and std_dev values
5. **Report:** Via `/daemon/status` endpoint with actionable remediation

### Rationale for Algorithm:
- **Median instead of mean:** Resistant to outliers (e.g., GC pauses, network jitter)
- **Outlier removal (3σ):** Filters transient network delays that could cause false alerts
- **Percentiles (p95, p99):** Help operator understand if skew is stable or highly variable
- **Confidence interval:** Quantifies uncertainty in the skew measurement
- **30-sample threshold:** Requires ~30 requests before declaring stable state (typical dev session reaches this in <1 minute)

### Auto-Correction (v6.0+, Out of Scope v5.4):

- After skew stabilizes (30+ samples, std-dev <5ms)
- Automatic FE timestamp adjustment by median skew
- User can disable via config: `clock_skew_auto_correct: false`

### Success Criteria (v5.4):

- [ ] Skew detected and reported within first 30 requests
- [ ] Same-machine: Reports <10ms with "synchronized" status
- [ ] Different machines: Reports skew with "detected" or "large_skew" status
- [ ] Operator can read `/daemon/status` to understand clock alignment
- [ ] No automatic adjustment (deferred to v6.0)

---

## API Contracts: Event Ingestion Endpoints

### Endpoint 1: POST /event (Single Event)

**Used by:** Browser extension, local file tailer, SSH tailer

#### Request:

```json
POST http://localhost:7890/event
Content-Type: application/json

{
  "timestamp": 1704067200000,
  "level": "info",
  "source": "stdout",
  "message": "Request received",
  "metadata": {
    "correlation_id": "550e8400-e29b-41d4"
  },
  "tags": ["dev"]
}
```

#### Response:

```json
200 OK
{
  "status": "ok",
  "event_id": "evt_12345"
}
```

### Endpoint 2: POST /events (Batch)

**Used by:** kaboom-run wrapper (for efficiency)

#### Request:

```json
POST http://localhost:7890/events
Content-Type: application/json

{
  "events": [
    { ...event1... },
    { ...event2... },
    { ...event3... }
  ]
}
```

#### Response:

```json
200 OK
{
  "status": "ok",
  "count": 3,
  "event_ids": ["evt_12345", "evt_12346", "evt_12347"]
}
```

#### Batch Parameters:

- Batch size: Up to 10K events per batch
- Timeout: Send every 100ms or when buffer fills
- Retry on 5xx: Yes, keep trying until daemon comes back online
- Retry on 4xx: No, discard batch (indicates client error)

### Common Event Fields

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| timestamp | number (ms) | Yes | Source system time, not daemon receive time |
| level | string | Yes | One of: debug, info, warn, error, critical |
| source | string | Yes | One of: stdout, stderr, extension, file, ssh |
| message | string | Yes | Log message text |
| metadata | object | No | Any custom fields (correlation_id, client_skew_ms, etc.) |
| tags | array | No | Optional tags (e.g., ["dev", "prod", "service:auth"]) |

---

## Architecture: Daemon with Multiple Responsibilities

### Current v5.3 Daemon
```
HTTP Server (localhost:7890)
├─ POST /event ← Extension sends FE events
├─ GET /buffers/timeline ← LLM queries
└─ Ring Buffers ← Stores all events

MCP Server (stdio)
├─ observe() ← LLM queries
├─ generate() ← LLM requests tests
└─ → Reads from Ring Buffers
```

### Extended v5.4+ Daemon
```
HTTP Server (localhost:7890)
├─ POST /event ← Extension sends FE events (single)
├─ POST /event ← File/SSH tailers send BE events (single)
├─ POST /events ← kaboom-run sends batches (10K lines or 100ms timeout)
├─ GET /buffers/timeline ← LLM queries
└─ Ring Buffers ← Stores all events (merged by timestamp)

Log Ingestion (Concurrent goroutines)
├─ kaboom-run listener
│  ├─ Receives POST /event from wrapper
│  └─ Normalizes to NormalizedEvent
├─ File tailer
│  ├─ Polls local files
│  └─ Sends to HTTP POST /event
└─ SSH tailer
   ├─ SSH connection to remote
   └─ Sends to HTTP POST /event

MCP Server (stdio)
├─ observe() ← LLM queries
├─ generate() ← LLM requests tests
└─ → Reads from Ring Buffers (merged FE + BE)
```

### Key Design Decisions

1. **Single daemon process** (not multiple services)
   - Simpler deployment
   - Shared ring buffers (automatic merging)
   - Single binary stays zero-deps

2. **All ingest via HTTP POST /event**
   - kaboom-run sends HTTP
   - File tailer sends HTTP
   - SSH tailer sends HTTP
   - Consistent interface

3. **Concurrent ingest goroutines**
   - Each log source runs independently
   - Non-blocking to each other
   - Graceful degradation if one fails

4. **Ring buffers unchanged**
   - Events from all sources merge naturally by timestamp
   - Correlation by trace ID works across sources

5. **Backward compatible**
   - v5.3 users don't need to use BE log ingestion
   - v5.4 adds it as optional feature
   - Old configs still work

---

## Success Criteria

### Functional
- [ ] FE and BE events queryable together
- [ ] Correlation by trace ID works reliably
- [ ] kaboom-run wrapper works with any command
- [ ] SSH tailer works without password auth
- [ ] File tailer handles log rotation
- [ ] Multi-source logging works simultaneously

### Performance
- [ ] Log line ingestion: <10ms (local) or <500ms (SSH)
- [ ] Parser throughput: 10K lines/sec
- [ ] Memory footprint: <50MB overhead for log ingestion
- [ ] No impact on FE event latency
- [ ] No impact on MCP query latency

### Developer Experience
- [ ] Local dev: `kaboom-run npm run dev` just works
- [ ] Production: YAML config, no code changes
- [ ] Help text: `kaboom-run --help` explains usage
- [ ] Errors: Clear messages when config wrong
- [ ] Debugging: Logs show what's being ingested

### Backward Compatibility
- [ ] v5.3 config still works
- [ ] Extension still works
- [ ] MCP queries still work
- [ ] No required migrations

---

## Out of Scope (v5.4)

- [ ] Log aggregation platform APIs (BigQuery, Datadog, Splunk) → v7.0
- [ ] Log filtering/sampling (capture all, filter later)
- [ ] Custom log format DSL (regex patterns only)
- [ ] Structured log enrichment (store as-is)
- [ ] Multi-user/multi-tenant support
- [ ] Long-term retention (24h TTL)
- [ ] Log alerting/webhooks

---

## Related Documents

- **Architecture:** [layer1-be-observability.md](../../core/layer1-be-observability.md)
- **Target Architecture:** [target-architecture.md](../../core/target-architecture.md)

---

**Status:** Ready for spec review and tech spec development
**Owner:** (Assign 1-2 engineers)
**Duration:** 2 weeks (kaboom-run + file tailer + parser)
**Next:** Tech specs per process (kaboom-run, local tailer, SSH tailer)
