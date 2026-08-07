---
status: implemented
scope: feature/file-upload/qa
ai-priority: high
tags: [testing, qa, security]
relates-to: [tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: file-upload
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: File Upload (4-Stage Escalation)

> QA plan for the file upload feature. Covers data leak analysis, security hardening verification, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. File upload is HIGH RISK because it reads arbitrary files from disk and can submit them to external servers. The Go server has full filesystem access.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | File read exposes arbitrary files | Verify `/api/file/read` enforces denylist and upload-dir restrictions; Stage 4 requires `--enable-os-upload-automation` | critical |
| DL-2 | Relative path traversal | Verify all 4 endpoints reject relative paths (`../../../etc/passwd`) | critical |
| DL-3 | Base64 data leaked to logs | Verify `DataBase64` response field is NOT written to the server log file | critical |
| DL-4 | Cookies forwarded to wrong host | Verify Stage 3 form submit only sends cookies to the `form_action` URL, not to arbitrary hosts | high |
| DL-5 | CSRF token exposure | Verify CSRF tokens from form interception are not logged or stored beyond the request lifecycle | high |
| DL-6 | Upload endpoints accessible without extension | Verify all 4 upload routes are wrapped with `extensionOnly()` middleware | critical |
| DL-7 | File content in error messages | Verify error responses never include file content, only file metadata (name, size, type) | medium |
| DL-8 | OS automation leaks file path to other apps | Verify AppleScript/PowerShell/xdotool only types in the focused file dialog, not in arbitrary windows | medium |

### Negative Tests (must NOT leak)
- [x] Stage 4 OS automation disabled by default — returns 403 without `--enable-os-upload-automation`
- [x] Relative paths rejected at all 4 stages
- [x] All upload routes behind `extensionOnly()` middleware — no `X-Kaboom-Client` header → 403
- [ ] Base64 file data not written to server log file (manual verification needed)
- [ ] Cookies sent only to `form_action` host, not logged

---

## 2. Security Hardening Verification

**Goal:** Verify all injection vectors are mitigated.

| # | Attack Vector | Mitigation | Test Coverage | Status |
|---|--------------|-----------|---------------|--------|
| SEC-1 | Content-Disposition header injection via filename | `sanitizeForContentDisposition()`: strips `"`, `\n`, `\r`, `\x00` | `TestUploadInteg_ContentDisposition_SafeFilename` (6 cases) | PASS |
| SEC-2 | Content-Disposition injection via `file_input_name` | Same sanitizer applied to `req.FileInputName` | `TestUploadInteg_ContentDisposition_InputNameInjection` (4 cases) | PASS |
| SEC-3 | AppleScript injection via file path | `sanitizeForAppleScript()`: escapes `\` and `"` | `TestUploadHandler_AppleScriptSanitization` (4 cases incl. `"; do shell script "rm -rf /"`) | PASS |
| SEC-4 | PowerShell/SendKeys injection via file path | `sanitizeForSendKeys()`: escapes `+^%~(){}` | `TestUploadHandler_SendKeysSanitization` (7 cases) | PASS |
| SEC-5 | Windows double-escape (SendKeys + PS quotes) | Layer 1: `sanitizeForSendKeys()`, Layer 2: `" → \`"` | `TestUploadHandler_WindowsDoubleEscape` (5 cases + invariant) | PASS |
| SEC-6 | OS automation path injection (null, newline, backtick) | `validatePathForOSAutomation()`: rejects `\x00`, `\n`, `\r`, `` ` `` | `TestUploadHandler_PathValidation_RejectsMetachars` (7 cases) | PASS |
| SEC-7 | xdotool flag injection via path starting with `--` | `--` argument terminator before file path | Code review (paths are absolute, start with `/`) | PASS |
| SEC-8 | Data race in form submit goroutine | `writeErrCh` channel (was shared `writeErr` variable) | `TestUploadInteg_ConcurrentFormSubmit` (10 workers + `-race`) | PASS |
| SEC-9 | Request body size overflow | `MaxBytesReader` on all endpoints (1MB or 10MB) | `TestUploadInteg_MaxBytesReader_FileRead`, `_FormSubmit` | PASS |
| SEC-10 | File read on directories | Explicit `info.IsDir()` check | `TestUploadHandler_FileRead_DirectoryRejected` | PASS |

### Defense-in-Depth Layers

1. **Feature gate:** `--enable-os-upload-automation` flag (disabled by default)
2. **Route access:** `extensionOnly()` middleware on all 4 endpoints
3. **Path validation:** Absolute path required + `validatePathForOSAutomation()` for Stage 4
4. **Input sanitization:** Platform-specific sanitizers (AppleScript, SendKeys, Content-Disposition)
5. **Body limits:** `MaxBytesReader` prevents memory exhaustion
6. **File existence check:** `os.Stat` before any processing

---

## 3. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | OS automation disabled error is actionable | Error includes "OS-level upload automation is disabled. Start server with --enable-os-upload-automation flag." | [x] |
| CL-2 | Missing parameters clearly identified | Error says "Missing required parameter: file_path" (not generic "Bad Request") | [x] |
| CL-3 | File not found distinguishable from permission denied | Separate error messages and HTTP status codes (404 vs 403) | [x] |
| CL-4 | Stage number in response | Every `UploadStageResponse` includes `stage` field (1-4) | [x] |
| CL-5 | Progress tier communicated | Response includes `progress_tier` ("simple", "periodic", "detailed") | [x] |
| CL-6 | Correlation ID prefix | `correlation_id` starts with `upload_` so AI can track async operations | [x] |
| CL-7 | HTTP error codes actionable | 401 → "Please log in and retry", 403 → "CSRF token may be expired", 422 → "Check required fields" | [x] |
| CL-8 | Recovery suggestions provided | Failed responses include `suggestions` array with next steps | [x] |

---

## 4. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Medium

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Upload a file | 1 step: `interact({what: "upload", selector: "...", file_path: "..."})` | No — already minimal |
| Upload with form submit | 1 step: same as above with `submit: true` | No — already minimal |
| Enable upload automation | 1 step: add `--enable-os-upload-automation` flag | No — intentional explicit opt-in |
| Diagnose upload failure | 1 step: read `suggestions` array in error response | No — already minimal |

### Default Behavior Verification
- [x] Upload disabled by default (no flag) → error with instructions
- [x] Relative paths rejected → error with "absolute path" message
- [x] Missing parameters → error naming the missing field
- [x] Stage 3 method defaults to POST if omitted

---

## 5. Code-Level Test Coverage

### Test files:
- `cmd/browser-agent/internal/toolinteract/interactupload/upload_test.go` — MCP handler tests (security gating, parameter validation, queue payloads, and response contracts)
- `internal/upload/behavior_test.go` — deterministic base64 boundaries, MIME/progress metadata, multipart form submission, and upstream errors
- `internal/upload/httpapi/handlers_test.go` — HTTP endpoint status, gating, cancellation, and adapter contracts
- `internal/upload/integrationtest/upload_test.go` — Integration tests for concurrency, response contracts, HTTP limits, and upload edge cases
- `internal/upload/integrationtest/security_test.go` — Integration tests for upload security and request-boundary behavior

### Run all upload tests:
```bash
go test ./internal/upload/... -run "TestUpload" -v -count=1 -race
```

| Category | Test Count | Status |
|----------|-----------|--------|
| Security gating (`--enable-os-upload-automation`, Stage 4 only) | 2 | PASS |
| Parameter validation (all stages) | 12 | PASS |
| File read (base64, MIME, permissions, large files) | 14 | PASS |
| Dialog injection (valid/invalid/missing PID) | 5 | PASS |
| Form submission (httptest roundtrip, HTTP errors) | 13 | PASS |
| OS automation (valid, missing PID, path validation) | 6 | PASS |
| Progress tiers (boundaries, zero bytes) | 6 | PASS |
| HTTP layer (status codes, MaxBytesReader, disabled) | 12 | PASS |
| Injection prevention (AppleScript, SendKeys, Content-Disposition, path, Windows double-escape) | 26 | PASS |
| Concurrency (race detector, correlation ID uniqueness) | 2 | PASS |
| Integration (middleware, pending query, truncate, permission) | 8 | PASS |
| **Total** | **106+** | **ALL PASS** |

All tests run with `-race` flag enabled.

---

## 6. UAT Scenarios

### Stage 1: File Read
| # | Scenario | Expected | Status |
|---|----------|----------|--------|
| UAT-1 | Read small text file (<100MB) | Returns success with base64 data | [x] automated |
| UAT-2 | Read binary file | Base64 round-trips perfectly | [x] automated |
| UAT-3 | Read file >100MB | Returns metadata only, no base64 | [x] automated |
| UAT-4 | Read file at exactly 100MB boundary | Includes base64 (<=) | [x] automated |
| UAT-5 | Read nonexistent file | Returns 404 with "File not found" | [x] automated |
| UAT-6 | Read directory path | Returns error with "directory" message | [x] automated |
| UAT-7 | Read file with no permissions | Returns error with "permission denied" | [x] automated |
| UAT-8 | MIME detection for 22+ file types | Correct MIME type returned | [x] automated |
| UAT-9 | MIME case insensitive (FILE.MP4) | Returns "video/mp4" | [x] automated |

### Stage 2: File Dialog Injection
| # | Scenario | Expected | Status |
|---|----------|----------|--------|
| UAT-10 | Valid request with file + browser PID | Returns success with "queued" status | [x] automated |
| UAT-11 | File not found | Returns error | [x] automated |
| UAT-12 | Missing browser PID | Returns error mentioning PID | [x] automated |
| UAT-13 | Response shape verification | Includes stage=2, file_name, file_size_bytes | [x] automated |

### Stage 3: Form Submission
| # | Scenario | Expected | Status |
|---|----------|----------|--------|
| UAT-14 | Full form submit with httptest server | File data, CSRF token, cookies, custom fields all received correctly | [x] automated |
| UAT-15 | Platform returns 401 | Error mentions "log in" | [x] automated |
| UAT-16 | Platform returns 403 | Error mentions "CSRF token" | [x] automated |
| UAT-17 | Platform returns 422 | Error mentions "form validation" | [x] automated |
| UAT-18 | Missing form_action | Returns validation error | [x] automated |
| UAT-19 | Empty method defaults to POST | No validation error, stage=3 | [x] automated |
| UAT-20 | 10 concurrent form submits | All succeed, no race conditions | [x] automated |
| UAT-21 | File deleted during upload | No panic or deadlock | [x] automated |
| UAT-22 | Permission denied on file open | Returns error mentioning permission | [x] automated |

### Stage 4: OS Automation
| # | Scenario | Expected | Status |
|---|----------|----------|--------|
| UAT-23 | Valid request on current OS | Returns stage=4 (success or env-specific failure, no crash) | [x] automated |
| UAT-24 | File not found | Returns error | [x] automated |
| UAT-25 | Missing browser PID | Returns error mentioning PID | [x] automated |
| UAT-26 | Path with null byte | Rejected by validatePathForOSAutomation | [x] automated |
| UAT-27 | Path with newline | Rejected | [x] automated |
| UAT-28 | Path with backtick | Rejected | [x] automated |
| UAT-29 | Valid paths with spaces, unicode, dashes | Pass validation | [x] automated |

### Cross-Cutting
| # | Scenario | Expected | Status |
|---|----------|----------|--------|
| UAT-30 | MCP happy path response contract | Has status, correlation_id, file_name, file_size, mime_type, progress_tier, message | [x] automated |
| UAT-31 | Correlation IDs unique across 20 concurrent uploads | No duplicates | [x] automated |
| UAT-32 | Pending query payload completeness | Contains action, selector, file metadata, submit, escalation_timeout_ms, progress_tier | [x] automated |
| UAT-33 | All endpoints return 403 without extension header | extensionOnly middleware blocks non-extension callers | [x] automated |
| UAT-34 | Stage 4 endpoint returns 403 when OS automation disabled | --enable-os-upload-automation not set | [x] automated |
| UAT-35 | Oversized request body rejected | MaxBytesReader limits enforced | [x] automated |
| UAT-36 | Relative path rejected at all stages | Returns "absolute path" error | [x] automated |
| UAT-37 | Invalid JSON returns error | No panic, returns isError:true | [x] automated |
| UAT-38 | No-panic sweep (5 malformed variants) | All return valid responses, none panic | [x] automated |

### Manual UAT (requires browser + extension)
| # | Scenario | Expected | Status |
|---|----------|----------|--------|
| UAT-M1 | Real file upload to test server via Stage 1 (drag-drop) | File appears in dropzone | [ ] manual |
| UAT-M2 | Real file upload via Stage 3 (form submit) to httpbin | multipart data received | [ ] manual |
| UAT-M3 | macOS: Stage 4 AppleScript types path in Finder dialog | Path entered correctly | [ ] manual |
| UAT-M4 | Bulk upload (3 files sequential) | All complete, no resource leak | [ ] manual |

---

## 7. Regression Risks

| Risk | Mitigation |
|------|-----------|
| New sanitizer breaks valid filenames with special chars | Tests cover spaces, unicode, dashes, underscores |
| `extensionOnly()` removed during refactor | `TestUploadInteg_ExtensionOnlyMiddleware` catches this |
| `MaxBytesReader` limits changed | `TestUploadInteg_MaxBytesReader_*` catches this |
| `validatePathForOSAutomation` bypassed | Test covers null/newline/backtick rejection |
| Race condition reintroduced in form submit | `-race` flag on `TestUploadInteg_ConcurrentFormSubmit` |
