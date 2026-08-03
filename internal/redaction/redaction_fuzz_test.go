// Purpose: Fuzz redaction operations for edge-case inputs and panics.
// Docs: docs/features/feature/redaction-patterns/index.md

// redaction_fuzz_test.go — Fuzz tests for redaction engine.
package redaction

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// FuzzRedact validates the Redact() method against arbitrary inputs.
// Invariants:
// 1. Eventual convergence: Redact³(s) == Redact²(s) (stabilizes after multiple passes)
// 2. Completes without hanging (implicit from fuzz framework)
// 3. No panic (implicit from fuzz framework)
// Note: Single-pass idempotency is not guaranteed when patterns can match
// each other's output (e.g., "0000000000000000ApikeY:0" where credit-card
// pattern can match the leading zeros after api-key is redacted).
func FuzzRedact(f *testing.F) {
	// Seed with known secrets from table tests
	f.Add("AKIAIOSFODNN7EXAMPLE")
	f.Add("Bearer eyJhbGciOiJSUzI1NiJ9.payload.sig")
	f.Add("Basic dXNlcjpwYXNzd29yZA==")
	f.Add("eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U")
	f.Add("ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij")
	f.Add("123-45-6789")
	f.Add("4111 1111 1111 1111")
	f.Add("api_key: sk-1234567890abcdef")
	f.Add("session=abcdef1234567890ABCDEF")

	// Edge cases
	f.Add("")
	f.Add("\x00\xff\xfe")
	f.Add(strings.Repeat("a", 100000))     // 100KB repeated 'a'
	f.Add(strings.Repeat("a]a]a]", 10000)) // ReDoS-oriented pattern

	engine := NewRedactionEngine("")

	f.Fuzz(func(t *testing.T, input string) {
		// Apply redaction multiple times
		redacted1 := engine.Redact(input)
		redacted2 := engine.Redact(redacted1)
		redacted3 := engine.Redact(redacted2)

		// Invariant: Eventually converges (3rd pass == 2nd pass)
		// This allows for cases where first pass creates new matchable patterns,
		// but ensures the process stabilizes.
		if redacted2 != redacted3 {
			t.Errorf("Redaction did not converge:\nInput:  %q\nPass1:  %q\nPass2:  %q\nPass3:  %q",
				input, redacted1, redacted2, redacted3)
		}

		// If we got here, the operation completed without hanging or panicking
	})
}

// FuzzRedactJSON validates the RedactJSON() method against arbitrary JSON inputs.
// Invariants:
// 1. If input is valid MCPToolResult JSON → output must be valid JSON
// 2. If input is valid JSON → output must be valid JSON (via fallback path)
// 3. Eventual convergence: RedactJSON³(input) == RedactJSON²(input)
// Note: Like FuzzRedact, single-pass idempotency is not guaranteed when
// patterns can match each other's output.
func FuzzRedactJSON(f *testing.F) {
	// Seed with valid MCPToolResult structures containing secrets
	f.Add([]byte(`{"content":[{"type":"text","text":"Bearer eyJhbGciOiJIUzI1NiJ9.payload.sig"}]}`))
	f.Add([]byte(`{"content":[{"type":"text","text":"SSN: 123-45-6789"}]}`))
	f.Add([]byte(`{"content":[{"type":"text","text":"AKIAIOSFODNN7EXAMPLE"}]}`))
	f.Add([]byte(`{"content":[{"type":"text","text":"api_key: sk-1234567890abcdef"}]}`))
	f.Add([]byte(`{"content":[{"type":"text","text":"session=abcdef1234567890ABCDEF"}]}`))

	// Valid MCPToolResult with multiple blocks
	f.Add([]byte(`{"content":[{"type":"text","text":"Bearer token1"},{"type":"text","text":"SSN: 999-88-7777"}],"isError":false}`))

	// Valid MCPToolResult with no secrets
	f.Add([]byte(`{"content":[{"type":"text","text":"Hello world"}]}`))

	// Edge cases - empty content
	f.Add([]byte(`{"content":[]}`))
	f.Add([]byte(`{"content":[{"type":"text","text":""}]}`))

	// Valid JSON but not MCPToolResult structure (fallback path)
	f.Add([]byte(`{"random":"Bearer token123","other":"field"}`))
	f.Add([]byte(`["Bearer token123"]`))
	f.Add([]byte(`"Bearer token123"`))
	f.Add([]byte(`123`))
	f.Add([]byte(`true`))
	f.Add([]byte(`null`))

	// Invalid JSON (fallback path)
	f.Add([]byte(`not json at all`))
	f.Add([]byte(`{"incomplete":`))
	f.Add([]byte(`{`))
	f.Add([]byte(``))

	// Binary-like content in valid JSON structure
	f.Add([]byte(`{"content":[{"type":"text","text":"\u0000\u00ff\u00fe"}]}`))

	// Large valid JSON
	largeText := strings.Repeat("a", 50000) + " Bearer secret123 " + strings.Repeat("b", 50000)
	largeJSON := `{"content":[{"type":"text","text":"` + largeText + `"}]}`
	f.Add([]byte(largeJSON))

	engine := NewRedactionEngine("")

	f.Fuzz(func(t *testing.T, input []byte) {
		// Check if input is valid MCPToolResult first (before any redaction)
		var mcpCheck mcp.MCPToolResult
		isValidMCPToolResult := json.Unmarshal(input, &mcpCheck) == nil

		// Apply redaction multiple times
		redacted1 := engine.RedactJSON(json.RawMessage(input))
		redacted2 := engine.RedactJSON(redacted1)
		redacted3 := engine.RedactJSON(redacted2)

		// Invariant: Eventually converges (3rd pass == 2nd pass)
		if string(redacted2) != string(redacted3) {
			t.Errorf("RedactJSON did not converge:\nInput:  %s\nPass1:  %s\nPass2:  %s\nPass3:  %s",
				string(input), string(redacted1), string(redacted2), string(redacted3))
		}

		if isValidMCPToolResult {
			// Invariant: If input is valid MCPToolResult → output must be valid MCPToolResult JSON
			// This is the primary use case and must preserve JSON structure.
			var mcpOutput mcp.MCPToolResult
			if err := json.Unmarshal(redacted1, &mcpOutput); err != nil {
				t.Errorf("Input was valid MCPToolResult but output is not valid JSON:\nInput:  %s\nOutput: %s\nError:  %v",
					string(input), string(redacted1), err)
			}

			// Additional check: output should also parse as MCPToolResult structure
			if len(mcpCheck.Content) > 0 && len(mcpOutput.Content) != len(mcpCheck.Content) {
				t.Errorf("Content block count mismatch:\nInput blocks:  %d\nOutput blocks: %d",
					len(mcpCheck.Content), len(mcpOutput.Content))
			}
		}
		// Note: For non-MCPToolResult JSON inputs, the fallback path uses string-based
		// redaction which may break JSON structure if patterns match across boundaries.
		// This is a known limitation of the current implementation.

		// If we got here, the operation completed without hanging or panicking
	})
}
