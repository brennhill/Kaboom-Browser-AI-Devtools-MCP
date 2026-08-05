// context_test.go — Tests for generateTestFromInteraction and generateTestFromRegression
// edge cases not covered by internal/testgen/generate_test.go.
// Docs: docs/features/feature/test-generation/index.md
package testgenhandler

import (
	"os"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testgen"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestTestgenHandlerPackageRespectsTenFileBoundary(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			files++
		}
	}
	if files > 10 {
		t.Fatalf("testgenhandler package has %d files; want at most 10 change-coupled owners", files)
	}
}

// TestBuildRegressionAssertions_ErrorsWithNetwork keeps the output-helper
// contract beside the context-generation behavior that consumes it.
func TestBuildRegressionAssertions_ErrorsWithNetwork(t *testing.T) {
	t.Parallel()

	errors := []string{"some error"}
	bodies := []types.NetworkBody{
		{Method: "GET", URL: "/api/data", Status: 200},
	}
	assertions, count := testgen.BuildRegressionAssertions(errors, bodies)

	if count != 1 {
		t.Fatalf("assertionCount = %d, want 1", count)
	}
	joined := strings.Join(assertions, "\n")
	if !strings.Contains(joined, "Baseline had 1 console errors") {
		t.Fatal("expected baseline error comment")
	}
	if !strings.Contains(joined, "Assert GET /api/data returns 200") {
		t.Fatal("expected network assertion")
	}
}

func TestInsertAssertionsBeforeClose_EmptyAssertions(t *testing.T) {
	t.Parallel()

	script := "test('test', () => {\n});\n"
	result := testgen.InsertAssertionsBeforeClose(script, nil)

	if !strings.Contains(result, "});") {
		t.Fatal("result should still contain closing brace")
	}
}

func TestInsertAssertionsBeforeClose_MultipleClosingBraces(t *testing.T) {
	t.Parallel()

	script := "test('outer', () => {\n  test('inner', () => {\n  });\n});\n"
	assertions := []string{"  // final assertion"}

	result := testgen.InsertAssertionsBeforeClose(script, assertions)

	lastClose := strings.LastIndex(result, "});")
	assertIdx := strings.LastIndex(result, "// final assertion")
	if assertIdx > lastClose {
		t.Fatal("assertion should appear before the last });")
	}
}

// ============================================
// Tests for generateTestFromInteraction
// ============================================

func TestGenerateTestFromInteraction_VitestFramework(t *testing.T) {
	t.Parallel()
	env := newTestEnv()

	env.cap.Telemetry().AddEnhancedActions([]types.EnhancedAction{
		{Type: "click", URL: "https://example.com"},
	})

	result, err := env.h.generateTestFromInteraction(testgen.TestFromContextRequest{
		Framework: "vitest",
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if result.Framework != "vitest" {
		t.Fatalf("Framework = %q, want vitest", result.Framework)
	}
	if !strings.HasSuffix(result.Filename, ".test.ts") {
		t.Fatalf("Filename = %q, want .test.ts suffix for vitest", result.Filename)
	}
}

func TestGenerateTestFromInteraction_Selectors(t *testing.T) {
	t.Parallel()
	env := newTestEnv()

	env.cap.Telemetry().AddEnhancedActions([]types.EnhancedAction{
		{Type: "click", Selectors: map[string]any{"testId": "login-btn", "id": "loginBtn"}},
	})

	result, err := env.h.generateTestFromInteraction(testgen.TestFromContextRequest{
		Framework: "playwright",
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(result.Selectors) == 0 {
		t.Fatal("Selectors should not be empty")
	}
}

func TestGenerateTestFromInteraction_NoMocksContextUsed(t *testing.T) {
	t.Parallel()
	env := newTestEnv()

	env.cap.Telemetry().AddEnhancedActions([]types.EnhancedAction{
		{Type: "click", Selectors: map[string]any{"target": "#btn"}},
	})

	result, err := env.h.generateTestFromInteraction(testgen.TestFromContextRequest{
		Framework:    "playwright",
		IncludeMocks: false,
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(result.Metadata.ContextUsed) != 1 {
		t.Fatalf("ContextUsed len = %d, want 1 (only actions)", len(result.Metadata.ContextUsed))
	}
	if result.Metadata.ContextUsed[0] != "actions" {
		t.Fatalf("ContextUsed[0] = %q, want actions", result.Metadata.ContextUsed[0])
	}
}

// ============================================
// Tests for generateTestFromRegression
// ============================================

func TestGenerateTestFromRegression_WithMocks(t *testing.T) {
	t.Parallel()
	env := newTestEnv()

	env.cap.Telemetry().AddEnhancedActions([]types.EnhancedAction{
		{Type: "click", Selectors: map[string]any{"target": "#x"}},
	})

	result, err := env.h.generateTestFromRegression(testgen.TestFromContextRequest{
		Framework:    "playwright",
		IncludeMocks: true,
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !result.Coverage.NetworkMocked {
		t.Fatal("NetworkMocked should be true when IncludeMocks is true")
	}
}

func TestGenerateTestFromRegression_JestFramework(t *testing.T) {
	t.Parallel()
	env := newTestEnv()

	env.cap.Telemetry().AddEnhancedActions([]types.EnhancedAction{
		{Type: "click", Selectors: map[string]any{"target": "#a"}},
	})

	result, err := env.h.generateTestFromRegression(testgen.TestFromContextRequest{
		Framework: "jest",
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if result.Framework != "jest" {
		t.Fatalf("Framework = %q, want jest", result.Framework)
	}
	if !strings.HasSuffix(result.Filename, ".test.ts") {
		t.Fatalf("Filename = %q, want .test.ts suffix for jest", result.Filename)
	}
}

func TestGenerateTestFromRegression_SelectorsExtracted(t *testing.T) {
	t.Parallel()
	env := newTestEnv()

	env.cap.Telemetry().AddEnhancedActions([]types.EnhancedAction{
		{Type: "click", Selectors: map[string]any{"testId": "save-btn", "id": "saveBtn"}},
	})

	result, err := env.h.generateTestFromRegression(testgen.TestFromContextRequest{
		Framework: "playwright",
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(result.Selectors) == 0 {
		t.Fatal("Selectors should not be empty for regression test")
	}
}

func TestGenerateTestFromRegression_ContentHasActions(t *testing.T) {
	t.Parallel()
	env := newTestEnv()

	env.cap.Telemetry().AddEnhancedActions([]types.EnhancedAction{
		{Type: "click", Selectors: map[string]any{"id": "go"}, URL: "https://example.com"},
		{Type: "input", Selectors: map[string]any{"id": "name"}, Value: "test"},
	})

	result, err := env.h.generateTestFromRegression(testgen.TestFromContextRequest{
		Framework: "playwright",
		BaseURL:   "https://example.com",
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(result.Content, "locator('#go').click()") {
		t.Fatalf("content should contain click action;\nContent:\n%s", result.Content)
	}
	if !strings.Contains(result.Content, "locator('#name').fill('test')") {
		t.Fatalf("content should contain fill action;\nContent:\n%s", result.Content)
	}
}
