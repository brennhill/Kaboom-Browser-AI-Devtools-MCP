---
status: draft
priority: tier-1
phase: v5.4-foundation
relates-to: [PRODUCT_SPEC.md]
last-updated: 2026-01-31
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Backend Log Ingestion: kaboom-run Wrapper — Technical Specification

**Goal:** Wrapper executable that intercepts stdout/stderr and streams logs to local Kaboom daemon.

---

## Process Architecture

```
Developer
   │
   ├─ $ kaboom-run npm run dev
   │
   ▼
kaboom-run (new executable)
   ├─ spawn: npm run dev (child process)
   ├─ Capture: stdout/stderr
   ├─ Parse: Each line
   └─ Stream: HTTP POST http://localhost:7890/event
       │
       ▼
Kaboom Daemon (existing)
   ├─ Receive: POST /event
   ├─ Normalize: to NormalizedEvent
   └─ Store: in ring buffer

Terminal
   └─ Show: passthrough of stdout/stderr (user sees logs normally)
```

---

## Implementation

### File: `cmd/kaboom-run/main.go`

```go
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"
)

func main() {
	configPath := flag.String("config", "", "Path to Kaboom config")
	daemonHost := flag.String("host", "localhost:7890", "Kaboom daemon address")
	streamType := flag.String("type", "auto", "Stream type: auto, stdout, stderr")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: kaboom-run [flags] command [args...]\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Spawn child process
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin

	// Capture stdout/stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fatal("Failed to create stdout pipe: %v", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		fatal("Failed to create stderr pipe: %v", err)
	}

	// Start process
	if err := cmd.Start(); err != nil {
		fatal("Failed to start command: %v", err)
	}

	fmt.Fprintf(os.Stderr, "[kaboom-run] Started: %s\n", strings.Join(args, " "))
	fmt.Fprintf(os.Stderr, "[kaboom-run] Streaming to: %s\n", *daemonHost)

	// Stream logs
	client := &http.Client{Timeout: 5 * time.Second}

	// Concurrent streaming
	go streamLogs(stdoutPipe, "stdout", *daemonHost, client)
	go streamLogs(stderrPipe, "stderr", *daemonHost, client)

	// Also pass-through to terminal
	go io.Copy(os.Stdout, stdoutPipe)
	go io.Copy(os.Stderr, stderrPipe)

	// Wait for process to finish or signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	exitCode := 0
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-sigChan:
		// User pressed Ctrl+C
		cmd.Process.Kill()
		exitCode = 1
	case err := <-done:
		// Process exited
		if err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok {
				exitCode = exiterr.ExitCode()
			} else {
				exitCode = 1
			}
		}
	}

	os.Exit(exitCode)
}

func streamLogs(
	reader io.Reader,
	streamType string,
	daemonHost string,
	client *http.Client,
) {
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()

		// Send to daemon
		go sendLogLine(line, streamType, daemonHost, client)
	}
}

type LogLineEvent struct {
	Timestamp int64  `json:"timestamp"`
	Level     string `json:"level"`
	Source    string `json:"source"`
	Message   string `json:"message"`
	Metadata  map[string]interface{} `json:"metadata"`
	Tags      []string `json:"tags"`
}

func sendLogLine(
	line string,
	streamType string,
	daemonHost string,
	client *http.Client,
) {
	// Parse line to extract fields
	event := parseLogLine(line, streamType)
	if event == nil {
		return
	}

	// Send via HTTP POST
	body, _ := json.Marshal(event)
	req, _ := http.NewRequest(
		"POST",
		fmt.Sprintf("http://%s/event", daemonHost),
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		// Daemon not running, skip (don't fail the child process)
		return
	}
	defer resp.Body.Close()
}

func parseLogLine(line string, streamType string) *LogLineEvent {
	// Try JSON
	if event := tryParseJSON(line); event != nil {
		return event
	}

	// Try structured pattern
	if event := tryParseStructured(line); event != nil {
		return event
	}

	// Fallback: simple plaintext
	level := "info"
	if streamType == "stderr" {
		level = "error"
	}

	return &LogLineEvent{
		Timestamp: time.Now().UnixMilli(),
		Level:     level,
		Source:    "backend",
		Message:   line,
		Tags:      []string{"dev", "local"},
	}
}

func tryParseJSON(line string) *LogLineEvent {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		return nil
	}

	event := &LogLineEvent{
		Source:   "backend",
		Metadata: data,
		Tags:     []string{"dev", "local"},
	}

	// Extract timestamp
	if ts, ok := data["timestamp"].(string); ok {
		event.Timestamp = parseTimestamp(ts)
	} else {
		event.Timestamp = time.Now().UnixMilli()
	}

	// Extract level
	if level, ok := data["level"].(string); ok {
		event.Level = normalizeLevel(level)
	} else {
		event.Level = "info"
	}

	// Extract message
	if msg, ok := data["message"].(string); ok {
		event.Message = msg
	} else if msg, ok := data["msg"].(string); ok {
		event.Message = msg
	}

	return event
}

func tryParseStructured(line string) *LogLineEvent {
	// Pattern: [timestamp] LEVEL [req:xyz] message
	// Example: [2024-01-01T12:00:00Z] INFO [req:abc123] User logged in

	parts := strings.Split(line, "]")
	if len(parts) < 3 {
		return nil
	}

	timestamp := strings.Trim(parts[0], " [")
	level := strings.Trim(parts[1], " ")
	rest := strings.Join(parts[2:], "]")

	if !isValidLevel(level) {
		return nil // Not a structured log
	}

	event := &LogLineEvent{
		Timestamp: parseTimestamp(timestamp),
		Level:     normalizeLevel(level),
		Source:    "backend",
		Message:   strings.TrimLeft(rest, " []"),
		Tags:      []string{"dev", "local"},
	}

	// Try to extract correlation_id
	if correlationID := extractCorrelationID(rest); correlationID != "" {
		event.Metadata = map[string]interface{}{
			"correlation_id": correlationID,
		}
	}

	return event
}

func extractCorrelationID(s string) string {
	patterns := []string{
		`req[_:]?(\w+)`,
		`trace[_]?id[_:]?(\w+)`,
		`correlation[_]?id[_:]?(\w+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(s)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

func parseTimestamp(s string) int64 {
	// Try common formats
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t.UnixMilli()
		}
	}

	return time.Now().UnixMilli()
}

func normalizeLevel(s string) string {
	s = strings.ToLower(s)
	switch {
	case s == "trace", s == "debug":
		return "debug"
	case s == "info", s == "information":
		return "info"
	case s == "warn", s == "warning":
		return "warn"
	case s == "error", s == "err":
		return "error"
	case s == "fatal", s == "panic", s == "critical":
		return "critical"
	default:
		return "info"
	}
}

func isValidLevel(s string) bool {
	s = strings.ToLower(s)
	valid := map[string]bool{
		"trace": true, "debug": true, "info": true, "warn": true,
		"error": true, "fatal": true, "panic": true, "critical": true,
	}
	return valid[s]
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[kaboom-run] ERROR: "+format+"\n", args...)
	os.Exit(1)
}
```

---

## Build & Installation

```bash
# Build
go build -o kaboom-run ./cmd/kaboom-run

# Install (optional)
go install ./cmd/kaboom-run

# Usage
kaboom-run npm run dev
kaboom-run python -m flask run
kaboom-run go run main.go
```

---

## Key Design Decisions

### 1. Wrapper vs Agent
- **Chose:** Wrapper (exec wrapper)
- **Why:** No code changes required. Works with any command.
- **Alternative:** Background agent (needs process manager)

### 2. Pass-Through Logs
- **Chose:** Copy stdout/stderr to terminal AND to daemon
- **Why:** User still sees logs normally. Doesn't interfere with debugging.
- **Alternative:** Capture only (would silence terminal)

### 3. HTTP POST vs Direct Connection
- **Chose:** HTTP POST to daemon
- **Why:** Daemon handles ingest, doesn't need special handling for kaboom-run
- **Alternative:** Unix socket (platform-specific)

### 4. Durable Batch Pipeline (3 Goroutines + Disk Buffer)
- **Chose:** Ingestion → Memory Queue → Disk Buffer → Daemon Sender (async pipeline)
- **Why:** Durable (survives crash), resilient (works offline), non-blocking (ingestion unaffected)
- **Memory queue:** Bounded (10K events ≈ 10MB); disk acts as backpressure
- **Disk buffer:** Up to 1GB JSONL file (~100K lines)
- **Batch size:** 10K lines or 100ms timeout
- **Pass-through:** Real-time to terminal (no impact on user output)
- **Alternative:** In-memory only (loses lines on crash); per-line POST (resource leak)

### 5. Error Handling
- **Chose:** Silently skip if daemon not running
- **Why:** Shouldn't break dev workflow if Kaboom daemon crashes
- **Alternative:** Fail hard (would break dev command)

---

## Batch Collection Strategy

### Architecture: 3-Goroutine Pipeline

**Problem:** Spawning goroutine per log line → resource leak if daemon is slow. Need durable buffering.

**Solution:** Three concurrent goroutines with separation of concerns:

1. **Ingestion Goroutine** — Read lines, timestamp, write to memory queue (fast)
2. **Disk Buffer Goroutine** — Read from memory queue, write to local disk file (persistent)
3. **Daemon Sender Goroutine** — Batch read from disk, POST to daemon (network)

### Pseudocode

```
// Goroutine 1: Ingestion
StreamLogs(reader):
  const MAX_QUEUE_SIZE = 10000      // 10K events ≈ 10MB buffered in memory
  memoryQueue = make(chan LogEvent, MAX_QUEUE_SIZE)

  for line := scanner.ReadLine():
    event := LogLineEvent{
      Timestamp: time.Now().UnixMilli(),
      Message: line,
      ...parsed fields...
    }
    // If queue is full, block until Goroutine 2 drains it
    // This provides backpressure: disk accumulation signals ingestion to slow down
    memoryQueue <- event              // Blocks if queue full (backpressure)
    io.Copy(stdout, line)             // Real-time pass-through (after send succeeds)

// Goroutine 2: Disk Buffer
BufferToDisk(memoryQueue):
  diskFile = open(~/.kaboom/buffer.jsonl, O_APPEND)

  for event := range memoryQueue:
    diskFile.Write(event.JSON() + "\n")
    diskFile.Sync()                   // fsync on each write
    // Max buffer: 1GB file on disk

// Goroutine 3: Daemon Sender
SendToDaemon():
  ticker = Timer(100ms)

  loop:
    wait ticker or buffer_full
    batch = read_up_to_10K_lines_from_disk()
    if batch.len() > 0:
      POST /events with batch
      if success:
        truncate_disk_buffer()
      else:
        log_error("daemon offline, will retry")
        // Lines stay on disk for next attempt
```

### Guarantees

| Scenario | Behavior |
|----------|----------|
| Normal operation | Batch POSTs to daemon every 100ms or at 10K lines |
| Daemon slow/offline | Lines accumulate on disk (up to 1GB); sent when daemon recovers |
| Ingestion spike (100+ lines/sec) | Memory queue unbounded; disk absorbs excess; daemon delivery async |
| kaboom-run crashes | Lines persisted to disk; can be recovered on restart |
| Process terminating | Flush memory queue to disk, wait 2s for daemon to POST remainder |
| Network error | Log error, retry next batch (lines safe on disk) |

### Disk Buffer Management

#### File Rotation Strategy (Avoid Write Contention)

**Problem:** Single file with concurrent writes (Goroutine 2) and reads (Goroutine 3) → race conditions

**Solution:** Rolling buffer files with clean reader/writer separation

```
Directory: ~/.kaboom/buffers/

Active write file:   buffer.active.jsonl    (Goroutine 2 appends)
Pending read files:  buffer.001.jsonl       (Goroutine 3 reads & deletes)
                     buffer.002.jsonl
                     buffer.003.jsonl
```

##### Writer (Goroutine 2):
1. Write events to `buffer.active.jsonl`
2. Periodically check file size
3. When file reaches 100MB (or every 10 seconds):
   - Close active file
   - Rename to `buffer.NNN.jsonl` (next sequence number)
   - Open new `buffer.active.jsonl`
4. On shutdown: Rename active to pending sequence

##### Reader (Goroutine 3):
1. List all `buffer.NNN.jsonl` files (not `.active`)
2. Read from lowest sequence number first
3. Parse up to 10K lines
4. If POST succeeds: Delete the file
5. If POST fails: Keep file for next cycle (don't read `.active`)

#### Retention Policy

- **Max buffer size on disk:** 1GB total
- **Per-file size:** 100MB (triggers rotation)
- **TTL per event:** 24 hours (delete older events)
- **On startup:** Check if buffer files exist from previous run, send first

##### Cleanup:
1. If total buffer size > 1GB: Delete oldest sequence files until under 1GB
2. Periodic cleanup: Every 60 seconds, delete events older than 24 hours
3. On success POST: Immediately delete read file

#### Example Timeline

```
t=0s:   buffer.active.jsonl created (0 bytes)
t=2s:   buffer.active.jsonl (50MB, Goroutine 2 writing)
t=5s:   buffer.active.jsonl (100MB) → rename to buffer.001.jsonl
        buffer.active.jsonl created (0 bytes)
t=10s:  buffer.001.jsonl exists, buffer.active.jsonl (50MB)
        Goroutine 3 reads buffer.001.jsonl (10K lines = ~5MB batch)
t=11s:  POST /events succeeds → delete buffer.001.jsonl
t=20s:  buffer.active.jsonl (100MB) → rename to buffer.002.jsonl
```

#### Race Condition Prevention (Two-Phase Commit)

##### Critical Race Window:

When Goroutine 2 (writer) rotates `buffer.active.jsonl` → `buffer.NNN.jsonl`, Goroutine 3 (reader) might begin reading that file mid-rotation, causing:

- Duplicate lines (reader gets partial file, then re-reads after new data arrives)
- Missed lines (reader skips lines written after rename but before read)
- Data corruption (reader reads file while writer still has file descriptor open)

##### Solution: Two-Phase Commit with Stability Check

1. **Writer rotates file** (atomic on POSIX):

```go
// Old active file reaches 100MB
oldFile := "buffer.active.jsonl"
newFile := "buffer.001.jsonl"

os.Rename(oldFile, newFile)  // Atomic on POSIX
newActiveFile := open("buffer.active.jsonl", O_CREATE | O_APPEND)

// Track rotation time
rotationTimes[newFile] = time.Now()
```

1. **Reader only touches stable files** (age > 1 second):

```go
func findReadableFiles() []string {
  now := time.Now()
  var readable []string

  for filename, rotatedAt := range rotationTimes {
    age := now.Sub(rotatedAt)
    if age > 1*time.Second {  // File has stabilized
      readable = append(readable, filename)
    }
  }
  return readable
}
```

1. **Reader atomically consumes file**:

```go
// Read entire file into memory/batch
batch := readFileCompletely(filename)

// Only delete if read succeeded
if batch.len() > 0 {
  os.Remove(filename)  // Atomic delete
  delete(rotationTimes, filename)
}
```

###### Why This Works:

- Rename is atomic on POSIX: file either renamed or not, no half-state
- 1-second stability check: ensures rename is complete before reader touches
- File descriptor independence: writer has own fd, reader has own fd
- Atomic delete: reader either deletes file or doesn't, no corruption

###### Inode Tracking (Extra Protection):

```go
type RotatedFile struct {
  Path      string
  Inode     uint64      // Detect if file was recreated
  RotatedAt time.Time
}

// Before reading, verify inode hasn't changed
// (detects if filesystem reused inode)
if stat, _ := os.Stat(file.Path); getInode(stat) != file.Inode {
  log.Warn("File inode changed, skipping to avoid corruption")
  continue
}
```

###### Test Cases for Race Conditions:

1. Reader blocks on first file, writer creates 10 more files → verify reader processes in order
2. Writer rotates every 10ms, reader processes 100-line batches → verify no duplicates/skips
3. Reader partially through file, writer tries to rotate → verify reader completes then deletes
4. Reader deletes file while writer is rotating next → verify no double-open errors

### Memory Queue (Bounded with Backpressure)

- **Buffer Size:** 10,000 events ≈ 10MB in memory (bounded Go channel)
- **Why bounded?** Go channels are always finite. Unbounded channels don't exist in Go.
- **Backpressure Mechanism:**
  1. When queue fills (10K events), Goroutine 1 blocks on `memoryQueue <- event`
  2. This signals to the subprocess that logs are accumulating
  3. Goroutine 2 continues draining to disk (always makes progress)
  4. When daemon recovers, Goroutine 3 sends batches and queue drains
- **Disk acts as secondary limiter** — If daemon is offline, disk buffers up to 1GB
- **No drop policy** — All lines are preserved. Queue size is the limit, not memory.

#### Failure Mode Handling:

| Scenario | Behavior |
| --- | --- |
| Logging spike (1000 lines/sec for 5s) | Queue fills to 10K; ingestion blocks; subprocess slows down |
| Daemon offline for 1 minute | All lines written to disk (up to 1GB); subprocess resumes when daemon back up |
| Disk full | Goroutine 2 attempts write, gets error, logs warning. Ingestion continues (queue drains to disk eventually). |
| Process terminating | Flush memory queue to disk; wait 2s for Goroutine 3 to POST batch |

### Benefits

- ✅ **Durable:** Lines on disk survive process crash
- ✅ **Resilient:** Works when daemon is offline (buffers locally)
- ✅ **Non-blocking:** Ingestion unaffected by daemon/network delays
- ✅ **Async:** Three independent goroutines, no tight coupling
- ✅ **Observable:** Operator can inspect `~/.kaboom/buffer.jsonl`
- ✅ **Bounded:** Disk size limits buffering (1GB max)

---

## Integration with Daemon

### HTTP Endpoint: POST /events (Batch)

#### Request:
```
POST http://localhost:7890/events
Content-Type: application/json

{
  "events": [
    {
      "timestamp": 1704067200000,
      "level": "info",
      "source": "backend",
      "message": "User logged in",
      "metadata": {
        "correlation_id": "abc123"
      },
      "tags": ["dev", "local"]
    },
    {
      "timestamp": 1704067200015,
      "level": "info",
      "source": "backend",
      "message": "Session created",
      "metadata": {
        "correlation_id": "abc123",
        "client_skew_ms": 3
      },
      "tags": ["dev", "local"]
    }
  ]
}
```

#### Response:
```
200 OK
{
  "status": "ok",
  "count": 2,
  "event_ids": ["550e8400-e29b-41d4...", "550e8400-e29b-41d5..."]
}
```

### Key Fields

- **timestamp:** Source timestamp (from log), NOT daemon receive time
- **client_skew_ms:** Optional, extracted from BE logs (skew between FE and BE clocks)
- **batch:** Send all events collected in last 100ms in one request

### Daemon Responsibilities
1. **Receive** HTTP POST /events
2. **Normalize** each event to NormalizedEvent
3. **Extract** correlation_id from metadata
4. **Detect** client_skew_ms and record for skew calculation
5. **Route** to appropriate ring buffer (backend_logs)
6. **Store** for query (using source timestamp, not receive time)

Daemon does NOT need to know where event came from (kaboom-run, file tailer, extension, etc.).

---

## Performance

### Throughput
- **Per-line latency:** <1ms (HTTP overhead ~500μs)
- **Throughput:** 1000+ lines/sec to daemon
- **Typical dev server:** 10-100 lines/sec (no impact)

### Memory
- **Per-process:** ~5MB (binary size)
- **Per-invocation:** Minimal (streaming, no buffering)

### CPU
- **Idle:** 0% (waits for input)
- **Streaming:** <1% (single core)

---

## Error Handling

| Error | Behavior |
|-------|----------|
| Daemon not running | Silently skip POST (don't break subprocess) |
| Network timeout | Skip that line, continue streaming |
| Parse error | Fallback to simple plaintext |
| Process dies | kaboom-run exits with same code |
| SIGINT (Ctrl+C) | Propagate to child, exit cleanly |

---

## Testing Strategy

### Unit Tests
- [ ] Parse JSON logs correctly
- [ ] Parse structured logs with regex
- [ ] Fallback to plaintext
- [ ] Extract correlation_id
- [ ] Normalize log levels
- [ ] Parse timestamps in various formats

### Integration Tests
- [ ] `kaboom-run npm run dev` works
- [ ] Logs appear in daemon
- [ ] Pass-through still visible in terminal
- [ ] Works with piped output
- [ ] Works with colored output (ANSI codes)

### Manual Tests
```bash
# Test 1: Simple command
kaboom-run echo "Hello World"

# Test 2: Dev server
kaboom-run npm run dev

# Test 3: Long-running
kaboom-run python -m http.server 8000

# Test 4: Error output
kaboom-run npm run build  # (if build has errors)

# Test 5: Piped output
kaboom-run npm run dev | tee output.log
```

---

## Limitations (v5.4)

- [ ] No log filtering (captures all)
- [ ] No sampling (all lines parsed)
- [ ] No structured log parsing (regex patterns only)
- [ ] ANSI color codes passed through (may clutter logs)

---

## Related Documents

- **Product Spec:** [PRODUCT_SPEC.md](PRODUCT_SPEC.md)
- **Local Tailer:** [TECH_SPEC_LOCAL_TAILER.md](TECH_SPEC_LOCAL_TAILER.md)
- **SSH Tailer:** [TECH_SPEC_SSH_TAILER.md](TECH_SPEC_SSH_TAILER.md)
- **Architecture:** [layer1-be-observability.md](../../core/layer1-be-observability.md)

---

**Status:** Ready for implementation
**Estimated Effort:** 3 days
**Dependencies:** None (standalone executable)
