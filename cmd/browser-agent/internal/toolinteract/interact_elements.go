// Purpose: Implements list_interactive with indexed element store, scope filtering, and index-to-selector resolution.
// Why: Provides stable numeric element handles that agents can reference across multiple interact calls.
// Docs: docs/features/feature/interact-explore/index.md
package toolinteract

import (
	"encoding/json"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract/elemindex"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

func (h *InteractActionHandler) HandleListInteractive(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID       int  `json:"tab_id,omitempty"`
		VisibleOnly bool `json:"visible_only,omitempty"`
		Limit       int  `json:"limit,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	args = normalizeDOMActionArgs(args, "list_interactive")

	resp, correlationID := h.newCommand("list_interactive").
		correlationPrefix("dom_list").
		reason("list_interactive").
		queryType("dom_action").
		queryParams(args).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
		recordAction("dom_list_interactive", "", nil).
		queuedMessage("list_interactive queued").
		executeWithCorrelation(req, args)

	// Post-process: extract elements from result and build index→selector store.
	// IMPORTANT: index store is built from ALL elements before truncation.
	indexGeneration := h.buildElementIndexFromResponse(req.ClientID, params.TabID, correlationID, resp)
	if indexGeneration != "" {
		resp = annotateListInteractiveIndexMetadata(resp, params.TabID, indexGeneration)
	}

	if params.Limit > 0 {
		resp = truncateListInteractiveResponse(resp, params.Limit)
	}

	return resp
}

// buildElementIndexFromResponse parses list_interactive results and stores scoped index selectors.
func (h *InteractActionHandler) buildElementIndexFromResponse(clientID string, tabID int, generation string, resp JSONRPCResponse) string {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || result.IsError {
		return ""
	}

	for _, block := range result.Content {
		idx := strings.Index(block.Text, "{")
		if idx < 0 {
			continue
		}
		jsonStr := block.Text[idx:]

		var data map[string]any
		if json.Unmarshal([]byte(jsonStr), &data) != nil {
			continue
		}

		elements := extractElementList(data)
		if elements == nil {
			continue
		}

		indexMap := make(map[int]string, len(elements))
		for _, elem := range elements {
			elemMap, ok := elem.(map[string]any)
			if !ok {
				continue
			}
			indexVal, _ := elemMap["index"].(float64)
			selector, _ := elemMap["selector"].(string)
			if selector != "" {
				indexMap[int(indexVal)] = selector
			}
		}
		if h.elementIndexRegistry == nil {
			h.elementIndexRegistry = elemindex.New()
		}
		return h.elementIndexRegistry.Store(clientID, tabID, generation, indexMap)
	}
	return ""
}

func annotateListInteractiveIndexMetadata(resp JSONRPCResponse, tabID int, generation string) JSONRPCResponse {
	if generation == "" {
		return resp
	}
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || result.IsError {
		return resp
	}
	for i, block := range result.Content {
		idx := strings.Index(block.Text, "{")
		if idx < 0 {
			continue
		}
		jsonStr := block.Text[idx:]
		prefix := block.Text[:idx]

		var data map[string]any
		if json.Unmarshal([]byte(jsonStr), &data) != nil {
			continue
		}
		data["index_generation"] = generation
		data["index_scope_tab_id"] = tabID
		newJSON, err := json.Marshal(data)
		if err != nil {
			continue
		}
		result.Content[i].Text = prefix + string(newJSON)
		newResult, _ := json.Marshal(result)
		resp.Result = newResult
		return resp
	}
	return resp
}

// truncateListInteractiveResponse limits the elements array in a list_interactive response.
func truncateListInteractiveResponse(resp JSONRPCResponse, limit int) JSONRPCResponse {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || result.IsError {
		return resp
	}

	for i, block := range result.Content {
		idx := strings.Index(block.Text, "{")
		if idx < 0 {
			continue
		}
		jsonStr := block.Text[idx:]
		prefix := block.Text[:idx]

		var data map[string]any
		if json.Unmarshal([]byte(jsonStr), &data) != nil {
			continue
		}

		elements := extractElementList(data)
		if elements == nil || len(elements) <= limit {
			continue
		}

		total := len(elements)
		setNestedElements(data, elements[:limit])
		data["total"] = total
		data["truncated"] = true

		newJSON, err := json.Marshal(data)
		if err != nil {
			continue
		}
		result.Content[i].Text = prefix + string(newJSON)
		newResult, _ := json.Marshal(result)
		resp.Result = newResult
		return resp
	}

	return resp
}

// setNestedElements updates the elements array at whatever nesting level it was found.
func setNestedElements(data map[string]any, elements []any) {
	if _, ok := data["elements"]; ok {
		data["elements"] = elements
		return
	}
	if r, ok := data["result"].(map[string]any); ok {
		if _, ok := r["elements"]; ok {
			r["elements"] = elements
			return
		}
		if rr, ok := r["result"].(map[string]any); ok {
			if _, ok := rr["elements"]; ok {
				rr["elements"] = elements
				return
			}
		}
	}
}

// extractElementList — delegated to internal/tools/interact package.
var extractElementList = act.ExtractElementList

// resolveIndexToSelector looks up a selector from the scoped element index store.
func (h *InteractActionHandler) resolveIndexToSelector(clientID string, tabID int, index int, generation string) (string, bool, bool, string) {
	if h.elementIndexRegistry == nil {
		return "", false, false, ""
	}
	return h.elementIndexRegistry.Resolve(clientID, tabID, index, generation)
}
