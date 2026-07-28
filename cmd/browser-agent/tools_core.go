// Purpose: Defines the ToolHandler struct, shared state (capture, AI client, sequence store), and tool dispatch infrastructure.
// Why: All five MCP tools share a common handler that owns capture state, extension connectivity, and session context.

package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/noiseautorun"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/screenrec"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/summarypref"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/testgenhandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/analyzedispatch"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/annotationanalysis"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/netrecord"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract/interactstate"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract/interactupload"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolmodule"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolobserve"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolrecording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/analysis/apicontract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/analysis/thirdparty"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/audit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/issuereport"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording/playback"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/redaction"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/scan"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/streaming/alertbuf"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	observe "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

// Note: Response helpers, error codes, and validation functions have been moved to:
// - internal/mcp and internal/toolresp — Canonical response formatting
// - tools_errors.go — Error codes and structured error handling
// - tools_validation.go — Parameter validation utilities

// ============================================
// ToolHandler Definition
// ============================================

// ToolHandler extends MCPHandler with composite tool dispatch
type ToolHandler struct {
	*MCPHandler
	capture *capture.Capture

	// shutdownCtx is cancelled when the ToolHandler is closed. Gates like
	// requireExtension pass this context to blocking waits so they abort
	// promptly on server shutdown instead of leaking goroutines.
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc

	// Health metrics for MCP get_health tool
	healthMetrics *health.Metrics

	// Redaction engine for scrubbing sensitive data from tool responses
	redactionEngine RedactionEngine

	// Rate limiter for MCP tool calls (sliding window)
	toolCallLimiter *toolresp.ToolCallLimiter
	*toolguard.Guards

	// Alert system + context streaming (delegates to internal/streaming)
	alertBuffer *alertbuf.AlertBuffer

	// Concrete implementations (interface signatures differ from types package)
	// These are used directly by tool handlers rather than through the interface fields above.
	noiseConfig           *noise.NoiseConfig
	sessionStoreImpl      *persistence.SessionStore
	securityScannerImpl   *scan.Scanner
	thirdPartyAuditorImpl *thirdparty.ThirdPartyAuditor
	sessionManager        *session.SessionManager
	auditTrail            *audit.AuditTrail
	auditRecorder         *audit.Recorder

	// Draw mode annotation store (in-memory, TTL-based)
	annotationStore *annotation.Store

	// API contract validation state (incremental over captured network bodies).
	apiContractRuntime *apicontract.Runtime

	// Upload security config (folder-scoped permissions + denylist)
	uploadSecurity *uploadsec.Security

	// Cold-start readiness gate timeout: how long requireExtension waits
	// for the extension to connect before failing. MaybeWaitForCommand only
	// does an instant check (P1-2: no double wait).
	// Default: 5s. Set to 0 in tests to restore instant-fail behavior.
	coldStartTimeout time.Duration

	// Dedicated interact action routing/jitter sub-handler.
	interactActionHandler *toolinteract.InteractActionHandler

	// Active test boundaries: test_id → start time.
	// Used to detect out-of-order test_boundary_end calls.
	activeBoundariesMu sync.Mutex
	activeBoundaries   map[string]time.Time

	recordingInteractHandler *screenrec.InteractHandler
	recordingHandler         *toolrecording.Handler
	uploadInteractHandler    *interactupload.Handler
	testGenHandler           *testgenhandler.Handler
	generateDispatcher       *toolgenerate.Dispatcher
	observeDispatcher        *toolobserve.Dispatcher
	stateInteractHandler     *interactstate.Handler
	configureSessions        *toolconfigure.SessionHandler
	annotationAnalysis       *annotationanalysis.Handler
	analyzeDispatcher        *analyzedispatch.Dispatcher

	// Passive network traffic recording state (start/stop capture).
	networkRecording *netrecord.NetworkRecordingState

	// Module registry for plugin-style tool dispatch (incremental migration).
	toolModulesOnce sync.Once
	toolModules     *toolmodule.Registry

	// Tool schema cache for parameter-warning validation.
	toolSchemasOnce sync.Once
	toolSchemas     map[string]map[string]any

	// Session-level response-mode preference.
	summaryPrefs *summarypref.Cache

	// noiseFirstConnectFn overrides the noise auto-detect function for first-connection.
	// When nil, the canonical noiseautorun detector is used.
	noiseFirstConnectFn func()

	// issueCommandRunner overrides the exec runner for issue submission.
	// When nil, issuereport.ExecRunner{} is used. Set in tests to inject a fake.
	issueCommandRunner issuereport.CommandRunner

	// usageCounter tracks tool:action call counts for periodic usage beacons.
	// When nil, usage counting is disabled (backwards compatible).
	usageTracker *telemetry.UsageTracker
}

// IsConsoleNoise implements mcp.NoiseFilterer.
func (h *ToolHandler) IsConsoleNoise(entry types.LogEntry) bool {
	if h.noiseConfig == nil {
		return false
	}
	return h.noiseConfig.IsConsoleNoise(entry)
}

// maybeWaitForCommand, formatCommandResult, and related async infrastructure
// live in tools_async_completion.go; the result shaping
// they call lives in internal/asyncresult.

// handleToolCall dispatches composite tool calls by mode parameter.
func (h *ToolHandler) HandleToolCall(req mcp.JSONRPCRequest, name string, args json.RawMessage) (mcp.JSONRPCResponse, bool) {
	start := time.Now()

	h.ensureToolModules()
	h.ensureToolSchemas()
	resp, handled := h.toolModules.Dispatch(req, name, args)
	if !handled {
		return mcp.JSONRPCResponse{}, false
	}

	parsedResult, parsedOK := parseToolResultForPostProcessing(resp.Result)
	resultIsError := false
	if parsedOK {
		resultIsError = parsedResult.IsError
	} else {
		resultIsError = isToolResultError(resp.Result)
	}

	// Validate params against tool schema and append warnings for unknown fields.
	// Skip validation for error responses (already failed, warnings would be noise).
	if !resultIsError {
		if schema := h.getToolSchema(name); schema != nil {
			if warnings := mcp.ValidateParamsAgainstSchema(args, schema); len(warnings) > 0 {
				if parsedOK && mcp.AppendWarningsToToolResult(parsedResult, warnings) {
					resp.Result = mcp.SafeMarshal(parsedResult, string(resp.Result))
				} else {
					resp = mcp.AppendWarningsToResponse(resp, warnings)
				}
			}
		}
	}

	// Health metrics: local-only monotonic counters for the MCP health dashboard.
	// Never beaconed — survives counter resets. Exposed via configure(what='health').
	if h.healthMetrics != nil {
		h.healthMetrics.IncrementRequest(name)
		if resp.Error != nil || resultIsError {
			h.healthMetrics.IncrementError(name)
		}
	}

	// Piggyback push inbox hint if events are pending
	resp = toolobserve.AppendPushPiggyback(h, resp)

	h.auditRecorder.Record(req, name, args, resp, start)

	// Usage tracker: per-call telemetry beaconed immediately + aggregated every 5 min.
	// Separate from healthMetrics — different lifecycle and purpose.
	if h.usageTracker != nil {
		key := usageKey(args)
		if key == "" {
			key = "unknown"
		}
		h.usageTracker.RecordToolCall(name+":"+key, time.Since(start), resp.Error != nil || resultIsError)
	}

	return resp, true
}

// getToolSchema returns the InputSchema for a tool by name (cached).
func (h *ToolHandler) getToolSchema(name string) map[string]any {
	h.ensureToolSchemas()
	return h.toolSchemas[name]
}

func (h *ToolHandler) ensureToolModules() {
	h.toolModulesOnce.Do(func() {
		h.toolModules = toolmodule.New(
			toolmodule.Spec{Name: "observe", Summary: "Read captured browser state: logs, network, screenshots, and async results",
				Examples: []json.RawMessage{json.RawMessage(`{"what":"logs"}`), json.RawMessage(`{"what":"screenshot"}`)}, Handle: h.observeDispatcher.Handle},
			toolmodule.Spec{Name: "analyze", Summary: "Run analysis checks over DOM, links, accessibility, and audits",
				Examples: []json.RawMessage{json.RawMessage(`{"what":"dom","selector":"body","background":true}`)}, Handle: h.analyzeDispatcher.Handle},
			toolmodule.Spec{Name: "generate", Summary: "Generate artifacts (reproduction, csp, sarif, tests) from captured context",
				Examples: []json.RawMessage{json.RawMessage(`{"what":"reproduction","last_n":20}`)}, Handle: h.generateDispatcher.Handle},
			toolmodule.Spec{Name: "configure", Summary: "Session settings, diagnostics, and recording utilities",
				Examples: []json.RawMessage{json.RawMessage(`{"what":"health"}`), json.RawMessage(`{"what":"clear","buffer":"logs"}`)}, Handle: h.toolConfigure},
			toolmodule.Spec{Name: "interact", Summary: "Browser automation: navigate, click, type, fill forms, take screenshots, and control any web page",
				Examples: []json.RawMessage{json.RawMessage(`{"what":"navigate","url":"https://example.com"}`), json.RawMessage(`{"what":"click","selector":"button.submit"}`), json.RawMessage(`{"what":"type","selector":"input[name=search]","text":"hello"}`)}, Handle: h.toolInteract},
		)
	})
}

// extractWhatParam extracts the "what" string from raw JSON args.
// Returns empty string if missing or unparseable.
func extractWhatParam(args json.RawMessage) string {
	if len(args) == 0 {
		return ""
	}
	var parsed struct {
		What string `json:"what"`
	}
	if json.Unmarshal(args, &parsed) != nil {
		return ""
	}
	return parsed.What
}

// usageKey builds the analytics key from tool args.
// For command_result calls, extracts the original command prefix from correlation_id
// (e.g. "nav_17083_123" → "command_result:nav") so analytics map back to the original action.
// For all other calls, returns the "what" param as-is.
func usageKey(args json.RawMessage) string {
	if len(args) == 0 {
		return ""
	}
	var parsed struct {
		What          string `json:"what"`
		CorrelationID string `json:"correlation_id"`
	}
	if json.Unmarshal(args, &parsed) != nil {
		return ""
	}
	if parsed.What != "command_result" {
		return parsed.What
	}
	// Extract the command prefix from correlation_id (format: prefix_timestamp_random).
	if parsed.CorrelationID == "" {
		return "command_result"
	}
	prefix := parsed.CorrelationID
	if idx := strings.IndexByte(prefix, '_'); idx > 0 {
		prefix = prefix[:idx]
	}
	return "command_result:" + prefix
}

func (h *ToolHandler) ensureToolSchemas() {
	h.toolSchemasOnce.Do(func() {
		h.toolSchemas = make(map[string]map[string]any)
		for _, tool := range h.ToolsList() {
			h.toolSchemas[tool.Name] = tool.InputSchema
		}
	})
}

func parseToolResultForPostProcessing(raw json.RawMessage) (*mcp.MCPToolResult, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, false
	}
	return &result, true
}

func isToolResultError(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return false
	}
	return result.IsError
}

// Close cancels readiness gates and other in-flight work owned by this handler.
func (h *ToolHandler) Close() {
	if h.shutdownCancel != nil {
		h.shutdownCancel()
	}
}

func (h *ToolHandler) GetCapture() *capture.Capture {
	return h.capture
}

func (h *ToolHandler) GetLogEntries() ([]types.LogEntry, []time.Time) {
	return h.server.logs.EntriesWithAddedAt()
}

func (h *ToolHandler) GetLogTotalAdded() int64 {
	return h.server.logs.TotalAdded()
}

func (h *ToolHandler) ToolsList() []mcp.MCPTool {
	return schema.AllTools()
}

func (h *ToolHandler) summaryPreference() *summarypref.Cache {
	if h.summaryPrefs == nil {
		return summarypref.New(nil)
	}
	return h.summaryPrefs
}

func (h *ToolHandler) loadSummaryPref() bool {
	return h.summaryPreference().Enabled()
}

func (h *ToolHandler) invalidateSummaryPref() {
	h.summaryPreference().Invalidate()
}

func (h *ToolHandler) maybeInjectSummary(args json.RawMessage) json.RawMessage {
	return h.summaryPreference().Inject(args)
}

func (h *ToolHandler) armEvidenceForCommand(correlationID, action string, args json.RawMessage, clientID string) {
	h.interactAction().ArmEvidenceForCommand(correlationID, action, args, clientID)
}

func (h *ToolHandler) getCommandResult(correlationID string) (*queries.CommandResult, bool) {
	return h.capture.Queries().GetCommandResult(correlationID)
}

func (h *ToolHandler) IsExtensionConnected() bool {
	return h.capture.Extension().IsExtensionConnected()
}

func (h *ToolHandler) PushInbox() *push.PushInbox {
	return h.server.pushInbox
}

func (h *ToolHandler) GetAnnotationStore() *annotation.Store {
	return h.annotationStore
}

func (h *ToolHandler) GetVersion() string {
	return version
}

func (h *ToolHandler) GetToolCallLimiter() RateLimiter {
	return h.toolCallLimiter
}

func (h *ToolHandler) GetRedactionEngine() RedactionEngine {
	return h.redactionEngine
}

func (h *ToolHandler) testGen() *testgenhandler.Handler {
	return h.testGenHandler
}

const (
	defaultColdStartTimeout       = 5 * time.Second
	testExtensionReadinessTimeout = time.Millisecond
)

func defaultExtensionReadinessTimeout() time.Duration {
	if strings.HasSuffix(os.Args[0], ".test") {
		return testExtensionReadinessTimeout
	}
	return capture.ExtensionReadinessTimeout
}

func sessionStoreGuard(store *persistence.SessionStore, req mcp.JSONRPCRequest) (mcp.JSONRPCResponse, bool) {
	if store != nil {
		return mcp.JSONRPCResponse{}, false
	}
	return mcp.Fail(req, mcp.ErrNotInitialized, "Session store not initialized", "Internal error — do not retry"), true
}

// NewToolHandler constructs the composite five-tool backend and its MCP adapter.
func NewToolHandler(server *Server, captureStore *capture.Capture) *MCPHandler {
	shutdownContext, shutdownCancel := context.WithCancel(context.Background())
	handler := &ToolHandler{
		MCPHandler:       NewMCPHandler(server, version),
		capture:          captureStore,
		shutdownCtx:      shutdownContext,
		shutdownCancel:   shutdownCancel,
		coldStartTimeout: defaultColdStartTimeout,
		networkRecording: &netrecord.NetworkRecordingState{},
	}
	handler.Guards = toolguard.New(captureStore, shutdownContext, defaultExtensionReadinessTimeout())
	var recordingStore toolrecording.Store
	if captureStore != nil {
		recordingStore = captureStore.Recordings()
	}
	handler.recordingHandler = toolrecording.NewHandler(recordingStore, handler.appendServerLog)
	handler.usageTracker = telemetry.NewUsageTracker()
	if captureStore != nil {
		tracker := handler.usageTracker
		captureStore.SetFeaturesCallback(func(features map[string]bool) {
			for key, used := range features {
				if used {
					tracker.RecordToolCall("ext:"+key, 0, false)
				}
			}
		})
	}

	handler.healthMetrics = health.NewMetrics()
	handler.toolCallLimiter = toolresp.NewToolCallLimiter(500, time.Minute)
	handler.alertBuffer = alertbuf.NewAlertBuffer()

	if currentDirectory, err := os.Getwd(); err == nil {
		if store, storeErr := persistence.NewSessionStore(currentDirectory); storeErr == nil {
			handler.sessionStoreImpl = store
		}
	}
	handler.summaryPrefs = summarypref.New(func() ([]byte, error) {
		if handler.sessionStoreImpl == nil {
			return nil, nil
		}
		return handler.sessionStoreImpl.Load("session", "response_mode")
	})
	if handler.sessionStoreImpl != nil {
		handler.noiseConfig = noise.NewNoiseConfigWithStore(handler.sessionStoreImpl)
	} else {
		handler.noiseConfig = noise.NewNoiseConfig()
	}
	handler.redactionEngine = redaction.NewRedactionEngine("")

	handler.annotationStore = server.getAnnotationStore()
	handler.annotationAnalysis = annotationanalysis.New(
		handler.annotationStore,
		handler.capture,
		handler.formatCommandResult,
		handler.server.logs.Entries,
	)
	if handler.capture != nil {
		handler.annotationStore.SetCommandCompleter(func(correlationID string, result json.RawMessage) {
			handler.capture.Queries().ApplyCommandResult(correlationID, "complete", result, "")
		})
	}
	detectNoise := func() {
		noiseautorun.Detect(handler.noiseConfig, handler.capture, handler.server.logs.Entries())
	}
	noiseautorun.WireNavigation(handler.capture, detectNoise)
	noiseautorun.WireFirstConnect(handler.capture, handler.shutdownCtx.Done(), func() {
		if handler.noiseFirstConnectFn != nil {
			handler.noiseFirstConnectFn()
			return
		}
		detectNoise()
		diag.Printf("[Kaboom] noise auto-detect: ran on first extension connection\n")
	})

	handler.securityScannerImpl = scan.NewScanner()
	handler.thirdPartyAuditorImpl = thirdparty.NewThirdPartyAuditor()
	handler.apiContractRuntime = apicontract.NewRuntime()
	var performanceEntries func() []performance.PerformanceSnapshot
	var queryStore toolobserve.CommandStore
	var inProgress func() []capture.SyncInProgress
	var captureReader session.RuntimeCaptureReader
	if handler.capture != nil {
		performanceEntries = handler.capture.Performance().Entries
		queryStore = handler.capture.Queries()
		inProgress = handler.capture.Extension().GetInProgressCommands
		captureReader = handler.capture
	}
	handler.sessionManager = session.NewSessionManager(
		10,
		session.NewRuntimeStateReader(
			handler.server.logs.Entries,
			performanceEntries,
			captureReader,
		),
	)
	handler.auditTrail = audit.NewAuditTrail(audit.AuditConfig{
		MaxEntries:   10000,
		Enabled:      true,
		RedactParams: true,
	})
	handler.auditRecorder = audit.NewRecorder(handler.auditTrail)

	handler.uploadSecurity = uploadSecurityConfig
	handler.recordingInteractHandler = screenrec.NewInteractHandler(handler.screenrecDeps())
	handler.analyzeDispatcher = analyzedispatch.NewDispatcher(analyzedispatch.Config{
		Host: handler, Version: version, AnnotationStore: handler.annotationStore,
		Visual: visualAnalyzeDeps{h: handler},
		ValidateAPI: func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handler.apiContractRuntime.Handle(req, args, handler.capture.GetNetworkBodies())
		},
		PageSummary: func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handler.interactAction().HandleContentExtraction(req, args, "page_summary", "page_summary")
		},
		Annotations: handler.annotationAnalysis.GetAnnotations, AnnotationDetail: handler.annotationAnalysis.GetAnnotationDetail,
		FeatureGates: func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handler.interactAction().HandleContentExtraction(req, args, "feature_gates", "feature_gates")
		},
	})
	handler.testGenHandler = testgenhandler.New(handler)
	handler.generateDispatcher = toolgenerate.NewDispatcher(handler, handler.testGenHandler)
	interactDeps := buildInteractDeps(handler)
	handler.interactActionHandler = toolinteract.NewInteractActionHandler(interactDeps)
	handler.uploadInteractHandler = toolinteract.NewUploadInteractHandler(interactDeps, handler.interactActionHandler)
	handler.observeDispatcher = toolobserve.NewDispatcher(toolobserve.Config{
		Host: handler, Commands: queryStore, InProgress: inProgress,
		AnnotationStore: handler.annotationStore,
		Annotations: func(_ toolobserve.Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handler.annotationAnalysis.GetAnnotations(req, args)
		},
		AnnotationDetail: func(_ toolobserve.Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handler.annotationAnalysis.GetAnnotationDetail(req, args)
		},
		Recordings: func(_ toolobserve.Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handler.recordingHandler.Recordings(req, args)
		},
		RecordingActions: func(_ toolobserve.Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handler.recordingHandler.RecordingActions(req, args)
		},
		PlaybackResults: func(_ toolobserve.Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handler.recordingHandler.PlaybackResults(req, args)
		},
		LogDiffReport: func(_ toolobserve.Host, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handler.recordingHandler.LogDiffReport(req, args)
		},
		FormatCommand: handler.formatCommandResult, InjectSummary: handler.maybeInjectSummary,
		DrainAlerts: handler.alertBuffer.DrainAlerts, DiagnosticHint: handler.Guards.DiagnosticHint(),
	})
	handler.stateInteractHandler = toolinteract.NewStateInteractHandler(interactDeps, handler.sessionStoreImpl)
	handler.configureSessions = toolconfigure.NewSessionHandler(toolconfigure.SessionDeps{
		RequireStore: func(req mcp.JSONRPCRequest) (mcp.JSONRPCResponse, bool) {
			return sessionStoreGuard(handler.sessionStoreImpl, req)
		},
		InvalidateSummary: handler.invalidateSummaryPref,
		SetActiveCodebase: handler.MCPHandler.server.SetActiveCodebase,
	},
		handler.sessionStoreImpl,
		handler.sessionManager,
	)
	handler.ensureToolModules()
	handler.ensureToolSchemas()

	return &MCPHandler{server: server, toolHandler: handler}
}

type visualAnalyzeDeps struct{ h *ToolHandler }

func (d visualAnalyzeDeps) CaptureScreenshot(req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
	return observe.GetScreenshot(d.h, req, json.RawMessage(`{}`))
}

func (d visualAnalyzeDeps) GetTrackingStatus() (bool, int, string) {
	return d.h.capture.Extension().GetTrackingStatus()
}

func (d visualAnalyzeDeps) HasSessionStore() bool { return d.h.sessionStoreImpl != nil }

func (d visualAnalyzeDeps) HandleSessionStore(args persistence.SessionStoreArgs) (json.RawMessage, error) {
	return d.h.sessionStoreImpl.HandleSessionStore(args)
}

func (h *ToolHandler) NetworkWaterfallEntries() []types.NetworkWaterfallEntry {
	return h.capture.NetworkWaterfall().Entries()
}

func (h *ToolHandler) ConsoleSecurityEntries() []types.LogEntry {
	snapshot := h.server.logs.Entries()
	entries := make([]types.LogEntry, len(snapshot))
	for index, entry := range snapshot {
		entries[index] = types.LogEntry(entry)
	}
	return entries
}

func (h *ToolHandler) SecurityScanner() toolanalyze.SecurityScannerInterface {
	if h.securityScannerImpl == nil {
		return nil
	}
	return h.securityScannerImpl
}

func (h *ToolHandler) LogEntries() []types.LogEntry {
	entries, _ := h.GetLogEntries()
	return entries
}

// buildInteractDeps is the composition boundary between ToolHandler and the
// canonical interact owner. All cross-feature dependencies are wired here.
func buildInteractDeps(h *ToolHandler) *toolinteract.Deps {
	if h.Guards == nil {
		h.Guards = toolguard.New(h.capture, h.shutdownCtx, defaultExtensionReadinessTimeout())
	}
	var getCommandResult func(string) (*queries.CommandResult, bool)
	if h.capture != nil {
		getCommandResult = h.capture.Queries().GetCommandResult
	}
	return &toolinteract.Deps{
		RequirePilot: h.Guards.RequirePilot, RequireExtension: h.Guards.RequireExtension,
		RequireTabTracking: h.Guards.RequireTabTracking, RequireCSPClear: h.Guards.RequireCSPClear,
		EnqueuePendingQuery: h.EnqueuePendingQuery, MaybeWaitForCommand: h.MaybeWaitForCommand,
		Capture:        func() *capture.Capture { return h.capture },
		RecordAIAction: h.recordAIAction, RecordAIEnhancedAction: h.recordAIEnhancedAction,
		RecordDOMPrimitiveAction: h.recordDOMPrimitiveAction,
		ToolInteract:             h.toolInteract, ToolAnalyze: h.analyzeDispatcher.Handle,
		ToolExportSARIF:         h.generateDispatcher.ExportSARIF,
		EnrichNavigateResponse:  h.enrichNavigateResponse,
		InjectCSPBlockedActions: h.Guards.InjectCSPBlockedActions,
		GetScreenshot: func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return observe.GetScreenshot(h, req, args)
		},
		GetPageInfo: func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return observe.GetPageInfo(h, req, args)
		},
		MarkDrawStarted: func() {
			if h.annotationStore != nil {
				h.annotationStore.MarkDrawStarted()
			}
		},
		GetListenPort: func() int {
			if h.server != nil {
				return h.server.getListenPort()
			}
			return defaultPort
		},
		DefaultEvidenceCapture: func(clientID string) toolinteract.EvidenceShot {
			return toolinteract.CaptureEvidence(h.capture, clientID)
		},
		RequireSessionStore: func(req mcp.JSONRPCRequest) (mcp.JSONRPCResponse, bool) {
			return sessionStoreGuard(h.sessionStoreImpl, req)
		},
		DiagnosticHint: h.Guards.DiagnosticHint,
		GetRedactionEngine: func() toolinteract.RedactionEngine {
			return h.GetRedactionEngine()
		},
		GetCommandResult: getCommandResult,
		ReplayMu:         &replayMu,
	}
}

func (h *ToolHandler) interactAction() *toolinteract.InteractActionHandler {
	return h.interactActionHandler
}

func (h *ToolHandler) stateInteract() *interactstate.Handler {
	return h.stateInteractHandler
}

func (h *ToolHandler) screenrecDeps() screenrec.Deps {
	if h.Guards == nil {
		h.Guards = toolguard.New(h.capture, h.shutdownCtx, defaultExtensionReadinessTimeout())
	}
	return screenrec.Deps{
		EnqueuePendingQuery: h.EnqueuePendingQuery,
		RequirePilot:        h.Guards.RequirePilot,
		RequireExtension:    h.Guards.RequireExtension,
		RecordAIAction:      h.recordAIAction,
		DiagnosticHint:      h.Guards.DiagnosticHint,
		GetCommandResult:    h.getCommandResult,
	}
}

func (h *ToolHandler) buildPlaybackResult(request mcp.JSONRPCRequest, recordingID string, session *playback.Session) mcp.JSONRPCResponse {
	return toolrecording.BuildPlaybackResult(request, recordingID, session)
}

func (h *ToolHandler) appendServerLog(entry types.LogEntry) {
	h.server.logs.AddEntries([]types.LogEntry{entry})
}
