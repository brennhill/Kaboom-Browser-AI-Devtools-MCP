// composition_test.go — Tests browser-agent composition-root wiring and contracts.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/binarywatch"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/runtimeflags"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func newTestServerForHandlers(t *testing.T) *Server {
	t.Helper()
	server, err := NewServer(filepath.Join(t.TempDir(), "logs.jsonl"), 1000)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(server.Close)
	return server
}

func TestEarlyDoctorActivatesExplicitStateDirectory(t *testing.T) {
	t.Setenv(state.StateDirEnv, "")
	stateRoot := t.TempDir()
	flags := runtimeflags.Values{DoctorMode: true, StateDir: stateRoot}
	if err := activateEarlyModeStateDir(flags); err != nil {
		t.Fatalf("activate early Doctor state: %v", err)
	}
	resolved, err := state.RootDir()
	if err != nil {
		t.Fatalf("resolve state root: %v", err)
	}
	if resolved != stateRoot {
		t.Fatalf("state root = %q, want %q", resolved, stateRoot)
	}
}

func makeToolHandler(t *testing.T) (*ToolHandler, *Server, *capture.Capture) {
	t.Helper()
	server := newTestServerForHandlers(t)
	server.sessionProjectPath = t.TempDir()
	captured := capture.NewCapture()
	capturefixture.SetPilot(captured, false)
	return NewToolHandler(server, captured), server, captured
}

func decodeJSONMap(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode JSON: %v; body=%q", err, body)
	}
	return decoded
}

func localRequest(method, path string, body io.Reader) *http.Request {
	return httptest.NewRequest(method, "http://localhost"+path, body)
}

func extensionRouteRequest(method, path string, body io.Reader) *http.Request {
	request := localRequest(method, path, body)
	request.Header.Set("X-Kaboom-Client", "kaboom-extension/test")
	return request
}

func TestSetupHTTPRoutesRejectsWrongMethodsForPostOnlyEndpoints(t *testing.T) {
	t.Parallel()
	server := newTestServerForHandlers(t)
	mux, _ := setupHTTPRoutes(server, capture.NewCapture())
	paths := []string{
		"/websocket-events", "/network-bodies", "/network-waterfall", "/query-result",
		"/enhanced-actions", "/performance-snapshots", "/sync", "/logs", "/screenshots",
		"/draw-mode/complete", "/shutdown", "/clear", "/test-boundary",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, extensionRouteRequest(http.MethodGet, path, nil))
			if recorder.Code != http.StatusMethodNotAllowed {
				t.Errorf("GET %s status = %d, want 405", path, recorder.Code)
			}
		})
	}
}

func TestSetupHTTPRoutesReturnsJSONForMalformedIngestPayloads(t *testing.T) {
	t.Parallel()
	server := newTestServerForHandlers(t)
	mux, _ := setupHTTPRoutes(server, capture.NewCapture())
	paths := []string{
		"/network-bodies", "/network-waterfall", "/query-result", "/enhanced-actions",
		"/performance-snapshots", "/logs", "/draw-mode/complete",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := extensionRouteRequest(http.MethodPost, path, bytes.NewBufferString("{invalid"))
			mux.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Errorf("POST %s status = %d, want 400", path, recorder.Code)
			}
			if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
				t.Errorf("POST %s content type = %q, want application/json", path, contentType)
			}
			if body := decodeJSONMap(t, recorder.Body.Bytes()); body["error"] == nil {
				t.Errorf("POST %s response missing error: %#v", path, body)
			}
		})
	}
}

func TestSetupHTTPRoutesRegistersCoreEndpoints(t *testing.T) {
	t.Parallel()
	server := newTestServerForHandlers(t)
	mux, _ := setupHTTPRoutes(server, capture.NewCapture())

	jsonRoot := localRequest(http.MethodGet, "/", nil)
	jsonRoot.Header.Set("Accept", "application/json")
	jsonRecorder := httptest.NewRecorder()
	mux.ServeHTTP(jsonRecorder, jsonRoot)
	if jsonRecorder.Code != http.StatusOK || decodeJSONMap(t, jsonRecorder.Body.Bytes())["name"] != "kaboom-browser-devtools" {
		t.Fatalf("JSON root = %d %s", jsonRecorder.Code, jsonRecorder.Body.String())
	}

	htmlRecorder := httptest.NewRecorder()
	mux.ServeHTTP(htmlRecorder, localRequest(http.MethodGet, "/", nil))
	if htmlRecorder.Code != http.StatusOK || htmlRecorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("HTML root = %d %q", htmlRecorder.Code, htmlRecorder.Header().Get("Content-Type"))
	}

	for _, test := range []struct {
		path   string
		status int
	}{
		{"/health", http.StatusOK},
		{"/diagnostics", http.StatusOK},
		{"/diagnostics.json", http.StatusNotFound},
		{"/missing", http.StatusNotFound},
	} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, localRequest(http.MethodGet, test.path, nil))
		if recorder.Code != test.status {
			t.Errorf("GET %s = %d, want %d", test.path, recorder.Code, test.status)
		}
	}
}

func TestSetupHTTPRoutesNilCaptureDoesNotPanic(t *testing.T) {
	server := newTestServerForHandlers(t)
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("setupHTTPRoutes panicked with nil capture: %v\n%s", recovered, debug.Stack())
		}
	}()
	mux, handler := setupHTTPRoutes(server, nil)
	if mux == nil || handler == nil {
		t.Fatal("setupHTTPRoutes returned a nil route dependency")
	}
}

func TestNewToolHandlerUsesServerSessionProjectPath(t *testing.T) {
	t.Parallel()
	projectPath := t.TempDir()
	server := newTestServerForHandlers(t)
	server.sessionProjectPath = projectPath
	handler := NewToolHandler(server, capture.NewCapture())
	if handler.sessionStoreImpl == nil {
		t.Fatal("session store was not initialized")
	}
	if err := handler.sessionStoreImpl.Save("saved_states", "isolated", []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	projectDir, err := state.ProjectDir(projectPath)
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
	captured := capture.NewCapture()
	server := newTestServerForHandlers(t)
	handler := NewToolHandler(server, captured)
	if handler.capture != captured {
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
	captured.Queries().RegisterCommand("warn-timeout", "query-warn-timeout", time.Minute)
	captured.Queries().ApplyCommandResult("warn-timeout", "timeout", nil, "synthetic-timeout")

	response := getHealthResponse(hm, captured, nil, nil, nil, "test")
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
	assertSameStringSet(t, "analyze.what enum vs analyze dispatcher", mustToolEnumValues(t, "analyze", "what"), handler.analyzeDispatcher.ValidModes())
}

func TestSchemaParity_GenerateWhatEnumMatchesHandlers(t *testing.T) {
	t.Parallel()
	assertSameStringSet(t, "generate.what enum vs generate handlers", mustToolEnumValues(t, "generate", "what"), strings.Split(toolgenerate.ValidFormats(), ", "))
}

func TestSchemaParity_ConfigureWhatEnumMatchesHandlers(t *testing.T) {
	t.Parallel()
	handler, _, _ := makeToolHandler(t)
	assertSameStringSet(t, "configure.what enum vs configure dispatcher", mustToolEnumValues(t, "configure", "what"), handler.configureDispatcher.Actions())
}

func TestSchemaParity_InteractWhatEnumMatchesDispatch(t *testing.T) {
	t.Parallel()
	handler, _, _ := makeToolHandler(t)
	assertSameStringSet(t, "interact.what enum vs interact runtime actions", mustToolEnumValues(t, "interact", "what"), handler.interactDispatcher.ActionNames())
}

func TestSchemaParity_ObserveWhatEnumMatchesHandlers(t *testing.T) {
	t.Parallel()
	handler, _, _ := makeToolHandler(t)
	silent := map[string]bool{"annotations": true, "annotation_detail": true, "draw_history": true, "draw_session": true}
	runtimeModes := make([]string, 0)
	for _, mode := range handler.observeDispatcher.ValidModes() {
		if !silent[mode] {
			runtimeModes = append(runtimeModes, mode)
		}
	}
	assertSameStringSet(t, "observe.what enum vs observe dispatcher", mustToolEnumValues(t, "observe", "what"), runtimeModes)
}

func TestIssueReportCompositionCollectsAvailableRuntimeEvidence(t *testing.T) {
	handler, server, captured := makeToolHandler(t)
	server.logs.AddEntries([]types.LogEntry{{"level": "error", "message": "fixture"}})
	captured.Telemetry().AddNetworkBodies([]types.NetworkBody{{URL: "https://example.test", Method: http.MethodGet}})
	captured.Telemetry().AddEnhancedActions([]types.EnhancedAction{{Type: "click"}})

	report := handler.issueReportDeps.Collect("bug", "Runtime evidence", "local context")
	if report.Template != "bug" || report.Title != "Runtime evidence" || report.UserContext != "local context" {
		t.Fatalf("report identity = %#v", report)
	}
	if report.Diagnostics.Server.Version != version || report.Diagnostics.Platform.OS == "" || report.Diagnostics.Platform.Arch == "" {
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
	handler, server, captured := makeToolHandler(t)
	server.logs.AddEntries([]types.LogEntry{{"level": "warn", "message": "fixture"}})
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

// --- Daemon startup wiring helpers (main_connection_mcp.go) ---

func TestReportUpgradeMarkerAnnouncesAndClearsMarker(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)
	server := newTestServerForHandlers(t)

	markerPath, err := state.UpgradeMarkerFile()
	if err != nil {
		t.Fatal(err)
	}
	if err := binarywatch.WriteMarker("1.2.3", "1.3.0", markerPath); err != nil {
		t.Fatal(err)
	}

	reportUpgradeMarker(server, 7890)

	warnings := server.warnings.Drain()
	if len(warnings) != 1 || !strings.Contains(warnings[0], "Upgraded from v1.2.3 to v1.3.0") {
		t.Fatalf("warnings = %#v", warnings)
	}
	if _, statErr := os.Stat(markerPath); !os.IsNotExist(statErr) {
		t.Fatalf("marker not cleared: %v", statErr)
	}
}

func TestReportUpgradeMarkerSilentWithoutMarker(t *testing.T) {
	t.Setenv(state.StateDirEnv, t.TempDir())
	server := newTestServerForHandlers(t)

	reportUpgradeMarker(server, 7890)

	if warnings := server.warnings.Drain(); len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
}

func TestEnsurePortAvailableSucceedsOnFreePort(t *testing.T) {
	server := newTestServerForHandlers(t)
	port := freeLoopbackPort(t)

	if err := ensurePortAvailable(server, daemonHTTPDeps(server), port, "test"); err != nil {
		t.Fatalf("free port rejected: %v", err)
	}
}

func TestEnsurePortAvailableErrorsWhenPortHeldBySelf(t *testing.T) {
	server := newTestServerForHandlers(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	heldPort := listener.Addr().(*net.TCPAddr).Port

	// The holder is this test process, which ReclaimPort must not kill; the
	// retry preflight still fails, so the error is returned.
	err = ensurePortAvailable(server, daemonHTTPDeps(server), heldPort, "test")
	if err == nil {
		t.Fatal("self-held port must surface an error, not start a rival server")
	}
	if !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("error = %v", err)
	}
}

func TestRecordTerminalUnavailableReportsBindFailure(t *testing.T) {
	server := newTestServerForHandlers(t)

	recordTerminalUnavailable(server, 17891, errors.New("bind: address already in use"))

	snapshot := server.terminalStatus.Snapshot()
	if snapshot.Available {
		t.Fatal("terminal must be marked unavailable")
	}
	if snapshot.Port != 17891 {
		t.Fatalf("port = %d, want the wanted port 17891", snapshot.Port)
	}
	if !strings.Contains(snapshot.Error, "address already in use") {
		t.Fatalf("error = %q", snapshot.Error)
	}
}

func TestStartBinaryUpgradeWatcherRespectsDisableEnv(t *testing.T) {
	t.Setenv("KABOOM_NO_AUTO_UPGRADE", "1")
	server := newTestServerForHandlers(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startBinaryUpgradeWatcher(server, 7890, ctx)

	// binarywatch.Start returns a nil *State when disabled, which reaches the
	// runtime interface as a typed nil — assert on the concrete value.
	if watcher, _ := server.runtime.Upgrade().(*binarywatch.State); watcher != nil {
		t.Fatal("disabled auto-upgrade must not register a watcher")
	}
}

func TestStartBinaryUpgradeWatcherRegistersWatcher(t *testing.T) {
	t.Setenv("KABOOM_NO_AUTO_UPGRADE", "")
	server := newTestServerForHandlers(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startBinaryUpgradeWatcher(server, 7890, ctx)

	watcher, ok := server.runtime.Upgrade().(*binarywatch.State)
	if !ok || watcher == nil {
		t.Fatal("watcher was not registered on the runtime")
	}
	if pending, newVersion, _ := watcher.UpgradeInfo(); pending || newVersion != "" {
		t.Fatalf("fresh watcher reports upgrade = (%v, %q)", pending, newVersion)
	}
	cancel()
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
	handler, server, captured := makeToolHandler(t)
	repeated := make([]types.LogEntry, 20)
	for i := range repeated {
		repeated[i] = types.LogEntry{"level": "error", "message": "third-party poll tick"}
	}
	server.logs.AddEntries(repeated)

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
	handler, server, captured := makeToolHandler(t)
	server.logs.AddEntries([]types.LogEntry{{"level": "error", "message": "boom"}})
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

	nilCapture := buildTestGenerationDeps(&ToolHandler{server: server})
	if nilCapture.EnhancedActions() != nil || nilCapture.NetworkBodies() != nil {
		t.Fatal("nil capture must yield nil action and network buffers")
	}
}

func TestVisualAnalyzeDepsExposeScreenshotGuardAndSessionStore(t *testing.T) {
	handler, server, _ := makeToolHandler(t)
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

	bare := visualAnalyzeDeps{h: &ToolHandler{server: server}}
	if bare.HasSessionStore() {
		t.Fatal("handler without a session store must report false")
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

func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
