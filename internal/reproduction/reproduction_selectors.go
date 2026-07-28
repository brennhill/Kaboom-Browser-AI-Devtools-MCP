// Purpose: Describes target elements using the most human-readable selector available.
// Why: Separates element description heuristics from script generation logic.
package reproduction

import (
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

type elementCandidate struct {
	label string
	ok    bool
}

// DescribeElement returns the most human-readable description of the target element.
// Priority: text+role > ariaLabel+role > role.name+role > testId > text > ariaLabel > id > cssPath
func DescribeElement(action types.EnhancedAction) string {
	s := action.Selectors
	if s == nil {
		return "(unknown element)"
	}
	text := selectorStr(s, "text")
	ariaLabel := selectorStr(s, "ariaLabel")
	testID := selectorStr(s, "testId")
	id := selectorStr(s, "id")
	cssPath := selectorStr(s, "cssPath")
	role, roleName := selectorRole(s)

	desc := describeWithRole(text, ariaLabel, roleName, role)
	if desc != "" {
		return desc
	}
	return describeWithoutRole(testID, text, ariaLabel, id, cssPath)
}

func describeWithRole(text, ariaLabel, roleName, role string) string {
	if role == "" {
		return ""
	}
	candidates := []elementCandidate{
		{text, text != ""},
		{ariaLabel, ariaLabel != ""},
		{roleName, roleName != ""},
	}
	for _, c := range candidates {
		if c.ok {
			return fmt.Sprintf("%q %s", c.label, role)
		}
	}
	return ""
}

func describeWithoutRole(testID, text, ariaLabel, id, cssPath string) string {
	if testID != "" {
		return fmt.Sprintf("[data-testid=%q]", testID)
	}
	if text != "" {
		return fmt.Sprintf("%q", text)
	}
	if ariaLabel != "" {
		return fmt.Sprintf("%q", ariaLabel)
	}
	if id != "" {
		return "#" + id
	}
	if cssPath != "" {
		return cssPath
	}
	return "(unknown element)"
}

type pwLocatorCandidate struct {
	value  string
	format func(string) string
}

// PlaywrightLocator returns the best Playwright locator string for a selector map.
// Priority: testId > role > ariaLabel > text > id > cssPath
func PlaywrightLocator(selectors map[string]any) string {
	if selectors == nil {
		return ""
	}

	// Role has special handling for optional name parameter.
	role, roleName := selectorRole(selectors)
	if loc := pwRoleLocator(role, roleName); loc != "" {
		// Role is priority 2; check testId first.
		testID := selectorStr(selectors, "testId")
		if testID != "" {
			return fmt.Sprintf("getByTestId('%s')", EscapeJS(testID))
		}
		return loc
	}

	candidates := []pwLocatorCandidate{
		{selectorStr(selectors, "testId"), func(v string) string { return fmt.Sprintf("getByTestId('%s')", EscapeJS(v)) }},
		{selectorStr(selectors, "ariaLabel"), func(v string) string { return fmt.Sprintf("getByLabel('%s')", EscapeJS(v)) }},
		{selectorStr(selectors, "text"), func(v string) string { return fmt.Sprintf("getByText('%s')", EscapeJS(v)) }},
		{selectorStr(selectors, "id"), func(v string) string { return fmt.Sprintf("locator('#%s')", EscapeJS(v)) }},
		{selectorStr(selectors, "cssPath"), func(v string) string { return fmt.Sprintf("locator('%s')", EscapeJS(v)) }},
	}
	for _, c := range candidates {
		if c.value != "" {
			return c.format(c.value)
		}
	}
	return ""
}

func pwRoleLocator(role, roleName string) string {
	if role == "" {
		return ""
	}
	if roleName != "" {
		return fmt.Sprintf("getByRole('%s', { name: '%s' })", EscapeJS(role), EscapeJS(roleName))
	}
	return fmt.Sprintf("getByRole('%s')", EscapeJS(role))
}

// selectorStr extracts a string value from the selectors map.
func selectorStr(selectors map[string]any, key string) string {
	v, ok := selectors[key].(string)
	if !ok {
		return ""
	}
	return v
}

// selectorRole extracts role and name from the selectors map.
func selectorRole(selectors map[string]any) (role, name string) {
	roleData, ok := selectors["role"]
	if !ok {
		return "", ""
	}
	roleMap, ok := roleData.(map[string]any)
	if !ok {
		return "", ""
	}
	role, _ = roleMap["role"].(string)
	name, _ = roleMap["name"].(string)
	return role, name
}
