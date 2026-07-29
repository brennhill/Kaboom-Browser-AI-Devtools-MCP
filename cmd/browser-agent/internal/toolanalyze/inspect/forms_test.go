// forms_test.go — Tests inspection response shaping at its package boundary.

package inspect

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func TestBuildFormValidationSummary(t *testing.T) {
	t.Parallel()
	forms, _ := json.Marshal(map[string]any{"forms": []any{
		map[string]any{"valid": true}, map[string]any{"valid": false},
	}})
	result, _ := json.Marshal(mcp.MCPToolResult{Content: []mcp.MCPContentBlock{
		{Type: "text", Text: "Form validation results\n" + string(forms)},
	}})
	response := BuildFormValidationSummary(mcp.JSONRPCResponse{JSONRPC: "2.0", Result: result})

	var shaped mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &shaped); err != nil {
		t.Fatal(err)
	}
	if len(shaped.Content) != 1 || shaped.Content[0].Text == "" {
		t.Fatalf("unexpected summary: %#v", shaped.Content)
	}
	var summary map[string]any
	text := shaped.Content[0].Text
	for index := range text {
		if text[index] == '{' {
			if err := json.Unmarshal([]byte(text[index:]), &summary); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if summary["total_forms"] != float64(2) || summary["valid"] != float64(1) || summary["invalid"] != float64(1) {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestExtractFormsListNested(t *testing.T) {
	t.Parallel()
	forms := ExtractFormsList(map[string]any{"result": map[string]any{
		"forms": []any{map[string]any{"id": "login"}},
	}})
	if len(forms) != 1 {
		t.Fatalf("forms = %#v", forms)
	}
}

func TestInspectionHandlersQueueCanonicalQueries(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	tests := []struct {
		name      string
		args      json.RawMessage
		queryType string
		call      func(Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
	}{
		{name: "computed styles", args: json.RawMessage(`{"selector":"body","tab_id":7}`), queryType: "computed_styles", call: HandleComputedStyles},
		{name: "form discovery", args: json.RawMessage(`{"tab_id":7}`), queryType: "form_discovery", call: HandleFormDiscovery},
		{name: "form state", args: json.RawMessage(`{"tab_id":7}`), queryType: "form_state", call: HandleFormState},
		{name: "data table", args: json.RawMessage(`{"tab_id":7}`), queryType: "data_table", call: HandleDataTable},
		{name: "form validation", args: json.RawMessage(`{"tab_id":7}`), queryType: "form_discovery", call: HandleFormValidation},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var queued queries.PendingQuery
			waited := false
			deps := Deps{
				EnqueuePendingQuery: func(_ mcp.JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (mcp.JSONRPCResponse, bool) {
					queued = query
					if timeout != queries.AsyncCommandTimeout {
						t.Fatalf("timeout = %s", timeout)
					}
					return mcp.JSONRPCResponse{}, false
				},
				MaybeWaitForCommand: func(_ mcp.JSONRPCRequest, correlationID string, args json.RawMessage, summary string) mcp.JSONRPCResponse {
					waited = true
					if correlationID == "" || len(args) == 0 || summary == "" {
						t.Fatalf("incomplete wait arguments: %q %s %q", correlationID, args, summary)
					}
					return mcp.JSONRPCResponse{JSONRPC: mcp.JSONRPCVersion, ID: req.ID}
				},
			}
			response := tc.call(deps, req, tc.args)
			if !waited || queued.Type != tc.queryType || queued.TabID != 7 || queued.CorrelationID == "" {
				t.Fatalf("queued = %+v, waited = %v", queued, waited)
			}
			if response.ID != req.ID {
				t.Fatalf("response ID = %v", response.ID)
			}
			if tc.name == "form validation" && !strings.Contains(string(queued.Params), `"mode":"validate"`) {
				t.Fatalf("validation params = %s", queued.Params)
			}
		})
	}
}

func TestInspectionHandlersReturnValidationAndQueueErrors(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: "test"}
	bad := json.RawMessage(`{bad`)
	for name, call := range map[string]func(Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse{
		"computed styles": HandleComputedStyles,
		"form discovery":  HandleFormDiscovery,
		"form state":      HandleFormState,
		"data table":      HandleDataTable,
		"form validation": HandleFormValidation,
	} {
		t.Run(name, func(t *testing.T) {
			response := call(Deps{}, req, bad)
			if !strings.Contains(string(response.Result), `"isError":true`) {
				t.Fatal("expected validation error")
			}
		})
	}

	blocked := mcp.JSONRPCResponse{JSONRPC: mcp.JSONRPCVersion, ID: req.ID, Error: &mcp.JSONRPCError{Code: -1}}
	response := HandleFormDiscovery(Deps{
		EnqueuePendingQuery: func(mcp.JSONRPCRequest, queries.PendingQuery, time.Duration) (mcp.JSONRPCResponse, bool) {
			return blocked, true
		},
		MaybeWaitForCommand: func(mcp.JSONRPCRequest, string, json.RawMessage, string) mcp.JSONRPCResponse {
			t.Fatal("wait called for blocked query")
			return mcp.JSONRPCResponse{}
		},
	}, req, json.RawMessage(`{}`))
	if response.Error == nil {
		t.Fatal("blocked response was not returned")
	}
}

func TestBuildFormValidationSummaryLeavesUnshapableResponsesAlone(t *testing.T) {
	t.Parallel()
	cases := []mcp.JSONRPCResponse{
		{Result: json.RawMessage(`not-json`)},
		{Result: mustJSON(t, mcp.MCPToolResult{IsError: true})},
		{Result: mustJSON(t, mcp.MCPToolResult{Content: []mcp.MCPContentBlock{{Type: "text", Text: "no object"}}})},
		{Result: mustJSON(t, mcp.MCPToolResult{Content: []mcp.MCPContentBlock{{Type: "text", Text: "{bad"}}})},
	}
	for _, response := range cases {
		before := string(response.Result)
		if got := BuildFormValidationSummary(response); string(got.Result) != before {
			t.Fatalf("response changed: %s -> %s", before, got.Result)
		}
	}
	if forms := ExtractFormsList(map[string]any{"result": map[string]any{"result": map[string]any{"forms": []any{"nested"}}}}); len(forms) != 1 {
		t.Fatalf("deep forms = %#v", forms)
	}
	if forms := ExtractFormsList(map[string]any{"result": "wrong"}); forms != nil {
		t.Fatalf("wrong result forms = %#v", forms)
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
