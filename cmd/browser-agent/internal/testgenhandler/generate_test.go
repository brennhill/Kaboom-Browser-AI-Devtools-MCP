// Purpose: Tests for test-generation script output.
// Docs: docs/features/feature/test-generation/index.md

// generate_test.go — Tests for generate.go pure/helper functions.
// Only tests that cover behavior not already tested in internal/testgen/*_test.go.
package testgenhandler

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testgen"
)

// ============================================
// Tests for testgen.BuildRegressionAssertions
// ============================================

func TestBuildRegressionAssertions_ErrorsWithNetwork(t *testing.T) {
	t.Parallel()

	errors := []string{"some error"}
	bodies := []capture.NetworkBody{
		{Method: "GET", URL: "/api/data", Status: 200},
	}
	assertions, count := testgen.BuildRegressionAssertions(errors, bodies)

	// 0 for baseline with errors + 1 network assertion
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

// ============================================
// Tests for testgen.InsertAssertionsBeforeClose
// ============================================

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
