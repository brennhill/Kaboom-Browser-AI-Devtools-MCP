// Purpose: Adapts Handler deps to the internal testgen DataProvider API.
// Why: Isolates data access and wrapper delegation from request parsing/response formatting.
// Docs: docs/features/feature/test-generation/index.md

package testgenhandler

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testgen"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testgen/heal"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// dataProviderAdapter adapts Deps to testgen.DataProvider.
type dataProviderAdapter struct {
	deps Deps
}

func (a *dataProviderAdapter) GetLogEntries() []types.LogEntry {
	entries, _ := a.deps.GetLogEntries()
	return entries
}

func (a *dataProviderAdapter) GetAllEnhancedActions() []types.EnhancedAction {
	return a.deps.GetCapture().Telemetry().GetAllEnhancedActions()
}

func (a *dataProviderAdapter) GetNetworkBodies() []types.NetworkBody {
	return a.deps.GetCapture().Telemetry().GetNetworkBodies()
}

// dataProvider returns a testgen.DataProvider backed by this test-generation handler.
func (h *Handler) dataProvider() testgen.DataProvider {
	return &dataProviderAdapter{deps: h.deps}
}

func (h *Handler) generateTestFromError(req testgen.TestFromContextRequest) (*testgen.GeneratedTest, error) {
	return testgen.GenerateTestFromError(h.dataProvider(), req)
}

func (h *Handler) generateTestFromInteraction(req testgen.TestFromContextRequest) (*testgen.GeneratedTest, error) {
	return testgen.GenerateTestFromInteraction(h.dataProvider(), req)
}

func (h *Handler) generateTestFromRegression(req testgen.TestFromContextRequest) (*testgen.GeneratedTest, error) {
	return testgen.GenerateTestFromRegression(h.dataProvider(), req)
}

func (h *Handler) analyzeTestFile(req heal.TestHealRequest, projectDir string) ([]string, error) {
	return heal.AnalyzeTestFile(req, projectDir)
}

func (h *Handler) repairSelectors(req heal.TestHealRequest, _ string) (*heal.HealResult, error) {
	return heal.RepairSelectors(req)
}

func (h *Handler) healTestBatch(req heal.TestHealRequest, projectDir string) (*heal.BatchHealResult, error) {
	return heal.HealTestBatch(req, projectDir)
}

func (h *Handler) classifyFailure(failure *testgen.TestFailure) *testgen.FailureClassification {
	return testgen.ClassifyFailure(failure)
}

func (h *Handler) classifyFailureBatch(failures []testgen.TestFailure) *testgen.BatchClassifyResult {
	return testgen.ClassifyFailureBatch(failures)
}
