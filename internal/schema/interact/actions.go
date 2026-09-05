// Purpose: Defines the canonical list of interact tool action values for the what enum.
// Why: Centralizes the action enum so schema definition and action dispatch share a single source.
package interact

// ActionSpec defines per-action metadata used across schema + runtime capability docs.
// Keep this as the single source of truth for interact action surface metadata.
type ActionSpec struct {
	Name string
	// Hint says what the action DOES; Returns says what the RESPONSE CONTAINS.
	Hint     string
	Returns  string
	Required []string
	Optional []string
}

// actionSpecs is the canonical interact action registry.
// Fields are consumed by:
// - interact schema enum (`what`)
// - describe_capabilities interact mode specs
var actionSpecs = []ActionSpec{
	{Name: "highlight", Hint: "Visually highlight an element with a colored overlay", Returns: "Confirmation the overlay was drawn. No page data.", Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "duration_ms"}},
	{Name: "subtitle", Hint: "Display a status subtitle in the extension UI", Returns: "Confirmation the subtitle was displayed.", Optional: []string{"text"}},
	{Name: "save_state", Hint: "Snapshot cookies/storage/URL for later restore", Returns: "Confirmation the snapshot was stored, with what it captured.", Required: []string{"snapshot_name"}, Optional: []string{"storage_type", "include_url"}},
	{Name: "load_state", Hint: "Restore a previously saved state snapshot", Returns: "Confirmation the snapshot was restored.", Required: []string{"snapshot_name"}, Optional: []string{"storage_type"}},
	{Name: "list_states", Hint: "List all saved state snapshots", Returns: "A list of saved snapshot NAMES. Names only."},
	{Name: "delete_state", Hint: "Delete a saved state snapshot", Returns: "Confirmation the snapshot was deleted.", Required: []string{"snapshot_name"}},
	{Name: "set_storage", Hint: "Set a localStorage or sessionStorage key", Returns: "Confirmation the key was written.", Required: []string{"key"}, Optional: []string{"storage_type", "value"}},
	{Name: "delete_storage", Hint: "Delete a storage key", Returns: "Confirmation the key was removed.", Required: []string{"key"}, Optional: []string{"storage_type"}},
	{Name: "clear_storage", Hint: "Clear all keys from a storage type", Returns: "Confirmation the store was emptied, and which one.", Optional: []string{"storage_type"}},
	{Name: "set_cookie", Hint: "Set a browser cookie", Returns: "Confirmation the cookie was set.", Required: []string{"name"}, Optional: []string{"value", "domain", "path"}},
	{Name: "delete_cookie", Hint: "Delete a browser cookie", Returns: "Confirmation the cookie was removed.", Required: []string{"name"}, Optional: []string{"domain", "path"}},
	{Name: "execute_js", Hint: "Run JavaScript in the page context", Returns: "Your script's return value, and which JavaScript world it ran in.", Required: []string{"script"}, Optional: []string{"world", "timeout_ms"}},
	{Name: "navigate", Hint: "Navigate to a URL", Returns: "The URL actually landed on after any redirects.", Required: []string{"url"}, Optional: []string{"include_content", "new_tab", "analyze", "auto_dismiss", "wait_for_stable", "stability_ms"}},
	{Name: "refresh", Hint: "Reload the current page", Returns: "The URL after reload, and whether CSP restricted the page.", Optional: []string{"analyze"}},
	{Name: "back", Hint: "Browser back button", Returns: "The URL after going back."},
	{Name: "forward", Hint: "Browser forward button", Returns: "The URL after going forward."},
	{Name: "new_tab", Hint: "Open a new browser tab", Returns: "The new tab's id — keep it if you plan to switch to or close that tab.", Optional: []string{"url"}},
	{Name: "switch_tab", Hint: "Switch to a different browser tab", Returns: "Confirmation of which tab is now tracked.", Optional: []string{"tab_id", "tab_index", "set_tracked"}},
	{Name: "close_tab", Hint: "Close a browser tab", Returns: "Confirmation of which tab was closed.", Optional: []string{"tab_id"}},
	{Name: "click", Hint: "Click an element named by selector, ref or element_id, or a viewport coordinate (x/y in CSS pixels, as produced by a screenshot's coordinate_frame). Name exactly one target", Returns: "Confirmation the element was clicked, and the URL if it navigated.", Optional: []string{"selector", "element_id", "ref", "index", "nth", "scope_selector", "frame", "reason", "correlation_id", "timeout_ms", "x", "y", "modifiers", "analyze", "wait_for_stable", "stability_ms"}},
	{Name: "type", Hint: "Type text into an input or textarea", Returns: "Confirmation the text was typed.", Required: []string{"text"}, Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "clear"}},
	{Name: "select", Hint: "Choose an option in a <select> dropdown", Returns: "Confirmation the option was chosen.", Required: []string{"value"}, Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame"}},
	{Name: "check", Hint: "Toggle a checkbox or radio button", Returns: "Confirmation the box or radio was toggled.", Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "checked"}},
	{Name: "get_text", Hint: "Read text content of an element", Returns: "The element's visible text.", Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "structured"}},
	{Name: "get_value", Hint: "Read value of an input element", Returns: "The input's current value.", Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame"}},
	{Name: "get_attribute", Hint: "Read an HTML attribute from an element", Returns: "The attribute's value.", Required: []string{"name"}, Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame"}},
	{Name: "query", Hint: "Query DOM elements: check existence, count, read text or attributes without screenshots", Returns: "Whether the selector matches, how many, and the text or attributes you asked for.", Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "query_type", "attribute_names"}},
	{Name: "set_attribute", Hint: "Set an HTML attribute on an element", Returns: "Confirmation the attribute was set.", Required: []string{"name"}, Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "value"}},
	{Name: "focus", Hint: "Focus an element", Returns: "Confirmation the element was focused.", Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame"}},
	{Name: "scroll_to", Hint: "Scroll an element into view, or scroll container directionally (direction='top'|'bottom'|'up'|'down')", Returns: "Confirmation the scroll happened.", Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "direction"}},
	{Name: "wait_for", Hint: "Wait until a selector appears (or disappears with absent=true), text appears, or URL contains a substring", Returns: "Whether the condition was met before the timeout.", Optional: []string{"selector", "timeout_ms", "frame", "absent", "url_contains", "text"}},
	{Name: "key_press", Hint: "Send keyboard keys (Enter, Tab, Escape, shortcuts)", Returns: "Confirmation the keys were sent.", Optional: []string{"text"}},
	{Name: "paste", Hint: "Paste text into an element via clipboard", Returns: "Confirmation the text was pasted.", Required: []string{"text"}, Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame"}},
	{Name: "open_composer", Hint: "Open the Claude composer interface", Returns: "Confirmation the composer opened."},
	{Name: "submit_active_composer", Hint: "Submit the active Claude composer message", Returns: "Confirmation the message was submitted."},
	{Name: "confirm_top_dialog", Hint: "Accept/confirm the top-most dialog or modal", Returns: "Confirmation the dialog was accepted."},
	{Name: "dismiss_top_overlay", Hint: "Dismiss/close the top-most overlay or popover", Returns: "Confirmation the overlay was closed."},
	{Name: "hover", Hint: "Trigger hover state on an element for tooltip discovery", Returns: "Confirmation the hover was triggered.", Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame"}},
	{Name: "auto_dismiss_overlays", Hint: "Auto-dismiss cookie consent banners and overlays using known framework selectors", Returns: "How many consent banners and overlays were dismissed.", Optional: []string{"timeout_ms"}},
	{Name: "wait_for_stable", Hint: "Wait for DOM stability (no mutations for stability_ms). Returns stable/timed_out status", Returns: "Whether the DOM settled, how long it took and how many mutations were seen.", Optional: []string{"stability_ms", "timeout_ms"}},
	{Name: "list_interactive", Hint: "List all clickable/typeable elements on the page. Use limit to cap results", Returns: "A list of clickable and typeable elements: id, label, type, selector. Lean by default; verbose=true adds geometry and landmark context.", Optional: []string{"visible_only", "frame", "scope_selector", "scope_rect", "text_contains", "role", "exclude_nav", "limit"}},
	{Name: "get_readable", Hint: "Extract readable text content from the page", Returns: "The page's main article text, stripped of navigation and chrome.", Optional: []string{"frame"}},
	{Name: "get_markdown", Hint: "Extract page content as markdown", Returns: "The page content converted to markdown text.", Optional: []string{"frame"}},
	{Name: "navigate_and_wait_for", Hint: "Navigate to a URL and wait for a selector to appear", Returns: "Confirmation both the navigation and the wait succeeded, stage by stage.", Required: []string{"url", "wait_for"}, Optional: []string{"include_content"}},
	{Name: "navigate_and_document", Hint: "Click to navigate, optionally wait for URL change/stability, then return page context", Returns: "The URL after clicking, plus what the destination page contains.", Optional: []string{"selector", "element_id", "index", "index_generation", "nth", "scope_selector", "scope_rect", "frame", "tab_id", "reason", "timeout_ms", "wait_for_url_change", "wait_for_stable", "stability_ms", "include_screenshot", "include_interactive"}},
	{Name: "fill_form_and_submit", Hint: "Fill form fields and click the submit button", Returns: "Per-field fill results and the submit outcome.", Optional: []string{"fields", "submit_selector", "submit_index", "scope_selector", "frame"}},
	{Name: "fill_form", Hint: "Fill multiple form fields at once", Returns: "Per-field fill results. Nothing is submitted.", Optional: []string{"fields", "scope_selector", "frame"}},
	{Name: "run_a11y_and_export_sarif", Hint: "Run accessibility audit and export results as SARIF", Returns: "Confirmation of each stage, and where the SARIF was written.", Optional: []string{"save_to", "scope_selector", "frame"}},
	{Name: "screen_recording_start", Hint: "Start recording browser session with video capture", Returns: "Confirmation recording began, or that it is awaiting your approval in the browser.", Optional: []string{"name", "audio", "fps"}},
	{Name: "screen_recording_stop", Hint: "Stop recording and save the session", Returns: "Where the video FILE was saved. A path, not the video bytes.", Optional: []string{"name"}},
	{Name: "upload", Hint: "Upload a file to a file input or API endpoint", Returns: "Confirmation the file was attached, with its name, size and type.", Optional: []string{"file_path", "api_endpoint", "submit", "escalation_timeout_ms"}},
	{Name: "draw_mode_start", Hint: "Activate annotation overlay for drawing rectangles and adding feedback", Returns: "Confirmation draw mode opened, with the session id to read annotations back.", Optional: []string{"annot_session", "timeout_ms"}},
	{Name: "activate_tab", Hint: "Bring the tracked tab to the foreground", Returns: "Confirmation the tab was brought to the foreground."},
	{Name: "explore_page", Hint: "Composite page exploration: screenshot, interactive elements, readable text, navigation links, and metadata in one call", Returns: "The page identity, its readable text, its links and its interactive elements in one response.", Optional: []string{"url", "visible_only", "limit"}},
	{Name: "batch", Hint: "Execute a sequence of interact actions in one call", Returns: "Per-step results for the whole batch, and how many succeeded, failed or queued.", Optional: []string{"steps", "step_timeout_ms", "continue_on_error", "stop_after_step"}},
	{Name: "clipboard_read", Hint: "Read current clipboard text content", Returns: "The clipboard text, or a named reason the browser refused."},
	{Name: "clipboard_write", Hint: "Write text to the clipboard", Returns: "Confirmation the text was copied.", Optional: []string{"text"}},
	{Name: "find", Hint: "Find elements by natural-language description using the accessibility tree (works where selectors cannot: canvas widgets, ARIA-only semantics)", Returns: "Ranked candidate elements with ref, role, accessible name, confidence and why it matched, plus an index_generation for the snapshot they came from — pass ref and index_generation together to click, right_click, double_click or triple_click. Multiple candidates mean the query was ambiguous — disambiguate rather than taking the first.", Required: []string{"query"}},
	// Environment pinning (kaboom-x0li.2). Opt-in per session: an unpinned tab records nothing
	// about its environment, so a generated test says outright that it inherits the machine's.
	{Name: "pin_environment", Hint: "Hold the tab's environment still for deterministic replay: clock, timezone, geolocation, viewport and a seeded Math.random/crypto", Returns: "What was actually pinned, and any knob the browser refused — the refused ones are what a replay will diverge on.", Required: []string{"environment"}},
	{Name: "unpin_environment", Hint: "Release every override pin_environment installed", Returns: "Whether the tab was pinned, and any override the browser refused to release."},
	// Pointer gestures (kaboom-05ue.5). Each dispatches hardware-level input through CDP when the
	// tab allows it and falls back to synthetic DOM events when it does not, so the evidence field
	// insertion_strategy says which one the page actually saw.
	{Name: "drag", Hint: "Drag along a route with the left button held: reorder a list, move a canvas object, resize a pane, or complete a drag-and-drop upload", Returns: "How many route points were sent, how many pointer moves were dispatched, and whether the HTML5 drag-and-drop events landed too.", Required: []string{"drag_path"}, Optional: []string{"modifiers", "reason", "correlation_id", "timeout_ms", "analyze", "wait_for_stable", "stability_ms"}},
	{Name: "right_click", Hint: "Right-click to open a context menu (raises a real contextmenu event, which element.click() cannot)", Returns: "Confirmation of the click, and whether the contextmenu event reached the page.", Optional: []string{"selector", "element_id", "ref", "index", "nth", "scope_selector", "frame", "x", "y", "modifiers", "reason", "correlation_id", "timeout_ms"}},
	{Name: "double_click", Hint: "Double-click an element or coordinate (one burst with clickCount=2, so the page receives dblclick)", Returns: "Confirmation the double click was dispatched, with the click count the page saw.", Optional: []string{"selector", "element_id", "ref", "index", "nth", "scope_selector", "frame", "x", "y", "modifiers", "reason", "correlation_id", "timeout_ms"}},
	{Name: "triple_click", Hint: "Triple-click to select a whole line or paragraph before replacing it", Returns: "Confirmation the triple click was dispatched, with the click count the page saw.", Optional: []string{"selector", "element_id", "ref", "index", "nth", "scope_selector", "frame", "x", "y", "modifiers", "reason", "correlation_id", "timeout_ms"}},
	{Name: "hover_at", Hint: "Move the pointer to a viewport coordinate to reveal a tooltip or hover state where no element can be named (canvas charts, map pins)", Returns: "Confirmation the pointer moved, and the coordinate it moved to. Use observe screenshot to see what appeared.", Required: []string{"x", "y"}, Optional: []string{"modifiers", "reason", "correlation_id"}},
	{Name: "scroll_at", Hint: "Send wheel scrolling at a viewport coordinate — scrolls the pane under the pointer rather than the page", Returns: "The wheel deltas that were dispatched, and where.", Required: []string{"x", "y"}, Optional: []string{"delta_x", "delta_y", "modifiers", "reason", "correlation_id"}},
	{Name: "zoom_region", Hint: "Capture one rectangle of the viewport, optionally supersampled, to read detail a full-page screenshot renders illegibly", Returns: "The captured region as an image, plus where the PNG was saved. No page text or element data.", Required: []string{"x", "y", "width", "height"}, Optional: []string{"scale", "correlation_id"}},
}

// actionEnum is the canonical list of values accepted by the 'what' parameter.
var actionEnum = actionNames(actionSpecs)

func actionNames(specs []ActionSpec) []string {
	out := make([]string, 0, len(specs))
	for _, spec := range specs {
		out = append(out, spec.Name)
	}
	return out
}

// ActionSpecs returns a defensive copy of canonical interact action specs.
func ActionSpecs() []ActionSpec {
	out := make([]ActionSpec, 0, len(actionSpecs))
	for _, spec := range actionSpecs {
		out = append(out, ActionSpec{
			Name:     spec.Name,
			Hint:     spec.Hint,
			Returns:  spec.Returns,
			Required: append([]string(nil), spec.Required...),
			Optional: append([]string(nil), spec.Optional...),
		})
	}
	return out
}
