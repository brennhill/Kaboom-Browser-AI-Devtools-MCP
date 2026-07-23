/**
 * Purpose: Single source of truth for interact action classification (read-only, mutating, requires pilot).
 * Docs: docs/features/feature/interact-explore/index.md
 *
 * SYNC NOTE: The Go side maintains a parallel readOnlyInteractActions map in
 * cmd/browser-agent/tools_interact_dispatch.go for jitter gating. When adding or
 * reclassifying actions here, update the Go map to match.
 */
/**
 * Canonical action metadata map.
 *
 * Every interact action recognized by the TS extension should have an entry.
 * The Go daemon in tools_interact_dispatch.go maintains a parallel
 * readOnlyInteractActions map — keep them in sync.
 */
const ACTION_METADATA = {
    // --- Read-only actions (no DOM mutation, no jitter) ---
    list_interactive: { readonly: true, mutating: false },
    query: { readonly: true, mutating: false },
    get_text: { readonly: true, mutating: false },
    get_value: { readonly: true, mutating: false },
    get_attribute: { readonly: true, mutating: false },
    get_readable: { readonly: true, mutating: false },
    get_markdown: { readonly: true, mutating: false },
    screenshot: { readonly: true, mutating: false },
    explore_page: { readonly: true, mutating: false },
    run_a11y_and_export_sarif: { readonly: true, mutating: false },
    wait_for: { readonly: true, mutating: false },
    wait_for_stable: { readonly: true, mutating: false },
    auto_dismiss_overlays: { readonly: true, mutating: false },
    batch: { readonly: true, mutating: false },
    highlight: { readonly: true, mutating: false },
    subtitle: { readonly: true, mutating: false },
    clipboard_read: { readonly: true, mutating: false },
    list_states: { readonly: true, mutating: false },
    state_list: { readonly: true, mutating: false },
    // --- Mutating actions (modify DOM, require match-evidence validation) ---
    click: { readonly: false, mutating: true },
    type: { readonly: false, mutating: true },
    select: { readonly: false, mutating: true },
    check: { readonly: false, mutating: true },
    set_attribute: { readonly: false, mutating: true },
    paste: { readonly: false, mutating: true },
    key_press: { readonly: false, mutating: true },
    focus: { readonly: false, mutating: true },
    scroll_to: { readonly: false, mutating: true },
    hover: { readonly: false, mutating: true },
    // --- Side-effecting but not DOM-mutating (no match-evidence required) ---
    navigate: { readonly: false, mutating: false },
    refresh: { readonly: false, mutating: false },
    back: { readonly: false, mutating: false },
    forward: { readonly: false, mutating: false },
    new_tab: { readonly: false, mutating: false },
    switch_tab: { readonly: false, mutating: false },
    close_tab: { readonly: false, mutating: false },
    execute_js: { readonly: false, mutating: false },
    navigate_and_wait_for: { readonly: false, mutating: false },
    navigate_and_document: { readonly: false, mutating: false },
    fill_form: { readonly: false, mutating: false },
    fill_form_and_submit: { readonly: false, mutating: false },
    upload: { readonly: false, mutating: false, requiresPilot: true },
    draw_mode_start: { readonly: false, mutating: false },
    hardware_click: { readonly: false, mutating: false },
    activate_tab: { readonly: false, mutating: false },
    save_state: { readonly: false, mutating: false },
    state_save: { readonly: false, mutating: false },
    load_state: { readonly: false, mutating: false },
    state_load: { readonly: false, mutating: false },
    delete_state: { readonly: false, mutating: false },
    state_delete: { readonly: false, mutating: false },
    set_storage: { readonly: false, mutating: false },
    delete_storage: { readonly: false, mutating: false },
    clear_storage: { readonly: false, mutating: false },
    set_cookie: { readonly: false, mutating: false },
    delete_cookie: { readonly: false, mutating: false },
    screen_recording_start: { readonly: false, mutating: false, requiresPilot: true },
    screen_recording_stop: { readonly: false, mutating: false, requiresPilot: true },
    clipboard_write: { readonly: false, mutating: false },
    open_composer: { readonly: false, mutating: false },
    submit_active_composer: { readonly: false, mutating: false },
    confirm_top_dialog: { readonly: false, mutating: false },
    dismiss_top_overlay: { readonly: false, mutating: false },
};
/** Returns true if the action only reads page state (no DOM mutation, no side effects worth toasting). */
export function isReadOnlyAction(action) {
    const meta = ACTION_METADATA[action];
    if (meta)
        return meta.readonly;
    // Fallback heuristic: get_* actions are read-only even if not explicitly listed.
    return action.startsWith('get_');
}
/** Returns true if the action modifies the DOM and requires match-evidence validation. */
export function isMutatingAction(action) {
    return ACTION_METADATA[action]?.mutating === true;
}
//# sourceMappingURL=action-metadata.js.map