---
feature: multiline-rich-editor
status: proposed
doc_type: tech-spec
feature_id: feature-multiline-rich-editor
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Multiline Rich Editor Support

> Plain language only. No code.

## Architecture Overview

This feature modifies the `type` and `paste` action handlers in `dom-primitives.js` to add a detection + dispatch layer before text insertion. The change is entirely within the extension's content script — no MCP server changes, no new wire types, no new commands.

```
Caller sends: interact({ what: "type", text: "line1\nline2\nline3" })
                                    |
                            dom-primitives.js
                                    |
                    +---------------+---------------+
                    |               |               |
              detect editor    no editor found   standard input
              framework        contenteditable    (<input>/<textarea>)
                    |               |               |
              native DOM       keyboard           native setter
              insertion        simulation          (existing path)
                    |               |               |
              set innerHTML    split on \n          set .value
              + input event    type + Enter keys    + input event
                    |               |               |
                    +-------+-------+---------------+
                            |
                    return result with insertion_strategy
```

## Key Components

### 1. Editor Detection (new)

A detection function that runs before insertion. Walks up from the target node to find known editor containers. Returns the editor type and the correct insertion target.

**Detection order (most specific first):**
1. Target or ancestor has class `ql-editor` -> Quill
2. Target or ancestor has class `ProseMirror` -> ProseMirror
3. Target or ancestor has attribute `data-editor` or `data-contents` -> Draft.js
4. Target or ancestor has class `mce-content-body` or id `tinymce` -> TinyMCE
5. Target or ancestor has class `ck-editor__editable` -> CKEditor 5
6. Target has `contenteditable="true"` but no known framework -> generic contenteditable
7. Target is `<input>` or `<textarea>` -> standard input

Detection must be fast (DOM class/attribute checks only, no JS API probing).

### 2. Native DOM Insertion (new)

For detected editors, the content script directly manipulates the editor's DOM:

**Strategy for Quill, ProseMirror, CKEditor, TinyMCE:**
- Split text on `\n`
- Build HTML: each line becomes a `<p>` element; empty lines become `<p><br></p>`
- If `clear` option is set, replace `innerHTML`; otherwise append to existing content
- Dispatch `input` event with `{ bubbles: true }` so the editor's internal model syncs
- Read back the final text content for the response

**Strategy for Draft.js:**
- Draft.js uses a more opaque DOM structure with `data-block` divs
- Same HTML insertion approach but wrap each line in `<div data-block="true"><div class="public-DraftStyleDefault-block"><span>text</span></div></div>`
- Dispatch `input` event on the editor root

**Why this works despite CSP:**
Content scripts share the page's DOM but run in an isolated JavaScript world. Setting `innerHTML` and dispatching DOM events are not restricted by CSP — CSP only blocks inline script execution and `eval()`. The content script can freely mutate the DOM.

### 3. Keyboard Simulation Fallback (modified)

For generic contenteditable elements where no framework is detected, replace the current `execCommand` approach:

- Split text on `\n`
- For each segment:
  - If not the first segment, dispatch a `KeyboardEvent` for Enter (keydown + keypress + keyup with `key: "Enter"`, `code: "Enter"`, `keyCode: 13`)
  - For the text content, use `document.execCommand('insertText', false, segment)` — this still works for basic contenteditable divs, just not for framework-managed editors
- All happens synchronously within a single action execution

### 4. Standard Input Path (unchanged)

For `<input>` and `<textarea>` elements, the existing `nativeSetter` path is unchanged. Multiline text in a `<textarea>` already works correctly via the native value setter.

## Data Flow

1. MCP server receives `interact({ what: "type", selector: ".ql-editor", text: "Hello\n\nWorld" })`
2. Server creates a pending query, extension picks it up
3. Extension resolves selector to DOM node
4. `dom-primitives.js` `type` handler runs
5. Editor detection: finds `.ql-editor` -> returns `{ type: "quill", target: <div.ql-editor> }`
6. Native insertion: builds `<p>Hello</p><p><br></p><p>World</p>`, sets innerHTML, dispatches input event
7. Returns `{ success: true, insertion_strategy: "quill_native", value: "Hello\n\nWorld" }`
8. Extension posts result to `/execute-result`
9. Client receives completed result

## Implementation Strategy

### Phase 1: Editor Detection + Native Insertion
- Add `detectRichEditor(node)` function to `dom-primitives.js`
- Add `insertViaEditorDOM(editorType, target, text, clear)` function
- Modify `type` handler: call detection first, dispatch to native insertion or fallback
- Modify `paste` handler: same detection and dispatch logic

### Phase 2: Keyboard Simulation Fallback
- Replace `execCommand('insertParagraph')` with `KeyboardEvent` dispatch for Enter
- Keep `execCommand('insertText')` for text segments (still works in basic contenteditable)
- This improves the generic case even when no framework is detected

### Phase 3: Testing + Edge Cases
- Test against live Quill, ProseMirror, Draft.js, TinyMCE instances
- Test CSP-restricted pages (LinkedIn, GitHub)
- Test `clear` option with each editor type
- Test empty lines, single lines, very long text
- Regression test standard inputs and textareas

## Files to Modify

| File | Change |
|---|---|
| `extension/background/dom/primitives/dom-primitives.js` | Add editor detection, native insertion, modify type/paste handlers |
| `extension/background/dom-primitives-richtext.test.js` | Add tests for each editor type, multiline insertion, fallback chain |
| `docs/features/feature-navigation.md` | Add entry for this feature |

No changes to: MCP server (Go), wire types, MCP tool schemas, content script messaging, or background service worker.

## Edge Cases & Assumptions

- **Editor not yet initialized:** Some editors initialize async. If `.ql-editor` exists but Quill hasn't bound yet, native DOM insertion may not sync to the editor model. Mitigation: dispatch both `input` and `change` events; most editors re-sync from DOM on these.
- **Collaborative editors:** Editors with real-time sync (Google Docs, Notion multiplayer) may reject or overwrite DOM mutations that bypass their sync protocol. This is documented as out of scope.
- **Nested editors:** A page may have multiple editors. Detection uses the target node's ancestor chain, so it naturally scopes to the correct editor instance.
- **Custom themes/wrappers:** Some sites wrap Quill in custom containers that rename classes. Detection checks for the standard class names only. If detection fails, falls through to keyboard simulation.
- **Undo/redo:** Native DOM insertion via innerHTML will likely break the editor's undo stack. This is an acceptable tradeoff — the alternative (keyboard simulation) is too slow for large text blocks.

## Risks & Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Editor DOM structure changes between versions | Insertion produces malformed content | Detection is class-based, insertion uses simple `<p>` elements. Degrade to keyboard simulation if insertion fails. |
| innerHTML mutation doesn't trigger editor model sync | Editor shows content but internal state is stale | Dispatch input, change, and compositionend events. Verify by reading back text content. |
| New editor frameworks not detected | Falls through to keyboard simulation | Keyboard simulation is the universal fallback. New editors can be added to the detection list incrementally. |

## Performance Considerations

- Editor detection: 1-3 DOM queries per action (class/attribute checks). Negligible.
- Native DOM insertion: Single innerHTML set + event dispatch. Sub-millisecond.
- Keyboard simulation: N keypresses for N lines. Still sub-millisecond since it's synchronous DOM event dispatch within the content script, not MCP round-trips.
- Net improvement: A 20-line post goes from ~100s (20 round-trips) to <100ms (single command).
