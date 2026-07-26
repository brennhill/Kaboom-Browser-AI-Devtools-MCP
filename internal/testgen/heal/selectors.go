// Purpose: Detects dangerous patterns in selectors and finds selectors in test file content.
// Why: Separates selector scanning and safety checks from repair and batch logic.
package heal

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ContainsDangerousPattern checks if a selector contains dangerous patterns.
func ContainsDangerousPattern(selector string) (string, bool) {
	dangerous := []string{"javascript:", "<script", "onerror=", "onload="}
	lowerSelector := strings.ToLower(selector)
	for _, pattern := range dangerous {
		if strings.Contains(lowerSelector, pattern) {
			return pattern, true
		}
	}
	return "", false
}

// validSelectorStartChars contains characters that may begin a CSS selector.
const validSelectorStartChars = "#.[*"

func isValidSelectorStart(ch byte) bool {
	c := rune(ch)
	if c >= 'a' && c <= 'z' {
		return true
	}
	if c >= 'A' && c <= 'Z' {
		return true
	}
	return strings.ContainsRune(validSelectorStartChars, c)
}

// ValidateSelector validates a CSS selector for safety and correctness.
func ValidateSelector(selector string) error {
	if len(selector) > 1000 {
		return fmt.Errorf(ErrInvalidSelectorSyntax + ": selector exceeds 1000 characters")
	}
	if selector == "" {
		return fmt.Errorf(ErrInvalidSelectorSyntax + ": selector is empty")
	}
	if pattern, found := ContainsDangerousPattern(selector); found {
		return fmt.Errorf("%s: selector contains dangerous pattern: %s", ErrSelectorInjection, pattern)
	}
	if !isValidSelectorStart(selector[0]) {
		return fmt.Errorf(ErrInvalidSelectorSyntax + ": selector must start with valid CSS selector character")
	}
	return nil
}

// ExtractSelectorsFromTestFile extracts selectors from test file content.
func ExtractSelectorsFromTestFile(content string) []string {
	var selectors []string
	seen := make(map[string]bool)

	patterns := []string{
		`getByTestId\(['"]([^'"]+)['"]\)`,
		`locator\(['"]([^'"]+)['"]\)`,
		`getByRole\(['"]([^'"]+)['"]\)`,
		`getByLabel\(['"]([^'"]+)['"]\)`,
		`getByText\(['"]([^'"]+)['"]\)`,
		`querySelector\(['"]([^'"]+)['"]\)`,
		`querySelectorAll\(['"]([^'"]+)['"]\)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 && !seen[match[1]] {
				selectors = append(selectors, match[1])
				seen[match[1]] = true
			}
		}
	}

	return selectors
}

// AnalyzeTestFile reads a test file and extracts selectors from it.
func AnalyzeTestFile(req TestHealRequest, projectDir string) ([]string, error) {
	if err := ValidateTestFilePath(req.TestFile, projectDir); err != nil {
		return nil, err
	}

	fullPath := ResolveTestPath(req.TestFile, projectDir)

	content, err := os.ReadFile(fullPath) // #nosec G304 -- path validated above // nosemgrep: go_filesystem_rule-fileread -- CLI tool reads user test file for healing
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s: %s", ErrTestFileNotFound, req.TestFile)
		}
		return nil, fmt.Errorf("failed to read test file: %w", err)
	}

	selectors := ExtractSelectorsFromTestFile(string(content))
	return selectors, nil
}
