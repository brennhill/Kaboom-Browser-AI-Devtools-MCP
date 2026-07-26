// toolgenerate_test.go — Unit tests for the generate-dispatch parameter validation API.

package toolgenerate

import (
	"encoding/json"
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
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	args := json.RawMessage(`{"format":"reproduction","bogus_param":true}`)

	resp := ValidateGenerateParams(req, "reproduction", args)
	if resp == nil {
		t.Fatal("expected error response for unknown param")
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
