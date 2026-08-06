---
status: draft
priority: tier-1
phase: v5.4-foundation
relates-to: [PRODUCT_SPEC.md, TECH_SPEC_KABOOM_RUN.md]
last-updated: 2026-01-31
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Backend Log Ingestion: Local File Tailer — Technical Specification

**Goal:** Poll and tail local log files, stream new lines to daemon ring buffer.

---

## Process Architecture

```
Kaboom Daemon
   │
   ├─ HTTP Server (localhost:7890)
   ├─ MCP Server (stdio)
   └─ Log Ingestion Goroutines
       │
       ├─ kaboom-run Listener (accepts HTTP /event)
       │
       ├─ Local File Tailer (this spec)
       │  ├─ Poll: /var/log/app.log every 100ms
       │  ├─ Read: New lines since last position
       │  ├─ Parse: Extract timestamp, level, message, correlation_id
       │  └─ Send: HTTP POST /event → Ring Buffer
       │
       └─ SSH Tailer
          └─ (separate spec)

Ring Buffer
   └─ Stores: FE events + BE events (merged by timestamp)
```

---

## Implementation

### File: `pkg/ingest/local_tailer.go`

```go
package ingest

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// LocalFileTailer polls a local file for new lines
type LocalFileTailer struct {
	path         string
	pollInterval time.Duration
	file         *os.File
	lastPos      int64
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	handler      LogLineHandler  // Callback to process lines
	stats        TailerStats
}

// LogLineHandler processes each log line
type LogLineHandler func(ctx context.Context, line string) error

// TailerStats tracks tailer health
type TailerStats struct {
	LinesRead       int64
	LinesFailed     int64
	FileRotations   int64
	LastReadTime    time.Time
	LastErrorTime   time.Time
	LastError       string
}

// NewLocalFileTailer creates a new file tailer
func NewLocalFileTailer(
	path string,
	handler LogLineHandler,
) (*LocalFileTailer, error) {
	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("log file not found: %s", path)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Seek to end (only read new lines)
	lastPos, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to seek: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &LocalFileTailer{
		path:         path,
		pollInterval: 100 * time.Millisecond,
		file:         file,
		lastPos:      lastPos,
		ctx:          ctx,
		cancel:       cancel,
		handler:      handler,
	}, nil
}

// Start begins polling the file
func (t *LocalFileTailer) Start(ctx context.Context) error {
	defer func() {
		t.file.Close()
	}()

	ticker := time.NewTicker(t.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.ctx.Done():
			return fmt.Errorf("tailer stopped")
		case <-ticker.C:
			if err := t.poll(ctx); err != nil {
				t.recordError(err)
				// Continue polling even on error
			}
		}
	}
}

func (t *LocalFileTailer) poll(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Check if file still exists or was rotated
	stat, err := t.file.Stat()
	if err != nil {
		// File was deleted/rotated
		if err := t.handleFileRotation(); err != nil {
			return fmt.Errorf("failed to handle rotation: %w", err)
		}
		return nil
	}

	currentSize := stat.Size()

	// Check for file rotation (size decreased or inode changed)
	if currentSize < t.lastPos {
		if err := t.handleFileRotation(); err != nil {
			return fmt.Errorf("file rotated but recovery failed: %w", err)
		}
		return nil
	}

	// Read new data since last position
	if currentSize > t.lastPos {
		if err := t.readNewLines(ctx); err != nil {
			return err
		}
	}

	t.stats.LastReadTime = time.Now()
	return nil
}

func (t *LocalFileTailer) readNewLines(ctx context.Context) error {
	// Seek to last known position
	_, err := t.file.Seek(t.lastPos, io.SeekStart)
	if err != nil {
		return fmt.Errorf("seek failed: %w", err)
	}

	scanner := bufio.NewScanner(t.file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)  // 64KB default, 1MB max

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()

		// Call handler (send to daemon)
		if err := t.handler(ctx, line); err != nil {
			t.stats.LinesFailed++
			// Log but continue
		} else {
			t.stats.LinesRead++
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan error: %w", err)
	}

	// Update last position
	pos, err := t.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("get position failed: %w", err)
	}
	t.lastPos = pos

	return nil
}

func (t *LocalFileTailer) handleFileRotation() error {
	// File was rotated (size decreased)
	t.stats.FileRotations++

	// Close old file
	t.file.Close()

	// Wait a moment for new file to be created
	time.Sleep(100 * time.Millisecond)

	// Try to open file again
	file, err := os.Open(t.path)
	if err != nil {
		return fmt.Errorf("failed to reopen after rotation: %w", err)
	}

	// Check if this is truly a new file or if rotation hasn't completed
	stat, _ := file.Stat()
	if stat.Size() == 0 {
		// New empty file, start from beginning
		t.lastPos = 0
	} else {
		// Partial file, read from start
		t.lastPos = 0
	}

	t.file = file
	return nil
}

func (t *LocalFileTailer) recordError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stats.LastError = err.Error()
	t.stats.LastErrorTime = time.Now()
}

// Stop stops the tailer
func (t *LocalFileTailer) Stop() error {
	t.cancel()
	return t.file.Close()
}

// Stats returns current tailer statistics
func (t *LocalFileTailer) Stats() TailerStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.stats
}
```

---

## Integration with Daemon

### File: `internal/daemon/ingestion.go`

```go
package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/kaboom/pkg/ingest"
)

// IngestionManager manages all log sources
type IngestionManager struct {
	tailers map[string]*ingest.LocalFileTailer
	mu      sync.RWMutex
	sender  EventSender  // Sends events to ring buffer
}

// EventSender interface (implemented by daemon)
type EventSender interface {
	SendEvent(event *ingest.LogEvent) error
}

func NewIngestionManager(sender EventSender) *IngestionManager {
	return &IngestionManager{
		tailers: make(map[string]*ingest.LocalFileTailer),
		sender:  sender,
	}
}

// AddLocalFileTailer starts tailing a local file
func (im *IngestionManager) AddLocalFileTailer(
	ctx context.Context,
	path string,
) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	// Prevent duplicate tailers
	absPath, _ := filepath.Abs(path)
	if _, exists := im.tailers[absPath]; exists {
		return fmt.Errorf("tailer already running for %s", absPath)
	}

	// Create handler that processes log lines
	handler := func(ctx context.Context, line string) error {
		event := ingest.ParseLogLine(line, "backend")
		if event == nil {
			return nil
		}

		// Send to daemon
		return im.sender.SendEvent(event)
	}

	// Create tailer
	tailer, err := ingest.NewLocalFileTailer(absPath, handler)
	if err != nil {
		return err
	}

	im.tailers[absPath] = tailer

	// Start tailing (in background goroutine)
	go func() {
		if err := tailer.Start(ctx); err != nil {
			fmt.Printf("[ingest] Tailer error for %s: %v\n", absPath, err)
		}
	}()

	fmt.Printf("[ingest] Started tailing: %s\n", absPath)
	return nil
}

// Stats returns statistics for all tailers
func (im *IngestionManager) Stats() map[string]interface{} {
	im.mu.RLock()
	defer im.mu.RUnlock()

	stats := make(map[string]interface{})
	for path, tailer := range im.tailers {
		stats[path] = tailer.Stats()
	}

	return stats
}

// HTTP Handler: GET /ingest/stats
func (im *IngestionManager) StatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(im.Stats())
}
```

---

## Configuration

### YAML Schema

```yaml
backend:
  logs:
    # Local file tailer
    - type: file
      path: /var/log/app.log
      poll_interval_ms: 100  # How often to check for new lines
      enabled: true
```

### Loading Configuration

```go
// pkg/config/config.go

type LogSourceConfig struct {
	Type             string `yaml:"type"`  // "file", "ssh", "http"
	Path             string `yaml:"path"`  // For file tailer
	PollIntervalMs   int    `yaml:"poll_interval_ms"`
	Enabled          bool   `yaml:"enabled"`
}

func LoadConfig(path string) ([]LogSourceConfig, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config struct {
		Backend struct {
			Logs []LogSourceConfig `yaml:"logs"`
		} `yaml:"backend"`
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config.Backend.Logs, nil
}
```

---

## File Rotation Handling

### Supported Rotation Patterns

#### Pattern 1: Size-Based Rotation (Atomic Rename)

##### How it works:
- Logger writes to `app.log`
- When file reaches size limit: rename `app.log` → `app.log.1`
- Logger creates new `app.log` file (new inode)

##### Detection:
- Inode changes between polls (most reliable)
- Fallback: Size decreased from last known position

```
Before:  app.log (inode 12345, size 1GB, pos=1GB)
After:   app.log.1 (inode 12345, size 1GB)  ← old file with old inode
         app.log (inode 12346, size 0)      ← new file with new inode

Tailer detects:
- stat(app.log) → inode changed from 12345 to 12346
- Close old file handle
- Reopen app.log with new inode
- Continue reading from position 0
```

#### Pattern 2: Copytruncate (In-Place Truncation)

##### How it works:

- Logger writes to `app.log` (inode 12345)
- When file reaches limit: system copies file to `app.log.1`
- Then truncates `app.log` to 0 bytes (SAME inode)

##### Detection:

- Inode is UNCHANGED (new inode NOT created)
- Size decreased → indicates truncation

```
Before:  app.log (inode 12345, size 1GB, pos=1GB)
After:   app.log.1 (inode 99999, copy of old content)
         app.log (inode 12345, size 0)      ← SAME inode, truncated

Tailer detects:
- stat(app.log) → inode unchanged (12345 == 12345)
- BUT size decreased (1GB → 0)
- Seek to 0 and continue reading from new logs
```

##### Special handling for copytruncate:

```go
func (t *LocalFileTailer) poll() error {
  stat, _ := os.Stat(t.path)
  newInode := getInode(stat)
  currentSize := stat.Size()

  // Case 1: Inode changed = atomic rename
  if newInode != t.lastInode {
    t.file.Close()
    t.file = openFile(t.path)
    t.lastInode = newInode
    t.lastPos = 0
    return nil
  }

  // Case 2: Inode unchanged but size decreased = copytruncate
  if currentSize < t.lastPos {
    t.file.Seek(0, 0)        // Start from beginning
    t.lastPos = 0
    // Read new content that was just written
    return nil
  }

  // Case 3: Normal append
  if currentSize > t.lastPos {
    t.file.Seek(t.lastPos, 0)
    // Read new lines
  }
}
```

#### Pattern 3: Delayed Compression (Gzip Rotation)

##### How it works:

- Logger writes to `app.log`
- System rotates: renames `app.log` → `app.log.1`
- Later: `app.log.1` → `app.log.1.gz` (compressed, renamed)

##### Detection:

- Inode changes, can't reopen old file
- File renamed with `.gz` extension (skip, can't read directly)

##### Handling:

- Read all lines from `app.log.1` before it gets compressed
- Once compressed, skip `.gz` files
- Continue with new `app.log`

#### Pattern 4: Logrotate with Postrotate

##### How it works:

- Logrotate runs: renames `app.log` → `app.log-20260131` or `app.log.1`
- Creates new empty `app.log`
- Sends SIGHUP to logger (or logger detects new file via inode)

##### Detection:

- Inode change is the primary signal
- Size decrease is secondary

### Rotation Detection Algorithm

```go
type FileTailerState struct {
  Path      string
  File      *os.File
  Inode     uint64        // Track inode to detect atomic rename
  Size      int64         // Last known size
  Position  int64         // Last read position
  RotatedAt time.Time     // When last rotation detected
}

func (t *FileTailer) poll() error {
  stat, err := os.Stat(t.path)
  if err != nil {
    // File deleted or inaccessible, handle gracefully
    return t.reopenFile()
  }

  newInode := getInode(stat)     // stat.Sys().(*syscall.Stat_t).Ino (Linux)
  newSize := stat.Size()

  // ===== ROTATION DETECTION =====

  // 1. Inode changed? (Atomic rename or new file creation)
  if newInode != t.state.Inode {
    log.Printf("[file-tailer] Inode changed: %d → %d (rotation detected)", t.state.Inode, newInode)
    t.file.Close()
    t.file = reopen(t.path)
    t.state.Inode = newInode
    t.state.Position = 0
    t.state.Size = 0
    t.state.RotatedAt = time.Now()
    return nil
  }

  // 2. Size decreased? (Copytruncate rotation)
  if newSize < t.state.Position {
    log.Printf("[file-tailer] Size decreased: %d → %d (copytruncate detected)", t.state.Size, newSize)
    t.file.Seek(0, 0)
    t.state.Position = 0
    t.state.Size = 0
    t.state.RotatedAt = time.Now()
    return nil
  }

  // 3. Size increased? (Normal append)
  if newSize > t.state.Position {
    t.file.Seek(t.state.Position, 0)
    lines := readLines(t.file)
    newPos, _ := t.file.Seek(0, 1)  // Current file position
    t.state.Position = newPos
    t.state.Size = newSize
    return t.sendLines(lines)
  }

  return nil
}
```

### Rotation Detection Test Cases

| Scenario | Mechanism | Detection | Expected Result |
| --- | --- | --- | --- |
| Size-based rename | `mv app.log app.log.1; app.log` created | Inode change | Read continues from position 0 in new file |
| copytruncate | `cp app.log app.log.1; truncate app.log` | Size decrease + same inode | Seek(0), read new content, no duplicates |
| Delayed gzip | `app.log` → `app.log.1` then `app.log.1.gz` | Can't re-read after gzip | Skip `.gz` files, continue with `app.log` |
| Multiple rotations | Rotation every 10ms, tailer polls every 100ms | Batch multiple rotations | Process each in sequence, no data loss |
| Rotation during read | File rotated while scanner reading | Detect after read completes | Seek to checkpoint, resume from position |
| File deleted entirely | `rm app.log` (tailer still has fd) | Stat fails, fd still open | Try to read from fd (may work briefly); detect via inode check |

---

## Performance

### Throughput
- **Poll interval:** 100ms (configurable)
- **Lines per poll:** 10-1000 (depends on activity)
- **Parsing time:** <1ms per line
- **Typical impact:** <1% CPU, <5MB memory

### Memory
- **Per tailer:** ~10MB (includes 1MB buffer)
- **Per-line:** ~1KB in handler (temporary)
- **Overhead:** Minimal, no buffering

### Network
- **Via HTTP to daemon:** ~500B per line (payload)
- **Throughput:** 1000+ lines/sec sustainable

---

## Error Handling

| Error | Behavior |
|-------|----------|
| File not found | Log error, retry on next poll |
| File rotated | Detect by size decrease, reopen and continue |
| Permission denied | Log error, fail tailer |
| Inode changed | Treat as rotation, reopen |
| Disk full | Log error, continue (daemon will handle) |
| Line too long (>1MB) | Split or truncate, continue |

---

## Monitoring & Debugging

### Health Check
```bash
curl http://localhost:7890/ingest/stats
```

Response:
```json
{
  "/var/log/app.log": {
    "lines_read": 1500,
    "lines_failed": 2,
    "file_rotations": 1,
    "last_read_time": "2024-01-01T12:00:05Z",
    "last_error_time": null,
    "last_error": ""
  }
}
```

### Debug Logging
```
[ingest] Started tailing: /var/log/app.log
[ingest] Poll: read 42 lines (position: 50000)
[ingest] File rotated: /var/log/app.log (size decreased)
[ingest] Reopened: /var/log/app.log (new inode)
```

---

## Testing Strategy

### Unit Tests
- [ ] Poll detects new lines
- [ ] Position tracking accurate
- [ ] File rotation detected
- [ ] Handler errors don't crash tailer
- [ ] Buffer handling (large lines)

### Integration Tests
- [ ] Real file tailing works
- [ ] File rotation works
- [ ] Multiple tailers don't interfere
- [ ] Stats accurate

### Manual Tests
```bash
# Test 1: Basic tailing
$ touch /tmp/test.log
$ kaboom-run --local-tailer /tmp/test.log
$ echo "Hello" >> /tmp/test.log
# Verify: Message appears in daemon

# Test 2: File rotation
$ # Stop daemon, rotate logs, restart daemon
$ sudo mv /tmp/test.log /tmp/test.log.1
$ touch /tmp/test.log
$ # Verify: Daemon continues tailing

# Test 3: Large file
$ # Create 1GB log file
$ # Verify: Tailer handles efficiently
```

---

## Log Parser Implementation

### Supported Formats (v5.4)

The log parser tries exactly 3 patterns in this order. If none match, falls back to simple plaintext.

#### Pattern 1: JSON (Auto-Detect)

**Condition:** Line starts with `{` and contains valid JSON

##### Extracted Fields:
- `timestamp` — Any of: `timestamp`, `ts`, `time`, `date`
- `level` — Any of: `level`, `severity`, `lvl`
- `message` — Any of: `message`, `msg`, `text`
- `trace_id` — Any of: `trace_id`, `request_id`, `correlation_id`, `req_id`, `traceID`
- Other fields preserved in `metadata`

##### Example:
```json
{
  "timestamp": "2024-01-01T12:00:00Z",
  "level": "info",
  "message": "User logged in",
  "trace_id": "abc123"
}
```

##### Extracted Result:
```json
{
  "timestamp": 1704067200000,
  "level": "info",
  "message": "User logged in",
  "source": "backend",
  "metadata": {"trace_id": "abc123"},
  "tags": ["dev", "local"]
}
```

#### Pattern 2: Structured Plaintext

**Regex:** `^\[(.*?)\]\s+(\w+)\s+(?:\[(.*?)\])?\s*(.*)$`

##### Groups:
- Group 1: Timestamp (any format)
- Group 2: Level (single word: DEBUG, INFO, WARN, ERROR, FATAL, etc.)
- Group 3: Optional metadata/trace-id in brackets
- Group 4: Message (rest of line)

##### Examples:
```
[2024-01-01T12:00:00Z] INFO [req:abc123] User logged in
  → timestamp=1704067200000, level=info, trace_id=abc123, message="User logged in"

[12:00:00] WARN [trace_id:xyz789] Slow query detected
  → timestamp=<today 12:00:00>, level=warn, trace_id=xyz789, message="Slow query detected"

[2024-01-01T12:00:00+00:00] ERROR Failed to connect to database
  → timestamp=1704067200000, level=error, trace_id=<none>, message="Failed to connect to database"
```

##### Trace ID Extraction (from Group 3 and Message):

Pre-compiled regex patterns tried in order (matches first successful):

1. **Standard UUID format:** `[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`
   - Matches: `550e8400-e29b-41d4-a456-426655440000`
   - Used by: W3C Trace Context, OpenTelemetry (partial)

2. **AWS X-Ray:** `1-[a-f0-9]{8}-[a-f0-9]{24}`
   - Matches: `1-5e6722a7-cc2c2a3ac9b6bdf330b34215`
   - Used by: AWS X-Ray tracing

3. **Jaeger (hex string):** `[a-f0-9]{32}`
   - Matches: `4bf92f3577b34da6a3ce929d0e0e4736`
   - Used by: Jaeger distributed tracing

4. **W3C Trace Context (full):** `[a-f0-9]{32}-[a-f0-9]{16}-[a-f0-9]{2}`
   - Matches: `00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01`
   - Used by: W3C standard tracing

5. **Field name patterns (case-insensitive):**
   - `req(?:[_:]|ID)([a-zA-Z0-9\-]+)` — Matches: `req:abc123`, `req-abc-123`, `reqID:xyz`
   - `trace(?:[_]?id)?(?:[_:]|ID)([a-zA-Z0-9\-]+)` — Matches: `trace_id:xyz`, `traceid:xyz`, `traceID:xyz`
   - `(?:correlation|request|span|span-)?id(?:[_:])?([a-zA-Z0-9\-]+)` — Matches: `correlation_id:xyz`, `request_id:xyz`, `span_id:xyz`
   - `x-trace-id\s*:\s*([a-zA-Z0-9\-]+)` — Matches: `X-Trace-ID: 550e8400-e29b-41d4`
   - `x-request-id\s*:\s*([a-zA-Z0-9\-]+)` — Matches: `X-Request-ID: xyz`
   - `baggage.*trace[_]?id[=:]([a-zA-Z0-9\-]+)` — Matches OpenTelemetry baggage

##### Extraction Behavior:

- First pattern match found = correlation_id extracted
- If multiple patterns match same line: Use first match in regex order
- If no pattern matches: correlation_id = null (events correlated by timestamp if within 100ms window)

#### Pattern 3: Simple Plaintext (Fallback)

**Condition:** No structured format detected

##### Behavior:
- Timestamp: Current time
- Level: "info" for stdout, "error" for stderr
- Message: Entire line as-is
- Trace ID: Extract regex patterns from message (same as Pattern 2)

##### Example:
```
User session timeout after 30 minutes
  → timestamp=<now>, level=info, message="User session timeout after 30 minutes"
```

### Test Cases (v5.4)

#### Unit Tests (10 required):

1. **JSON: Valid with all fields**
   ```json
   {"timestamp": "2024-01-01T12:00:00Z", "level": "info", "message": "Test", "trace_id": "abc123"}
   ```
   Expected: ✓ All fields extracted correctly

2. **JSON: Missing optional trace_id**
   ```json
   {"timestamp": "2024-01-01T12:00:00Z", "level": "warn", "message": "Warning"}
   ```
   Expected: ✓ Parsed, trace_id empty

3. **JSON: Variant field names (msg, lvl, ts)**
   ```json
   {"ts": "2024-01-01T12:00:00Z", "lvl": "error", "msg": "Error occurred"}
   ```
   Expected: ✓ Fields mapped correctly

4. **Structured: Bracketed format with req ID**
   ```
   [2024-01-01T12:00:00Z] INFO [req:abc123] User login
   ```
   Expected: ✓ trace_id=abc123, message="User login"

5. **Structured: No trace ID bracket**
   ```
   [2024-01-01T12:00:00Z] WARN Database connection slow
   ```
   Expected: ✓ Parsed as structured, trace_id empty, message="Database connection slow"

6. **Structured: Complex trace ID format**
   ```
   [2024-01-01T12:00:00Z] ERROR [trace_id:xyz789-abc] Failed to process
   ```
   Expected: ✓ trace_id=xyz789, message="Failed to process"

7. **Plaintext: Simple message on stdout**
   ```
   Server started on port 3000
   ```
   Expected: ✓ level=info, message="Server started on port 3000"

8. **Plaintext: With trace ID pattern embedded**
   ```
   [req:def456] Processing request timeout
   ```
   Expected: ✓ level=info, trace_id=def456, message="[req:def456] Processing request timeout"

9. **Edge Case: Message contains brackets**
   ```
   [2024-01-01T12:00:00Z] INFO [req:abc123] Array access: data[0][1] returned null
   ```
   Expected: ✓ trace_id=abc123, message="Array access: data[0][1] returned null"

10. **Edge Case: Timestamp parsing - ISO 8601, RFC3339, Unix**
    ```
    [1704067200000] INFO Log with Unix timestamp
    [2024-01-01 12:00:00] INFO Log with space separator
    [2024-01-01T12:00:00+05:30] INFO Log with timezone offset
    ```
    Expected: ✓ All parsed correctly to milliseconds

### Performance Targets

- **Throughput:** Parse 10,000 log lines in < 100ms
- **Per-line latency:** < 10μs (microseconds)
- **Memory:** No allocation beyond immediate struct (< 1KB per line)
- **Regex cost:** Pre-compile patterns on startup, reuse

### Implementation Notes

#### DO:
- Pre-compile all regex patterns at startup
- Reuse regex objects (don't recompile per line)
- Store only extracted fields (discard original line)

#### DON'T:
- Use `strings.Split()` for bracket extraction (brittle)
- Allocate new regex per line
- Store full line in event (save ~1KB per line)

---

## Limitations (v5.4)

- [ ] No line filtering
- [ ] No sampling
- [ ] No deduplication
- [ ] Poll-based (not inotify)
- [ ] Local filesystem only (not remote filesystems like NFS)

---

## Future Improvements

- [ ] Use inotify on Linux (faster than polling)
- [ ] Support NFS/remote filesystems
- [ ] Line-level filtering (regex exclude)
- [ ] Sampling (1-in-N lines)
- [ ] Compression for old tailers

---

## Related Documents

- **Product Spec:** [PRODUCT_SPEC.md](PRODUCT_SPEC.md)
- **kaboom-run:** [TECH_SPEC_KABOOM_RUN.md](TECH_SPEC_KABOOM_RUN.md)
- **SSH Tailer:** [TECH_SPEC_SSH_TAILER.md](TECH_SPEC_SSH_TAILER.md)
- **Architecture:** [layer1-be-observability.md](../../core/target-architecture.md)

---

**Status:** Ready for implementation
**Estimated Effort:** 3 days
**Dependencies:** Context, sync, os, io packages (stdlib only)
