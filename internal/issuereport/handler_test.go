// handler_test.go — Tests configure issue-report operation routing.

package issuereport

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func decodeHandlerResult(t *testing.T, response mcp.JSONRPCResponse) (mcp.MCPToolResult, map[string]any) {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Content) == 0 {
		t.Fatal("handler returned no content")
	}
	start := strings.IndexByte(result.Content[0].Text, '{')
	if start < 0 {
		t.Fatalf("handler response has no JSON payload: %q", result.Content[0].Text)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text[start:]), &data); err != nil {
		t.Fatal(err)
	}
	return result, data
}

func fakeHandlerDeps() HandlerDeps {
	return HandlerDeps{
		Collect: func(template, title, userContext string) IssueReport {
			return IssueReport{Template: template, Title: title, UserContext: userContext}
		},
		Sanitize: func(report IssueReport) IssueReport { return report },
		Submit: func(report IssueReport) SubmitResult {
			return SubmitResult{Status: "submitted", IssueURL: report.Title}
		},
	}
}

func TestHandleListsAndPreviewsKnownTemplates(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	for name, args := range map[string]json.RawMessage{
		"list":            json.RawMessage(`{"operation":"list_templates"}`),
		"default preview": nil,
		"named preview":   json.RawMessage(`{"operation":"preview","template":"feature_request","user_context":"details"}`),
	} {
		t.Run(name, func(t *testing.T) {
			response := Handle(fakeHandlerDeps(), req, args)
			var result mcp.MCPToolResult
			if err := json.Unmarshal(response.Result, &result); err != nil || result.IsError || len(result.Content) == 0 {
				t.Fatalf("Handle(%s) = %+v, %v", args, result, err)
			}
		})
	}
}

func TestHandleRejectsMalformedAndIncompleteIssueRequests(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 3}
	for name, args := range map[string]json.RawMessage{
		"malformed":       json.RawMessage(`not-json`),
		"missing title":   json.RawMessage(`{"operation":"submit"}`),
		"unknown preview": json.RawMessage(`{"operation":"preview","template":"missing"}`),
		"unknown submit":  json.RawMessage(`{"operation":"submit","template":"missing","title":"broken"}`),
	} {
		t.Run(name, func(t *testing.T) {
			response := Handle(fakeHandlerDeps(), req, args)
			var result mcp.MCPToolResult
			if err := json.Unmarshal(response.Result, &result); err != nil || !result.IsError {
				t.Fatalf("Handle(%s) = %+v, %v", args, result, err)
			}
			if strings.Contains(result.Content[0].Text, "private") {
				t.Fatal("error response leaked private context")
			}
		})
	}
}

func TestHandleRejectsUnknownOperation(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	response := Handle(fakeHandlerDeps(), req, json.RawMessage(`{"operation":"destroy"}`))
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected unknown operation to fail")
	}
}

func TestHandleSubmitsSanitizedReport(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	response := Handle(fakeHandlerDeps(), req, json.RawMessage(`{"operation":"submit","title":"Broken","template":"bug"}`))
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatal("expected submit to succeed")
	}
}

func TestHandlePreviewReturnsSanitizedDiagnosticsWithSnakeCaseFields(t *testing.T) {
	t.Parallel()
	const secret = "AKIAIOSFODNN7EXAMPLE"
	deps := fakeHandlerDeps()
	deps.Collect = func(template, title, userContext string) IssueReport {
		return IssueReport{
			Template:    template,
			Title:       title,
			UserContext: userContext,
			Diagnostics: DiagnosticData{
				Server:   ServerDiag{Version: "0.9.0", UptimeSeconds: 12, TotalCalls: 4},
				Platform: PlatformDiag{OS: "darwin", Arch: "arm64", GoVersion: "go1.25"},
			},
		}
	}
	deps.Sanitize = func(report IssueReport) IssueReport {
		report.UserContext = strings.ReplaceAll(report.UserContext, secret, "[REDACTED]")
		return report
	}

	response := Handle(deps, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 4}, json.RawMessage(
		`{"operation":"preview","template":"crash","user_context":"daemon froze with `+secret+`"}`,
	))
	result, data := decodeHandlerResult(t, response)
	if result.IsError {
		t.Fatalf("preview failed: %s", result.Content[0].Text)
	}
	if data["operation"] != "preview" || data["template"] != "crash" {
		t.Fatalf("unexpected preview metadata: %+v", data)
	}
	if strings.Contains(result.Content[0].Text, secret) || !strings.Contains(result.Content[0].Text, "[REDACTED]") {
		t.Fatalf("preview did not redact user context: %s", result.Content[0].Text)
	}
	report, ok := data["report"].(map[string]any)
	if !ok {
		t.Fatalf("preview report missing: %+v", data)
	}
	diagnostics, ok := report["diagnostics"].(map[string]any)
	if !ok {
		t.Fatalf("preview diagnostics missing: %+v", report)
	}
	server, ok := diagnostics["server"].(map[string]any)
	if !ok || server["version"] != "0.9.0" || server["uptime_seconds"] != float64(12) {
		t.Fatalf("unexpected server diagnostics: %+v", diagnostics["server"])
	}
	if _, found := server["uptimeSeconds"]; found {
		t.Fatalf("camelCase field leaked into wire response: %+v", server)
	}
	if body, ok := data["formatted_body"].(string); !ok || !strings.Contains(body, "daemon froze") {
		t.Fatalf("formatted preview body missing context: %v", data["formatted_body"])
	}
	if data["hint"] == "" {
		t.Fatal("preview recovery hint is missing")
	}
}
