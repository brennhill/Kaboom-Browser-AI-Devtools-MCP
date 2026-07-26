// Purpose: Tests for CSP policy generation from captured traffic.
// Docs: docs/features/feature/test-generation/index.md

// csp_test.go — Tests for CSP generation helpers.
package generate

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func TestExtractOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		url  string
		want string
	}{
		{"https://example.com/path", "https://example.com"},
		{"https://example.com:8080/path", "https://example.com:8080"},
		{"http://localhost:3000/api/v1", "http://localhost:3000"},
		{"https://cdn.example.com/js/app.js?v=1", "https://cdn.example.com"},
		{"http://127.0.0.1:9222", "http://127.0.0.1:9222"},
		{"", ""},
		{"ftp://files.example.com", ""},
		{"not-a-url", ""},
	}

	for _, tt := range tests {
		got := ExtractOrigin(tt.url)
		if got != tt.want {
			t.Errorf("ExtractOrigin(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestResourceTypeToCSPDirective(t *testing.T) {
	t.Parallel()

	tests := []struct {
		contentType string
		want        string
	}{
		{"application/javascript", "script-src"},
		{"text/javascript", "script-src"},
		{"text/css", "style-src"},
		{"font/woff2", "font-src"},
		{"image/png", "img-src"},
		{"image/svg+xml", "img-src"},
		{"video/mp4", "media-src"},
		{"audio/mpeg", "media-src"},
		{"application/json", "connect-src"},
		{"text/html", "connect-src"},
		{"", "connect-src"},
	}

	for _, tt := range tests {
		got := resourceTypeToCSPDirective(tt.contentType)
		if got != tt.want {
			t.Errorf("resourceTypeToCSPDirective(%q) = %q, want %q", tt.contentType, got, tt.want)
		}
	}
}

// TestBuildCSPPolicyString_Deterministic pins the policy string to a stable
// ordering. Both BuildCSPDirectives and BuildCSPPolicyString ranged Go maps
// directly, and Go randomizes map iteration order, so generate(csp) emitted a
// different directive order — and a different origin order within each
// directive — on every call for byte-identical input.
//
// That makes the output undiffable: a user comparing two runs, or committing a
// generated policy, sees spurious churn with no change in meaning. It also
// makes any golden-file test of this output impossible to write.
func TestBuildCSPPolicyString_Deterministic(t *testing.T) {
	t.Parallel()

	directives := map[string][]string{
		"script-src":  {"'self'", "https://cdn.example.com", "https://a.example.com"},
		"img-src":     {"'self'", "https://img.example.com"},
		"default-src": {"'self'"},
		"connect-src": {"'self'", "https://api.example.com"},
		"style-src":   {"'self'"},
	}

	first := BuildCSPPolicyString(directives)
	for i := 0; i < 50; i++ {
		if got := BuildCSPPolicyString(directives); got != first {
			t.Fatalf("policy string is not deterministic:\n run 0: %s\n run %d: %s", first, i+1, got)
		}
	}

	// default-src must lead: it is the fallback every other directive narrows,
	// so a reader scanning the policy should meet it first.
	if !strings.HasPrefix(first, "default-src ") {
		t.Errorf("default-src must come first, got: %s", first)
	}
}

// TestBuildCSPDirectives_DeterministicOrigins covers the same defect one level
// up: the origin set inside a single directive was also an unsorted map range.
func TestBuildCSPDirectives_DeterministicOrigins(t *testing.T) {
	t.Parallel()

	bodies := []capture.NetworkBody{
		{URL: "https://zebra.example.com/a.js", ContentType: "application/javascript"},
		{URL: "https://alpha.example.com/b.js", ContentType: "application/javascript"},
		{URL: "https://middle.example.com/c.js", ContentType: "application/javascript"},
	}

	first := BuildCSPDirectives(bodies)["script-src"]
	for i := 0; i < 50; i++ {
		got := BuildCSPDirectives(bodies)["script-src"]
		if len(got) != len(first) {
			t.Fatalf("origin count changed between runs: %d vs %d", len(first), len(got))
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("origin order is not deterministic at %d:\n run 0: %v\n run %d: %v", j, first, i+1, got)
			}
		}
	}

	// 'self' stays pinned at the front; the rest sort so the output is diffable.
	want := []string{"'self'", "https://alpha.example.com", "https://middle.example.com", "https://zebra.example.com"}
	for i := range want {
		if i >= len(first) || first[i] != want[i] {
			t.Fatalf("origins not sorted with 'self' first:\n got:  %v\n want: %v", first, want)
		}
	}
}
