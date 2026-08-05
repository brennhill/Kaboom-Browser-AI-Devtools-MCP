// Purpose: Package observe — the implementation of the observe MCP tool's twenty read modes.
// Why: Centralizes observe query behavior so evidence filtering, pagination and response shape stay
// predictable across every mode.
// Docs: docs/features/feature/observe/index.md

/*
Package observe implements the observe MCP tool, which answers questions about a
tracked browser tab. Most modes read a server-side buffer that the extension has
been filling; a few round-trip to the live page.

Every mode has the same shape:

	func GetX(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse

It takes Deps (satisfied by *ToolHandler in cmd/browser-agent), clamps its limit
through clampLimit, narrows the buffer with the predicates in filtering.go,
stamps freshness on the reply with BuildResponseMetadata, and — when the result
is empty — attaches an explanation from the hints subpackage.

Source layout, by telemetry domain:

	deps.go             Deps, MaxObserveLimit, clampLimit
	filtering.go        LogLevelRank, ContainsIgnoreCase, ApplyNetworkBodyFilter
	metadata.go         ResponseMetadata and its paginated spellings
	logs.go             errors, logs, extension_logs, error_clusters
	summarized_logs.go  summarized_logs: fingerprint, group, detect periodicity
	network.go          network_waterfall, network_bodies, websocket_events, websocket_status
	session.go          actions, transients, pilot, history, vitals, tabs, performance
	correlation.go      error_bundles and timeline — modes that join streams on time
	page_state.go       page, storage, indexeddb, screenshot, accessibility

Subpackages:

	hints     the "why is this empty?" copy attached to empty responses
	idbquery  IndexedDB reads, the one path that runs generated JS in the page

Four of the exported handlers back analyze tool modes rather than observe modes —
RunA11yAudit, CheckPerformance, AnalyzeErrors and AnalyzeHistory — because they
read the same buffers. cmd/browser-agent registers them under analyze.
*/
package core
