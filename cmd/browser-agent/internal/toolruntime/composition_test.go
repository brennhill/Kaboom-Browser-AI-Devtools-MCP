// composition_test.go — Tests the five-tool runtime's composition and its
// schema parity with the shipped tool document.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package toolruntime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/appruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcptelemetry"
	terminalintent "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/intent"
	terminalstatus "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/status"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/activecodebase"
	annotationruntime "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation/runtime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/listenport"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/warningqueue"
)

// fixtureVersion is the build string the fixture state reports. It is not the
// shipped version: nothing here should assert on the real release number.
const fixtureVersion = "0.0.0-test"

// newFixtureState builds the real daemon stores the runtime reads, rooted in a
// temp directory. Every store is the production type — a stubbed store would
// prove the test's own wiring rather than the runtime's.
func newFixtureState(t *testing.T) ServerState {
	t.Helper()
	warnings := warningqueue.New()
	logs := logstore.New(logstore.Config{
		LogFile:       filepath.Join(t.TempDir(), "logs.jsonl"),
		MaxEntries:    1000,
		TelemetryMode: mcptelemetry.ModeAuto,
		AddWarning:    warnings.Add,
	})
	annotations := annotationruntime.New(10 * time.Minute)
	t.Cleanup(annotations.Close)
	return ServerState{
		Version:            fixtureVersion,
		Runtime:            appruntime.New(fixtureVersion),
		SessionProjectPath: t.TempDir(),
		Logs:               logs,
		Warnings:           warnings,
		Incidents:          incident.NewStore(100, nil),
		PushInbox:          push.NewPushInbox(50),
		ActiveCodebase:     activecodebase.New(),
		AnnotationRuntime:  annotations,
		ListenPort:         listenport.New(),
		TerminalStatus:     terminalstatus.New(),
		IntentStore:        terminalintent.NewStore(),
		StateRecovery:      statediag.NewCollector(),
	}
}

func makeToolHandler(t *testing.T) (*ToolHandler, ServerState, *capture.Capture) {
	t.Helper()
	runtimeState := newFixtureState(t)
	captured := capture.NewCapture()
	capturefixture.SetPilot(captured, false)
	handler := NewToolHandler(runtimeState, captured)
	t.Cleanup(handler.Close)
	return handler, runtimeState, captured
}

func TestNewToolHandlerUsesTheDaemonSessionProjectPath(t *testing.T) {
	t.Parallel()
	handler, runtimeState, _ := makeToolHandler(t)
	if handler.sessionStoreImpl == nil {
		t.Fatal("session store was not initialized")
	}
	if err := handler.sessionStoreImpl.Save("saved_states", "isolated", []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	projectDir, err := state.ProjectDir(runtimeState.SessionProjectPath)
	if err != nil {
		t.Fatalf("ProjectDir() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "saved_states", "isolated.json")); err != nil {
		t.Fatalf("isolated state missing: %v", err)
	}
}

func TestToolHandlerRecordsUsageOutcomesAndSessionDepth(t *testing.T) {
	t.Parallel()
	handler, _, _ := makeToolHandler(t)
	tracker := telemetry.NewUsageTracker()
	handler.usageTracker = tracker
	request := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1, Method: "tools/call"}

	if _, handled := handler.HandleToolCall(request, "observe", json.RawMessage(`{"what":"errors"}`)); !handled {
		t.Fatal("observe was not handled")
	}
	if _, handled := handler.HandleToolCall(request, "interact", json.RawMessage(`{}`)); !handled {
		t.Fatal("interact was not handled")
	}
	counts := tracker.DebugCounts()
	if counts["observe:errors"] != 1 || counts["interact:unknown"] != 1 || counts["err:interact:unknown"] != 1 {
		t.Fatalf("usage counts = %#v", counts)
	}
	if tracker.SessionDepth() != 2 {
		t.Fatalf("session depth = %d, want 2", tracker.SessionDepth())
	}
	snapshot := tracker.SwapAndReset()
	if snapshot == nil || len(snapshot.ToolStats) == 0 {
		t.Fatalf("usage snapshot = %#v", snapshot)
	}
}

func TestMCPCaptureConfigured(t *testing.T) {
	t.Parallel()
	handler, _, captured := makeToolHandler(t)
	if handler.Capture() != captured {
		t.Fatal("MCP handler should retain the injected capture")
	}
}

func TestNewToolHandlerWiresCanonicalFiveToolCatalog(t *testing.T) {
	t.Parallel()
	handler, _, _ := makeToolHandler(t)
	for _, name := range []string{"observe", "analyze", "generate", "configure", "interact"} {
		module, ok := handler.toolCatalog.Get(name)
		if !ok || module == nil || module.Describe().Name != name || len(module.Examples()) == 0 {
			t.Errorf("tool catalog module %q = %#v, %t", name, module, ok)
		}
	}
}

func TestMCPToolCallLimiterConfigured(t *testing.T) {
	t.Parallel()
	handler, _, _ := makeToolHandler(t)
	if handler.toolCallLimiter == nil || !handler.toolCallLimiter.Allow() {
		t.Fatal("fresh MCP tool call limiter should be configured and allow its first call")
	}
}

func TestMCPRedactionEngineConfigured(t *testing.T) {
	t.Parallel()
	handler, _, _ := makeToolHandler(t)
	if handler.redactionEngine == nil {
		t.Fatal("MCP redaction engine should be configured")
	}
}

func TestHealthResponseIncludesCommandExecution(t *testing.T) {
	t.Parallel()
	hm := health.NewMetrics()
	captured := capture.NewCapture()
	t.Cleanup(captured.Close)
	captured.Queries().RegisterCommand("warn-timeout", "query-warn-timeout", time.Minute)
	captured.Queries().ApplyCommandResult("warn-timeout", "timeout", nil, "synthetic-timeout")

	response := getHealthResponse(hm, captured, ServerState{}, nil, nil, "test")
	if response.CommandExecution.Status != "warn" || response.CommandExecution.Ready {
		t.Fatalf("command execution = %#v, want non-ready warning", response.CommandExecution)
	}
	if response.CommandExecution.RecentTimeoutCount != 1 {
		t.Fatalf("recent timeout count = %d, want 1", response.CommandExecution.RecentTimeoutCount)
	}
}

func TestSchemaParity_AnalyzeWhatEnumMatchesHandlers(t *testing.T) {
	t.Parallel()
	handler, _, _ := makeToolHandler(t)
	assertSameStringSet(t, "analyze.what enum vs analyze dispatcher", mustToolEnumValues(t, "analyze", "what"), handler.DispatchableModes()["analyze"])
}

func TestSchemaParity_GenerateWhatEnumMatchesHandlers(t *testing.T) {
	t.Parallel()
	handler, _, _ := makeToolHandler(t)
	assertSameStringSet(t, "generate.what enum vs generate handlers", mustToolEnumValues(t, "generate", "what"), handler.DispatchableModes()["generate"])
}

func TestSchemaParity_ConfigureWhatEnumMatchesHandlers(t *testing.T) {
	t.Parallel()
	handler, _, _ := makeToolHandler(t)
	assertSameStringSet(t, "configure.what enum vs configure dispatcher", mustToolEnumValues(t, "configure", "what"), handler.DispatchableModes()["configure"])
}

func TestSchemaParity_InteractWhatEnumMatchesDispatch(t *testing.T) {
	t.Parallel()
	handler, _, _ := makeToolHandler(t)
	assertSameStringSet(t, "interact.what enum vs interact runtime actions", mustToolEnumValues(t, "interact", "what"), handler.DispatchableModes()["interact"])
}

func TestSchemaParity_ObserveWhatEnumMatchesHandlers(t *testing.T) {
	t.Parallel()
	handler, _, _ := makeToolHandler(t)
	silent := map[string]bool{"annotations": true, "annotation_detail": true, "draw_history": true, "draw_session": true}
	runtimeModes := make([]string, 0)
	for _, mode := range handler.DispatchableModes()["observe"] {
		if !silent[mode] {
			runtimeModes = append(runtimeModes, mode)
		}
	}
	assertSameStringSet(t, "observe.what enum vs observe dispatcher", mustToolEnumValues(t, "observe", "what"), runtimeModes)
}

func TestIssueReportCompositionCollectsAvailableRuntimeEvidence(t *testing.T) {
	handler, runtimeState, captured := makeToolHandler(t)
	runtimeState.Logs.AddEntries([]types.LogEntry{{"level": "error", "message": "fixture"}})
	captured.Telemetry().AddNetworkBodies([]types.NetworkBody{{URL: "https://example.test", Method: http.MethodGet}})
	captured.Telemetry().AddEnhancedActions([]types.EnhancedAction{{Type: "click"}})

	report := handler.issueReportDeps.Collect("bug", "Runtime evidence", "local context")
	if report.Template != "bug" || report.Title != "Runtime evidence" || report.UserContext != "local context" {
		t.Fatalf("report identity = %#v", report)
	}
	if report.Diagnostics.Server.Version != fixtureVersion || report.Diagnostics.Platform.OS == "" || report.Diagnostics.Platform.Arch == "" {
		t.Fatalf("report runtime identity = %#v", report.Diagnostics)
	}
	if report.Diagnostics.Buffers.ConsoleEntries != 1 || report.Diagnostics.Buffers.NetworkEntries != 1 || report.Diagnostics.Buffers.ActionEntries != 1 {
		t.Fatalf("report buffer evidence = %#v", report.Diagnostics.Buffers)
	}

	minimal := buildIssueReportDeps(&ToolHandler{}).Collect("feature", "Minimal", "")
	if minimal.Template != "feature" || minimal.Diagnostics.Buffers.ConsoleEntries != 0 {
		t.Fatalf("minimal report = %#v", minimal)
	}
}

func TestConfigureCompositionReadsCanonicalRuntimeOwners(t *testing.T) {
	handler, runtimeState, captured := makeToolHandler(t)
	runtimeState.Logs.AddEntries([]types.LogEntry{{"level": "warn", "message": "fixture"}})
	captured.Telemetry().AddNetworkBodies([]types.NetworkBody{{URL: "https://example.test/api"}})
	captured.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{{Type: "ws_connect", ID: "fixture"}})

	deps := handler.configureLocalDeps
	if deps.NoiseConfig() != handler.noiseConfig || !deps.HasCapture() {
		t.Fatal("configure dependencies are detached from canonical runtime owners")
	}
	if len(deps.ConsoleEntries()) != 1 || len(deps.NetworkBodies()) != 1 || len(deps.AllWebSocketEvents()) != 1 {
		t.Fatalf("configure runtime evidence is incomplete: logs=%d network=%d websocket=%d",
			len(deps.ConsoleEntries()), len(deps.NetworkBodies()), len(deps.AllWebSocketEvents()))
	}
	if len(deps.ToolsList()) != len(schema.AllTools()) || deps.GetToolModuleExamples("observe") == nil {
		t.Fatal("configure schema dependencies are not canonical")
	}
	if deps.GetToolModuleExamples("missing-tool") != nil {
		t.Fatal("unknown tool unexpectedly returned module examples")
	}
	deps.InteractActionSetJitter(17)
	if deps.InteractActionGetJitter() != 17 || deps.GetTelemetryMode() == "" {
		t.Fatal("configure mutable settings are detached from their runtime owners")
	}
}

func TestInitNoiseAutoDetectionRunsConfiguredFirstConnectDetector(t *testing.T) {
	handler, _, captured := makeToolHandler(t)
	done := make(chan struct{})
	var once sync.Once
	handler.noiseFirstConnectFn = func() { once.Do(func() { close(done) }) }

	// NewToolHandler already wired one subscription; wiring again proves the
	// callback path still routes to the handler's configured detector.
	initNoiseAutoDetection(handler)
	captured.Lifecycle().Emit(lifecycle.EventExtensionConnected, nil)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("first-connect detector never ran")
	}
}

func TestInitNoiseAutoDetectionAppliesDetectedNoiseRules(t *testing.T) {
	handler, runtimeState, captured := makeToolHandler(t)
	repeated := make([]types.LogEntry, 20)
	for i := range repeated {
		repeated[i] = types.LogEntry{"level": "error", "message": "third-party poll tick"}
	}
	runtimeState.Logs.AddEntries(repeated)

	initNoiseAutoDetection(handler)
	captured.Lifecycle().Emit(lifecycle.EventExtensionConnected, nil)

	deadline := time.After(3 * time.Second)
	for len(handler.noiseConfig.ListRules()) == 0 {
		select {
		case <-deadline:
			t.Fatal("noise auto-detect did not apply the repetitive-message rule")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func TestBuildTestGenerationDepsReadsCanonicalBuffers(t *testing.T) {
	handler, runtimeState, captured := makeToolHandler(t)
	runtimeState.Logs.AddEntries([]types.LogEntry{{"level": "error", "message": "boom"}})
	captured.Telemetry().AddNetworkBodies([]types.NetworkBody{{URL: "https://example.test/api"}})
	captured.Telemetry().AddEnhancedActions([]types.EnhancedAction{{Type: "click"}})

	deps := buildTestGenerationDeps(handler)
	if entries := deps.LogEntries(); len(entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(entries))
	}
	if bodies := deps.NetworkBodies(); len(bodies) != 1 {
		t.Fatalf("network bodies = %d, want 1", len(bodies))
	}
	if actions := deps.EnhancedActions(); len(actions) != 1 {
		t.Fatalf("actions = %d, want 1", len(actions))
	}

	nilCapture := buildTestGenerationDeps(&ToolHandler{state: runtimeState})
	if nilCapture.EnhancedActions() != nil || nilCapture.NetworkBodies() != nil {
		t.Fatal("nil capture must yield nil action and network buffers")
	}
}

func TestVisualAnalyzeDepsExposeScreenshotGuardAndSessionStore(t *testing.T) {
	handler, runtimeState, _ := makeToolHandler(t)
	request := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: float64(7), ClientID: "client-test"}

	deps := visualAnalyzeDeps{h: handler}
	if !deps.HasSessionStore() {
		t.Fatal("handler with a session project path must report a session store")
	}
	resp := deps.CaptureScreenshot(request)
	result := decodeMCPToolResult(t, resp)
	if !result.IsError {
		t.Fatal("screenshot without a tracked tab must fail, not hang")
	}

	bare := visualAnalyzeDeps{h: &ToolHandler{state: runtimeState}}
	if bare.HasSessionStore() {
		t.Fatal("handler without a session store must report false")
	}
}

func mustToolEnumValues(t *testing.T, toolName, propertyName string) []string {
	t.Helper()
	for _, tool := range schema.AllTools() {
		if tool.Name != toolName {
			continue
		}
		properties, ok := tool.InputSchema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("tool %q schema missing properties", toolName)
		}
		property, ok := properties[propertyName].(map[string]any)
		if !ok {
			t.Fatalf("tool %q schema missing property %q", toolName, propertyName)
		}
		values, err := stringSlice(property["enum"])
		if err != nil {
			t.Fatalf("tool %q property %q enum: %v", toolName, propertyName, err)
		}
		sort.Strings(values)
		return values
	}
	t.Fatalf("tool %q not found", toolName)
	return nil
}

func stringSlice(value any) ([]string, error) {
	switch values := value.(type) {
	case []string:
		return append([]string(nil), values...), nil
	case []any:
		result := make([]string, 0, len(values))
		for index, item := range values {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("enum[%d] is %T, want string", index, item)
			}
			result = append(result, text)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported enum type %T", value)
	}
}

func assertSameStringSet(t *testing.T, label string, got, want []string) {
	t.Helper()
	gotSet := make(map[string]bool, len(got))
	wantSet := make(map[string]bool, len(want))
	for _, value := range got {
		gotSet[value] = true
	}
	for _, value := range want {
		wantSet[value] = true
	}
	missingInSchema := make([]string, 0)
	missingInRuntime := make([]string, 0)
	for value := range wantSet {
		if !gotSet[value] {
			missingInSchema = append(missingInSchema, value)
		}
	}
	for value := range gotSet {
		if !wantSet[value] {
			missingInRuntime = append(missingInRuntime, value)
		}
	}
	sort.Strings(missingInSchema)
	sort.Strings(missingInRuntime)
	if len(missingInSchema) > 0 || len(missingInRuntime) > 0 {
		t.Fatalf("%s mismatch\nmissing_in_schema=%v\nmissing_in_runtime=%v", label, missingInSchema, missingInRuntime)
	}
}

func decodeMCPToolResult(t *testing.T, resp mcp.JSONRPCResponse) mcp.MCPToolResult {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("decode tool result: %v", err)
	}
	return result
}
