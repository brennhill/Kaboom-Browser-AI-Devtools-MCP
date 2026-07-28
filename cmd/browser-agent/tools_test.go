// Purpose: Tests for browser-agent tools.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_test.go — Tests for MCP tool handlers.
// Covers core functionality: tool dispatch, error handling, and response formatting.
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// NewToolHandler Tests
// ============================================

func TestNewToolHandler(t *testing.T) {
	// Create minimal dependencies
	server, err := NewServer(t.TempDir()+"/test-kaboom.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	cap := capture.NewCapture()
	// Create handler
	handler := NewToolHandler(server, cap)

	if handler == nil {
		t.Fatal("NewToolHandler returned nil")
	}
	if handler.server != server {
		t.Error("MCPHandler.server not set correctly")
	}
	if handler.tools.Executor == nil {
		t.Fatal("MCPHandler.tools.Executor should not be nil")
	}
	th, ok := handler.tools.Executor.(*ToolHandler)
	if !ok {
		t.Fatalf("toolHandler type = %T, want *ToolHandler", handler.tools.Executor)
	}
	if th.server != server {
		t.Error("ToolHandler.server not set correctly")
	}
	if th.capture != cap {
		t.Error("ToolHandler.capture not set correctly")
	}
}

// ============================================
// HandleToolCall Tests
// ============================================

func TestHandleToolCall_UnknownTool(t *testing.T) {
	server, _ := NewServer(t.TempDir()+"/test-kaboom.jsonl", 100)
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)

	// Get the tool handler
	toolHandler := mcpHandler.tools.Executor.(*ToolHandler)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
		Method:  "tools/call",
	}

	// Try to handle an unknown tool
	resp, handled := toolHandler.HandleToolCall(req, "unknown_tool", json.RawMessage(`{}`))

	if handled {
		t.Error("Expected handler to NOT handle unknown tool")
	}
	if resp.JSONRPC != "" {
		t.Errorf("Expected empty JSONRPC version for unhandled tool, got %q", resp.JSONRPC)
	}
	if resp.Result != nil {
		t.Errorf("Expected nil result for unhandled tool, got %s", string(resp.Result))
	}
}

func TestHandleToolCall_ObserveTool(t *testing.T) {
	server, _ := NewServer(t.TempDir()+"/test-kaboom.jsonl", 100)
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	toolHandler := mcpHandler.tools.Executor.(*ToolHandler)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
		Method:  "tools/call",
	}

	args := json.RawMessage(`{"what": "logs"}`)
	resp, handled := toolHandler.HandleToolCall(req, "observe", args)

	if !handled {
		t.Error("Expected handler to handle observe tool")
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSON-RPC version 2.0, got %s", resp.JSONRPC)
	}

	// Result should be valid JSON
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Invalid result JSON: %v", err)
	}
	if result.IsError {
		t.Errorf("Expected observe(logs) to succeed, got error: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("Expected at least one content block in observe response")
	}
	if result.Content[0].Type != "text" {
		t.Errorf("Expected content type 'text', got %q", result.Content[0].Type)
	}
}

func TestHandleToolCall_GenerateTool(t *testing.T) {
	server, _ := NewServer(t.TempDir()+"/test-kaboom.jsonl", 100)
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	toolHandler := mcpHandler.tools.Executor.(*ToolHandler)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
		Method:  "tools/call",
	}

	args := json.RawMessage(`{"what": "reproduction"}`)
	resp, handled := toolHandler.HandleToolCall(req, "generate", args)

	if !handled {
		t.Error("Expected handler to handle generate tool")
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSON-RPC version 2.0, got %s", resp.JSONRPC)
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Invalid result JSON: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("Expected at least one content block in generate response")
	}
	if result.Content[0].Type != "text" {
		t.Errorf("Expected content type 'text', got %q", result.Content[0].Type)
	}
}

func TestHandleToolCall_ConfigureTool(t *testing.T) {
	server, _ := NewServer(t.TempDir()+"/test-kaboom.jsonl", 100)
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	toolHandler := mcpHandler.tools.Executor.(*ToolHandler)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
		Method:  "tools/call",
	}

	args := json.RawMessage(`{"what": "health"}`)
	resp, handled := toolHandler.HandleToolCall(req, "configure", args)

	if !handled {
		t.Error("Expected handler to handle configure tool")
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSON-RPC version 2.0, got %s", resp.JSONRPC)
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Invalid result JSON: %v", err)
	}
	if result.IsError {
		t.Errorf("Expected configure(health) to succeed, got error: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("Expected at least one content block in configure response")
	}
}

func TestHandleToolCall_InteractTool(t *testing.T) {
	server, _ := NewServer(t.TempDir()+"/test-kaboom.jsonl", 100)
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	toolHandler := mcpHandler.tools.Executor.(*ToolHandler)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
		Method:  "tools/call",
	}

	// list_states doesn't require pilot to be enabled
	args := json.RawMessage(`{"what": "list_states"}`)
	resp, handled := toolHandler.HandleToolCall(req, "interact", args)

	if !handled {
		t.Error("Expected handler to handle interact tool")
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSON-RPC version 2.0, got %s", resp.JSONRPC)
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Invalid result JSON: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("Expected at least one content block in interact response")
	}
	if result.Content[0].Type != "text" {
		t.Errorf("Expected content type 'text', got %q", result.Content[0].Type)
	}
}

// ============================================
// Observe Mode Tests
// ============================================

func TestToolObserve_MissingWhat(t *testing.T) {
	server, _ := NewServer(t.TempDir()+"/test-kaboom.jsonl", 100)
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	toolHandler := mcpHandler.tools.Executor.(*ToolHandler)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.observeDispatcher.Handle(req, json.RawMessage(`{}`))

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Fatal("Expected error response for missing 'what' parameter")
	}
	if len(result.Content) == 0 {
		t.Fatal("Expected content in error response")
	}
	if !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Errorf("Expected error code 'missing_param' in text, got: %s", result.Content[0].Text)
	}
}

func TestToolObserve_UnknownMode(t *testing.T) {
	server, _ := NewServer(t.TempDir()+"/test-kaboom.jsonl", 100)
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	toolHandler := mcpHandler.tools.Executor.(*ToolHandler)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.observeDispatcher.Handle(req, json.RawMessage(`{"what": "invalid_mode"}`))

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Fatal("Expected error response for unknown mode")
	}
	if len(result.Content) == 0 {
		t.Fatal("Expected content in error response")
	}
	if !strings.Contains(result.Content[0].Text, "unknown_mode") {
		t.Errorf("Expected error code 'unknown_mode' in text, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "invalid_mode") {
		t.Errorf("Expected invalid mode name in error text, got: %s", result.Content[0].Text)
	}
}

func TestToolObserve_NetworkBodies(t *testing.T) {
	server, _ := NewServer(t.TempDir()+"/test-kaboom.jsonl", 100)
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	toolHandler := mcpHandler.tools.Executor.(*ToolHandler)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.observeDispatcher.Handle(req, json.RawMessage(`{"what": "network_bodies"}`))

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if result.IsError {
		t.Errorf("Did not expect error for network_bodies mode, got: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("Expected at least one content block")
	}
	if result.Content[0].Type != "text" {
		t.Errorf("Expected content type 'text', got %q", result.Content[0].Type)
	}
}

// ============================================
// Generate Mode Tests
// ============================================

func TestToolGenerate_MissingFormat(t *testing.T) {
	server, _ := NewServer(t.TempDir()+"/test-kaboom.jsonl", 100)
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	toolHandler := mcpHandler.tools.Executor.(*ToolHandler)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.generateDispatcher.Handle(req, json.RawMessage(`{}`))

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Fatal("Expected error response for missing 'format' parameter")
	}
	if !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Errorf("Expected error code 'missing_param', got: %s", result.Content[0].Text)
	}
}

func TestToolGenerate_UnknownFormat(t *testing.T) {
	server, _ := NewServer(t.TempDir()+"/test-kaboom.jsonl", 100)
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	toolHandler := mcpHandler.tools.Executor.(*ToolHandler)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.generateDispatcher.Handle(req, json.RawMessage(`{"what": "invalid_format"}`))

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Fatal("Expected error response for unknown format")
	}
	if !strings.Contains(result.Content[0].Text, "unknown_mode") {
		t.Errorf("Expected error code 'unknown_mode', got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "invalid_format") {
		t.Errorf("Expected format name in error text, got: %s", result.Content[0].Text)
	}
}

// ============================================
// Configure Mode Tests
// ============================================

func TestToolConfigure_MissingAction(t *testing.T) {
	server, _ := NewServer(t.TempDir()+"/test-kaboom.jsonl", 100)
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	toolHandler := mcpHandler.tools.Executor.(*ToolHandler)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.configureDispatcher.Handle(req, json.RawMessage(`{}`))

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Fatal("Expected error response for missing 'action' parameter")
	}
	if !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Errorf("Expected error code 'missing_param', got: %s", result.Content[0].Text)
	}
}

func TestToolConfigure_UnknownAction(t *testing.T) {
	server, _ := NewServer(t.TempDir()+"/test-kaboom.jsonl", 100)
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	toolHandler := mcpHandler.tools.Executor.(*ToolHandler)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.configureDispatcher.Handle(req, json.RawMessage(`{"what": "invalid_action"}`))

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Fatal("Expected error response for unknown action")
	}
	if !strings.Contains(result.Content[0].Text, "unknown_mode") {
		t.Errorf("Expected error code 'unknown_mode', got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "invalid_action") {
		t.Errorf("Expected action name in error text, got: %s", result.Content[0].Text)
	}
}

func TestToolConfigure_Health(t *testing.T) {
	server, _ := NewServer(t.TempDir()+"/test-kaboom.jsonl", 100)
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	toolHandler := mcpHandler.tools.Executor.(*ToolHandler)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.configureDispatcher.Handle(req, json.RawMessage(`{"what": "health"}`))

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if result.IsError {
		t.Fatalf("Did not expect error for health action, got: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("Expected at least one content block")
	}
	if result.Content[0].Type != "text" {
		t.Errorf("Expected content type 'text', got %q", result.Content[0].Type)
	}
	// Health response should contain non-empty text
	if result.Content[0].Text == "" {
		t.Error("Expected non-empty text in health response")
	}
}

// ============================================
// Interact Mode Tests
// ============================================

func TestToolInteract_MissingAction(t *testing.T) {
	server, _ := NewServer(t.TempDir()+"/test-kaboom.jsonl", 100)
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	toolHandler := mcpHandler.tools.Executor.(*ToolHandler)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.toolInteract(req, json.RawMessage(`{}`))

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Fatal("Expected error response for missing 'action' parameter")
	}
	if !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Errorf("Expected error code 'missing_param', got: %s", result.Content[0].Text)
	}
}

func TestToolInteract_UnknownAction(t *testing.T) {
	server, _ := NewServer(t.TempDir()+"/test-kaboom.jsonl", 100)
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	toolHandler := mcpHandler.tools.Executor.(*ToolHandler)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"test-id"`),
	}

	resp := toolHandler.toolInteract(req, json.RawMessage(`{"what": "invalid_action"}`))

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Fatal("Expected error response for unknown action")
	}
	if !strings.Contains(result.Content[0].Text, "unknown_mode") {
		t.Errorf("Expected error code 'unknown_mode', got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "invalid_action") {
		t.Errorf("Expected action name in error text, got: %s", result.Content[0].Text)
	}
}

// ============================================
// ToolsList Tests
// ============================================

func TestToolsList(t *testing.T) {
	server, _ := NewServer(t.TempDir()+"/test-kaboom.jsonl", 100)
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)

	tools := mcpHandler.tools.Schemas

	if len(tools) != 5 {
		t.Errorf("Expected 5 tools, got %d", len(tools))
	}

	// Check tool names
	// Updated in Phase 0 to include new "analyze" tool for active analysis operations
	expectedTools := map[string]bool{
		"observe":   false,
		"analyze":   false,
		"generate":  false,
		"configure": false,
		"interact":  false,
	}

	for _, tool := range tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("Expected tool '%s' not found in ToolsList", name)
		}
	}
}

// ============================================
// Response Helper Tests
// ============================================

func TestMcpTextResponse(t *testing.T) {
	resp := mcp.TextResponse("Hello, World!")

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if len(result.Content) != 1 {
		t.Errorf("Expected 1 content block, got %d", len(result.Content))
	}

	if result.Content[0].Type != "text" {
		t.Errorf("Expected type 'text', got '%s'", result.Content[0].Type)
	}

	if result.Content[0].Text != "Hello, World!" {
		t.Errorf("Expected text 'Hello, World!', got '%s'", result.Content[0].Text)
	}

	if result.IsError {
		t.Error("Expected IsError to be false")
	}
}

func TestMcpJSONResponse(t *testing.T) {
	data := map[string]any{
		"status": "ok",
		"count":  42,
	}
	resp := mcp.JSONResponse("Test summary", data)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if len(result.Content) != 1 {
		t.Errorf("Expected 1 content block, got %d", len(result.Content))
	}

	// Text should contain the summary and JSON
	text := result.Content[0].Text
	if text == "" {
		t.Fatal("Expected non-empty text")
	}

	// Should start with summary
	if len(text) < 12 || text[:12] != "Test summary" {
		t.Error("Expected text to start with summary")
	}

	// Extract and validate JSON portion
	jsonStart := -1
	for i, c := range text {
		if c == '{' {
			jsonStart = i
			break
		}
	}
	if jsonStart < 0 {
		t.Fatal("Expected JSON object in response text")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(text[jsonStart:]), &parsed); err != nil {
		t.Fatalf("Expected valid JSON in response, got error: %v", err)
	}
	if parsed["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", parsed["status"])
	}
	if parsed["count"] != float64(42) {
		t.Errorf("Expected count 42, got %v", parsed["count"])
	}

	// Verify snake_case JSON field names
	assertSnakeCaseFields(t, string(resp))
}

func TestMcpStructuredError(t *testing.T) {
	resp := mcp.StructuredErrorResponse(mcp.ErrMissingParam, "Missing parameter 'what'", "Add the 'what' parameter", mcp.WithParam("what"), mcp.WithHint("Valid values: logs, errors"))

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if !result.IsError {
		t.Error("Expected IsError to be true")
	}

	// Check that the text contains the error code
	text := result.Content[0].Text
	if text == "" {
		t.Fatal("Expected non-empty text")
	}

	// Should contain the error code and retry instruction
	if !strings.Contains(text, "missing_param") {
		t.Error("Expected text to contain error code 'missing_param'")
	}
	if !strings.Contains(text, "Missing parameter 'what'") {
		t.Error("Expected text to contain message")
	}
	if !strings.Contains(text, "Add the 'what' parameter") {
		t.Error("Expected text to contain retry instruction")
	}
	if !strings.Contains(text, "Valid values: logs, errors") {
		t.Error("Expected text to contain hint")
	}

	// Verify the JSON portion is valid
	jsonStart := -1
	for i, c := range text {
		if c == '{' {
			jsonStart = i
			break
		}
	}
	if jsonStart >= 0 {
		var se map[string]any
		if err := json.Unmarshal([]byte(text[jsonStart:]), &se); err != nil {
			t.Fatalf("Expected valid JSON in structured error, got error: %v", err)
		}
		if se["error_code"] != "missing_param" {
			t.Errorf("Expected canonical error_code 'missing_param', got %v", se["error_code"])
		}
		if se["param"] != "what" {
			t.Errorf("Expected param 'what', got %v", se["param"])
		}
		if se["recovery_playbook"] != "Add the 'what' parameter" {
			t.Errorf("Expected canonical recovery_playbook, got %v", se["recovery_playbook"])
		}
		if _, exists := se["error"]; exists {
			t.Errorf("Legacy field 'error' should not be present: %v", se["error"])
		}
		if _, exists := se["retry"]; exists {
			t.Errorf("Legacy field 'retry' should not be present: %v", se["retry"])
		}

		// Verify snake_case in the structured error JSON
		assertSnakeCaseFields(t, text[jsonStart:])
	}
}

// ============================================
// Rate Limiter Tests
// ============================================

func TestToolCallLimiter_Allow(t *testing.T) {
	limiter := toolresp.NewToolCallLimiter(3, time.Second) // 3 calls per second

	// First 3 calls should be allowed
	for i := 0; i < 3; i++ {
		if !limiter.Allow() {
			t.Errorf("Call %d should be allowed", i+1)
		}
	}

	// 4th call should be rate limited
	if limiter.Allow() {
		t.Error("4th call should be rate limited")
	}
}

// containsSubstring removed — replaced by strings.Contains at all call sites.
