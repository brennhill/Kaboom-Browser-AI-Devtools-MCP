// Purpose: Handles observe(what:"storage") mode: reads localStorage/sessionStorage and returns filtered entries.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/idbquery"
)

// summarizeStorageMap returns a summary of a key-value storage map.
func summarizeStorageMap(data map[string]any) map[string]any {
	keys := make([]string, 0, len(data))
	totalBytes := 0
	for k, v := range data {
		keys = append(keys, k)
		totalBytes += len(k)
		if s, ok := v.(string); ok {
			totalBytes += len(s)
		} else {
			b, _ := json.Marshal(v)
			totalBytes += len(b)
		}
	}
	sampleKeys := keys
	if len(sampleKeys) > 5 {
		sampleKeys = sampleKeys[:5]
	}
	return map[string]any{
		"key_count":   len(data),
		"total_bytes": totalBytes,
		"sample_keys": sampleKeys,
	}
}

// summarizeCookies returns a summary of a cookie array.
func summarizeCookies(cookies []any) map[string]any {
	names := make([]string, 0, len(cookies))
	totalBytes := 0
	for _, c := range cookies {
		if m, ok := c.(map[string]any); ok {
			if name, ok := m["name"].(string); ok {
				names = append(names, name)
			}
			b, _ := json.Marshal(m)
			totalBytes += len(b)
		}
	}
	sampleKeys := names
	if len(sampleKeys) > 5 {
		sampleKeys = sampleKeys[:5]
	}
	return map[string]any{
		"key_count":   len(cookies),
		"total_bytes": totalBytes,
		"sample_keys": sampleKeys,
	}
}

// storageParams holds parsed parameters for storage queries.
type storageParams struct {
	Summary     bool
	StorageType string // "local", "session", "cookies", or "" for all
	Key         string // specific key/cookie name filter
}

func parseStorageParams(args json.RawMessage) storageParams {
	p := storageParams{Summary: true}
	if len(args) == 0 {
		return p
	}
	var raw struct {
		Summary     *bool  `json:"summary"`
		StorageType string `json:"storage_type"`
		Key         string `json:"key"`
	}
	if json.Unmarshal(args, &raw) == nil {
		if raw.Summary != nil {
			p.Summary = *raw.Summary
		}
		p.StorageType = raw.StorageType
		p.Key = raw.Key
	}
	return p
}

// filterStorageMap filters a storage map by key. Returns nil if key not found.
func filterStorageMap(data map[string]any, key string) map[string]any {
	if key == "" {
		return data
	}
	if v, ok := data[key]; ok {
		return map[string]any{key: v}
	}
	return map[string]any{}
}

// filterCookies filters a cookie array by name.
func filterCookies(cookies []any, name string) []any {
	if name == "" {
		return cookies
	}
	var filtered []any
	for _, c := range cookies {
		if m, ok := c.(map[string]any); ok {
			if n, ok := m["name"].(string); ok && n == name {
				filtered = append(filtered, c)
			}
		}
	}
	return filtered
}

// GetStorage returns localStorage, sessionStorage, and cookies from the tracked tab.
func GetStorage(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	params := parseStorageParams(args)
	cap := deps.GetCapture()
	enabled, _, _ := cap.GetTrackingStatus()
	if !enabled {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrNoData,
			"No tab is being tracked. Open the Kaboom extension popup and click 'Track This Tab'.",
			"Track a tab first, then call observe with what='storage'.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)}
	}

	queryID, qerr := cap.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "state_capture",
			Params: json.RawMessage(`{"action":"capture"}`),
		},
		10*time.Second,
		"",
	)
	if qerr != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrQueueFull,
			"Command queue full: "+qerr.Error(),
			"Wait for in-flight commands to complete, then retry.",
			mcp.WithRecoveryToolCall(map[string]any{"tool": "observe", "arguments": map[string]any{"what": "pending_commands"}}),
		)}
	}

	result, err := cap.WaitForResult(queryID, 10*time.Second)
	if err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrExtTimeout,
			"Storage capture timeout: "+err.Error(),
			"Ensure the extension is connected and the page has loaded.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)}
	}

	var stateResult map[string]any
	if err := json.Unmarshal(result, &stateResult); err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrInvalidJSON,
			"Failed to parse storage result: "+err.Error(),
			"Check extension logs for errors",
		)}
	}

	if errMsg, ok := stateResult["error"].(string); ok {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrExtError,
			"Storage capture failed: "+errMsg,
			"Check that the tab is accessible.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)}
	}

	response := map[string]any{
		"url":      stateResult["url"],
		"metadata": BuildResponseMetadata(cap, time.Now()),
	}

	includeLocal := params.StorageType == "" || params.StorageType == "local"
	includeSession := params.StorageType == "" || params.StorageType == "session"
	includeCookies := params.StorageType == "" || params.StorageType == "cookies"

	if params.Summary {
		if includeLocal {
			if v, ok := stateResult["localStorage"].(map[string]any); ok {
				response["local_storage"] = summarizeStorageMap(filterStorageMap(v, params.Key))
			}
		}
		if includeSession {
			if v, ok := stateResult["sessionStorage"].(map[string]any); ok {
				response["session_storage"] = summarizeStorageMap(filterStorageMap(v, params.Key))
			}
		}
		if includeCookies {
			if v, ok := stateResult["cookies"].([]any); ok {
				response["cookies"] = summarizeCookies(filterCookies(v, params.Key))
			}
		}
	} else {
		if includeLocal {
			if v, ok := stateResult["localStorage"].(map[string]any); ok {
				response["local_storage"] = filterStorageMap(v, params.Key)
			}
		}
		if includeSession {
			if v, ok := stateResult["sessionStorage"].(map[string]any); ok {
				response["session_storage"] = filterStorageMap(v, params.Key)
			}
		}
		if includeCookies {
			if v, ok := stateResult["cookies"].([]any); ok {
				response["cookies"] = filterCookies(v, params.Key)
			}
		}
	}

	// IndexedDB listing is best-effort (skip if storage_type filter excludes it)
	if params.StorageType == "" {
		if indexeddb, err := idbquery.Listing(cap); err != nil {
			response["indexeddb"] = map[string]any{
				"supported": false,
				"databases": []any{},
			}
			response["indexeddb_error"] = err.Error()
		} else {
			response["indexeddb"] = indexeddb
		}
	}

	return mcp.Succeed(req, "Browser storage", response)
}
