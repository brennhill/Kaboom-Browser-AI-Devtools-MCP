// csp_helpers_test.go — Tests CSP resource classification and URL extraction.
// Docs: docs/features/feature/security-hardening/index.md

package csp

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestDirectiveForResourceType_UnknownType(t *testing.T) {
	t.Parallel()
	if got := directiveForResourceType("unknown"); got != "default-src" {
		t.Errorf("directiveForResourceType(unknown) = %q, want default-src", got)
	}
}

func TestDirectiveForResourceType_KnownTypes(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"script": "script-src", "style": "style-src", "font": "font-src",
		"img": "img-src", "connect": "connect-src",
	}
	for resourceType, want := range cases {
		if got := directiveForResourceType(resourceType); got != want {
			t.Errorf("directiveForResourceType(%q) = %q, want %q", resourceType, got, want)
		}
	}
}

func TestCSPRecordOriginFromBody(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	// JavaScript resource → script-src
	gen.RecordOriginFromBody(types.NetworkBody{
		URL:         "https://cdn.example.com/app.js",
		ContentType: "application/javascript",
	}, "https://myapp.com/")

	// CSS resource → style-src
	gen.RecordOriginFromBody(types.NetworkBody{
		URL:         "https://cdn.example.com/style.css",
		ContentType: "text/css",
	}, "https://myapp.com/")

	// Font resource → font-src
	gen.RecordOriginFromBody(types.NetworkBody{
		URL:         "https://fonts.gstatic.com/font.woff2",
		ContentType: "font/woff2",
	}, "https://myapp.com/")

	// Image resource → img-src
	gen.RecordOriginFromBody(types.NetworkBody{
		URL:         "https://images.example.com/logo.png",
		ContentType: "image/png",
	}, "https://myapp.com/")

	// API call → connect-src
	gen.RecordOriginFromBody(types.NetworkBody{
		URL:         "https://api.example.com/data",
		ContentType: "application/json",
	}, "https://myapp.com/")

	gen.mu.RLock()
	defer gen.mu.RUnlock()

	// Verify origins are recorded with correct resource types
	if gen.origins["https://cdn.example.com|script"] == nil {
		t.Error("expected script origin for cdn.example.com")
	}
	if gen.origins["https://cdn.example.com|style"] == nil {
		t.Error("expected style origin for cdn.example.com")
	}
	if gen.origins["https://fonts.gstatic.com|font"] == nil {
		t.Error("expected font origin for fonts.gstatic.com")
	}
	if gen.origins["https://images.example.com|img"] == nil {
		t.Error("expected img origin for images.example.com")
	}
	if gen.origins["https://api.example.com|connect"] == nil {
		t.Error("expected connect origin for api.example.com")
	}

	// Verify page was recorded
	if !gen.pages["https://myapp.com/"] {
		t.Error("expected page to be recorded")
	}
}

func TestContentTypeToResourceType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		contentType string
		want        string
	}{
		{"application/javascript", "script"},
		{"text/javascript", "script"},
		{"application/javascript; charset=utf-8", "script"},
		{"text/css", "style"},
		{"text/css; charset=utf-8", "style"},
		{"font/woff2", "font"},
		{"font/woff", "font"},
		{"font/ttf", "font"},
		{"application/font-woff", "font"},
		{"application/x-font-ttf", "font"},
		{"application/x-font-woff", "font"},
		{"image/png", "img"},
		{"image/jpeg", "img"},
		{"image/svg+xml", "img"},
		{"image/webp", "img"},
		{"audio/mpeg", "media"},
		{"video/mp4", "media"},
		{"application/json", "connect"},
		{"text/html", "connect"},
		{"text/plain", "connect"},
		{"application/xml", "connect"},
		{"", "connect"},
	}

	for _, tc := range tests {
		t.Run(tc.contentType, func(t *testing.T) {
			got := contentTypeToResourceType(tc.contentType)
			if got != tc.want {
				t.Errorf("contentTypeToResourceType(%q) = %q, want %q", tc.contentType, got, tc.want)
			}
		})
	}
}

func TestCSPRecordOriginFromBodyInvalidURL(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	// Empty URL should not panic
	gen.RecordOriginFromBody(types.NetworkBody{
		URL:         "",
		ContentType: "application/javascript",
	}, "https://myapp.com/")

	// Invalid URL should not panic
	gen.RecordOriginFromBody(types.NetworkBody{
		URL:         "://invalid",
		ContentType: "application/javascript",
	}, "https://myapp.com/")

	gen.mu.RLock()
	defer gen.mu.RUnlock()

	if len(gen.origins) != 0 {
		t.Errorf("expected no origins recorded for invalid URLs, got %d", len(gen.origins))
	}
}

func TestCSPExtractPageOriginsInvalidURL(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()
	gen.pages["://invalid-url"] = true
	gen.pages["https://valid.com/path"] = true

	origins := gen.extractPageOrigins()
	// Invalid URL should be skipped, valid should be included
	if !origins["https://valid.com"] {
		t.Error("expected valid origin to be extracted")
	}
}
