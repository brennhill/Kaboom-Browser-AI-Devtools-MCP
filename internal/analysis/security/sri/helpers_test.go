// helpers_test.go — Branch tests for tag template rendering.
// Purpose: Coverage-expansion tests for SRI helper edge cases.
// Docs: docs/features/feature/security-hardening/index.md
package sri

import (
	"strings"
	"testing"
)

func TestGenerateTagTemplate_Script(t *testing.T) {
	t.Parallel()
	got := generateTagTemplate("https://cdn.example.com/app.js", "sha384-abc", "script")
	if got == "" {
		t.Fatal("expected non-empty script tag")
	}
	if !strings.Contains(got, "script") || !strings.Contains(got, "integrity") {
		t.Errorf("unexpected tag: %q", got)
	}
}

func TestGenerateTagTemplate_Style(t *testing.T) {
	t.Parallel()
	got := generateTagTemplate("https://cdn.example.com/app.css", "sha384-abc", "style")
	if got == "" {
		t.Fatal("expected non-empty link tag")
	}
	if !strings.Contains(got, "stylesheet") || !strings.Contains(got, "integrity") {
		t.Errorf("unexpected tag: %q", got)
	}
}

func TestGenerateTagTemplate_Unknown(t *testing.T) {
	t.Parallel()
	got := generateTagTemplate("https://cdn.example.com/data.json", "sha384-abc", "data")
	if got != "" {
		t.Errorf("expected empty for unknown type, got %q", got)
	}
}
