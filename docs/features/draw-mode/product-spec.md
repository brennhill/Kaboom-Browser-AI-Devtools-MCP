---
feature: Draw Mode
status: proposed
tool: interact, analyze
version: 0.7.12
doc_type: product-spec
feature_id: draw-mode
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Draw Mode (Visual Annotation for LLM-Assisted Development)

## Problem Statement

### LLMs cannot see what users see on a web page, and users cannot easily communicate visual intent to LLMs.

Today, when a developer wants an LLM to change a specific part of a UI:

1. **Description gap**: The user describes what they want changed in natural language ("make the submit button darker"), but the LLM cannot map this to the correct DOM element with confidence
2. **Selector gap**: The LLM must guess which element the user means, often requiring back-and-forth clarification
3. **Context gap**: Even with a screenshot, the LLM lacks computed styles, element hierarchy, and precise selectors needed to write correct CSS/code changes
4. **Feedback gap**: There is no structured way for users to mark up a live page and have those markups flow back to the LLM as actionable data

**Result:** Visual UI changes that should take seconds require minutes of back-and-forth, and the LLM often targets the wrong element.

---

## Solution

**Draw Mode** lets users visually annotate web pages with rectangles and text feedback. Annotations are readable by LLMs via MCP and exportable as annotated PNG screenshots.

### The workflow:
1. **LLM activates** draw mode via `interact({what: "draw_mode_start"})` and asks the user to annotate what they want changed
2. **User draws** rectangles around elements and types feedback (e.g., "this button should be darker", "add padding here")
3. **User presses ESC** to finish annotating
4. **LLM reads** all annotations via `analyze({what: "annotations"})` -- gets element summaries, user text, and an annotated screenshot
5. **LLM drills down** via `analyze({what: "annotation_detail", correlation_id: "..."})` to get full computed styles and precise selectors
6. **LLM writes code** with exact knowledge of which elements to change and what the user wants

### Why Draw Mode:
- **Direct communication** -- users point at exactly what they mean, eliminating ambiguity
- **DOM-aware** -- every rectangle is mapped to the underlying DOM element with selector, classes, computed styles
- **LLM-native** -- annotations are structured data, not images; LLMs can act on them programmatically
- **Bidirectional** -- both humans and LLMs can initiate draw mode
- **Local-first** -- all data stays on localhost, screenshots saved to temp directory

---

## User Stories

### US-1: LLM Requests User Annotation

```
Developer: "Make the hero section look better"
LLM: "I'll activate draw mode -- please draw rectangles around the
      elements you want changed and type what you'd like."
LLM: [calls interact({what: "draw_mode_start"})]
User: [draws rectangle around heading] "make this bigger and bolder"
User: [draws rectangle around button] "change color to dark blue"
User: [presses ESC]
LLM: [calls analyze({what: "annotations"})]
LLM: "I see 2 annotations. Let me get the details..."
LLM: [calls analyze({what: "annotation_detail", correlation_id: "..."})]
LLM: "The heading is an h1.hero-title with font-size 24px. The button is
      button.cta-primary with background-color rgb(59, 130, 246).
      I'll update both now."
```

### US-2: User Initiates Draw Mode Directly

```
User: [presses Cmd+Shift+D or clicks toggle in popup]
User: [draws rectangles and types annotations]
User: [presses ESC]
User: "I just annotated the page -- can you fix these issues?"
LLM: [calls analyze({what: "annotations"})]
LLM: "I see your annotations. Let me make those changes."
```

### US-3: LLM Gets DOM Details for Code Changes

```
LLM: [calls analyze({what: "annotation_detail", correlation_id: "ann_detail_abc"})]
Result: {
  selector: "button.btn-primary",
  computed_styles: { "background-color": "rgb(59, 130, 246)", "font-size": "14px" },
  parent_selector: "form.checkout-form > div.actions",
  ...
}
LLM: "I have the exact selector and current styles. Updating the CSS now."
```

---

## Activation and Deactivation

### Activation Methods

| Method | Actor | Trigger |
|--------|-------|---------|
| Keyboard shortcut | User | Cmd+Shift+D (macOS) / Ctrl+Shift+D (Windows/Linux) |
| Popup toggle | User | Click "Draw Mode" toggle in extension popup |
| MCP interact tool | LLM | `interact({what: "draw_mode_start"})` |

### Deactivation Methods

| Method | Actor | Trigger |
|--------|-------|---------|
| ESC key | User | Press ESC while in draw mode |
| Popup toggle | User | Click "Draw Mode" toggle again in extension popup |

**Design constraint:** LLMs cannot deactivate draw mode. Only the user can exit, ensuring the user always has control over when they are done annotating.

---

## Core Requirements

### R1: Drawing Interface

- [ ] Full-viewport transparent overlay (canvas) that intercepts mouse events
- [ ] Draw rectangles by click-and-drag
- [ ] Rectangles rendered with visible border (e.g., 2px red dashed)
- [ ] Minimum rectangle size: 5x5px (smaller ignored as accidental clicks)
- [ ] After drawing a rectangle, a text input appears for the user to type feedback
- [ ] Text input auto-focuses after rectangle is drawn
- [ ] Enter key confirms text and allows drawing another rectangle
- [ ] Empty text on blur removes the annotation
- [ ] Existing annotations remain visible while drawing new ones

### R2: DOM Element Capture

- [ ] On rectangle creation, identify the topmost DOM element under the rectangle center
- [ ] Capture lightweight element summary: tag name, primary class, text content snippet
- [ ] Generate unique CSS selector for the element
- [ ] Full computed styles captured on demand (not eagerly, to avoid performance cost)

### R3: Annotation Persistence

- [ ] Annotations persisted to `chrome.storage.session` on every change
- [ ] Survives in-page navigation (SPA route changes)
- [ ] Cleared on full page navigation (new document load)
- [ ] Each annotation has a unique ID: `ann_{timestamp}_{random}`

### R4: MCP Integration

- [ ] `interact({what: "draw_mode_start"})` activates draw mode
- [ ] `analyze({what: "annotations"})` returns all annotations with lightweight element summaries
- [ ] `analyze({what: "annotations", wait: true})` blocks until user exits draw mode (5-minute timeout)
- [ ] `analyze({what: "annotation_detail", correlation_id: "..."})` returns full computed styles and DOM detail
- [ ] Annotated screenshot (PNG) captured on deactivation and included in results

### R5: Screenshot Export

- [ ] On deactivation, capture a screenshot of the page with annotation overlays visible
- [ ] Save to temp directory (consistent with existing screenshot behavior)
- [ ] Screenshot path returned in the analyze response
- [ ] PNG format for quality (annotations include text)

---

## Out of Scope

- Freehand drawing (lines, circles, arrows) -- rectangles only for MVP
- Annotation editing (move, resize, delete individual annotations) -- draw new ones instead
- Cross-tab annotations -- draw mode is per-tab
- Collaborative annotations -- single user per session
- Video recording of annotation process
- Annotation templates or presets

---

## Success Criteria

### Functional

- LLM can activate draw mode, wait for user annotations, and read structured results
- User can draw rectangles and type feedback on any web page
- DOM element details (selector, computed styles) are accurate and actionable
- Annotated screenshot is captured and accessible via MCP
- All three activation methods work (keyboard, popup, MCP)
- ESC exits cleanly with all data preserved

### Non-Functional

- Draw mode overlay renders in < 50ms
- DOM capture per rectangle completes in < 100ms
- Full annotation detail (computed styles) returns in < 200ms
- Screenshot capture completes in < 500ms
- Zero impact on page functionality while draw mode is inactive
- Works on CSP-restricted pages (overlay runs in isolated world)

### Integration

- Works with existing `interact` and `analyze` tools
- Uses existing PendingQuery system for async LLM activation
- Compatible with existing screenshot infrastructure
- Annotations accessible alongside other `analyze` modes

---

## Relationship to Other Tools

| Tool | Role in Draw Mode |
|------|-------------------|
| `interact` | Activates draw mode (`draw_mode_start`) |
| `analyze` | Reads annotations (`annotations`, `annotation_detail`) |
| `observe` | Not directly involved; draw mode data flows through `analyze` |
| `generate` | LLM may use annotation data to generate code fixes |
| `configure` | Not involved |

---

## Next Steps

1. **Tech Spec** -- Architecture, data model, sequence diagrams, edge cases
2. **QA Plan** -- Unit tests, integration tests, manual test checklist
3. **Implementation** -- TDD, write failing tests first
