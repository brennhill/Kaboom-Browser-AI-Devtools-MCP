// Purpose: Tests for Playwright test script generation from recordings.
// Docs: docs/features/feature/test-generation/index.md

// test_script_test.go — Tests for Playwright test script generation.
package generate

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strings"
	"testing"
)

func TestGenerateTestScript_NoActions(t *testing.T) {
	t.Parallel()

	params := TestGenParams{TestName: "empty test"}
	script := GenerateTestScript(nil, params)

	if !strings.Contains(script, "import { test, expect }") {
		t.Error("script should contain Playwright imports")
	}
	if !strings.Contains(script, "test.describe('empty test'") {
		t.Error("script should contain test.describe with test name")
	}
	if !strings.Contains(script, "No actions captured") {
		t.Error("script should contain comment about no actions")
	}
	if !strings.Contains(script, "page.goto('/')") {
		t.Error("script should contain default goto")
	}
}

func TestGenerateTestScript_WithActions(t *testing.T) {
	t.Parallel()

	actions := []types.EnhancedAction{
		{Type: "navigate", Timestamp: 1000, ToURL: "https://example.com/page"},
		{Type: "click", Timestamp: 2000, URL: "https://example.com/page"},
	}
	params := TestGenParams{TestName: "e2e test", AssertNoErrors: true}
	script := GenerateTestScript(actions, params)

	if !strings.Contains(script, "test.describe('e2e test'") {
		t.Error("script should contain test name")
	}
	if !strings.Contains(script, "expect(page).toHaveTitle") {
		t.Error("script should contain title assertion for navigate action")
	}
	if !strings.Contains(script, "Assert no console errors") {
		t.Error("script should contain error assertion when AssertNoErrors=true")
	}
}

func TestGroupActionsByNavigation(t *testing.T) {
	t.Parallel()

	// Empty
	groups := GroupActionsByNavigation(nil)
	if len(groups) != 0 {
		t.Errorf("nil actions should return 0 groups, got %d", len(groups))
	}

	// Single navigate
	groups = GroupActionsByNavigation([]types.EnhancedAction{
		{Type: "navigate", Timestamp: 1000},
	})
	if len(groups) != 1 {
		t.Errorf("single navigate should return 1 group, got %d", len(groups))
	}

	// Navigate + click + navigate + click
	groups = GroupActionsByNavigation([]types.EnhancedAction{
		{Type: "navigate", Timestamp: 1000},
		{Type: "click", Timestamp: 2000},
		{Type: "navigate", Timestamp: 3000},
		{Type: "click", Timestamp: 4000},
	})
	if len(groups) != 2 {
		t.Errorf("two navigates should create 2 groups, got %d", len(groups))
	}
	if len(groups[0]) != 2 {
		t.Errorf("first group should have 2 actions, got %d", len(groups[0]))
	}
	if len(groups[1]) != 2 {
		t.Errorf("second group should have 2 actions, got %d", len(groups[1]))
	}
}

func TestTestLabelForGroup(t *testing.T) {
	t.Parallel()

	// Empty group
	label := testLabelForGroup(nil, 0)
	if label != "step 1" {
		t.Errorf("empty group label = %q, want 'step 1'", label)
	}

	// Navigate with URL
	label = testLabelForGroup([]types.EnhancedAction{
		{Type: "navigate", ToURL: "https://example.com/dashboard"},
	}, 0)
	if !strings.Contains(label, "/dashboard") {
		t.Errorf("navigate group label should contain path, got: %q", label)
	}

	// Non-navigate first action
	label = testLabelForGroup([]types.EnhancedAction{
		{Type: "click", Timestamp: 1000},
	}, 2)
	if label != "step 3" {
		t.Errorf("non-navigate group label = %q, want 'step 3'", label)
	}
}

func TestFilterLastN(t *testing.T) {
	t.Parallel()

	actions := []types.EnhancedAction{
		{Type: "navigate", Timestamp: 1000},
		{Type: "click", Timestamp: 2000},
		{Type: "click", Timestamp: 3000},
	}

	// n <= 0 returns all
	if got := FilterLastN(actions, 0); len(got) != 3 {
		t.Errorf("FilterLastN(0) = %d, want 3", len(got))
	}
	if got := FilterLastN(actions, -1); len(got) != 3 {
		t.Errorf("FilterLastN(-1) = %d, want 3", len(got))
	}

	// n >= len returns all
	if got := FilterLastN(actions, 5); len(got) != 3 {
		t.Errorf("FilterLastN(5) = %d, want 3", len(got))
	}

	// n = 2 returns last 2
	got := FilterLastN(actions, 2)
	if len(got) != 2 {
		t.Fatalf("FilterLastN(2) = %d, want 2", len(got))
	}
	if got[0].Timestamp != 2000 {
		t.Errorf("FilterLastN(2)[0].Timestamp = %d, want 2000", got[0].Timestamp)
	}
}
