---
doc_type: product_spec
feature_id: browser-push
status: proposed
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Browser Push — Product Spec

## Problem Statement

Using Kaboom requires constant context-switching between the browser and the AI chat. The worst case is annotations: the user activates draw mode, draws on the page, hits ESC — then has to switch to chat, type "check my annotations," and wait for the AI to call `analyze({what: "annotations"})`. Even with `wait: true`, the AI has to initiate and then poll via correlation_id.

Screenshots have the same problem. The user sees something wrong in the browser and wants the AI to see it — but has to switch contexts to ask.

This friction breaks flow state and makes Kaboom feel like two disconnected tools instead of one.

## Solution

When the user completes an action in the browser (finishes drawing annotations, takes a screenshot via hotkey), the content is automatically delivered to the AI chat. No switching, no typing, no polling.

The delivery mechanism adapts to client capabilities:

1. **MCP Sampling** (best) — daemon sends `sampling/createMessage` to the client, creating a new AI turn with the content inline. The AI sees the annotations/screenshot and responds immediately.
2. **MCP Notifications** (fallback) — daemon sends `notifications/message` with a text summary. Less useful (no image support, no AI turn created) but signals that content is available.
3. **Inbox polling** (universal fallback) — daemon queues the content. The AI's system prompt or skills instruct it to periodically check `observe({what: "inbox"})`. Next check returns the queued content.

## User Stories

### Story 1: Annotation push (sampling client)

1. User is working with Claude Code on a bug
2. User sees a broken layout in the browser
3. User presses `Alt+Shift+D` to activate draw mode (existing hotkey)
4. User draws a rectangle around the broken area, types "this is misaligned"
5. User presses ESC
6. **Automatically**, Claude Code receives the annotated screenshot and the user's feedback
7. Claude Code responds: "I can see the misalignment. The flex container is..."

No context switch. No typing. The user stays in the browser until they're ready to look at the AI's response.

### Story 2: Screenshot push (any client)

1. User is debugging a form validation issue
2. User fills out the form in the browser and sees an unexpected error toast
3. User presses `Alt+Shift+S` (new hotkey) to capture and push a screenshot
4. The screenshot appears in the AI chat with a note: "User pushed a screenshot from the browser"
5. AI responds with analysis

### Story 3: Fallback for non-sampling clients (Codex, Gemini CLI)

1. User draws annotations and hits ESC
2. Daemon detects the client doesn't support sampling
3. Daemon queues the annotations in the inbox
4. On the next Kaboom tool call (any tool), the response includes a `_pending_push` field with the queued annotations
5. AI sees the piggybacked content and responds

## Client Capability Matrix

| Client | Sampling | Notifications | Inbox Fallback |
|--------|----------|---------------|----------------|
| Claude Code | Yes | Yes | Yes |
| Codex | No | Partial (strict schema) | Yes |
| Gemini CLI | No | No | Yes |
| Cursor | Unknown | Unknown | Yes |
| Windsurf | Unknown | Unknown | Yes |

## Core Requirements

### R1: Annotation auto-push
- [ ] When user completes draw mode (ESC), annotations + screenshot are delivered to the AI automatically
- [ ] Delivery uses best available mechanism for the connected client
- [ ] Works for both LLM-initiated and user-initiated (hotkey) draw mode sessions

### R2: Screenshot push hotkey
- [ ] New keyboard shortcut (`Alt+Shift+S`) captures viewport screenshot and pushes to AI
- [ ] User can optionally add a text note before pushing (toast input, 3s timeout, auto-sends if no input)
- [ ] Screenshot is delivered via the same push pipeline as annotations

### R3: Client capability detection
- [ ] Daemon detects client capabilities during MCP initialization handshake
- [ ] Stores whether client supports `sampling`, `notifications/message`, neither
- [ ] Selects delivery mechanism per push event

### R4: MCP sampling implementation
- [ ] Daemon implements `sampling/createMessage` as an MCP client→server request
- [ ] Sampling message includes screenshot as base64 image content
- [ ] Sampling message includes annotation text and coordinates
- [ ] Sampling message includes page URL and timestamp for context

### R5: Inbox fallback
- [ ] New observe mode: `observe({what: "inbox"})` returns queued push events
- [ ] Push events are queued with timestamp, type (annotation/screenshot), and payload
- [ ] Events are cleared after retrieval (read-once)
- [ ] Any Kaboom tool response can piggyback pending push events via `_pending_push` field
- [ ] Skill/system prompt guidance instructs AI to check inbox periodically

### R6: Blocking wait for annotations
- [ ] `analyze({what: "annotations", wait: true})` blocks the response for up to 15s
- [ ] If annotations arrive within 15s, return them directly (zero polling)
- [ ] If 15s expires, return a correlation_id for polling (existing behavior)
- [ ] Blocking uses a goroutine + channel — no busy-wait, no CPU cost

### R7: Notifications fallback
- [ ] When client supports notifications but not sampling, send `notifications/message`
- [ ] Message includes text description of the push content
- [ ] Message includes instruction to call `observe({what: "inbox"})` for full content

## Out of Scope (MVP)

- Continuous screenshot streaming (only on-demand push)
- Push from multiple tabs simultaneously (single active tab only)
- Push to multiple connected AI clients (first connected client only)
- Video clip push (screenshots and annotations only)
- Two-way push (AI pushing visual overlays to the browser — already handled by `interact`)

## Success Criteria

### Functional
- Annotation push works end-to-end with Claude Code (sampling path)
- Screenshot push works end-to-end with Claude Code (sampling path)
- Inbox fallback works with Codex and Gemini CLI
- No data loss: every push event reaches the AI within 30 seconds

### Non-Functional
- Push delivery latency < 500ms (daemon receives annotation → sampling request sent)
- Inbox queue: max 50 events, FIFO eviction
- Zero external network calls (all localhost)
- No new production dependencies

### Integration
- Works with existing draw mode (no breaking changes to `draw_mode_start` or `annotations`)
- Works with existing screenshot capture (`observe({what: "screenshot"})`)
- Keyboard shortcuts don't conflict with browser defaults or existing Kaboom shortcuts

## Relationship to Other Tools

| Tool | Relationship |
|------|-------------|
| `interact({what: "draw_mode_start"})` | Existing activation path — push adds auto-delivery on completion |
| `analyze({what: "annotations"})` | Still works — push is an additional delivery path, not a replacement |
| `observe({what: "screenshot"})` | Pull-based capture — push hotkey is the push-based complement |
| `observe({what: "inbox"})` | New mode — returns queued push events |
| `configure({what: "streaming"})` | Related but different — streaming is for error/perf events, push is for user-initiated content |
