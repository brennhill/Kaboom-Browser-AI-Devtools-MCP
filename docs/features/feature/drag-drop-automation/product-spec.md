---
feature: drag-drop-automation
status: proposed
tool: interact
mode: execute_js
version: 0.7.12
doc_type: product-spec
feature_id: feature-drag-drop-automation
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Drag & Drop Automation

## Problem Statement

Modern web apps extensively use drag-and-drop interfaces (Trello-style boards, file uploads, sortable lists, visual editors). AI agents testing these apps cannot programmatically simulate drag-and-drop interactions, making automation of complex workflows impossible.

## Solution

Add `drag_drop` action to the `interact` tool. Agent specifies source element (what to drag) and target element (where to drop). Extension simulates both HTML5 drag-and-drop API events and legacy mouse event sequences to support all drag-drop implementations.

## Requirements

- Support HTML5 drag-and-drop API (dragstart, drag, dragover, drop, dragend events)
- Support legacy mouse event simulation (mousedown, mousemove, mouseup)
- Auto-detect which method target element uses
- Handle drag-and-drop with dataTransfer (file drops, text drops)
- Support coordinate-based drops (x, y position) for canvas/visual editors
- Work with sortable lists (reordering items)
- Support drag-from-outside (e.g., file from desktop) simulation
- Return success/failure with final element position

## Out of Scope

- Actual file system access for file drops (security restriction) — can simulate drop event, but file data must be provided by agent
- Cross-origin drag-drop (security boundary)
- Native OS drag-drop (outside browser window)

## Success Criteria

- Agent can reorder Trello-style cards programmatically
- Agent can simulate file drop on upload zone
- Agent can drag elements in visual editors (diagram tools, layout builders)
- Sortable lists respond correctly to programmatic drag-drop

## User Workflow

1. Agent observes DOM to identify draggable and drop target elements
2. Agent calls `interact({what: "drag_drop", source: "#card-1", target: "#column-2"})`
3. Extension simulates drag sequence, triggers all required events
4. App's drag-drop handlers execute, element moves
5. Agent observes DOM changes or network requests to verify operation

## Examples

### Trello-style card move:
```json
{
  "action": "drag_drop",
  "source": "#card-123",
  "target": "#column-done"
}
```

### Sortable list reorder:
```json
{
  "action": "drag_drop",
  "source": "li[data-id='5']",
  "target": "li[data-id='2']",
  "position": "before"
}
```

### File drop simulation:
```json
{
  "action": "drag_drop",
  "source": null,
  "target": "#file-upload-zone",
  "data_transfer": {
    "files": [{"name": "test.pdf", "type": "application/pdf", "size": 1024}]
  }
}
```

### Coordinate-based drop (canvas):
```json
{
  "action": "drag_drop",
  "source": "#shape-1",
  "target_x": 250,
  "target_y": 150
}
```

---

## Notes

- Supports both HTML5 and legacy mouse event patterns
- Uses async command architecture (60s timeout window)
- AI Web Pilot toggle must be enabled
