---
doc_type: reference
status: active
scope: core/mcp-contract/matrix
ai-priority: high
tags: [core, mcp, command-matrix, option-traceability, canonical]
relates-to: [product-spec.md, tech-spec.md, qa-spec.md]
last-verified: 2026-03-05
canonical: true
---

# MCP Command and Option Matrix

## Source of Truth

- Schemas: `internal/schema/*.go` (one file per tool)
- Dispatchers: `cmd/browser-agent/tools_*_registry.go` and `tools_*_dispatch.go`
- Extension execution: `src/background/pending-queries.ts`

## Dispatch Key

All five tools use `what` as their only top-level dispatch parameter. Alternate
selector parameters and shorthand mode names are rejected.

---

## Command Traceability

### `observe` — 30 modes (`what` -> handler)

| Mode | Handler / File | Description |
|---|---|---|
| `errors` | `observe.GetBrowserErrors` | Browser console errors |
| `logs` | `observe.GetBrowserLogs` | Browser console logs |
| `extension_logs` | `observe.GetExtensionLogs` | Internal extension debug logs |
| `network_waterfall` | `observe.GetNetworkWaterfall` | All network requests (waterfall) |
| `network_bodies` | `observe.GetNetworkBodies` | fetch() request/response bodies |
| `websocket_events` | `observe.GetWSEvents` | WebSocket frame events |
| `websocket_status` | `observe.GetWSStatus` | WebSocket connection status |
| `actions` | `observe.GetEnhancedActions` | Recorded user interactions |
| `vitals` | `observe.GetWebVitals` | Core Web Vitals |
| `page` | `observe.GetPageInfo` | Current page metadata |
| `tabs` | `observe.GetTabs` | Open browser tabs |
| `history` | `observe.AnalyzeHistory` | Navigation history analysis |
| `pilot` | `observe.ObservePilot` | Pilot session state |
| `timeline` | `observe.GetSessionTimeline` | Session event timeline |
| `error_bundles` | `observe.GetErrorBundles` | Pre-assembled debug context per error |
| `screenshot` | `observe.GetScreenshot` | Page screenshot capture |
| `storage` | `observe.GetStorage` | localStorage / sessionStorage / cookies |
| `indexeddb` | `observe.GetIndexedDB` | IndexedDB contents |
| `summarized_logs` | `observe.GetSummarizedLogs` | Grouped/summarized console logs |
| `transients` | `observe.GetTransients` | Transient UI elements (toasts, alerts, banners) |
| `page_inventory` | `toolObservePageInventory` | Inventory of all interactive page elements |
| `inbox` | `toolObserveInbox` | Pending agent inbox items |
| `command_result` | `toolObserveCommandResult` | Result of a specific async command |
| `pending_commands` | `toolObservePendingCommands` | Commands awaiting extension response |
| `failed_commands` | `toolObserveFailedCommands` | Commands that exceeded timeout |
| `saved_videos` | `toolObserveSavedVideos` | List of saved screen recordings |
| `recordings` | `toolGetRecordings` | Event recording sessions |
| `recording_actions` | `toolGetRecordingActions` | Actions from a recording session |
| `playback_results` | `toolGetPlaybackResults` | Results from replaying a recording |
| `log_diff_report` | `toolGetLogDiffReport` | Diff between two log snapshots |

---

### `analyze` — 27 modes (`what` -> handler)

| Mode | Handler / File | Description |
|---|---|---|
| `dom` | `inspect.HandleDOM` | Query DOM structure and elements |
| `performance` | `observe.CheckPerformance` | Performance metrics and timing |
| `accessibility` | `observe.RunA11yAudit` | WCAG accessibility audit |
| `error_clusters` | `observe.AnalyzeErrors` | Cluster and categorize errors |
| `navigation_patterns` | `observe.AnalyzeHistory` | Navigation history analysis |
| `security_audit` | `toolAnalyzeSecurityAudit` | Credential, PII, header, cookie checks |
| `third_party_audit` | `toolAuditThirdParties` | Third-party origin inventory |
| `link_health` | `toolAnalyzeLinkHealth` | Crawl and check link reachability |
| `link_validation` | `toolValidateLinks` | Validate a list of URLs via HTTP |
| `page_summary` | `toolAnalyzePageSummary` | AI-ready page summary via script execution |
| `annotations` | `toolGetAnnotations` | Retrieve draw-mode annotation sessions |
| `annotation_detail` | `toolGetAnnotationDetail` | Full computed styles/DOM for one annotation |
| `api_validation` | `apicontract.Runtime.Handle` | API endpoint contract validation |
| `draw_history` | `toolListDrawHistory` | List persisted draw session files |
| `draw_session` | `toolGetDrawSession` | Load a specific draw session file |
| `computed_styles` | `toolComputedStyles` | Computed CSS styles for an element |
| `forms` | `toolFormDiscovery` | Discover form elements on the page |
| `form_state` | `toolFormState` | Current values of form fields |
| `form_validation` | `toolFormValidation` | Validation state of form fields |
| `data_table` | `toolDataTable` | Extract tabular data from the page |
| `visual_baseline` | `visual.SaveBaseline` | Capture a visual baseline screenshot |
| `visual_diff` | `visual.DiffBaseline` | Compare current page against a baseline |
| `visual_baselines` | `visual.ListBaselines` | List all saved visual baselines |
| `navigation` | `toolAnalyzeNavigation` | Analyze navigation structure |
| `page_structure` | `toolAnalyzePageStructure` | Analyze semantic page structure |
| `audit` | `combinedaudit.Handle` | Multi-category Lighthouse-style audit |
| `feature_gates` | `handleContentExtraction` (inline) | Feature gate status from page context |

---

### `configure` — 30 modes (`what` -> handler)

| Mode | Handler / File | Description |
|---|---|---|
| `store` | `configureSession().toolConfigureStore` | Persist key/value data to session store |
| `load` | `configureSession().toolLoadSessionContext` | Load persisted session context |
| `diff_sessions` | `configureSession().toolDiffSessionsWrapper` | Diff two named session snapshots |
| `health` | `toolGetHealth` | Daemon and extension health check |
| `restart` | `toolConfigureRestart` | Force-restart the daemon |
| `doctor` | `toolDoctor` | Diagnostic check with remediation hints |
| `noise_rule` | `toolConfigureNoiseRule` | Add/remove/list console noise suppression rules |
| `clear` | `toolConfigureClear` | Clear buffered telemetry data |
| `audit_log` | `toolGetAuditLog` | Query the tool invocation audit log |
| `streaming` | `toolConfigureStreaming` | Enable/disable streaming event push |
| `test_boundary_start` | `toolConfigureTestBoundaryStart` | Mark the start of a test boundary |
| `test_boundary_end` | `toolConfigureTestBoundaryEnd` | Mark the end of a test boundary |
| `event_recording_start` | `toolConfigureEventRecordingStart` | Start capturing an event recording session |
| `event_recording_stop` | `toolConfigureEventRecordingStop` | Stop and save the event recording session |
| `playback` | `toolConfigurePlayback` | Replay a saved event recording |
| `log_diff` | `toolConfigureLogDiff` | Initiate a log diff between two recordings |
| `telemetry` | `toolConfigureTelemetry` | Set the global telemetry mode |
| `describe_capabilities` | `toolConfigureDescribeCapabilities` | Return per-mode parameter specs for any tool |
| `tutorial` | `toolConfigureTutorial` | Return getting-started tutorial content |
| `save_sequence` | `toolConfigureSaveSequence` | Save a named interact action sequence |
| `get_sequence` | `toolConfigureGetSequence` | Retrieve a saved sequence by name |
| `list_sequences` | `toolConfigureListSequences` | List all saved sequences |
| `delete_sequence` | `toolConfigureDeleteSequence` | Delete a saved sequence |
| `replay_sequence` | `toolConfigureReplaySequence` | Replay a saved sequence |
| `security_mode` | `toolConfigureSecurityMode` | Get or set security mode (normal / insecure_proxy) |
| `network_recording` | `toolConfigureNetworkRecording` | Configure network request recording filters |
| `action_jitter` | `toolConfigureActionJitter` | Set random delay before interact actions |
| `report_issue` | `issuereport.Handle` | Submit a bug report or issue template |
| `setup_quality_gates` | `qualitygates.Handle` | Install the managed quality-gate configuration |
| `qa_fixture` | `qafixture.Handler.Handle` | Validate, atomically apply, inspect, or idempotently restore a strict versioned QA environment fixture |

---

### `interact` — 63 modes (`what` -> handler)

| Mode | Handler / File | Description |
|---|---|---|
| `highlight` | `handleHighlightImpl` | Visually highlight an element with a colored overlay |
| `subtitle` | `handleSubtitleImpl` | Display a status subtitle in the extension UI |
| `save_state` | `stateInteractHandler.HandleStateSave` | Snapshot cookies/storage/URL for later restore |
| `load_state` | `stateInteractHandler.HandleStateLoad` | Restore a previously saved state snapshot |
| `list_states` | `stateInteractHandler.HandleStateList` | List all saved state snapshots |
| `delete_state` | `stateInteractHandler.HandleStateDelete` | Delete a saved state snapshot |
| `set_storage` | `handleSetStorage` | Set a localStorage or sessionStorage key |
| `delete_storage` | `handleDeleteStorage` | Delete a storage key |
| `clear_storage` | `handleClearStorage` | Clear all keys from a storage type |
| `set_cookie` | `handleSetCookie` | Set a browser cookie |
| `delete_cookie` | `handleDeleteCookie` | Delete a browser cookie |
| `execute_js` | `handleExecuteJSImpl` | Run JavaScript in the page context |
| `navigate` | `handleBrowserActionNavigateImpl` | Navigate to a URL |
| `refresh` | `handleBrowserActionRefreshImpl` | Reload the current page |
| `back` | `handleBrowserActionBackImpl` | Browser back button |
| `forward` | `handleBrowserActionForwardImpl` | Browser forward button |
| `new_tab` | `handleBrowserActionNewTabImpl` | Open a new browser tab |
| `switch_tab` | `handleBrowserActionSwitchTabImpl` | Switch to a different browser tab |
| `close_tab` | `handleBrowserActionCloseTabImpl` | Close a browser tab |
| `click` | `handleDOMPrimitive` (dom_action) | Click an element by selector, element_id, or coordinates |
| `type` | `handleDOMPrimitive` (dom_action) | Type text into an input or textarea |
| `select` | `handleDOMPrimitive` (dom_action) | Choose an option in a select dropdown |
| `check` | `handleDOMPrimitive` (dom_action) | Toggle a checkbox or radio button |
| `get_text` | `handleDOMPrimitive` (dom_action) | Read text content of an element |
| `get_value` | `handleDOMPrimitive` (dom_action) | Read value of an input element |
| `get_attribute` | `handleDOMPrimitive` (dom_action) | Read an HTML attribute from an element |
| `set_attribute` | `handleDOMPrimitive` (dom_action) | Set an HTML attribute on an element |
| `focus` | `handleDOMPrimitive` (dom_action) | Focus an element |
| `scroll_to` | `handleDOMPrimitive` (dom_action) | Scroll an element into view or scroll directionally |
| `wait_for` | `handleDOMPrimitive` (dom_action) | Wait until a selector appears in the DOM |
| `key_press` | `handleDOMPrimitive` (dom_action) | Send keyboard keys (Enter, Tab, Escape, shortcuts) |
| `paste` | `handleDOMPrimitive` (dom_action) | Paste text into an element via clipboard |
| `open_composer` | `handleDOMPrimitive` (dom_action) | Open the Claude composer interface |
| `submit_active_composer` | `handleDOMPrimitive` (dom_action) | Submit the active Claude composer message |
| `confirm_top_dialog` | `handleDOMPrimitive` (dom_action) | Accept/confirm the top-most dialog or modal |
| `dismiss_top_overlay` | `handleDOMPrimitive` (dom_action) | Dismiss/close the top-most overlay or popover |
| `hover` | `handleDOMPrimitive` (dom_action) | Trigger hover state on an element for tooltip discovery |
| `auto_dismiss_overlays` | `handleAutoDismissOverlays` (composable) | Auto-dismiss cookie consent banners using known framework selectors |
| `wait_for_stable` | `handleWaitForStable` (composable) | Wait for DOM stability (no mutations for stability_ms) |
| `query` | `handleDOMPrimitive` (dom_action) | Query DOM: check existence, count, read text or attributes |
| `list_interactive` | `handleListInteractive` | List all clickable/typeable elements on the page |
| `get_readable` | `handleGetReadable` | Extract readable text content from the page |
| `get_markdown` | `handleGetMarkdown` | Extract page content as markdown |
| `navigate_and_wait_for` | `handleNavigateAndWaitFor` | Navigate to a URL and wait for a selector to appear |
| `navigate_and_document` | `handleNavigateAndDocument` | Click to navigate, wait for URL change/stability, return page context |
| `fill_form_and_submit` | `handleFillFormAndSubmit` | Fill form fields and click the submit button |
| `fill_form` | `handleFillForm` | Fill multiple form fields at once |
| `run_a11y_and_export_sarif` | `handleRunA11yAndExportSARIF` | Run accessibility audit and export as SARIF |
| `screen_recording_start` | `recordingInteractHandler.handleRecordStart` | Start recording browser session with video capture |
| `screen_recording_stop` | `recordingInteractHandler.handleRecordStop` | Stop recording and save the session |
| `upload` | `uploadInteractHandler.handleUpload` | Upload a file to a file input or API endpoint |
| `draw_mode_start` | `handleDrawModeStart` | Activate annotation overlay for drawing rectangles |
| `hardware_click` | `handleHardwareClick` | CDP-level click at x/y coordinates for isTrusted events |
| `activate_tab` | `handleActivateTabImpl` | Bring the tracked tab to the foreground |
| `explore_page` | `handleExplorePage` | Composite page exploration: screenshot, elements, text, links in one call |
| `batch` | `handleBatch` | Execute a sequence of interact actions in one call |
| `clipboard_read` | `handleClipboardRead` | Read current clipboard text content |
| `clipboard_write` | `handleClipboardWrite` | Write text to the clipboard |

---

### `generate` — 13 modes (`what` -> handler)

| Mode | Handler / File | Description |
|---|---|---|
| `reproduction` | `toolGetReproductionScript` | Generate a bug reproduction script |
| `test` | `toolGenerateTest` | Generate a Playwright/Puppeteer test |
| `pr_summary` | `toolGeneratePRSummary` | Generate a PR summary from captured actions |
| `har` | `toolExportHAR` | Export captured requests as HAR |
| `csp` | `toolGenerateCSP` | Generate a Content Security Policy |
| `sri` | `toolGenerateSRI` | Generate Subresource Integrity hashes |
| `sarif` | `toolExportSARIF` | Export accessibility results as SARIF |
| `visual_test` | `toolGenerateVisualTest` | Generate visual regression test |
| `annotation_report` | `toolGenerateAnnotationReport` | Export annotation session as report |
| `annotation_issues` | `toolGenerateAnnotationIssues` | Export annotation session as issue list |
| `test_from_context` | `testGen().handleGenerateTestFromContext` | Generate a test from current error/interaction context |
| `test_heal` | `testGen().handleGenerateTestHeal` | Heal broken selectors in existing test files |
| `test_classify` | `testGen().handleGenerateTestClassify` | Classify test failures by root cause |

---

## Option Traceability

### `observe` options

- Dispatch key: `what`
- Pagination keys: `limit`, `after_cursor`, `before_cursor`, `since_cursor`, `restart_on_eviction`
- Filtering keys: `min_level`, `source`, `url`, `method`, `status_min`, `status_max`, `body_path`, `connection_id`, `direction`, `last_n`, `include`, `window_seconds`, `scope`
- Log detail keys: `include_internal`, `include_extension_logs`, `extension_limit`, `min_group_size`
- Screenshot keys: `format`, `quality`, `full_page`, `selector`, `wait_for_stable`, `save_to`
- Storage keys: `storage_type`, `key`, `database`, `store`
- Transients key: `classification`
- Page inventory key: `visible_only`
- Recording keys: `recording_id`, `correlation_id`, `original_id`, `replay_id`
- Summary mode applies to: `errors`, `logs`, `network_waterfall`, `network_bodies`, `websocket_events`, `websocket_status`, `actions`, `error_bundles`, `timeline`, `history`, `transients`, `storage`
- Cross-cutting key: `telemetry_mode`

### `analyze` options

- Dispatch key: `what`
- Async control keys: `sync`, `wait`, `background`, `correlation_id`
- Element targeting: `selector`, `frame`, `tab_id`
- Operation control: `operation`, `ignore_endpoints`
- Accessibility keys: `scope`, `tags`, `force_refresh`
- Link keys: `domain`, `max_workers`, `urls`
- Page summary key: `world`
- Timing key: `timeout_ms`
- Security keys: `checks`, `severity_min`
- Third-party keys: `first_party_origins`, `include_static`, `custom_lists`
- Annotation keys: `annot_session`, `url`
- Draw session keys: `file`
- Visual diff keys: `name`, `baseline`, `threshold`
- Data table keys: `max_rows`, `max_cols`
- Summary key applies to: `accessibility`, `security_audit`, `third_party_audit`, `form_validation`, `audit`
- Audit categories key: `categories`
- Cross-cutting key: `telemetry_mode`

### `generate` options

- Dispatch key: `what`
- Shared generation keys: `error_message`, `last_n`, `base_url`, `include_screenshots`, `generate_fixtures`, `visual_assertions`, `test_name`, `assert_network`, `assert_no_errors`, `assert_response_shape`, `scope`, `include_passes`, `save_to`, `url`, `method`, `status_min`, `status_max`, `mode`, `include_report_uri`, `exclude_origins`, `resource_types`, `origins`
- Annotation session key: `annot_session`
- Test-heal/classify keys: `context`, `action`, `test_file`, `test_dir`, `broken_selectors`, `auto_apply`, `failure`, `failures`, `error_id`, `include_mocks`, `output_format`
- Cross-cutting key: `telemetry_mode`

### `configure` options

- Dispatch key: `what`
- `store`: `store_action`, `namespace`, `key`, `data`
- `noise_rule`: `noise_action`, `rules`, `rule_id`, `pattern`, `category`, `reason`, `classification`, `message_regex`, `source_regex`, `url_regex`, `method`, `status_min`, `status_max`, `level`
- `clear`: `buffer`
- `streaming`: `streaming_action`, `events`, `throttle_seconds`, `severity_min`, `url`
- `telemetry`: `telemetry_mode`
- `diff_sessions`: `compare_a`, `compare_b`, `name`, `url`
- `audit_log`: `operation`, `audit_session_id`, `tool_name`, `since`, `limit`
- `test_boundary_start`: `test_id`, `label`
- `test_boundary_end`: `test_id`
- `event_recording_start`: `name`, `url`, `sensitive_data_enabled`
- `event_recording_stop`: `recording_id`
- `playback`: `recording_id`
- `log_diff`: `original_id`, `replay_id`
- `report_issue`: `operation`, `template`, `title`, `user_context`
- `security_mode`: `mode`, `confirm`
- `describe_capabilities`: `tool`
- `save_sequence`: `name`, `steps`, `description`, `tags`
- `get_sequence` / `delete_sequence` / `replay_sequence`: `name`
- `network_recording`: `domain`
- `action_jitter`: `action_jitter_ms`
- Cross-cutting key: `telemetry_mode`

### `interact` options

- Dispatch key: `what`
- Async control keys: `sync`, `wait`, `background`, `correlation_id`
- Element targeting keys: `selector`, `element_id`, `index`, `index_generation`, `nth`, `scope_selector`, `scope_rect`, `frame`, `tab_id`
- Action keys: `text`, `value`, `clear`, `checked`, `name`, `direction`, `query_type`, `attribute_names`, `structured`, `include_content`, `include_interactive`, `include_screenshot`
- Navigation keys: `url`, `new_tab`, `wait_for`, `wait_for_url_change`, `wait_for_stable`, `stability_ms`, `auto_dismiss`, `analyze`
- Screenshot/observe keys: (delegates to observe/screenshot)
- JS execution keys: `script`, `world`
- Timing keys: `timeout_ms`, `duration_ms`
- State keys: `snapshot_name`, `storage_type`, `include_url`
- Cookie keys: `domain`, `path`
- Form keys: `fields`, `submit_selector`, `submit_index`
- Recording keys: `audio`, `fps`
- Upload keys: `file_path`, `api_endpoint`, `submit`, `escalation_timeout_ms`
- Annotation keys: `annot_session`
- Batch keys: `steps`, `step_timeout_ms`, `continue_on_error`, `stop_after_step`
- Tab keys: `tab_index`, `set_tracked`
- Jitter note: read-only actions (`list_interactive`, `get_text`, `get_value`, `get_attribute`, `query`, `list_states`, `get_readable`, `get_markdown`, `explore_page`, `run_a11y_and_export_sarif`, `wait_for`, `wait_for_stable`, `auto_dismiss_overlays`, `batch`, `highlight`, `subtitle`, `clipboard_read`) are exempt from action jitter
- Cross-cutting key: `telemetry_mode`

---

## End-to-End Processing Layers

1. Schema exposure (`tools/list`) from `internal/schema/*.go`.
2. Tool dispatch and option parsing in `cmd/browser-agent/tools_*_registry.go` / `tools_*_dispatch.go`.
3. Extension-bound options forwarded inside queued query `Params` JSON.
4. Extension execution and result correlation in `pending-queries.ts` + `/sync`.
5. Final command status surfaced via `observe(what=command_result)`.
