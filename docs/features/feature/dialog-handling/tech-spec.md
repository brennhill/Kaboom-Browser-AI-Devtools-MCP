---
feature: dialog-handling
status: proposed
doc_type: tech-spec
feature_id: feature-dialog-handling
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Dialog Handling

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Dialog handling uses browser's `beforeunload` and dialog event interception. Extension injects dialog handlers in page context, captures dialog details, stores in memory buffer, exposes via observe tool, and provides handle_dialog action in interact tool to respond programmatically.

## Key Components

- **Dialog interceptor (inject.js)**: Override window.alert, window.confirm, window.prompt to capture calls
- **Beforeunload handler**: Listen for beforeunload event, capture message
- **Dialog buffer (server)**: Store pending dialogs with metadata (type, message, timestamp)
- **Observe integration**: Add "dialogs" mode to observe tool
- **Interact integration**: Add "handle_dialog" action to interact tool

## Data Flows

```
Page calls alert("message")
  → inject.js intercepts via window.alert override
  → Capture {type: "alert", message: "message", timestamp}
  → POST to /dialogs endpoint
  → Server stores in dialog buffer
  → Agent polls: observe({what: "dialogs"})
  → Agent responds: interact({what: "handle_dialog", response: "accept"})
  → Server creates pending query
  → Extension polls, executes dialog.accept()
  → Dialog dismissed, workflow continues
```

## Implementation Strategy

### Dialog interception approach:
1. Override native dialog functions in inject.js:
   - Save original functions (window._originalAlert = window.alert)
   - Replace with capture wrapper (window.alert = function(msg) { captureDialog('alert', msg); return _originalAlert(msg); })
2. For confirm/prompt, return agent's response instead of showing native dialog
3. For beforeunload, prevent default and handle programmatically

### Buffer management:
- Store dialogs in ring buffer (max 50, evict oldest)
- Each dialog has: id, type, message, default_value (for prompt), timestamp, status (pending/handled)
- When agent handles dialog, mark status as "handled"

### Response handling:
- Accept: for alert (dismiss), confirm (true), prompt (with text input)
- Dismiss: for confirm (false), prompt (null)

## Edge Cases & Assumptions

- **Edge Case 1**: Multiple dialogs in rapid succession → **Handling**: Queue all in buffer, agent handles one at a time
- **Edge Case 2**: Dialog shown before extension loads → **Handling**: Can't intercept, native dialog shown (acceptable limitation)
- **Edge Case 3**: Beforeunload with no message → **Handling**: Capture with empty message, allow navigation
- **Edge Case 4**: Timeout on dialog handling → **Handling**: Auto-dismiss after 10s if agent doesn't respond
- **Assumption 1**: Dialogs are shown in active tab only (cross-tab dialogs out of scope)

## Risks & Mitigations

- **Risk 1**: Native dialog shows before interception → **Mitigation**: Inject as early as possible (document_start), acceptable edge case
- **Risk 2**: Browser blocks dialog override → **Mitigation**: Test across Chrome versions, fallback to observation-only mode
- **Risk 3**: Beforeunload prevents navigation → **Mitigation**: Agent can force navigate via interact tool
- **Risk 4**: Dialog text contains sensitive data → **Mitigation**: Redaction rules apply to dialog messages

## Dependencies

- Existing inject.js injection mechanism
- Async command architecture for handle_dialog action
- New /dialogs HTTP endpoint for posting captured dialogs

## Performance Considerations

- Dialog interception adds negligible overhead (<0.01ms per dialog call)
- Dialog buffer limited to 50 entries (~5KB memory)

## Security Considerations

- Dialog handling gated by AI Web Pilot toggle
- Cannot override browser security dialogs (permission prompts, authentication)
- Dialog messages subject to same redaction rules as logs
