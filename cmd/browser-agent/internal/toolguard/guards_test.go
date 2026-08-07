// guards_test.go — Defines browser-runtime guard policy contracts.
package toolguard

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func guardError(t *testing.T, response mcp.JSONRPCResponse) mcp.StructuredError {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	start := strings.IndexByte(result.Content[0].Text, '{')
	var structured mcp.StructuredError
	if start < 0 || json.Unmarshal([]byte(result.Content[0].Text[start:]), &structured) != nil {
		t.Fatalf("structured guard error missing: %#v", result)
	}
	return structured
}

func TestRuntimeGuardsIncludeDiagnosticStateHints(t *testing.T) {
	t.Parallel()
	request := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}

	disconnectedCapture := capture.NewCapture()
	disconnected := New(disconnectedCapture, context.Background(), time.Millisecond)
	response, blocked := disconnected.RequireExtension(request)
	if !blocked {
		t.Fatal("disconnected extension passed its guard")
	}
	hint := guardError(t, response).Hint
	for _, expected := range []string{"extension=DISCONNECTED", "pilot=", "tracked_tab=", "csp="} {
		if !strings.Contains(hint, expected) {
			t.Errorf("extension hint missing %q: %s", expected, hint)
		}
	}

	pilotCapture := capture.NewCapture()
	capturefixture.SetPilot(pilotCapture, false)
	response, blocked = New(pilotCapture, context.Background(), time.Millisecond).RequirePilot(request)
	if !blocked || !strings.Contains(guardError(t, response).Hint, "pilot=DISABLED") {
		t.Fatalf("pilot guard response = %#v, %t", response, blocked)
	}

	trackingCapture := capture.NewCapture()
	response, blocked = New(trackingCapture, context.Background(), time.Millisecond).RequireTabTracking(request)
	if !blocked || !strings.Contains(guardError(t, response).Hint, "tracked_tab=NONE") {
		t.Fatalf("tracking guard response = %#v, %t", response, blocked)
	}

	cspCapture := capture.NewCapture()
	capturefixture.SetCSP(cspCapture, true, "script_exec")
	response, blocked = New(cspCapture, context.Background(), time.Millisecond).RequireCSPClear(request, "main")
	if !blocked || !strings.Contains(guardError(t, response).Hint, "csp=RESTRICTED(script_exec)") {
		t.Fatalf("CSP guard response = %#v, %t", response, blocked)
	}
}

func TestDefaultExtensionReadinessTimeoutIsBounded(t *testing.T) {
	if DefaultExtensionReadinessTimeout <= 0 || DefaultExtensionReadinessTimeout > 10*time.Second {
		t.Fatalf("DefaultExtensionReadinessTimeout = %v, want (0, 10s]", DefaultExtensionReadinessTimeout)
	}
}

func TestRuntimeGuardPassAndRecoveryContracts(t *testing.T) {
	request := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	captured := capture.NewCapture()
	capturefixture.SetPilot(captured, false)
	guards := New(captured, context.Background(), time.Millisecond)

	pilotResponse, blocked := guards.RequirePilot(request)
	if !blocked || guardError(t, pilotResponse).RecoveryToolCall["tool"] != "observe" {
		t.Fatalf("pilot recovery = %#v blocked=%v", guardError(t, pilotResponse), blocked)
	}
	capturefixture.SetPilot(captured, true)
	if _, blocked = guards.RequirePilot(request); blocked {
		t.Fatal("enabled pilot was blocked")
	}

	extensionResponse, blocked := guards.RequireExtension(request)
	if !blocked || guardError(t, extensionResponse).RecoveryToolCall["tool"] == nil {
		t.Fatalf("extension recovery = %#v blocked=%v", guardError(t, extensionResponse), blocked)
	}
	capturefixture.Connect(captured)
	if _, blocked = guards.RequireExtension(request); blocked {
		t.Fatal("connected extension was blocked")
	}

	trackingResponse, blocked := guards.RequireTabTracking(request)
	if !blocked || guardError(t, trackingResponse).RecoveryToolCall != nil || guardError(t, trackingResponse).RecoveryPlaybook == "" {
		t.Fatalf("tracking recovery = %#v blocked=%v", guardError(t, trackingResponse), blocked)
	}
	capturefixture.Track(captured, 42, "https://example.test")
	if _, blocked = guards.RequireTabTracking(request); blocked {
		t.Fatal("tracked tab was blocked")
	}

	capturefixture.SetCSP(captured, true, "script_exec")
	response, blocked := guards.RequireCSPClear(request, "main")
	if !blocked || guardError(t, response).RecoveryToolCall["tool"] != "interact" {
		t.Fatalf("CSP recovery = %#v blocked=%v", guardError(t, response), blocked)
	}
	for _, world := range []string{"auto", "isolated"} {
		if _, blocked = guards.RequireCSPClear(request, world); blocked {
			t.Fatalf("world %q was blocked", world)
		}
	}
	capturefixture.SetCSP(captured, false, "script_exec")
	if _, blocked = guards.RequireCSPClear(request, "main"); blocked {
		t.Fatal("unrestricted page was blocked")
	}
}

func TestExtensionReadinessUnblocksOnConnectionEvent(t *testing.T) {
	captured := capture.NewCapture()
	guards := New(captured, context.Background(), 500*time.Millisecond)
	done := make(chan bool, 1)
	go func() {
		_, blocked := guards.RequireExtension(mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1})
		done <- blocked
	}()
	capturefixture.Connect(captured)
	if blocked := <-done; blocked {
		t.Fatal("connection event did not release readiness guard")
	}
}

func TestInjectCSPBlockedActionsUsesCanonicalRestrictionMatrix(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		restricted bool
		level      string
		wantCount  int
	}{
		{name: "clear", level: "none"},
		{name: "script execution", restricted: true, level: "script_exec", wantCount: 1},
		{name: "page blocked", restricted: true, level: "page_blocked", wantCount: 16},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			captured := capture.NewCapture()
			capturefixture.SetCSP(captured, testCase.restricted, testCase.level)
			response := New(captured, context.Background(), time.Millisecond).InjectCSPBlockedActions(
				mcp.Succeed(request(), "queued", map[string]any{"status": "queued"}),
			)
			data := guardResultData(t, response)
			actions, present := data["blocked_actions"].([]any)
			if testCase.wantCount == 0 {
				if present || data["blocked_reason"] != nil {
					t.Fatalf("clear response includes CSP guidance: %#v", data)
				}
				return
			}
			if !present || len(actions) != testCase.wantCount || data["blocked_reason"] == "" {
				t.Fatalf("CSP guidance = %#v", data)
			}
			if testCase.level == "script_exec" && actions[0] != "execute_js" {
				t.Fatalf("script-exec actions = %#v", actions)
			}
		})
	}
}

func request() mcp.JSONRPCRequest {
	return mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
}

func guardResultData(t *testing.T, response mcp.JSONRPCResponse) map[string]any {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil || len(result.Content) == 0 {
		t.Fatalf("decode result: %v (%#v)", err, result)
	}
	start := strings.IndexByte(result.Content[0].Text, '{')
	var data map[string]any
	if start < 0 || json.Unmarshal([]byte(result.Content[0].Text[start:]), &data) != nil {
		t.Fatalf("decode result data: %#v", result)
	}
	return data
}
