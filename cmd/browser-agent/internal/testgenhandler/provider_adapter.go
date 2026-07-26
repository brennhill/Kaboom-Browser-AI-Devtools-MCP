// Purpose: Adapts Handler deps to the internal testgen DataProvider API.
// Why: Isolates data access and wrapper delegation from request parsing/response formatting.
// Docs: docs/features/feature/test-generation/index.md

package testgenhandler

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testgen"
)

// dataProviderAdapter adapts Deps to testgen.DataProvider.
type dataProviderAdapter struct {
	deps Deps
}

func (a *dataProviderAdapter) GetLogEntries() []map[string]any {
	entries, _ := a.deps.GetLogEntries()
	return entries
}

func (a *dataProviderAdapter) GetAllEnhancedActions() []capture.EnhancedAction {
	return a.deps.GetCapture().GetAllEnhancedActions()
}

func (a *dataProviderAdapter) GetNetworkBodies() []capture.NetworkBody {
	return a.deps.GetCapture().GetNetworkBodies()
}

// dataProvider returns a testgen.DataProvider backed by this test-generation handler.
func (h *Handler) dataProvider() testgen.DataProvider {
	return &dataProviderAdapter{deps: h.deps}
}

func (h *Handler) generateTestFromError(req TestFromContextRequest) (*GeneratedTest, error) {
	return testgen.GenerateTestFromError(h.dataProvider(), req)
}

func (h *Handler) generateTestFromInteraction(req TestFromContextRequest) (*GeneratedTest, error) {
	return testgen.GenerateTestFromInteraction(h.dataProvider(), req)
}

func (h *Handler) generateTestFromRegression(req TestFromContextRequest) (*GeneratedTest, error) {
	return testgen.GenerateTestFromRegression(h.dataProvider(), req)
}

func (h *Handler) analyzeTestFile(req TestHealRequest, projectDir string) ([]string, error) {
	return testgen.AnalyzeTestFile(req, projectDir)
}

func (h *Handler) repairSelectors(req TestHealRequest, _ string) (*HealResult, error) {
	return testgen.RepairSelectors(req)
}

func (h *Handler) healTestBatch(req TestHealRequest, projectDir string) (*BatchHealResult, error) {
	return testgen.HealTestBatch(req, projectDir)
}

func (h *Handler) classifyFailure(failure *TestFailure) *FailureClassification {
	return testgen.ClassifyFailure(failure)
}

func (h *Handler) classifyFailureBatch(failures []TestFailure) *BatchClassifyResult {
	return testgen.ClassifyFailureBatch(failures)
}
