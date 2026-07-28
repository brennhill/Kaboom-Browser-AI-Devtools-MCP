// Purpose: Tests for lint-hardening rules and static checks.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// lint_hardening_test.go — Go test wrapper for custom lint rules.
// Runs scripts/lint-hardening.sh as a Go test so violations are caught
// by `go test` (including `go test -short`). Fast: only grep-based scans.
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// projectRoot returns the repository root by navigating from this source file.
func projectRoot() string {
	_, thisFile, _, _ := runtime.Caller(0)
	// thisFile = .../cmd/browser-agent/lint_hardening_test.go
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// TestLintHardening runs the custom hardening lint script and fails the test
// if any violations are found. The script checks for bare goroutines, unchecked
// JSON encodes, missing headers, route sync, middleware, SafeGo closures, and
// queue overflow logging.
func TestLintHardening(t *testing.T) {
	t.Parallel()

	root := projectRoot()
	scriptPath := filepath.Join(root, "scripts", "lint-hardening.sh")

	cmd := exec.Command("bash", scriptPath) // #nosec G204 -- fixed script path, test-only
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("lint-hardening.sh failed (exit %v):\n%s", err, output)
	}
	t.Logf("lint-hardening.sh passed:\n%s", output)
}

func TestRootDoesNotReexportCanonicalTypes(t *testing.T) {
	rootFiles, err := filepath.Glob(filepath.Join(projectRoot(), "cmd", "browser-agent", "*.go"))
	if err != nil {
		t.Fatalf("list browser-agent root files: %v", err)
	}
	for _, forbidden := range []string{
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
		"test_helpers_test.go",
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
		"internal/pagination/pagination_logs.go",
		"internal/security/scan/types.go",
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

	sourcePath := filepath.Join(projectRoot(), "cmd", "browser-agent", "internal", "toolinteract", "interact_evidence.go")
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
		"cmd/browser-agent/internal/toolinteract/deps.go",
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
