// tools_configure_handler_test.go — Tests top-level configure dispatch and response shape.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/launchmode"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/logstore"
	terminalstatus "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/status"
	qafixturetransport "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/qafixture/transport"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestConfigureHealthComposesServerRecoveryAndLaunchState(t *testing.T) {
	previousLaunchMode := launchmode.Current()
	launchmode.SetCurrent(launchmode.Info{
		Mode: launchmode.LikelyTransient, Reason: "interactive_shell_parent", ParentProcess: "zsh",
	})
	t.Cleanup(func() { launchmode.SetCurrent(previousLaunchMode) })

	server := &Server{
		terminalStatus: terminalstatus.New(),
		logs: logstore.New(logstore.Config{
			MaxEntries: 100, ChanSize: 1, AddWarning: func(string) {},
		}),
	}
	t.Cleanup(func() { server.logs.Shutdown(10 * time.Millisecond) })
	_ = server.logs.AppendToFile([]types.LogEntry{{"level": "info", "message": "fill"}})
	_ = server.logs.AppendToFile([]types.LogEntry{{"level": "info", "message": "drop-1"}})
	_ = server.logs.AppendToFile([]types.LogEntry{{"level": "info", "message": "drop-2"}})

	recovery := statediag.NewCollector()
	recovery.Report(statediag.Diagnostic{Name: "restart", CorrelationID: "corr-1", Detail: "recovering", Fix: "wait"})
	recovery.Resolve("restart")
	response := getHealthResponse(health.NewMetrics(), nil, server, nil, recovery, "test-version")

	if response.Buffers.Console.DroppedCount != 2 {
		t.Fatalf("console dropped count = %d, want 2", response.Buffers.Console.DroppedCount)
	}
	pressure := response.ResourcePressure["doctor_timeline"]
	if pressure.Entries != 1 || pressure.ActiveEntries != 0 || pressure.Capacity == 0 {
		t.Fatalf("doctor timeline pressure = %#v", pressure)
	}
	if response.Server.LaunchMode != launchmode.LikelyTransient || response.Server.ParentProcess != "zsh" {
		t.Fatalf("launch metadata = %#v", response.Server)
	}
}

func TestExecuteQAFixtureCommandHonorsConnectionCancellationAndQueuePressure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := qafixturetransport.Execute(ctx, nil, "qa", nil, time.Second); err != context.Canceled {
		t.Fatalf("cancelled context error = %v", err)
	}

	disconnected := capture.NewCapture()
	defer disconnected.Close()
	if _, err := qafixturetransport.Execute(context.Background(), disconnected, "qa", nil, time.Second); err != context.Canceled {
		t.Fatalf("disconnected extension error = %v", err)
	}

	connected := capture.NewCapture()
	defer connected.Close()
	syncRequest := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"fixture-test"}`))
	newSyncHandler(connected).HandleSync(httptest.NewRecorder(), syncRequest)
	for i := 0; i < queries.MaxPendingQueries; i++ {
		if _, err := connected.Queries().CreatePendingQuery(queries.PendingQuery{Type: "occupied"}); err != nil {
			t.Fatalf("fill command queue: %v", err)
		}
	}
	if _, err := qafixturetransport.Execute(context.Background(), connected, "qa", nil, time.Second); err == nil {
		t.Fatal("saturated command queue accepted QA fixture command")
	}
}

func TestExecuteQAFixtureCommandReturnsExtensionResult(t *testing.T) {
	cap := capture.NewCapture()
	defer cap.Close()
	syncRequest := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"fixture-test"}`))
	newSyncHandler(cap).HandleSync(httptest.NewRecorder(), syncRequest)

	go func() {
		cap.Queries().WaitForPendingQueries(time.Second)
		pending := cap.Queries().GetPendingQueries()
		if len(pending) > 0 {
			cap.Queries().SetQueryResultWithClient(pending[0].ID, json.RawMessage(`{"restored":true}`), "")
		}
	}()
	result, err := qafixturetransport.Execute(context.Background(), cap, "qa_restore", json.RawMessage(`{"fixture":"corrupt"}`), time.Second)
	if err != nil || string(result) != `{"restored":true}` {
		t.Fatalf("fixture command result = %s, err = %v", result, err)
	}
}

func TestHandleConfigureDoctorReportsReadinessAndExtraChecks(t *testing.T) {
	cap := capture.NewCapture()
	defer cap.Close()
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 42}
	resp := handleConfigureDoctor(
		health.NewMetrics(), cap, nil,
		func() string { return "inspect local lifecycle logs" },
		[]health.DoctorCheck{{Name: "fixture_recovery", Status: "warn", Detail: "recovery pending"}},
		req,
	)
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("decode doctor result: %v", err)
	}
	if result.IsError {
		t.Fatalf("doctor returned error: %s", result.Content[0].Text)
	}
	for _, want := range []string{"Doctor: unhealthy", "ready_for_interaction", "fixture_recovery", "server_uptime", "inspect local lifecycle logs"} {
		if !strings.Contains(result.Content[0].Text, want) {
			t.Errorf("doctor response missing %q: %s", want, result.Content[0].Text)
		}
	}
}

type shutdownFixtureRecovery struct {
	calledBeforeCancellation bool
}

func (recovery *shutdownFixtureRecovery) RecoverPending(ctx context.Context) []string {
	recovery.calledBeforeCancellation = ctx.Err() == nil
	return nil
}

func TestToolHandlerCloseRecoversFixturesBeforeCancellation(t *testing.T) {
	shutdownCtx, cancel := context.WithCancel(context.Background())
	recovery := &shutdownFixtureRecovery{}
	handler := &ToolHandler{
		shutdownCtx: shutdownCtx, shutdownCancel: cancel,
		fixtureRecovery: recovery, stateRecovery: statediag.NewCollector(),
	}
	handler.Close()
	handler.Close()
	if !recovery.calledBeforeCancellation {
		t.Fatal("fixture recovery did not run before shutdown cancellation")
	}
	if shutdownCtx.Err() == nil {
		t.Fatal("shutdown context remains active after Close")
	}
}

// ============================================
// Dispatch Tests
// ============================================

func TestToolsConfigureDispatch_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{bad json`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_json") {
		t.Errorf("error code should be 'invalid_json', got: %s", result.Content[0].Text)
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureDispatch_MissingAction(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("missing 'action' should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Errorf("error code should be 'missing_param', got: %s", result.Content[0].Text)
	}
	// Verify hint lists valid actions
	text := result.Content[0].Text
	for _, action := range []string{"clear", "health", "noise_rule", "store"} {
		if !strings.Contains(text, action) {
			t.Errorf("hint should list valid action %q", action)
		}
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureDispatch_UnknownAction(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"nonexistent_action"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("unknown action should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "unknown_mode") {
		t.Errorf("error code should be 'unknown_mode', got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "nonexistent_action") {
		t.Error("error should mention the invalid action name")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureDispatch_EmptyArgs(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.configureDispatcher.Handle(req, nil)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("nil args (no 'action') should return isError:true")
	}
}

// ============================================
// Canonical configure action registry tests
// ============================================

func TestToolsConfigure_GetValidConfigureActions(t *testing.T) {
	t.Parallel()

	h, _, _ := makeToolHandler(t)
	actionList := h.configureDispatcher.Actions()
	for i := 1; i < len(actionList); i++ {
		if actionList[i-1] > actionList[i] {
			t.Errorf("actions not sorted: %q > %q", actionList[i-1], actionList[i])
		}
	}

	actions := strings.Join(actionList, ", ")
	for _, required := range []string{"clear", "health", "noise_rule", "store", "load", "streaming"} {
		if !strings.Contains(actions, required) {
			t.Errorf("valid actions missing %q: %s", required, actions)
		}
	}
}

func TestToolsConfigure_QAFixtureValidationIsRegistered(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)
	resp := callConfigureRaw(h, `{"what":"qa_fixture","fixture_action":"validate","fixture":{"version":1}}`)
	result := parseToolResult(t, resp)
	if result.IsError || !strings.Contains(result.Content[0].Text, "QA fixture valid") {
		t.Fatalf("qa_fixture validation failed: %s", result.Content[0].Text)
	}
}

// ============================================
// configure(action:"health") — Response Fields
// ============================================

func TestToolsConfigure_AllActions_ResponseStructure(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	actions := []struct {
		action string
		args   string
	}{
		{"health", `{"what":"health"}`},
		{"telemetry", `{"what":"telemetry"}`},
		{"clear", `{"what":"clear"}`},
		{"noise_rule", `{"what":"noise_rule","noise_action":"list"}`},
		{"load", `{"what":"load"}`},
		{"audit_log", `{"what":"audit_log"}`},
		{"diff_sessions", `{"what":"diff_sessions"}`},
		{"test_boundary_start", `{"what":"test_boundary_start","test_id":"test"}`},
		{"test_boundary_end", `{"what":"test_boundary_end","test_id":"test"}`},
	}

	for _, tc := range actions {
		t.Run(tc.action, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("configure(%s) PANICKED: %v", tc.action, r)
				}
			}()

			resp := callConfigureRaw(h, tc.args)
			if resp.Result == nil {
				t.Fatalf("configure(%s) returned nil result", tc.action)
			}

			result := parseToolResult(t, resp)
			if len(result.Content) == 0 {
				t.Errorf("configure(%s) should return at least one content block", tc.action)
			}
			if result.Content[0].Type != "text" {
				t.Errorf("configure(%s) content type = %q, want 'text'", tc.action, result.Content[0].Type)
			}

			if !result.IsError {
				assertSnakeCaseFields(t, string(resp.Result))
			}
		})
	}
}
