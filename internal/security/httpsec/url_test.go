// url_test.go — Unit tests for URL classification helpers.
// Docs: docs/features/feature/security-hardening/index.md
package httpsec

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
)

// ============================================
// IsThirdPartyURL — Edge cases
// ============================================

func TestIsThirdPartyURL_EmptyPageURLs(t *testing.T) {
	t.Parallel()
	got := IsThirdPartyURL("https://example.com/api", nil)
	if got {
		t.Error("IsThirdPartyURL with empty pageURLs should return false")
	}
}

func TestIsThirdPartyURL_InvalidRequestURL(t *testing.T) {
	t.Parallel()
	got := IsThirdPartyURL("://invalid", []string{"https://example.com"})
	if got {
		t.Error("IsThirdPartyURL with invalid request URL should return false")
	}
}

func TestIsThirdPartyURL_SubdomainMatch(t *testing.T) {
	t.Parallel()
	got := IsThirdPartyURL("https://api.example.com/data", []string{"https://example.com"})
	if got {
		t.Error("api.example.com should be first-party relative to example.com")
	}
}

func TestIsThirdPartyURL_ReverseSubdomain(t *testing.T) {
	t.Parallel()
	got := IsThirdPartyURL("https://example.com/data", []string{"https://api.example.com"})
	if got {
		t.Error("example.com should be first-party relative to api.example.com")
	}
}

func TestIsThirdPartyURL_ThirdParty(t *testing.T) {
	t.Parallel()
	got := IsThirdPartyURL("https://analytics.google.com/collect", []string{"https://example.com"})
	if !got {
		t.Error("analytics.google.com should be third-party relative to example.com")
	}
}

func TestIsThirdPartyURL_InvalidPageURL(t *testing.T) {
	t.Parallel()
	got := IsThirdPartyURL("https://example.com/api", []string{"://invalid"})
	if !got {
		t.Error("should be third-party when page URL is invalid")
	}
}

// ============================================
// IsLocalhostURL — Edge cases
// ============================================

func TestIsLocalhostURL_Variants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		url  string
		want bool
	}{
		{"http://localhost:3000/api", true},
		{"http://127.0.0.1:8080/test", true},
		{"http://[::1]:3000/api", true},
		{"http://0.0.0.0:5000/test", true},
		{"https://example.com/api", false},
		{"://invalid", false},
	}
	for _, tc := range cases {
		got := IsLocalhostURL(tc.url)
		if got != tc.want {
			t.Errorf("IsLocalhostURL(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

// ============================================
// Content-type classification
// ============================================

func TestIsHTMLResponse(t *testing.T) {
	t.Parallel()
	if !IsHTMLResponse(types.NetworkBody{ContentType: "Text/HTML; charset=utf-8"}) {
		t.Error("text/html content type should be reported as HTML")
	}
	if IsHTMLResponse(types.NetworkBody{ContentType: "application/json"}) {
		t.Error("application/json should not be reported as HTML")
	}
}

func TestIsJavaScriptContent(t *testing.T) {
	t.Parallel()
	if !IsJavaScriptContent("Application/JavaScript") {
		t.Error("application/javascript should be reported as JavaScript")
	}
	if IsJavaScriptContent("text/css") {
		t.Error("text/css should not be reported as JavaScript")
	}
}
