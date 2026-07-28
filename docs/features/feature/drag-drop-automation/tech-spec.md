---
feature: drag-drop-automation
status: proposed
doc_type: tech-spec
feature_id: feature-drag-drop-automation
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Drag & Drop Automation

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Drag-drop automation simulates user drag actions via JavaScript event creation and dispatch. Two approaches: HTML5 DnD API (modern) and mouse event simulation (legacy). Extension auto-detects which method to use based on element's event listeners, falls back to both if uncertain.

## Key Components

- **Event simulator (inject.js)**: Create and dispatch drag/mouse events in correct sequence
- **Element detector**: Identify source and target elements via selectors or coordinates
- **DataTransfer handler**: Build DataTransfer object for HTML5 drops (files, text, URLs)
- **Position calculator**: Compute element centers or use explicit coordinates
- **Async command handler**: Standard interact tool async pattern

## Data Flows

```
Agent: interact({what: "drag_drop", source: "#elem1", target: "#elem2"})
  → Server validates, creates pending query
  → Extension polls, receives drag_drop command
  → inject.js:
      1. Query source element, get center coordinates
      2. Query target element, get drop coordinates
      3. Dispatch HTML5 sequence:
         - dragstart on source
         - drag on source (continuously)
         - dragenter on target
         - dragover on target
         - drop on target
         - dragend on source
      4. If HTML5 fails, dispatch mouse sequence:
         - mousedown on source
         - mousemove to intermediate points
         - mouseup on target
  → POST result to /execute-result
  → Agent polls for result
```

## Implementation Strategy

### HTML5 Drag-Drop sequence:
1. Create DataTransfer object (polyfill if needed)
2. Dispatch dragstart event on source with dataTransfer
3. Dispatch drag event continuously (simulate dragging motion)
4. Dispatch dragenter event on target
5. Dispatch dragover event on target (must call preventDefault() to allow drop)
6. Dispatch drop event on target with dataTransfer
7. Dispatch dragend event on source

### Mouse event fallback sequence:
1. Dispatch mousedown event on source at element center
2. Dispatch multiple mousemove events along path from source to target (simulate dragging motion)
3. Dispatch mouseup event on target at element center

### Auto-detection strategy:
- Check if target has dragover or drop event listeners (HTML5)
- Check if source has mousedown listeners (mouse events)
- If both, try HTML5 first, fallback to mouse if no effect
- If uncertain, dispatch both sequences

## Edge Cases & Assumptions

- **Edge Case 1**: Element not draggable → **Handling**: Force draggable attribute, attempt drag anyway
- **Edge Case 2**: Drop rejected by handler → **Handling**: Return error with handler's feedback
- **Edge Case 3**: Element moves during drag (animated) → **Handling**: Recalculate target position mid-drag
- **Edge Case 4**: Multiple drop targets overlap → **Handling**: Use first matching target in DOM order
- **Assumption 1**: Elements are visible and have layout (not display:none)
- **Assumption 2**: Drag handlers execute synchronously (or complete within timeout)

## Risks & Mitigations

- **Risk 1**: Framework doesn't recognize synthetic events → **Mitigation**: Dispatch both HTML5 and mouse sequences
- **Risk 2**: Complex drag logic times out → **Mitigation**: 60s timeout, return partial result
- **Risk 3**: DataTransfer not fully polyfilled → **Mitigation**: Create minimal DataTransfer object with required properties
- **Risk 4**: Coordinate calculation wrong for transformed elements → **Mitigation**: Use getBoundingClientRect() for accurate positioning

## Dependencies

- inject.js for event dispatch
- Async command architecture
- AI Web Pilot toggle

## Performance Considerations

- Simple drag-drop: <100ms (event dispatch is fast)
- Animated drag (many mousemove events): <500ms
- Complex drop handlers: may approach async timeout limits
- Total timeout: 60s

## Security Considerations

- Cannot drag files from OS into browser (security restriction)
- Can simulate drop event with file metadata, but actual file data must be provided by agent
- Gated by AI Web Pilot toggle
- Cross-origin drops blocked by same-origin policy
