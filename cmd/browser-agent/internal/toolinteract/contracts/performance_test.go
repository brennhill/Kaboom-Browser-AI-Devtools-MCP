// performance_test.go — External contracts for interact performance baselines.
// Docs: docs/features/feature/interact-explore/index.md

package contracts_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func TestNavigationActionsStoreChangeCoupledPerformanceBaselines(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		action string
		args   json.RawMessage
	}{
		{action: "refresh", args: json.RawMessage(`{"background":true}`)},
		{action: "navigate", args: json.RawMessage(`{"url":"https://example.test/settings","background":true}`)},
	} {
		t.Run(testCase.action, func(t *testing.T) {
			captured := capture.NewCapture()
			defer captured.Close()
			capturefixture.Connect(captured)
			capturefixture.SetPilot(captured, true)
			capturefixture.Track(captured, 42, "https://example.test/dashboard")
			before := performance.PerformanceSnapshot{
				URL: "/dashboard", Timestamp: "before",
				Timing: performance.PerformanceTiming{TimeToFirstByte: 120},
			}
			captured.Performance().Add([]performance.PerformanceSnapshot{before})
			var queued queries.PendingQuery
			runtime := toolinteract.NewActionRuntime(toolinteract.RuntimeDeps{
				RecordAIAction: func(string, string, map[string]any) {},
				EnqueuePendingQuery: func(_ mcp.JSONRPCRequest, query queries.PendingQuery, _ time.Duration) (mcp.JSONRPCResponse, bool) {
					queued = query
					return mcp.JSONRPCResponse{}, false
				},
				MaybeWaitForCommand: func(req mcp.JSONRPCRequest, correlationID string, _ json.RawMessage, summary string) mcp.JSONRPCResponse {
					return mcp.Succeed(req, summary, map[string]any{"correlation_id": correlationID})
				},
			})
			actions := toolinteract.NewBrowserActions(runtime, nil, toolinteract.BrowserDeps{
				RequirePilot: allow, RequireExtension: allow, RequireTabTracking: allow,
				Capture:                 func() *capture.Capture { return captured },
				InjectCSPBlockedActions: func(response mcp.JSONRPCResponse) mcp.JSONRPCResponse { return response },
			})
			response := actions.Handle(testCase.action, request(), testCase.args)
			if result(t, response).IsError || queued.CorrelationID == "" {
				t.Fatalf("%s response=%#v queued=%#v", testCase.action, response, queued)
			}
			stored, exists := captured.Performance().TakeBefore(queued.CorrelationID)
			if !exists || stored.URL != before.URL || stored.Timing.TimeToFirstByte != 120 {
				t.Fatalf("%s baseline = %#v, exists=%t", testCase.action, stored, exists)
			}
		})
	}
}
