// Purpose: Tests for interact feature-gate enforcement.
// Docs: docs/features/feature/interact-explore/index.md
//
// tools_interact_gate_test.go — Tests for pre-dispatch fast-fail gates on interact commands.
// Verifies that extension-disconnect and CSP-restricted states return immediate structured errors
// instead of queuing commands destined to time out.
//
// Run: go test ./cmd/browser-agent -run "TestRequireExtension|TestRequireCSP|TestGateOrder|TestDiagnosticHint|TestNavigate_Ext|TestExecuteJS_CSP|TestClick_Ext|TestSubtitle_No|TestRequirePilot" -v -count=1
package main

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Test Infrastructure
// ============================================

type gateTestEnv struct {
	handler *ToolHandler
	server  *Server
	capture *capture.Store
}

// newGateTestEnv creates a test env WITHOUT extension connection (for disconnect tests).
// Sets coldStartTimeout to 0 so fast-fail gate tests don't wait for the readiness gate.
// Cold-start-specific tests override handler.coldStartTimeout directly.
func newGateTestEnv(t *testing.T) *gateTestEnv {
	t.Helper()
	logFile := filepath.Join(t.TempDir(), "test-gate.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	t.Cleanup(func() { server.Close() })
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.toolHandler.(*ToolHandler)
	// Keep disconnect tests fast — override the 5s production readiness timeout.
	handler.extensionReadinessTimeout = 100 * time.Millisecond
	return &gateTestEnv{handler: handler, server: server, capture: cap}
}

// newGateTestEnvWithTimeout creates a gate test env with an explicit readiness timeout,
// avoiding the double-override pattern (newGateTestEnv default + manual field write).
func newGateTestEnvWithTimeout(t *testing.T, timeout time.Duration) *gateTestEnv {
	t.Helper()
	env := newGateTestEnv(t)
	env.handler.extensionReadinessTimeout = timeout
	return env
}

// simulateConnection sends a /sync POST to mark extension as connected.
func (e *gateTestEnv) simulateConnection(t *testing.T) {
	t.Helper()
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	e.capture.HandleSync(httptest.NewRecorder(), httpReq)
}

// enablePilot turns on pilot for tests that need it.
func (e *gateTestEnv) enablePilot(t *testing.T) {
	t.Helper()
	e.capture.SetPilotEnabled(true)
}

// extractErrorCode parses the structured error code from a JSONRPCResponse result.
func extractErrorCode(t *testing.T, resp JSONRPCResponse) string {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error response, got success")
	}
	if len(result.Content) == 0 {
		t.Fatal("error response has no content blocks")
	}
	text := result.Content[0].Text
	// The text may have a human-readable prefix before the JSON object.
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatalf("no JSON found in error text: %s", text)
	}
	var se mcp.StructuredError
	if err := json.Unmarshal([]byte(text[idx:]), &se); err != nil {
		t.Fatalf("unmarshal structured error: %v\nraw: %s", err, text[idx:])
	}
	return se.ErrorCode
}

// isSuccessOrQueued returns true if the response is not a structured error.
func isSuccessOrQueued(t *testing.T, resp JSONRPCResponse) bool {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	return !result.IsError
}

// ============================================
// Gate unit tests: requireExtension
// ============================================

func TestRequireExtension_Disconnected(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requireExtension(req)
	if !blocked {
		t.Fatal("expected requireExtension to block when extension is disconnected")
	}
	code := extractErrorCode(t, resp)
	if code != mcp.ErrNoData {
		t.Fatalf("expected error code %q, got %q", mcp.ErrNoData, code)
	}
}

func TestRequireExtension_Connected(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.simulateConnection(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requireExtension(req)
	if blocked {
		t.Fatalf("expected requireExtension to pass when connected, got blocked with: %v", resp)
	}
}

// ============================================
// Gate unit tests: requireCSPClear
// ============================================

func TestRequireCSPClear_MainWorldBlocked(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetCSPStatusForTest(true, "script_exec")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requireCSPClear(req, "main")
	if !blocked {
		t.Fatal("expected requireCSPClear to block world=main when CSP restricts script_exec")
	}
	code := extractErrorCode(t, resp)
	if code != mcp.ErrExtError {
		t.Fatalf("expected error code %q, got %q", mcp.ErrExtError, code)
	}
}

func TestRequireCSPClear_AutoWorldPasses(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetCSPStatusForTest(true, "script_exec")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	_, blocked := env.handler.requireCSPClear(req, "auto")
	if blocked {
		t.Fatal("expected requireCSPClear to pass for world=auto (extension handles fallback)")
	}
}

func TestRequireCSPClear_IsolatedWorldPasses(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetCSPStatusForTest(true, "script_exec")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	_, blocked := env.handler.requireCSPClear(req, "isolated")
	if blocked {
		t.Fatal("expected requireCSPClear to pass for world=isolated (bypasses page CSP)")
	}
}

func TestRequireCSPClear_PageBlocked(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetCSPStatusForTest(true, "page_blocked")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	_, blocked := env.handler.requireCSPClear(req, "main")
	if !blocked {
		t.Fatal("expected requireCSPClear to block world=main when CSP level is page_blocked")
	}
}

func TestRequireCSPClear_None(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetCSPStatusForTest(false, "none")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	_, blocked := env.handler.requireCSPClear(req, "main")
	if blocked {
		t.Fatal("expected requireCSPClear to pass when CSP is not restricted")
	}
}

func TestRequireCSPClear_NotRestricted(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	// restricted=false takes precedence even with a non-none level
	env.capture.SetCSPStatusForTest(false, "script_exec")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	_, blocked := env.handler.requireCSPClear(req, "main")
	if blocked {
		t.Fatal("expected requireCSPClear to pass when restricted flag is false (flag wins)")
	}
}

// ============================================
// Integration tests through handlers
// ============================================

func TestNavigate_ExtDisconnected_FastFail(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	// Extension NOT connected — no simulateConnection call

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"navigate","url":"https://example.com","sync":false}`)
	resp := env.handler.interactAction().HandleBrowserActionNavigateImpl(req, args)

	code := extractErrorCode(t, resp)
	if code != mcp.ErrNoData {
		t.Fatalf("expected %q error for navigate with ext disconnected, got %q", mcp.ErrNoData, code)
	}
}

func TestExecuteJS_CSP_MainWorld_FastFail(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	env.simulateConnection(t)
	env.simulateTabTracking(t)
	env.capture.SetCSPStatusForTest(true, "script_exec")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"execute_js","script":"return 1","world":"main","sync":false}`)
	resp := env.handler.interactAction().HandleExecuteJSImpl(req, args)

	code := extractErrorCode(t, resp)
	if code != mcp.ErrExtError {
		t.Fatalf("expected %q error for execute_js world=main with CSP restricted, got %q", mcp.ErrExtError, code)
	}
}

func TestExecuteJS_CSP_AutoWorld_PassesThrough(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	env.simulateConnection(t)
	env.simulateTabTracking(t)
	env.capture.SetCSPStatusForTest(true, "script_exec")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"execute_js","script":"return 1","sync":false}`)
	resp := env.handler.interactAction().HandleExecuteJSImpl(req, args)

	// world=auto (default) passes through gate — extension handles CSP fallback
	if !isSuccessOrQueued(t, resp) {
		t.Fatal("expected execute_js with world=auto to pass through CSP gate, got error")
	}
}

func TestClick_ExtDisconnected_FastFail(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	// Extension NOT connected

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"click","selector":"#btn","sync":false}`)
	resp := env.handler.interactAction().HandleDOMPrimitive(req, args, "click")

	code := extractErrorCode(t, resp)
	if code != mcp.ErrNoData {
		t.Fatalf("expected %q error for click with ext disconnected, got %q", mcp.ErrNoData, code)
	}
}

func TestSubtitle_NoExtensionGate(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	// Extension NOT connected, pilot not needed for subtitle

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"subtitle","text":"hello","sync":false}`)
	resp := env.handler.interactAction().HandleSubtitleImpl(req, args)

	// Subtitle should succeed (queued) even without extension
	if !isSuccessOrQueued(t, resp) {
		t.Fatal("expected subtitle to succeed without extension gate, got error")
	}
}

// ============================================
// Gate unit tests: requireTabTracking
// ============================================

func TestRequireTabTracking_NoTabTracked(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	// No tab tracking set — default state

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requireTabTracking(req)
	if !blocked {
		t.Fatal("expected requireTabTracking to block when no tab is tracked")
	}
	code := extractErrorCode(t, resp)
	if code != mcp.ErrNoData {
		t.Fatalf("expected error code %q, got %q", mcp.ErrNoData, code)
	}
}

func TestRequireTabTracking_TabTracked(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.simulateTabTracking(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	_, blocked := env.handler.requireTabTracking(req)
	if blocked {
		t.Fatal("expected requireTabTracking to pass when a tab is tracked")
	}
}

// simulateTabTracking sets tracking state so tab-tracking gates pass.
func (e *gateTestEnv) simulateTabTracking(t *testing.T) {
	t.Helper()
	e.capture.SetTrackingStatusForTest(42, "https://example.com")
}

func TestRequireTabTracking_NoRecoveryToolCall(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	// No tab tracking set

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requireTabTracking(req)
	if !blocked {
		t.Fatal("expected requireTabTracking to block")
	}
	se := extractStructuredError(t, resp)
	// recovery_tool_call is intentionally absent: navigate requires url which the
	// server can't know, so providing a recovery_tool_call would guarantee a second
	// failure. The recovery text in the error message guides the LLM instead.
	if se.RecoveryToolCall != nil {
		t.Fatal("requireTabTracking should NOT include recovery_tool_call (navigate requires url)")
	}
	if se.RecoveryPlaybook == "" {
		t.Fatal("recovery text should guide the LLM to call navigate with a URL")
	}
}

// ============================================
// Tab tracking integration tests through handlers
// ============================================

func TestNavigate_NoTabTracking_NotBlocked(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	env.simulateConnection(t)
	// No tab tracking — navigate should NOT be blocked (it creates tracking)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"navigate","url":"https://example.com","sync":false}`)
	resp := env.handler.interactAction().HandleBrowserActionNavigateImpl(req, args)

	// Navigate should succeed (queued) even without tab tracking
	if !isSuccessOrQueued(t, resp) {
		t.Fatal("expected navigate to succeed without tab tracking gate, got error")
	}
}

func TestClick_NoTabTracking_FastFail(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	env.simulateConnection(t)
	// No tab tracking — click should be blocked

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"click","selector":"#btn","sync":false}`)
	resp := env.handler.interactAction().HandleDOMPrimitive(req, args, "click")

	code := extractErrorCode(t, resp)
	if code != mcp.ErrNoData {
		t.Fatalf("expected %q error for click without tab tracking, got %q", mcp.ErrNoData, code)
	}
}

func TestSwitchTab_NoTabTracking_NotBlocked(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	env.simulateConnection(t)
	// No tab tracking — switch_tab should NOT be blocked because it IS how
	// you establish tracking for an existing tab (P1-2 fix).

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"switch_tab","tab_id":42,"sync":false}`)
	resp := env.handler.interactAction().HandleBrowserActionSwitchTabImpl(req, args)

	// switch_tab should succeed (queued) even without tab tracking.
	if !isSuccessOrQueued(t, resp) {
		t.Fatal("expected switch_tab to succeed without tab tracking gate, got error")
	}
}

func TestSaveState_NoTabTracking_NoGate(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	env.simulateConnection(t)
	// No tab tracking — save_state should NOT be blocked by tab tracking gate.
	// It may fail for other reasons (e.g. session store not initialized) but that is
	// unrelated to the gate — the important thing is that it is NOT a tab tracking error.

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"save_state","snapshot_name":"test-state","sync":false}`)
	resp := env.handler.stateInteract().HandleStateSave(req, args)

	// If it is an error, it must NOT be the tab tracking error (mcp.ErrNoData with "tab" message).
	if !isSuccessOrQueued(t, resp) {
		se := extractStructuredError(t, resp)
		if se.ErrorCode == mcp.ErrNoData && strings.Contains(se.Message, "tab") {
			t.Fatalf("save_state was blocked by tab tracking gate — it should bypass this gate. Error: %s", se.Message)
		}
		// Other errors (e.g. session store not initialized) are fine — they are not gate errors.
	}
}

// ============================================
// Gate ordering tests
// ============================================

func TestGateOrder_ParamValidation_BeforeExtension(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	// Extension NOT connected + missing required 'script' param

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"execute_js","sync":false}`)
	resp := env.handler.interactAction().HandleExecuteJSImpl(req, args)

	code := extractErrorCode(t, resp)
	if code != mcp.ErrMissingParam {
		t.Fatalf("expected %q (param validation first), got %q", mcp.ErrMissingParam, code)
	}
}

func TestGateOrder_Pilot_BeforeExtension(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetPilotEnabled(false)
	// Extension NOT connected + pilot disabled

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"navigate","url":"https://example.com","sync":false}`)
	resp := env.handler.interactAction().HandleBrowserActionNavigateImpl(req, args)

	code := extractErrorCode(t, resp)
	if code != mcp.ErrCodePilotDisabled {
		t.Fatalf("expected %q (pilot before extension), got %q", mcp.ErrCodePilotDisabled, code)
	}
}

func TestGateOrder_Extension_BeforeTabTracking(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	// Extension NOT connected + no tab tracking — extension gate should fire first

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"click","selector":"#btn","sync":false}`)
	resp := env.handler.interactAction().HandleDOMPrimitive(req, args, "click")

	code := extractErrorCode(t, resp)
	if code != mcp.ErrNoData {
		t.Fatalf("expected error code %q, got %q", mcp.ErrNoData, code)
	}
	// Both extension disconnect and tab tracking return mcp.ErrNoData,
	// but extension gate should fire first. Verify via message.
	se := extractStructuredError(t, resp)
	if !strings.Contains(se.Message, "Extension") {
		t.Fatalf("expected extension gate to fire before tab tracking, got: %s", se.Message)
	}
}

func TestGateOrder_TabTracking_BeforeCSP(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	env.simulateConnection(t)
	// No tab tracking + CSP restricted — tab tracking gate should fire before CSP
	env.capture.SetCSPStatusForTest(true, "script_exec")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"execute_js","script":"return 1","world":"main","sync":false}`)
	resp := env.handler.interactAction().HandleExecuteJSImpl(req, args)

	code := extractErrorCode(t, resp)
	if code != mcp.ErrNoData {
		t.Fatalf("expected %q (tab tracking before CSP), got %q", mcp.ErrNoData, code)
	}
	se := extractStructuredError(t, resp)
	if !strings.Contains(se.Message, "tab") {
		t.Fatalf("expected tab tracking error message, got: %s", se.Message)
	}
}

func TestGateOrder_Extension_BeforeCSP(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	// Extension NOT connected + CSP restricted
	env.capture.SetCSPStatusForTest(true, "script_exec")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"execute_js","script":"return 1","world":"main","sync":false}`)
	resp := env.handler.interactAction().HandleExecuteJSImpl(req, args)

	code := extractErrorCode(t, resp)
	if code != mcp.ErrNoData {
		t.Fatalf("expected %q (extension before CSP), got %q", mcp.ErrNoData, code)
	}
}

// ============================================
// Recovery tool call tests
// ============================================

// extractStructuredError parses the full mcp.StructuredError from a JSONRPCResponse result.
func extractStructuredError(t *testing.T, resp JSONRPCResponse) mcp.StructuredError {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error response, got success")
	}
	if len(result.Content) == 0 {
		t.Fatal("error response has no content blocks")
	}
	text := result.Content[0].Text
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatalf("no JSON found in error text: %s", text)
	}
	var se mcp.StructuredError
	if err := json.Unmarshal([]byte(text[idx:]), &se); err != nil {
		t.Fatalf("unmarshal structured error: %v\nraw: %s", err, text[idx:])
	}
	return se
}

func TestRequirePilot_RecoveryToolCall(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetPilotEnabled(false)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requirePilot(req)
	if !blocked {
		t.Fatal("expected requirePilot to block when pilot is disabled")
	}
	se := extractStructuredError(t, resp)
	if se.RecoveryToolCall == nil {
		t.Fatal("expected recovery_tool_call in pilot_disabled error")
	}
	toolName, _ := se.RecoveryToolCall["tool"].(string)
	if toolName != "observe" {
		t.Fatalf("expected recovery_tool_call tool='observe', got %q", toolName)
	}
	args, _ := se.RecoveryToolCall["arguments"].(map[string]any)
	if args == nil {
		t.Fatal("expected recovery_tool_call to have 'arguments'")
	}
	if what, _ := args["what"].(string); what != "pilot" {
		t.Fatalf("expected recovery_tool_call arguments.what='pilot', got %q", what)
	}
}

func TestRequireExtension_RecoveryToolCall(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requireExtension(req)
	if !blocked {
		t.Fatal("expected requireExtension to block when extension is disconnected")
	}
	se := extractStructuredError(t, resp)
	if se.RecoveryToolCall == nil {
		t.Fatal("expected recovery_tool_call in extension disconnected error")
	}
	toolName, _ := se.RecoveryToolCall["tool"].(string)
	if toolName == "" {
		t.Fatal("expected recovery_tool_call to have a 'tool' field")
	}
}

func TestRequireCSPClear_RecoveryToolCall(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetCSPStatusForTest(true, "script_exec")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requireCSPClear(req, "main")
	if !blocked {
		t.Fatal("expected requireCSPClear to block when CSP restricts main world")
	}
	se := extractStructuredError(t, resp)
	if se.RecoveryToolCall == nil {
		t.Fatal("expected recovery_tool_call in CSP blocked error")
	}
	toolName, _ := se.RecoveryToolCall["tool"].(string)
	if toolName != "interact" {
		t.Fatalf("expected recovery_tool_call tool='interact', got %q", toolName)
	}
	args, _ := se.RecoveryToolCall["arguments"].(map[string]any)
	if args == nil {
		t.Fatal("expected recovery_tool_call to have 'arguments'")
	}
	// The recovery for CSP should suggest world=auto or world=isolated
	if world, ok := args["world"]; !ok || world == "main" {
		t.Fatalf("expected recovery_tool_call to suggest world != 'main', got %v", world)
	}
}

// ============================================
// Diagnostic hint test
// ============================================

// TestRequireExtension_ConnectsDuringWait verifies the cold-start readiness gate:
// if the extension connects within the wait window, the gate passes instead of failing.
func TestRequireExtension_ConnectsDuringWait(t *testing.T) {
	t.Parallel()
	// 500ms window: goroutine connects at 150ms, caught on the 200ms poll tick.
	env := newGateTestEnvWithTimeout(t, 500*time.Millisecond)

	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(150 * time.Millisecond)
		env.simulateConnection(t)
	}()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requireExtension(req)
	<-done // ensure goroutine finished before test returns
	if blocked {
		t.Fatalf("expected requireExtension to pass after late connection, got blocked: %v", resp)
	}
}

func TestDiagnosticHint_IncludesCSP(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetCSPStatusForTest(true, "script_exec")

	hint := env.handler.DiagnosticHintString()
	if !strings.Contains(hint, "csp=") {
		t.Fatalf("expected diagnostic hint to include CSP status, got: %s", hint)
	}
	if !strings.Contains(hint, "script_exec") {
		t.Fatalf("expected diagnostic hint to include CSP level 'script_exec', got: %s", hint)
	}
}

// ============================================
// Smoke Tests: Stream 2 — Sequential gate firing for execute_js
// ============================================

func TestSmoke_AllGates_SequentialFiring_ExecuteJS(t *testing.T) {
	t.Parallel()

	// This test verifies that for execute_js(world="main"), gates fire in the
	// correct priority order as conditions are progressively fixed.
	// Gate order: param validation → pilot → extension → tab tracking → CSP

	t.Run("1_no_script_missing_param", func(t *testing.T) {
		env := newGateTestEnv(t)
		env.capture.SetPilotEnabled(false)
		// No script param, pilot off, ext off, no tab, CSP on
		env.capture.SetCSPStatusForTest(true, "script_exec")

		req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
		args := json.RawMessage(`{"what":"execute_js","world":"main","sync":false}`)
		resp := env.handler.interactAction().HandleExecuteJSImpl(req, args)

		code := extractErrorCode(t, resp)
		if code != mcp.ErrMissingParam {
			t.Fatalf("step 1: expected %q (param validation first), got %q", mcp.ErrMissingParam, code)
		}
	})

	t.Run("2_script_present_pilot_disabled", func(t *testing.T) {
		env := newGateTestEnv(t)
		env.capture.SetPilotEnabled(false)
		// Script present, pilot off, ext off, no tab, CSP on
		env.capture.SetCSPStatusForTest(true, "script_exec")

		req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
		args := json.RawMessage(`{"what":"execute_js","script":"return 1","world":"main","sync":false}`)
		resp := env.handler.interactAction().HandleExecuteJSImpl(req, args)

		code := extractErrorCode(t, resp)
		if code != mcp.ErrCodePilotDisabled {
			t.Fatalf("step 2: expected %q (pilot before extension), got %q", mcp.ErrCodePilotDisabled, code)
		}
	})

	t.Run("3_pilot_on_ext_disconnected", func(t *testing.T) {
		env := newGateTestEnv(t)
		env.enablePilot(t)
		// Pilot on, ext off, no tab, CSP on
		env.capture.SetCSPStatusForTest(true, "script_exec")

		req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
		args := json.RawMessage(`{"what":"execute_js","script":"return 1","world":"main","sync":false}`)
		resp := env.handler.interactAction().HandleExecuteJSImpl(req, args)

		code := extractErrorCode(t, resp)
		if code != mcp.ErrNoData {
			t.Fatalf("step 3: expected %q (extension gate), got %q", mcp.ErrNoData, code)
		}
		se := extractStructuredError(t, resp)
		if !strings.Contains(se.Message, "Extension") {
			t.Fatalf("step 3: expected extension-related message, got: %s", se.Message)
		}
	})

	t.Run("4_ext_connected_no_tab", func(t *testing.T) {
		env := newGateTestEnv(t)
		env.enablePilot(t)
		env.simulateConnection(t)
		// Ext on, no tab, CSP on
		env.capture.SetCSPStatusForTest(true, "script_exec")

		req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
		args := json.RawMessage(`{"what":"execute_js","script":"return 1","world":"main","sync":false}`)
		resp := env.handler.interactAction().HandleExecuteJSImpl(req, args)

		code := extractErrorCode(t, resp)
		if code != mcp.ErrNoData {
			t.Fatalf("step 4: expected %q (tab tracking gate), got %q", mcp.ErrNoData, code)
		}
		se := extractStructuredError(t, resp)
		if !strings.Contains(se.Message, "tab") {
			t.Fatalf("step 4: expected tab-related message, got: %s", se.Message)
		}
	})

	t.Run("5_tab_tracked_csp_blocked", func(t *testing.T) {
		env := newGateTestEnv(t)
		env.enablePilot(t)
		env.simulateConnection(t)
		env.simulateTabTracking(t)
		env.capture.SetCSPStatusForTest(true, "script_exec")

		req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
		args := json.RawMessage(`{"what":"execute_js","script":"return 1","world":"main","sync":false}`)
		resp := env.handler.interactAction().HandleExecuteJSImpl(req, args)

		code := extractErrorCode(t, resp)
		if code != mcp.ErrExtError {
			t.Fatalf("step 5: expected %q (CSP gate), got %q", mcp.ErrExtError, code)
		}
	})
}
