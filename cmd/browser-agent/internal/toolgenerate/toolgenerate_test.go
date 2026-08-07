// toolgenerate_test.go — Unit tests for the generate-dispatch parameter validation API.

package toolgenerate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ---------------------------------------------------------------------------
// FilterGenerateDispatchWarnings
// ---------------------------------------------------------------------------

func TestFilterGenerateDispatchWarnings(t *testing.T) {
	tests := []struct {
		name     string
		warnings []string
		want     int
	}{
		{"nil", nil, 0},
		{"empty", []string{}, 0},
		{"only ignored", []string{
			"unknown parameter 'format' (ignored)",
			"unknown parameter 'what' (ignored)",
		}, 0},
		{"mixed", []string{
			"unknown parameter 'format' (ignored)",
			"unknown parameter 'bad_param' (ignored)",
			"some other warning",
		}, 2},
		{"non-matching format", []string{"random warning"}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterGenerateDispatchWarnings(tt.warnings)
			if len(got) != tt.want {
				t.Errorf("len(filtered) = %d, want %d, got %v", len(got), tt.want, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ParseUnknownParamWarning
// ---------------------------------------------------------------------------

func TestParseUnknownParamWarning(t *testing.T) {
	tests := []struct {
		input  string
		param  string
		wantOK bool
	}{
		{"unknown parameter 'format' (ignored)", "format", true},
		{"unknown parameter 'bad' (ignored)", "bad", true},
		{"unknown parameter '' (ignored)", "", false},
		{"random warning", "", false},
		{"unknown parameter 'x'", "", false}, // missing suffix
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			param, ok := ParseUnknownParamWarning(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ok: want %v, got %v", tt.wantOK, ok)
			}
			if param != tt.param {
				t.Errorf("param: want %q, got %q", tt.param, param)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ValidateGenerateParams
// ---------------------------------------------------------------------------

func TestValidateGenerateParams_ValidFormat(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	args := json.RawMessage(`{"what":"reproduction","format":"reproduction","last_n":10}`)

	resp := ValidateGenerateParams(req, "reproduction", args)
	if resp != nil {
		t.Errorf("expected nil (valid params), got error response")
	}
}

func TestValidateGenerateParams_UnknownParam(t *testing.T) {
	for _, test := range []struct {
		format string
		args   string
	}{
		{format: "reproduction", args: `{"what":"reproduction","bogus":true}`},
		{format: "test", args: `{"what":"test","scope":"page"}`},
		{format: "har", args: `{"what":"har","include_passes":true}`},
		{format: "csp", args: `{"what":"csp","resource_types":["script"]}`},
	} {
		resp := ValidateGenerateParams(mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, test.format, json.RawMessage(test.args))
		if resp == nil {
			t.Fatalf("%s accepted unknown parameter", test.format)
		}
		var result mcp.MCPToolResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatal(err)
		}
		if !result.IsError || !strings.Contains(result.Content[0].Text, mcp.ErrInvalidParam) {
			t.Fatalf("%s response = %#v", test.format, result)
		}
	}
}

func TestValidateGenerateParams_AcceptsFormatSpecificParameters(t *testing.T) {
	for _, test := range []struct {
		format string
		args   string
	}{
		{format: "reproduction", args: `{"what":"reproduction","error_message":"404","last_n":5}`},
		{format: "test", args: `{"what":"test","test_name":"login","telemetry_mode":"auto"}`},
		{format: "har", args: `{"what":"har","url":"/api","method":"GET"}`},
	} {
		if response := ValidateGenerateParams(mcp.JSONRPCRequest{ID: 1}, test.format, json.RawMessage(test.args)); response != nil {
			t.Fatalf("%s rejected valid parameters: %#v", test.format, response)
		}
	}
}

func TestValidateGenerateParams_EmptyArgs(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := ValidateGenerateParams(req, "reproduction", nil)
	if resp != nil {
		t.Error("expected nil for empty args")
	}
}

func TestValidateGenerateParams_UnknownFormat(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	args := json.RawMessage(`{"unknown_key":"val"}`)
	resp := ValidateGenerateParams(req, "nonexistent_format", args)
	if resp != nil {
		t.Error("expected nil for unknown format (handled elsewhere)")
	}
}

// ---------------------------------------------------------------------------
// GenerateValidParams coverage
// ---------------------------------------------------------------------------

func TestGenerateValidParams_AllFormatsPresent(t *testing.T) {
	expectedFormats := []string{
		"reproduction", "test", "pr_summary", "har", "csp", "sri", "sarif",
		"visual_test", "annotation_report", "annotation_issues",
		"test_from_context", "test_heal", "test_classify",
	}
	for _, format := range expectedFormats {
		if _, ok := GenerateValidParams[format]; !ok {
			t.Errorf("GenerateValidParams missing format %q", format)
		}
	}
}
