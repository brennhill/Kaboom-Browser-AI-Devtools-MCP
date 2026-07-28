// Purpose: Defines the canonical list of interact tool action values for the what enum.
// Why: Centralizes the action enum so schema definition and action dispatch share a single source.
package interact

// ActionSpec defines per-action metadata used across schema + runtime capability docs.
// Keep this as the single source of truth for interact action surface metadata.
type ActionSpec struct {
	Name     string
	Hint     string
	Required []string
	Optional []string
	IsAlias  bool
}

// actionSpecs is the canonical interact action registry.
// Fields are consumed by:
// - interact schema enum (`what`/`action`)
// - describe_capabilities interact mode specs
var actionSpecs = []ActionSpec{
	{Name: "highlight", Hint: "Visually highlight an element with a colored overlay", Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "duration_ms"}},
	{Name: "subtitle", Hint: "Display a status subtitle in the extension UI", Optional: []string{"text"}},
	{Name: "save_state", Hint: "Snapshot cookies/storage/URL for later restore", Required: []string{"snapshot_name"}, Optional: []string{"storage_type", "include_url"}},
	{Name: "load_state", Hint: "Restore a previously saved state snapshot", Required: []string{"snapshot_name"}, Optional: []string{"storage_type"}},
	{Name: "list_states", Hint: "List all saved state snapshots"},
	{Name: "delete_state", Hint: "Delete a saved state snapshot", Required: []string{"snapshot_name"}},
	{Name: "set_storage", Hint: "Set a localStorage or sessionStorage key", Required: []string{"key"}, Optional: []string{"storage_type", "value"}},
	{Name: "delete_storage", Hint: "Delete a storage key", Required: []string{"key"}, Optional: []string{"storage_type"}},
	{Name: "clear_storage", Hint: "Clear all keys from a storage type", Optional: []string{"storage_type"}},
	{Name: "set_cookie", Hint: "Set a browser cookie", Required: []string{"name"}, Optional: []string{"value", "domain", "path"}},
	{Name: "delete_cookie", Hint: "Delete a browser cookie", Required: []string{"name"}, Optional: []string{"domain", "path"}},
	{Name: "execute_js", Hint: "Run JavaScript in the page context", Required: []string{"script"}, Optional: []string{"world", "timeout_ms"}},
	{Name: "navigate", Hint: "Navigate to a URL", Required: []string{"url"}, Optional: []string{"include_content", "new_tab", "analyze", "auto_dismiss", "wait_for_stable", "stability_ms"}},
	{Name: "refresh", Hint: "Reload the current page", Optional: []string{"analyze"}},
	{Name: "back", Hint: "Browser back button"},
	{Name: "forward", Hint: "Browser forward button"},
	{Name: "new_tab", Hint: "Open a new browser tab", Optional: []string{"url"}},
	{Name: "switch_tab", Hint: "Switch to a different browser tab", Optional: []string{"tab_id", "tab_index", "set_tracked"}},
	{Name: "close_tab", Hint: "Close a browser tab", Optional: []string{"tab_id"}},
	{Name: "click", Hint: "Click an element by selector, element_id, or coordinates", Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "reason", "correlation_id", "timeout_ms", "x", "y", "analyze", "wait_for_stable", "stability_ms"}},
	{Name: "type", Hint: "Type text into an input or textarea", Required: []string{"text"}, Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "clear"}},
	{Name: "select", Hint: "Choose an option in a <select> dropdown", Required: []string{"value"}, Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame"}},
	{Name: "check", Hint: "Toggle a checkbox or radio button", Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "checked"}},
	{Name: "get_text", Hint: "Read text content of an element", Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "structured"}},
	{Name: "get_value", Hint: "Read value of an input element", Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame"}},
	{Name: "get_attribute", Hint: "Read an HTML attribute from an element", Required: []string{"name"}, Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame"}},
	{Name: "query", Hint: "Query DOM elements: check existence, count, read text or attributes without screenshots", Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "query_type", "attribute_names"}},
	{Name: "set_attribute", Hint: "Set an HTML attribute on an element", Required: []string{"name"}, Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "value"}},
	{Name: "focus", Hint: "Focus an element", Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame"}},
	{Name: "scroll_to", Hint: "Scroll an element into view, or scroll container directionally (direction='top'|'bottom'|'up'|'down')", Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame", "direction"}},
	{Name: "wait_for", Hint: "Wait until a selector appears (or disappears with absent=true), text appears, or URL contains a substring", Optional: []string{"selector", "timeout_ms", "frame", "absent", "url_contains", "text"}},
	{Name: "key_press", Hint: "Send keyboard keys (Enter, Tab, Escape, shortcuts)", Optional: []string{"text"}},
	{Name: "paste", Hint: "Paste text into an element via clipboard", Required: []string{"text"}, Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame"}},
	{Name: "open_composer", Hint: "Open the Claude composer interface"},
	{Name: "submit_active_composer", Hint: "Submit the active Claude composer message"},
	{Name: "confirm_top_dialog", Hint: "Accept/confirm the top-most dialog or modal"},
	{Name: "dismiss_top_overlay", Hint: "Dismiss/close the top-most overlay or popover"},
	{Name: "hover", Hint: "Trigger hover state on an element for tooltip discovery", Optional: []string{"selector", "element_id", "index", "nth", "scope_selector", "frame"}},
	{Name: "auto_dismiss_overlays", Hint: "Auto-dismiss cookie consent banners and overlays using known framework selectors", Optional: []string{"timeout_ms"}},
	{Name: "wait_for_stable", Hint: "Wait for DOM stability (no mutations for stability_ms). Returns stable/timed_out status", Optional: []string{"stability_ms", "timeout_ms"}},
	{Name: "list_interactive", Hint: "List all clickable/typeable elements on the page. Use limit to cap results", Optional: []string{"visible_only", "frame", "scope_selector", "scope_rect", "text_contains", "role", "exclude_nav", "limit"}},
	{Name: "get_readable", Hint: "Extract readable text content from the page", Optional: []string{"frame"}},
	{Name: "get_markdown", Hint: "Extract page content as markdown", Optional: []string{"frame"}},
	{Name: "navigate_and_wait_for", Hint: "Navigate to a URL and wait for a selector to appear", Required: []string{"url", "wait_for"}, Optional: []string{"include_content"}},
	{Name: "navigate_and_document", Hint: "Click to navigate, optionally wait for URL change/stability, then return page context", Optional: []string{"selector", "element_id", "index", "index_generation", "nth", "scope_selector", "scope_rect", "frame", "tab_id", "reason", "timeout_ms", "wait_for_url_change", "wait_for_stable", "stability_ms", "include_screenshot", "include_interactive"}},
	{Name: "fill_form_and_submit", Hint: "Fill form fields and click the submit button", Optional: []string{"fields", "submit_selector", "submit_index", "scope_selector", "frame"}},
	{Name: "fill_form", Hint: "Fill multiple form fields at once", Optional: []string{"fields", "scope_selector", "frame"}},
	{Name: "run_a11y_and_export_sarif", Hint: "Run accessibility audit and export results as SARIF", Optional: []string{"save_to", "scope_selector", "frame"}},
	{Name: "screen_recording_start", Hint: "Start recording browser session with video capture", Optional: []string{"name", "audio", "fps"}},
	{Name: "screen_recording_stop", Hint: "Stop recording and save the session", Optional: []string{"name"}},
	{Name: "upload", Hint: "Upload a file to a file input or API endpoint", Optional: []string{"file_path", "api_endpoint", "submit", "escalation_timeout_ms"}},
	{Name: "draw_mode_start", Hint: "Activate annotation overlay for drawing rectangles and adding feedback", Optional: []string{"annot_session", "timeout_ms"}},
	{Name: "hardware_click", Hint: "CDP-level click at x/y coordinates for isTrusted events", Optional: []string{"x", "y"}},
	{Name: "activate_tab", Hint: "Bring the tracked tab to the foreground"},
	{Name: "explore_page", Hint: "Composite page exploration: screenshot, interactive elements, readable text, navigation links, and metadata in one call", Optional: []string{"url", "visible_only", "limit"}},
	{Name: "batch", Hint: "Execute a sequence of interact actions in one call", Optional: []string{"steps", "step_timeout_ms", "continue_on_error", "stop_after_step"}},
	{Name: "clipboard_read", Hint: "Read current clipboard text content"},
	{Name: "clipboard_write", Hint: "Write text to the clipboard", Optional: []string{"text"}},
}

// actionEnum is the canonical list of values accepted by the 'what' parameter.
var actionEnum = actionNames(actionSpecs)

func actionNames(specs []ActionSpec) []string {
	out := make([]string, 0, len(specs))
	for _, spec := range specs {
		if spec.IsAlias {
			continue
		}
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
			Required: append([]string(nil), spec.Required...),
			Optional: append([]string(nil), spec.Optional...),
			IsAlias:  spec.IsAlias,
		})
	}
	return out
}
