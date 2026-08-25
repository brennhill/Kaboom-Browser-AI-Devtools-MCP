// forms.go — Structured computed-style, form, and data-table inspection handlers.
// Docs: docs/features/feature/analyze-tool/index.md

package inspect

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	analyze "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/analyze"
)

type Deps struct {
	EnqueuePendingQuery func(mcp.JSONRPCRequest, queries.PendingQuery, time.Duration) (mcp.JSONRPCResponse, bool)
	MaybeWaitForCommand func(mcp.JSONRPCRequest, string, json.RawMessage, string) mcp.JSONRPCResponse
}

// queueSpec describes one inspection command to queue: the correlation ID
// prefix, the extension query type, and the summary shown while waiting.
type queueSpec struct {
	correlationPrefix string
	queryType         string
	summary           string
}

func queue(d Deps, req mcp.JSONRPCRequest, spec queueSpec, args json.RawMessage, tabID int) mcp.JSONRPCResponse {
	correlationID := toolresp.NewCorrelationID(spec.correlationPrefix)
	query := queries.PendingQuery{Type: spec.queryType, Params: args, TabID: tabID, CorrelationID: correlationID}
	if response, blocked := d.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return response
	}
	return d.MaybeWaitForCommand(req, correlationID, args, spec.summary)
}

func HandleComputedStyles(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	parsed, err := analyze.ParseComputedStylesArgs(args)
	if err != nil {
		return mcp.Fail(req, mcp.ErrMissingParam, err.Error(), "Add the 'selector' parameter with a CSS selector", mcp.WithParam("selector"))
	}
	return queue(d, req, queueSpec{correlationPrefix: "computed_styles", queryType: "computed_styles", summary: "Computed styles query queued"}, args, parsed.TabID)
}

func HandleFormDiscovery(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	parsed, err := analyze.ParseFormsArgs(args)
	if err != nil {
		return invalidJSON(req, err)
	}
	return queue(d, req, queueSpec{correlationPrefix: "form_discovery", queryType: "form_discovery", summary: "Form discovery queued"}, args, parsed.TabID)
}

func HandleFormState(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	parsed, err := analyze.ParseFormsArgs(args)
	if err != nil {
		return invalidJSON(req, err)
	}
	return queue(d, req, queueSpec{correlationPrefix: "form_state", queryType: "form_state", summary: "Form state extraction queued"}, args, parsed.TabID)
}

func HandleDataTable(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	parsed, err := analyze.ParseDataTableArgs(args)
	if err != nil {
		return invalidJSON(req, err)
	}
	return queue(d, req, queueSpec{correlationPrefix: "data_table", queryType: "data_table", summary: "Data table extraction queued"}, args, parsed.TabID)
}

func HandleFormValidation(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	parsed, err := analyze.ParseFormValidationArgs(args)
	if err != nil {
		return invalidJSON(req, err)
	}
	var params map[string]any
	if json.Unmarshal(args, &params) == nil {
		params["mode"] = "validate"
	}
	augmentedArgs, _ := json.Marshal(params)
	wantSummary, _ := params["summary"].(bool)
	response := queue(d, req, queueSpec{correlationPrefix: "form_validation", queryType: "form_discovery", summary: "Form validation queued"}, augmentedArgs, parsed.TabID)
	if wantSummary {
		return BuildFormValidationSummary(response)
	}
	return response
}

func invalidJSON(req mcp.JSONRPCRequest, err error) mcp.JSONRPCResponse {
	return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
}

func BuildFormValidationSummary(response mcp.JSONRPCResponse) mcp.JSONRPCResponse {
	var result mcp.MCPToolResult
	if json.Unmarshal(response.Result, &result) != nil || result.IsError {
		return response
	}
	for _, block := range result.Content {
		jsonStart := -1
		for index, character := range block.Text {
			if character == '{' {
				jsonStart = index
				break
			}
		}
		if jsonStart < 0 {
			continue
		}
		var data map[string]any
		if json.Unmarshal([]byte(block.Text[jsonStart:]), &data) != nil {
			continue
		}
		forms := ExtractFormsList(data)
		if forms == nil {
			continue
		}
		valid := 0
		for _, form := range forms {
			fields, ok := form.(map[string]any)
			if isValid, exists := fields["valid"].(bool); ok && exists && isValid {
				valid++
			}
		}
		summary, _ := json.Marshal(map[string]any{
			"total_forms": len(forms), "valid": valid, "invalid": len(forms) - valid,
		})
		result.Content = []mcp.MCPContentBlock{{Type: "text", Text: "Form validation summary\n" + string(summary)}}
		updated, err := json.Marshal(result)
		if err == nil {
			response.Result = updated
		}
		return response
	}
	return response
}

func ExtractFormsList(data map[string]any) []any {
	if forms, ok := data["forms"].([]any); ok {
		return forms
	}
	result, ok := data["result"].(map[string]any)
	if !ok {
		return nil
	}
	if forms, ok := result["forms"].([]any); ok {
		return forms
	}
	if inner, ok := result["result"].(map[string]any); ok {
		if forms, ok := inner["forms"].([]any); ok {
			return forms
		}
	}
	return nil
}
