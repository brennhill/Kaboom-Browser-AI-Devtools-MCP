// Purpose: Fuzz file upload operations for edge-case inputs and panics.
// Docs: docs/features/feature/file-upload/index.md

// validators_fuzz_test.go — Fuzz tests for OS automation path validation and sanitizers.
package uploadsec

import (
	"strings"
	"testing"
)

func FuzzValidatePathForOSAutomation(f *testing.F) {
	f.Add("/Users/test/file.pdf")
	f.Add("C:\\Users\\test\\file.pdf")
	f.Add("/tmp/upload.txt")
	f.Add("./relative/path.doc")
	f.Add("path\x00evil")
	f.Add("path\nevil")
	f.Add("path\revil")
	f.Add("path`evil")
	f.Add("\"; do shell script \"rm -rf /\"")
	f.Add("$(whoami)")
	f.Add("path\r\nevil")
	f.Add("`rm -rf /`")
	f.Add("path\x00\x00evil")

	f.Fuzz(func(t *testing.T, input string) {
		err := ValidatePathForOSAutomation(input)

		if err == nil {
			if strings.ContainsRune(input, '\x00') {
				t.Errorf("ValidatePathForOSAutomation accepted path with null byte: %q", input)
			}
			if strings.ContainsRune(input, '\n') {
				t.Errorf("ValidatePathForOSAutomation accepted path with newline: %q", input)
			}
			if strings.ContainsRune(input, '\r') {
				t.Errorf("ValidatePathForOSAutomation accepted path with carriage return: %q", input)
			}
			if strings.ContainsRune(input, '`') {
				t.Errorf("ValidatePathForOSAutomation accepted path with backtick: %q", input)
			}
		}

		hasDangerousChars := strings.ContainsAny(input, "\x00\n\r`")
		if hasDangerousChars && err == nil {
			t.Errorf("ValidatePathForOSAutomation should reject path with dangerous chars: %q", input)
		}
	})
}

func FuzzSanitizeForAppleScript(f *testing.F) {
	f.Add("/Users/test/file.pdf")
	f.Add("C:\\Users\\test\\file.pdf")
	f.Add("simple-filename.txt")
	f.Add("\"; do shell script \"rm -rf /\"")
	f.Add("path\\\\evil")
	f.Add("path\"evil")
	f.Add("\\\"test\\\"")
	f.Add("quote\"and\\backslash")
	f.Add("\\")
	f.Add("\"")
	f.Add("\"\"\"")
	f.Add("\\\\\\")

	f.Fuzz(func(t *testing.T, input string) {
		result := SanitizeForAppleScript(input)

		cleaned := strings.ReplaceAll(result, `\"`, "")
		cleaned = strings.ReplaceAll(cleaned, `\\`, "")

		if strings.Contains(cleaned, `"`) {
			t.Errorf("SanitizeForAppleScript left bare quote in output\ninput:  %q\noutput: %q\ncleaned: %q",
				input, result, cleaned)
		}

		for i := 0; i < len(result); i++ {
			if result[i] == '"' {
				if i == 0 || result[i-1] != '\\' {
					t.Errorf("SanitizeForAppleScript has unescaped quote at position %d\ninput:  %q\noutput: %q",
						i, input, result)
					break
				}
			}
		}
	})
}

func FuzzSanitizeForSendKeys(f *testing.F) {
	f.Add("C:\\Users\\test\\file.pdf")
	f.Add("D:\\Documents\\report.docx")
	f.Add("simple-filename.txt")
	f.Add("file+name")
	f.Add("file^name")
	f.Add("file%name")
	f.Add("file~name")
	f.Add("file(name)")
	f.Add("file{name}")
	f.Add("all+^%~(){}specials")
	f.Add("+++")
	f.Add("^^^")
	f.Add("{{{}}")
	f.Add("()()")

	f.Fuzz(func(t *testing.T, input string) {
		result := SanitizeForSendKeys(input)

		cleaned := result
		cleaned = strings.ReplaceAll(cleaned, "{+}", "")
		cleaned = strings.ReplaceAll(cleaned, "{^}", "")
		cleaned = strings.ReplaceAll(cleaned, "{%}", "")
		cleaned = strings.ReplaceAll(cleaned, "{~}", "")
		cleaned = strings.ReplaceAll(cleaned, "{(}", "")
		cleaned = strings.ReplaceAll(cleaned, "{)}", "")
		cleaned = strings.ReplaceAll(cleaned, "{{}", "")
		cleaned = strings.ReplaceAll(cleaned, "{}}", "")

		bareSpecials := []struct {
			char string
			name string
		}{
			{"+", "plus"}, {"^", "caret"}, {"%", "percent"}, {"~", "tilde"},
			{"(", "open paren"}, {")", "close paren"}, {"{", "open brace"}, {"}", "close brace"},
		}

		for _, spec := range bareSpecials {
			if strings.Contains(cleaned, spec.char) {
				t.Errorf("SanitizeForSendKeys left bare %s (%q) in output\ninput:  %q\noutput: %q\ncleaned: %q",
					spec.name, spec.char, input, result, cleaned)
				break
			}
		}

		specialChars := "+^%~(){}‌"
		for _, char := range specialChars {
			inputCount := strings.Count(input, string(char))
			if inputCount > 0 {
				escapedForm := "{" + string(char) + "}"
				outputCount := strings.Count(result, escapedForm)
				if outputCount < inputCount {
					t.Errorf("SanitizeForSendKeys may have lost %q escaping\ninput count: %d, escaped output count: %d\ninput:  %q\noutput: %q",
						string(char), inputCount, outputCount, input, result)
				}
			}
		}
	})
}
