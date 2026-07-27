// tools_configure_report_issue_test.go — Tests for configure(what="report_issue") handler.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/issuereport"
)

// fakeIssueRunner implements issuereport.CommandRunner for handler tests.
type fakeIssueRunner struct {
	lookPathErr error
	stdout      string
	stderr      string
	runErr      error
}

func (r *fakeIssueRunner) LookPath(_ string) (string, error) {
	if r.lookPathErr != nil {
		return "", r.lookPathErr
	}
	return "/usr/bin/gh", nil
}

func (r *fakeIssueRunner) Run(_ context.Context, name string, args ...string) (string, string, error) {
	return r.stdout, r.stderr, r.runErr
}

func makeToolHandlerWithIssueRunner(t *testing.T, runner issuereport.CommandRunner) *ToolHandler {
	t.Helper()
	h, _, _ := makeToolHandler(t)
	h.issueCommandRunner = runner
	return h
}

func TestReportIssue_ListTemplates(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"report_issue","operation":"list_templates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	count, ok := data["count"].(float64)
	if !ok || count != 5 {
		t.Fatalf("count = %v, want 5", data["count"])
	}

	templates, ok := data["templates"].([]any)
	if !ok || len(templates) != 5 {
		t.Fatalf("templates length = %v, want 5", len(templates))
	}

	// Verify each template has required fields
	for i, tmpl := range templates {
		m, ok := tmpl.(map[string]any)
		if !ok {
			t.Fatalf("template[%d] is not a map", i)
		}
		if m["name"] == nil || m["name"] == "" {
			t.Errorf("template[%d] missing name", i)
		}
		if m["title"] == nil || m["title"] == "" {
			t.Errorf("template[%d] missing title", i)
		}
		if m["description"] == nil || m["description"] == "" {
			t.Errorf("template[%d] missing description", i)
		}
	}
}

func TestReportIssue_PreviewDefault(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"report_issue"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	if data["operation"] != "preview" {
		t.Fatalf("operation = %v, want preview", data["operation"])
	}
	if data["formatted_body"] == nil || data["formatted_body"] == "" {
		t.Fatal("formatted_body is missing or empty")
	}
	if data["hint"] == nil {
		t.Fatal("hint is missing")
	}
}

func TestReportIssue_PreviewWithTemplate(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"report_issue","operation":"preview","template":"crash","user_context":"daemon froze"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	if data["template"] != "crash" {
		t.Fatalf("template = %v, want crash", data["template"])
	}

	body, _ := data["formatted_body"].(string)
	if !strings.Contains(body, "daemon froze") {
		t.Fatal("formatted_body should contain user_context")
	}
}

func TestReportIssue_PreviewContainsDiagnostics(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"report_issue","operation":"preview","template":"bug"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	report, ok := data["report"].(map[string]any)
	if !ok {
		t.Fatal("report field missing or not a map")
	}

	diag, ok := report["diagnostics"].(map[string]any)
	if !ok {
		t.Fatal("diagnostics missing or not a map")
	}

	server, ok := diag["server"].(map[string]any)
	if !ok {
		t.Fatal("diagnostics.server missing")
	}
	if server["version"] == nil || server["version"] == "" {
		t.Error("diagnostics.server.version is empty")
	}

	platform, ok := diag["platform"].(map[string]any)
	if !ok {
		t.Fatal("diagnostics.platform missing")
	}
	if platform["os"] == nil || platform["os"] == "" {
		t.Error("diagnostics.platform.os is empty")
	}
}

func TestReportIssue_PreviewRedactsSecrets(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// Inject a fake secret in user_context — the real redaction engine catches AWS keys.
	resp := callConfigureRaw(h, `{"what":"report_issue","operation":"preview","template":"bug","user_context":"my key AKIAIOSFODNN7EXAMPLE is broken"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	// Check ALL content blocks, not just the first, to ensure no leaks.
	for i, block := range result.Content {
		if strings.Contains(block.Text, "AKIAIOSFODNN7EXAMPLE") {
			t.Fatalf("AWS key found in content block[%d]: %s", i, block.Text)
		}
	}
}

func TestReportIssue_SubmitRequiresTitle(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"report_issue","operation":"submit"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("expected error for submit without title")
	}
	text := firstText(result)
	if !strings.Contains(text, "title") {
		t.Fatalf("error should mention title, got: %s", text)
	}
}

func TestReportIssue_InvalidOperation(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"report_issue","operation":"destroy"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("expected error for invalid operation")
	}
	text := firstText(result)
	if !strings.Contains(text, "destroy") {
		t.Fatalf("error should mention the invalid operation, got: %s", text)
	}
}

func TestReportIssue_InvalidTemplate(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"report_issue","operation":"preview","template":"nonexistent"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("expected error for invalid template")
	}
}

func TestReportIssue_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := issuereport.Handle(h, req, json.RawMessage(`{invalid`))
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestReportIssue_SnakeCaseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"report_issue","operation":"list_templates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	jsonPart := extractJSONFromText(firstText(result))
	assertSnakeCaseFields(t, jsonPart)
}

func TestReportIssue_PreviewSnakeCaseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"report_issue","operation":"preview","template":"bug"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	jsonPart := extractJSONFromText(firstText(result))
	assertSnakeCaseFields(t, jsonPart)
}

func TestReportIssue_SubmitWithFakeRunner_Success(t *testing.T) {
	t.Parallel()
	h := makeToolHandlerWithIssueRunner(t, &fakeIssueRunner{
		stdout: "https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues/99\n",
	})

	resp := callConfigureRaw(h, `{"what":"report_issue","operation":"submit","title":"Test issue","template":"bug","user_context":"testing"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	if data["status"] != "submitted" {
		t.Fatalf("status = %v, want submitted", data["status"])
	}
	if data["method"] != "gh_cli" {
		t.Fatalf("method = %v, want gh_cli", data["method"])
	}
	if data["issue_url"] != "https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues/99" {
		t.Fatalf("issue_url = %v", data["issue_url"])
	}
}

func TestReportIssue_SubmitWithFakeRunner_GHNotFound(t *testing.T) {
	t.Parallel()
	h := makeToolHandlerWithIssueRunner(t, &fakeIssueRunner{
		lookPathErr: errors.New("not found"),
	})

	resp := callConfigureRaw(h, `{"what":"report_issue","operation":"submit","title":"Test issue","template":"bug"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success (manual fallback), got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	if data["status"] != "manual" {
		t.Fatalf("status = %v, want manual", data["status"])
	}
	if data["formatted_body"] == nil || data["formatted_body"] == "" {
		t.Fatal("formatted_body missing in manual fallback")
	}
}

func TestReportIssue_SubmitWithFakeRunner_GHError(t *testing.T) {
	t.Parallel()
	h := makeToolHandlerWithIssueRunner(t, &fakeIssueRunner{
		runErr: errors.New("exit status 1"),
		stderr: "auth required",
	})

	resp := callConfigureRaw(h, `{"what":"report_issue","operation":"submit","title":"Test issue","template":"bug"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success (error result), got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	if data["status"] != "error" {
		t.Fatalf("status = %v, want error", data["status"])
	}
	errMsg, _ := data["error"].(string)
	if !strings.Contains(errMsg, "auth required") {
		t.Fatalf("error = %q, want to contain stderr", errMsg)
	}
}
