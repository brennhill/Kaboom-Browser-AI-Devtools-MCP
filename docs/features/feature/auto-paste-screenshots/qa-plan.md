---
status: proposed
scope: feature/auto-paste-screenshots/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-auto-paste-screenshots
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Auto-Paste Screenshots to IDE

> QA plan for the Auto-Paste Screenshots to IDE feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

**Note:** No tech-spec.md is available for this feature. This QA plan is based solely on the product-spec.md.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Screenshots capture visible passwords, credit cards, PII on screen | Verify screenshots only flow through localhost MCP response pipeline. Confirm no external transmission of image data. | critical |
| DL-2 | Screenshot mode "on" sends screenshots with every observe response, including when viewing sensitive pages | Verify sensitive content warning is displayed when screenshot_mode is first set to "on" or "errors_only". Confirm user is informed about potential exposure. | critical |
| DL-3 | Base64 image data persisted to server logs | Verify server does not log base64 screenshot payload. Only metadata (size, capture timing) should be logged. | high |
| DL-4 | Screenshot data remains in server memory after response | Verify base64 string is freed after MCP response serialization (< 2MB transient). No persistent in-memory cache. | medium |
| DL-5 | Screenshots saved to disk by this feature | Verify this feature does NOT save screenshots to disk. Existing disk-save via `/screenshots` is separate and independent. | high |
| DL-6 | Screenshot mode setting persists across server restarts | Verify `screenshot_mode` resets to `off` on server restart (session-scoped). A restart must not continue sending screenshots from a previous session. | high |
| DL-7 | MCP response intercepted in transit | Verify MCP communication is localhost-only (stdio or localhost HTTP). No network transmission of screenshot data. | critical |
| DL-8 | Concurrent requests cause screenshot data to leak to wrong MCP response | Verify screenshot capture is correctly associated with the requesting MCP call. No cross-contamination between concurrent requests. | high |

### Negative Tests (must NOT leak)
- [ ] Base64 image data must NOT appear in server log files
- [ ] Screenshot data must NOT be saved to disk by this feature (separate from existing `/screenshots` disk-save)
- [ ] `screenshot_mode` must NOT persist across server restarts -- must reset to `off`
- [ ] No external HTTP request must be made containing screenshot data
- [ ] Screenshot must NOT be appended to responses from other MCP sessions (session isolation)

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Image block position in content array is predictable | Verify image block is always the LAST content block in the array, after all text blocks. | [ ] |
| CL-2 | Screenshot unavailability is explained, not silent | When capture fails, verify a text block like `[Screenshot unavailable: reason]` replaces the image block. Never silently omit. | [ ] |
| CL-3 | Configure response confirms the setting change | After `configure({action: "capture", settings: {screenshot_mode: "on"}})`, verify response text confirms the new setting value. | [ ] |
| CL-4 | Sensitive content warning on first enable | When screenshot_mode is set to "on" or "errors_only" for the first time, verify response includes privacy warning about screenshots potentially containing sensitive content. | [ ] |
| CL-5 | screenshot_mode values are clearly documented in error messages | If an invalid screenshot_mode value is provided, verify error lists valid values ("off", "on", "errors_only"). | [ ] |
| CL-6 | Rate limit explanation is actionable | When screenshot is rate-limited, verify the text block includes specific info (e.g., cooldown remaining, session count). | [ ] |
| CL-7 | Image MIME type is correct | Verify `mimeType` field is `"image/jpeg"` (not `"image/jpg"` or other variants). | [ ] |
| CL-8 | Base64 data does not include data URL prefix | Verify `data` field is raw base64 (not `data:image/jpeg;base64,...`). | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM may assume screenshots are always present when mode is "on" -- verify it understands rate limiting and failure messages. Test with unavailability text blocks.
- [ ] LLM may confuse `screenshot_mode: "on"` (MCP response screenshots) with `screenshotOnError` (existing disk-save on error). Verify these are independent settings.
- [ ] LLM may interpret `[Screenshot unavailable: ...]` as an error state rather than graceful degradation. Verify the text data is still complete and valid.
- [ ] LLM may not realize screenshots apply to `generate` responses too when mode is "on". Verify documentation and configure response clarify scope.

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Enable screenshots for all responses | 1 step: `configure({action: "capture", settings: {screenshot_mode: "on"}})` | No -- already minimal |
| Enable screenshots for errors only | 1 step: `configure({action: "capture", settings: {screenshot_mode: "errors_only"}})` | No -- already minimal |
| Disable screenshots | 1 step: `configure({action: "capture", settings: {screenshot_mode: "off"}})` | No -- already minimal |
| Get observe response with screenshot | 1 step: `observe({what: "errors"})` (screenshot auto-appended if mode is on) | No -- zero extra steps once configured |
| Check current screenshot_mode | 1 step: `configure({action: "health"})` or similar status query | Could expose in health response if not already present |

### Default Behavior Verification
- [ ] Feature defaults to `screenshot_mode: "off"` -- no screenshots unless explicitly enabled
- [ ] Enabling screenshot_mode requires only one configure call
- [ ] Once enabled, screenshots are automatically included with no per-request configuration
- [ ] Disabling is equally simple -- one configure call to set "off"

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Parse screenshot_mode "on" | `{settings: {screenshot_mode: "on"}}` | Setting stored as "on" | must |
| UT-2 | Parse screenshot_mode "off" | `{settings: {screenshot_mode: "off"}}` | Setting stored as "off" | must |
| UT-3 | Parse screenshot_mode "errors_only" | `{settings: {screenshot_mode: "errors_only"}}` | Setting stored as "errors_only" | must |
| UT-4 | Reject invalid screenshot_mode | `{settings: {screenshot_mode: "always"}}` | Validation error listing valid values | must |
| UT-5 | Default screenshot_mode is "off" | New server session, no configure call | screenshot_mode = "off" | must |
| UT-6 | MCPImageContentBlock serialization | Struct with Type="image", Data="abc123", MimeType="image/jpeg" | Valid JSON: `{"type":"image","data":"abc123","mimeType":"image/jpeg"}` | must |
| UT-7 | Mixed content array serialization | Content with text block + image block | JSON array with both types correctly serialized | must |
| UT-8 | Screenshot_mode resets on restart | Server restart simulation | screenshot_mode = "off" after restart | must |
| UT-9 | Sensitive content warning on first enable | Set screenshot_mode from "off" to "on" | Response includes privacy warning text | should |
| UT-10 | Screenshot mode "errors_only" filters correctly | observe({what: "page"}) with mode "errors_only" | No screenshot appended to page response | must |
| UT-11 | Screenshot mode "errors_only" includes on errors | observe({what: "errors"}) with mode "errors_only" | Screenshot appended to errors response | must |
| UT-12 | Payload size warning at 500KB | Screenshot base64 exceeding 500KB | Server logs warning | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | End-to-end: enable mode then observe with screenshot | Go server + Extension + captureVisibleTab | observe response contains text + image content blocks | must |
| IT-2 | Screenshot capture via async command pipeline | Go server pending queries + Extension screenshot handler | Screenshot captured on-demand and returned via correlation ID | must |
| IT-3 | Rate limiting: rapid observe calls with screenshots | Go server + Extension rate limiter | First gets screenshot, subsequent within 5s get unavailability text | must |
| IT-4 | Extension disconnected with screenshot_mode on | Go server + timeout handler | Response contains text data + `[Screenshot unavailable: extension not connected]` text block | must |
| IT-5 | Tab closed during screenshot capture | Go server + Extension tab tracking | Response contains text data + failure explanation text block | should |
| IT-6 | Concurrent MCP requests both wanting screenshots | Go server concurrency | First gets screenshot, second may be rate-limited. Both get valid text data. | should |
| IT-7 | Configure then observe across multiple modes | Go server configure + observe pipeline | Screenshot appended consistently to all observe modes when "on" | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Screenshot capture latency | captureVisibleTab + async pipeline time | < 500ms | must |
| PT-2 | JPEG payload size at quality 60 | Base64 encoded size for 1920x1080 | < 500KB typical | must |
| PT-3 | Base64 encoding overhead | Encoding time for ~300KB raw JPEG | < 50ms | must |
| PT-4 | Total response time increase | End-to-end response with vs without screenshot | < 1s increase | must |
| PT-5 | Server memory during serialization | Transient memory for base64 handling | < 2MB | must |
| PT-6 | 4K display screenshot size | Base64 size at 3840x2160, quality 60 | Log warning if > 500KB; no hard failure | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | No tracked tab | screenshot_mode on, no tab tracked | `[Screenshot unavailable: no tracked tab]` text block appended | must |
| EC-2 | Extension disconnected | screenshot_mode on, extension offline | `[Screenshot unavailable: extension not connected]` after 2s timeout | must |
| EC-3 | Tab closed mid-capture | Tab closes during captureVisibleTab | Graceful failure, explanation text block, no crash | must |
| EC-4 | Rate-limited (5s cooldown) | Two observe calls within 5 seconds | Second gets `[Screenshot unavailable: rate-limited (5s cooldown)]` | must |
| EC-5 | Session limit reached (10/10) | 11th screenshot request in session | `[Screenshot unavailable: session limit reached (10/10)]` | must |
| EC-6 | Very large viewport (4K) | 3840x2160 display | Screenshot captured, warning logged if > 500KB, no hard cap | should |
| EC-7 | Non-visual observe modes with mode "on" | observe({what: "websocket_status"}) with mode "on" | Screenshot still captured (page state is always relevant context) | should |
| EC-8 | observe errors with mode "errors_only" and no errors | observe({what: "errors"}) returns 0 errors | Screenshot still appended (mode is "errors_only" for the observe type, not conditional on error count) | should |
| EC-9 | Tab minimized | Tab not visible when capture requested | captureVisibleTab captures whatever Chrome provides (may be blank) | should |
| EC-10 | Multiple configure calls toggling mode | Toggle on -> off -> on rapidly | Final state is "on"; each configure call takes effect immediately | must |
| EC-11 | Generate response with screenshot | generate({format: "csp"}) with mode "on" | Screenshot appended to generate response | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web page loaded in tracked tab
- [ ] MCP client that supports image content blocks (Claude Code or Cursor)

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "configure", "arguments": {"action": "capture", "settings": {"screenshot_mode": "on"}}}` | Server logs confirm setting change | Response confirms `screenshot_mode=on`; includes sensitive content warning | [ ] |
| UAT-2 | `{"tool": "observe", "arguments": {"what": "page"}}` | Browser may briefly flash during capture | Response contains text block with page metadata AND image block with base64 JPEG | [ ] |
| UAT-3 | Visually inspect the screenshot in MCP client | Screenshot matches current browser viewport | Image accurately represents the visible browser content | [ ] |
| UAT-4 | `{"tool": "observe", "arguments": {"what": "errors"}}` | No special browser behavior | Response contains error data text block AND screenshot image block | [ ] |
| UAT-5 | Immediately repeat: `{"tool": "observe", "arguments": {"what": "errors"}}` (within 5s) | No capture flash | Response contains error data text block AND `[Screenshot unavailable: rate-limited]` text block | [ ] |
| UAT-6 | `{"tool": "configure", "arguments": {"action": "capture", "settings": {"screenshot_mode": "errors_only"}}}` | Server logs confirm setting change | Response confirms `screenshot_mode=errors_only` | [ ] |
| UAT-7 | `{"tool": "observe", "arguments": {"what": "page"}}` | No screenshot capture | Response contains only page metadata text block, NO image block | [ ] |
| UAT-8 | `{"tool": "observe", "arguments": {"what": "errors"}}` | Screenshot capture occurs | Response contains error text AND image block | [ ] |
| UAT-9 | `{"tool": "configure", "arguments": {"action": "capture", "settings": {"screenshot_mode": "off"}}}` | Server logs confirm setting change | Response confirms `screenshot_mode=off` | [ ] |
| UAT-10 | `{"tool": "observe", "arguments": {"what": "errors"}}` | No screenshot capture | Response contains only error text, NO image block | [ ] |
| UAT-11 | Disconnect extension, then with mode "on": `{"tool": "observe", "arguments": {"what": "page"}}` | Extension icon shows disconnected | Response contains page metadata text AND `[Screenshot unavailable: extension not connected]` text block | [ ] |
| UAT-12 | Restart server, then: `{"tool": "observe", "arguments": {"what": "page"}}` | Fresh server start | No screenshot appended (screenshot_mode reset to "off") | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Screenshot data is localhost-only | Monitor all network traffic during screenshot-enabled observe calls | No outbound requests with image data | [ ] |
| DL-UAT-2 | No disk persistence | Check server directory for new image files after multiple observe calls with mode "on" | No new files created by this feature | [ ] |
| DL-UAT-3 | Server logs are clean | Search server log output for base64 strings | No base64 image data in logs (size warnings OK, raw data not OK) | [ ] |
| DL-UAT-4 | Mode resets on restart | Restart server after setting mode to "on" | Default mode is "off" after restart | [ ] |
| DL-UAT-5 | Sensitive page screenshot warning | Navigate to a login page, enable screenshot_mode | Warning text about sensitive content in first configure response | [ ] |

### Regression Checks
- [ ] Existing `observe` responses unchanged when screenshot_mode is "off"
- [ ] Existing `screenshotOnError` disk-save feature still works independently
- [ ] `configure({action: "capture"})` with other settings (not screenshot_mode) still works
- [ ] MCP clients that do not support image blocks handle responses gracefully
- [ ] Server restart does not crash or error due to screenshot_mode cleanup

---

## Sign-Off

| Area | Tester | Date | Pass/Fail |
|------|--------|------|-----------|
| Data Leak Analysis | | | |
| LLM Clarity | | | |
| Simplicity | | | |
| Code Tests | | | |
| UAT | | | |
| **Overall** | | | |
