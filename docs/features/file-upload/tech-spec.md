---
doc_type: tech-spec
feature_id: file-upload
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# File Upload via Go Server — Tech Spec (Revised)

**Feature:** Multi-stage intelligent file upload with automatic escalation, using the Go server and OS-level automation to bypass browser sandbox restrictions.

## Use Cases:
- Bulk video uploads (Rumble, YouTube, etc.)
- Enterprise form submissions (CSRF-protected)
- Large file uploads (>2GB) to any HTML form
- Unattended automation (LLM-driven, OS-level interaction)

---

## Problem Statement

Chrome extensions run in a sandboxed environment that prevents:

- Reading arbitrary files from disk
- Programmatically populating `<input type="file">` elements
- Direct filesystem access from JavaScript
- Simulating native OS file dialogs

However, Kaboom has a Go server running as a native process with full filesystem and OS access. This spec defines how to leverage it with **intelligent escalation**: start with the least invasive approach (drag-drop), then escalate through form interception and finally OS-level automation.

---

## Security Model: Folder-Scoped Permissions + Sensitive Path Denylist

File upload automation requires **explicit server-side flags** to enable:

```bash
kaboom-mcp --enable-os-upload-automation --upload-dir=/Users/brenn/Uploads
```

### Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--enable-os-upload-automation` | For Stage 4 | Enables OS-level automation (AppleScript/xdotool). Stages 1-3 are always available. |
| `--upload-dir=<path>` | For Stages 2-4 | Single directory from which files may be uploaded. Recursive (subdirectories included). |
| `--upload-deny-pattern=<glob>` | No | Additional sensitive path patterns to block (extends built-in denylist). Repeatable. |

### Behavior Matrix

| Flags set | Stage 1 (file read) | Stages 2-3 (dialog/form) | Stage 4 (OS automation) |
|-----------|---------------------|--------------------------|-------------------------|
| Neither | Allowed (denylist enforced) | Allowed (denylist enforced) | 403 Forbidden |
| `--upload-dir` only | Allowed within `--upload-dir` + denylist | Allowed within `--upload-dir` + denylist | 403 Forbidden |
| `--enable-os-upload-automation` only | Allowed (denylist enforced) | Allowed (denylist enforced) | Error: `--upload-dir required` |
| Both flags | Allowed within `--upload-dir` + denylist | Allowed within `--upload-dir` + denylist | Allowed within `--upload-dir` + denylist |

### Upload Directory Validation (at startup)

When `--upload-dir` is provided, the server validates it at startup and **refuses to start** if any check fails:

1. **Must be an absolute path** — no relative paths
2. **Must exist and be a directory** — not a file, not missing
3. **Must not be a symlink** — `filepath.EvalSymlinks(dir)` must equal the original path. Prevents pointing `--upload-dir` at a symlink that targets a sensitive directory.
4. **Must not be a known sensitive directory** — checked against the built-in denylist (see below)
5. **Must not be a root or home directory** — rejects `/`, `/home`, `/Users`, `C:\`, `C:\Users`, or the user's home directory itself (`~`). Upload dir must be a subdirectory.

### File Path Validation (per request, all stages)

Every file access request goes through this validation chain:

```
1. filepath.Clean(path)              — resolve .. components
2. filepath.IsAbs(cleaned)           — must be absolute
3. filepath.EvalSymlinks(cleaned)    — resolve symlinks to real path
4. checkDenylist(resolved)           — reject sensitive paths
5. checkUploadDir(resolved)          — must be within --upload-dir (Stages 2-4)
6. os.Open(resolved)                 — open the resolved path, not the original
7. file.Stat()                       — fstat on open handle (TOCTOU safe)
```

### Sensitive Path Denylist (always active, all stages)

The denylist is **always enforced**, even for Stage 1 without `--upload-dir`. It blocks file access based on resolved (symlink-free) paths.

#### Built-in patterns (hardcoded, cannot be removed):

```
# SSH & GPG keys
~/.ssh/*
~/.gnupg/*

# Cloud credentials
~/.aws/*
~/.config/gcloud/*
~/.azure/*
~/.config/doctl/*
~/.kube/config

# Environment & secrets
**/.env
**/.env.*
**/credentials*
**/secrets*
**/*.pem
**/*.key
**/*.p12
**/*.pfx
**/*.keystore

# Shell history
~/.bash_history
~/.zsh_history
~/.node_repl_history
~/.python_history

# Browser data
~/Library/Application Support/Google/Chrome/*
~/.config/google-chrome/*
~/.config/chromium/*
~/Library/Application Support/Firefox/*
~/.mozilla/firefox/*

# System files (Unix)
/etc/shadow
/etc/passwd
/etc/sudoers
/proc/*
/sys/*

# System files (Windows)
C:\Windows\System32\config\*
C:\Windows\System32\drivers\etc\*

# Package manager auth
~/.npmrc
~/.pypirc
~/.docker/config.json
~/.config/gh/hosts.yml
~/.gitconfig
**/.git/config
```

**User-extensible:** Additional patterns can be added via `--upload-deny-pattern`:

```bash
kaboom-mcp --enable-os-upload-automation \
  --upload-dir=/Users/brenn/Uploads \
  --upload-deny-pattern="**/company-secrets/*" \
  --upload-deny-pattern="**/*.sqlite"
```

### Error Response (denied path)

When a file is blocked by the denylist or is outside `--upload-dir`:

```json
{
  "success": false,
  "error": "path_denied",
  "message": "File path is not allowed: /Users/brenn/.ssh/id_rsa matches sensitive path pattern '~/.ssh/*'.",
  "retry": "Move the file to your upload directory (/Users/brenn/Uploads) and retry.",
  "hint": "The upload directory is set via --upload-dir. Sensitive paths like SSH keys, credentials, and environment files are always blocked."
}
```

When `--upload-dir` is not set and Stages 2-4 are attempted:

```json
{
  "success": false,
  "error": "upload_dir_required",
  "message": "Stages 2-4 require --upload-dir to be set.",
  "retry": "Restart the server with --upload-dir=/path/to/folder and move your files there.",
  "hint": "Stage 1 (file read) works without --upload-dir but is subject to the sensitive path denylist."
}
```

### Symlink Protection

Symlinks are resolved **before** any validation:

```
Original path:    /Users/brenn/Uploads/photo.jpg
Resolved path:    /Users/brenn/.ssh/id_rsa        ← symlink target
Result:           BLOCKED (matches ~/.ssh/*)
```

This prevents:
- Symlinks from `--upload-dir` pointing to sensitive files
- Symlinks bypassing the `--upload-dir` boundary
- `--upload-dir` itself being a symlink to a sensitive directory

### Rationale

File upload automation is powerful and invasive (especially Stage 4 OS automation). The folder-scoped model:
- **Minimizes blast radius** — even if the LLM is manipulated by prompt injection, it can only access files the user explicitly placed in the upload folder.
- **Zero implicit access** — no default directories, no expanding home access.
- **Defense in depth** — denylist + folder scope + symlink resolution + TOCTOU-safe opens.
- **Auditable** — the upload directory is a single, visible location the user controls.

---

## Architecture: 4-Stage Intelligent Escalation

### System Design

```
Browser Extension (MV3 sandboxed)
  ↓ HTTP/stdio
Go Server (native process, full filesystem + OS access)
  ├─ Stage 1: Drag-drop simulation (extension only)
  ├─ Stage 2: File dialog interception (extension + Go monitoring)
  ├─ Stage 3: Form interception (Go direct submission)
  └─ Stage 4: OS automation (Go simulates native file picker)
```

### Escalation Flow (User Perspective)

```
User calls: interact(action: "upload", selector: "#Filedata", file_path: "/path/to/video.mp4")
  ↓
[5 second timeout for manual interaction]
  ├─ User clicks file input → STAGE 2 (file dialog)
  └─ User does nothing → STAGE 1 (drag-drop attempt)
     ├─ Success → DONE ✅
     └─ Fails → STAGE 2 (file dialog interception)
        ├─ Success → DONE ✅
        └─ Fails → STAGE 3 (form interception, Go POSTs directly)
           ├─ Success → DONE ✅
           └─ Fails → STAGE 4 (OS automation, Go simulates file picker)
              ├─ Success → DONE ✅
              └─ Fails (retried 3x) → Ask user or return error
```

---

## API Design

### Tool Signature: `interact(action: "upload", ...)`

#### Parameters:

- `selector` (required): CSS selector for `<input type="file">` element, OR `{ apiEndpoint: "..." }` for direct API uploads
- `file_path` (required): Absolute path to file on user's disk
- `submit` (optional, default: false): Whether to auto-submit form after injection
- `escalation_timeout_ms` (optional, default: 5000): Time to wait for manual interaction before auto-escalating

#### Example Usage:

```typescript
// HTML form upload
await interact({
  action: "upload",
  selector: "#Filedata",
  file_path: "/Users/brenn/Videos/video.mp4",
  submit: true
});

// API endpoint upload
await interact({
  action: "upload",
  apiEndpoint: "https://api.example.com/upload",
  file_path: "/Users/brenn/Videos/video.mp4"
});
```

#### Response (on success):

```json
{
  "success": true,
  "stage": 1,
  "file_size_bytes": 1073741824,
  "file_name": "video.mp4",
  "duration_ms": 2345,
  "status": "File injected and ready"
}
```

#### Response (if escalation occurred):

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

#### Error Response (all stages exhausted):

```json
{
  "success": false,
  "error": "upload_failed_all_stages",
  "last_stage": 4,
  "last_error": "OS automation failed after 3 retries",
  "suggestions": [
    "Verify file path is correct",
    "Check file permissions",
    "Ensure user is logged into platform",
    "Try manual upload"
  ]
}
```

### Stage-by-Stage Implementation

#### Stage 1: Drag-Drop Simulation (Least Invasive)

##### How it works:
1. Extension loads file data from Go server (via `POST /api/file/read`)
2. Decodes base64 → creates File/Blob object
3. Simulates drag-drop event on target element
4. Platform's JavaScript handler receives synthetic File object

**Go Endpoint:** `POST /api/file/read`

##### Request:
```json
{
  "file_path": "/Users/brenn/Videos/video.mp4"
}
```

##### Response (streaming for large files):
```json
{
  "success": true,
  "file_name": "video.mp4",
  "file_size": 1073741824,
  "mime_type": "video/mp4",
  "data_base64": "AAAA...[base64]...AAAA"
}
```

**When it works:** YouTube, Vimeo, custom dropzone handlers

**When it fails:** Rumble (rejects synthetic Files), strict CORS

**Timeout:** 5s before escalating to Stage 2

---

#### Stage 2: File Dialog Interception (Medium)

##### How it works:
1. Extension hooks the file input's click handler
2. User (or automation) clicks file input → browser native file dialog opens
3. Go server monitors for dialog and injects file path
4. Browser populates `<input type="file">` with user's file

**Go Endpoint:** `POST /api/file/dialog/inject`

##### Request:
```json
{
  "file_path": "/Users/brenn/Videos/video.mp4",
  "browser_pid": 12345
}
```

**When it works:** Most platforms (requires actual file picker interaction)

**When it fails:** Automated systems without user interaction

**Timeout:** 10s (waiting for user to click or Go to auto-simulate)

---

#### Stage 3: Form Interception (Most Invasive)

##### How it works:
1. Extension captures form submission attempt
2. Extracts: form action, fields, CSRF token, cookies
3. Sends to Go server
4. Go server reads file from disk and POSTs directly to platform
5. No base64 encoding needed—streams file directly

**Go Endpoint:** `POST /api/form/submit`

##### Request:
```json
{
  "form_action": "https://rumble.com/upload.php",
  "method": "POST",
  "fields": {
    "title": "My Video",
    "tags": "tag1,tag2",
    "category_primary": "News"
  },
  "file_input_name": "Filedata",
  "file_path": "/Users/brenn/Videos/video.mp4",
  "csrf_token": "abc123xyz",
  "cookies": "session=xyz;csrf=abc;..."
}
```

##### Go Implementation (pseudocode):

```go
func handleFormSubmit(w http.ResponseWriter, r *http.Request) {
  // 1. Read file directly from disk (no base64)
  file, _ := os.Open(req.FilePath)
  defer file.Close()

  // 2. Build multipart form with file
  body := &bytes.Buffer{}
  writer := multipart.NewWriter(body)

  // Add form fields
  for k, v := range req.Fields {
    writer.WriteField(k, v)
  }

  // Add file (streamed, no memory bloat)
  fw, _ := writer.CreateFormFile(req.FileInputName, filepath.Base(req.FilePath))
  io.Copy(fw, file) // Stream directly
  writer.Close()

  // 3. POST to platform with user's auth
  httpReq, _ := http.NewRequest(req.Method, req.FormAction, body)
  httpReq.Header.Set("Cookie", req.Cookies)
  httpReq.Header.Set("Content-Type", writer.FormDataContentType())

  resp, _ := http.DefaultClient.Do(httpReq)
  return resp.StatusCode < 400
}
```

##### Advantages:
- Works for ANY HTML form with CSRF protection
- File streaming = zero memory overhead
- Handles multi-GB files

**When it works:** Rumble, YouTube, enterprise systems, any form-based upload

---

#### Stage 4: OS-Level Automation (Most Invasive)

##### How it works:
1. Go binary directly simulates OS file picker interaction
2. Platform-specific: AppleScript (macOS), UIA (Windows), xdotool (Linux)
3. Go finds the open file dialog, injects the file path, simulates "Open" button click
4. Browser receives file from native dialog (highest fidelity)

##### macOS Implementation (AppleScript):

```bash
osascript -e 'tell application "Google Chrome"
  tell application "System Events"
    keystroke "'$FILE_PATH'" using command down
    keystroke return
  end tell
end tell'
```

##### Windows Implementation (UIA + SendInput):

```go
// Locate file dialog window
dialog := findWindow("Open", "File")
if dialog != nil {
  // Type path and press Enter
  sendKeys(filepath.ToShortName(filePath))
  sendKey("Return")
}
```

##### Linux Implementation (xdotool):

```bash
xdotool search --name "Open File" windowactivate key ctrl+l
xdotool type "$FILE_PATH"
xdotool key Return
```

##### Advantages:
- Works with ANY file dialog (no platform detection needed)
- Highest fidelity (browser sees real file dialog result)
- Bypasses all CSRF/auth checks

##### Limitations:
- Requires OS-level permissions (accessibility, input simulation)
- Platform-specific code (3 implementations)
- Slowest (1-2s per upload due to OS interaction)

**Retries:** 3 retries with 1s exponential backoff if dialog doesn't appear

### Progress Tracking & Large File Strategy

**Smart Progress Reporting** (based on file size):

| File Size | Strategy | Overhead | Use Case |
|-----------|----------|----------|----------|
| < 100MB | Simple (result only) | ~2% | Fast uploads, quick feedback |
| 100MB - 2GB | Periodic (10% chunks) | ~5% | Medium files, user confidence |
| > 2GB | Detailed (byte-level + ETA) | ~8% | Large files, transparency |

#### Implementation:

For files > 2GB, Go server streams directly (no base64):
- Reads file in 64MB chunks
- POSTs chunks to platform (multipart range support)
- Reports progress: `{ bytes_sent: 500MB, total: 4GB, percent: 12.5%, eta_seconds: 3600 }`

For files < 100MB:
- Load into memory → base64 encode → inject
- Return only final success/error

For files 100MB - 2GB:
- Hybrid: load 100MB chunks → base64 → inject
- Report progress every 10% or 100MB whichever is smaller

#### Progress Callback (MCP streaming):

```json
{
  "status": "uploading",
  "bytes_sent": 536870912,
  "total_bytes": 1073741824,
  "percent": 50,
  "duration_ms": 15000,
  "eta_seconds": 15,
  "speed_mbps": 35.8
}
```

---

## Platform Guidance

### Rumble Upload Example

#### Platform-specific details:
- File input: `#Filedata`
- Form action: `https://rumble.com/upload.php`
- Method: POST multipart/form-data
- Max file size: 15 GB
- CSRF protection: Yes (auto-detected)
- Auth required: Yes (session cookies)

#### Bulk upload workflow:

```typescript
// Read CSV with columns: file_path, title, tags, category
const videos = await readCSV('videos.csv');

for (const video of videos) {
  // Fill form fields
  await interact({ action: 'fill', selector: '#title', text: video.title });
  await interact({ action: 'fill', selector: '#tags', text: video.tags });

  // Upload file (auto-escalates if needed)
  const result = await interact({
    action: 'upload',
    selector: '#Filedata',
    file_path: video.file_path
  });

  if (!result.success) {
    console.log(`Failed to upload ${video.file_path}: ${result.error}`);
    continue;
  }

  console.log(`Uploaded with Stage ${result.stage}`);

  // Submit form
  await interact({ action: 'click', selector: 'button[type=submit]' });

  // Wait for redirect
  await wait(5000);
}
```

### Generic Platform Support

This design is **platform-agnostic**. Works with:

- **HTML form-based:** YouTube, Vimeo, Dailymotion, Rumble, custom CMS
- **API endpoints:** Platforms with file upload endpoints
- **Enterprise systems:** CSRF-protected, multi-field forms
- **Any file type:** Videos, documents, images, archives

Escalation strategy automatically detects the right approach per platform.

---

## Escalation State Machine

```
IDLE
  ↓ [User calls interact(action: "upload", ...)]

WAITING_FOR_USER_INPUT
  ├─ [User clicks file input within 5s timeout] → STAGE_2_FILE_DIALOG
  └─ [5s elapsed, no user action] → STAGE_1_DRAGDROP

STAGE_1_DRAGDROP
  ├─ [Success: File injected] → COMPLETE ✅
  ├─ [Failed: Platform rejected] → STAGE_2_FILE_DIALOG
  └─ [Error: Exception] → STAGE_3_FORM_INTERCEPTION

STAGE_2_FILE_DIALOG
  ├─ [Success: File picker completed] → COMPLETE ✅
  ├─ [Failed: Dialog not intercepted] → STAGE_3_FORM_INTERCEPTION
  └─ [Error: No user interaction] → STAGE_3_FORM_INTERCEPTION

STAGE_3_FORM_INTERCEPTION
  ├─ [Success: POST accepted] → COMPLETE ✅
  ├─ [Failed: CSRF mismatch/expired] → STAGE_4_OS_AUTOMATION
  ├─ [Failed: User not logged in] → ERROR (ask user to login)
  └─ [Error: Form not found] → STAGE_4_OS_AUTOMATION

STAGE_4_OS_AUTOMATION
  ├─ [Success: File dialog appeared, file injected] → COMPLETE ✅
  ├─ [Retry 1: Dialog not found] → wait 1s, retry
  ├─ [Retry 2: Dialog not found] → wait 2s, retry
  ├─ [Retry 3: Dialog not found] → STAGE_4_FALLBACK
  └─ [Error: OS permissions denied] → ERROR (user must grant access)

STAGE_4_FALLBACK (manual upload)
  ├─ [User manually uploads] → COMPLETE ✅
  ├─ [Timeout 30s] → ERROR (all stages exhausted)
  └─ [User cancels] → ERROR (upload cancelled)

COMPLETE
  ↓ [Return success + stage used + duration]

ERROR
  ↓ [Return error + stage reached + recovery suggestions]
```

---

## Error Escalation Rules

### Auto-Retry
- **Stage 1-3:** Auto-retry once before escalating
- **Stage 4:** Retry 3 times with exponential backoff (1s, 2s, 4s)

### User Interaction Required
- **If user said "don't bother me" or "force it":** Skip confirmation, escalate automatically
- **If user said nothing (default):** Ask before Stage 4 OS automation

### Failure Terminal States
1. **File not found/unreadable** → Return error immediately, don't escalate
2. **User not logged into platform** → Ask user to login, retry Stage 3
3. **OS permissions denied** → Return error, user must grant accessibility/input simulation permission
4. **All stages exhausted + manual fallback timeout** → Return error with recovery suggestions

---

## Edge Cases & Recovery

| Scenario | Detection | Recovery |
|----------|-----------|----------|
| **File not found** | Stage 1: `ENOENT` error | Return 404, suggest checking path |
| **File too large (>15GB)** | Stage 1: File size check | Return 413, suggest chunking or alternative |
| **Permission denied on file** | Stage 1: `EACCES` error | Return 403, ask to check file permissions |
| **Drag-drop rejected by platform** | Stage 1: Event silently ignored | Escalate to Stage 2 (expected behavior) |
| **CSRF token expired/mismatch** | Stage 3: 403/422 response | Escalate to Stage 4 (token is stale) |
| **User not logged in** | Stage 3: 401 response | Ask user to login, retry Stage 3 |
| **File dialog doesn't appear** | Stage 4: Dialog not found after 3s | Retry with backoff, then give up |
| **OS automation permission denied** | Stage 4: `permission_denied` error | Return error, user must grant permission |
| **Form structure changed (fields moved)** | Stage 3: Form validation fails | Document limitation, suggest manual upload |
| **Network timeout (large file)** | Go server: Connection timeout | Return error, user can retry with streaming |
| **Session cookie expired mid-upload** | Stage 3: 401 during POST | Return error, user must re-login |

---

## Deployment & Testing

### Go Server Implementation (All Stages)

#### New endpoints:
- `POST /api/file/read` — Stage 1: Load file into memory (base64 for small files)
- `POST /api/file/dialog/inject` — Stage 2: Inject path into native file dialog
- `POST /api/form/submit` — Stage 3: Extract form metadata and submit with file
- `POST /api/os-automation/inject` — Stage 4: Simulate OS-level file picker interaction

#### Startup flags:
```bash
kaboom-mcp --enable-os-upload-automation --upload-dir=/path/to/uploads
```

#### File size handling:
- < 100MB: Load entirely into memory, base64 encode
- 100MB - 2GB: Load in 100MB chunks, stream to extension
- > 2GB: Direct streaming from Go server to platform (no base64)

### Extension Implementation

#### New `interact` action:
```typescript
interface UploadRequest {
  action: "upload"
  selector?: string                 // CSS selector for file input
  apiEndpoint?: string             // Alternative: direct API endpoint
  file_path: string                // Absolute path to file
  submit?: boolean                 // Auto-submit form (default: false)
  escalation_timeout_ms?: number   // Time before auto-escalation (default: 5000)
}
```

#### State tracking:
- Track which stage was used
- Log escalation reasons
- Report progress for large files
- Handle concurrent uploads (queue if needed)

### UAT Scenarios

#### Stage 1 (Drag-Drop):
1. ✅ Small file (50MB) to YouTube-style dropzone
2. ✅ Medium file (500MB) to custom Dropzone.js handler
3. ✅ Verify drag-drop event fired with correct File object

#### Stage 2 (File Dialog):
1. ✅ Simulate user clicking file input
2. ✅ Verify file picker dialog opens
3. ✅ Verify correct file injected via path

#### Stage 3 (Form Interception):
1. ✅ Rumble form: Extract fields + CSRF + cookies
2. ✅ YouTube form: Detect auth requirements
3. ✅ Enterprise form: Handle custom CSRF token names
4. ✅ Large file (2GB): Stream without memory bloat
5. ✅ Verify form submission successful

#### Stage 4 (OS Automation):
1. ✅ macOS: Simulate AppleScript file dialog injection
2. ✅ Windows: Verify UIA finds file dialog
3. ✅ Linux: Test xdotool integration
4. ✅ Retry logic with exponential backoff

#### Error Cases:
1. ✅ File not found → Return 404, helpful error message
2. ✅ Permission denied → Return 403, suggest checking file permissions
3. ✅ CSRF token mismatch → Escalate to Stage 4
4. ✅ Session expired → Ask user to re-login
5. ✅ All stages fail → Return comprehensive error + recovery suggestions

#### Bulk Upload (Rumble Example):
1. ✅ Read CSV with 10 videos + metadata
2. ✅ Fill form fields programmatically
3. ✅ Upload each file with auto-escalation
4. ✅ Wait between uploads to avoid rate limiting
5. ✅ Report success/failure per video

---

## References

- [Rumble Upload Page](https://rumble.com/upload.php)
- [Chrome Extension File Access](https://developer.chrome.com/docs/extensions/reference/api/scripting/)
- [DataTransfer API](https://developer.mozilla.org/en-US/docs/Web/API/DataTransfer)
- [File API](https://developer.mozilla.org/en-US/docs/Web/API/File)
