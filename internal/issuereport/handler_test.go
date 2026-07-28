// handler_test.go — Tests configure issue-report operation routing.

package issuereport

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

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
