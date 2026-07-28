// Purpose: Tests for test-generation context-based test creation.
// Docs: docs/features/feature/test-generation/index.md

// context_test.go — Tests for generateTestFromInteraction and generateTestFromRegression
// edge cases not covered by internal/testgen/generate_test.go.
package testgenhandler

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testgen"
)

// ============================================
// Tests for generateTestFromInteraction
// ============================================

func TestGenerateTestFromInteraction_VitestFramework(t *testing.T) {
	t.Parallel()
	env := newTestEnv()

	env.cap.Telemetry().AddEnhancedActionsForTest([]types.EnhancedAction{
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

	env.cap.Telemetry().AddEnhancedActionsForTest([]types.EnhancedAction{
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

	env.cap.Telemetry().AddEnhancedActionsForTest([]types.EnhancedAction{
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

	env.cap.Telemetry().AddEnhancedActionsForTest([]types.EnhancedAction{
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

	env.cap.Telemetry().AddEnhancedActionsForTest([]types.EnhancedAction{
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

	env.cap.Telemetry().AddEnhancedActionsForTest([]types.EnhancedAction{
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

	env.cap.Telemetry().AddEnhancedActionsForTest([]types.EnhancedAction{
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
