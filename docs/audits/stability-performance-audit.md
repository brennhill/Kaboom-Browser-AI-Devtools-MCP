---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Kaboom MCP Server -- Stability & Performance Audit

**Date:** 2026-02-14
**Auditor:** Senior Reliability Engineer (automated)
**Scope:** Go server (`cmd/browser-agent/`), capture package (`internal/capture/`), utility packages
**Version audited:** 0.7.12

---

## Table of Contents

1. [Concurrency & Deadlocks](#1-concurrency--deadlocks)
2. [Resource Leaks](#2-resource-leaks)
3. [Blocking Operations](#3-blocking-operations)
4. [Error Handling & Recovery](#4-error-handling--recovery)
5. [Memory & Performance](#5-memory--performance)
6. [Shutdown & Cleanup](#6-shutdown--cleanup)
7. [External Process Execution](#7-external-process-execution)
8. [Severity Summary](#severity-summary)
9. [Top 5 Priority Fixes](#top-5-priority-fixes)
10. [Ship / No-Ship Recommendation](#ship--no-ship-recommendation)

---

## 1. Concurrency & Deadlocks

### FINDING 1.1 -- Connect mode stdout writes lack serialization

- **Location:** `cmd/browser-agent/connect_mode.go:130` and `cmd/browser-agent/connect_mode.go:174`
- **Category:** Concurrency
- **Severity:** HIGH

**Description:**
In connect mode, `connectForwardRequest` writes JSON-RPC responses directly to stdout via `fmt.Println` without acquiring the `mcpStdoutMu` mutex. Similarly, `sendMCPError` at line 174 also writes to stdout without the lock.

```go
// connect_mode.go:130
fmt.Println(string(respData))
```

```go
// connect_mode.go:173-174
respJSON, _ := json.Marshal(resp)
fmt.Println(string(respJSON))
```

In contrast, bridge mode always acquires `mcpStdoutMu` before writing:

```go
// bridge.go:539-542
mcpStdoutMu.Lock()
fmt.Print(string(body))
flushStdout()
mcpStdoutMu.Unlock()
```

**Impact:**
Currently mitigated because `connectForwardLoop` processes requests sequentially (single goroutine reads stdin and forwards one at a time). However, if connect mode is ever made concurrent (like bridge mode), interleaved stdout writes would corrupt the JSON-RPC stream, causing the MCP client to receive malformed responses. This is a latent bug that becomes critical under any concurrency change.

**Fix recommendation:**
Wrap all stdout writes in connect mode with `mcpStdoutMu.Lock()`/`mcpStdoutMu.Unlock()`, or extract a shared `writeToStdout()` function that both modes use.

---

### FINDING 1.2 -- QueryDispatcher.Close() is not concurrency-safe

- **Location:** `internal/capture/query_dispatcher.go:67-72`
- **Category:** Concurrency
- **Severity:** LOW

**Description:**
`QueryDispatcher.Close()` reads and writes `stopCleanup` without holding any lock:

```go
func (qd *QueryDispatcher) Close() {
    if qd.stopCleanup != nil {
        qd.stopCleanup()
        qd.stopCleanup = nil
    }
}
```

The comment says "Safe to call multiple times" but concurrent calls could race: both goroutines could pass the nil check, leading to a double-close of the stop channel.

**Impact:**
Low practical risk -- `Close()` is called only during shutdown from `awaitShutdownSignal`. Double-close of a channel would panic, but the shutdown path is single-threaded in practice.

**Fix recommendation:**
Use `sync.Once` to make Close truly safe for concurrent callers.

---

### FINDING 1.3 -- Lock hierarchy is well-documented and consistently followed

- **Location:** `internal/capture/capture-struct.go:18-19`, `internal/capture/query_dispatcher.go:34`
- **Category:** Concurrency
- **Severity:** N/A (Positive finding)

**Description:**
The codebase has a clearly documented lock hierarchy:
1. `ClientRegistry.mu` (position 1, outermost)
2. `ClientState` (position 2)
3. `Capture.mu` (position 3)
4. `QueryDispatcher.mu` -> `QueryDispatcher.resultsMu` (always in this order)
5. `CircuitBreaker.mu` (independent)
6. `DebugLogger.mu` (independent)
7. `RecordingManager.mu` (independent)

All code paths verified during this audit follow this hierarchy. Methods like `cleanExpiredResults`, `ExpireAllPendingQueries`, and `SetQueryResultWithClient` release `mu` before acquiring `resultsMu`, as documented.

**Impact:** No deadlock risk from lock ordering violations detected.

---

## 2. Resource Leaks

### FINDING 2.1 -- globalAnnotationStore starts background goroutine at package init

- **Location:** `cmd/browser-agent/annotation_store.go:14`
- **Category:** Resource Leak
- **Severity:** MEDIUM

**Description:**
`globalAnnotationStore` is initialized at package load time as a module-level variable:

```go
var globalAnnotationStore = NewAnnotationStore(10 * time.Minute)
```

`NewAnnotationStore` spawns a background cleanup goroutine:

```go
func NewAnnotationStore(detailTTL time.Duration) *AnnotationStore {
    s := &AnnotationStore{...}
    util.SafeGo(func() { s.cleanupLoop() })
    return s
}
```

This goroutine starts even for modes that do not need it (e.g., `--version`, `--help`, `--doctor`, `--stop`, `--force`). It also starts for bridge mode, which never uses the annotation store.

**Impact:**
Minor resource waste: one goroutine + ticker running in all execution modes. The goroutine is properly cleaned up in daemon mode via `globalAnnotationStore.Close()` in the shutdown path. In other modes, the process exits quickly enough that the leaked goroutine is inconsequential.

**Fix recommendation:**
Use lazy initialization with `sync.Once` -- only create the store when first accessed.

---

### FINDING 2.2 -- WaitForResultWithClient spawns a 10ms ticker goroutine per blocking call

- **Location:** `internal/capture/queries.go:311-322`
- **Category:** Resource Leak
- **Severity:** LOW

**Description:**
Each call to `WaitForResultWithClient` spawns a wakeup goroutine with a 10ms ticker:

```go
done := make(chan struct{})
util.SafeGo(func() {
    ticker := time.NewTicker(10 * time.Millisecond)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            qd.queryCond.Broadcast()
        case <-done:
            return
        }
    }
})
```

The goroutine is properly cleaned up via `defer close(done)` on function return (LIFO ordering ensures `close(done)` executes before `mu.Unlock()`). The comment notes this replaced a per-iteration goroutine that caused approximately 3000 goroutines per 30s call.

**Impact:**
Current design is correct -- one goroutine per active blocking wait, cleaned up on completion. The 10ms polling cadence adds slight overhead (100 broadcasts/second per active wait) but is bounded by the number of concurrent tool calls.

**Fix recommendation:**
No action needed. The current pattern is the result of a previous optimization.

---

### FINDING 2.3 -- HTTP response body not drained in stopViaHTTP error path

- **Location:** `cmd/browser-agent/main_connection_stop.go:119-131`
- **Category:** Resource Leak
- **Severity:** LOW

**Description:**
In `stopViaHTTP`, the HTTP request is created without a context:

```go
req, _ := http.NewRequest("POST", shutdownURL, nil)
resp, err := client.Do(req)
```

When `err == nil` but `resp.StatusCode != 200`, the body is closed but not fully drained:

```go
if resp != nil {
    _ = resp.Body.Close()
}
```

Not draining the body before close prevents connection reuse. Additionally, the `http.NewRequest` error is silently discarded.

**Impact:**
Negligible -- this is a one-shot operation during server stop, not a hot path. The 3-second client timeout prevents hanging.

**Fix recommendation:**
Add `io.Copy(io.Discard, resp.Body)` before closing, and check the `http.NewRequest` error.

---

## 3. Blocking Operations

### FINDING 3.1 -- HTTP WriteTimeout conflicts with slow MCP tools

- **Location:** `cmd/browser-agent/main_connection_mcp.go:171-174`
- **Category:** Blocking Operations
- **Severity:** CRITICAL

**Description:**
The HTTP server is configured with a 10-second `WriteTimeout`:

```go
srv := &http.Server{
    ReadTimeout:  5 * time.Second,
    WriteTimeout: 10 * time.Second,
    IdleTimeout:  120 * time.Second,
    Handler:      AuthMiddleware(apiKey)(mux),
}
```

However, several MCP tool operations take significantly longer:
- `analyze` and `interact` tools: up to 35s (bridge timeout via `toolCallTimeout`)
- Annotation `observe command_result` polling: up to 65s (blocking poll)
- `WaitForCommand`: configurable, up to 55s for annotation observation
- `WaitForSession`: configurable timeout for draw mode results

When a tool handler blocks beyond 10 seconds, the `net/http` server kills the connection. The bridge receives a truncated/empty response and sends a `-32603` error to the MCP client.

```go
// bridge.go:53-54
case "analyze", "interact":
    return slow // 35 * time.Second
```

**Impact:**
Any `analyze` or `interact` tool call that takes more than 10 seconds will be silently killed by the HTTP server. The bridge will receive a connection reset or empty response and report a generic "Server connection error" to the LLM. This is the most impactful bug in the codebase -- it means the two most complex tools are unreliable for any non-trivial operation. Annotation polling (65s) is similarly affected.

**Fix recommendation:**
Set `WriteTimeout: 0` (disable) or set it to at least 70 seconds (above the maximum tool timeout). Since this is a localhost-only server, the security benefit of a short write timeout is minimal compared to the operational impact. Alternatively, use `http.TimeoutHandler` per-route for fine-grained control.

---

### FINDING 3.2 -- Preflight port check has TOCTOU race with daemon bind

- **Location:** `cmd/browser-agent/main_connection_mcp.go:155-163`
- **Category:** Blocking Operations
- **Severity:** LOW

**Description:**
`preflightPortCheck` opens and immediately closes a TCP listener to test port availability:

```go
func preflightPortCheck(server *Server, port int) error {
    testAddr := fmt.Sprintf("127.0.0.1:%d", port)
    testLn, err := net.Listen("tcp", testAddr)
    if err != nil {
        // ...
        return fmt.Errorf(...)
    }
    return testLn.Close()
}
```

Between `testLn.Close()` and `startHTTPServer` calling `net.Listen`, another process could bind the port. The check also runs before `cleanupStalePIDFile`, creating a window where both checks pass but the real bind fails.

**Impact:**
Low -- the real `net.Listen` in `startHTTPServer` will catch any actual conflict, and the error is properly propagated. The preflight check provides a better error message, which is its sole purpose.

**Fix recommendation:**
No action needed. The defense-in-depth approach is acceptable.

---

## 4. Error Handling & Recovery

### FINDING 4.1 -- Bare goroutine in production code (upload_form_submit.go)

- **Location:** `cmd/browser-agent/upload_form_submit.go:175`
- **Category:** Error Handling
- **Severity:** MEDIUM

**Description:**
The `executeFormSubmit` function uses a bare `go func()` instead of `util.SafeGo`:

```go
go func() { // lint:allow-bare-goroutine -- short-lived pipe writer, error captured via channel
    writeErrCh <- streamMultipartForm(pw, writer, req, file)
}()
```

The comment acknowledges this deviation from the codebase convention. If `streamMultipartForm` panics, the goroutine crashes without recovery. The `util.SafeGo` wrapper would catch the panic, log it to stderr, and prevent a full process crash.

**Impact:**
If a panic occurs during multipart form writing (e.g., nil pointer in MIME header construction), the goroutine dies silently. The pipe reader would get EOF, and `uploadHTTPClient.Do` would complete with a partial request. The error would eventually surface as a confusing HTTP error from the remote server rather than a clear panic trace. Given that `streamMultipartForm` handles external data (user-provided file paths, field names, CSRF tokens), defensive recovery is warranted.

**Fix recommendation:**
Replace `go func()` with `util.SafeGo(func() { ... })`. The error channel pattern still works identically since `SafeGo` recovers panics to stderr without re-panicking.

---

### FINDING 4.2 -- sendStartupError writes to stdout without mcpStdoutMu

- **Location:** `cmd/browser-agent/main.go:573-589`
- **Category:** Error Handling
- **Severity:** LOW

**Description:**
`sendStartupError` writes a JSON-RPC error to stdout without the `mcpStdoutMu` lock:

```go
func sendStartupError(message string) {
    // ...
    fmt.Println(string(respJSON))
    if err := os.Stdout.Sync(); err != nil {
        // ...
    }
}
```

**Impact:**
No practical risk -- this function is called during startup before any concurrent goroutines are active.

**Fix recommendation:**
Acquire the lock for consistency, even though the race window is non-existent.

---

### FINDING 4.3 -- Server.addEntries file I/O can race with clearEntries

- **Location:** `cmd/browser-agent/server.go:175-231`
- **Category:** Error Handling
- **Severity:** LOW

**Description:**
`addEntries` takes a snapshot of entries under the lock, then performs file I/O outside the lock:

```go
s.mu.Unlock()
// File I/O outside lock -- snapshot protects consistency
if rotated {
    if err := s.saveEntriesCopy(entriesToSave); err != nil {
        // ...
    }
}
```

The code contains an explicit comment acknowledging this:

```go
// Note: If clearEntries() is called between unlock and file I/O, the file
// may temporarily contain stale entries that were cleared from memory.
```

**Impact:**
Acknowledged and documented. The window is microseconds, and the next rotation rewrites the entire file. Memory state is always consistent.

**Fix recommendation:**
No action needed. The tradeoff is intentional and well-documented.

---

## 5. Memory & Performance

### FINDING 5.1 -- screenshotRateLimiter map can grow unbounded between cleanup cycles

- **Location:** `cmd/browser-agent/main.go:67`, `cmd/browser-agent/server_routes.go:56-79`
- **Category:** Memory
- **Severity:** LOW

**Description:**
The `screenshotRateLimiter` map tracks per-client upload timestamps:

```go
var screenshotRateLimiter = make(map[string]time.Time)
```

Cleanup runs every 30 seconds (removing entries older than 1 minute). Between cleanup cycles, the map can grow. However, `checkScreenshotRateLimit` has inline eviction at 10,000 entries:

```go
if len(screenshotRateLimiter) >= 10000 && !exists {
    for id, ts := range screenshotRateLimiter {
        if time.Since(ts) > screenshotMinInterval {
            delete(screenshotRateLimiter, id)
        }
    }
    if len(screenshotRateLimiter) >= 10000 {
        return http.StatusServiceUnavailable, "Rate limiter capacity exceeded"
    }
}
```

**Impact:**
Well-mitigated. The 10,000-entry cap with inline eviction prevents unbounded growth. Each entry is approximately 80 bytes (string key + time.Time), so worst case is approximately 800KB.

**Fix recommendation:**
No action needed. The dual cleanup strategy (periodic + inline eviction) is robust.

---

### FINDING 5.2 -- StreamState.SeenMessages map grows during dedup window

- **Location:** `cmd/browser-agent/streaming.go:47`, `cmd/browser-agent/streaming.go:284-296`
- **Category:** Memory
- **Severity:** LOW

**Description:**
The `SeenMessages` map tracks recently-sent notification keys for deduplication within a 30-second window:

```go
type StreamState struct {
    SeenMessages map[string]time.Time // dedupKey -> last sent
    // ...
}
```

Pruning happens inline during `recordDedupKey`:

```go
for k, t := range s.SeenMessages {
    if now.Sub(t) > dedupWindow {
        delete(s.SeenMessages, k)
    }
}
```

**Impact:**
Bounded by the maximum notification rate (12 per minute) and dedup window (30s). Maximum size is approximately 6 entries. No concern.

**Fix recommendation:**
No action needed.

---

### FINDING 5.3 -- completedResults map cleaned every 30 seconds with 60s TTL

- **Location:** `internal/capture/queries.go:374-415`
- **Category:** Memory
- **Severity:** LOW

**Description:**
The `completedResults` map stores async command tracking entries with a 60-second TTL. Cleanup runs every 30 seconds via `startResultCleanup`. Between cleanup cycles, expired entries persist for up to 30 additional seconds.

Additionally, `cleanExpiredCommands` is called eagerly by `GetCommandResult`, `GetPendingCommands`, `GetCompletedCommands`, and `GetFailedCommands` -- ensuring stale entries are cleaned on access even between timer ticks.

**Impact:**
Minimal. Pending command count is bounded by `maxPendingQueries = 5`, and each entry is lightweight.

**Fix recommendation:**
No action needed. The eager-cleanup-on-access pattern keeps the map tight.

---

## 6. Shutdown & Cleanup

### FINDING 6.1 -- Capture.Close() does not stop all background goroutines

- **Location:** `internal/capture/capture-struct.go:184-188`
- **Category:** Shutdown
- **Severity:** MEDIUM

**Description:**
`Capture.Close()` only stops the QueryDispatcher's cleanup goroutine:

```go
func (c *Capture) Close() {
    if c.qd != nil {
        c.qd.Close()
    }
}
```

However, `NewCapture()` does not start any goroutines directly -- `QueryDispatcher` starts one cleanup goroutine (stopped by `Close`), and `CircuitBreaker` spawns short-lived goroutines via `SafeGo` for lifecycle events (these complete quickly).

The issue is that `Capture.Close()` is never called in the shutdown path. Looking at `awaitShutdownSignal`:

```go
// main_connection_mcp.go:234-236
server.shutdownAsyncLogger(2 * time.Second)
globalAnnotationStore.Close()
removePIDFile(port)
```

The `Capture` instance's `Close()` is not called. The QueryDispatcher's cleanup goroutine continues running after `srv.Shutdown()` until the process exits.

**Impact:**
The process is exiting anyway (this is the shutdown path), so the orphaned goroutine is killed by the OS. However, if the cleanup goroutine is mid-write when the process exits, it could leave partial data.

**Fix recommendation:**
Add `cap.Close()` to the shutdown path in `awaitShutdownSignal`, before `server.shutdownAsyncLogger`.

---

### FINDING 6.2 -- Async logger shutdown has 2-second timeout that may drop logs

- **Location:** `cmd/browser-agent/server.go:339-355`
- **Category:** Shutdown
- **Severity:** LOW

**Description:**
The async logger drains its 10,000-capacity buffered channel on shutdown with a 2-second timeout:

```go
func (s *Server) shutdownAsyncLogger(timeout time.Duration) {
    close(s.logChan)
    select {
    case <-s.logDone:
        // Worker exited cleanly
    case <-time.After(timeout):
        fmt.Fprintf(os.Stderr, "[kaboom] Async logger shutdown timeout, %d logs may be lost\n", len(s.logChan))
    }
}
```

If the channel has many pending entries and the filesystem is slow, logs can be lost. The function correctly reports the count of potentially lost logs.

**Impact:**
Low -- the log channel typically has very few entries during normal operation. The 10,000 buffer only fills during extreme burst traffic when entries are being dropped anyway (as reported by `logDropCount`).

**Fix recommendation:**
No action needed. The current behavior is appropriate for a graceful-best-effort shutdown.

---

### FINDING 6.3 -- Bridge mode does not stop the daemon on exit

- **Location:** `cmd/browser-agent/bridge.go:225-295`
- **Category:** Shutdown
- **Severity:** N/A (Design decision)

**Description:**
When the bridge process exits (stdin EOF), the daemon continues running. This is by design -- the daemon persists between MCP sessions, and the comment on `runMCPMode` says:

```go
// If stdin closes (EOF), the HTTP server keeps running until killed.
```

The daemon is stopped via `--stop`, signal, or the `/shutdown` HTTP endpoint.

**Impact:**
Intentional design. Users who expect the daemon to die with the bridge may be surprised, but this is documented behavior.

---

## 7. External Process Execution

### FINDING 7.1 -- findProcessOnPort and getProcessCommand lack context timeout

- **Location:** `cmd/browser-agent/platform_errors.go:71-88` and `cmd/browser-agent/platform_errors.go:92-115`
- **Category:** External Process
- **Severity:** HIGH

**Description:**
Both functions use `exec.Command` without a context timeout:

```go
// platform_errors.go:74-76
func findProcessOnPort(port int) ([]int, error) {
    var cmd *exec.Cmd
    if runtime.GOOS == "windows" {
        cmd = exec.Command("netstat", "-ano")
    } else {
        cmd = exec.Command("lsof", "-ti", fmt.Sprintf(":%d", port))
    }
    output, err := cmd.Output()
    // ...
}
```

```go
// platform_errors.go:92-100
func getProcessCommand(pid int) string {
    var cmd *exec.Cmd
    if runtime.GOOS == "windows" {
        cmd = exec.Command("tasklist", "/FI", ...)
    } else {
        cmd = exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=")
    }
    output, err := cmd.Output()
    // ...
}
```

If `lsof`, `netstat`, `tasklist`, or `ps` hangs (e.g., waiting on NFS, heavy system load), the calling goroutine blocks indefinitely.

**Impact:**
These functions are called from `stopViaProcessLookup` (the `--stop` command) and from `preflightPortCheck` error paths. A hung `lsof` call during `--stop` would make the CLI appear frozen. On macOS, `lsof` is known to hang when scanning certain file descriptors. On Windows, `netstat` can be slow with many connections.

**Fix recommendation:**
Use `exec.CommandContext` with a 5-second timeout, consistent with the pattern used elsewhere in the codebase (e.g., `detectBrowserPIDDarwin` at `upload_os_automation.go:122-123`).

---

### FINDING 7.2 -- killUnixKaboomProcesses lsof and pkill lack context timeout

- **Location:** `cmd/browser-agent/main_connection_stop.go:189-212`
- **Category:** External Process
- **Severity:** HIGH

**Description:**
The force cleanup path uses `exec.Command` without timeouts for both `lsof` and `pkill`:

```go
// main_connection_stop.go:189
cmd := exec.Command("lsof", "-c", "kaboom")
output, err := cmd.Output()
```

```go
// main_connection_stop.go:209
pkillCmd := exec.Command("pkill", "-f", "kaboom.*--daemon")
_ = pkillCmd.Run()
```

Neither command has a context timeout. The `pkill` result is silently discarded.

**Impact:**
`runForceCleanup` is invoked during npm package install (`--force` flag). If `lsof` hangs, the npm install appears frozen. This is particularly problematic because users cannot easily Ctrl+C out of an npm postinstall script on some platforms.

**Fix recommendation:**
Use `exec.CommandContext` with a 10-second timeout for `lsof` and a 5-second timeout for `pkill`.

---

### FINDING 7.3 -- OS automation commands use proper timeouts

- **Location:** `cmd/browser-agent/upload_os_automation.go:122-137`, `cmd/browser-agent/upload_os_automation.go:407-409`
- **Category:** External Process
- **Severity:** N/A (Positive finding)

**Description:**
All `exec.CommandContext` calls in the OS automation path use proper timeouts:
- Browser PID detection: 5-second timeout
- AppleScript execution: 15-second timeout
- PowerShell execution: 15-second timeout
- xdotool execution: 15-second timeout (shared context across 4 sequential commands)
- File dialog dismiss: 5-second timeout

Input sanitization is also thorough:
- `validatePathForOSAutomation` rejects null bytes, newlines, and backticks
- `sanitizeForAppleScript` escapes backslashes and double quotes
- `sanitizeForSendKeys` escapes PowerShell special characters
- xdotool uses `--` argument terminator to prevent flag injection

**Impact:** No issues found. This is a well-secured external process execution path.

---

### FINDING 7.4 -- stopViaHTTP creates request without context

- **Location:** `cmd/browser-agent/main_connection_stop.go:119`
- **Category:** External Process
- **Severity:** LOW

**Description:**
```go
req, _ := http.NewRequest("POST", shutdownURL, nil)
resp, err := client.Do(req)
```

The request is created with `http.NewRequest` (no context) and the error is discarded. The 3-second client timeout on the `http.Client` provides an upper bound, but the request cannot be cancelled.

**Impact:**
Minimal -- the 3-second timeout is sufficient for a localhost shutdown request.

**Fix recommendation:**
Use `http.NewRequestWithContext` with a 3-second context for consistency.

---

## Severity Summary

| Severity | Count |
|----------|-------|
| CRITICAL | 1     |
| HIGH     | 3     |
| MEDIUM   | 2     |
| LOW      | 9     |

**CRITICAL (1):**
- 3.1: HTTP WriteTimeout (10s) kills slow tool responses (analyze/interact/annotation polling)

**HIGH (3):**
- 1.1: Connect mode stdout writes lack `mcpStdoutMu` serialization
- 7.1: `findProcessOnPort` and `getProcessCommand` lack context timeout
- 7.2: `killUnixKaboomProcesses` lsof/pkill lack context timeout

**MEDIUM (2):**
- 2.1: `globalAnnotationStore` starts background goroutine at package init
- 4.1: Bare `go func()` in `upload_form_submit.go` bypasses SafeGo panic recovery

**LOW (9):**
- 1.2: `QueryDispatcher.Close()` not concurrency-safe (theoretical double-close)
- 2.2: WaitForResultWithClient 10ms ticker goroutine (properly cleaned up)
- 2.3: HTTP response body not drained in stopViaHTTP error path
- 3.2: Preflight port check TOCTOU race (defense-in-depth, handled by real bind)
- 4.2: `sendStartupError` writes without lock (no concurrent writers at startup)
- 4.3: `addEntries` file I/O race with `clearEntries` (documented, accepted)
- 5.1: `screenshotRateLimiter` bounded by 10k inline eviction
- 5.2: `StreamState.SeenMessages` bounded by notification rate limit
- 5.3: `completedResults` cleaned eagerly on access

---

## Top 5 Priority Fixes

### 1. Increase HTTP WriteTimeout to accommodate slow tools
**File:** `cmd/browser-agent/main_connection_mcp.go:173`
**Change:** `WriteTimeout: 10 * time.Second` -> `WriteTimeout: 70 * time.Second` (or `0` to disable)
**Why:** This is the single most impactful bug. The `analyze` and `interact` tools -- the two most complex tools in the system -- silently fail for any operation exceeding 10 seconds. Annotation polling (up to 65s) is completely broken. Users experience this as random, unreproducible failures with no clear error message.

### 2. Add context timeouts to exec.Command in platform_errors.go
**File:** `cmd/browser-agent/platform_errors.go:72-100`
**Change:** Replace `exec.Command(...)` with `exec.CommandContext(ctx, ...)` using a 5-second timeout
**Why:** A hung `lsof` or `ps` command during `--stop` makes the CLI appear frozen. This is user-facing and erodes trust in the tool.

### 3. Add context timeouts to exec.Command in killUnixKaboomProcesses
**File:** `cmd/browser-agent/main_connection_stop.go:189-210`
**Change:** Replace `exec.Command(...)` with `exec.CommandContext(ctx, ...)` using 10-second and 5-second timeouts
**Why:** This runs during npm package install. A hung command blocks the entire installation pipeline.

### 4. Add mcpStdoutMu to connect mode stdout writes
**File:** `cmd/browser-agent/connect_mode.go:130` and `cmd/browser-agent/connect_mode.go:173-174`
**Change:** Wrap stdout writes with `mcpStdoutMu.Lock()`/`mcpStdoutMu.Unlock()`
**Why:** Prevents a latent bug from becoming critical if connect mode is ever made concurrent.

### 5. Replace bare goroutine with SafeGo in upload_form_submit.go
**File:** `cmd/browser-agent/upload_form_submit.go:175`
**Change:** Replace `go func()` with `util.SafeGo(func() { ... })`
**Why:** Maintains consistency with the codebase-wide SafeGo convention and ensures panic recovery on a path that processes external data.

---

## Ship / No-Ship Recommendation

**Recommendation: CONDITIONAL SHIP**

The codebase demonstrates strong engineering practices overall:
- Well-documented lock hierarchy with consistent adherence
- SafeGo pattern for goroutine panic recovery (with one exception)
- Bounded data structures with ring buffers and LRU eviction
- Proper SSRF protection and input sanitization
- Graceful shutdown with signal handling and PID management
- Defense-in-depth error handling throughout

However, **Finding 3.1 (HTTP WriteTimeout)** is a **CRITICAL** production blocker. The two most important tools (`analyze` and `interact`) silently fail for any non-trivial operation. This must be fixed before any release that advertises these tools as reliable.

**Ship criteria:**
1. **Must fix:** Finding 3.1 (WriteTimeout). One-line change with immediate impact.
2. **Should fix before next release:** Findings 7.1 and 7.2 (exec.Command timeouts). Prevents CLI hangs.
3. **Nice to have:** Findings 1.1 and 4.1 (stdout serialization and SafeGo). Defensive improvements.

All other findings are LOW severity with appropriate mitigations already in place.
