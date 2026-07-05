---
feature: dialog-handling
status: proposed
tool: interact
mode: execute_js
version: 0.7.12
doc_type: product-spec
feature_id: feature-dialog-handling
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Dialog Handling

## Problem Statement

Web applications use JavaScript dialogs (alert, confirm, prompt, beforeunload) to interact with users. When AI agents automate browser workflows, these dialogs block execution and prevent automation from continuing. Agents need programmatic dialog handling to respond to dialogs without human intervention.

## Solution

Add `handle_dialog` action to the `interact` tool. Extension automatically detects when dialogs appear, notifies agent via observe tool, and allows agent to programmatically accept, dismiss, or provide input to dialogs. Supports alert, confirm, prompt, and beforeunload dialogs.

## Requirements

- Detect all dialog types (alert, confirm, prompt, beforeunload)
- Auto-capture dialog text/message for agent observation
- Programmatic response: accept, dismiss, or provide text input
- Handle dialogs triggered by page JavaScript or browser actions
- Support beforeunload dialogs (triggered on page navigation/close)
- Queue multiple dialogs if they appear in rapid succession
- Return dialog response status to agent

## Out of Scope

- Browser-native dialogs (file picker, print dialog) — these require OS-level interaction
- HTTP authentication dialogs — separate feature
- Extension permission prompts — cannot be automated due to browser security
- Custom modal dialogs (non-native) — handled via execute_js or form filling

## Success Criteria

- Agent can detect alert dialog and acknowledge it programmatically
- Agent can respond to confirm dialog (accept or cancel)
- Agent can provide input to prompt dialog
- Beforeunload dialogs can be handled to allow navigation
- Automated workflows don't hang waiting for human dialog interaction

## User Workflow

1. Agent triggers action that shows dialog (e.g., click button that runs `alert('Hello')`)
2. Extension detects dialog, captures text, stores in buffer
3. Agent observes dialogs via `observe({what: "dialogs"})`
4. Agent responds via `interact({action: "handle_dialog", response: "accept"})` or `{response: "dismiss"}` or `{response: "accept", text: "user input"}`
5. Extension handles dialog, workflow continues

## Examples

### Alert dialog:
```json
// Observe detects dialog
{
  "type": "alert",
  "message": "Form submitted successfully!",
  "timestamp": "2026-01-28T10:00:00Z"
}

// Agent acknowledges
{
  "action": "handle_dialog",
  "response": "accept"
}
```

### Confirm dialog:
```json
// Observe detects
{
  "type": "confirm",
  "message": "Are you sure you want to delete this item?",
  "timestamp": "2026-01-28T10:01:00Z"
}

// Agent confirms
{
  "action": "handle_dialog",
  "response": "accept"  // or "dismiss"
}
```

### Prompt dialog:
```json
// Observe detects
{
  "type": "prompt",
  "message": "Enter your name:",
  "default_value": "Guest",
  "timestamp": "2026-01-28T10:02:00Z"
}

// Agent provides input
{
  "action": "handle_dialog",
  "response": "accept",
  "text": "John Doe"
}
```

---

## Notes

- Dialogs are captured in separate buffer (similar to logs, network events)
- Uses async command architecture for handling
- AI Web Pilot toggle must be enabled for dialog handling
