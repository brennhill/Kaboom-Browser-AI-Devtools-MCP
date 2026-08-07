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
