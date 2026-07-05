---
status: proposed
scope: feature/multiline-rich-editor
ai-priority: high
tags: [interact, type, paste, rich-text, form-filling]
doc_type: product-spec
feature_id: feature-multiline-rich-editor
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Multiline Rich Editor Support

## Problem Statement

The `interact` tool's `type` and `paste` actions fail to insert multiline text into modern rich text editors (Quill, ProseMirror, Draft.js, TinyMCE). The current implementation uses `document.execCommand('insertParagraph')` which is deprecated and ignored by these frameworks. This forces agents to type line-by-line with separate `Enter` keypresses — each a ~5s round-trip — making a simple LinkedIn post take 2.5 minutes instead of 1 second.

**Root causes:**
1. `execCommand` is deprecated and modern editors don't respond to it
2. `paste` dispatches a synthetic `ClipboardEvent` that editors may strip or ignore
3. `execute_js` (the escape hatch) is blocked by CSP on sites like LinkedIn
4. No awareness of which rich text framework is active on the page

## Solution

Two layered improvements:

### A. Robust Multiline Type via Keyboard Simulation

When `type` or `paste` receives multiline text targeting a `contenteditable` element, the extension splits on `\n` and simulates real keyboard events (`KeyboardEvent` for Enter) between segments — all within a single command execution. No MCP round-trips. The caller sends one action, the extension handles it atomically.

This is the universal fallback that works everywhere, regardless of editor framework.

### B. Rich Editor Detection + Native API Insertion

The extension detects known rich text editor frameworks by their DOM signatures and uses their native APIs from the content script context. This bypasses both the `execCommand` limitation and CSP restrictions (content scripts have full DOM access).

**Detected editors and their signatures:**

| Editor | DOM Signature | Native API |
|---|---|---|
| Quill | `.ql-editor` | Set `innerHTML` on `.ql-editor` + dispatch `input` event |
| ProseMirror | `.ProseMirror` | Set `innerHTML` on `.ProseMirror` + dispatch `input` event |
| Draft.js | `[data-editor]`, `[data-contents]` | Set `innerHTML` on editor root + dispatch `input` event |
| TinyMCE | `.mce-content-body`, `#tinymce` | Set `innerHTML` on body element + dispatch `input` event |
| CKEditor 5 | `.ck-editor__editable` | Set `innerHTML` on editable + dispatch `input` event |

**Fallback chain:**
1. Detect known editor framework -> use native DOM insertion via content script
2. No known editor detected -> use keyboard simulation (Option A)
3. Not a contenteditable element -> use existing `nativeSetter` path for standard inputs

## API

No API changes. The existing `type` and `paste` actions accept multiline text already — they just need to handle it correctly. The improvement is entirely internal to the extension.

```json
{
  "what": "type",
  "selector": "div.ql-editor",
  "text": "Line one.\n\nLine two.\n\nLine three.",
  "clear": true
}
```

Expected result: three paragraphs inserted with blank lines between them, in a single command round-trip.

### New Response Fields

The action result includes a new optional field indicating which insertion strategy was used:

```json
{
  "success": true,
  "action": "type",
  "insertion_strategy": "quill_native",
  "value": "Line one.\n\nLine two.\n\nLine three."
}
```

Values: `"quill_native"`, `"prosemirror_native"`, `"draftjs_native"`, `"tinymce_native"`, `"ckeditor_native"`, `"keyboard_simulation"`, `"native_setter"`.

## Success Criteria

1. Multiline `type` into a Quill editor (LinkedIn, Notion) inserts all lines in a single command (<1s)
2. Multiline `type` into a ProseMirror editor (Confluence, Atlassian) inserts all lines in a single command
3. Multiline `type` into a standard `contenteditable` div works via keyboard simulation in a single command
4. Multiline `type` into a `<textarea>` continues to work via existing native setter path
5. `paste` follows the same detection and insertion logic as `type`
6. `insertion_strategy` field is present in the response
7. No regressions on standard `<input>` and `<textarea>` elements
8. Works on CSP-restricted pages (LinkedIn, GitHub, Google Docs)

## Out of Scope

- Google Docs (uses a custom canvas-based editor, not contenteditable)
- Collaborative editors with OT/CRDT sync (may reject external DOM mutations)
- Image/media insertion (this feature is text-only)
- New MCP tool parameters (this is an internal extension improvement)

## Notes

- Content scripts run in an isolated world but share the DOM with the page. They can read/write `innerHTML`, `textContent`, and dispatch DOM events without being blocked by CSP. This is the key insight that makes Option B possible.
- `document.execCommand` is formally deprecated since 2023. Quill 2.x and ProseMirror explicitly do not listen for execCommand events.
- The keyboard simulation path (Option A) should use `KeyboardEvent` with proper `key`, `code`, `keyCode` fields to maximize compatibility with editor event handlers.
