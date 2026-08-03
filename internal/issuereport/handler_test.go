// handler_test.go — Tests configure issue-report operation routing.

package issuereport

import (
	"encoding/json"
	"strings"
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
