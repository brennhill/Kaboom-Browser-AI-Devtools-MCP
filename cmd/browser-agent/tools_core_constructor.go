// Purpose: ToolHandler constructor and startup-time defaults.
// Why: Isolates initialization policy from dispatch/type definitions.

package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/screenrec"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/testgenhandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/netrecord"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolrecording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/analysis/apicontract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/analysis/thirdparty"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/audit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/redaction"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/scan"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/streaming/alertbuf"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

// defaultColdStartTimeout is how long requireExtension waits for the extension
// to connect during a cold start before returning an error.
// This eliminates "no_data" failures when the LLM sends a command before the
// extension's first /sync heartbeat arrives.
// Note: MaybeWaitForCommand does only an instant IsExtensionConnected() check;
// the blocking wait is exclusively in requireExtension (P1-2: no double wait).
const defaultColdStartTimeout = 5 * time.Second

// testExtensionReadinessTimeout keeps extension-gate failures fast in unit tests.
// Production remains at capture.ExtensionReadinessTimeout (5s).
const testExtensionReadinessTimeout = 1 * time.Millisecond

func defaultExtensionReadinessTimeout() time.Duration {
	if strings.HasSuffix(os.Args[0], ".test") {
		return testExtensionReadinessTimeout
	}
	return capture.ExtensionReadinessTimeout
}

// NewToolHandler creates an MCP handler with composite tool capabilities.
func NewToolHandler(server *Server, capture *capture.Store) *MCPHandler {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	handler := &ToolHandler{
		MCPHandler:                NewMCPHandler(server, version),
		capture:                   capture,
		shutdownCtx:               shutdownCtx,
		shutdownCancel:            shutdownCancel,
		coldStartTimeout:          defaultColdStartTimeout,
		extensionReadinessTimeout: defaultExtensionReadinessTimeout(),
		networkRecording:          &netrecord.NetworkRecordingState{},
	}
	handler.recordingHandler = toolrecording.NewHandler(capture, handler.appendServerLog)

	// Initialize usage tracker for structured telemetry beacons.
	handler.usageTracker = telemetry.NewUsageTracker()

	// Wire extension UI feature flags into the tracker so they appear
	// in the aggregated usage_summary beacon alongside MCP tool stats.
	if capture != nil {
		tracker := handler.usageTracker
		capture.SetFeaturesCallback(func(features map[string]bool) {
			for key, used := range features {
				if used {
					tracker.RecordToolCall("ext:"+key, 0, false)
				}
			}
		})
	}

	// Initialize health metrics.
	handler.healthMetrics = health.NewMetrics()
	handler.toolCallLimiter = NewToolCallLimiter(500, time.Minute)
	handler.alertBuffer = alertbuf.NewAlertBuffer()

	// Initialize session store (use current working directory as project path).
	cwd, err := os.Getwd()
	if err == nil {
		if store, err := persistence.NewSessionStore(cwd); err == nil {
			handler.sessionStoreImpl = store
		}
	}

	// Initialize noise filtering with persistence support.
	if handler.sessionStoreImpl != nil {
		handler.noiseConfig = noise.NewNoiseConfigWithStore(handler.sessionStoreImpl)
	} else {
		handler.noiseConfig = noise.NewNoiseConfig()
	}
	handler.redactionEngine = redaction.NewRedactionEngine("")

	// Use server-scoped annotation store for draw mode.
	handler.annotationStore = server.getAnnotationStore()

	// Wire async annotation waiter → CommandTracker completion.
	if handler.capture != nil {
		handler.annotationStore.SetCommandCompleter(func(correlationID string, result json.RawMessage) {
			handler.capture.CompleteCommand(correlationID, result, "")
		})
	}

	// Wire automatic noise detection hooks.
	wireNoiseAutoDetect(handler)
	wireNoiseFirstConnect(handler)

	// Initialize security and audit tools.
	handler.securityScannerImpl = scan.NewScanner()
	handler.thirdPartyAuditorImpl = thirdparty.NewThirdPartyAuditor()
	handler.apiContractValidator = apicontract.NewAPIContractValidator()
	handler.sessionManager = session.NewSessionManager(10, newToolCaptureStateReader(handler))
	handler.auditTrail = audit.NewAuditTrail(audit.Config{
		MaxEntries:   10000,
		Enabled:      true,
		RedactParams: true,
	})
	handler.auditSessionMap = make(map[string]string)

	// Initialize upload security config from package-level var set by CLI.
	handler.uploadSecurity = uploadSecurityConfig
	handler.recordingInteractHandler = screenrec.NewInteractHandler(handler.screenrecDeps())
	interactDeps := buildInteractDeps(handler)
	handler.interactActionHandler = toolinteract.NewInteractActionHandler(interactDeps)
	handler.uploadInteractHandler = toolinteract.NewUploadInteractHandler(interactDeps, handler.interactActionHandler)
	handler.testGenHandler = testgenhandler.New(handler) // *ToolHandler satisfies testgenhandler.Deps
	handler.stateInteractHandler = toolinteract.NewStateInteractHandler(interactDeps, handler.sessionStoreImpl)
	handler.configureSessionHandler = newConfigureSessionHandler(handler, handler.sessionStoreImpl, handler.sessionManager, handler.MCPHandler.server)

	// Initialize dispatch modules and tool schemas once at startup.
	handler.ensureToolModules()
	handler.ensureToolSchemas()

	// Return as MCPHandler but with overridden methods via the wrapper.
	return &MCPHandler{
		server:      server,
		toolHandler: handler,
	}
}
