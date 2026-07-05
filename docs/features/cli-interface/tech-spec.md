---
doc_type: tech-spec
feature_id: cli-interface
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# CLI Interface — Tech Spec

**Feature:** `kaboom-cmd` binary for direct CLI access to all MCP tools, enabling script-based automation, CI/CD integration, and future web client support.

## Use Cases:
- Command-line automation (e.g., `kaboom-cmd interact upload --selector "#File" --file /path`)
- CI/CD pipelines (bulk uploads, form submissions, automation)
- Headless systems (no browser extension needed)
- Future web client wrapper (HTTP → `kaboom-cmd` → MCP server)
- Direct command invocation without MCP overhead

---

## Architecture

### System Design

```
CLI User / Script / CI Pipeline
  ↓
kaboom-cmd (separate binary)
  ├─ Connects to kaboom-mcp (auto-start or existing)
  ├─ Translates CLI args → MCP JSON-RPC
  ├─ Handles async (streaming vs sync)
  └─ Returns JSON/human/CSV output

kaboom-mcp (existing MCP server)
  ├─ 5 tools: observe, generate, configure, interact, analyze
  ├─ Unchanged from MCP perspective
  └─ Serves both extension + CLI clients
```

### Why Separate Binary?

- **kaboom-mcp**: MCP server, talks JSON-RPC via stdio
- **kaboom-cmd**: CLI client, translates flags → MCP calls, manages server lifecycle
- **Future**: Web client can also connect to same `kaboom-mcp`
- **Cleaner**: Each has single responsibility

---

## Command Structure

### Format

```bash
kaboom-cmd <tool> <action> [options] [--flags]
```

### Examples

#### Single file upload:
```bash
kaboom-cmd interact upload \
  --selector "#Filedata" \
  --file-path "./video.mp4" \
  --format human
```

#### Output (human-readable by default):
```
✅ Upload successful
   Stage: 1 (Drag-drop)
   File: video.mp4 (1.0 GB)
   Duration: 2.3s
```

#### Bulk CSV upload:
```bash
kaboom-cmd interact upload \
  --csv-file videos.csv \
  --selector "#Filedata" \
  --format csv \
  --stream
```

#### Output (CSV, streaming progress):
```
file_path,status,stage,duration_ms,error
/videos/video1.mp4,success,3,5600,
/videos/video2.mp4,success,1,2300,
/videos/video3.mp4,error,3,8000,"CSRF token expired"
```

#### Streaming progress (newline-delimited JSON):
```bash
kaboom-cmd interact upload \
  --selector "#Filedata" \
  --file-path large-4gb-video.mp4 \
  --stream
```

#### Output (one JSON object per line):
```json
{"status":"reading_file","percent":0,"bytes_sent":0,"total_bytes":4294967296}
{"status":"uploading","percent":5,"bytes_sent":214748365,"total_bytes":4294967296,"eta_seconds":3600}
{"status":"uploading","percent":10,"bytes_sent":429496730,"total_bytes":4294967296,"eta_seconds":3240}
...
{"status":"complete","success":true,"stage":3,"duration_ms":3245000}
```

#### Fill form field:
```bash
kaboom-cmd interact fill \
  --selector "#title" \
  --text "My Video Title"
```

#### Click button:
```bash
kaboom-cmd interact click --selector "button[type=submit]"
```

#### Get page text:
```bash
kaboom-cmd interact get-text --selector ".upload-status"
```

---

## Configuration Loading

### Priority Order (Lowest to Highest)

```
Defaults
  ↓ (override with)
.kaboom.json (project directory)
  ↓ (override with)
$KABOOM_PORT, $KABOOM_FORMAT, $KABOOM_SERVER
  ↓ (override with)
--flags (command-line arguments)
```

### Configuration Files

#### Global (~/.kaboom/config.json)

```json
{
  "server_port": 9223,
  "server_timeout_ms": 5000,
  "format": "human",
  "auto_start_server": true
}
```

#### Project (.kaboom.json, checked into repo)

```json
{
  "server_port": 9223,
  "format": "json",
  "escalation_timeout_ms": 10000,
  "wait_for_completion": true
}
```

#### Environment Variables

```bash
export KABOOM_PORT=9224              # Override server port
export KABOOM_FORMAT=json            # Override output format
export KABOOM_TIMEOUT=30000          # Override global timeout
export KABOOM_NO_AUTO_START=1        # Don't auto-start server
```

#### Command-Line Flags (Highest Priority)

```bash
kaboom-cmd interact upload \
  --server-port 9224 \
  --format csv \
  --timeout 30000 \
  --no-stream \
  --selector "#File" \
  --file-path ./video.mp4
```

---

## Output Formats

### Human-Readable (Default)

```bash
$ kaboom-cmd interact upload \
    --selector "#Filedata" \
    --file-path video.mp4

✅ Upload successful
   Stage: 3 (Form interception)
   File: video.mp4
   Size: 1.0 GB
   Duration: 5.6s
   Escalation: Drag-drop failed → File dialog timeout → Form interception succeeded
```

### JSON (machine-parseable)

```bash
$ kaboom-cmd interact upload \
    --selector "#Filedata" \
    --file-path video.mp4 \
    --format json
```

#### Output:
```json
{
  "success": true,
  "stage": 3,
  "file_size_bytes": 1073741824,
  "file_name": "video.mp4",
  "duration_ms": 5600,
  "status": "Form interception: POST submitted to platform",
  "escalation_reason": "Drag-drop failed (platform rejected synthetic File)"
}
```

### CSV (bulk operations)

```bash
$ kaboom-cmd interact upload \
    --csv-file videos.csv \
    --selector "#Filedata" \
    --format csv
```

#### Output:
```csv
file_path,file_name,size_bytes,status,stage,duration_ms,error
/videos/v1.mp4,v1.mp4,1073741824,success,3,5600,
/videos/v2.mp4,v2.mp4,2147483648,success,1,2300,
/videos/v3.mp4,v3.mp4,536870912,error,4,8000,OS automation failed after 3 retries
```

---

## Streaming vs Synchronous

### Default: Synchronous (Block Until Complete)

```bash
kaboom-cmd interact upload --selector "#File" --file video.mp4
# Blocks until upload finishes
# Returns final result
```

## Exit codes:
- `0` = Success
- `1` = Error (upload failed, all stages exhausted, etc.)
- `2` = Usage error (missing args, invalid selector)

### Optional: Streaming Progress (--stream flag)

```bash
kaboom-cmd interact upload \
  --selector "#File" \
  --file video.mp4 \
  --stream
```

**Output:** Newline-delimited JSON (one event per line)

```json
{"type":"start","file":"video.mp4","size":1073741824}
{"type":"progress","stage":1,"status":"drag_drop_attempt","percent":0}
{"type":"progress","stage":1,"status":"drag_drop_failed","percent":0,"reason":"platform rejected synthetic File"}
{"type":"progress","stage":2,"status":"file_dialog_attempt","percent":0}
{"type":"progress","stage":2,"status":"file_dialog_timeout","percent":0}
{"type":"progress","stage":3,"status":"form_interception_attempt","percent":5}
{"type":"progress","stage":3,"status":"uploading","percent":50,"bytes_sent":536870912,"eta_seconds":15}
{"type":"progress","stage":3,"status":"uploading","percent":100,"bytes_sent":1073741824}
{"type":"complete","stage":3,"success":true,"duration_ms":5600}
```

**Advantage:** Real-time monitoring, good for CI/CD logs, large files

---

## Bulk Operations (CSV)

### CSV Input Format

#### videos.csv:
```csv
file_path,title,tags,category
/videos/video1.mp4,My First Video,tag1;tag2,News
/videos/video2.mp4,My Second Video,tag3;tag4,Entertainment
/videos/video3.mp4,My Third Video,tag5;tag6,Gaming
```

### Command

```bash
kaboom-cmd interact upload \
  --csv-file videos.csv \
  --selector "#Filedata" \
  --selector-title "#title" \
  --selector-tags "#tags" \
  --selector-category "#category_primary" \
  --format csv \
  --output results.csv
```

### Processing Flow

```
For each row in CSV:
  1. Fill --selector-title with title value
  2. Fill --selector-tags with tags value
  3. Fill --selector-category with category value
  4. Call interact(upload, selector, file_path)
  5. Click submit button (if --auto-submit flag)
  6. Wait for redirect/completion
  7. Log result to results.csv
```

### Output (results.csv)

```csv
file_path,title,status,stage,duration_ms,error
/videos/video1.mp4,My First Video,success,3,5600,
/videos/video2.mp4,My Second Video,success,1,2300,
/videos/video3.mp4,My Third Video,error,4,8000,OS automation permission denied
```

---

## Server Lifecycle Management

### Auto-Start Behavior

```bash
kaboom-cmd interact upload --selector "#File" --file video.mp4
```

#### What happens:
1. Check if `kaboom-mcp` is running on `$KABOOM_PORT` (default 9223)
2. If not running, auto-start: `kaboom-mcp --enable-os-upload-automation --trust-llm-context`
3. Wait for server to be ready (5s timeout)
4. Execute command
5. Keep server running (don't shut down)

### Disable Auto-Start

```bash
export KABOOM_NO_AUTO_START=1
kaboom-cmd interact upload --selector "#File" --file video.mp4
# Error: Server not running on port 9223
# Exit code 1
```

### Manual Server Control

```bash
# Start server manually
kaboom-mcp --enable-os-upload-automation --trust-llm-context &

# Use it
kaboom-cmd interact upload --selector "#File" --file video.mp4

# Server keeps running for next command
```

### Port Selection

```bash
# Use custom port
kaboom-cmd interact upload \
  --selector "#File" \
  --file video.mp4 \
  --server-port 9224

# Starts server on 9224 if not running
```

---

## Error Handling

### Human-Readable Errors

```bash
$ kaboom-cmd interact upload --selector "#BadSelector" --file video.mp4
❌ Upload failed: Form not found
   Selector: #BadSelector
   Reason: No element matching selector on current page
   Recovery: Check selector syntax, verify page has loaded
Exit code: 1
```

### JSON Errors

```bash
$ kaboom-cmd interact upload \
    --selector "#BadSelector" \
    --file video.mp4 \
    --format json
```

#### Output:
```json
{
  "success": false,
  "error": "form_not_found",
  "selector": "#BadSelector",
  "message": "No element matching selector on current page",
  "stage_reached": 3,
  "recovery_suggestions": [
    "Check selector syntax",
    "Verify page has loaded",
    "Try a different selector"
  ]
}
```

### Exit Codes

| Code | Meaning | Example |
|------|---------|---------|
| 0 | Success | Upload completed |
| 1 | Error | File not found, upload failed |
| 2 | Usage error | Missing required args, bad JSON |

### Suggested Error Messages (stderr)

```
{OPERATION}: {ROOT_CAUSE}. {RECOVERY_ACTION}

upload_failed_all_stages: All escalation stages exhausted after 3 retries.
Try manual upload or check file permissions.

form_not_found: Selector "#WrongID" did not match any element.
Verify selector is correct and page has loaded.

file_not_found: /path/to/video.mp4 does not exist.
Check file path and permissions.
```

---

## All Tool Commands

### interact tool

```bash
kaboom-cmd interact <action> [options]
```

#### Actions:

| Action | Example | Purpose |
|--------|---------|---------|
| `upload` | `--selector "#File" --file-path video.mp4` | Upload file (4-stage escalation) |
| `fill` | `--selector "#title" --text "My Title"` | Fill form field |
| `click` | `--selector "button[type=submit]"` | Click element |
| `get-text` | `--selector ".status"` | Read element text |
| `get-value` | `--selector "input[name=title]"` | Read input value |
| `get-attribute` | `--selector "a" --attribute href` | Read attribute |
| `set-attribute` | `--selector "input" --attribute disabled --value true` | Set attribute |
| `wait-for` | `--selector ".loading" --timeout 5000` | Wait for element |
| `scroll-to` | `--selector "form"` | Scroll element into view |
| `key-press` | `--key Enter` | Press keyboard key |

### observe tool

```bash
kaboom-cmd observe <mode> [options]
```

#### Modes:

| Mode | Example | Purpose |
|------|---------|---------|
| `logs` | `--limit 50 --min-level warn` | Get browser console logs |
| `network` | `--url "api.example.com" --status-min 400` | Get network requests |
| `errors` | `--limit 20` | Get JavaScript errors |
| `performance` | `--limit 5` | Get Web Vitals |
| `accessibility` | `--scope "form" --tags wcag20` | Run a11y audit |

### configure tool

```bash
kaboom-cmd configure <action> [options]
```

#### Actions:

| Action | Example | Purpose |
|--------|---------|---------|
| `noise-rule` | `--pattern "favicon.ico" --reason "noise"` | Add noise filter |
| `store` | `--key "session" --data '{"id":"123"}'` | Save to local storage |
| `load` | `--key "session"` | Load from local storage |

### generate tool

```bash
kaboom-cmd generate <format> [options]
```

#### Formats:

| Format | Example | Purpose |
|--------|---------|---------|
| `test` | `--test-name "upload_test"` | Generate Playwright test |
| `reproduction` | `--error-message "timeout"` | Generate bug repro script |
| `har` | `--url "api.example.com"` | Export HAR file |
| `csp` | `--mode strict` | Generate CSP policy |

---

## Implementation Details

### Go Binary Structure

```
kaboom-cmd (CLI binary)
  ├─ main.go
  │   ├─ Parse CLI args
  │   ├─ Load config (env vars, .kaboom.json, flags)
  │   ├─ Check/start server
  │   ├─ Route to command handler
  │   └─ Format output
  │
  ├─ commands/
  │   ├─ interact.go
  │   ├─ observe.go
  │   ├─ configure.go
  │   └─ generate.go
  │
  ├─ server/
  │   ├─ lifecycle.go (start/check/connect)
  │   └─ client.go (JSON-RPC wrapper)
  │
  ├─ output/
  │   ├─ human.go (pretty formatting)
  │   ├─ json.go (JSON serialization)
  │   └─ csv.go (CSV writer)
  │
  └─ config/
      └─ loader.go (config cascade)
```

### How CLI Args Map to MCP Calls

```bash
kaboom-cmd interact upload \
  --selector "#Filedata" \
  --file-path ./video.mp4
```

#### Becomes:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "interact",
    "arguments": {
      "action": "upload",
      "selector": "#Filedata",
      "file_path": "/absolute/path/to/video.mp4"
    }
  }
}
```

### Streaming Implementation

#### For synchronous mode:
- Call MCP tool, wait for complete response
- Format output, print to stdout
- Exit with appropriate code

#### For streaming mode (--stream):
- Subscribe to MCP progress events (newline-delimited JSON)
- Print each event as received to stdout
- Wait for completion event
- Exit with appropriate code

---

## Security Considerations

### No Credential Storage

- CLI does NOT store passwords/auth tokens
- User must be logged into browser beforehand
- Forms/uploads use existing session cookies
- No new credential handling needed

### No Remote Server Connections

- `kaboom-cmd` only talks to local `kaboom-mcp`
- Default: localhost:9223 (not exposed to network)
- File paths are local filesystem only
- No external API calls from CLI

### Privilege Escalation

- Stage 4 OS automation requires OS-level permissions
- macOS: AppleScript (needs accessibility permission)
- Windows: UIA (needs input simulation permission)
- Linux: xdotool (needs X11/Wayland access)
- User must grant permissions explicitly (first time)

---

## Testing Strategy

### Unit Tests

- Config loading (priority order)
- Argument parsing
- Output formatting (human/JSON/CSV)
- Error message generation

### Integration Tests

- Start server, run command, verify output
- Bulk CSV processing
- Streaming progress
- Exit codes

### End-to-End Tests

- Upload workflow (all 4 stages)
- Form filling + upload
- Error recovery

---

## Future: Web Client Wrapper

```
User (Web Browser)
  ↓
Web Client (Vue.js, HTML)
  ↓ HTTP requests
Web Wrapper Service (Node.js, Python, Go)
  ↓ shell execution
kaboom-cmd (CLI binary)
  ↓ MCP JSON-RPC
kaboom-mcp (MCP server)
  ↓
Browser Extension / Go Server
```

### Web Client Command Translation

```typescript
// Web client calls (JavaScript)
await client.upload({
  selector: "#File",
  filePath: "./video.mp4"
});

// Translates to shell command
shell.exec("kaboom-cmd interact upload --selector '#File' --file-path ./video.mp4");

// Returns JSON result
{
  "success": true,
  "stage": 3,
  "duration_ms": 5600
}
```

---

## References

- [CLAUDE.md](../../CLAUDE.md) — Core rules (TDD, no deps, 5 tools only)
- [upload/tech-spec.md](../file-upload/tech-spec.md) — Upload feature
- Unix Philosophy: Do one thing, do it well
- Exit codes: [POSIX Standard](https://en.wikipedia.org/wiki/Exit_status)
