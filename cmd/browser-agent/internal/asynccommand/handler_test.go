// handler_test.go — Failure-transition tests for asynchronous command completion.

package asynccommand

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func TestFormatCommandResultPreservesCancellationDiagnosis(t *testing.T) {
	cap := capture.NewCapture()
	defer cap.Close()
	evidenceCalls := 0
	retryCalls := 0
	outcomes := make([]string, 0, 1)
	handler := New(Deps{
		Capture:              cap,
		DiagnosticHintString: func() string { return "doctor" },
		AttachEvidence:       func(string, map[string]any) { evidenceCalls++ },
		AttachRetryContext:   func(string, map[string]any, string, string) { retryCalls++ },
		RecordAsyncOutcome:   func(status string) { outcomes = append(outcomes, status) },
	})
	cmd := queries.CommandResult{
		CorrelationID: "cancel-correlation", TraceID: "cancel-trace", QueryID: "query-1",
		Status: "cancelled", Error: "navigation replaced the target document", CreatedAt: time.Now(),
	}
	resp := handler.FormatCommandResult(mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}, cmd, cmd.CorrelationID)
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("decode cancelled result: %v", err)
	}
	if !result.IsError || !strings.Contains(result.Content[0].Text, "navigation replaced") || !strings.Contains(result.Content[0].Text, "cancel-trace") {
		t.Fatalf("cancelled result = %+v", result)
	}
	if evidenceCalls != 1 || retryCalls != 1 || len(outcomes) != 1 || outcomes[0] != "cancelled" {
		t.Fatalf("enrichment calls evidence=%d retry=%d outcomes=%v", evidenceCalls, retryCalls, outcomes)
	}
}

func TestBuildA11yQueryParamsOmitsDefaultsAndPreservesTargets(t *testing.T) {
	t.Parallel()
	empty := BuildA11yQueryParams("", nil, nil, false)
	for _, key := range []string{"scope", "tags", "frame", "force_refresh"} {
		if _, exists := empty[key]; exists {
			t.Errorf("empty params unexpectedly include %q: %#v", key, empty)
		}
	}
	populated := BuildA11yQueryParams("#app", []string{"wcag2a"}, "iframe.editor", true)
	if populated["scope"] != "#app" || populated["frame"] != "iframe.editor" || populated["force_refresh"] != true {
		t.Fatalf("populated params = %#v", populated)
	}
	if tags, ok := populated["tags"].([]string); !ok || len(tags) != 1 || tags[0] != "wcag2a" {
		t.Fatalf("tags = %#v", populated["tags"])
	}
}
