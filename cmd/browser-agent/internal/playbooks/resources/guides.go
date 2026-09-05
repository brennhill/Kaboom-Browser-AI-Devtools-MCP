// guides.go — Stores long-form guide/quickstart markdown resources for MCP resource reads.
// Why: Keeps documentation payloads separate from playbook catalogs and resolver logic.

package resources

// GuideContent is the full usage guide resource.
const GuideContent = `# Kaboom MCP Tools

Agentic Browser Devtools - rapid e2e web development. 5 tools for real-time browser telemetry.

## Quick Reference

| Tool | Purpose | Key Parameters |
|------|---------|----------------|
| observe | Read passive browser buffers | what: errors, logs, extension_logs, network_waterfall, network_bodies, websocket_events, websocket_status, actions, vitals, page, tabs, history, pilot, timeline, error_bundles, screenshot, storage, indexeddb, command_result, pending_commands, failed_commands, saved_videos, recordings, recording_actions, playback_results, log_diff_report, summarized_logs, page_inventory, transients, inbox |
| analyze | Trigger active analysis (synchronous by default) | what: dom, accessibility, performance, security_audit, third_party_audit, link_health, link_validation, page_summary, error_clusters, navigation_patterns, api_validation, annotations, annotation_detail, draw_history, draw_session, computed_styles, forms, form_state, form_validation, data_table, visual_baseline, visual_diff, visual_baselines, navigation, page_structure, audit, feature_gates, verification, design_audit |
| generate | Create artifacts from captured data | what: test, reproduction, pr_summary, sarif, har, csp, sri, visual_test, annotation_report, annotation_issues, test_from_context, test_heal, test_classify |
| configure | Session settings and utilities | what: health, store, load, noise_rule, clear, streaming, test_boundary_start, test_boundary_end, event_recording_start, event_recording_stop, playback, log_diff, telemetry, describe_capabilities, diff_sessions, audit_log, restart, save_sequence, get_sequence, list_sequences, delete_sequence, replay_sequence, doctor, security_mode, network_recording, action_jitter, report_issue, setup_quality_gates, qa_fixture, tutorial |
| interact | Browser automation (needs AI Web Pilot) | what: highlight, subtitle, save_state, load_state, list_states, delete_state, set_storage, delete_storage, clear_storage, set_cookie, delete_cookie, execute_js, navigate, refresh, back, forward, new_tab, switch_tab, close_tab, screenshot, click, type, select, check, get_text, get_value, get_attribute, query, set_attribute, focus, scroll_to, wait_for, key_press, paste, open_composer, submit_active_composer, confirm_top_dialog, dismiss_top_overlay, hover, auto_dismiss_overlays, wait_for_stable, list_interactive, get_readable, get_markdown, navigate_and_wait_for, navigate_and_document, fill_form_and_submit, fill_form, run_a11y_and_export_sarif, screen_recording_start, screen_recording_stop, upload, draw_mode_start, activate_tab, explore_page, batch, clipboard_read, clipboard_write |

## Key Patterns

### Check Extension Status First
Always verify the extension is connected before debugging:
  {"tool":"configure","arguments":{"what":"health"}}
If extension_connected is false, ask the user to click "Track This Tab" in the extension popup.

### Analyze Tool (Synchronous by Default)
analyze blocks until the extension returns a result (up to 15s). No polling needed:
  {"tool":"analyze","arguments":{"what":"accessibility"}}
For long-running analyses, set background:true to get a correlation_id and poll:
  1. {"tool":"analyze","arguments":{"what":"accessibility","background":true}}  -> returns correlation_id
  2. {"tool":"observe","arguments":{"what":"command_result","correlation_id":"..."}}

### Pagination (observe tool)
Responses include cursors in metadata. Pass back for next page:
  {"tool":"observe","arguments":{"what":"logs","after_cursor":"...","limit":50}}
Use restart_on_eviction=true if a cursor expires.

## Common Workflows

  // See errors with surrounding context (network + actions + logs)
  {"tool":"observe","arguments":{"what":"error_bundles"}}

  // Check failed network requests
  {"tool":"observe","arguments":{"what":"network_waterfall","status_min":400}}

  // Run accessibility audit (async)
  {"tool":"analyze","arguments":{"what":"accessibility"}}

  // Query DOM elements (async)
  {"tool":"analyze","arguments":{"what":"dom","selector":".error-message"}}

  // Generate Playwright test from session
  {"tool":"generate","arguments":{"what":"test","test_name":"user_login"}}

  // Check Web Vitals (LCP, CLS, INP, FCP)
  {"tool":"observe","arguments":{"what":"vitals"}}

  // Navigate and measure performance (auto perf_diff)
  {"tool":"interact","arguments":{"what":"navigate","url":"https://example.com"}}

  // Suppress noisy console errors
  {"tool":"configure","arguments":{"what":"noise_rule","noise_action":"auto_detect"}}

  // Take a screenshot to see current page state
  {"tool":"observe","arguments":{"what":"screenshot"}}

  // Click a button or link
  {"tool":"interact","arguments":{"what":"click","selector":"text=Submit"}}

  // Type into an input field
  {"tool":"interact","arguments":{"what":"type","selector":"input[name=search]","text":"hello world"}}

  // Discover clickable elements on the page
  {"tool":"interact","arguments":{"what":"list_interactive","scope_selector":"main"}}

  // Record a user flow for playback
  {"tool":"configure","arguments":{"what":"event_recording_start","name":"my-flow"}}
  // ... perform actions ...
  {"tool":"configure","arguments":{"what":"event_recording_stop","recording_id":"..."}}

  // List saved recordings
  {"tool":"observe","arguments":{"what":"recordings"}}

  // Start annotation/drawing mode for visual feedback
  {"tool":"interact","arguments":{"what":"draw_mode_start","annot_session":"review-1"}}

  // Retrieve annotations from a drawing session
  {"tool":"analyze","arguments":{"what":"annotations","annot_session":"review-1"}}

  // Generate annotation report
  {"tool":"generate","arguments":{"what":"annotation_report","annot_session":"review-1"}}

## Tips

- Best starting point: interact(what:"explore_page") returns a comprehensive page overview with interactive elements, headings, and semantic structure
- Start with configure(what:"health") to verify extension is connected
- Use observe(what:"error_bundles") instead of raw errors — includes surrounding context
- Use observe(what:"page") to confirm which URL the browser is on
- interact actions require the AI Web Pilot extension feature to be enabled
- interact navigate and refresh automatically include performance diff metrics
- Data comes from the active tracked browser tab
- For SEO/best-practices work: check if the site serves /llms.txt (llmstxt.org). Sites with llms.txt and per-page .md variants get better AI agent and LLM discoverability. analyze(what:"audit") includes this recommendation automatically
`

// QuickstartContent is the short quickstart resource.
const QuickstartContent = `# Kaboom MCP Quickstart

## 1. Health Check
{"tool":"configure","arguments":{"what":"health"}}

## 2. Confirm Tracked Page
{"tool":"observe","arguments":{"what":"page"}}

## 3. Collect Errors + Context
{"tool":"observe","arguments":{"what":"error_bundles"}}

## 4. Network Failures
{"tool":"observe","arguments":{"what":"network_waterfall","status_min":400}}

## 5. WebSocket Status
{"tool":"observe","arguments":{"what":"websocket_status"}}

## 6. Explore the Page (Best Starting Point)
{"tool":"interact","arguments":{"what":"explore_page"}}

## 7. Accessibility Audit
{"tool":"analyze","arguments":{"what":"accessibility"}}

## 8. DOM Query
{"tool":"analyze","arguments":{"what":"dom","selector":".error-message"}}

## 9. Performance Check
{"tool":"interact","arguments":{"what":"navigate","url":"https://example.com"}}

## 10. Start Recording
{"tool":"configure","arguments":{"what":"event_recording_start","name":"demo-run"}}

## 11. Stop Recording
{"tool":"configure","arguments":{"what":"event_recording_stop","recording_id":"..."}}

## 12. Navigate and Take Screenshot
{"tool":"interact","arguments":{"what":"navigate","url":"https://example.com"}}
{"tool":"observe","arguments":{"what":"screenshot"}}

## 13. Click a Button
{"tool":"interact","arguments":{"what":"click","selector":"text=Sign In"}}

## 14. Type Into a Field
{"tool":"interact","arguments":{"what":"type","selector":"input[name=email]","text":"user@example.com"}}

## 15. Fill and Submit a Form (Single Call)
{"tool":"interact","arguments":{"what":"fill_form_and_submit","selector":"form","fields":{"email":"user@example.com","password":"password123"}}}

## 16. Discover Interactive Elements
{"tool":"interact","arguments":{"what":"list_interactive","scope_selector":"form"}}

## 17. Interact Failure Recovery (Quick)

- element_not_found
  - Run interact({what:"list_interactive", scope_selector:"<container>"}) and retry with element_id/index.
- ambiguous_target
  - Add scope_selector/scope_rect, refresh list_interactive, then retry with element_id/index.
- stale_element_id
  - Refresh list_interactive in same scope, reacquire element_id, retry once.
- scope_not_found
  - Fallback from scope_selector to scope_rect/frame, then rerun list_interactive.
- blocked_by_overlay
  - Run interact({what:"dismiss_top_overlay"}) to close the modal, then retry the original action.

Stop and report evidence when recovery does not converge after one scoped retry cycle:
- observe({what:"command_result", correlation_id:"..."})
- observe({what:"screenshot"})
- interact({what:"list_interactive", scope_selector:"<best-known-scope>"})
`
