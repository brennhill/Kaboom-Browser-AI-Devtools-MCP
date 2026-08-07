// handler_test.go — Failure-transition tests for asynchronous command completion.

package asynccommand

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
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

func TestFormatCommandResultLifecycleAndEffectiveContext(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	defer cap.Close()
	handler := New(Deps{
		Capture:              cap,
		DiagnosticHintString: func() string { return "doctor" },
		AttachEvidence:       func(string, map[string]any) {},
		AttachRetryContext:   func(string, map[string]any, string, string) {},
	})

	tests := []struct {
		name       string
		command    queries.CommandResult
		wantError  bool
		wantFinal  bool
		wantStatus string
	}{
		{
			name: "completed result promotes effective browser context",
			command: queries.CommandResult{
				CorrelationID: "complete-1",
				Status:        "complete",
				Result: json.RawMessage(`{
					"success":true,
					"resolved_tab_id":42,
					"resolved_url":"https://example.test/before",
					"effective_tab_id":42,
					"effective_url":"https://example.test/after"
				}`),
			},
			wantFinal:  true,
			wantStatus: "complete",
		},
		{
			name: "failed result is terminal",
			command: queries.CommandResult{
				CorrelationID: "error-1",
				Status:        "error",
				Error:         "element_not_found",
			},
			wantError:  true,
			wantFinal:  true,
			wantStatus: "error",
		},
		{
			name: "pending result remains nonterminal",
			command: queries.CommandResult{
				CorrelationID: "pending-1",
				Status:        "pending",
			},
			wantFinal:  false,
			wantStatus: "pending",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			response := handler.FormatCommandResult(
				mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1},
				testCase.command,
				testCase.command.CorrelationID,
			)
			var result mcp.MCPToolResult
			if err := json.Unmarshal(response.Result, &result); err != nil {
				t.Fatalf("decode tool result: %v", err)
			}
			if result.IsError != testCase.wantError {
				t.Fatalf("is_error = %v, want %v", result.IsError, testCase.wantError)
			}
			data := decodeResponseData(t, result)
			if data["status"] != testCase.wantStatus || data["final"] != testCase.wantFinal || data["queued"] != false {
				t.Fatalf("lifecycle data = %#v", data)
			}
			if testCase.command.CorrelationID == "complete-1" {
				if data["effective_tab_id"] != float64(42) || data["effective_url"] != "https://example.test/after" || data["resolved_url"] != "https://example.test/before" {
					t.Fatalf("effective context = %#v", data)
				}
			}
		})
	}
}

func TestMaybeWaitForCommandBackgroundResponseIsQueued(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	defer cap.Close()
	handler := New(Deps{Capture: cap})
	response := handler.MaybeWaitForCommand(
		mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1},
		"click-1",
		json.RawMessage(`{"background":true}`),
		"Click queued",
	)
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatalf("decode tool result: %v", err)
	}
	data := decodeResponseData(t, result)
	if data["status"] != "queued" || data["queued"] != true || data["final"] != false || data["correlation_id"] != "click-1" {
		t.Fatalf("queued lifecycle data = %#v", data)
	}
}

func TestMaybeWaitForCommandUsesCurrentConnectionAndResultEvents(t *testing.T) {
	t.Run("disconnected fails without waiting for command expiry", func(t *testing.T) {
		captured := capture.NewCapture()
		defer captured.Close()
		correlationID := "disconnected"
		captured.Queries().RegisterCommand(correlationID, "query-disconnected", time.Hour)
		response := New(Deps{Capture: captured}).MaybeWaitForCommand(
			mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}, correlationID, json.RawMessage(`{}`), "queued",
		)
		result := decodeToolResult(t, response)
		if !result.IsError || !strings.Contains(result.Content[0].Text, mcp.ErrNoData) {
			t.Fatalf("disconnected result = %#v", result)
		}
	})

	t.Run("connected command completes from dispatcher event", func(t *testing.T) {
		captured := capture.NewCapture()
		defer captured.Close()
		capturefixture.Connect(captured)
		correlationID := "connected"
		captured.Queries().RegisterCommand(correlationID, "query-connected", time.Hour)
		responses := make(chan mcp.JSONRPCResponse, 1)
		go func() {
			responses <- New(Deps{Capture: captured}).MaybeWaitForCommand(
				mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}, correlationID, json.RawMessage(`{}`), "queued",
			)
		}()
		captured.Queries().ApplyCommandResult(correlationID, "complete", json.RawMessage(`{"success":true}`), "")
		result := decodeToolResult(t, <-responses)
		if result.IsError || decodeResponseData(t, result)["status"] != "complete" {
			t.Fatalf("connected result = %#v", result)
		}
	})
}

func decodeToolResult(t *testing.T, response mcp.JSONRPCResponse) mcp.MCPToolResult {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatalf("decode tool result: %v", err)
	}
	return result
}

func decodeResponseData(t *testing.T, result mcp.MCPToolResult) map[string]any {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("tool result has no content")
	}
	start := strings.IndexByte(result.Content[0].Text, '{')
	if start < 0 {
		t.Fatalf("response has no structured data: %q", result.Content[0].Text)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text[start:]), &data); err != nil {
		t.Fatalf("decode response data: %v", err)
	}
	return data
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
