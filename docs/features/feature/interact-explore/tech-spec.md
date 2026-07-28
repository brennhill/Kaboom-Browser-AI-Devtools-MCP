---
doc_type: tech-spec
feature_id: feature-interact-explore
status: shipped
owners: []
last_reviewed: 2026-07-27
links:
  product: ./product-spec.md
  tech: ./tech-spec.md
  qa: ./qa-plan.md
  feature_index: ./index.md
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Interact Tech Spec (TARGET)

## Dispatcher
- Entry: `toolInteract` in `cmd/browser-agent/tools_interact_dispatch.go`
- Action routing:
- named action handlers (`interactActionHandler.interactDispatch` map in `cmd/browser-agent/tools_interact_dispatch.go`)
- DOM primitives (`domPrimitiveActions` -> `handleDOMPrimitive`)

### Handler Decomposition (Issue #402)
- `ToolHandler` remains the MCP entrypoint for `interact`, then delegates to `interactActionHandler` for:
- dispatch map construction/caching
- action list generation for schema/error hints
- jitter application policy
- named vs DOM-primitive action routing
- `list_interactive` orchestration + index metadata/truncation post-processing
- DOM primitive selector resolution (`index`/`index_generation`) before queueing `dom_action`
- browser action implementation internals (navigate/refresh/back/forward/new_tab/switch_tab/activate_tab/close_tab, highlight, execute_js, and subtitle)
- URL rewrite (`kaboom-insecure://`) and perf snapshot staging for `perf_diff`
- interact dispatch map now points directly to `interactActionHandler` methods (ToolHandler browser wrappers removed)
- storage/cookie mutation handlers (`set/delete/clear_storage`, `set/delete_cookie`) and execute-script queueing helper
- composed workflow handlers (`navigate_and_wait_for`, `fill_form`, `fill_form_and_submit`, `run_a11y_and_export_sarif`) plus field-step internals
- content extraction handlers (`get_readable`, `get_markdown`, `page_summary` delegation), `explore_page`, `batch`, `clipboard_read/write`, and composable standalone handlers (`wait_for_stable`, `auto_dismiss_overlays`)
- retry/evidence policy state machine (`retry_context`, terminal guidance, evidence before/after capture + attach) now owned by `interactActionHandler`
- draw mode start, CDP/hardware click path, and post-switch tracked-tab sync update are now owned by `interactActionHandler`
- composable side-effect queue helpers and response enrichers (`include_screenshot`, `include_interactive`) are owned by `interactActionHandler`

## Query-Type Mapping
- `navigate/refresh/back/forward/new_tab` -> `browser_action`
- `execute_js` -> `execute`
- DOM primitive actions -> `dom_action`
- `highlight` -> `highlight`
- `subtitle` -> `subtitle`
- `upload` -> `upload`
- `draw_mode_start` -> `draw_mode`
- `screen_recording_start` / `screen_recording_stop` -> `screen_recording_start` / `screen_recording_stop`

## Execution Pipeline
1. Go queues command with correlation ID.
2. Extension receives command through `/sync`.
3. `pending-queries.ts` resolves target tab and executes.
4. Result is posted back through `/sync` command results.
5. Server completes command and returns inline or via observe polling.

## Additional Mechanics
- `query-execution.ts` handles world-aware `execute_js` fallback logic.
- DOM primitives run through extension-side DOM dispatch utilities.
- Performance diff enrichment is attached at command-result formatting time.

## Code Anchors
- `cmd/browser-agent/tools_interact_action_handler.go`
- `cmd/browser-agent/tools_interact_dispatch.go`
- `cmd/browser-agent/tools_interact_dispatch.go`
- `cmd/browser-agent/tools_interact_draw.go`
- `cmd/browser-agent/tools_interact_upload.go`
- `src/background/pending-queries.ts`
- `src/background/exec/query-execution.ts`
