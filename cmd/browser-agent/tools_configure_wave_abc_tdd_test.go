// Purpose: TDD wave tests for configure tool features.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func configureSchemaPropertiesForTest(t *testing.T) map[string]any {
	t.Helper()
	server, err := NewServer(t.TempDir()+"/schema-wave-abc.jsonl", 10)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { server.Close() })
	tools := toolSchemasForTest()
	for _, tool := range tools {
		if tool.Name != "configure" {
			continue
		}
		props, ok := tool.InputSchema["properties"].(map[string]any)
		if !ok {
			t.Fatal("configure tool schema missing properties")
		}
		return props
	}
	t.Fatal("configure tool not found in schema")
	return nil
}

func callHandledTool(t *testing.T, h *ToolHandler, req mcp.JSONRPCRequest, name, argsJSON string) mcp.JSONRPCResponse {
	t.Helper()
	resp, handled := h.HandleToolCall(req, name, json.RawMessage(argsJSON))
	if !handled {
		t.Fatalf("tool %q was not handled", name)
	}
	return resp
}

func TestWaveA_ConfigureSchema_DiffSessionsURLPropertyPresent(t *testing.T) {
	t.Parallel()

	props := configureSchemaPropertiesForTest(t)
	if _, ok := props["url"]; !ok {
		t.Fatal("configure schema should include 'url' for diff_sessions capture filtering")
	}
}

func TestWaveB_ConfigureSchema_AuditLogOperationPropertyPresent(t *testing.T) {
	t.Parallel()

	props := configureSchemaPropertiesForTest(t)
	if _, ok := props["operation"]; !ok {
		t.Fatal("configure schema should expose 'operation' key")
	}

	opRaw, ok := props["operation"].(map[string]any)
	if !ok {
		t.Fatal("configure schema should include operation property")
	}
	enumVals, ok := opRaw["enum"].([]string)
	if !ok {
		t.Fatal("operation enum should be []string")
	}
	got := strings.Join(enumVals, ",")
	for _, want := range []string{"analyze", "report", "clear"} {
		if !strings.Contains(got, want) {
			t.Fatalf("operation enum missing %q: %v", want, enumVals)
		}
	}
}

func TestWaveB_ConfigureSchema_AuditLogAllowsTelemetryMode(t *testing.T) {
	t.Parallel()

	props := configureSchemaPropertiesForTest(t)
	if _, ok := props["telemetry_mode"]; !ok {
		t.Fatal("configure schema should include telemetry_mode property")
	}
}

func TestWaveB_AuditLogOperationAnalyzeAndClear(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/audit-waveb.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { server.Close() })

	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	h := mcpHandler.tools.Executor.(*ToolHandler)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, ClientID: "wave-b-test"}

	callHandledTool(t, h, req, "configure", `{"what":"health"}`)
	callHandledTool(t, h, req, "observe", `{"what":"logs"}`)

	analyzeResp := callHandledTool(t, h, req, "configure", `{"what":"audit_log","operation":"analyze"}`)
	analyzeResult := parseToolResult(t, analyzeResp)
	if analyzeResult.IsError {
		t.Fatalf("audit_log analyze should succeed, got: %s", analyzeResult.Content[0].Text)
	}
	analyzeData := extractResultJSON(t, analyzeResult)
	if analyzeData["operation"] != "analyze" {
		t.Fatalf("operation = %v, want analyze", analyzeData["operation"])
	}
	if _, ok := analyzeData["summary"]; !ok {
		t.Fatal("audit_log analyze should include summary")
	}

	clearResp := callHandledTool(t, h, req, "configure", `{"what":"audit_log","operation":"clear"}`)
	clearResult := parseToolResult(t, clearResp)
	if clearResult.IsError {
		t.Fatalf("audit_log clear should succeed, got: %s", clearResult.Content[0].Text)
	}
	clearData := extractResultJSON(t, clearResult)
	if clearData["operation"] != "clear" {
		t.Fatalf("operation = %v, want clear", clearData["operation"])
	}
	if _, ok := clearData["cleared"]; !ok {
		t.Fatal("audit_log clear should report cleared count")
	}
}

func TestWaveB_AuditLogClear_DoesNotReinsertClearCall(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/audit-clear-empty.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { server.Close() })

	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	h := mcpHandler.tools.Executor.(*ToolHandler)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, ClientID: "audit-clear-test"}

	// Seed at least one entry.
	callHandledTool(t, h, req, "configure", `{"what":"health"}`)

	clearResp := callHandledTool(t, h, req, "configure", `{"what":"audit_log","operation":"clear"}`)
	clearResult := parseToolResult(t, clearResp)
	if clearResult.IsError {
		t.Fatalf("audit_log clear should succeed, got: %s", clearResult.Content[0].Text)
	}

	// Report should be empty. If clear got re-recorded after execution, count would be 1.
	reportResp := callHandledTool(t, h, req, "configure", `{"what":"audit_log","operation":"report"}`)
	reportResult := parseToolResult(t, reportResp)
	if reportResult.IsError {
		t.Fatalf("audit_log report should succeed, got: %s", reportResult.Content[0].Text)
	}
	reportData := extractResultJSON(t, reportResult)
	if got := int(reportData["count"].(float64)); got != 0 {
		t.Fatalf("audit_log should remain empty after clear, got count=%d", got)
	}
}

func TestWaveB_AuditLogClear_ResetsToolHandlerSessionMap(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/audit-clear-session-reset.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { server.Close() })

	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	h := mcpHandler.tools.Executor.(*ToolHandler)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, ClientID: "session-reset-client"}

	callHandledTool(t, h, req, "configure", `{"what":"health"}`)
	oldSessionID := h.auditRecorder.SessionID("session-reset-client")
	if oldSessionID == "" {
		t.Fatal("expected initial audit session id")
	}

	callHandledTool(t, h, req, "configure", `{"what":"audit_log","operation":"clear"}`)
	if stale := h.auditRecorder.SessionID("session-reset-client"); stale != "" {
		t.Fatalf("audit session map should be reset on clear, found stale id: %s", stale)
	}

	callHandledTool(t, h, req, "configure", `{"what":"health"}`)
	newSessionID := h.auditRecorder.SessionID("session-reset-client")
	if newSessionID == "" {
		t.Fatal("expected new audit session id after clear")
	}
	if newSessionID == oldSessionID {
		t.Fatalf("session id should be recreated after clear; old=%s new=%s", oldSessionID, newSessionID)
	}
}

func TestWaveC_RedactionEngineIsWiredAndApplied(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/redaction-wavec.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { server.Close() })

	cap := capture.NewCapture()
	h := NewToolHandler(server, cap)
	if h.tools.Redactor == nil {
		t.Fatal("MCP handler should provide a redaction engine")
	}

	input := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  json.RawMessage(`{"content":[{"type":"text","text":"Authorization: Bearer ghp_1234567890abcdef"}],"isError":false}`),
	}
	output := h.applyToolResponsePostProcessing(input, "wave-c-test", "configure", "")
	result := parseToolResult(t, output)
	if len(result.Content) == 0 {
		t.Fatal("expected content in redacted response")
	}
	if !strings.Contains(result.Content[0].Text, "[REDACTED") {
		t.Fatalf("expected redacted output, got: %q", result.Content[0].Text)
	}
}
