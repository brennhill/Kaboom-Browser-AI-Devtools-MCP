---
doc_type: feature_index
feature_id: feature-interact-explore
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-07
code_paths:
  - cmd/browser-agent/internal/interactdispatch/handler.go
  - cmd/browser-agent/internal/toolinteract/action_owners.go
  - cmd/browser-agent/internal/toolguard/guards.go
  - cmd/browser-agent/internal/toolinteract/interactstate/state.go
  - cmd/browser-agent/internal/toolinteract/interactupload/upload.go
  - cmd/browser-agent/internal/toolinteract/action_runtime.go
  - cmd/browser-agent/internal/toolinteract/interact_dom.go
  - cmd/browser-agent/internal/toolinteract/interact_browser.go
  - cmd/browser-agent/internal/toolinteract/interact_page.go
  - cmd/browser-agent/internal/toolinteract/interact_workflow.go
  - cmd/browser-agent/internal/toolinteract/elemindex/registry.go
  - cmd/browser-agent/tools_core.go
  - cmd/browser-agent/tools_configure.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher.go
  - cmd/browser-agent/tools_interact_dispatch.go
  - internal/recording/actionlog/recorder.go
  - internal/tools/interact/workflow.go
  - internal/schema/interact/actions.go
  - internal/schema/interact/tool.go
  - internal/schema/interact/properties_targeting.go
  - internal/schema/interact/properties_output_batch.go
  - internal/tools/configure/capabilities/modespecs_interact.go
  - scripts/docs/reference/check-reference-schema-sync.mjs
  - src/background.ts
  - src/background/pending-queries.ts
  - src/background/exec/query-execution.ts
  - src/background/commands/helpers.ts
  - src/background/commands/results/element-results.ts
  - src/background/commands/interact-explore.ts
  - src/background/exec/browser-actions.ts
  - src/background/runtime-state/csp-state.ts
  - src/background/runtime-state/content-readiness.ts
  - src/background/commands/registry.ts
  - src/background/dom/cdp/cdp-dispatch.ts
  - src/background/dom/dom-dispatch.ts
  - src/background/exec/frame-targeting.ts
  - src/background/exec/content-fallback-scripts.ts
  - src/background/exec/upload-handler.ts
  - src/lib/daemon-http.ts
  - src/background/ui/draw-mode-toggle.ts
  - src/background/dom/dom-types.ts
  - src/background/dom/primitives/dom-primitives-pointer.ts
  - src/background/dom/primitives/dom-primitives-form.ts
  - src/background/dom/primitives/dom-primitives-read.ts
  - src/inject.ts
  - src/inject/execute-js.ts
  - src/inject/message-handlers.ts
  - src/inject/state.ts
  - src/content/message-handlers.ts
  - src/content/request-tracking.ts
  - src/content/script-injection.ts
  - src/content/window-message-listener.ts
  - src/content/runtime-message-listener.ts
  - src/content/ui/toast.ts
  - src/types/runtime-messages.ts
  - src/background/dom/primitives/dom-primitives-list-interactive.ts
  - src/background/dom/primitives/dom-primitives-intent.ts
  - src/background/dom/primitives/dom-primitives-overlay.ts
  - src/background/dom/primitives/dom-primitives-stability.ts
  - scripts/templates/partials/_dom-intent.tpl
  - scripts/templates/partials/_dom-selectors.tpl
  - scripts/templates/dom-primitives.ts.tpl
  - scripts/templates/dom-primitives-intent.ts.tpl
  - scripts/templates/dom-primitives-overlay.ts.tpl
  - scripts/templates/partials/shared/_dom-self-contained-core.tpl
  - scripts/build/generate-dom-primitives.js
  - cmd/browser-agent/internal/asyncresult/normalization.go
  - cmd/browser-agent/internal/asyncresult/enrichment.go
  - cmd/browser-agent/internal/asyncresult/enrichment_csp.go
  - cmd/browser-agent/internal/asyncresult/enrichment_recovery.go
  - cmd/browser-agent/internal/asyncresult/lifecycle.go
  - cmd/browser-agent/internal/asynccommand/handler.go
  - cmd/browser-agent/internal/summarypref/cache.go
  - cmd/browser-agent/tools_core.go
test_paths:
  - cmd/browser-agent/internal/interactdispatch/handler_test.go
  - tests/extension/content/content.test.js
  - tests/extension/content/content-ui.test.js
  - scripts/contracts/check-architecture-boundaries.test.cjs
  - cmd/browser-agent/internal/toolinteract/action_runtime_test.go
  - cmd/browser-agent/internal/toolinteract/interact_browser_test.go
  - tests/extension/misc/upload-handler.test.js
  - tests/extension/dom/command-element-results.test.js
  - cmd/browser-agent/internal/toolconfigure/dispatcher_test.go
  - scripts/contracts/goarchitecturetests/contracts_test.go
  - internal/recording/actionlog/recorder_test.go
  - cmd/browser-agent/internal/summarypref/cache_test.go
  - cmd/browser-agent/internal/toolinteract/interact_dom_test.go
  - cmd/browser-agent/internal/toolinteract/interact_workflow_test.go
  - cmd/browser-agent/internal/toolinteract/contracts/explore_test.go
  - cmd/browser-agent/internal/toolinteract/elemindex/registry_test.go
  - cmd/browser-agent/internal/toolguard/guards_test.go
  - cmd/browser-agent/internal/toolinteract/contracts/gates_test.go
  - cmd/browser-agent/internal/toolinteract/action_runtime_test.go
  - cmd/browser-agent/internal/toolinteract/contracts/rich_action_test.go
  - cmd/browser-agent/internal/toolinteract/contracts/performance_test.go
  - cmd/browser-agent/internal/asynccommand/handler_test.go
  - cmd/browser-agent/composition_test.go
  - internal/schema/interact/schema_test.go
  - cmd/browser-agent/internal/toolinteract/interactstate/state_test.go
  - extension/background/__tests__/dom-dispatch-structured.test.js
  - tests/extension/dom/dom-primitives-branding.test.js
  - tests/extension/dom/dom-primitives-generation.test.js
  - tests/extension/dom/dom-action-family-routing.test.js
  - tests/extension/ui-controls/action-toast-labels.test.js
  - tests/extension/injection/execute-js.test.js
  - internal/tools/interact/workflow_test.go
  - internal/tools/configure/capabilities/modespecs_test.go
  - tests/extension/ui-controls/toggle-overlay.test.js
  - cmd/browser-agent/internal/asyncresult/asyncresult_test.go
  - cmd/browser-agent/internal/asynccommand/formatting_test.go
  - tests/extension/pilot/interact-content-fallback.test.js
  - tests/extension/pilot/async-timeout.test.js
  - tests/extension/sync/pending-query-targeting.test.js
  - tests/extension/pilot/pilot-execute.test.js
  - tests/extension/pilot/pilot-state.test.js
  - tests/extension/pilot/pilot-toggle.test.js
  - tests/extension/contracts/no-compatibility-facades.test.js
  - tests/extension/content/content-message-correlation.test.js
  - tests/extension/tab-state/content-readiness.test.js
  - tests/architecture/async-failure-evidence.test.cjs
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Interact Tool

The background service-worker entrypoint owns startup only. Interaction
consumers import state, command, snapshot, and query APIs from their focused
owner modules; no compatibility facade is retained.
Analyze and generate dispatchers are constructed before interact dependencies
capture their cross-tool entry points, so workflows never retain nil dispatchers.
The action registry contains canonical actions only; schemas, capability metadata,
runtime parity, and reference generators consume it directly without alias filters.
Page-world interaction tests import action, state, serialization, and message
handler modules directly; the injected bundle is not an API surface.
Every content/inject `window.postMessage` request and response requires the
current per-page nonce. Missing and mismatched nonces fail closed; there is no
unauthenticated migration path.
Highlight responses explicitly echo that authenticated nonce. This lets the
content listener resolve a valid highlight immediately while continuing to
reject missing or mismatched response envelopes instead of waiting for the
command timeout.
Screenshot capture belongs only to `observe({what:"screenshot"})`; the former
`interact` screenshot compatibility action has been removed.
Pilot navigation lifecycle tests model tab readiness explicitly and use
controlled completion signals, so consecutive operations prove exactly-once
settlement without wall-clock sleeps or load-sensitive timeout behavior.
`include_screenshot` remains a composable response enrichment after a real
interact action. Tests acknowledge the action query exactly as the extension
does, then await the screenshot query through the canonical pending-query
notification barrier; no polling sleeps or undelivered-query shortcuts remain.
Browser-action completion helpers use that same dispatcher notification rather
than polling. Rich-result timing accepts zero at millisecond resolution and
asserts only the public nonnegative timing contract.
Extension-gate interaction coverage uses the readiness transition itself as its
synchronization event; it no longer sleeps before connecting.
State snapshot handlers accept only the canonical `snapshot_name` parameter;
the former generic `name` request alias has been removed.
State dispatch and tests use the composed `stateInteractHandler` directly; the
root unchanged-return accessor has been deleted and is structurally prohibited.
The unused generic `internal/tools/interact` host declaration is also deleted;
interaction dependencies live only with the handlers that consume them.
Action dispatch is split among direct DOM, browser/tab, page/composable,
workflow, storage, and batch owners. A small action runtime owns only shared
command lifecycle policy. The former broad `InteractActionHandler` and
`toolinteract.Deps` surfaces are deleted and structurally prohibited; no
forwarding facade remains.
The browser/tab owner exposes one `Handle(action, request, args)` boundary.
Action-specific implementations, insecure-navigation rewriting, and tracking
continuations are private; an AST contract test rejects any new exported
`BrowserActions` method so registry callers cannot couple to implementation
details.
Composition also supplies evidence and query callbacks directly; dead or
one-line ToolHandler forwarding methods are structurally prohibited.
Evidence capture is runtime-scoped: no process-global test override exists.
Its retry wait is injected for deterministic tests, and owner-level contracts
cover complete, partial, read-only, cached, and mutation evidence lifecycles.
Evidence screenshot tests await the query dispatcher's enqueue notification,
complete the exact screenshot query, and close their capture runtime. No
polling sleep or leaked cleanup goroutine participates in the result contract.
Summary response-mode behavior belongs to `summarypref.Cache`; async formatting
and dependency wiring use that owner directly, and the former four-method root
forwarding layer plus its duplicate tests have been deleted.
Configure action-jitter handlers receive `ActionRuntime` callbacks through the
explicit configure dependency value; root jitter forwarding methods are deleted.
Public state actions likewise use only `save_state`, `load_state`,
`list_states`, and `delete_state`; duplicate `state_*` entry points are not
registered. The similarly named extension pending-query types remain internal.
Intent, overlay, pointer, form, and read DOM primitives are generated from
canonical templates. Shared injected traversal logic is maintained once in a
generator partial; generated copies remain self-contained because Chrome
serializes injected functions without module scope.

## TL;DR

- Action jitter uses the standard-library cryptographic random source and stays
  within the configured exclusive upper bound.
- Status: shipped
- Tool: `interact`
- Mode key: `what`
- Contract source: `internal/schema/interact/tool.go`

## Specs
- Product: `product-spec.md`
- Tech: `tech-spec.md`
- QA: `qa-plan.md`
- Flow Map Pointer: `flow-map.md`

## Canonical Note
This feature documents the shipped `interact` action surface (not a batched `interact.explore` action).

The generated DOM primitive modules intentionally contain shared selector,
target-resolution, and result machinery. Chrome serializes each injected
function independently, so imports or shared closures would fail at runtime.
Their duplication is generated from one template and focused partials; only the
action-family handlers differ. These generated modules are the documented
`jscpd` exception. Handwritten command clones are not exempt:
`selectCommandElements`, `collectCommandElements`, and `commandPageMetadata`
are grouped in the pure `commands/results` module and shared directly by
`observe` and `interact-explore`.

After a document-changing browser action, content-dependent interaction is
gated on a correlation-matched acknowledgement from the newly injected content
script. The bounded retry schedule replaces fixed navigation sleeps; a final
failure is retryable and retained by System Doctor.

`get_text` supports `structured:true` for hierarchical extraction (for example accordion/list sections), and this option must be forwarded through DOM dispatch into extension primitives.

`execute_js` host-object serialization must preserve prototype-backed values (for example `DOMRect`) so return payloads remain structured and parse-safe.

Back/forward navigation uses Chrome's tab-history API first so restricted pages
remain controllable. If Chrome incorrectly rejects an available transition, the
extension falls back to page history and reports success only after a bounded,
correlation-logged URL/load transition; an unacknowledged fallback remains an
error.

`navigate_and_document` combines click-driven navigation, optional URL-change/stability waits, and page-context enrichment (`url`, `title`, `tab_id`) in a single interact workflow. URL transitions use a generation-channel notification owned by the synchronized extension runtime; both direct tab retargeting and extension `/sync` state updates wake the bounded wait without polling or sleeps. A page unload may destroy the old content-script context before its click acknowledgement arrives; an exact `no_result` is therefore accepted only when that tracked-URL transition independently confirms that navigation completed. Other click errors still fail normally.

The workflow owns an explicit clock dependency for its total timeout budget.
Production supplies the system clock; tests advance a controlled clock after
observing click dispatch and assert exact remaining-stage budgets without
sleeping or relying on scheduler timing.

Upload-handler unit tests replace native-dialog and verification waits at the
time boundary. Production backoff remains unchanged, while mocked Chrome and
daemon paths complete deterministically under concurrent shard load.

Navigation page-summary enrichment is colocated with the page action owner.
Browser and workflow owners call it directly through explicit owner
relationships; no broad dependency bag or enrichment callback facade remains.

Workflow types and response classification come directly from
`internal/tools/interact/workflow.go`; browser-agent layers do not maintain
aliases or pass-through response helpers.

DOM action metadata, selector extraction, execution-world validation, and
preview truncation are also called directly from `internal/tools/interact`.
The browser-agent action owners do not retain package-variable delegate aliases.

Evidence capture uses the concrete
`toolinteract.EvidenceShot` contract directly in runtime wiring and tests; the
former private/exported type alias and root-package test shim have been deleted.

Pilot, extension, and tab preconditions use the canonical `toolguard.Check`
contract; interact packages do not mirror the host guard signature.

Interact handlers and their dependency seams use `internal/mcp` and
`internal/toolresp` directly. Package-local protocol type, error-code, and
structured-error option aliases are prohibited so the canonical contract stays
visible at every call site.
State redaction is injected as the single map transformation the state owner
needs; `toolinteract` does not mirror or expose a host redaction-engine
interface.

List-interactive response indexing, metadata annotation, and truncation share a
single JSON-content decoder. Tests cover prefixed payloads, malformed blocks,
index construction, and truncation so response-shaping paths cannot drift.

`navigate_and_document` now returns structured metadata for machine consumers:
1. `metadata.page_context` (`url`, `title`, `tab_id`) while preserving the legacy text block.
2. `metadata.workflow_trace` (`trace_id`, `status`, stage-level timing/status envelope).
3. Explicit `tab_id` now requires an actively tracked tab and must match tracked context before click dispatch.

Interact action metadata now has a single canonical registry in `internal/schema/interact/actions.go`, consumed by both schema enum generation and `describe_capabilities` mode specs.

Extension-dispatched interact actions now use shared enqueue fail-fast handling: when queue capacity is saturated, responses return structured `queue_full` immediately rather than entering async wait mode.

State-save interaction tests use a shared event-driven capture responder for
form, storage, redaction, legacy-shape, and failure results. The responder owns
serialization errors and query-type validation; no state workflow test polls
command creation by sleeping.

Content-to-page requests have one lifecycle owner. Each request timer is cleared
on response, explicit deletion, pagehide, beforeunload, or content-script
shutdown; page lifecycle cancellation resolves the pending callback. The dead
shared cleanup interval and timestamp registry are deleted, so importing the
content bundle cannot keep focused test runners alive.

Async execution is controlled only by the canonical `background` parameter.
The entrypoint does not translate alternate parameter names.

All MCP tools route exclusively through `what`; `interact` does not accept
`action` as a selector.

Region targeting uses only `scope_rect`; `annotation_rect` is not translated.
Directional scrolling uses only `direction`; `value` remains reserved for
actions whose canonical payload is a value.
Pilot interaction assertions use the shared browser-agent MCP result decoder;
observe-specific root fixtures no longer provide cross-feature test helpers.
DOM primitive parsing for index selection, scroll direction, and structured
responses is tested with the canonical DOM action owner; no root parser suite
mirrors its public parameter struct.
The same owner-level action-family table verifies required parameters,
selector-optional intent actions, canonical queued action payloads, and
correlation identity across every DOM primitive family.
Hardware-click validation, pilot gating, response correlation, and CDP-versus-
DOM routing are tested at that same owner. Action enumeration remains sourced
from the canonical interact schema instead of a root handler string check.
Clipboard read/write validation and pilot gates live with the browser-action
owner, including proof that blocked mutations do not record an action.
Readable text, Markdown, and page-summary extraction share one owner-level
routing contract for dedicated query types, tab forwarding, timeout defaults
and clamping, and the absence of CSP-fragile injected scripts.
Highlight, subtitle, and interactive-list response and failure contracts live
with their browser/DOM owners, including invalid JSON, pilot gating, tab
forwarding, queue type, and subtitle-clear semantics.
Subtitle responses expose the exact queued correlation identifier from that
owner. Async lifecycle markers and completed-command browser context promotion
are verified beside the canonical async-command response formatter.
Navigate and script-execution dispatch contracts live with the canonical
browser action owner, including action payloads, queue types, correlation
metadata, and invalid-input behavior.
Switch-tab tracking policy is also deterministic at that owner boundary:
successful retarget, explicit opt-out, command failure, invalid extension tab
identity, and background deferral require no polling goroutine or wall clock.
Composable subtitle and action-diff payloads, unique correlation IDs, and state
navigation acceptance/rejection are verified directly beside their owning
runtime and state modules; the former root-only helper environment is deleted.
Highlight, script execution, navigation, history, tab creation, and subtitle
validation and command routing are verified at the canonical browser-action
owner. The duplicate root page-command suite is deleted.
Every DOM action family—including paste and hover—has owner-level required
parameter, pilot-gate, query type, action payload, and correlation coverage.
The bounded external contract suite covers explore-page URL security, parameter
and tab forwarding, correlation identity, action recording, and guard ordering
without constructing the daemon or connecting an extension.
The former root audit suite and its skipped placeholders are deleted. Canonical
dispatch, browser, DOM, state, and schema owners now provide its real coverage;
rich-result tests use an explicitly connected shared fixture instead of hidden
connection state owned by an unrelated audit file.
Guard policy, readiness events, CSP response guidance, and cross-action gate
ordering now run at their canonical owners. The async-command constructor also
normalizes optional enrichment callbacks so disconnected or partial embeddings
cannot panic while producing a structured error.
Form, navigation, and accessibility/SARIF workflow validation lives with the
workflow owner. Its deterministic SARIF contract proves one accessibility
analysis is reused directly instead of issuing a duplicate browser query.
Draw-mode activation rejects malformed JSON before any state mutation and its
owner-level contract verifies pilot gating, tab/session forwarding, queue
metadata, and draw-lifecycle marking.
Insecure-proxy navigation contracts live with the browser action owner and
verify rejection without queue mutation, complete target encoding, and
consistent rewriting for both current-tab and new-tab navigation.
Interact mode resolution and composable response enrichment are owned by one
immutable per-handler dispatcher. It defensively copies the action surface,
injects side-effect timing for deterministic tests, preserves request fields,
and decorates only successful compatible actions; the root package now wires
capabilities without mutable handler caches or routing globals.
Overlay dismissal and stability waiting have owner-level queue contracts for
pilot gating, correlation identity, tab forwarding, default and custom timing,
and malformed-input rejection before mutation. Their navigate/click composition
is verified deterministically by the dispatcher owner without browser waits.
Screenshot composition is split across its true owners: the interact schema
proves the boolean input contract, the immutable dispatcher proves successful
post-action ordering, and the action runtime proves exact image-block transfer
plus deterministic preservation of the base response when no image is usable.
The dispatcher owner rejects malformed, missing, unknown, and obsolete mode
selectors before action execution. Required text, value, and attribute
parameters are verified beside the canonical DOM validation function.
Action jitter classification and bounds are verified directly on the action
runtime, while configure-owner tests prove clamping and setter invocation.
Storage and cookie mutation contracts are colocated with their canonical owner
and table-driven across queue type, exact script semantics, shared tab/world/
timeout forwarding, required parameters, and invalid storage types.
Snapshot response shapes live with the state owner, while list-interactive and
structured DOM query identity/forwarding live with their DOM and page owners.
Ambiguous-target candidate promotion, visible-candidate selection, and direct
retry guidance live with the async-result enrichment owner instead of an
end-to-end interact fixture.
