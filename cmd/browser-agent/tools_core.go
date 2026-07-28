// Purpose: Defines the ToolHandler struct, shared state (capture, AI client, sequence store), and tool dispatch infrastructure.
// Why: All five MCP tools share a common handler that owns capture state, extension connectivity, and session context.

package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/asynccommand"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/noiseautorun"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/screenrec"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/sequencehandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/summarypref"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/testgenhandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/analyzedispatch"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/annotationanalysis"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/combinedaudit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/inspect"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolcatalog"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/netrecord"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/tutorial"
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
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording/actionlog"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/redaction"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/scan"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/streaming/alertbuf"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	cfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/configure"
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

	recordingInteractHandler *screenrec.InteractHandler
	recordingHandler         *toolrecording.Handler
	uploadInteractHandler    *interactupload.Handler
	testGenHandler           *testgenhandler.Handler
	generateDispatcher       *toolgenerate.Dispatcher
	observeDispatcher        *toolobserve.Dispatcher
	stateInteractHandler     *interactstate.Handler
	configureLocalDeps       toolconfigure.Deps
	configureDispatcher      *toolconfigure.Dispatcher
	tutorialDeps             *tutorial.Deps
	issueReportDeps          issuereport.HandlerDeps
	configureSessions        *toolconfigure.SessionHandler
	testBoundaries           *cfg.BoundaryHandler
	sequences                *sequencehandler.Handler
	annotationAnalysis       *annotationanalysis.Handler
	analyzeDispatcher        *analyzedispatch.Dispatcher
	asyncCommands            *asynccommand.Handler
	actionRecorder           *actionlog.Recorder

	// Passive network traffic recording state (start/stop capture).
	networkRecording *netrecord.NetworkRecordingState

	// Immutable executable modules, examples, and validation schemas.
	toolCatalog *toolcatalog.Catalog

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

// handleToolCall dispatches composite tool calls by mode parameter.
func (h *ToolHandler) HandleToolCall(req mcp.JSONRPCRequest, name string, args json.RawMessage) (mcp.JSONRPCResponse, bool) {
	start := time.Now()

	resp, handled := h.toolCatalog.Dispatch(req, name, args)
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
		if inputSchema := h.toolCatalog.Schema(name); inputSchema != nil {
			if warnings := mcp.ValidateParamsAgainstSchema(args, inputSchema); len(warnings) > 0 {
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
	resp = toolobserve.AppendPushPiggyback(buildObserveLocalDeps(h), resp)

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

func buildToolCatalog(h *ToolHandler) *toolcatalog.Catalog {
	return toolcatalog.New(
		[]toolmodule.Spec{
			toolmodule.Spec{Name: "observe", Summary: "Read captured browser state: logs, network, screenshots, and async results",
				Examples: []json.RawMessage{json.RawMessage(`{"what":"logs"}`), json.RawMessage(`{"what":"screenshot"}`)}, Handle: h.observeDispatcher.Handle},
			toolmodule.Spec{Name: "analyze", Summary: "Run analysis checks over DOM, links, accessibility, and audits",
				Examples: []json.RawMessage{json.RawMessage(`{"what":"dom","selector":"body","background":true}`)}, Handle: h.analyzeDispatcher.Handle},
			toolmodule.Spec{Name: "generate", Summary: "Generate artifacts (reproduction, csp, sarif, tests) from captured context",
				Examples: []json.RawMessage{json.RawMessage(`{"what":"reproduction","last_n":20}`)}, Handle: h.generateDispatcher.Handle},
			toolmodule.Spec{Name: "configure", Summary: "Session settings, diagnostics, and recording utilities",
				Examples: []json.RawMessage{json.RawMessage(`{"what":"health"}`), json.RawMessage(`{"what":"clear","buffer":"logs"}`)}, Handle: h.configureDispatcher.Handle},
			toolmodule.Spec{Name: "interact", Summary: "Browser automation: navigate, click, type, fill forms, take screenshots, and control any web page",
				Examples: []json.RawMessage{json.RawMessage(`{"what":"navigate","url":"https://example.com"}`), json.RawMessage(`{"what":"click","selector":"button.submit"}`), json.RawMessage(`{"what":"type","selector":"input[name=search]","text":"hello"}`)}, Handle: h.toolInteract},
		},
		schema.AllTools(),
	)
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
		testBoundaries:   cfg.NewBoundaryHandler(),
	}
	handler.Guards = toolguard.New(captureStore, shutdownContext, defaultExtensionReadinessTimeout())
	var recordingStore toolrecording.Store
	if captureStore != nil {
		recordingStore = captureStore.Recordings()
	}
	handler.recordingHandler = toolrecording.NewHandler(recordingStore, func(entry types.LogEntry) {
		handler.server.logs.AddEntries([]types.LogEntry{entry})
	})
	handler.usageTracker = telemetry.NewUsageTracker()
	if captureStore != nil {
		tracker := handler.usageTracker
		captureStore.FeatureUsage().SetCallback(func(features map[string]bool) {
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
	var actionTelemetry *capture.TelemetryStore
	if captureStore != nil {
		actionTelemetry = captureStore.Telemetry()
	}
	handler.actionRecorder = actionlog.New(actionTelemetry)
	handler.asyncCommands = asynccommand.New(asynccommand.Deps{
		Capture:              handler.capture,
		DiagnosticHint:       handler.Guards.DiagnosticHint(),
		DiagnosticHintString: handler.Guards.DiagnosticHintString,
		AttachEvidence: func(correlationID string, responseData map[string]any) {
			handler.interactActionHandler.AttachEvidencePayload(correlationID, responseData)
		},
		AttachRetryContext: func(correlationID string, responseData map[string]any, status, reason string) {
			handler.interactActionHandler.AttachRetryContext(correlationID, responseData, status, reason)
		},
		SummaryEnabled:     handler.summaryPrefs.Enabled,
		RecordAsyncOutcome: handler.usageTracker.RecordAsyncOutcome,
	})

	handler.annotationStore = server.getAnnotationStore()
	handler.annotationAnalysis = annotationanalysis.New(
		handler.annotationStore,
		handler.capture,
		handler.asyncCommands.FormatCommandResult,
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
	handler.recordingInteractHandler = screenrec.NewInteractHandler(buildScreenrecDeps(handler))
	analyzeDeps := buildAnalyzeDeps(handler)
	observeDeps := buildObserveReadDeps(handler)
	handler.analyzeDispatcher = analyzedispatch.NewDispatcher(analyzedispatch.Config{
		Analyze: analyzeDeps,
		Inspect: buildInspectDeps(handler),
		Observe: observeDeps,
		Audit:   combinedaudit.Deps{Analyze: analyzeDeps, Observe: observeDeps},
		Version: version, AnnotationStore: handler.annotationStore, Visual: visualAnalyzeDeps{h: handler},
		ValidateAPI: func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handler.apiContractRuntime.Handle(req, args, handler.capture.Telemetry().GetNetworkBodies())
		},
		PageSummary: func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handler.interactActionHandler.HandleContentExtraction(req, args, "page_summary", "page_summary")
		},
		Annotations: handler.annotationAnalysis.GetAnnotations, AnnotationDetail: handler.annotationAnalysis.GetAnnotationDetail,
		FeatureGates: func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handler.interactActionHandler.HandleContentExtraction(req, args, "feature_gates", "feature_gates")
		},
	})
	handler.testGenHandler = testgenhandler.New(buildTestGenerationDeps(handler))
	handler.generateDispatcher = toolgenerate.NewDispatcher(buildGenerateDeps(handler), handler.testGenHandler)
	interactDeps := buildInteractDeps(handler)
	handler.interactActionHandler = toolinteract.NewInteractActionHandler(interactDeps)
	handler.configureLocalDeps = buildConfigureLocalDeps(handler)
	handler.tutorialDeps = buildTutorialDeps(handler)
	handler.issueReportDeps = buildIssueReportDeps(handler)
	handler.uploadInteractHandler = toolinteract.NewUploadInteractHandler(interactDeps, handler.interactActionHandler)
	handler.observeDispatcher = toolobserve.NewDispatcher(toolobserve.Config{
		Observe: observeDeps, Local: buildObserveLocalDeps(handler),
		IsExtensionConnected: func() bool { return handler.capture.Extension().IsExtensionConnected() },
		Commands:             queryStore, InProgress: inProgress,
		AnnotationStore: handler.annotationStore,
		Annotations: func(_ observe.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handler.annotationAnalysis.GetAnnotations(req, args)
		},
		AnnotationDetail: func(_ observe.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handler.annotationAnalysis.GetAnnotationDetail(req, args)
		},
		Recordings: func(_ observe.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handler.recordingHandler.Recordings(req, args)
		},
		RecordingActions: func(_ observe.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handler.recordingHandler.RecordingActions(req, args)
		},
		PlaybackResults: func(_ observe.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handler.recordingHandler.PlaybackResults(req, args)
		},
		LogDiffReport: func(_ observe.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handler.recordingHandler.LogDiffReport(req, args)
		},
		FormatCommand: handler.asyncCommands.FormatCommandResult, InjectSummary: handler.summaryPrefs.Inject,
		DrainAlerts: handler.alertBuffer.DrainAlerts, DiagnosticHint: handler.Guards.DiagnosticHint(),
	})
	handler.stateInteractHandler = toolinteract.NewStateInteractHandler(interactDeps, handler.sessionStoreImpl)
	handler.configureSessions = toolconfigure.NewSessionHandler(toolconfigure.SessionDeps{
		RequireStore: func(req mcp.JSONRPCRequest) (mcp.JSONRPCResponse, bool) {
			return sessionStoreGuard(handler.sessionStoreImpl, req)
		},
		InvalidateSummary: handler.summaryPrefs.Invalidate,
		SetActiveCodebase: handler.MCPHandler.server.SetActiveCodebase,
	},
		handler.sessionStoreImpl,
		handler.sessionManager,
	)
	var waitForSequenceCommand func(string, time.Duration) (*queries.CommandResult, bool)
	if captureStore != nil {
		waitForSequenceCommand = captureStore.Queries().WaitForCommand
	}
	handler.sequences = sequencehandler.New(sequencehandler.Deps{
		Store:          handler.sessionStoreImpl,
		ReplayMu:       &replayMu,
		Interact:       handler.toolInteract,
		WaitForCommand: waitForSequenceCommand,
		RecordAction:   handler.actionRecorder.Record,
	})
	handler.configureDispatcher = buildConfigureDispatcher(handler)
	handler.toolCatalog = buildToolCatalog(handler)

	handler.MCPHandler.SetToolBackend(buildMCPToolBackend(handler))
	return handler.MCPHandler
}

type visualAnalyzeDeps struct{ h *ToolHandler }

func buildMCPToolBackend(handler *ToolHandler) ToolBackend {
	return ToolBackend{
		Executor: handler, Capture: handler.capture,
		Limiter: handler.toolCallLimiter, Redactor: handler.redactionEngine,
		Schemas: schema.AllTools(), UsageTracker: handler.usageTracker,
	}
}

func (d visualAnalyzeDeps) CaptureScreenshot(req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
	return observe.GetScreenshot(buildObserveReadDeps(d.h), req, json.RawMessage(`{}`))
}

func (d visualAnalyzeDeps) GetTrackingStatus() (bool, int, string) {
	return d.h.capture.Extension().GetTrackingStatus()
}

func (d visualAnalyzeDeps) HasSessionStore() bool { return d.h.sessionStoreImpl != nil }

func (d visualAnalyzeDeps) HandleSessionStore(args persistence.SessionStoreArgs) (json.RawMessage, error) {
	return d.h.sessionStoreImpl.HandleSessionStore(args)
}

func buildGenerateDeps(h *ToolHandler) toolgenerate.Deps {
	return toolgenerate.Deps{
		Capture:          h.capture,
		AnnotationStore:  h.annotationStore,
		Version:          version,
		ExecuteA11yQuery: h.asyncCommands.ExecuteA11yQuery,
		IsExtensionConnected: func() bool {
			return h.capture.Extension().IsExtensionConnected()
		},
	}
}

func buildTestGenerationDeps(h *ToolHandler) testgenhandler.Deps {
	return testgenhandler.Deps{
		LogEntries: func() []types.LogEntry {
			entries, _ := h.server.logs.EntriesWithAddedAt()
			return entries
		},
		EnhancedActions: func() []types.EnhancedAction {
			if h.capture == nil {
				return nil
			}
			return h.capture.Telemetry().GetAllEnhancedActions()
		},
		NetworkBodies: func() []types.NetworkBody {
			if h.capture == nil {
				return nil
			}
			return h.capture.Telemetry().GetNetworkBodies()
		},
	}
}

func buildAnalyzeDeps(h *ToolHandler) toolanalyze.Deps {
	return toolanalyze.Deps{
		EnqueuePendingQuery: h.asyncCommands.EnqueuePendingQuery,
		MaybeWaitForCommand: h.asyncCommands.MaybeWaitForCommand,
		GetTrackingStatus: func() (bool, int, string) {
			if h.capture == nil {
				return false, 0, ""
			}
			return h.capture.Extension().GetTrackingStatus()
		},
		NetworkBodies: func() []types.NetworkBody {
			if h.capture == nil {
				return nil
			}
			return h.capture.Telemetry().GetNetworkBodies()
		},
		NetworkWaterfallEntries: func() []types.NetworkWaterfallEntry {
			if h.capture == nil {
				return nil
			}
			return h.capture.Telemetry().NetworkWaterfall().Entries()
		},
		ConsoleSecurityEntries: func() []types.LogEntry {
			snapshot := h.server.logs.Entries()
			entries := make([]types.LogEntry, len(snapshot))
			for index, entry := range snapshot {
				entries[index] = types.LogEntry(entry)
			}
			return entries
		},
		SecurityScanner: func() toolanalyze.SecurityScannerInterface {
			if h.securityScannerImpl == nil {
				return nil
			}
			return h.securityScannerImpl
		},
		LogEntries: func() []types.LogEntry {
			entries, _ := h.server.logs.EntriesWithAddedAt()
			return entries
		},
		ExecuteA11yQuery: h.asyncCommands.ExecuteA11yQuery,
	}
}

func buildInspectDeps(h *ToolHandler) inspect.Deps {
	return inspect.Deps{
		EnqueuePendingQuery: h.asyncCommands.EnqueuePendingQuery,
		MaybeWaitForCommand: h.asyncCommands.MaybeWaitForCommand,
	}
}

func buildObserveLocalDeps(h *ToolHandler) toolobserve.Deps {
	return toolobserve.Deps{
		Inbox:               h.server.pushInbox,
		EnqueuePendingQuery: h.asyncCommands.EnqueuePendingQuery,
		MaybeWaitForCommand: h.asyncCommands.MaybeWaitForCommand,
	}
}

func buildObserveReadDeps(h *ToolHandler) observe.Deps {
	return observe.Deps{
		Capture: h.capture,
		LogEntries: func() ([]types.LogEntry, []time.Time) {
			return h.server.logs.EntriesWithAddedAt()
		},
		LogTotalAdded:    h.server.logs.TotalAdded,
		ExecuteA11yQuery: h.asyncCommands.ExecuteA11yQuery,
		IsConsoleNoise: func(entry types.LogEntry) bool {
			if h.noiseConfig == nil {
				return false
			}
			return h.noiseConfig.IsConsoleNoise(entry)
		},
		DiagnosticHintString: h.Guards.DiagnosticHintString,
	}
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
		EnqueuePendingQuery: h.asyncCommands.EnqueuePendingQuery, MaybeWaitForCommand: h.asyncCommands.MaybeWaitForCommand,
		Capture:        func() *capture.Capture { return h.capture },
		RecordAIAction: h.actionRecorder.Record, RecordAIEnhancedAction: h.actionRecorder.RecordEnhanced,
		RecordDOMPrimitiveAction: h.actionRecorder.RecordDOMPrimitive,
		ToolInteract:             h.toolInteract, ToolAnalyze: h.analyzeDispatcher.Handle,
		ToolExportSARIF:         h.generateDispatcher.ExportSARIF,
		InjectCSPBlockedActions: h.Guards.InjectCSPBlockedActions,
		GetScreenshot: func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return observe.GetScreenshot(buildObserveReadDeps(h), req, args)
		},
		GetPageInfo: func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return observe.GetPageInfo(buildObserveReadDeps(h), req, args)
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
			return h.redactionEngine
		},
		GetCommandResult: getCommandResult,
		ReplayMu:         &replayMu,
	}
}

func buildScreenrecDeps(h *ToolHandler) screenrec.Deps {
	if h.Guards == nil {
		h.Guards = toolguard.New(h.capture, h.shutdownCtx, defaultExtensionReadinessTimeout())
	}
	var getCommandResult func(string) (*queries.CommandResult, bool)
	if h.capture != nil {
		getCommandResult = h.capture.Queries().GetCommandResult
	}
	return screenrec.Deps{
		EnqueuePendingQuery: h.asyncCommands.EnqueuePendingQuery,
		RequirePilot:        h.Guards.RequirePilot,
		RequireExtension:    h.Guards.RequireExtension,
		RecordAIAction:      h.actionRecorder.Record,
		DiagnosticHint:      h.Guards.DiagnosticHint,
		GetCommandResult:    getCommandResult,
	}
}
