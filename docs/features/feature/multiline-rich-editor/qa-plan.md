---
feature: multiline-rich-editor
doc_type: qa-plan
feature_id: feature-multiline-rich-editor
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Multiline Rich Editor Support

## Testing Strategy

### Code Testing (Automated)

Unit tests in `dom-primitives-richtext.test.js` covering:
- Editor detection for each framework (Quill, ProseMirror, Draft.js, TinyMCE, CKEditor)
- Detection returns null for standard inputs and textareas
- Detection scopes to nearest editor ancestor (not sibling editors)
- Native insertion produces correct HTML structure for each editor
- Native insertion with `clear: true` replaces content
- Native insertion with `clear: false` appends content
- Keyboard simulation fallback for generic contenteditable
- Standard input/textarea path is unchanged
- `insertion_strategy` field is present and correct in response
- Empty text, single line, multiline, text with consecutive newlines
- Very long text (1000+ characters)

### Human UAT Walkthrough

#### Scenario 1: LinkedIn Post (Quill Editor)

- **Given:** LinkedIn feed page is open, composer is activated
- **When:** `interact({ what: "type", selector: "div.ql-editor", text: "Line 1\n\nLine 2\n\nLine 3", clear: true })`
- **Then:** All three lines appear as separate paragraphs with blank lines between them
- **Verification:** `interact({ what: "get_text", selector: "div.ql-editor" })` returns all three lines. Response includes `insertion_strategy: "quill_native"`.

#### Scenario 2: GitHub Issue Body (ProseMirror / CodeMirror)

- **Given:** GitHub new issue page is open
- **When:** `interact({ what: "type", selector: "textarea#issue_body", text: "## Title\n\nBody paragraph.\n\n- Item 1\n- Item 2" })`
- **Then:** Full markdown text appears in the textarea
- **Verification:** `interact({ what: "get_value", selector: "textarea#issue_body" })` returns the complete text.

#### Scenario 3: Generic Contenteditable (Keyboard Simulation Fallback)

- **Given:** A page with a basic `<div contenteditable="true">` (no framework)
- **When:** `interact({ what: "type", selector: "[contenteditable]", text: "First\nSecond\nThird" })`
- **Then:** Three lines appear in the editor, separated by line breaks
- **Verification:** Response includes `insertion_strategy: "keyboard_simulation"`.

#### Scenario 4: Standard Textarea (No Regression)

- **Given:** A page with a `<textarea>`
- **When:** `interact({ what: "type", selector: "textarea", text: "Line A\nLine B\nLine C" })`
- **Then:** All three lines appear in the textarea separated by newlines
- **Verification:** `interact({ what: "get_value", selector: "textarea" })` returns `"Line A\nLine B\nLine C"`. Response includes `insertion_strategy: "native_setter"`.

#### Scenario 5: Paste Action with Multiline Text

- **Given:** LinkedIn composer is open (Quill editor)
- **When:** `interact({ what: "paste", selector: "div.ql-editor", text: "Pasted line 1\n\nPasted line 2" })`
- **Then:** Both lines appear as separate paragraphs
- **Verification:** Same detection and insertion logic as `type`. Response includes `insertion_strategy`.

#### Scenario 6: Clear + Replace in Rich Editor

- **Given:** LinkedIn composer already has existing text
- **When:** `interact({ what: "type", selector: "div.ql-editor", text: "Completely new content\n\nSecond paragraph", clear: true })`
- **Then:** Old content is gone, new content appears with two paragraphs
- **Verification:** `get_text` returns only the new content.

#### Scenario 7: CSP-Restricted Page

- **Given:** LinkedIn feed (CSP blocks execute_js)
- **When:** `interact({ what: "type", selector: "div.ql-editor", text: "This works despite CSP\n\nBecause content scripts bypass it" })`
- **Then:** Text is inserted successfully
- **Verification:** No CSP errors in response. `insertion_strategy` is `"quill_native"`.

### Regression Testing

- Run existing `dom-primitives-richtext.test.js` tests — all must pass
- Run full UAT script `scripts/uat/runners/test-all-tools-comprehensive.sh`
- Verify `fill_form` action still works for standard forms
- Verify single-line `type` into standard inputs is unaffected

### Performance Testing

- Measure time for inserting a 20-line post into a Quill editor
- Before: ~100s (20 round-trips at ~5s each)
- After: <1s (single command)
- Measure time for inserting a 100-line text block
- Should complete in <1s regardless of line count

## Test Status

- [ ] Unit tests written
- [ ] Unit tests passing
- [ ] UAT Scenario 1 (LinkedIn/Quill) passing
- [ ] UAT Scenario 2 (GitHub textarea) passing
- [ ] UAT Scenario 3 (generic contenteditable) passing
- [ ] UAT Scenario 4 (standard textarea regression) passing
- [ ] UAT Scenario 5 (paste action) passing
- [ ] UAT Scenario 6 (clear + replace) passing
- [ ] UAT Scenario 7 (CSP-restricted page) passing
- [ ] Full UAT script passing
- [ ] Performance target met (<1s for 20-line insertion)
