// Purpose: Defines the ToolHandler struct, shared state (capture, AI client, sequence store), and tool dispatch infrastructure.
// Why: All five MCP tools share a common handler that owns capture state, extension connectivity, and session context.

package main

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/screenrec"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/summarypref"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/testgenhandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/netrecord"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract/interactstate"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract/interactupload"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolrecording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/analysis/apicontract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/analysis/thirdparty"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/audit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/issuereport"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/scan"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/streaming/alertbuf"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

// Note: Response helpers, error codes, and validation functions have been moved to:
// - tools_response.go — Response formatting helpers
// - tools_errors.go — Error codes and structured error handling
// - tools_validation.go — Parameter validation utilities

// ============================================
// ToolHandler Definition
// ============================================

// ToolHandler extends MCPHandler with composite tool dispatch
type ToolHandler struct {
	*MCPHandler
	capture *capture.Store

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
	toolCallLimiter *ToolCallLimiter

	// Alert system + context streaming (delegates to internal/streaming)
	alertBuffer *alertbuf.AlertBuffer

	// Concrete implementations (interface signatures differ from types package)
	// These are used directly by tool handlers rather than through the interface fields above.
	noiseConfig           *noise.NoiseConfig
	sessionStoreImpl      *persistence.SessionStore
	securityScannerImpl   *scan.Scanner
	thirdPartyAuditorImpl *thirdparty.ThirdPartyAuditor
	sessionManager        *session.SessionManager
	auditTrail            *audit.Trail

	// Per-client audit session mapping (client_id -> session_id).
	auditMu         sync.Mutex
	auditSessionMap map[string]string

	// Draw mode annotation store (in-memory, TTL-based)
	annotationStore *AnnotationStore

	// API contract validation state (incremental over captured network bodies).
	apiContractRuntime *apicontract.Runtime

	// Upload security config (folder-scoped permissions + denylist)
	uploadSecurity *UploadSecurity

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
	stateInteractHandler     *interactstate.Handler
	configureSessionHandler  *configureSessionHandler

	// Passive network traffic recording state (start/stop capture).
	networkRecording *netrecord.NetworkRecordingState

	// Module registry for plugin-style tool dispatch (incremental migration).
	toolModulesOnce sync.Once
	toolModules     *toolModuleRegistry

	// Tool schema cache for parameter-warning validation.
	toolSchemasOnce sync.Once
	toolSchemas     map[string]map[string]any

	// Session-level response-mode preference.
	summaryPrefs *summarypref.Cache

	// extensionReadinessTimeout overrides the cold-start wait duration for requireExtension.
	// Zero uses capture.ExtensionReadinessTimeout (5s). Tests set this to 100ms.
	extensionReadinessTimeout time.Duration

	// noiseFirstConnectFn overrides the noise auto-detect function for first-connection.
	// When nil, runNoiseAutoDetect() is used. Set in tests to inject counting stubs.
	noiseFirstConnectFn func()

	// issueCommandRunner overrides the exec runner for issue submission.
	// When nil, issuereport.ExecRunner{} is used. Set in tests to inject a fake.
	issueCommandRunner issuereport.CommandRunner

	// usageCounter tracks tool:action call counts for periodic usage beacons.
	// When nil, usage counting is disabled (backwards compatible).
	usageTracker *telemetry.UsageTracker
}

// maybeWaitForCommand, formatCommandResult, and related async infrastructure
// live in tools_async_completion.go; the result shaping
// they call lives in internal/asyncresult.

// handleToolCall dispatches composite tool calls by mode parameter.
func (h *ToolHandler) HandleToolCall(req JSONRPCRequest, name string, args json.RawMessage) (JSONRPCResponse, bool) {
	start := time.Now()

	h.ensureToolModules()
	h.ensureToolSchemas()
	resp, handled := h.dispatchViaModules(req, name, args)
	if !handled {
		return JSONRPCResponse{}, false
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
					resp.Result = safeMarshal(parsedResult, string(resp.Result))
				} else {
					resp = appendWarningsToResponse(resp, warnings)
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
	resp = h.appendPushPiggyback(resp)

	h.recordAuditToolCall(req, name, args, resp, start)

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
		h.toolModules = h.buildToolModuleRegistry()
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

func parseToolResultForPostProcessing(raw json.RawMessage) (*MCPToolResult, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var result MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, false
	}
	return &result, true
}

func isToolResultError(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var result MCPToolResult
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

func (h *ToolHandler) GetCapture() *capture.Store {
	return h.capture
}

func (h *ToolHandler) GetLogEntries() ([]LogEntry, []time.Time) {
	return h.server.logs.EntriesWithAddedAt()
}

func (h *ToolHandler) GetLogTotalAdded() int64 {
	return h.server.logs.TotalAdded()
}

func (h *ToolHandler) ToolsList() []MCPTool {
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
	return h.capture.GetCommandResult(correlationID)
}

func (h *ToolHandler) IsExtensionConnected() bool {
	return h.capture.IsExtensionConnected()
}

func (h *ToolHandler) PushInbox() *push.PushInbox {
	return h.server.pushInbox
}

func (h *ToolHandler) GetAnnotationStore() *AnnotationStore {
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
