---
review_date: 2026-01-31
reviewer: Principal Engineer (AI)
spec_version: v1.0 (PRODUCT_SPEC.md, TECH_SPEC_KABOOM_RUN.md, TECH_SPEC_LOCAL_TAILER.md, TECH_SPEC_SSH_TAILER.md)
status: NEEDS REVISION
relates_to: [PRODUCT_SPEC.md, TECH_SPEC_KABOOM_RUN.md, TECH_SPEC_LOCAL_TAILER.md, TECH_SPEC_SSH_TAILER.md]
last-updated: 2026-01-31
last_reviewed: 2026-02-16
---

# Backend Log Ingestion — Spec Review

**Review Timing:**
| Agent | Round | Start | End | Duration |
|-------|-------|-------|-----|----------|
| Principal Engineer | R1 | 14:30 UTC | 14:42 UTC | 12m |

---

## Executive Summary

The backend log ingestion specifications introduce a sophisticated multi-tailer architecture that significantly extends Kaboom's observability capabilities. The design demonstrates strong architectural thinking around separation of concerns (kaboom-run wrapper, local tailer, SSH tailer) and proper use of Go concurrency primitives. However, the specifications contain **8 critical issues** in performance guarantees, concurrency safety, data contract consistency, and error handling that must be addressed before implementation begins.

**Key Concern:** The unbounded memory queue pattern in kaboom-run contradicts stated durability guarantees and creates backpressure failure modes that will cause dev server hangs under high throughput.

**Status:** BLOCKED until critical issues are resolved.

---

## Critical Issues (Must Fix Before Implementation)

### 1. Unbounded Memory Queue Creates Backpressure Failure Mode
- **Location:** TECH_SPEC_KABOOM_RUN.md, lines 398-547
- **Severity:** CRITICAL
- **Issue:**
  - Specification claims "unbounded channel" (line 544) with "Non-blocking send" pseudocode (line 438)
  - Go channels cannot be truly unbounded; all channels have finite buffer
  - Under high throughput (100+ lines/sec), ingestion goroutine will block when queue fills
  - Violates guarantee: "ingestion must never block or slow down subprocess" (line 545)
- **Impact:** Dev server's log I/O will hang under sustained load
- **Test Case:** kaboom-run with Node server logging 1000 lines/sec for 10 seconds
- **Recommended Fix:**
  ```go
  // Replace unbounded channel with bounded queue
  const MAX_QUEUE_SIZE = 10000  // 10K events ≈ 10MB

  // Option A: Bounded channel with explicit drop policy
  memoryQueue := make(chan LogEvent, MAX_QUEUE_SIZE)

  // Option B: Custom bounded queue with backpressure signal
  type BoundedQueue struct {
    mu    sync.Mutex
    queue []*LogEvent
    cond  *sync.Cond
  }
  ```
  - Define backpressure behavior when queue reaches capacity
  - Document whether to: drop oldest lines, block subprocess, or error to caller
  - Add metrics: queue depth, drop count, backpressure events

**PR Blocker:** Yes. Must be resolved before kaboom-run implementation begins.

---

### 2. File Rotation Reader/Writer Race Condition
- **Location:** TECH_SPEC_KABOOM_RUN.md, lines 479-540
- **Severity:** CRITICAL
- **Issue:**
  - Claims "clean reader/writer separation" with `.active` and `.NNN` files
  - No atomic marker for "completed" file
  - When Goroutine 2 rotates `buffer.active.jsonl` → `buffer.001.jsonl`, Goroutine 3 may be mid-read
  - Pseudocode: "Reader only touches completed `.NNN` files" but no exclusion mechanism
- **Impact:** Lines could be duplicated, skipped, or partially read. Durability guarantee violated.
- **Test Case:** Concurrent rotation every 10ms with reader processing 100-line batches
- **Recommended Fix:**
  ```go
  // Two-phase commit pattern:
  // 1. Writer writes to buffer.active.jsonl, syncs
  // 2. Writer renames to buffer.NNN.jsonl (atomic on POSIX)
  // 3. Reader waits for file to be "stable" (age > 1 second)
  // 4. Reader reads entire file, then fsync()
  // 5. Reader deletes file only after successful POST to daemon

  // Add explicit coordination:
  type BufferFile struct {
    Path      string
    CreatedAt time.Time  // When file was created
    ClosedAt  *time.Time // When rotation completed (nil = still active)
  }

  // Reader pseudocode:
  // - List all files
  // - Skip files where (now - ClosedAt) < 1 second  // Still being rotated
  // - Read from oldest ClosedAt files first
  ```
  - Implement inode tracking to detect incomplete rotations
  - Write integration test: 2 goroutines, concurrent read/write, verify no data loss
- **Atomicity Guarantees:** Document POSIX requirements (rename is atomic)

**PR Blocker:** Yes. Must have passing integration tests before merge.

---

### 3. Clock Skew Detection Accuracy Understated
- **Location:** PRODUCT_SPEC.md, lines 332-387
- **Severity:** HIGH
- **Issue:**
  - Claims "Operator can read `/daemon/status` to understand clock alignment"
  - Status values ("synchronized", "detected", "large_skew") not actionable
  - Algorithm for detecting systematic vs. transient skew not specified
  - With only 30 samples, transient network delays could incorrectly trigger alerts
  - Recommendation "Same machine, within acceptable range" is vague
- **Impact:** Operators may misinterpret alerts, causing incorrect manual adjustments
- **Recommended Fix:**
  ```json
  GET /daemon/status

  {
    "clock_skew": {
      "samples_collected": 42,
      "median_skew_ms": 5,
      "p95_skew_ms": 12,
      "p99_skew_ms": 25,
      "std_dev_ms": 3.2,
      "status": "detected",
      "confidence_95": "[3ms, 7ms]",
      "recommendation": "Same machine (FE laptop + prod server). NTP-aligned. No action needed.",
      "algorithm": "Median of last 30 stable samples (outliers >3σ removed)"
    }
  }
  ```
  - Report percentiles (p95, p99) not just median
  - Include confidence interval
  - Define stability threshold: require std_dev < 5ms for 30+ consecutive samples
  - Provide specific remediation steps in recommendation field
  - Version the algorithm in response (for future v6.0 changes)

**PR Blocker:** No. Can ship v5.4 with detection-only. Defer auto-correct to v6.0+.

---

### 4. SSH Reconnection Strategy Under Network Partition
- **Location:** TECH_SPEC_SSH_TAILER.md, lines 339-365
- **Severity:** HIGH
- **Issue:**
  - Fixed 5-second backoff with "unlimited retries" (line 342)
  - No circuit breaker, max retry count, or exponential backoff
  - Under network partition, will continuously attempt reconnect every 5 seconds
  - Will exhaust file descriptors, goroutines, system resources
  - Note "operator intervention stops" is not a viable production failure mode
- **Impact:** On extended network outages (hours), daemon resource usage grows unbounded
- **Test Case:** SSH server down for 1 hour; verify file descriptor count stays bounded
- **Recommended Fix:**
  ```go
  // Circuit breaker pattern
  const MAX_CONSECUTIVE_FAILURES = 10
  const BACKOFF_INITIAL_MS = 5000
  const BACKOFF_MAX_MS = 120000
  const BACKOFF_MULTIPLIER = 2.0

  type SSHTailer struct {
    consecutiveFailures int
    backoffMS          int64
    lastFailureTime    time.Time
  }

  func (t *SSHTailer) Start(ctx context.Context) error {
    for {
      if t.consecutiveFailures > MAX_CONSECUTIVE_FAILURES {
        // Circuit breaker: exponential backoff
        waitTime := time.Duration(t.backoffMS) * time.Millisecond
        log.Printf("[ssh-tailer] Circuit breaker active. Retrying in %v", waitTime)
        select {
        case <-ctx.Done():
          return ctx.Err()
        case <-time.After(waitTime):
          t.backoffMS = min(int64(float64(t.backoffMS)*BACKOFF_MULTIPLIER), BACKOFF_MAX_MS)
          continue
        }
      }

      err := t.connect()
      if err != nil {
        t.consecutiveFailures++
        if t.consecutiveFailures == 10 {
          log.Printf("[WARN] SSH tailer %s: 10 consecutive failures. Entering circuit breaker.", t.host)
        }
        if t.consecutiveFailures == 30 {
          log.Printf("[ERROR] SSH tailer %s: 30 consecutive failures over 5 minutes. Manual intervention required.", t.host)
        }
        continue
      }

      // Success: reset
      t.consecutiveFailures = 0
      t.backoffMS = BACKOFF_INITIAL_MS

      err = t.stream(ctx)
      if err != nil {
        t.recordError(err)
        continue
      }
    }
  }
  ```
  - Implement exponential backoff: 5s → 10s → 20s → 40s → max 120s
  - Add jitter (±10%) to prevent thundering herd
  - Log critical alert after 10 failures
  - Add circuit breaker state to health check endpoint
  - Document SSH session timeout behavior

**PR Blocker:** Yes. Must have circuit breaker before production deployment.

---

### 5. Local File Tailer Doesn't Handle All Rotation Patterns
- **Location:** TECH_SPEC_LOCAL_TAILER.md, lines 143-175
- **Severity:** HIGH
- **Issue:**
  - Only handles size-based rotation (detecting size decrease)
  - Doesn't handle common patterns:
    - **copytruncate:** File truncated in place; inode unchanged. Tailer reads same lines twice.
    - **Atomic rename:** Old file moved to `.1`, new file created. Inode changes; tailer may miss lines written between size check and reopen.
    - **Delayed compression:** Rotated file renamed to `.1.gz`. Not detected at all.
- **Impact:** Log lines could be lost or duplicated depending on rotation strategy used
- **Test Cases:**
  1. copytruncate: Write 100 lines → truncate → write 100 more → verify no duplicates
  2. Atomic rename: Write 100 lines → rename to `.1` → create new file → verify no missed lines
  3. Delayed compress: Verify detection of `.gz` files
- **Recommended Fix:**
  ```go
  type FileTailer struct {
    path       string
    file       *os.File
    inode      uint64        // Track inode to detect rotation
    lastOffset int64
    lastSize   int64
  }

  func (t *FileTailer) Poll() error {
    stat, _ := os.Stat(t.path)
    currentInode := getInode(stat)  // Platform-specific
    currentSize := stat.Size()

    // Case 1: Inode changed = atomic rename
    if currentInode != t.inode {
      log.Printf("[file-tailer] Inode changed: %v → %v (rotation detected)", t.inode, currentInode)
      t.file.Close()
      t.file = openNewFile(t.path)
      t.inode = currentInode
      t.lastOffset = 0
      return nil
    }

    // Case 2: Size decreased = copytruncate or manual truncation
    if currentSize < t.lastSize {
      log.Printf("[file-tailer] Size decreased: %d → %d (copytruncate detected)", t.lastSize, currentSize)
      t.file.Seek(0, 0)
      t.lastOffset = 0
      t.lastSize = 0
      return nil
    }

    // Case 3: Size increased = new lines
    if currentSize > t.lastOffset {
      t.file.Seek(t.lastOffset, 0)
      // ... read new lines
    }

    t.lastSize = currentSize
    return nil
  }
  ```
  - Add inode tracking for all platforms (Linux/macOS via Stat.Sys())
  - Handle copytruncate by re-seeking to 0 after size decrease
  - Handle compressed rotations: check for `.gz` suffix and skip
  - Write integration tests for each rotation pattern (use logrotate config)
  - Document supported patterns and tested OS versions

**PR Blocker:** Yes. Must test all 3 rotation patterns before merge.

---

### 6. API Contract Ambiguity: Event vs Events Endpoint
- **Location:** TECH_SPEC_KABOOM_RUN.md line 562 vs PRODUCT_SPEC.md line 410
- **Severity:** HIGH
- **Issue:**
  - PRODUCT_SPEC mentions `POST /event` (singular)
  - TECH_SPEC_KABOOM_RUN describes `POST /events` (plural with batch)
  - Daemon not specified to handle both formats
  - TECH_SPEC_LOCAL_TAILER mentions `SendEvent(event)` (singular)
  - This inconsistency will cause integration failures
- **Impact:** Different tailers send to incompatible endpoints → "endpoint not found" errors
- **Recommended Fix:**
  ```go
  // Daemon must support both formats for backward compatibility

  // Format 1: Single event (for compat with local tailer)
  POST /event
  Content-Type: application/json

  {
    "timestamp": 1704067200000,
    "source": "stdout",
    "message": "[INFO] Request received",
    "correlation_id": "550e8400-e29b-41d4",
    "level": "info"
  }

  Response:
  {
    "status": "ok",
    "event_id": "evt_12345"
  }

  // Format 2: Batch events (for kaboom-run efficiency)
  POST /events
  Content-Type: application/json

  {
    "events": [
      {...event1...},
      {...event2...}
    ]
  }

  Response:
  {
    "status": "ok",
    "event_ids": ["evt_12345", "evt_12346"],
    "failed": []
  }
  ```
  - Daemon accepts both `/event` and `/events` endpoints
  - Default batch size: 10K lines or 100ms timeout
  - Add response field for event IDs to enable retries
  - Define version in API: `/v1/event` for future-proofing
  - Update PRODUCT_SPEC to document both endpoints

**PR Blocker:** Yes. Must finalize before any tailer implementation.

---

### 7. Correlation ID Extraction Reliability Not Quantified
- **Location:** PRODUCT_SPEC.md line 294; TECH_SPEC_KABOOM_RUN.md lines 291-307
- **Severity:** MEDIUM
- **Issue:**
  - 95% success rate stated but not defined
  - Regex patterns (lines 291-307) are basic
  - Won't match common trace ID formats:
    - AWS X-Ray: `1-5e6722a7-cc2c2a3ac9b6bdf330b34215`
    - Jaeger: `4bf92f3577b34da6a3ce929d0e0e4736`
    - W3C Trace Context: `00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01`
  - OpenTelemetry, Zipkin, etc. all have different formats
  - Logs without explicit trace IDs fall back to timestamp matching (unreliable)
- **Impact:** Many real-world services will have <95% correlation success
- **Recommended Fix:**
  ```go
  // Expand regex patterns to support common formats
  var correlationPatterns = []struct {
    name    string
    pattern string
  }{
    // Standard formats
    {"uuid", `[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`},
    {"uuid_compact", `[a-f0-9]{32}`},

    // AWS X-Ray
    {"xray", `1-[a-f0-9]{8}-[a-f0-9]{24}`},

    // W3C Trace Context
    {"w3c", `[a-f0-9]{32}-[a-f0-9]{16}-[a-f0-9]{2}`},

    // Field names to search
    {"req_id", `(?i)(?:req|request|trace|span|correlation)[\s:_=]+([a-zA-Z0-9-]+)`},
  }
  ```
  - Document 95% target as "for logs with explicitly embedded correlation IDs in standard formats"
  - Expand regex to support AWS X-Ray, Jaeger, W3C Trace Context, OpenTelemetry
  - Pre-compile all patterns at daemon startup
  - Add test cases for popular frameworks (Express.js, Flask, Go stdlib, etc.)
  - Consider adding optional correlation_id field to config (required vs best-effort)

**PR Blocker:** No. Can improve post-launch. Document 95% target with caveats for v5.4.

---

### 8. Missing Specification for Daemon's Ring Buffer Integration
- **Location:** PRODUCT_SPEC.md lines 391-429 (no corresponding tech spec)
- **Severity:** HIGH
- **Issue:**
  - PRODUCT_SPEC describes "new log ingestion layer" within daemon
  - No TECH_SPEC_DAEMON.md file exists
  - Unspecified:
    1. How events routed to existing ring buffers
    2. How daemon merges FE (extension) + BE (tailers) by correlation_id
    3. What happens if correlation_id spans multiple ring buffers
    4. Query interface changes needed
    5. Backward compatibility approach
  - Implicitly assumes existing ring buffer code extends without breaking changes
- **Impact:** Tailer implementation could complete successfully but daemon integration fails
- **Recommended Fix:**
  - **Create TECH_SPEC_DAEMON_INGESTION.md specifying:**
    ```
    1. Ring Buffer Strategy
       - Add new `backend_logs` ring buffer separate from FE events?
       - Or merge into existing ring buffer with source tag?
       - Implications for buffer capacity and query performance

    2. Event Routing
       - POST /event handler logic
       - How to store and index by correlation_id
       - How to query events by trace ID across sources

    3. Query API Changes
       - New filter: `source: "stdout" | "file" | "ssh"`
       - New filter: `correlation_id: "550e8400-e29b-41d4"`
       - New endpoint: GET /buffers/timeline?correlation_id=XXX
       - Should return merged FE+BE timeline

    4. Backward Compatibility
       - v5.3 clients should still work with v5.4 daemon
       - Existing queries should work with new events mixed in
       - How to handle daemon restarts with disk buffer?
    ```

**PR Blocker:** Yes. Blocks daemon integration work.

---

## Recommendations (Should Consider)

### R1: Explicit Backpressure Mechanism
Rather than claiming an "unbounded" queue, implement explicit backpressure signaling. When disk buffer reaches threshold (e.g., 500MB), signal ingestion goroutine to apply backpressure (log warning, optionally drop lines). Makes failure modes explicit and testable.

### R2: Pre-Compile Regex Patterns
Both TECH_SPEC_KABOOM_RUN.md and TECH_SPEC_LOCAL_TAILER.md mention log parsing with regex. Pre-compile all patterns at daemon startup to avoid repeated compilation overhead. Critical for 10K lines/sec throughput target.

### R3: Add Correlation ID Validation
When receiving events, daemon should validate correlation_id is valid format (UUID, hex-string, or expected format). Prevents accidentally matching unrelated events. Catch misconfigured services early.

### R4: SSH Key Rotation & Expiry
SSH tailer reads private key once at startup (line 83). If key is rotated or expires, tailer won't detect it until reconnection fails. Consider adding periodic key reload or key expiry detection.

### R5: Local File Tailer Should Track Inode
Store inode of opened file when polling. Compare inode to detect rotation. Catches atomic-rename and copytruncate rotations that position-based detection misses.

### R6: Clock Skew Reporting Should Include Distribution
Current status endpoint only reports `median_skew_ms`. Add percentiles (p95, p99) to help operators understand if skew is stable or varies widely (indicating real clock drift vs. transient network delay).

### R7: Define Success Metrics for Testing
Testing strategy lists manual tests but no quantitative success criteria. Add:
- kaboom-run overhead <5ms per 100 log lines
- Memory usage <50MB for 1 hour of 1000 lines/sec
- SSH reconnect <2s after network recovery
- File tailer rotation detection within 100ms

### R8: Explicit Data Retention Policy
Disk buffer has 24-hour TTL, but unclear if applies to all events. Clarify: Do FE events from extension also have 24h TTL? How does retention interact with ring buffer size limits?

---

## Implementation Roadmap (Ordered by Dependency)

### Phase 1: Spec Clarification & Daemon Design (3 days)
1. Resolve API contract (`POST /event` vs `/events`) — **1 day**
2. Create TECH_SPEC_DAEMON_INGESTION.md — **2 days**
3. Get design review sign-off from principal engineer

### Phase 2: Core Infrastructure (5 days)
4. Fix bounded memory queue implementation (kaboom-run) — **1 day**
5. Fix file rotation with atomicity guarantees (kaboom-run) — **2 days**
6. Implement Log Parsers (JSON, structured, plaintext) with expanded regex — **2 days**

### Phase 3: kaboom-run Executable (5 days)
7. Implement 3-goroutine pipeline (ingestion, disk buffer, sender) — **3 days**
8. Integration & benchmarking tests — **2 days**

### Phase 4: Tailers (8 days)
9. Local File Tailer with inode tracking & rotation patterns — **3 days**
10. SSH Tailer with circuit breaker & exponential backoff — **3 days**
11. Integration tests for both tailers — **2 days**

### Phase 5: Daemon Integration (4 days)
12. Implement daemon ingestion layer per TECH_SPEC_DAEMON_INGESTION.md — **3 days**
13. End-to-end testing (FE + BE correlation) — **1 day**

### Phase 6: Clock Skew Detection (2 days)
14. Implement clock skew detection with percentiles — **1 day**
15. Daemon integration & health endpoint — **1 day**

**Total Effort: 27 days (4-5 weeks)**

---

## Risk Assessment

### Critical Risks

1. **Concurrency Under Load**
   - Risk: Subtle race conditions under sustained high load (1000+ lines/sec)
   - Mitigation: Add benchmarks and stress tests before release
   - Verification: Goroutine leak detection, data race detector

2. **File System Compatibility**
   - Risk: File rotation detection assumes POSIX semantics; Windows/NFS may differ
   - Mitigation: Document supported platforms; add conditional code for Windows
   - Verification: Test on Windows and NFS in staging

3. **SSH Security**
   - Risk: Insecure host key verification fallback creates MITM vulnerability
   - Mitigation: Require explicit opt-in; default to ~/.ssh/known_hosts
   - Verification: Security review of SSH key handling

4. **Correlation ID Collisions**
   - Risk: Non-unique correlation IDs cause events to merge incorrectly
   - Mitigation: Document correlation ID format requirements; add validation
   - Verification: Monitoring for duplicate correlation IDs

### Medium Risks

5. **Clock Skew Misinterpretation**
6. **SSH Resource Exhaustion**
7. **Disk Full Scenario**
8. **Log Parser False Positives**

(See full review output for details on each risk.)

---

## Critical Path Dependencies

```
TECH_SPEC_DAEMON_INGESTION.md (Critical)
  ↓
API Contract Resolution (Critical)
  ↓
Bounded Queue Implementation (Critical)
  ↓
File Rotation Atomicity (Critical)
  ↓
kaboom-run Executable
  ↓
Tailer Implementations (can run in parallel)
  ↓
Daemon Integration
  ↓
End-to-End Testing
```

---

## Sign-Off Checklist

- [ ] Issue #1: Unbounded queue — FIXED and tested
- [ ] Issue #2: File rotation race — FIXED and tested
- [ ] Issue #3: Clock skew accuracy — CLARIFIED in spec
- [ ] Issue #4: SSH reconnection — FIXED with circuit breaker
- [ ] Issue #5: File rotation patterns — FIXED with inode tracking
- [ ] Issue #6: API contract — RESOLVED (dual endpoint support)
- [ ] Issue #7: Correlation ID patterns — EXPANDED regex
- [ ] Issue #8: Daemon spec — TECH_SPEC_DAEMON_INGESTION.md CREATED
- [ ] All recommendations reviewed
- [ ] Implementation roadmap approved
- [ ] Risk assessment sign-off

---

## Next Steps

**For Review Authors:** Address the 8 critical issues and recommendations in the respective specification files.

**For Implementation Lead:**
1. Obtain approval on TECH_SPEC_DAEMON_INGESTION.md before starting coding
2. Write tests for Critical Issues #1, #2, #4, #5 before implementation
3. Execute Phase 1 (spec clarification) immediately
4. Schedule architectural review with team before Phase 2 begins

**Timeline:** This review gates implementation. No coding should begin until Critical Issues 1, 2, 4, 5, 6, and 8 are resolved and re-reviewed.

---

**Status:** NEEDS REVISION → Re-review after fixes

**Last Updated:** 2026-01-31 14:42 UTC

