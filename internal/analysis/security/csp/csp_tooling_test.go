// csp_tooling_test.go — Tests CSP tool parameter handling and dispatch.
// Docs: docs/features/feature/security-hardening/index.md

package csp

import (
	"encoding/json"
	"testing"
)

func TestCSPHandleGenerateCSPValid(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/settings")

	params := json.RawMessage(`{"mode": "moderate"}`)
	result, err := gen.HandleGenerateCSP(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestCSPHandleGenerateCSPEmptyParams(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/settings")

	// Empty params should use defaults
	params := json.RawMessage(`{}`)
	result, err := gen.HandleGenerateCSP(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestCSPHandleGenerateCSPWithExclusions(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/settings")
	gen.RecordOrigin("https://evil.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://evil.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://evil.com", "script", "https://myapp.com/settings")

	params := json.RawMessage(`{"mode": "strict", "exclude_origins": ["https://evil.com"]}`)
	result, err := gen.HandleGenerateCSP(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, ok := result.(*Response)
	if !ok {
		t.Fatal("expected *Response type")
	}

	if scripts, exists := resp.Directives["script-src"]; exists {
		assertNotContains(t, scripts, "https://evil.com")
	}
}

// --- Concurrent Access Test ---

func TestHandleGenerateCSPInvalidParams(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	// Invalid JSON params should return error
	_, err := gen.HandleGenerateCSP(json.RawMessage(`{invalid}`))
	if err == nil {
		t.Error("expected error for invalid JSON params")
	}

	// Valid empty params should work
	resp, err := gen.HandleGenerateCSP(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Error("expected non-nil response")
	}

	// Nil params should work (defaults)
	resp, err = gen.HandleGenerateCSP(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Error("expected non-nil response")
	}
}
