---
feature: Draw Mode
version: v0.7-rev1
doc_type: qa-plan
feature_id: draw-mode
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Draw Mode -- Comprehensive QA Plan

## Testing Strategy Overview

### Test Pyramid

| Layer | Count | Focus |
|-------|-------|-------|
| Unit (Extension) | ~60 tests | Overlay, drawing, DOM capture, annotation CRUD, persistence, export |
| Unit (Go) | ~30 tests | Interact handler, analyze handler, annotation store, HTTP endpoints |
| Integration | ~15 tests | MCP round-trip, blocking analyze, screenshot capture, detail drill-down |
| Edge Cases | ~20 tests | Navigation, resize, empty sessions, TTL expiry, concurrent activation |
| Manual | 7 scenarios | End-to-end verification with real browser |

### Test Locations

- **Extension:** `tests/extension/draw-mode/draw-mode.test.js`, `tests/extension/draw-mode-export.test.js`
- **Go Server:** `cmd/browser-agent/draw_mode_test.go`, `cmd/browser-agent/annotation_store_test.go`
- **Integration:** `tests/integration/draw_mode_test.go`

---

## Unit Tests: Extension -- Draw Mode Overlay

**Location:** `tests/extension/draw-mode/draw-mode.test.js`

### 1. Overlay Lifecycle

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-EXT-001 | Activate draw mode creates canvas overlay | Canvas element appended to document with full viewport dimensions |
| DM-EXT-002 | Overlay has correct z-index | z-index is maximum (2147483647) to sit above all page content |
| DM-EXT-003 | Overlay is transparent (click-through until mousedown) | Canvas background is transparent, cursor changes to crosshair |
| DM-EXT-004 | Deactivate draw mode removes canvas overlay | Canvas element removed from document |
| DM-EXT-005 | Deactivate cleans up all event listeners | No mousedown/mousemove/mouseup/keydown handlers remain |
| DM-EXT-006 | Multiple activate calls are idempotent | Only one canvas overlay exists after two activate calls |

### 2. Rectangle Drawing

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-EXT-010 | Mousedown + mousemove + mouseup draws rectangle | Rectangle rendered on canvas with red dashed border |
| DM-EXT-011 | Rectangle coordinates are viewport-relative | rect.x, rect.y match mouse event clientX, clientY |
| DM-EXT-012 | Rectangle width/height calculated correctly | width = mouseup.x - mousedown.x, height = mouseup.y - mousedown.y |
| DM-EXT-013 | Rectangle < 5px in width ignored | No annotation created, no text input shown |
| DM-EXT-014 | Rectangle < 5px in height ignored | No annotation created, no text input shown |
| DM-EXT-015 | Rectangle exactly 5px accepted | Annotation created, text input shown |
| DM-EXT-016 | Drawing from bottom-right to top-left normalizes coordinates | rect.x and rect.y are the top-left corner regardless of draw direction |
| DM-EXT-017 | Multiple rectangles rendered simultaneously | All previous rectangles remain visible when drawing new one |
| DM-EXT-018 | Rectangle rendered during drag (live preview) | Rectangle visible as user drags mouse |

### 3. Text Input

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-EXT-020 | Text input appears after rectangle drawn | Input element auto-focused near rectangle |
| DM-EXT-021 | Enter key confirms text | Text saved to annotation, input removed, state returns to DRAWING |
| DM-EXT-022 | Blur with non-empty text auto-confirms | Text saved to annotation, same as Enter |
| DM-EXT-023 | Blur with empty text removes annotation | Rectangle removed from canvas, annotation deleted |
| DM-EXT-024 | Text input does not capture ESC key | ESC during text input exits draw mode (not just text input) |
| DM-EXT-025 | Text input positioned near rectangle | Input appears adjacent to or overlapping the drawn rectangle |
| DM-EXT-026 | Long text truncated in display | Text longer than 200 chars displayed with ellipsis on canvas |

### 4. DOM Element Capture

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-EXT-030 | Element under rectangle center identified | document.elementFromPoint called with center coordinates |
| DM-EXT-031 | Element summary includes tag name | element_summary starts with tag name (e.g., "button") |
| DM-EXT-032 | Element summary includes primary class | First class from classList included (e.g., "button.btn-primary") |
| DM-EXT-033 | Element summary includes text content snippet | First 50 chars of textContent included in quotes |
| DM-EXT-034 | Element summary for element with no text | Tag and class only (e.g., "div.hero-image") |
| DM-EXT-035 | Element under overlay (canvas) skipped | elementFromPoint temporarily hides overlay to hit page elements |
| DM-EXT-036 | Unique CSS selector generated | Selector is specific enough to match exactly one element |
| DM-EXT-037 | Selector uses ID if element has one | Selector is `#submit-btn` for element with id="submit-btn" |
| DM-EXT-038 | Selector falls back to tag.class path | If no ID, generates path like `form > div.actions > button.primary` |

### 5. Annotation CRUD

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-EXT-040 | Annotation created with unique ID | ID format: `ann_{timestamp}_{random}` |
| DM-EXT-041 | Annotation includes rect, text, timestamp, page_url | All fields populated correctly |
| DM-EXT-042 | Annotation includes element_summary | Summary from DOM capture present |
| DM-EXT-043 | Annotation includes correlation_id | Format: `ann_detail_{random}` |
| DM-EXT-044 | Creating annotation with empty text after blur removes it | Annotation array does not include the removed annotation |
| DM-EXT-045 | Multiple annotations stored in order | Annotations array ordered by creation time |

### 6. Persistence

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-EXT-050 | Annotations saved to chrome.storage.session on add | chrome.storage.session.set called with annotations |
| DM-EXT-051 | Annotations saved on text confirm | Storage updated after Enter/blur |
| DM-EXT-052 | Annotations saved on annotation removal | Storage updated after empty-text deletion |
| DM-EXT-053 | Storage key is "kaboom_draw_annotations" | Correct key used for session storage |
| DM-EXT-054 | Storage includes metadata (active, tab_id, page_url) | All metadata fields present |
| DM-EXT-055 | New session overwrites previous session data | Only latest session's annotations in storage |

### 7. Keyboard Shortcuts

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-EXT-060 | ESC key deactivates draw mode | Overlay removed, results packaged |
| DM-EXT-061 | Cmd+Shift+D activates draw mode (macOS) | Overlay created, state set to DRAWING |
| DM-EXT-062 | Ctrl+Shift+D activates draw mode (Windows/Linux) | Overlay created, state set to DRAWING |
| DM-EXT-063 | Cmd+Shift+D while active deactivates | Toggle behavior, same as ESC |
| DM-EXT-064 | Other keyboard shortcuts still work during draw mode | Tab, Cmd+C, etc. not intercepted |

---

## Unit Tests: Extension -- Screenshot Export

**Location:** `tests/extension/draw-mode-export.test.js`

### 1. Screenshot Capture

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-EXP-001 | Screenshot captured on deactivation | chrome.tabs.captureVisibleTab called |
| DM-EXP-002 | Screenshot is PNG format | Format parameter is "png" |
| DM-EXP-003 | Screenshot includes annotation overlays | Canvas overlay still visible during capture |
| DM-EXP-004 | Screenshot encoded as base64 | Valid base64 string in result |
| DM-EXP-005 | Screenshot capture failure handled gracefully | Error logged, results still sent without screenshot |

### 2. Result Packaging

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-EXP-010 | Result includes all annotations | annotations array matches stored annotations |
| DM-EXP-011 | Result includes screenshot base64 | screenshot_base64 field populated |
| DM-EXP-012 | Result includes page_url | Current page URL included |
| DM-EXP-013 | Result includes correlation_id | Matches the PendingQuery correlation_id (if LLM-initiated) |
| DM-EXP-014 | Result sent to background via message | DRAW_MODE_COMPLETED message dispatched |
| DM-EXP-015 | Result includes count | count field matches annotations.length |

---

## Unit Tests: Go Server

**Location:** `cmd/browser-agent/draw_mode_test.go`

### 1. Interact Handler (draw_mode_start)

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-GO-001 | draw_mode_start creates PendingQuery | Query appears in GetPendingQueries() with type "draw_mode" |
| DM-GO-002 | PendingQuery has correct correlation_id | Unique ID in `dm_` prefix format |
| DM-GO-003 | Response includes status "pending" | JSON response has `status: "pending"` |
| DM-GO-004 | Response includes correlation_id | correlation_id matches PendingQuery ID |
| DM-GO-005 | draw_mode_start while active returns already_active | `{status: "already_active", annotation_count: N}` |
| DM-GO-006 | draw_mode_start requires AI Web Pilot enabled | Returns error if pilot toggle is off |

### 2. Analyze Handler (annotations)

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-GO-010 | analyze({what: "annotations"}) returns stored annotations | annotations array returned |
| DM-GO-011 | analyze({what: "annotations"}) with no session returns empty | count: 0, hint message included |
| DM-GO-012 | analyze({what: "annotations", wait: true}) blocks until results | Response received only after draw-mode-result POST |
| DM-GO-013 | Blocking analyze times out after 5 minutes | Returns timeout error after 300s |
| DM-GO-014 | Blocking analyze resolves when results posted | Response includes annotations and screenshot_path |
| DM-GO-015 | Screenshot saved to temp directory | File exists at returned path |
| DM-GO-016 | Screenshot path uses correct format | `/tmp/kaboom-draw-mode-{timestamp}.png` |

### 3. Analyze Handler (annotation_detail)

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-GO-020 | annotation_detail returns full detail for valid correlation_id | computed_styles, selector, etc. present |
| DM-GO-021 | annotation_detail with expired correlation_id returns error | error code: "correlation_expired" |
| DM-GO-022 | annotation_detail with unknown correlation_id returns error | error code: "correlation_expired" |
| DM-GO-023 | annotation_detail creates PendingQuery for extension | Query type: "annotation_detail" |

### 4. HTTP Endpoints

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-GO-030 | POST /draw-mode-result accepts valid payload | 200 OK, annotations stored |
| DM-GO-031 | POST /draw-mode-result with missing correlation_id | 400 Bad Request |
| DM-GO-032 | POST /draw-mode-result resolves blocked analyze call | Blocked goroutine receives data |
| DM-GO-033 | POST /draw-mode-result with screenshot | Screenshot decoded and saved to disk |
| DM-GO-034 | POST /annotation-detail-result accepts valid payload | 200 OK, detail stored |
| DM-GO-035 | POST /annotation-detail-result with missing correlation_id | 400 Bad Request |
| DM-GO-036 | POST /draw-mode-result body > 10MB rejected | 413 Payload Too Large |

---

## Unit Tests: Go Server -- Annotation Store

**Location:** `cmd/browser-agent/annotation_store_test.go`

### 1. Storage and Retrieval

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-STORE-001 | Store annotations for a session | GetAnnotations returns stored data |
| DM-STORE-002 | Store overwrites previous session | Only latest session data returned |
| DM-STORE-003 | Store detail for correlation_id | GetAnnotationDetail returns detail |
| DM-STORE-004 | Get detail for unknown correlation_id | Returns nil/not-found |

### 2. TTL Expiry

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-STORE-010 | Annotation detail expires after 10 minutes | GetAnnotationDetail returns nil after TTL |
| DM-STORE-011 | Annotation session does not expire (session-scoped) | GetAnnotations returns data regardless of time |
| DM-STORE-012 | Cleanup removes expired detail entries | Memory freed after TTL sweep |

### 3. Concurrency

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-STORE-020 | Concurrent store and get operations | No race conditions (mutex protected) |
| DM-STORE-021 | Concurrent detail requests | All return correct data |

---

## Integration Tests

**Location:** `tests/integration/draw_mode_test.go`

### End-to-End Flows

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-INT-001 | LLM activates draw mode -> user draws -> LLM reads annotations | Full round-trip with annotations returned |
| DM-INT-002 | Blocking analyze resolves on draw completion | analyze({wait: true}) returns after user presses ESC |
| DM-INT-003 | Screenshot included in results | screenshot_path points to valid PNG file |
| DM-INT-004 | Annotation detail drill-down | annotation_detail returns computed_styles for valid correlation_id |
| DM-INT-005 | Detail after TTL expiry returns error | correlation_expired error after 10 min |
| DM-INT-006 | User-initiated draw mode -> LLM reads after | analyze({what: "annotations"}) returns user annotations |
| DM-INT-007 | Draw mode with zero annotations | count: 0, screenshot still captured |
| DM-INT-008 | Multiple annotations in one session | All annotations returned in order |

### Error Propagation

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-INT-010 | AI Web Pilot disabled | draw_mode_start returns error |
| DM-INT-011 | Extension not connected | draw_mode_start times out with error |
| DM-INT-012 | Screenshot capture fails | Results returned without screenshot, warning included |
| DM-INT-013 | Page navigates during draw mode | Partial results with warning: "page_navigated" |

### Alert Flow

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-INT-020 | draw_mode_start while already active | Returns already_active with annotation count |
| DM-INT-021 | analyze without prior draw session | Returns empty array with hint |

---

## Edge Case Tests

### 1. Navigation and Lifecycle

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-EDGE-001 | Page navigates during draw mode (full navigation) | Annotations sent to server via beforeunload handler |
| DM-EDGE-002 | SPA route change during draw mode | Draw mode remains active (overlay persists) |
| DM-EDGE-003 | Tab closed during draw mode | Background detects tab removal, cleans up PendingQuery |
| DM-EDGE-004 | Browser closed during draw mode | No crash; data in storage.session is lost (by design) |

### 2. Resize and Viewport

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-EDGE-010 | Window resize during draw mode | Canvas resized, annotations re-rendered |
| DM-EDGE-011 | Zoom level changes during draw mode | Overlay scales with page zoom |
| DM-EDGE-012 | Scroll during draw mode | Canvas covers visible viewport; annotations stay at viewport-relative positions |

### 3. Timing and Concurrency

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-EDGE-020 | draw_mode_start called twice in rapid succession | Second call returns already_active |
| DM-EDGE-021 | analyze({wait: true}) called before draw_mode_start | Blocks, resolves when draw mode completes (or timeout) |
| DM-EDGE-022 | Two analyze({wait: true}) calls simultaneously | Both resolve with same data |
| DM-EDGE-023 | draw_mode_start then immediate ESC (< 1 second) | Valid result with 0 annotations |

### 4. Content Edge Cases

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-EDGE-030 | Draw on page with iframes | Overlay covers main document only; iframe content not annotatable |
| DM-EDGE-031 | Draw on page with shadow DOM | elementFromPoint returns shadow host, not shadow children |
| DM-EDGE-032 | Draw on about:blank | Draw mode activates, no elements to capture |
| DM-EDGE-033 | Draw on chrome:// page | Content script cannot inject; returns error |
| DM-EDGE-034 | Draw on page with overflow scroll | Annotations captured at current scroll position |

### 5. Data Limits

| Test ID | Test Case | Expected Result |
|---------|-----------|-----------------|
| DM-EDGE-040 | 50 annotations in one session | All 50 stored and returned |
| DM-EDGE-041 | Annotation text with 1000 characters | Full text stored, display truncated |
| DM-EDGE-042 | Screenshot > 5MB | Compression applied, or screenshot omitted with warning |
| DM-EDGE-043 | Element with no classes and no ID | Selector generated via tag + nth-child path |

---

## Manual Test Checklist

### Test 1: Popup Toggle Activation

#### Steps:
1. Open any web page
2. Click the Kaboom extension icon to open popup
3. Click the "Draw Mode" toggle
4. Verify: red dashed crosshair cursor appears, overlay covers page
5. Draw a rectangle by clicking and dragging
6. Verify: rectangle appears with red dashed border
7. Type annotation text "make this bigger"
8. Press Enter
9. Verify: text appears near rectangle
10. Press ESC
11. Verify: overlay removed, page is interactive again

**Expected:** Draw mode activates and deactivates cleanly via popup.

### Test 2: Keyboard Shortcut Toggle

#### Steps:
1. Open any web page
2. Press Cmd+Shift+D (macOS) or Ctrl+Shift+D (Windows/Linux)
3. Verify: draw mode activates (crosshair cursor)
4. Draw a rectangle and type annotation
5. Press ESC
6. Verify: draw mode deactivates

**Expected:** Keyboard shortcut toggles draw mode correctly.

### Test 3: MCP Round-Trip

#### Steps:
1. Start Kaboom MCP server
2. Connect an MCP client (e.g., Claude Code)
3. Call `interact({what: "draw_mode_start"})`
4. Verify: extension activates draw mode on active tab
5. Draw 2-3 rectangles with annotations in the browser
6. Press ESC
7. Call `analyze({what: "annotations"})`
8. Verify: response includes all annotations with element_summary and screenshot_path

**Expected:** Full LLM workflow produces structured annotation data.

### Test 4: Detail Drill-Down

#### Steps:
1. Complete Test 3 (have annotations from a session)
2. From the analyze response, copy a correlation_id
3. Call `analyze({what: "annotation_detail", correlation_id: "ann_detail_..."})`
4. Verify: response includes selector, computed_styles, parent_selector, classes
5. Verify: computed_styles has actual CSS values (e.g., background-color, font-size)

**Expected:** Annotation detail returns actionable DOM information.

### Test 5: TTL Expiry

#### Steps:
1. Complete Test 3 (have annotations from a session)
2. Wait 10+ minutes
3. Call `analyze({what: "annotation_detail", correlation_id: "ann_detail_..."})`
4. Verify: response is an error with code "correlation_expired"

**Expected:** Detail data expires after 10-minute TTL.

### Test 6: Screenshot Verification

#### Steps:
1. Complete Test 3 (have annotations from a session)
2. Check the screenshot_path from the analyze response
3. Open the PNG file
4. Verify: screenshot shows the web page with annotation rectangles and text visible

**Expected:** Annotated screenshot is a valid PNG with visible annotations.

### Test 7: Edge Cases

#### Steps:
1. Activate draw mode via Cmd+Shift+D
2. Resize the browser window
3. Verify: overlay resizes, existing annotations still visible
4. Press ESC with no annotations drawn
5. Verify: results returned with count: 0
6. Activate draw mode again
7. Navigate to a different page (type new URL)
8. Verify: annotations from previous page are sent to server

**Expected:** Edge cases handled gracefully without errors.

---

## Regression Testing Protocol

After any changes to draw mode:

### 1. Quick Smoke Test (2 min)

```bash
node --test tests/extension/draw-mode/draw-mode.test.js
node --test tests/extension/draw-mode-export.test.js
go test -short ./cmd/browser-agent/... -run DrawMode
go test -short ./cmd/browser-agent/... -run AnnotationStore
```

### 2. Full Test Suite (8 min)

```bash
make test
node --test tests/extension/*.test.js
```

### 3. Integration Verification (5 min)

- Start server: `./dist/kaboom --port 7890`
- Open test page
- Call `interact({what: "draw_mode_start"})` via MCP client
- Draw 2 annotations, press ESC
- Call `analyze({what: "annotations"})` and verify response
- Call `analyze({what: "annotation_detail"})` and verify computed styles

### 4. Performance Regression Check

- Measure overlay activation time (target: < 50ms)
- Measure DOM capture per rectangle (target: < 100ms)
- Measure screenshot capture (target: < 500ms)
- Compare against previous baselines

---

## Browser Compatibility

| Test ID | Browser | Expected Result |
|---------|---------|-----------------|
| DM-COMPAT-001 | Chrome (latest) | Full functionality |
| DM-COMPAT-002 | Chrome (latest - 1) | Full functionality |
| DM-COMPAT-003 | Edge (Chromium) | Full functionality |
| DM-COMPAT-004 | Brave | Full functionality |
