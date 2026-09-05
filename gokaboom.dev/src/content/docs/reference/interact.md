---
title: "Interact — Control the Browser"
description: "Complete reference for the interact tool. 59 actions for navigation, DOM manipulation, element discovery, tab management, storage & cookies, dialog handling, file upload, draw mode, recording, compound actions, content extraction, state management, and JavaScript execution."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['reference', 'interact']
---

The `interact` tool gives the AI control over the browser — navigate, click, type, read elements, run JavaScript, upload files, record sessions, manage state, and display narration. **Requires AI Web Pilot to be enabled** in the extension popup.

Need one runnable call + response shape + failure fix for every action? See [Interact Executable Examples](/reference/examples/interact-examples/).

:::note[Synchronous Mode]
Tools now block until the extension returns a result (up to 15s). Set `background: true` to return immediately with a `correlation_id`.
:::

<!-- Screenshot: Extension popup with AI Web Pilot toggle enabled -->

## Quick Reference

```js
interact({what: "navigate", url: "https://example.com"})
interact({what: "click", selector: "text=Submit"})
interact({what: "type", selector: "label=Email", text: "user@example.com"})
interact({what: "list_interactive"})
observe({what: "screenshot"})
interact({what: "execute_js", script: "document.title"})
interact({what: "batch", steps: [{what: "click", selector: "text=Menu"}, ...]})
interact({what: "set_cookie", name: "theme", value: "dark", domain: "example.com"})
```

## Common Parameters

These parameters can be added to **any** action:

| Parameter | Type | Description |
|-----------|------|-------------|
| `subtitle` | string | Narration text displayed at bottom of viewport. Empty string clears. |
| `reason` | string | Why this action is being performed — shown in the action toast |
| `correlation_id` | string | Link this action to a specific error or investigation |
| `analyze` | boolean | Enable performance profiling (returns perf_diff in result) |
| `tab_id` | number | Target a specific tab (omit for active tab) |
| `element_id` | string | Stable element handle from `list_interactive` (preferred for deterministic follow-up actions) |
| `index` | number | Element index from `list_interactive` results (legacy alternative to selector/element_id) |
| `index_generation` | string | Generation token from `list_interactive` to ensure index resolves against the same element snapshot |
| `scope_selector` | string | Container CSS selector to constrain DOM actions to a specific region |
| `include_screenshot` | boolean | Capture a screenshot after the action completes and return it inline |
| `evidence` | string | Visual evidence capture mode: `off` (default), `on_mutation`, `always` |
| `observe_mutations` | boolean | Track element-level DOM mutations during action execution |
| `wait_for_stable` | boolean | Wait for DOM stability (no mutations) before returning. Composable with `navigate` and `click`. |
| `frame` | string | Target iframe: CSS selector, 0-based index, or `"all"` |

### Subtitles

<!-- Screenshot: Subtitle overlay at bottom of browser viewport showing narration text -->

Subtitles are persistent narration text displayed at the bottom of the viewport, like closed captions. They stay visible until replaced or cleared.

```js
// Standalone
interact({what: "subtitle", text: "Creating a new project"})

// Composable — narration + action in one call
interact({what: "click", selector: "text=Create",
          subtitle: "One click to start a new project"})

// Clear
interact({what: "subtitle", text: ""})
```

### Action Toasts

<!-- Screenshot: Blue "trying" action toast at top of viewport showing "Click: Submit" -->
<!-- Screenshot: Green "success" action toast at top of viewport -->

When enabled, action toasts show brief notifications at the top of the viewport for each action. The `reason` parameter customizes the toast label:

```js
// Without reason: toast shows "Click: #submit-btn"
interact({what: "click", selector: "#submit-btn"})

// With reason: toast shows "Submit the registration form"
interact({what: "click", selector: "#submit-btn", reason: "Submit the registration form"})
```

Toasts can be toggled off in the extension popup — useful during demos.

---

## Semantic Selectors

All actions that accept a `selector` parameter support semantic selectors in addition to CSS:

| Syntax | Example | How it works |
|--------|---------|-------------|
| `text=` | `text=Submit` | Finds element whose text content contains "Submit" |
| `role=` | `role=button` | Finds `[role="button"]` |
| `placeholder=` | `placeholder=Email` | Finds `[placeholder="Email"]` |
| `label=` | `label=Username` | Finds label containing text, follows `for` attribute to target |
| `aria-label=` | `aria-label=Close` | Finds `[aria-label="Close"]` with starts-with matching |
| CSS | `#submit-btn` | Standard `document.querySelector()` |

Semantic selectors are **resilient to UI changes** — they target meaning, not structure. `text=Submit` still works after a CSS redesign, a framework migration, or a component library upgrade.

The `aria-label=` selector uses starts-with matching, so `aria-label=Send` matches Gmail's `"Send ‪(⌘Enter)‬"`.

### Iframe Targeting

Use the `frame` parameter to target elements inside iframes:

```js
interact({what: "click", selector: "text=Submit", frame: "#payment-iframe"})
interact({what: "type", selector: "label=Card", text: "4242...", frame: 0})
```

| Value | Behavior |
|-------|----------|
| CSS selector | Target iframe matching the selector |
| Number | Target iframe by 0-based index |
| `"all"` | Search all iframes |

---

## Navigation

### navigate

```js
interact({what: "navigate", url: "https://example.com"})
```

Navigates the active tab to the specified URL. Automatically includes `perf_diff` in the async result (before/after performance comparison).

| Parameter | Type | Description |
|-----------|------|-------------|
| `url` | string (required) | URL to navigate to |
| `include_content` | boolean | Return page content with response (url, title, text_content, vitals) |
| `new_tab` | boolean | Open URL in a background tab instead of replacing current tab |
| `auto_dismiss` | boolean | After navigation completes, automatically dismiss cookie consent banners and overlays |

### refresh

```js
interact({what: "refresh"})
```

Refreshes the current page. Includes `perf_diff` automatically.

### back / forward

```js
interact({what: "back"})
interact({what: "forward"})
```

Browser history navigation.

### new_tab

```js
interact({what: "new_tab", url: "https://example.com"})
```

Opens a new tab with the specified URL.

| Parameter | Type | Description |
|-----------|------|-------------|
| `url` | string (required) | URL to open in the new tab |

### switch_tab

Switch to a different browser tab.

```js
interact({what: "switch_tab", tab_id: 123})
interact({what: "switch_tab", tab_index: 2})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `tab_id` | number | Tab ID to switch to |
| `tab_index` | number | Tab index in current window ordering (alternative to tab_id) |
| `set_tracked` | boolean | Whether to update the tracked tab to the newly activated tab (default: true). Set to false to switch focus without changing which tab the server targets. |

### close_tab

Close a browser tab.

```js
interact({what: "close_tab", tab_id: 123})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `tab_id` | number | Tab ID to close (omit for active tab) |

### activate_tab

Activate (bring to front) a specific browser tab.

```js
interact({what: "activate_tab", tab_id: 123})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `tab_id` | number | Tab ID to activate |

---

## DOM Primitives

### click

```js
interact({what: "click", selector: "text=Submit"})
interact({what: "click", selector: "#confirm-btn", reason: "Confirm the order"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `selector` | string | CSS or semantic selector |

### type

```js
interact({what: "type", selector: "label=Email", text: "user@example.com"})
interact({what: "type", selector: "placeholder=Search", text: "wireless headphones", clear: true})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `selector` | string | CSS or semantic selector |
| `text` | string | Text to type |
| `clear` | boolean | Clear existing value before typing |

### select

```js
interact({what: "select", selector: "#country", value: "US"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `selector` | string | CSS or semantic selector for `<select>` element |
| `value` | string | Option value to select |

### check

```js
interact({what: "check", selector: "text=I agree to the Terms"})
interact({what: "check", selector: "#newsletter", checked: false})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `selector` | string | — | CSS or semantic selector |
| `checked` | boolean | true | Whether to check or uncheck |

### key_press

```js
interact({what: "key_press", selector: "label=Search", text: "Enter"})
interact({what: "key_press", text: "Escape"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `selector` | string | Element to target (optional — omit for page-level key press) |
| `text` | string | Key name: `Enter`, `Tab`, `Escape`, `Backspace`, `ArrowDown`, `ArrowUp`, `Space` |

### focus

```js
interact({what: "focus", selector: "label=Email"})
```

### hover

Hover over an element.

```js
interact({what: "hover", selector: "text=Products"})
interact({what: "hover", selector: ".dropdown-trigger"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `selector` | string | CSS or semantic selector |

### hardware_click

Click using OS-level hardware simulation instead of DOM events. Useful for elements that don't respond to synthetic clicks (e.g., custom canvas, embedded iframes with strict event handling).

```js
interact({what: "hardware_click", selector: "#canvas-element"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `selector` | string | CSS or semantic selector |

### scroll_to

```js
interact({what: "scroll_to", selector: "#pricing-section"})
```

### wait_for

Wait for an element to appear on the page. Useful for async content.

```js
interact({what: "wait_for", selector: "text=Dashboard", timeout_ms: 10000})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `selector` | string | — | CSS or semantic selector |
| `timeout_ms` | number | 5000 | Maximum wait time in milliseconds |

### wait_for_stable

Wait for the DOM to stop changing (no mutations) before returning. Useful after dynamic content loads.

```js
interact({what: "wait_for_stable"})
interact({what: "wait_for_stable", stability_ms: 1000})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `stability_ms` | number | 500 | Milliseconds of DOM quiet time required |

---

## Element Reading

### get_text

```js
interact({what: "get_text", selector: ".order-total"})
```

Returns the text content of the matched element.

### get_value

```js
interact({what: "get_value", selector: "input[name='email']"})
```

Returns the current value of a form element.

### get_attribute

```js
interact({what: "get_attribute", selector: "#submit", name: "disabled"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Attribute name to read |

### query

Run selector queries without mutating page state.

```js
interact({what: "query", selector: "text=Submit", query_type: "exists"})
interact({what: "query", selector: ".error", query_type: "count"})
interact({what: "query", selector: "#status", query_type: "text"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `query_type` | string | `exists`, `count`, `text`, `text_all`, or `attributes` |
| `attribute_names` | array | Attribute names when `query_type` is `attributes` |

### set_attribute

```js
interact({what: "set_attribute", selector: "#submit", name: "disabled", value: "false"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Attribute name |
| `value` | string | Value to set |

### list_interactive

Discovers all interactive elements on the page — buttons, links, inputs, selects, etc. Returns up to 100 elements with suggested selectors.

```js
interact({what: "list_interactive"})
interact({what: "list_interactive", limit: 20, visible_only: true})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `limit` | number | Max elements to return (default: all) |
| `visible_only` | boolean | Only return visible elements |

Returns an array of elements, each with:

| Field | Description |
|-------|-------------|
| `tag` | Element tag name (`button`, `input`, `a`, etc.) |
| `type` | Input type (`email`, `password`, `text`) — only for `<input>` |
| `selector` | Suggested selector (semantic preferred: `text=`, `aria-label=`, `placeholder=`) |
| `label` | Human-readable label (aria-label > title > placeholder > text content) |
| `role` | ARIA role |
| `visible` | Whether the element is visible on screen |

This is the best way to discover what's clickable on an unfamiliar page. The AI can read the list and choose the right selector.

### find

Finds elements by natural-language description using the accessibility tree, so it reaches controls
no CSS selector can name — canvas widgets, ARIA-only semantics, and components whose class names
change on every build.

```js
interact({what: "find", query: "add to cart button"})
interact({what: "find", query: "search bar"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `query` | string | **Required.** A natural-language description of the element, matched against role + accessible name |

Returns ranked candidates, each with:

| Field | Description |
|-------|-------------|
| `ref` | Accessibility reference, usable as a target in a following action |
| `role` | ARIA role of the candidate |
| `name` | Accessible name |
| `confidence` | How well the candidate matched the query |
| `why` | What matched — role, name, or both |

More than one candidate means the query was ambiguous. Disambiguate it rather than taking the
first result: `find` ranks, it does not decide.

---

## JavaScript Execution

### execute_js

Run arbitrary JavaScript in the page context. See [How KaBOOM Executes Page Scripts](/execute-scripts/) for the full deep dive.

```js
interact({what: "execute_js", script: "document.title"})
interact({what: "execute_js", script: "window.location.href"})
interact({what: "execute_js",
          script: "document.querySelectorAll('li').length",
          world: "main"})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `script` | string | — | JavaScript code to execute |
| `world` | string | `auto` | `auto` (try main, fallback to isolated), `main` (page globals), `isolated` (CSP-proof) |
| `timeout_ms` | number | 5000 | Maximum execution time |

---

## Visual

### highlight

Highlights an element on the page with a visual overlay. Useful for demos and debugging.

```js
interact({what: "highlight", selector: "#error-message"})
interact({what: "highlight", selector: "text=Submit", duration_ms: 3000})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `selector` | string | — | Element to highlight |
| `duration_ms` | number | 5000 | How long to show the highlight |

<!-- Screenshot: Element highlighted with KaBOOM's visual overlay -->

### subtitle

Display persistent narration text at the bottom of the viewport.

```js
interact({what: "subtitle", text: "Welcome to the product tour"})
interact({what: "subtitle", text: ""})  // Clear
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `text` | string (required) | Narration text. Empty string clears. |

<!-- Screenshot: Subtitle overlay showing narration text at bottom of viewport -->

---

## Environment Pinning

Deterministic replay needs the environment held still. Pinning is **opt-in per session** and
everything pinned is reported in the generated reproduction artifact — a test that silently
depends on a pinned clock passes only on the machine that recorded it.

### pin_environment

```js
interact({what: "pin_environment", environment: {
  clock_epoch_ms: 1767225600000,
  timezone_id: "UTC",
  random_seed: "run-42"
}})
```

| `environment` key | Type | Description |
|-------------------|------|-------------|
| `clock_epoch_ms` | number | Fix the clock's origin to this epoch, in milliseconds |
| `timezone_id` | string | IANA timezone, e.g. `UTC` or `America/New_York` |
| `virtual_time_policy` | string | `advance` (default) fixes the clock's origin and lets it run. `pause` stops time outright, which freezes the page you are recording — use it for replay only. `pauseIfNetworkFetchesPending` is the third option. |
| `latitude`, `longitude`, `accuracy_m` | number | Geolocation override; accuracy defaults to 1 metre |
| `viewport_width`, `viewport_height` | number | Device metrics in CSS pixels |
| `device_scale_factor` | number | Device pixel ratio (default 1) |
| `mobile` | boolean | Emulate a mobile device |
| `random_seed` | string | Seed `Math.random` and `crypto.getRandomValues` so a replay draws the same values |

Returns what was actually pinned, and any knob the browser refused. **The refused ones are what a
replay will diverge on**, so they are named rather than dropped. A seed is only reported as pinned
once the page confirms the patch installed — `early-patch` does not run on cloaked domains or where
CSP blocked injection, and declaring determinism a run never had is worse than declaring none.

### unpin_environment

```js
interact({what: "unpin_environment"})
```

Releases every override `pin_environment` installed. Returns whether the tab was pinned, and any
override the browser refused to release.

Note that a navigation clears CDP overrides on its own, so the pin is stamped per recorded action
rather than once per session: a single session-level record would claim a pin over steps taken
after it lapsed.

---

## State Management

### save_state

Save the current browser state (URL, title, tab) as a named checkpoint.

```js
interact({what: "save_state", snapshot_name: "logged-in"})
```

### load_state

Restore a saved state checkpoint.

```js
interact({what: "load_state", snapshot_name: "logged-in"})
interact({what: "load_state", snapshot_name: "logged-in", include_url: true})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `snapshot_name` | string | — | Name of the saved state |
| `include_url` | boolean | false | Navigate to the saved URL when restoring |

### list_states

```js
interact({what: "list_states"})
```

Returns all saved states with metadata (name, URL, title, saved_at).

### delete_state

```js
interact({what: "delete_state", snapshot_name: "old-checkpoint"})
```

---

## Browser Storage

### set_storage

Set a key in localStorage or sessionStorage.

```js
interact({what: "set_storage", storage_type: "localStorage", key: "theme", value: "dark"})
interact({what: "set_storage", storage_type: "sessionStorage", key: "debug", value: "true"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `storage_type` | string | `localStorage` or `sessionStorage` |
| `key` | string | Storage key |
| `value` | string | Value to set |

### delete_storage

Remove a key from localStorage or sessionStorage.

```js
interact({what: "delete_storage", storage_type: "localStorage", key: "theme"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `storage_type` | string | `localStorage` or `sessionStorage` |
| `key` | string | Storage key to delete |

### clear_storage

Clear all keys from localStorage or sessionStorage.

```js
interact({what: "clear_storage", storage_type: "localStorage"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `storage_type` | string | `localStorage` or `sessionStorage` |

### set_cookie

Set a browser cookie.

```js
interact({what: "set_cookie", name: "session_id", value: "abc123", domain: "example.com"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Cookie name |
| `value` | string | Cookie value |
| `domain` | string | Cookie domain |
| `path` | string | Cookie path (default: `/`) |

### delete_cookie

Delete a browser cookie.

```js
interact({what: "delete_cookie", name: "session_id", domain: "example.com"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Cookie name |
| `domain` | string | Cookie domain |
| `path` | string | Cookie path (default: `/`) |

---

## Dialog Handling

### confirm_top_dialog

Accept (confirm/OK) the topmost browser dialog (alert, confirm, prompt).

```js
interact({what: "confirm_top_dialog"})
```

### dismiss_top_overlay

Dismiss the topmost overlay, modal, or popup on the page.

```js
interact({what: "dismiss_top_overlay"})
```

### auto_dismiss_overlays

Automatically detect and dismiss cookie consent banners, newsletter popups, and other overlays.

```js
interact({what: "auto_dismiss_overlays"})
```

---

## File Upload

### upload

Upload a file via native file dialogs. Works with `<input type="file">` elements and drag-and-drop zones.

```js
interact({what: "upload", selector: "input[type='file']", file_path: "/path/to/report.pdf"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `selector` | string | File input element selector |
| `file_path` | string | Absolute path to the file to upload |
| `name` | string | Filename (alternative to file_path) |
| `api_endpoint` | string | API endpoint for direct upload mode |
| `submit` | boolean | Submit form after upload |

---

## Draw Mode

### draw_mode_start

Activate the visual annotation overlay. Users draw rectangles on the page and type feedback. Press ESC to finish. Retrieve annotations with `analyze({what: "annotations"})`.

```js
interact({what: "draw_mode_start"})
interact({what: "draw_mode_start", annot_session: "checkout-review", wait: true, timeout_ms: 300000})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `annot_session` | string | — | Named session for multi-page annotation review |
| `wait` | boolean | — | Block until the user finishes drawing (press ESC) |
| `timeout_ms` | number | 300000 | Max wait time (only with `wait: true`, max 600000) |

---

## Recording

### screen_recording_start

Start video recording of the current tab.

```js
interact({what: "screen_recording_start", name: "checkout-flow", audio: "tab", fps: 30})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `name` | string | Auto-generated | Recording name |
| `audio` | string | — | Audio source: `tab`, `mic`, or `both`. Omit for video-only. |
| `fps` | number | 15 | Frames per second (5-60) |

### screen_recording_stop

Stop the active video recording.

```js
interact({what: "screen_recording_stop", name: "checkout-flow"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Recording name (if multiple active) |

## Clipboard

### clipboard_read

Read the current clipboard text.

```js
interact({what: "clipboard_read"})
```

### clipboard_write

Write text to the clipboard.

```js
interact({what: "clipboard_write", text: "Deploy status: green"})
```

### paste

Paste text at the currently focused element.

```js
interact({what: "paste", text: "Hello, world!"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `text` | string | Text to paste |

---

## Composer

### open_composer

Open the page's native compose/new-message UI (e.g., Gmail's compose window).

```js
interact({what: "open_composer"})
```

### submit_active_composer

Submit the currently active compose form.

```js
interact({what: "submit_active_composer"})
```

---

## Content Extraction

### get_readable

Extract the page content as readable text (similar to reader mode).

```js
interact({what: "get_readable"})
```

### get_markdown

Extract the page content as markdown.

```js
interact({what: "get_markdown"})
```

---

## Compound Actions

These actions combine multiple steps into a single call, reducing round-trips.

### navigate_and_wait_for

Navigate to a URL and wait for a specific element to appear before returning.

```js
interact({what: "navigate_and_wait_for", url: "https://example.com/dashboard", wait_for: ".dashboard-loaded"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `url` | string (required) | URL to navigate to |
| `wait_for` | string | CSS selector to wait for after navigation |
| `timeout_ms` | number | Max wait time for the selector |

### navigate_and_document

Navigate and return a structured page snapshot in one call.

```js
interact({what: "navigate_and_document", url: "https://example.com/docs"})
interact({what: "navigate_and_document", url: "https://example.com/docs", include_screenshot: true})
```

### fill_form

Fill multiple form fields without submitting. Use when you need to fill fields and then perform additional actions before submission.

```js
interact({what: "fill_form",
          fields: [
            {selector: "label=Email", value: "user@example.com"},
            {selector: "label=Name", value: "Jane Doe"}
          ]})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `fields` | array | Form fields: `[{selector, value, index?}]` |

### fill_form_and_submit

Fill multiple form fields and submit the form in a single action.

```js
interact({what: "fill_form_and_submit",
          fields: [
            {selector: "label=Email", value: "user@example.com"},
            {selector: "label=Password", value: "password123"}
          ],
          submit_selector: "text=Sign In"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `fields` | array | Form fields: `[{selector, value, index?}]` |
| `submit_selector` | string | CSS or semantic selector for submit button |
| `submit_index` | number | Submit button index from `list_interactive` (alternative to selector) |

### run_a11y_and_export_sarif

Run an accessibility audit and export results as SARIF in one step.

```js
interact({what: "run_a11y_and_export_sarif", save_to: "/tmp/a11y.sarif"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `save_to` | string | File path to save the SARIF report |

### explore_page

Autonomous page exploration — discovers interactive elements, follows links, and maps the page structure.

```js
interact({what: "explore_page"})
```

### batch

Execute multiple interact actions sequentially in a single call. Reduces round-trips for multi-step flows.

```js
interact({what: "batch",
          steps: [
            {what: "navigate", url: "https://example.com/login"},
            {what: "type", selector: "label=Email", text: "user@example.com"},
            {what: "type", selector: "label=Password", text: "password123"},
            {what: "click", selector: "text=Sign In"}
          ]})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `steps` | array (required) | — | Ordered list of interact action objects |
| `step_timeout_ms` | number | 10000 | Timeout per step |
| `continue_on_error` | boolean | true | Continue executing remaining steps after a failure |
| `stop_after_step` | number | — | Stop execution after this many steps |

---

## Pointer Gestures

Hardware-level pointer input dispatched through the Chrome DevTools Protocol, so the events carry
`isTrusted: true`. When CDP cannot attach (an internal page, a tab held by a performance trace, no
debugger permission) each gesture falls back to synthetic DOM events; the result's
`insertion_strategy` says which path ran.

Every gesture accepts `modifiers` — `ctrl`, `shift`, `alt`, `cmd` (meta), combinable — which also
works on plain `click`.

### drag

Drag along a route with the left button held: reorder a list, move a canvas object, resize a pane,
or complete a drag-and-drop upload.

```js
interact({what: "drag",
          drag_path: [{x: 120, y: 300}, {x: 260, y: 300}, {x: 420, y: 300}]})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `drag_path` | array (required) | Ordered viewport points, at least 2. The route to follow, not just the endpoints — drag libraries begin on the first intermediate move, so add waypoints to trace a curve. Named `drag_path` because `path` is the cookie path. |
| `modifiers` | array | Modifier keys held for the whole drag |

### right_click

Right-click to open a context menu. Raises a real `contextmenu` event, which `element.click()`
cannot. The result's `context_menu` field says whether the event reached the page.

```js
interact({what: "right_click", selector: "text=Report row"})
```

### double_click

Double-click an element or coordinate. Sent as one burst with `clickCount: 2`, so the page
receives `dblclick` — two separate clicks never coalesce into one.

```js
interact({what: "double_click", selector: ".cell[data-col='name']"})
```

### triple_click

Triple-click to select a whole line or paragraph before replacing it.

```js
interact({what: "triple_click", selector: "#bio"})
interact({what: "type", selector: "#bio", text: "Replacement text"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `selector` / `element_id` / `index` | — | Target element (right_click, double_click, triple_click) |
| `x`, `y` | number | Viewport coordinates, as an alternative to a selector |
| `modifiers` | array | Modifier keys held during the click |

### hover_at

Move the pointer to a viewport coordinate to reveal a tooltip or hover state where no element can
be named — canvas charts, map pins, SVG regions. Use `hover` with a selector for ordinary elements.

```js
interact({what: "hover_at", x: 640, y: 380})
```

### scroll_at

Send wheel scrolling at a viewport coordinate. Scrolls the pane under the pointer rather than the
page, which is what an inner scroll container or a virtualized list needs.

```js
interact({what: "scroll_at", x: 400, y: 500, delta_y: 600})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `x`, `y` | number (required) | Viewport coordinates to act at |
| `delta_x` | number | Horizontal wheel delta in pixels; positive scrolls right (scroll_at) |
| `delta_y` | number | Vertical wheel delta in pixels; positive scrolls down (scroll_at) |
| `modifiers` | array | Modifier keys held during the gesture |

---

## Region Capture

### zoom_region

Capture one rectangle of the viewport, optionally supersampled, to read detail that a full-page
screenshot renders illegibly — a chart axis, a licence plate in a photo, small print in a table.
The image comes back inline and is also written to the screenshots directory.

```js
interact({what: "zoom_region", x: 320, y: 180, width: 400, height: 220, scale: 2})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `x`, `y` | number (required) | — | Top-left corner of the region, in viewport pixels |
| `width`, `height` | number (required) | — | Size of the region in pixels |
| `scale` | number | 1 | Supersampling factor, 0-4. Use 2 to render the region at twice its on-screen size. |

`x` and `y` are **viewport** pixels — the same coordinates `click`, `hover_at` and
`scroll_at` take, and the same ones an `observe screenshot` coordinate frame
produces. They are translated to the document coordinates Chrome clips by, so a
region is captured where you pointed regardless of how far the page is scrolled.
The result reports both: `clip` is what you asked for, `page_clip` is what Chrome
was given.

The result also carries a `coordinate_frame` in the same shape
[`observe screenshot`](/reference/observe/#clicking-what-you-can-see-coordinate_frame)
returns, so a detail read off the enlarged image can be clicked directly without
undoing the supersampling by hand:

```js
// Zoom a chart legend at 2x, then click an entry read off the zoomed image.
const zoom = interact({what: "zoom_region", x: 320, y: 180, width: 400, height: 220, scale: 2})
// zoom.coordinate_frame.image_to_viewport = {scale_x: 0.5, scale_y: 0.5, offset_x: 320, offset_y: 180}
// An entry at image pixel (160, 60) is at viewport (400, 210):
interact({what: "click", x: 400, y: 210})
```

When the page's metrics cannot be read the capture still succeeds and the result
carries `coordinate_frame_error` instead of a frame — an unusable frame is never
returned in place of a good one.

---

## Performance Profiling

Add `analyze: true` to any DOM action to get a `perf_diff` in the result — before/after timing comparison with Web Vitals ratings and a verdict.

```js
interact({what: "click", selector: "text=Load Dashboard", analyze: true})
```

The `navigate` and `refresh` actions include `perf_diff` automatically.
