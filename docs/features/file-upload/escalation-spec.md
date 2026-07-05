---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Upload Escalation Spec

Behavioral specification for Stage 1 → Stage 4 escalation pipeline.

---

## Section 1: Escalation Trigger

**When does escalation happen?**

After the extension injects a file via DataTransfer (Stage 1), it MUST verify the file persisted on the input element. Some forms validate `event.isTrusted` on the `change` event and clear programmatically-set files. The extension detects this by checking `el.files.length` after a delay.

**Verification timing:** 500ms after Stage 1 injection. This is sufficient because:
- The `onchange` handler fires synchronously during `el.dispatchEvent(new Event('change'))`
- The handler that clears `this.value = ''` executes immediately, before the event dispatch returns
- By the time 500ms elapses, the file is either present (Stage 1 succeeded) or gone (Stage 1 was rejected)

**Verification method:** `chrome.scripting.executeScript(MAIN world)` checks:
- `el.files !== null && el.files.length > 0` → file present = Stage 1 success
- `el.files === null || el.files.length === 0` → file cleared = escalate

---

## Section 2: Escalation Path

When Stage 1 verification fails, the extension escalates directly to Stage 4. Stages 2 and 3 are skipped because:
- Stage 2 (dialog inject) is a stub in the Go daemon — validates file but doesn't inject
- Stage 3 (form submit) bypasses the browser, so the user never sees the upload happen in the browser (violates the "everything happens visually in the browser" requirement)
- Stage 4 (OS automation) is the only approach that produces a real user interaction with `isTrusted=true`

**Escalation sequence:**

1. Extension detects Stage 1 file was cleared
2. Extension shows toast: "Escalating to OS automation..."
3. Extension clicks the file input element via `el.click()` in MAIN world
4. Wait 1500ms for native file dialog to open
5. Extension calls Go daemon `POST /api/os-automation/inject` with `{ file_path, browser_pid: 0 }`
6. Daemon auto-detects Chrome PID (if `browser_pid=0`)
7. Daemon executes platform-specific OS automation:
   - **macOS:** AppleScript sends Cmd+Shift+G, types path, presses Enter twice
   - **Linux:** xdotool activates "Open" window, Ctrl+L, types path, presses Return
   - **Windows:** PowerShell SendKeys types path, presses Enter
8. Wait 2000ms for dialog to close and file to appear on input
9. Extension verifies file is on input: `el.files[0]?.name`
10. Extension reports result with `stage: 4` and `escalation_reason`

---

## Section 3: Data Flow

```
AI Agent
  │
  │ MCP: interact(upload, selector="#file-input", file_path="/path/to/file.txt")
  ▼
Go Daemon (tools_interact_upload.go)
  │ Creates PendingQuery{type:"upload", params:{selector, file_path, ...}}
  │ Returns {status:"queued", correlation_id:"upload_xxx"}
  ▼
Extension (pending-queries.ts → upload-handler.ts)
  │
  ├─ Stage 1: POST /api/file/read → base64 data
  │  └─ chrome.scripting.executeScript(MAIN) → DataTransfer inject → dispatch change/input
  │
  ├─ Verify (500ms later): chrome.scripting.executeScript(MAIN) → check el.files.length
  │  ├─ files.length > 0 → SUCCESS (Stage 1) → sendAsyncResult(complete, {stage:1})
  │  └─ files.length === 0 → ESCALATE
  │
  ├─ Stage 4 Escalation:
  │  ├─ chrome.scripting.executeScript(MAIN) → el.click() → native file dialog opens
  │  ├─ Wait 1500ms
  │  ├─ POST /api/os-automation/inject {file_path, browser_pid:0}
  │  │   └─ Go daemon: detectBrowserPID() → AppleScript/xdotool/PowerShell
  │  ├─ Wait 2000ms
  │  ├─ chrome.scripting.executeScript(MAIN) → check el.files[0]?.name
  │  └─ sendAsyncResult(complete, {stage:4, escalation_reason:"stage1_file_cleared"})
  │
  ▼
AI Agent
  │ MCP: observe(what:"command_result", correlation_id:"upload_xxx")
  │ Gets: {status:"complete", data:{stage:4, escalation_reason:"stage1_file_cleared", file_name:"file.txt"}}
```

---

## Section 4: Hardened Upload Form Behavior

The Python test server (`upload-server.py`) serves two forms:
- `GET /upload` — standard form, Stage 1 works (existing)
- `GET /upload/hardened` — hardened form, Stage 1 fails, Stage 4 required (new)

**Hardened form differences from standard form:**
- The `<input type="file">` has an inline `onchange` handler
- On `change` event: checks `event.isTrusted`
- If `isTrusted === false`: sets `this.value = ''` (clears file), updates a `<p id="trust-status">` element to show "REJECTED: event.isTrusted=false"
- If `isTrusted === true`: updates status to "OK: trusted event"
- Form POSTs to `/upload` (same backend handler, unchanged)
- Same session cookie and CSRF token flow as standard form

**Why `isTrusted` works as the gating mechanism:**
- DataTransfer + `el.dispatchEvent(new Event('change'))` → `isTrusted=false` (synthetic)
- Native file dialog selection → `isTrusted=true` (user-initiated)
- There is no way to forge `isTrusted=true` from JavaScript — it's a read-only property set by the browser

---

## Section 5: Chrome PID Auto-Detection

The Go daemon's Stage 4 handler currently requires `browser_pid > 0`. The macOS AppleScript doesn't use the PID (targets "System Events" globally), but the handler validates it.

**New behavior when `browser_pid <= 0`:**
1. Call platform-specific detection function
2. If Chrome found → use detected PID, continue
3. If Chrome not found → return error with OS-specific instructions

**Detection methods:**
- **macOS:** `pgrep -x "Google Chrome"` → parse first line as PID
- **Linux:** `pgrep -x "chrome"`, if fails try `pgrep -x "chromium"` → parse first line
- **Windows:** `tasklist /FI "IMAGENAME eq chrome.exe" /FO CSV /NH` → parse second CSV field

---

## Section 6: Error Messages (Complete Catalog)

Every error the user might encounter, with exact text and resolution:

**Extension errors (shown in MCP observe result):**

| Error | Message | Resolution |
|-------|---------|------------|
| Click failed | `"Escalation failed: could not click file input '{selector}'. Verify the element exists, is visible, and is type='file'."` | Check selector, page state |
| Daemon unreachable | `"Escalation failed: cannot reach daemon at {url}/api/os-automation/inject. Error: {fetch_error}"` | Verify daemon is running |
| Stage 4 disabled | `"Escalation failed: OS automation disabled on daemon. Restart with: kaboom-mcp --daemon --enable-os-upload-automation --upload-dir=/path/to/uploads"` | Restart daemon with flag |
| OS automation failed (macOS) | `"Stage 4 AppleScript failed: {error}. Grant Accessibility: System Settings → Privacy & Security → Accessibility → enable {terminal_app}."` | Grant macOS Accessibility |
| OS automation failed (Linux) | `"Stage 4 xdotool failed: {error}. Install: sudo apt install xdotool (Debian/Ubuntu) or sudo dnf install xdotool (Fedora). Ensure X11/Wayland session is active."` | Install xdotool |
| OS automation failed (Windows) | `"Stage 4 SendKeys failed: {error}. Run terminal as Administrator. Ensure Chrome file dialog is visible."` | Elevate privileges |
| File not on input after Stage 4 | `"Stage 4 completed but file not found on input '{selector}'. The native file dialog may not have been in focus. Verify file exists: {file_path}"` | Check dialog focus, file path |
| Chrome PID not found (macOS) | `"Cannot detect Chrome: pgrep -x 'Google Chrome' found no process. Launch Google Chrome first."` | Launch Chrome |
| Chrome PID not found (Linux) | `"Cannot detect Chrome: pgrep found neither 'chrome' nor 'chromium'. Launch Chrome/Chromium first."` | Launch browser |
| Chrome PID not found (Windows) | `"Cannot detect Chrome: tasklist found no chrome.exe. Launch Google Chrome first."` | Launch Chrome |

**Go daemon errors (returned in UploadStageResponse):**

| Error | Message | Resolution |
|-------|---------|------------|
| `--upload-dir` not set | `"Stages 2-4 require --upload-dir. Restart: kaboom-mcp --daemon --upload-dir=/path/to/uploads"` | Restart with flag |
| `--enable-os-upload-automation` not set | `"OS-level upload automation is disabled. Restart: kaboom-mcp --daemon --enable-os-upload-automation --upload-dir=/path/to/uploads"` | Restart with flag |
| Path not in upload-dir | `"File path not within --upload-dir ({upload_dir}). Move the file to {upload_dir} and retry."` | Move file |
| Path on denylist | `"File path is not allowed: {path} matches sensitive pattern '{pattern}'."` | Use a different file |
| macOS Accessibility denied | `"AppleScript failed: {osascript_error}. Fix: System Settings → Privacy & Security → Accessibility → enable {TERM_PROGRAM or 'your terminal'}."` | Grant permission |
| Linux xdotool missing | `"xdotool not found. Install: sudo apt install xdotool (Debian/Ubuntu) or sudo dnf install xdotool (Fedora)."` | Install package |
| AppleScript path injection | `"Invalid file path for OS automation: file path contains {chars}."` | Use clean file path |

**Smoke test errors (shown in test output):**

| Scenario | Test output | Category |
|----------|------------|----------|
| Stage 4 succeeded, MD5 match | `PASS [15.16]: Stage 4 escalation: file reached server via OS automation, MD5 match (abc123)` | pass |
| Stage 4 succeeded, MD5 mismatch | `FAIL [15.16]: Stage 4 file reached server but MD5 mismatch. Expected abc123, got def456.` | fail |
| Accessibility denied | `SKIP [15.16]: Stage 4 needs macOS Accessibility permission. Fix: System Settings → Privacy & Security → Accessibility → enable {app}. Error: {detail}` | skip |
| xdotool missing | `SKIP [15.16]: Stage 4 needs xdotool. Fix: sudo apt install xdotool. Error: {detail}` | skip |
| Flag missing | `FAIL [15.16]: Daemon missing --enable-os-upload-automation flag. Error: {detail}` | fail |
| Chrome not running | `FAIL [15.16]: Chrome not detected. Is Google Chrome running? Error: {detail}` | fail |
| Upload timed out | `SKIP [15.16]: Upload timed out — extension may not have escalation support yet.` | skip |
| No extension | `SKIP [15.16]: Extension or pilot not available.` | skip |

---

## Section 7: Timing Constants

| Constant | Value | Where | Why |
|----------|-------|-------|-----|
| Post-inject verification delay | 500ms | Extension | Wait for synchronous onchange to clear file |
| Post-click dialog open delay | 1500ms | Extension | Native file dialog animation + render |
| AppleScript internal delays | 1800ms total | Go daemon | 0.5s wait + 0.5s Go-To-Folder + 0.3s type + 0.5s navigate |
| Post-automation dialog close delay | 2000ms | Extension | Dialog close + Chrome processes file + change event |
| Poll interval for command_result | 500ms (normal), 1000ms (Stage 4) | Smoke test | Stage 4 needs longer total timeout due to ~6s escalation |
| Max polls for Stage 4 tests | 30 | Smoke test | 30 x 1s = 30s max wait |

---

## Section 8: What Does NOT Change

- Tests 15.0-15.15 — unchanged, all existing behavior preserved
- `POST /upload` handler in upload-server.py — unchanged, accepts both standard and hardened form submissions
- Stage 1, 2, 3 Go daemon HTTP endpoints — unchanged
- The `queueUpload()` function in tools_interact_upload.go — unchanged
- `escalation_timeout_ms` parameter — still unused (future: could be used to control verification delay)
