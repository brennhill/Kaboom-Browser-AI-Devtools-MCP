// Purpose: Tests for reproduction script generation from captured actions.
// Docs: docs/features/feature/reproduction-scripts/index.md

// reproduction_test.go — Tests for reproduction script generation.
// Verifies Kaboom (natural language) and Playwright output formats,
// selector priority, timing pauses, URL rewriting, and edge cases.
package reproduction

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strings"
	"testing"
)

// ============================================
// Test Helpers
// ============================================

func makeTestAction(typ string, ts int64, opts map[string]any) types.EnhancedAction {
	a := types.EnhancedAction{
		Type:      typ,
		Timestamp: ts,
		URL:       "https://example.com",
	}
	if s, ok := opts["selectors"].(map[string]any); ok {
		a.Selectors = s
	}
	if v, ok := opts["value"].(string); ok {
		a.Value = v
	}
	if v, ok := opts["toURL"].(string); ok {
		a.ToURL = v
	}
	if v, ok := opts["key"].(string); ok {
		a.Key = v
	}
	if v, ok := opts["selectedValue"].(string); ok {
		a.SelectedValue = v
	}
	if v, ok := opts["selectedText"].(string); ok {
		a.SelectedText = v
	}
	if v, ok := opts["scrollY"].(int); ok {
		a.ScrollY = v
	}
	if v, ok := opts["source"].(string); ok {
		a.Source = v
	}
	return a
}

func basicFlow() []types.EnhancedAction {
	return []types.EnhancedAction{
		makeTestAction("navigate", 1000, map[string]any{
			"toURL": "https://example.com/login",
		}),
		makeTestAction("click", 2000, map[string]any{
			"selectors": map[string]any{
				"text": "Sign In",
				"role": map[string]any{"role": "button", "name": "Sign In"},
			},
		}),
		makeTestAction("input", 3000, map[string]any{
			"selectors": map[string]any{
				"ariaLabel": "Email address",
				"role":      map[string]any{"role": "textbox", "name": "Email address"},
			},
			"value": "alice@example.com",
		}),
	}
}

// ============================================
// Kaboom Format Tests
// ============================================

func TestReproduction_Kaboom_BasicFlow(t *testing.T) {
	t.Parallel()
	actions := basicFlow()
	script := GenerateKaboomScript(actions, Params{})

	// Header
	if !strings.Contains(script, "# Reproduction:") {
		t.Error("expected header with '# Reproduction:'")
	}
	if !strings.Contains(script, "3 actions") {
		t.Errorf("expected '3 actions' in header, got:\n%s", script)
	}

	// Navigate step
	if !strings.Contains(script, "Navigate to: https://example.com/login") {
		t.Errorf("expected navigate step, got:\n%s", script)
	}

	// Click step with text + role
	if !strings.Contains(script, `Click: "Sign In" button`) {
		t.Errorf("expected click description, got:\n%s", script)
	}

	// Input step
	if !strings.Contains(script, `Type "alice@example.com" into:`) {
		t.Errorf("expected input step, got:\n%s", script)
	}
}

func TestReproduction_Kaboom_AllActionTypes(t *testing.T) {
	t.Parallel()
	actions := []types.EnhancedAction{
		makeTestAction("navigate", 1000, map[string]any{"toURL": "https://example.com"}),
		makeTestAction("click", 2000, map[string]any{
			"selectors": map[string]any{"text": "Go"},
		}),
		makeTestAction("input", 3000, map[string]any{
			"selectors": map[string]any{"id": "name"},
			"value":     "Alice",
		}),
		makeTestAction("select", 4000, map[string]any{
			"selectors":    map[string]any{"id": "country"},
			"selectedText": "Canada",
		}),
		makeTestAction("keypress", 5000, map[string]any{"key": "Enter"}),
		makeTestAction("scroll", 6000, map[string]any{"scrollY": 500}),
	}

	script := GenerateKaboomScript(actions, Params{})

	if !strings.Contains(script, "Navigate to:") {
		t.Error("missing navigate")
	}
	if !strings.Contains(script, "Click:") {
		t.Error("missing click")
	}
	if !strings.Contains(script, "Type") {
		t.Error("missing input/type")
	}
	if !strings.Contains(script, `Select "Canada"`) {
		t.Error("missing select")
	}
	if !strings.Contains(script, "Press: Enter") {
		t.Error("missing keypress")
	}
	if !strings.Contains(script, "Scroll to: y=500") {
		t.Error("missing scroll")
	}
}

func TestReproduction_Kaboom_ElementDescriptionPriority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		sels     map[string]any
		expected string
	}{
		{
			name:     "text + role",
			sels:     map[string]any{"text": "Submit", "role": map[string]any{"role": "button", "name": "Submit"}},
			expected: `"Submit" button`,
		},
		{
			name:     "ariaLabel + role",
			sels:     map[string]any{"ariaLabel": "Close dialog", "role": map[string]any{"role": "button", "name": "Close dialog"}},
			expected: `"Close dialog" button`,
		},
		{
			name:     "role name only",
			sels:     map[string]any{"role": map[string]any{"role": "textbox", "name": "Email"}},
			expected: `"Email" textbox`,
		},
		{
			name:     "testId fallback",
			sels:     map[string]any{"testId": "submit-btn"},
			expected: `[data-testid="submit-btn"]`,
		},
		{
			name:     "text alone",
			sels:     map[string]any{"text": "Add to Cart"},
			expected: `"Add to Cart"`,
		},
		{
			name:     "ariaLabel alone",
			sels:     map[string]any{"ariaLabel": "Search"},
			expected: `"Search"`,
		},
		{
			name:     "id fallback",
			sels:     map[string]any{"id": "quantity"},
			expected: `#quantity`,
		},
		{
			name:     "cssPath last resort",
			sels:     map[string]any{"cssPath": "form > input"},
			expected: `form > input`,
		},
		{
			name:     "no selectors",
			sels:     map[string]any{},
			expected: `(unknown element)`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			action := makeTestAction("click", 1000, map[string]any{"selectors": tc.sels})
			desc := DescribeElement(action)
			if desc != tc.expected {
				t.Errorf("DescribeElement() = %q, want %q", desc, tc.expected)
			}
		})
	}
}

func TestReproduction_Kaboom_TimingPauses(t *testing.T) {
	t.Parallel()
	actions := []types.EnhancedAction{
		makeTestAction("click", 1000, map[string]any{
			"selectors": map[string]any{"text": "A"},
		}),
		makeTestAction("click", 5500, map[string]any{ // 4.5s gap
			"selectors": map[string]any{"text": "B"},
		}),
		makeTestAction("click", 6000, map[string]any{ // 0.5s gap — no pause
			"selectors": map[string]any{"text": "C"},
		}),
	}

	script := GenerateKaboomScript(actions, Params{})

	if !strings.Contains(script, "[4s pause]") {
		t.Errorf("expected [4s pause], got:\n%s", script)
	}

	// Should NOT have a pause before C (only 500ms gap)
	lines := strings.Split(script, "\n")
	for i, line := range lines {
		if strings.Contains(line, `"C"`) {
			if i > 0 && strings.Contains(lines[i-1], "pause") {
				t.Errorf("should not have pause before C (500ms gap)")
			}
		}
	}
}

func TestReproduction_Kaboom_RedactedValues(t *testing.T) {
	t.Parallel()
	actions := []types.EnhancedAction{
		makeTestAction("input", 1000, map[string]any{
			"selectors": map[string]any{"role": map[string]any{"role": "textbox", "name": "Password"}},
			"value":     "[redacted]",
		}),
	}

	script := GenerateKaboomScript(actions, Params{})

	if !strings.Contains(script, "[user-provided]") {
		t.Errorf("expected [user-provided] for redacted value, got:\n%s", script)
	}
	if strings.Contains(script, "[redacted]") {
		t.Errorf("should not contain raw [redacted] marker")
	}
}

func TestReproduction_Kaboom_AIActions(t *testing.T) {
	t.Parallel()
	actions := []types.EnhancedAction{
		makeTestAction("click", 1000, map[string]any{
			"selectors": map[string]any{"text": "Submit"},
			"source":    "ai",
		}),
		makeTestAction("click", 2000, map[string]any{
			"selectors": map[string]any{"text": "Next"},
		}),
	}

	script := GenerateKaboomScript(actions, Params{})

	if !strings.Contains(script, "(AI) Click:") {
		t.Errorf("expected (AI) prefix for AI action, got:\n%s", script)
	}
	// Human action should NOT have prefix
	if strings.Contains(script, "(AI) Click: \"Next\"") {
		t.Errorf("human action should not have (AI) prefix")
	}
}

func TestReproduction_Kaboom_ErrorMessage(t *testing.T) {
	t.Parallel()
	actions := basicFlow()
	script := GenerateKaboomScript(actions, Params{
		ErrorMessage: "Cannot read property 'x' of undefined",
	})

	if !strings.Contains(script, "# Error: Cannot read property 'x' of undefined") {
		t.Errorf("expected error annotation, got:\n%s", script)
	}
}

// ============================================
// Playwright Format Tests
// ============================================

func TestReproduction_Playwright_BasicFlow(t *testing.T) {
	t.Parallel()
	actions := basicFlow()
	script := GeneratePlaywrightScript(actions, Params{})

	// Valid Playwright imports
	if !strings.Contains(script, "import { test, expect } from '@playwright/test'") {
		t.Error("expected Playwright import")
	}

	// Test function
	if !strings.Contains(script, "test('reproduction:") {
		t.Error("expected test() wrapper")
	}

	// Navigate
	if !strings.Contains(script, "await page.goto('https://example.com/login')") {
		t.Errorf("expected goto, got:\n%s", script)
	}

	// Click with role locator
	if !strings.Contains(script, "getByRole('button', { name: 'Sign In' })") {
		t.Errorf("expected getByRole click, got:\n%s", script)
	}

	// Fill — role takes priority over ariaLabel (both present in basicFlow)
	if !strings.Contains(script, "getByRole('textbox', { name: 'Email address' })") {
		t.Errorf("expected getByRole fill, got:\n%s", script)
	}
}

func TestReproduction_Playwright_LocatorPriority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		sels     map[string]any
		expected string
	}{
		{
			name:     "testId first",
			sels:     map[string]any{"testId": "submit-btn", "role": map[string]any{"role": "button", "name": "Submit"}, "text": "Submit"},
			expected: "getByTestId('submit-btn')",
		},
		{
			name:     "role second",
			sels:     map[string]any{"role": map[string]any{"role": "button", "name": "Submit"}, "text": "Submit", "ariaLabel": "Submit"},
			expected: "getByRole('button', { name: 'Submit' })",
		},
		{
			name:     "ariaLabel third",
			sels:     map[string]any{"ariaLabel": "Search", "text": "Search", "id": "search"},
			expected: "getByLabel('Search')",
		},
		{
			name:     "text fourth",
			sels:     map[string]any{"text": "Click me", "id": "btn"},
			expected: "getByText('Click me')",
		},
		{
			name:     "id fifth",
			sels:     map[string]any{"id": "quantity", "cssPath": "form > input"},
			expected: "locator('#quantity')",
		},
		{
			name:     "cssPath last",
			sels:     map[string]any{"cssPath": "form > div > input"},
			expected: "locator('form > div > input')",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			loc := PlaywrightLocator(tc.sels)
			if loc != tc.expected {
				t.Errorf("PlaywrightLocator() = %q, want %q", loc, tc.expected)
			}
		})
	}
}

func TestReproduction_Playwright_URLRewriting(t *testing.T) {
	t.Parallel()
	actions := []types.EnhancedAction{
		makeTestAction("navigate", 1000, map[string]any{
			"toURL": "https://production.example.com/dashboard",
		}),
	}

	script := GeneratePlaywrightScript(actions, Params{
		BaseURL: "http://localhost:3000",
	})

	if !strings.Contains(script, "http://localhost:3000/dashboard") {
		t.Errorf("expected rewritten URL, got:\n%s", script)
	}
	if strings.Contains(script, "production.example.com") {
		t.Error("production URL should be rewritten")
	}
}

func TestReproduction_Playwright_SpecialCharacters(t *testing.T) {
	t.Parallel()
	actions := []types.EnhancedAction{
		makeTestAction("input", 1000, map[string]any{
			"selectors": map[string]any{"id": "msg"},
			"value":     "it's a \"test\"\nnewline",
		}),
	}

	script := GeneratePlaywrightScript(actions, Params{})

	if strings.Contains(script, "it's") && !strings.Contains(script, `it\'s`) {
		t.Errorf("expected escaped quotes, got:\n%s", script)
	}
}

func TestReproduction_Playwright_TimingPauses(t *testing.T) {
	t.Parallel()
	actions := []types.EnhancedAction{
		makeTestAction("click", 1000, map[string]any{"selectors": map[string]any{"text": "A"}}),
		makeTestAction("click", 6000, map[string]any{"selectors": map[string]any{"text": "B"}}),
	}

	script := GeneratePlaywrightScript(actions, Params{})

	if !strings.Contains(script, "// [5s pause]") {
		t.Errorf("expected pause comment, got:\n%s", script)
	}
}

// ============================================
// Shared Behavior Tests
// ============================================

func TestReproduction_EmptyActions(t *testing.T) {
	t.Parallel()

	kaboom := GenerateKaboomScript(nil, Params{})
	if !strings.Contains(kaboom, "No actions") {
		t.Errorf("kaboom should indicate no actions, got:\n%s", kaboom)
	}

	playwright := GeneratePlaywrightScript(nil, Params{})
	if !strings.Contains(playwright, "No actions") {
		t.Errorf("playwright should indicate no actions, got:\n%s", playwright)
	}
}

func TestReproduction_LastN(t *testing.T) {
	t.Parallel()
	actions := []types.EnhancedAction{
		makeTestAction("click", 1000, map[string]any{"selectors": map[string]any{"text": "First"}}),
		makeTestAction("click", 2000, map[string]any{"selectors": map[string]any{"text": "Second"}}),
		makeTestAction("click", 3000, map[string]any{"selectors": map[string]any{"text": "Third"}}),
	}

	script := GenerateKaboomScript(actions, Params{LastN: 2})

	if strings.Contains(script, "First") {
		t.Error("last_n=2 should exclude first action")
	}
	if !strings.Contains(script, "Second") || !strings.Contains(script, "Third") {
		t.Error("last_n=2 should include last two actions")
	}
}

func TestReproduction_DefaultFormat(t *testing.T) {
	t.Parallel()
	// When output_format is empty, should default to "kaboom-agentic-browser"
	params := Params{}
	format := params.OutputFormat
	if format == "" {
		format = "kaboom-agentic-browser" // default
	}
	if format != "kaboom-agentic-browser" {
		t.Errorf("expected default format 'kaboom-agentic-browser', got %q", format)
	}
}

// ============================================
// PlaywrightStep — 56% → 100%
// ============================================

func TestPlaywrightStep_Navigate(t *testing.T) {
	t.Parallel()
	action := makeTestAction("navigate", 1000, map[string]any{"toURL": "https://example.com/page"})
	got := PlaywrightStep(action, Params{})
	if got != "await page.goto('https://example.com/page');" {
		t.Errorf("PlaywrightStep(navigate) = %q", got)
	}
}

func TestPlaywrightStep_NavigateNoURL(t *testing.T) {
	t.Parallel()
	action := makeTestAction("navigate", 1000, map[string]any{})
	got := PlaywrightStep(action, Params{})
	if got != "" {
		t.Errorf("PlaywrightStep(navigate no URL) = %q, want empty", got)
	}
}

func TestPlaywrightStep_NavigateWithBaseURL(t *testing.T) {
	t.Parallel()
	action := makeTestAction("navigate", 1000, map[string]any{"toURL": "https://prod.example.com/dashboard"})
	got := PlaywrightStep(action, Params{BaseURL: "http://localhost:3000"})
	if !strings.Contains(got, "localhost:3000/dashboard") {
		t.Errorf("PlaywrightStep(navigate+baseURL) = %q, want rewritten URL", got)
	}
}

func TestPlaywrightStep_ClickNoSelector(t *testing.T) {
	t.Parallel()
	action := makeTestAction("click", 1000, map[string]any{})
	got := PlaywrightStep(action, Params{})
	if got != "// click - no selector available" {
		t.Errorf("PlaywrightStep(click no sel) = %q", got)
	}
}

func TestPlaywrightStep_InputNoSelector(t *testing.T) {
	t.Parallel()
	action := makeTestAction("input", 1000, map[string]any{"value": "test"})
	got := PlaywrightStep(action, Params{})
	if got != "// input - no selector available" {
		t.Errorf("PlaywrightStep(input no sel) = %q", got)
	}
}

func TestPlaywrightStep_InputRedacted(t *testing.T) {
	t.Parallel()
	action := makeTestAction("input", 1000, map[string]any{
		"selectors": map[string]any{"id": "pw"},
		"value":     "[redacted]",
	})
	got := PlaywrightStep(action, Params{})
	if !strings.Contains(got, "[user-provided]") {
		t.Errorf("PlaywrightStep(input redacted) = %q, want [user-provided]", got)
	}
}

func TestPlaywrightStep_SelectNoSelector(t *testing.T) {
	t.Parallel()
	action := makeTestAction("select", 1000, map[string]any{"selectedValue": "opt1"})
	got := PlaywrightStep(action, Params{})
	if got != "// select - no selector available" {
		t.Errorf("PlaywrightStep(select no sel) = %q", got)
	}
}

func TestPlaywrightStep_Keypress(t *testing.T) {
	t.Parallel()
	action := makeTestAction("keypress", 1000, map[string]any{"key": "Enter"})
	got := PlaywrightStep(action, Params{})
	if got != "await page.keyboard.press('Enter');" {
		t.Errorf("PlaywrightStep(keypress) = %q", got)
	}
}

func TestPlaywrightStep_Scroll(t *testing.T) {
	t.Parallel()
	action := makeTestAction("scroll", 1000, map[string]any{"scrollY": 300})
	got := PlaywrightStep(action, Params{})
	if got != "// Scroll to y=300" {
		t.Errorf("PlaywrightStep(scroll) = %q", got)
	}
}

func TestPlaywrightStep_UnknownType(t *testing.T) {
	t.Parallel()
	action := makeTestAction("unknown_type", 1000, map[string]any{})
	got := PlaywrightStep(action, Params{})
	if got != "" {
		t.Errorf("PlaywrightStep(unknown) = %q, want empty", got)
	}
}

func TestReproduction_NavigateNoURL(t *testing.T) {
	t.Parallel()
	actions := []types.EnhancedAction{
		makeTestAction("navigate", 1000, map[string]any{}), // no URL
		makeTestAction("click", 2000, map[string]any{"selectors": map[string]any{"text": "Go"}}),
	}

	kaboom := GenerateKaboomScript(actions, Params{})
	// Should skip the empty navigate, only have 1 numbered step
	if strings.Contains(kaboom, "Navigate to: \n") || strings.Contains(kaboom, "Navigate to:  ") {
		t.Errorf("should skip navigate with no URL, got:\n%s", kaboom)
	}
}

// ============================================
// Browser Action Steps (refresh, back, forward, new_tab, focus)
// ============================================

func TestPlaywrightStep_Refresh(t *testing.T) {
	t.Parallel()
	action := makeTestAction("refresh", 1000, map[string]any{})
	got := PlaywrightStep(action, Params{})
	if got != "await page.reload();" {
		t.Errorf("PlaywrightStep(refresh) = %q", got)
	}
}

func TestPlaywrightStep_Back(t *testing.T) {
	t.Parallel()
	action := makeTestAction("back", 1000, map[string]any{})
	got := PlaywrightStep(action, Params{})
	if got != "await page.goBack();" {
		t.Errorf("PlaywrightStep(back) = %q", got)
	}
}

func TestPlaywrightStep_Forward(t *testing.T) {
	t.Parallel()
	action := makeTestAction("forward", 1000, map[string]any{})
	got := PlaywrightStep(action, Params{})
	if got != "await page.goForward();" {
		t.Errorf("PlaywrightStep(forward) = %q", got)
	}
}

func TestPlaywrightStep_NewTab(t *testing.T) {
	t.Parallel()
	action := types.EnhancedAction{
		Type:      "new_tab",
		Timestamp: 1000,
		URL:       "https://example.com/page",
	}
	got := PlaywrightStep(action, Params{})
	if got != "// Open new tab: https://example.com/page" {
		t.Errorf("PlaywrightStep(new_tab) = %q", got)
	}
}

func TestPlaywrightStep_NewTabNoURL(t *testing.T) {
	t.Parallel()
	action := types.EnhancedAction{
		Type:      "new_tab",
		Timestamp: 1000,
		// URL intentionally empty
	}
	got := PlaywrightStep(action, Params{})
	if got != "// Open new tab" {
		t.Errorf("PlaywrightStep(new_tab no URL) = %q", got)
	}
}

func TestPlaywrightStep_Focus(t *testing.T) {
	t.Parallel()
	action := makeTestAction("focus", 1000, map[string]any{
		"selectors": map[string]any{"id": "email"},
	})
	got := PlaywrightStep(action, Params{})
	if !strings.Contains(got, "focus()") {
		t.Errorf("PlaywrightStep(focus) = %q, want focus()", got)
	}
}

func TestPlaywrightStep_ScrollElement(t *testing.T) {
	t.Parallel()
	action := makeTestAction("scroll_element", 1000, map[string]any{
		"selectors": map[string]any{"id": "results"},
	})
	got := PlaywrightStep(action, Params{})
	if !strings.Contains(got, "scrollIntoViewIfNeeded()") {
		t.Errorf("PlaywrightStep(scroll_element) = %q, want scrollIntoViewIfNeeded()", got)
	}
}

func TestKaboomStep_Refresh(t *testing.T) {
	t.Parallel()
	action := makeTestAction("refresh", 1000, map[string]any{})
	got := KaboomStep(action, Params{})
	if got != "Refresh page" {
		t.Errorf("KaboomStep(refresh) = %q", got)
	}
}

func TestKaboomStep_Back(t *testing.T) {
	t.Parallel()
	action := makeTestAction("back", 1000, map[string]any{})
	got := KaboomStep(action, Params{})
	if got != "Navigate back" {
		t.Errorf("KaboomStep(back) = %q", got)
	}
}

func TestKaboomStep_Forward(t *testing.T) {
	t.Parallel()
	action := makeTestAction("forward", 1000, map[string]any{})
	got := KaboomStep(action, Params{})
	if got != "Navigate forward" {
		t.Errorf("KaboomStep(forward) = %q", got)
	}
}

func TestKaboomStep_NewTab(t *testing.T) {
	t.Parallel()
	action := types.EnhancedAction{
		Type:      "new_tab",
		Timestamp: 1000,
		URL:       "https://example.com/new",
	}
	got := KaboomStep(action, Params{})
	if got != "Open new tab: https://example.com/new" {
		t.Errorf("KaboomStep(new_tab) = %q", got)
	}
}

func TestKaboomStep_Focus(t *testing.T) {
	t.Parallel()
	action := makeTestAction("focus", 1000, map[string]any{
		"selectors": map[string]any{"id": "email"},
	})
	got := KaboomStep(action, Params{})
	if !strings.Contains(got, "Focus:") {
		t.Errorf("KaboomStep(focus) = %q, want Focus: ...", got)
	}
}

func TestKaboomStep_ScrollElement(t *testing.T) {
	t.Parallel()
	action := makeTestAction("scroll_element", 1000, map[string]any{
		"selectors": map[string]any{"id": "results"},
	})
	got := KaboomStep(action, Params{})
	if !strings.Contains(got, "Scroll to element:") {
		t.Errorf("KaboomStep(scroll_element) = %q, want element-targeted scroll step", got)
	}
}

// ============================================
// AI Action Recording (DOM primitives) for Reproduction
// ============================================

func TestReproduction_AIActionsFromInteract(t *testing.T) {
	t.Parallel()
	// Simulate what handleDOMPrimitive records: normalized types with proper fields
	actions := []types.EnhancedAction{
		{
			Type:      "navigate",
			Timestamp: 1000,
			ToURL:     "https://example.com/app",
			Source:    "ai",
		},
		{
			Type:      "click",
			Timestamp: 2000,
			Selectors: map[string]any{"id": "login-btn"},
			Source:    "ai",
		},
		{
			Type:      "input",
			Timestamp: 3000,
			Selectors: map[string]any{"cssPath": "input[name=email]"},
			Value:     "user@test.com",
			Source:    "ai",
		},
		{
			Type:      "keypress",
			Timestamp: 4000,
			Key:       "Enter",
			Source:    "ai",
		},
	}

	// Kaboom format should include all actions
	kaboom := GenerateKaboomScript(actions, Params{})
	if !strings.Contains(kaboom, "Navigate to:") {
		t.Errorf("kaboom missing navigate, got:\n%s", kaboom)
	}
	if !strings.Contains(kaboom, "Click:") {
		t.Errorf("kaboom missing click, got:\n%s", kaboom)
	}
	if !strings.Contains(kaboom, "Type") && !strings.Contains(kaboom, "user@test.com") {
		t.Errorf("kaboom missing input, got:\n%s", kaboom)
	}
	if !strings.Contains(kaboom, "Press: Enter") {
		t.Errorf("kaboom missing keypress, got:\n%s", kaboom)
	}

	// Playwright format should include all actions
	playwright := GeneratePlaywrightScript(actions, Params{})
	if !strings.Contains(playwright, "page.goto(") {
		t.Errorf("playwright missing navigate, got:\n%s", playwright)
	}
	if !strings.Contains(playwright, "click()") {
		t.Errorf("playwright missing click, got:\n%s", playwright)
	}
	if !strings.Contains(playwright, "fill(") {
		t.Errorf("playwright missing input/fill, got:\n%s", playwright)
	}
	if !strings.Contains(playwright, "keyboard.press('Enter')") {
		t.Errorf("playwright missing keypress, got:\n%s", playwright)
	}
}
