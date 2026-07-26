// coverage_test.go — Branch tests for origin eviction, page listing and directive mapping.
// Purpose: Coverage-expansion tests for CSP edge cases and branch paths.
// Docs: docs/features/feature/security-hardening/index.md
package csp

import (
	"strings"
	"testing"
)

// ============================================
// Generator — evictOldestOrigin / GetPages
// ============================================

func TestGenerator_EvictOldestOrigin(t *testing.T) {
	t.Parallel()
	g := NewGenerator()

	// Record 10001+ unique origins to trigger eviction
	for i := 0; i < 10002; i++ {
		origin := "https://evict-origin-" + strings.Repeat("x", 5) + "-" + padInt(i)
		g.RecordOrigin(origin, "script", "https://evict-page.example.com")
	}

	g.mu.RLock()
	count := len(g.origins)
	g.mu.RUnlock()

	if count > 10001 {
		t.Errorf("origin count = %d, should be capped after eviction", count)
	}
}

func padInt(n int) string {
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if s == "" {
		return "0"
	}
	return s
}

func TestGenerator_GetPages(t *testing.T) {
	t.Parallel()
	g := NewGenerator()

	g.RecordOrigin("https://cdn.example.com", "script", "https://page1.example.com")
	g.RecordOrigin("https://cdn.example.com", "style", "https://page2.example.com")

	pages := g.GetPages()
	if len(pages) != 2 {
		t.Errorf("GetPages() len = %d, want 2", len(pages))
	}
}

// ============================================
// directiveForResourceType — unknown type fallback
// ============================================

func TestDirectiveForResourceType_UnknownType(t *testing.T) {
	t.Parallel()
	got := directiveForResourceType("unknown")
	if got != "default-src" {
		t.Errorf("directiveForResourceType(unknown) = %q, want default-src", got)
	}
}

func TestDirectiveForResourceType_KnownTypes(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"script":  "script-src",
		"style":   "style-src",
		"font":    "font-src",
		"img":     "img-src",
		"connect": "connect-src",
	}
	for resType, want := range cases {
		got := directiveForResourceType(resType)
		if got != want {
			t.Errorf("directiveForResourceType(%q) = %q, want %q", resType, got, want)
		}
	}
}
