// Purpose: Generates Playwright test scripts from captured browser actions.
// Why: Separates Playwright-specific code generation from the Kaboom-native step format.
package reproduction

import (
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strings"
)

// GeneratePlaywrightScript converts actions to a Playwright test script.
func GeneratePlaywrightScript(actions []types.EnhancedAction, opts Params) string {
	if len(actions) == 0 {
		return "// No actions captured\n"
	}
	actions = FilterLastN(actions, opts.LastN)

	var b strings.Builder
	writePlaywrightHeader(&b, opts)
	writePlaywrightSteps(&b, actions, opts)
	writePlaywrightFooter(&b, opts)

	script := b.String()
	if len(script) > maxReproOutputBytes {
		script = script[:maxReproOutputBytes]
	}
	return script
}

func writePlaywrightHeader(b *strings.Builder, opts Params) {
	b.WriteString("import { test, expect } from '@playwright/test';\n\n")
	testName := "reproduction: captured user actions"
	if opts.ErrorMessage != "" {
		testName = "reproduction: " + ChopString(opts.ErrorMessage, 80)
	}
	fmt.Fprintf(b, "test('%s', async ({ page }) => {\n", EscapeJS(testName))
}

func writePlaywrightSteps(b *strings.Builder, actions []types.EnhancedAction, opts Params) {
	var prevTs int64
	for _, action := range actions {
		WritePauseComment(b, prevTs, action.Timestamp, "  // [%ds pause]\n")
		prevTs = action.Timestamp
		line := PlaywrightStep(action, opts)
		if line != "" {
			b.WriteString("  " + line + "\n")
		}
	}
}

func writePlaywrightFooter(b *strings.Builder, opts Params) {
	if opts.ErrorMessage != "" {
		fmt.Fprintf(b, "  // Error: %s\n", opts.ErrorMessage)
	}
	b.WriteString("});\n")
}

// PlaywrightStep converts a single action to a Playwright code line.
func PlaywrightStep(action types.EnhancedAction, opts Params) string {
	switch action.Type {
	case "navigate":
		return pwNavigateStep(action, opts)
	case "click":
		return pwLocatorAction(action, "click", "click")
	case "input":
		return pwInputStep(action)
	case "select":
		return pwSelectStep(action)
	case "keypress":
		return fmt.Sprintf("await page.keyboard.press('%s');", EscapeJS(action.Key))
	case "scroll":
		return fmt.Sprintf("// Scroll to y=%d", action.ScrollY)
	case "scroll_element":
		return pwLocatorAction(action, "scrollIntoViewIfNeeded", "scroll element into view")
	case "refresh":
		return "await page.reload();"
	case "back":
		return "await page.goBack();"
	case "forward":
		return "await page.goForward();"
	case "new_tab":
		return pwNewTabStep(action, opts)
	case "focus":
		return pwLocatorAction(action, "focus", "focus")
	default:
		return ""
	}
}

func pwNavigateStep(action types.EnhancedAction, opts Params) string {
	toURL := action.ToURL
	if toURL == "" {
		return ""
	}
	if opts.BaseURL != "" {
		toURL = rewriteURL(toURL, opts.BaseURL)
	}
	return fmt.Sprintf("await page.goto('%s');", EscapeJS(toURL))
}

func pwNewTabStep(action types.EnhancedAction, opts Params) string {
	targetURL := action.URL
	if targetURL == "" {
		return "// Open new tab"
	}
	if opts.BaseURL != "" {
		targetURL = rewriteURL(targetURL, opts.BaseURL)
	}
	return fmt.Sprintf("// Open new tab: %s", EscapeJS(targetURL))
}

func pwLocatorAction(action types.EnhancedAction, actionName, fallbackLabel string) string {
	loc := PlaywrightLocator(action.Selectors)
	if loc == "" {
		return fmt.Sprintf("// %s - no selector available", fallbackLabel)
	}
	return fmt.Sprintf("await page.%s.%s();", loc, actionName)
}

func pwInputStep(action types.EnhancedAction) string {
	loc := PlaywrightLocator(action.Selectors)
	if loc == "" {
		return "// input - no selector available"
	}
	value := action.Value
	if value == "[redacted]" {
		value = "[user-provided]"
	}
	return fmt.Sprintf("await page.%s.fill('%s');", loc, EscapeJS(value))
}

func pwSelectStep(action types.EnhancedAction) string {
	loc := PlaywrightLocator(action.Selectors)
	if loc == "" {
		return "// select - no selector available"
	}
	return fmt.Sprintf("await page.%s.selectOption('%s');", loc, EscapeJS(action.SelectedValue))
}
