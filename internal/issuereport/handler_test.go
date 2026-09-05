// handler_test.go — Tests configure issue-report operation routing.

package issuereport

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/configure/capabilities"
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
		"missing title":   json.RawMessage(`{"operation":"submit","confirm":true}`),
		"unknown preview": json.RawMessage(`{"operation":"preview","template":"missing"}`),
		"unknown submit":  json.RawMessage(`{"operation":"submit","template":"missing","title":"broken","confirm":true}`),
		"unconfirmed":     json.RawMessage(`{"operation":"submit","template":"bug","title":"broken"}`),
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
	response := Handle(fakeHandlerDeps(), req, json.RawMessage(`{"operation":"submit","title":"Broken","template":"bug","confirm":true}`))
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

// recordingSubmitDeps counts how many times the outbound submit dependency is
// reached. That count is the transmission: Submit is the only path in this
// package that shells out to `gh issue create` against the public product repo.
func recordingSubmitDeps(calls *int) HandlerDeps {
	deps := fakeHandlerDeps()
	deps.Submit = func(report IssueReport) SubmitResult {
		*calls++
		return SubmitResult{Status: "submitted", IssueURL: report.Title}
	}
	return deps
}

// An unconfirmed call must not reach the network. Before the confirm gate,
// {"what":"report_issue","operation":"submit","title":"..."} ran
// `gh issue create` against brennhill/Kaboom-Browser-AI-Devtools-MCP with the
// caller's own GitHub credentials, so an agent enumerating configure modes
// could file a real public issue from a user's machine.
func TestSubmitWithoutConfirmTransmitsNothing(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	for name, args := range map[string]json.RawMessage{
		"list_templates":  json.RawMessage(`{"operation":"list_templates"}`),
		"default preview": nil,
		"preview":         json.RawMessage(`{"operation":"preview","template":"bug","user_context":"details"}`),
		"submit":          json.RawMessage(`{"operation":"submit","template":"bug","title":"Filed while exploring"}`),
		"confirm false":   json.RawMessage(`{"operation":"submit","template":"bug","title":"Filed while exploring","confirm":false}`),
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			Handle(recordingSubmitDeps(&calls), req, args)
			if calls != 0 {
				t.Fatalf("Handle(%s) reached the outbound submit path %d time(s); nothing may leave the machine without confirm=true", args, calls)
			}
		})
	}
}

// The refusal must say where the report would have gone and what unlocks it.
func TestUnconfirmedSubmitNamesTheDestinationAndTheGate(t *testing.T) {
	t.Parallel()
	calls := 0
	response := Handle(recordingSubmitDeps(&calls), mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)},
		json.RawMessage(`{"operation":"submit","template":"bug","title":"Broken"}`))
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("submit without confirm must fail loudly, not succeed quietly")
	}
	text := result.Content[0].Text
	for _, want := range []string{"confirm", TargetRepo, "preview"} {
		if !strings.Contains(text, want) {
			t.Fatalf("refusal does not mention %q: %s", want, text)
		}
	}
}

// confirm=true is the only path that transmits, and it still transmits.
func TestConfirmedSubmitTransmitsOnce(t *testing.T) {
	t.Parallel()
	calls := 0
	response := Handle(recordingSubmitDeps(&calls), mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)},
		json.RawMessage(`{"operation":"submit","template":"bug","title":"Broken","confirm":true}`))
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("confirmed submit failed: %s", result.Content[0].Text)
	}
	if calls != 1 {
		t.Fatalf("confirmed submit reached the outbound path %d time(s), want 1", calls)
	}
}

// A confirmed submit must still reject a missing title and an unknown template,
// so confirm cannot be used to push an unvalidated report onto the repo.
func TestConfirmedSubmitStillValidatesItsPayload(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	for name, args := range map[string]json.RawMessage{
		"missing title":    json.RawMessage(`{"operation":"submit","template":"bug","confirm":true}`),
		"unknown template": json.RawMessage(`{"operation":"submit","template":"missing","title":"Broken","confirm":true}`),
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			response := Handle(recordingSubmitDeps(&calls), req, args)
			var result mcp.MCPToolResult
			if err := json.Unmarshal(response.Result, &result); err != nil {
				t.Fatal(err)
			}
			if !result.IsError || calls != 0 {
				t.Fatalf("Handle(%s) = error:%v calls:%d; want a refusal that transmits nothing", args, result.IsError, calls)
			}
		})
	}
}

// reportIssueModeSpec returns the describe_capabilities entry an agent reads
// before deciding whether configure(what="report_issue") is safe to call.
func reportIssueModeSpec(t *testing.T) map[string]any {
	t.Helper()
	toolCap, ok := capabilities.BuildCapabilitiesForTool(schema.AllTools(), "configure")
	if !ok {
		t.Fatal("configure tool missing from the capability map")
	}
	modeCap, ok := capabilities.FilterToolByMode(toolCap, "configure", "report_issue")
	if !ok {
		t.Fatal("report_issue missing from configure mode_params")
	}
	return modeCap
}

// The mode spec and the handler must tell the same story. The mode spec used to
// read "A formatted issue body ready to file. Text only — nothing is submitted
// for you." while submit filed a real public issue on the product repo, so an
// agent that trusted describe_capabilities had every reason to call submit
// while exploring. This test fails if that claim comes back, if the gate the
// text promises stops being enforced, or if the destination changes without the
// text changing with it.
func TestModeSpecDescribesWhatSubmitActuallyDoes(t *testing.T) {
	t.Parallel()
	modeCap := reportIssueModeSpec(t)
	returns, ok := modeCap["returns"].(string)
	if !ok || returns == "" {
		t.Fatalf("report_issue states no response contract: %+v", modeCap)
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	unconfirmed, confirmed := 0, 0
	Handle(recordingSubmitDeps(&unconfirmed), req, json.RawMessage(`{"operation":"submit","template":"bug","title":"Probe"}`))
	Handle(recordingSubmitDeps(&confirmed), req, json.RawMessage(`{"operation":"submit","template":"bug","title":"Probe","confirm":true}`))

	if unconfirmed != 0 {
		t.Fatalf("submit transmitted %d time(s) without confirm; the mode spec promises a confirm gate", unconfirmed)
	}
	if confirmed != 1 {
		t.Fatalf("confirmed submit transmitted %d time(s), want 1", confirmed)
	}

	// Behaviour above proves the mode does transmit, so the text may not say it
	// does not.
	for _, denial := range []string{"nothing is submitted", "nothing is sent", "nothing leaves", "text only"} {
		if strings.Contains(strings.ToLower(returns), denial) {
			t.Fatalf("report_issue claims %q, but a confirmed submit files a real issue on %s: %q", denial, TargetRepo, returns)
		}
	}
	// And it must name the gate and the destination the behaviour enforces.
	for _, required := range []string{"confirm", TargetRepo} {
		if !strings.Contains(returns, required) {
			t.Fatalf("report_issue response contract omits %q: %q", required, returns)
		}
	}
	if !containsParam(modeCap["optional"], "confirm") {
		t.Fatalf("report_issue does not advertise the confirm param, so no caller can reach submit: %+v", modeCap["optional"])
	}
}

func containsParam(raw any, want string) bool {
	params, ok := raw.([]string)
	if !ok {
		return false
	}
	for _, param := range params {
		if param == want {
			return true
		}
	}
	return false
}
