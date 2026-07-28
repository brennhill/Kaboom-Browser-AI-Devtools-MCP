// Purpose: Pure helper functions for test generation: filename sanitization, selector extraction, Playwright script building.
// Docs: docs/features/feature/test-generation/index.md

package testgen

import (
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"regexp"
	"strings"
	"time"
)

// filenameAllowlistRe matches any character NOT in the safe set [a-z0-9-].
var filenameAllowlistRe = regexp.MustCompile(`[^a-z0-9]+`)

var windowsReservedFilenames = map[string]struct{}{
	"con": {}, "prn": {}, "aux": {}, "nul": {},
	"com1": {}, "com2": {}, "com3": {}, "com4": {}, "com5": {},
	"com6": {}, "com7": {}, "com8": {}, "com9": {},
	"lpt1": {}, "lpt2": {}, "lpt3": {}, "lpt4": {}, "lpt5": {},
	"lpt6": {}, "lpt7": {}, "lpt8": {}, "lpt9": {},
}

// GenerateTestFilename creates a sanitized test filename from input and framework.
func GenerateTestFilename(input, framework string) string {
	name := strings.ToLower(strings.TrimSpace(input))
	name = filenameAllowlistRe.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")

	if len(name) > 50 {
		name = name[:50]
		if i := strings.LastIndex(name, "-"); i > 0 {
			name = name[:i]
		}
	}
	name = strings.TrimRight(name, "-")

	if name == "" {
		name = "generated-test"
	}
	if _, reserved := windowsReservedFilenames[name]; reserved {
		name = "test-" + name
	}

	ext := ".spec.ts"
	if framework == "vitest" || framework == "jest" {
		ext = ".test.ts"
	}

	return name + ext
}

// ExtractSelectorsFromActions extracts unique CSS selectors from action entries.
func ExtractSelectorsFromActions(actions []types.EnhancedAction) []string {
	selectorSet := make(map[string]bool)
	for i := range actions {
		addSelectorsFromEntry(selectorSet, actions[i].Selectors)
	}

	var result []string
	for selector := range selectorSet {
		result = append(result, selector)
	}
	return result
}

func addSelectorsFromEntry(selectorSet map[string]bool, selectors map[string]any) {
	if selectors == nil {
		return
	}
	if testID, ok := selectors["testId"].(string); ok && testID != "" {
		selectorSet["[data-testid=\""+testID+"\"]"] = true
	}
	if role := extractRoleFromSelectors(selectors); role != "" {
		selectorSet["[role=\""+role+"\"]"] = true
	}
	if id, ok := selectors["id"].(string); ok && id != "" {
		selectorSet["#"+id] = true
	}
}

func extractRoleFromSelectors(selectors map[string]any) string {
	roleData, ok := selectors["role"]
	if !ok {
		return ""
	}
	roleMap, ok := roleData.(map[string]any)
	if !ok {
		return ""
	}
	role, _ := roleMap["role"].(string)
	return role
}

// NormalizeTimestamp parses an RFC3339 timestamp string and returns milliseconds.
func NormalizeTimestamp(tsStr string) int64 {
	t, err := time.Parse(time.RFC3339, tsStr)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, tsStr)
		if err != nil {
			return 0
		}
	}
	return t.UnixMilli()
}

// DeriveInteractionTestName derives a test name from the first action's URL or type.
func DeriveInteractionTestName(actions []types.EnhancedAction) string {
	if len(actions) == 0 {
		return "user-interaction"
	}
	if actions[0].URL != "" {
		return actions[0].URL
	}
	if actions[0].Type != "" {
		return actions[0].Type + "-flow"
	}
	return "user-interaction"
}

// BuildRegressionAssertions builds assertion lines for regression tests.
func BuildRegressionAssertions(errorMessages []string, networkBodies []types.NetworkBody) ([]string, int) {
	var assertions []string
	assertionCount := 0

	if len(errorMessages) == 0 {
		assertions = append(assertions,
			"  // Assert no console errors (baseline was clean)",
			"  const consoleErrors = []",
			"  page.on('console', msg => { if (msg.type() === 'error') consoleErrors.push(msg.text()) })",
			"  // After actions complete:",
			"  expect(consoleErrors).toHaveLength(0)",
		)
		assertionCount++
	} else {
		assertions = append(assertions,
			fmt.Sprintf("  // Baseline had %d console errors", len(errorMessages)),
			"  // TODO: Add assertions to verify errors haven't changed",
		)
	}

	networkAssertions := 0
	for _, nb := range networkBodies {
		if nb.Status > 0 && networkAssertions < 3 {
			assertions = append(assertions,
				fmt.Sprintf("  // Assert %s %s returns %d", nb.Method, nb.URL, nb.Status),
				fmt.Sprintf("  // TODO: await page.waitForResponse(r => r.url().includes('%s') && r.status() === %d)", nb.URL, nb.Status),
			)
			networkAssertions++
			assertionCount++
		}
	}

	assertions = append(assertions,
		"",
		"  // TODO: Add performance assertions",
		"  // - Load time within acceptable range",
		"  // - Key metrics (FCP, LCP) haven't regressed",
	)

	return assertions, assertionCount
}

// InsertAssertionsBeforeClose inserts assertion lines before the final }); in a script.
func InsertAssertionsBeforeClose(script string, assertions []string) string {
	assertionBlock := strings.Join(assertions, "\n")
	lastBrace := strings.LastIndex(script, "});")
	if lastBrace > 0 {
		return script[:lastBrace] + "\n" + assertionBlock + "\n" + script[lastBrace:]
	}
	return script
}
