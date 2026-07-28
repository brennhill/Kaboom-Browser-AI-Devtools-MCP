# Interact Tool — Action Reference

59 actions + 6 aliases grouped by category.

**Targeting params** (most DOM actions): `selector`, `element_id`, `index`, `index_generation`, `nth`, `x`, `y`, `scope_selector`, `scope_rect`, `frame`

**Enrichment params:** `include_screenshot`, `include_interactive`, `action_diff`, `evidence` (off|on_mutation|always), `observe_mutations`, `wait_for_stable`, `stability_ms`, `analyze`, `subtitle`

**Dispatch params:** `what` (required), `telemetry_mode`, `background`, `reason`, `correlation_id`

---

# Navigation

## navigate
Navigate to a URL in the tracked tab or a new tab.
**Params:** `url` (string, required), `include_content` (bool), `new_tab` (bool), `analyze` (bool), `auto_dismiss` (bool), `wait_for_stable` (bool), `stability_ms` (number)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"navigate","url":"https://example.com"}'
```

## refresh
Reload the current page.
**Params:** `analyze` (bool)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"refresh"}'
```

## back
Navigate back in browser history.
**Params:** none
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"back"}'
```

## forward
Navigate forward in browser history.
**Params:** none
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"forward"}'
```

## new_tab
Open a new browser tab.
**Params:** `url` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"new_tab","url":"https://example.com"}'
```

## switch_tab
Switch to a different browser tab.
**Params:** `tab_id` (number), `tab_index` (number), `set_tracked` (bool)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"switch_tab","tab_index":2,"set_tracked":true}'
```

## close_tab
Close a browser tab.
**Params:** `tab_id` (number)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"close_tab","tab_id":123}'
```

## navigate_and_wait_for
Navigate to a URL and wait for a specific selector to appear.
**Params:** `url` (string, required), `wait_for` (string, required), `include_content` (bool)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"navigate_and_wait_for","url":"https://example.com","wait_for":"#main-content"}'
```

## navigate_and_document
Click an element to navigate and document the result.
**Params:** `selector` (string), `element_id` (string), `index` (number), `timeout_ms` (number), `wait_for_url_change` (bool), `wait_for_stable` (bool), `stability_ms` (number), `include_screenshot` (bool), `include_interactive` (bool)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"navigate_and_document","selector":"a.nav-link","include_screenshot":true}'
```

---

# DOM Interaction

## click
Click an element on the page.
**Params:** `selector` (string), `element_id` (string), `index` (number), `nth` (number), `scope_selector` (string), `frame` (string), `reason` (string), `correlation_id` (string), `timeout_ms` (number), `x` (number), `y` (number), `analyze` (bool), `wait_for_stable` (bool), `stability_ms` (number)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"click","selector":"button.submit"}'
```

## type
Type text into an input element.
**Params:** `text` (string, required), `selector` (string), `element_id` (string), `index` (number), `nth` (number), `scope_selector` (string), `frame` (string), `clear` (bool)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"type","selector":"#search","text":"hello world","clear":true}'
```

## select
Choose an option from a dropdown element.
**Params:** `value` (string, required), `selector` (string), `element_id` (string), `index` (number), `nth` (number), `scope_selector` (string), `frame` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"select","selector":"#country","value":"US"}'
```

## check
Toggle a checkbox or radio button.
**Params:** `selector` (string), `element_id` (string), `index` (number), `nth` (number), `scope_selector` (string), `frame` (string), `checked` (bool)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"check","selector":"#agree-terms","checked":true}'
```

## focus
Focus an element.
**Params:** `selector` (string), `element_id` (string), `index` (number), `nth` (number), `scope_selector` (string), `frame` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"focus","selector":"#email-input"}'
```

## hover
Hover over an element.
**Params:** `selector` (string), `element_id` (string), `index` (number), `nth` (number), `scope_selector` (string), `frame` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"hover","selector":".dropdown-trigger"}'
```

## scroll_to
Scroll an element into view or scroll the page directionally.
**Params:** `selector` (string), `element_id` (string), `direction` (string: top|bottom|up|down)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"scroll_to","direction":"bottom"}'
```

## key_press
Send keyboard key presses.
**Params:** `text` (string: Enter|Tab|Escape|Backspace|ArrowDown|ArrowUp|Space)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"key_press","text":"Enter"}'
```

## paste
Paste text via the clipboard into an element.
**Params:** `text` (string, required), `selector` (string), `element_id` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"paste","text":"pasted content","selector":"#editor"}'
```

## hardware_click
CDP-level click at exact viewport coordinates.
**Params:** `x` (number), `y` (number)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"hardware_click","x":150,"y":300}'
```

## highlight
Highlight an element with a visual overlay.
**Params:** `selector` (string), `element_id` (string), `duration_ms` (number)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"highlight","selector":".target-element","duration_ms":3000}'
```

---

# DOM Reading

## get_text
Read the text content of an element.
**Params:** `selector` (string), `element_id` (string), `structured` (bool)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"get_text","selector":"h1"}'
```

## get_value
Read the current value of an input element.
**Params:** `selector` (string), `element_id` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"get_value","selector":"#username"}'
```

## get_attribute
Read an HTML attribute from an element.
**Params:** `name` (string, required), `selector` (string), `element_id` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"get_attribute","selector":"a.link","name":"href"}'
```

## query
Query the DOM for existence, count, text, or attributes.
**Params:** `selector` (string), `query_type` (string: exists|count|text|text_all|attributes), `attribute_names` (array of strings)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"query","selector":".item","query_type":"count"}'
```

## set_attribute
Set an HTML attribute on an element.
**Params:** `name` (string, required), `selector` (string), `value` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"set_attribute","selector":"#logo","name":"alt","value":"Company Logo"}'
```

---

# Page Discovery

## explore_page
Composite page exploration returning screenshot, interactive elements, text, navigation, and metadata.
**Params:** `url` (string), `visible_only` (bool), `limit` (number)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"explore_page","visible_only":true,"limit":50}'
```

## list_interactive
List all clickable and typeable elements on the page.
**Params:** `visible_only` (bool), `frame` (string), `scope_selector` (string), `scope_rect` (object), `text_contains` (string), `role` (string), `exclude_nav` (bool), `limit` (number)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"list_interactive","visible_only":true,"role":"button"}'
```

## get_readable
Extract readable text content from the page.
**Params:** `frame` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"get_readable"}'
```

## get_markdown
Extract the page content as markdown.
**Params:** `frame` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"get_markdown"}'
```

---

# Forms

## fill_form
Fill multiple form fields at once.
**Params:** `fields` (array of `{selector, value, index}`, required), `scope_selector` (string), `frame` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"fill_form","fields":[{"selector":"#name","value":"Jane"},{"selector":"#email","value":"jane@example.com"}]}'
```

## fill_form_and_submit
Fill form fields and click the submit button.
**Params:** `fields` (array of `{selector, value, index}`), `submit_selector` (string), `submit_index` (number), `scope_selector` (string), `frame` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"fill_form_and_submit","fields":[{"selector":"#user","value":"admin"}],"submit_selector":"button[type=submit]"}'
```

---

# Wait & Stability

## wait_for
Wait for a selector, text, or URL pattern to appear.
**Params:** `selector` (string), `timeout_ms` (number), `frame` (string), `absent` (bool), `url_contains` (string), `text` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"wait_for","selector":".loaded","timeout_ms":5000}'
```

## wait_for_stable
Wait for the DOM to stabilize (no mutations for a period).
**Params:** `stability_ms` (number), `timeout_ms` (number)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"wait_for_stable","stability_ms":500,"timeout_ms":10000}'
```

## auto_dismiss_overlays
Automatically dismiss cookie banners, consent dialogs, and overlays.
**Params:** `timeout_ms` (number)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"auto_dismiss_overlays","timeout_ms":3000}'
```

---

# Dialog & Overlay

## confirm_top_dialog
Accept the top-most dialog or modal.
**Params:** none
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"confirm_top_dialog"}'
```

## dismiss_top_overlay
Close the top-most overlay or popover.
**Params:** none
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"dismiss_top_overlay"}'
```

## subtitle
Display a status subtitle in the extension UI.
**Params:** `text` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"subtitle","text":"Processing step 3 of 5..."}'
```

---

# State Management

## save_state
Snapshot cookies, storage, and/or URL into a named state.
**Params:** `snapshot_name` (string, required), `storage_type` (string), `include_url` (bool)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"save_state","snapshot_name":"logged_in","include_url":true}'
```

## load_state
Restore a previously saved state snapshot.
**Params:** `snapshot_name` (string, required), `storage_type` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"load_state","snapshot_name":"logged_in"}'
```

## list_states
List all saved state snapshots.
**Params:** none
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"list_states"}'
```

## delete_state
Delete a saved state snapshot.
**Params:** `snapshot_name` (string, required)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"delete_state","snapshot_name":"logged_in"}'
```

---

# Storage

## set_storage
Set a key in localStorage or sessionStorage.
**Params:** `key` (string, required), `storage_type` (string), `value` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"set_storage","key":"theme","value":"dark","storage_type":"local"}'
```

## delete_storage
Delete a key from localStorage or sessionStorage.
**Params:** `key` (string, required), `storage_type` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"delete_storage","key":"theme","storage_type":"local"}'
```

## clear_storage
Clear all keys from localStorage or sessionStorage.
**Params:** `storage_type` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"clear_storage","storage_type":"session"}'
```

## set_cookie
Set a browser cookie.
**Params:** `name` (string, required), `value` (string), `domain` (string), `path` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"set_cookie","name":"session_id","value":"abc123","domain":".example.com"}'
```

## delete_cookie
Delete a browser cookie.
**Params:** `name` (string, required), `domain` (string), `path` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"delete_cookie","name":"session_id","domain":".example.com"}'
```

---

# Clipboard

## clipboard_read
Read the current clipboard text content.
**Params:** none
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"clipboard_read"}'
```

## clipboard_write
Write text to the clipboard.
**Params:** `text` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"clipboard_write","text":"copied text"}'
```

---

# JavaScript

## execute_js
Run JavaScript in the page context.
**Params:** `script` (string, required), `world` (string: auto|main|isolated), `timeout_ms` (number)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"execute_js","script":"document.title","world":"main"}'
```

---

# Recording

## screen_recording_start
Start a video recording of the browser tab.
**Params:** `name` (string), `audio` (string: tab|mic|both), `fps` (number, 5-60, default 15)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"screen_recording_start","name":"test_run","fps":30}'
```

## screen_recording_stop
Stop a video recording.
**Params:** `name` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"screen_recording_stop","name":"test_run"}'
```

---

# File Upload

## upload
Upload a file to a file input or API endpoint.
**Params:** `file_path` (string), `api_endpoint` (string), `submit` (bool), `escalation_timeout_ms` (number)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"upload","file_path":"/tmp/document.pdf","submit":true}'
```

---

# Annotation

## draw_mode_start
Activate the annotation drawing overlay.
**Params:** `annot_session` (string), `timeout_ms` (number)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"draw_mode_start","annot_session":"review_1","timeout_ms":60000}'
```

## run_a11y_and_export_sarif
Run an accessibility audit and export results as SARIF.
**Params:** `save_to` (string), `scope_selector` (string), `frame` (string)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"run_a11y_and_export_sarif","save_to":"/tmp/a11y-report.sarif"}'
```

---

# Composer (Claude-specific)

## open_composer
Open the Claude composer interface.
**Params:** none
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"open_composer"}'
```

## submit_active_composer
Submit the message in the active Claude composer.
**Params:** none
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"submit_active_composer"}'
```

## activate_tab
Bring the tracked browser tab to the foreground.
**Params:** none
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"activate_tab"}'
```

---

# Batch

## batch
Execute a sequence of interact actions in order.
**Params:** `steps` (array of action objects, required), `step_timeout_ms` (number, default 10000), `continue_on_error` (bool, default true), `stop_after_step` (number)
**Example:**
```bash
bash scripts/kaboom-call.sh interact '{"what":"batch","steps":[{"what":"click","selector":"#login"},{"what":"type","selector":"#user","text":"admin"},{"what":"key_press","text":"Enter"}]}'
```

---

# Aliases

| Alias | Canonical Action |
|-------|-----------------|
