---
feature: annotated-screenshots
status: proposed
doc_type: tech-spec
feature_id: feature-annotated-screenshots
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Annotated Screenshots

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Annotated Screenshots adds `generate({what: "annotated_screenshot"})` -- captures screenshot of current page and overlays element labels and bounding boxes to help AI vision models understand page structure. Implemented using Chrome's `chrome.tabs.captureVisibleTab()` API (background.js) and Canvas API (inject.js) for annotation rendering.

## Key Components

**Screenshot Capture**: Uses `chrome.tabs.captureVisibleTab({format: "png"})` to capture visible viewport as data URL. Captures at current zoom level and window size (no forced dimensions).

**Element Detection**: Queries DOM for interactive elements (links, buttons, inputs, forms) using same logic as a11y tree. Computes bounding box for each element via `getBoundingClientRect()`.

**Annotation Rendering**: Creates offscreen canvas matching viewport size. Draws captured screenshot as background. Overlays for each element: colored bounding box (green for buttons, blue for links, yellow for inputs), label showing element type and accessible name, numerical ID corresponding to UID from a11y tree.

**Output Format**: Returns annotated PNG as base64 data URL. Also returns metadata JSON mapping IDs to selectors and element info (same as a11y tree uidMap).

## Data Flows

```
AI calls generate({what: "annotated_screenshot", scope: "#main"})
  |
  v
Server creates pending query
  |
  v
background.js captures screenshot
  -> chrome.tabs.captureVisibleTab()
  |
  v
inject.js receives screenshot data URL
  -> Queries DOM for interactive elements
  -> Computes bounding boxes
  -> Creates canvas, draws screenshot
  -> Overlays boxes and labels
  -> Exports annotated image as data URL
  |
  v
Returns {image: "data:image/png;base64,...", uidMap: {...}}
```

## Implementation Strategy

**Extension files**:
- `extension/background.js` (modified): Add screenshot capture logic
- `extension/lib/annotated-screenshot.js` (new): Canvas rendering, annotation logic
- `extension/inject.js` (modified): Coordinate screenshot capture and annotation

**Server files**:
- `cmd/browser-agent/generate.go`: Add `generateAnnotatedScreenshot()` handler

**Trade-offs**:
- Visible viewport only (not full-page scrolling screenshot) to keep simple
- Canvas-based annotation (not SVG overlay) for broader browser compatibility
- PNG format (not JPEG) to preserve annotation quality

## Edge Cases & Assumptions

- **Extension lacks tabs permission**: Error returned
- **Tab not visible**: Screenshot fails with error
- **No interactive elements**: Screenshot captured without annotations
- **Large viewport (4K display)**: Image size may be large (>2MB). AI warned if size exceeds token budget.

## Risks & Mitigations

**Risk**: Large screenshots exceed context window.
**Mitigation**: Resize image to max 1920x1080 before returning. Include original dimensions in metadata.

**Risk**: Elements outside viewport not annotated.
**Mitigation**: Document limitation. AI can scroll and capture multiple screenshots if needed.

## Dependencies

- `chrome.tabs.captureVisibleTab` API
- Canvas API
- a11y tree logic (for element detection and UID assignment)

## Performance Considerations

| Metric | Target |
|--------|--------|
| Capture + annotation time | < 500ms |
| Image size | < 2MB (after resize) |
| Memory impact | < 5MB |

## Security Considerations

- Screenshot captures visible content only (same as user sees)
- Data URL stays on localhost (never uploaded)
- Sensitive input values redacted before annotation (same redaction as a11y tree)
