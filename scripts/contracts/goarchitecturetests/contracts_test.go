// Purpose: Tests for lint-hardening rules and static checks.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// contracts_test.go — Go architecture contracts enforced through static analysis.
// Runs scripts/quality/verification/lint-hardening.sh as a Go test so violations are caught
// by `go test` (including `go test -short`). Fast: only grep-based scans.
package goarchitecturetests

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBrowserAgentRootContainsNoCompiledArtifacts(t *testing.T) {
	root := filepath.Join(projectRoot(), "cmd", "browser-agent")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read browser-agent root: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name())
		file, openErr := os.Open(path)
		if openErr != nil {
			t.Fatalf("open %s: %v", path, openErr)
		}
		header := make([]byte, 4)
		count, readErr := file.Read(header)
		closeErr := file.Close()
		if closeErr != nil {
			t.Fatalf("close %s: %v", path, closeErr)
		}
		if readErr != nil && count == 0 {
			continue // EXPECTED_ABSENCE: empty source assets have no binary signature to classify.
		}
		binary := bytes.Equal(header, []byte{0x7f, 'E', 'L', 'F'}) ||
			bytes.Equal(header, []byte{0xcf, 0xfa, 0xed, 0xfe}) ||
			bytes.Equal(header, []byte{0xfe, 0xed, 0xfa, 0xcf}) ||
			bytes.Equal(header[:2], []byte{'M', 'Z'})
		if binary {
			t.Errorf("compiled artifact is tracked in source root: %s", entry.Name())
		}
	}
}

// projectRoot returns the repository root containing go.mod.
func projectRoot() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	for dir := filepath.Dir(thisFile); ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		if filepath.Dir(dir) == dir {
			return ""
		}
	}
}

func TestRootDoesNotReexportCanonicalTypes(t *testing.T) {
	rootFiles, err := filepath.Glob(filepath.Join(projectRoot(), "cmd", "browser-agent", "*.go"))
	if err != nil {
		t.Fatalf("list browser-agent root files: %v", err)
	}
	for _, forbidden := range []string{
		"var releaseChecker ",
		"var binaryUpgradeState ",
		"var startTime ",
		"var updateNotifyLastShown ",
		"osUploadAutomationFlag ",
		"uploadSecurityConfig ",
		"\n\tstartupWarnings []string",
		"var bridgeRunner ",
		"var exitDiagnostics ",
		"daemonProcessCommand =",
		"daemonIsProcessAlive =",
		"daemonIsServerRunning =",
		"daemonTryShutdown =",
		"daemonWaitForPortRelease =",
		"daemonTerminatePID =",
		"daemonFindProcessOnPort =",
		"type JSONRPCRequest =",
		"type JSONRPCResponse =",
		"type MCPToolResult =",
		"type Annotation =",
		"type AnnotationStore =",
		"type ToolCallLimiter =",
		"ErrInvalidJSON =",
		"type StructuredError =",
		"func mcpStructuredError(",
		"func withParam(",
		"func withHint(",
		"func withAction(",
		"func withSelector(",
		"func withRetryable(",
		"func withRetryAfterMs(",
		"func withFinal(",
		"func withRecoveryToolCall(",
		"var NewToolCallLimiter =",
		"var randomInt63 =",
		"var legacyMCPServerNames =",
		"activeBoundariesMu ",
		"activeBoundaries ",
		"toolConfigureTestBoundaryStart(",
		"toolConfigureTestBoundaryEnd(",
		"func (h *ToolHandler) sequenceHandler(",
		"toolConfigureSaveSequence(",
		"toolConfigureGetSequence(",
		"toolConfigureListSequences(",
		"toolConfigureDeleteSequence(",
		"toolConfigureReplaySequence(",
		"toolConfigureEventRecordingStart(",
		"toolConfigureEventRecordingStop(",
		"toolConfigurePlayback(",
		"toolConfigureLogDiff(",
		"toolConfigureNetworkRecording(",
		"func (h *ToolHandler) testGen(",
		"func (h *ToolHandler) stateInteract(",
		"func (h *ToolHandler) interactAction(",
		"func (h *ToolHandler) armEvidenceForCommand(",
		"func (h *ToolHandler) getCommandResult(",
		"func (h *ToolHandler) buildPlaybackResult(",
		"func (h *ToolHandler) appendServerLog(",
		"func (h *ToolHandler) summaryPreference(",
		"func (h *ToolHandler) loadSummaryPref(",
		"func (h *ToolHandler) invalidateSummaryPref(",
		"func (h *ToolHandler) maybeInjectSummary(",
		"func (h *ToolHandler) NoiseConfig(",
		"func (h *ToolHandler) ConsoleEntries(",
		"func (h *ToolHandler) AllWebSocketEvents(",
		"func (h *ToolHandler) GetPilotStatus(",
		"func (h *ToolHandler) GetToolModuleExamples(",
		"func (h *ToolHandler) GetSecurityMode(",
		"func (h *ToolHandler) SetSecurityMode(",
		"func (h *ToolHandler) GetTelemetryMode(",
		"func (h *ToolHandler) SetTelemetryMode(",
		"func (h *ToolHandler) InteractActionSetJitter(",
		"func (h *ToolHandler) InteractActionGetJitter(",
		"func (h *ToolHandler) HasCapture(",
		"func (h *ToolHandler) GetTrackingStatus(",
		"func (h *ToolHandler) NetworkBodies(",
		"func (h *ToolHandler) NetworkWaterfallEntries(",
		"func (h *ToolHandler) ConsoleSecurityEntries(",
		"func (h *ToolHandler) SecurityScanner(",
		"func (h *ToolHandler) LogEntries(",
		"func (h *ToolHandler) CollectIssueReport(",
		"func (h *ToolHandler) SanitizeIssueReport(",
		"func (h *ToolHandler) SubmitIssueReport(",
		"func (h *ToolHandler) GetAnnotationStore(",
		"func (h *ToolHandler) GetVersion(",
		"func (h *ToolHandler) IsExtensionConnected(",
		"func (h *ToolHandler) PushInbox(",
		"func (h *ToolHandler) IsConsoleNoise(",
		"func (h *ToolHandler) GetLogEntries(",
		"func (h *ToolHandler) GetLogTotalAdded(",
		"func (h *ToolHandler) GetToolCallLimiter(",
		"func (h *ToolHandler) GetRedactionEngine(",
		"func (h *ToolHandler) ToolsList(",
		"func (h *ToolHandler) GetCapture(",
		"func (h *ToolHandler) attachTransientElements(",
		"func (h *ToolHandler) EnqueuePendingQuery(",
		"func (h *ToolHandler) ExecuteA11yQuery(",
		"func (h *ToolHandler) finalizeResponseEnrichment(",
		"func (h *ToolHandler) formatCommandResult(",
		"func (h *ToolHandler) formatErrorCommandResult(",
		"func (h *ToolHandler) formatExpiredCommandResult(",
		"func (h *ToolHandler) formatTimeoutCommandResult(",
		"func (h *ToolHandler) formatCancelledCommandResult(",
		"func (h *ToolHandler) formatCompletedCommand(",
		"func (h *ToolHandler) attachPerfDiffIfAvailable(",
		"func (h *ToolHandler) waitForCommandWithConnectivity(",
		"func (h *ToolHandler) finalizePendingDisconnect(",
		"func (h *ToolHandler) MaybeWaitForCommand(",
	} {
		for _, path := range rootFiles {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			source, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("read %s: %v", path, readErr)
			}
			if strings.Contains(string(source), forbidden) {
				t.Errorf("%s re-exports canonical API %q", filepath.Base(path), forbidden)
			}
		}
	}
}

func TestMCPToolBackendIsExecutionOnly(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(projectRoot(), "cmd", "browser-agent", "internal", "mcpcall", "handler.go"))
	if err != nil {
		t.Fatalf("read MCP call owner: %v", err)
	}
	for _, forbidden := range []string{
		"type ToolHandlerInterface interface {",
		"GetCapture() *capture.Capture",
		"GetToolCallLimiter() RateLimiter",
		"GetRedactionEngine() RedactionEngine",
		"ToolsList() []mcp.MCPTool",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("MCP tool backend retains transport dependency %q", forbidden)
		}
	}
}

func TestBrowserActionsExposeOnlyCanonicalDispatch(t *testing.T) {
	path := filepath.Join(projectRoot(), "cmd", "browser-agent", "internal", "toolinteract", "interact_browser.go")
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse browser actions: %v", err)
	}
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv == nil || !function.Name.IsExported() {
			continue
		}
		receiver := function.Recv.List[0].Type
		pointer, ok := receiver.(*ast.StarExpr)
		if !ok {
			continue
		}
		name, ok := pointer.X.(*ast.Ident)
		if ok && name.Name == "BrowserActions" && function.Name.Name != "Handle" {
			t.Errorf("BrowserActions exports implementation method %s; route through Handle", function.Name.Name)
		}
	}
}

func TestObserveDispatcherDoesNotRequireHostInterfaces(t *testing.T) {
	for relativePath, forbidden := range map[string]string{
		"cmd/browser-agent/internal/toolobserve/deps.go":       "type Deps interface {",
		"cmd/browser-agent/internal/toolobserve/dispatcher.go": "type Host interface {",
		"internal/tools/observe/core/deps.go":                  "type Deps interface {",
	} {
		source, err := os.ReadFile(filepath.Join(projectRoot(), relativePath))
		if err != nil {
			t.Fatalf("read %s: %v", relativePath, err)
		}
		if strings.Contains(string(source), forbidden) {
			t.Errorf("%s retains host dependency interface %q", relativePath, forbidden)
		}
	}
}

func TestUnusedToolHostContractsStayDeleted(t *testing.T) {
	for _, relativePath := range []string{
		"internal/mcp/deps.go",
		"internal/tools/analyze/deps.go",
		"internal/tools/configure/deps.go",
		"internal/tools/interact/deps.go",
	} {
		if _, err := os.Stat(filepath.Join(projectRoot(), relativePath)); !os.IsNotExist(err) {
			t.Errorf("unused tool host contract still exists: %s", relativePath)
		}
	}
}

func TestActionRecordingDoesNotReturnToToolHandler(t *testing.T) {
	for _, forbidden := range []string{
		"func (h *ToolHandler) recordAIAction(",
		"func (h *ToolHandler) recordAIEnhancedAction(",
		"func (h *ToolHandler) recordDOMPrimitiveAction(",
	} {
		for _, relativePath := range []string{
			"cmd/browser-agent/tools_core.go",
			"cmd/browser-agent/tools_interact_dispatch.go",
			"cmd/browser-agent/tools_configure.go",
		} {
			source, err := os.ReadFile(filepath.Join(projectRoot(), relativePath))
			if err != nil {
				t.Fatalf("read %s: %v", relativePath, err)
			}
			if strings.Contains(string(source), forbidden) {
				t.Errorf("%s retains action-recording root method %q", relativePath, forbidden)
			}
		}
	}
}

func TestNavigateEnrichmentBelongsToInteractOwner(t *testing.T) {
	checks := map[string]string{
		"cmd/browser-agent/tools_interact_dispatch.go":                   "func (h *ToolHandler) enrichNavigateResponse(",
		"cmd/browser-agent/internal/toolinteract/action_owners.go":       "EnrichNavigateResponse func(",
		"cmd/browser-agent/internal/toolinteract/action_runtime_test.go": "EnrichNavigateResponse:",
	}
	for relativePath, forbidden := range checks {
		source, err := os.ReadFile(filepath.Join(projectRoot(), relativePath))
		if err != nil {
			t.Fatalf("read %s: %v", relativePath, err)
		}
		if strings.Contains(string(source), forbidden) {
			t.Errorf("%s retains navigation enrichment facade %q", relativePath, forbidden)
		}
	}
}

func TestClientRegistryUsesCanonicalConcreteOwner(t *testing.T) {
	checks := map[string][]string{
		"internal/capture/clientstore/owner.go": {
			"type Registry interface",
			"registry Registry",
			"Registry() Registry",
		},
		"cmd/browser-agent/main_connection_mcp.go": {
			"sessionClientRegistryAdapter",
			"newSessionClientRegistryAdapter",
		},
		"cmd/browser-agent/internal/terminal/handlers.go": {
			"clients.(type)",
			"json.Marshal(v)",
		},
	}
	for relativePath, forbiddenValues := range checks {
		source, err := os.ReadFile(filepath.Join(projectRoot(), relativePath))
		if err != nil {
			t.Fatalf("read %s: %v", relativePath, err)
		}
		for _, forbidden := range forbiddenValues {
			if strings.Contains(string(source), forbidden) {
				t.Errorf("%s retains client-registry compatibility surface %q", relativePath, forbidden)
			}
		}
	}
}

func TestDaemonRecoveryPrimitivesDoNotReturnToMain(t *testing.T) {
	sourcePath := filepath.Join(projectRoot(), "cmd", "browser-agent", "main_connection_recovery.go")
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("main recovery compatibility surface still exists: %s", sourcePath)
	}
	canonicalPath := filepath.Join(projectRoot(), "cmd", "browser-agent", "internal", "daemonrecovery", "reclaimer.go")
	source, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatalf("read canonical daemon recovery owner: %v", err)
	}
	for _, required := range []string{"type Reclaimer struct", "func (r *Reclaimer) ReclaimPort(", "func (r *Reclaimer) LifecycleDeps("} {
		if !strings.Contains(string(source), required) {
			t.Errorf("canonical daemon recovery owner is missing %q", required)
		}
	}
}

func TestPassiveTelemetryDoesNotLeakIntoEndpointOwner(t *testing.T) {
	sourcePath := filepath.Join(projectRoot(), "cmd", "browser-agent", "internal", "mcpendpoint", "handler.go")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read MCP endpoint owner: %v", err)
	}
	for _, forbidden := range []string{
		"type passiveTelemetryCursor struct",
		"telemetryCursors map[",
		"func (h *Handler) telemetryDeltasForClient(",
		"func (h *Handler) evictStaleCursorsLocked(",
		"func parseTelemetryModeOverride(",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("root MCP handler retains passive telemetry owner surface %q", forbidden)
		}
	}
}

func TestResponsePolicyDoesNotLeakIntoEndpointOwner(t *testing.T) {
	sourcePath := filepath.Join(projectRoot(), "cmd", "browser-agent", "internal", "mcpendpoint", "handler.go")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read MCP endpoint owner: %v", err)
	}
	for _, forbidden := range []string{
		"func (h *Handler) warnUnknownToolArguments(",
		"func (h *Handler) maybeAddPendingIntents(",
		"func (h *Handler) maybeAddSecurityModeWarning(",
		"func (h *Handler) maybeAddVersionWarning(",
		"func (h *Handler) maybeAddUpdateAvailableWarning(",
		"func (h *Handler) maybeAddUpgradeWarning(",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("root MCP handler retains response-policy surface %q", forbidden)
		}
	}
}

func TestStatelessProtocolResponsesDoNotLeakIntoEndpointOwner(t *testing.T) {
	sourcePath := filepath.Join(projectRoot(), "cmd", "browser-agent", "internal", "mcpendpoint", "handler.go")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read MCP endpoint owner: %v", err)
	}
	for _, forbidden := range []string{
		"const serverInstructions =",
		"func (h *Handler) handleInitialize(",
		"func (h *Handler) handleResourcesList(",
		"func (h *Handler) handleResourcesRead(",
		"func (h *Handler) handleResourcesTemplatesList(",
		"func (h *Handler) handleToolsList(",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("root MCP handler retains stateless protocol surface %q", forbidden)
		}
	}
}

func TestToolCallPipelineDoesNotLeakIntoEndpointOwner(t *testing.T) {
	sourcePath := filepath.Join(projectRoot(), "cmd", "browser-agent", "internal", "mcpendpoint", "handler.go")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read MCP endpoint owner: %v", err)
	}
	for _, forbidden := range []string{
		"type ToolExecutor interface",
		"type ToolBackend struct",
		"type RateLimiter interface",
		"type RedactionEngine interface",
		"func (h *Handler) handleToolsCall(",
		"func (h *Handler) checkToolRateLimit(",
		"func (h *Handler) applyToolResponsePostProcessing(",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("root MCP handler retains tool-call pipeline surface %q", forbidden)
		}
	}
}

func TestJSONRPCRouterDoesNotLeakIntoEndpointOwner(t *testing.T) {
	sourcePath := filepath.Join(projectRoot(), "cmd", "browser-agent", "internal", "mcpendpoint", "handler.go")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read MCP endpoint owner: %v", err)
	}
	for _, forbidden := range []string{
		"type mcpMethodHandler ",
		"var mcpMethodHandlers =",
		"var mcpStaticResponses =",
		"request.HasInvalidID()",
		"request.JSONRPC != mcp.JSONRPCVersion",
		"Method not found: ",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("root MCP handler retains JSON-RPC routing surface %q", forbidden)
		}
	}
}

func TestClientRegistryHTTPDoesNotReturnToRootServer(t *testing.T) {
	sourcePath := filepath.Join(projectRoot(), "cmd", "browser-agent", "server.go")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read root server: %v", err)
	}
	for _, forbidden := range []string{
		"func resolveClientRegistry(",
		"func registerClientRegistryRoutes(",
		"func handleClientsList(",
		"func handleClientByID(",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("root server retains client registry HTTP surface %q", forbidden)
		}
	}
	if !strings.Contains(string(source), "clientapi.Register(") {
		t.Error("root server does not compose the canonical clientapi owner")
	}
}

func TestTelemetryHTTPDoesNotReturnToRootServer(t *testing.T) {
	sourcePath := filepath.Join(projectRoot(), "cmd", "browser-agent", "server.go")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read root server: %v", err)
	}
	if strings.Contains(string(source), "func handleTelemetry(") {
		t.Error("root server retains local telemetry HTTP implementation")
	}
	if !strings.Contains(string(source), "telemetryapi.Handler(") {
		t.Error("root server does not compose the canonical telemetryapi owner")
	}
}

func TestDependencyBuildersAreNotToolHandlerMethods(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(projectRoot(), "cmd", "browser-agent", "tools_core.go"))
	if err != nil {
		t.Fatalf("read tools_core.go: %v", err)
	}
	if strings.Contains(string(source), "func (h *ToolHandler) screenrecDeps(") {
		t.Fatal("screen recording dependency builder remains a ToolHandler method")
	}
}

func TestConfigureLifecycleDoesNotReturnToToolHandler(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(projectRoot(), "cmd", "browser-agent", "tools_configure.go"))
	if err != nil {
		t.Fatalf("read tools_configure.go: %v", err)
	}
	for _, forbidden := range []string{
		"func (h *ToolHandler) toolGetHealth(",
		"func (h *ToolHandler) toolDoctor(",
		"func (h *ToolHandler) toolConfigure(",
		"func (h *ToolHandler) toolGetAuditLog(",
		"func (h *ToolHandler) toolConfigureStreaming(",
		"func (h *ToolHandler) toolConfigureRestart(",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("configure lifecycle retains root method %q", forbidden)
		}
	}
}

func TestToolCatalogDoesNotReturnToCompositionRoot(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(projectRoot(), "cmd", "browser-agent", "tools_core.go"))
	if err != nil {
		t.Fatalf("read tools_core.go: %v", err)
	}
	for _, forbidden := range []string{
		"func (h *ToolHandler) getToolSchema(",
		"func (h *ToolHandler) ensureToolModules(",
		"func (h *ToolHandler) ensureToolSchemas(",
		"toolModulesOnce sync.Once",
		"toolSchemasOnce sync.Once",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("composition root retains tool catalog state %q", forbidden)
		}
	}
}

func TestTestGenerationDoesNotRequireHostInterface(t *testing.T) {
	relativePath := "cmd/browser-agent/internal/testgenhandler/handler.go"
	source, err := os.ReadFile(filepath.Join(projectRoot(), relativePath))
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}
	if strings.Contains(string(source), "type Deps interface {") {
		t.Fatal("test generation retains host dependency interface")
	}
}

func TestGeneratePackagesDoNotRequireHostInterfaces(t *testing.T) {
	for relativePath, forbidden := range map[string]string{
		"cmd/browser-agent/internal/toolgenerate/deps.go":                 "type Deps interface {",
		"cmd/browser-agent/internal/toolgenerate/annotations/handlers.go": "type Deps interface {",
	} {
		source, err := os.ReadFile(filepath.Join(projectRoot(), relativePath))
		if err != nil {
			t.Fatalf("read %s: %v", relativePath, err)
		}
		if strings.Contains(string(source), forbidden) {
			t.Errorf("%s retains host dependency interface %q", relativePath, forbidden)
		}
	}
}

func TestIssueReportDoesNotRequireHostInterface(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(projectRoot(), "internal", "issuereport", "handler.go"))
	if err != nil {
		t.Fatalf("read issue report handler: %v", err)
	}
	if strings.Contains(string(source), "type HandlerDeps interface {") {
		t.Fatal("issue report handler retains host dependency interface")
	}
}

func TestAnalyzePackagesDoNotRequireCatchAllHost(t *testing.T) {
	for relativePath, forbidden := range map[string]string{
		"cmd/browser-agent/internal/toolanalyze/deps.go":                       "type Deps interface {",
		"cmd/browser-agent/internal/toolanalyze/analyzedispatch/dispatcher.go": "type Host interface {",
		"cmd/browser-agent/internal/toolanalyze/combinedaudit/handler.go":      "type Deps interface {",
	} {
		source, err := os.ReadFile(filepath.Join(projectRoot(), relativePath))
		if err != nil {
			t.Fatalf("read %s: %v", relativePath, err)
		}
		if strings.Contains(string(source), forbidden) {
			t.Errorf("%s retains catch-all dependency interface %q", relativePath, forbidden)
		}
	}
}

func TestInteractTestsDoNotMirrorRootAccessor(t *testing.T) {
	sourcePath := filepath.Join(
		projectRoot(),
		"cmd",
		"browser-agent",
		"internal",
		"toolinteract",
		"interact_dom_test.go",
	)
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read interact test helpers: %v", err)
	}
	for _, forbidden := range []string{
		"newTestToolHandler",
		"testToolHandlerShim",
		"func (s *testToolHandlerShim) interactAction(",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("interact tests retain root accessor facade %q", forbidden)
		}
	}
}

func TestPackagesDoNotReexportCanonicalLogEntry(t *testing.T) {
	for _, relativePath := range []string{
		"internal/mcp/types.go",
		"internal/server/main_handlers.go",
		"internal/pagination/entries.go",
		"internal/security/scan/scan.go",
		"cmd/browser-agent/internal/logstore/store.go",
	} {
		path := filepath.Join(projectRoot(), relativePath)
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", relativePath, err)
		}
		for _, forbidden := range []string{"type LogEntry =", "type Entry ="} {
			if strings.Contains(string(source), forbidden) {
				t.Errorf("%s re-exports canonical log contract with %q", relativePath, forbidden)
			}
		}
	}
}

func TestEvidenceCaptureHasNoCompatibilityShim(t *testing.T) {
	shimPath := filepath.Join(projectRoot(), "cmd", "browser-agent", "interact_evidence_test_aliases_test.go")
	if _, err := os.Stat(shimPath); !os.IsNotExist(err) {
		t.Fatalf("evidence compatibility shim still exists: %s", shimPath)
	}

	sourcePath := filepath.Join(projectRoot(), "cmd", "browser-agent", "internal", "toolinteract", "action_owners.go")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read evidence implementation: %v", err)
	}
	if strings.Contains(string(source), "type EvidenceShot =") {
		t.Fatal("evidence implementation still aliases its public type")
	}
}

func TestHardeningLintTracksCanonicalRouteAndGoroutineOwners(t *testing.T) {
	scriptPath := filepath.Join(projectRoot(), "scripts", "quality", "verification", "lint-hardening.sh")
	source, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read hardening lint: %v", err)
	}
	text := string(source)
	for _, canonicalOwner := range []string{
		"cmd/browser-agent/internal/clientapi/handler.go",
		"internal/util/response.go",
	} {
		if !strings.Contains(text, canonicalOwner) {
			t.Errorf("hardening lint does not recognize canonical owner %s", canonicalOwner)
		}
	}
}

func TestFeaturePackagesDoNotMirrorGuardContract(t *testing.T) {
	for _, relativePath := range []string{
		"cmd/browser-agent/internal/toolinteract/action_owners.go",
		"cmd/browser-agent/internal/toolinteract/interactupload/upload.go",
		"cmd/browser-agent/internal/screenrec/deps.go",
	} {
		source, err := os.ReadFile(filepath.Join(projectRoot(), relativePath))
		if err != nil {
			t.Fatalf("read %s: %v", relativePath, err)
		}
		for _, forbidden := range []string{"type GuardCheck =", "type Guard ="} {
			if strings.Contains(string(source), forbidden) {
				t.Errorf("%s mirrors guard contract with %q", relativePath, forbidden)
			}
		}
	}
}

// isAuthoredGoFile reports whether a walked entry is Go source a human wrote.
//
// Dot-prefixed .go files are excluded. Tools write scratch files into the repository
// root — internal/hook/eval creates ".kaboom-eval-oversized-*.go" there — and under
// `go test ./...` those packages run concurrently with this walk. Without this filter
// the walk parses a scratch file that the tool then deletes, and the whole suite fails
// with "no such file or directory" on a file that was never authored source.
func isAuthoredGoFile(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasPrefix(name, ".")
}

func TestAuthoredGoFileFilterIgnoresToolScratchFiles(t *testing.T) {
	if isAuthoredGoFile(".kaboom-eval-oversized-238948641.go") {
		t.Error("tool scratch files must not be walked; a concurrent package deletes them mid-walk")
	}
	if !isAuthoredGoFile("contracts_test.go") {
		t.Error("authored Go source must still be walked")
	}
}

func TestAuthoredGoDoesNotDeclareTypeAliases(t *testing.T) {
	err := filepath.WalkDir(projectRoot(), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !isAuthoredGoFile(entry.Name()) {
			return nil
		}
		fileSet := token.NewFileSet()
		parsed, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			// A file deleted between the walk and the read is not a contract violation.
			if errors.Is(parseErr, os.ErrNotExist) {
				return nil
			}
			return parseErr
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if ok && spec.Assign.IsValid() {
				position := fileSet.Position(spec.Pos())
				t.Errorf("%s:%d declares Go type alias %s", path, position.Line, spec.Name.Name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
