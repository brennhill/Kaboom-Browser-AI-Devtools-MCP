// Purpose: Tests for lint-hardening rules and static checks.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// contracts_test.go — Go architecture contracts enforced through static analysis.
// Runs scripts/quality/verification/lint-hardening.sh as a Go test so violations are caught
// by `go test` (including `go test -short`). Fast: only grep-based scans.
package goarchitecturetests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

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
	source, err := os.ReadFile(filepath.Join(projectRoot(), "cmd", "browser-agent", "handler.go"))
	if err != nil {
		t.Fatalf("read MCP handler: %v", err)
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

func TestPassiveTelemetryDoesNotReturnToRootHandler(t *testing.T) {
	sourcePath := filepath.Join(projectRoot(), "cmd", "browser-agent", "handler.go")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read root MCP handler: %v", err)
	}
	for _, forbidden := range []string{
		"type passiveTelemetryCursor struct",
		"telemetryCursors map[",
		"func (h *MCPHandler) telemetryDeltasForClient(",
		"func (h *MCPHandler) evictStaleCursorsLocked(",
		"func parseTelemetryModeOverride(",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("root MCP handler retains passive telemetry owner surface %q", forbidden)
		}
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
		if filepath.Ext(path) != ".go" {
			return nil
		}
		fileSet := token.NewFileSet()
		parsed, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
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
