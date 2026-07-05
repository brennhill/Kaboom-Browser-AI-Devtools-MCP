---
doc_type: qa_plan
feature_id: browser-push
status: proposed
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Browser Push — QA Plan

## Data Leak Analysis

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Screenshot contains sensitive page data (passwords, PII) | Sampling request doesn't add new exposure — same as existing screenshot capture. Verify no disk persistence. | medium |
| DL-2 | Push events sent to wrong MCP client | Verify push goes to first connected client only. No broadcast. | high |
| DL-3 | Inbox events persist after session end | Verify inbox is cleared on daemon shutdown. No file persistence. | medium |
| DL-4 | Sampling systemPrompt leaks internal instructions | Verify systemPrompt is minimal and contains no secrets. | low |

## LLM Clarity Assessment

| # | Clarity Check | What to Verify |
|---|--------------|----------------|
| CL-1 | Sampling message clearly identifies push source | Message says "The user pushed [annotations/screenshot] from the browser" |
| CL-2 | Inbox response is unambiguous | Events have clear type, timestamp, page_url |
| CL-3 | Piggyback hint is actionable | AI knows to call `observe({what: "inbox"})` |
| CL-4 | No confusion between push and pull annotations | Push delivery doesn't interfere with `analyze({what: "annotations"})` |

## Code-Level Testing

### Unit Tests

| Test | File | What to Verify |
|------|------|----------------|
| Inbox enqueue/drain | `internal/push/inbox_test.go` | FIFO order, drain clears, concurrent safety |
| Inbox overflow | `internal/push/inbox_test.go` | Oldest evicted at cap (50), count correct |
| Router: sampling path | `internal/push/router_test.go` | Sampling called when capability present |
| Router: notification fallback | `internal/push/router_test.go` | Notification sent when no sampling |
| Router: inbox fallback | `internal/push/router_test.go` | Always queued regardless of other paths |
| Sampling message format | `internal/push/sampling_test.go` | Valid JSON-RPC, correct content types |
| Capability detection | `internal/push/router_test.go` | Extracted from initialize handshake |

### Integration Tests

| Test | What to Verify |
|------|----------------|
| `/push/screenshot` endpoint | Returns 200, event queued in inbox |
| `/draw-mode/complete` → push pipeline | Annotations routed through push after storage |
| `observe({what: "inbox"})` | Returns queued events, clears on drain |
| Piggyback on tool response | `_pending_push` present when inbox non-empty, absent when empty |

## UAT Verification

### Sampling Path (requires Claude Code)

1. Start daemon, connect Claude Code
2. Open a web page in Chrome
3. Press `Alt+Shift+D` to start draw mode
4. Draw a rectangle, type feedback, press ESC
5. Verify: Claude Code shows annotations without user typing anything
6. Verify: AI responds with analysis of annotations

### Screenshot Push

1. Open a web page in Chrome
2. Press `Alt+Shift+S`
3. Verify: toast appears asking for optional note
4. Type a note (or wait 3s for auto-send)
5. Verify: screenshot + note appear in AI chat

### Inbox Fallback (simulate non-sampling client)

1. Disable sampling in daemon config (or use Codex/Gemini)
2. Push annotations from browser
3. Verify: no sampling request sent
4. Make any Kaboom tool call
5. Verify: response includes `_pending_push` hint
6. Call `observe({what: "inbox"})`
7. Verify: full push content returned
8. Call `observe({what: "inbox"})` again
9. Verify: empty (events cleared after drain)
