// Purpose: Dispatches IndexedDB queries to the extension and formats database/store enumeration responses.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/idbquery"
)

// GetIndexedDB returns rows from one IndexedDB object store.
func GetIndexedDB(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Database string `json:"database"`
		Store    string `json:"store"`
		Limit    int    `json:"limit"`
	}
	mcp.LenientUnmarshal(args, &params)

	if params.Database == "" {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrMissingParam,
			"Required parameter 'database' is missing for observe(what='indexeddb')",
			"Add the 'database' parameter and call again.",
			mcp.WithParam("database"),
		)}
	}
	if params.Store == "" {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrMissingParam,
			"Required parameter 'store' is missing for observe(what='indexeddb')",
			"Add the 'store' parameter and call again.",
			mcp.WithParam("store"),
		)}
	}
	params.Limit = clampLimit(params.Limit, 100)

	cap := deps.GetCapture()
	enabled, _, _ := cap.GetTrackingStatus()
	if !enabled {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrNoData,
			"No tab is being tracked. Open the Kaboom extension popup and click 'Track This Tab'.",
			"Track a tab first, then call observe with what='indexeddb'.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)}
	}

	storeData, err := idbquery.Entries(cap, params.Database, params.Store, params.Limit)
	if err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrExtError,
			"IndexedDB inspection failed: "+err.Error(),
			"Ensure the tab is accessible and the database/store names are correct.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)}
	}

	entries, _ := storeData["entries"].([]any)
	count := len(entries)
	if c, ok := toInt(storeData["count"]); ok {
		count = c
	}

	response := map[string]any{
		"database": params.Database,
		"store":    params.Store,
		"entries":  entries,
		"count":    count,
		"limit":    params.Limit,
		"metadata": BuildResponseMetadata(cap, time.Now()),
	}
	if v, ok := storeData["object_stores"]; ok {
		response["object_stores"] = v
	}

	return mcp.Succeed(req, "IndexedDB entries", response)
}

// toInt reads a count out of the page's reply, which arrives as float64 through
// encoding/json but may be any numeric kind when the value is synthesized server-side.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}
