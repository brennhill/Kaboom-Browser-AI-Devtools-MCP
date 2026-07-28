// csp_test.go — Tests for CSP accumulation, generation and tool dispatch.
// Purpose: Tests for CSP policy generation and directive validation.
// Docs: docs/features/feature/security-hardening/index.md

package csp

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strings"
	"testing"
	"time"
)

// ============================================
// CSP Generator Tests — TDD Red Phase
// ============================================

// --- Functional Tests ---

func TestCSPDefaultModeGeneratesAllDirectives(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	// Simulate an app loading resources from 5 different origins
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/settings")
	gen.RecordOrigin("https://fonts.googleapis.com", "style", "https://myapp.com/")
	gen.RecordOrigin("https://fonts.googleapis.com", "style", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://fonts.googleapis.com", "style", "https://myapp.com/settings")
	gen.RecordOrigin("https://fonts.gstatic.com", "font", "https://myapp.com/")
	gen.RecordOrigin("https://fonts.gstatic.com", "font", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://fonts.gstatic.com", "font", "https://myapp.com/settings")
	gen.RecordOrigin("https://images.example.com", "img", "https://myapp.com/")
	gen.RecordOrigin("https://images.example.com", "img", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://images.example.com", "img", "https://myapp.com/settings")
	gen.RecordOrigin("https://api.example.com", "connect", "https://myapp.com/")
	gen.RecordOrigin("https://api.example.com", "connect", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://api.example.com", "connect", "https://myapp.com/settings")

	resp := gen.Generate(Params{Mode: "moderate"})

	// Should have directives for each resource type
	if resp.Directives == nil {
		t.Fatal("expected directives map, got nil")
	}
	if _, ok := resp.Directives["script-src"]; !ok {
		t.Error("expected script-src directive")
	}
	if _, ok := resp.Directives["style-src"]; !ok {
		t.Error("expected style-src directive")
	}
	if _, ok := resp.Directives["font-src"]; !ok {
		t.Error("expected font-src directive")
	}
	if _, ok := resp.Directives["img-src"]; !ok {
		t.Error("expected img-src directive")
	}
	if _, ok := resp.Directives["connect-src"]; !ok {
		t.Error("expected connect-src directive")
	}

	// CSP header should be non-empty
	if resp.CSPHeader == "" {
		t.Error("expected non-empty CSP header")
	}

	// Each directive should contain the corresponding origin
	assertContains(t, resp.Directives["script-src"], "https://cdn.example.com")
	assertContains(t, resp.Directives["style-src"], "https://fonts.googleapis.com")
	assertContains(t, resp.Directives["font-src"], "https://fonts.gstatic.com")
	assertContains(t, resp.Directives["img-src"], "https://images.example.com")
	assertContains(t, resp.Directives["connect-src"], "https://api.example.com")
}

func TestCSPSameOriginProducesSelf(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	// Record same-origin resources (page origin matches resource origin)
	gen.RecordOrigin("https://myapp.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://myapp.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://myapp.com", "script", "https://myapp.com/settings")

	resp := gen.Generate(Params{Mode: "moderate"})

	// 'self' should always be in default-src
	assertContains(t, resp.Directives["default-src"], "'self'")
}

func TestCSPWebSocketConnectionsInConnectSrc(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("wss://realtime.example.com", "connect", "https://myapp.com/")
	gen.RecordOrigin("wss://realtime.example.com", "connect", "https://myapp.com/dashboard")
	gen.RecordOrigin("wss://realtime.example.com", "connect", "https://myapp.com/settings")

	resp := gen.Generate(Params{Mode: "moderate"})

	assertContains(t, resp.Directives["connect-src"], "wss://realtime.example.com")
}

func TestCSPDataURIsInImgSrc(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("data:", "img", "https://myapp.com/")
	gen.RecordOrigin("data:", "img", "https://myapp.com/dashboard")
	gen.RecordOrigin("data:", "img", "https://myapp.com/settings")

	resp := gen.Generate(Params{Mode: "moderate"})

	assertContains(t, resp.Directives["img-src"], "data:")
}

func TestCSPEmptyAccumulatorReturnsHelpfulError(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	resp := gen.Generate(Params{Mode: "moderate"})

	// Should still produce a minimal policy
	assertContains(t, resp.Directives["default-src"], "'self'")

	// Should include a warning about no origins observed
	foundWarning := false
	for _, w := range resp.Warnings {
		if strings.Contains(strings.ToLower(w), "no") && strings.Contains(strings.ToLower(w), "origin") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("expected a warning about no origins observed")
	}

	// pages_visited should be 0
	if resp.Observations.PagesVisited != 0 {
		t.Errorf("expected pages_visited=0, got %d", resp.Observations.PagesVisited)
	}
}

func TestCSPExcludeOriginsParameter(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/settings")
	gen.RecordOrigin("https://tracking.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://tracking.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://tracking.example.com", "script", "https://myapp.com/settings")

	resp := gen.Generate(Params{
		Mode:           "moderate",
		ExcludeOrigins: []string{"https://tracking.example.com"},
	})

	// cdn should be included
	assertContains(t, resp.Directives["script-src"], "https://cdn.example.com")

	// tracking should be excluded
	assertNotContains(t, resp.Directives["script-src"], "https://tracking.example.com")
}

func TestCSPReportOnlyMode(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/settings")

	resp := gen.Generate(Params{Mode: "report_only"})

	if resp.HeaderName != "Content-Security-Policy-Report-Only" {
		t.Errorf("expected header name 'Content-Security-Policy-Report-Only', got %q", resp.HeaderName)
	}
}

func TestCSPEnforcingMode(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/settings")

	resp := gen.Generate(Params{Mode: "strict"})

	if resp.HeaderName != "Content-Security-Policy" {
		t.Errorf("expected header name 'Content-Security-Policy', got %q", resp.HeaderName)
	}
}

func TestCSPMetaTagGenerated(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/settings")

	resp := gen.Generate(Params{Mode: "moderate"})

	if !strings.Contains(resp.MetaTag, "<meta") {
		t.Error("expected meta tag in response")
	}
	if !strings.Contains(resp.MetaTag, "Content-Security-Policy") {
		t.Error("expected Content-Security-Policy in meta tag")
	}
}

// --- Origin Accumulator Tests ---

func TestCSPOriginAccumulatorPersistsAfterBufferWrap(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	// Record an early origin
	gen.RecordOrigin("https://early-cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://early-cdn.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://early-cdn.example.com", "script", "https://myapp.com/settings")

	// Simulate 2000 more requests from other origins (accumulator should keep all)
	for i := 0; i < 2000; i++ {
		gen.RecordOrigin("https://other.example.com", "connect", "https://myapp.com/page")
	}

	resp := gen.Generate(Params{Mode: "moderate"})

	// Early origin should still be present
	assertContains(t, resp.Directives["script-src"], "https://early-cdn.example.com")
}

func TestCSPObservationCountIncrements(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")

	// Check internal state
	gen.mu.RLock()
	entry := gen.origins["https://cdn.example.com|script"]
	gen.mu.RUnlock()

	if entry == nil {
		t.Fatal("expected origin entry, got nil")
	}
	if entry.Count != 3 {
		t.Errorf("expected count=3, got %d", entry.Count)
	}
}

func TestCSPAccumulatorClearsOnReset(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")

	gen.Reset()

	gen.mu.RLock()
	count := len(gen.origins)
	gen.mu.RUnlock()

	if count != 0 {
		t.Errorf("expected 0 origins after reset, got %d", count)
	}
}

// --- Confidence Scoring Tests ---

func TestCSPConfidenceHighOrigin(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	// Origin seen 5+ times across 3 pages -> high confidence
	for i := 0; i < 5; i++ {
		gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	}
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/settings")

	resp := gen.Generate(Params{Mode: "moderate"})

	// Should be included with high confidence
	found := false
	for _, detail := range resp.OriginDetails {
		if detail.Origin == "https://cdn.example.com" && detail.Directive == "script-src" {
			found = true
			if detail.Confidence != "high" {
				t.Errorf("expected confidence=high, got %q", detail.Confidence)
			}
			if !detail.Included {
				t.Error("expected high confidence origin to be included")
			}
			break
		}
	}
	if !found {
		t.Error("expected origin detail for https://cdn.example.com")
	}
}

func TestCSPConfidenceMediumOrigin(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	// Origin seen 2-4 times -> medium confidence
	gen.RecordOrigin("https://analytics.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://analytics.example.com", "script", "https://myapp.com/dashboard")

	resp := gen.Generate(Params{Mode: "moderate"})

	found := false
	for _, detail := range resp.OriginDetails {
		if detail.Origin == "https://analytics.example.com" && detail.Directive == "script-src" {
			found = true
			if detail.Confidence != "medium" {
				t.Errorf("expected confidence=medium, got %q", detail.Confidence)
			}
			if !detail.Included {
				t.Error("expected medium confidence origin to be included")
			}
			break
		}
	}
	if !found {
		t.Error("expected origin detail for https://analytics.example.com")
	}
}

func TestCSPConfidenceLowOriginExcluded(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	// Origin seen exactly once -> low confidence -> excluded
	gen.RecordOrigin("https://evil.com", "script", "https://myapp.com/")

	resp := gen.Generate(Params{Mode: "moderate"})

	// Should NOT be in directives
	if directives, ok := resp.Directives["script-src"]; ok {
		assertNotContains(t, directives, "https://evil.com")
	}

	// Should be in origin_details as excluded
	found := false
	for _, detail := range resp.OriginDetails {
		if detail.Origin == "https://evil.com" {
			found = true
			if detail.Confidence != "low" {
				t.Errorf("expected confidence=low, got %q", detail.Confidence)
			}
			if detail.Included {
				t.Error("expected low confidence origin to be excluded")
			}
			if detail.ExclusionReason == "" {
				t.Error("expected exclusion reason for low confidence origin")
			}
			break
		}
	}
	if !found {
		t.Error("expected origin detail for https://evil.com (even if excluded)")
	}
}

func TestCSPConnectSrcRelaxedThreshold(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	// API endpoint seen once — connect-src has relaxed threshold
	gen.RecordOrigin("https://api.example.com", "connect", "https://myapp.com/")

	resp := gen.Generate(Params{Mode: "moderate"})

	// Should be included at medium confidence for connect-src
	found := false
	for _, detail := range resp.OriginDetails {
		if detail.Origin == "https://api.example.com" && detail.Directive == "connect-src" {
			found = true
			if detail.Confidence != "medium" {
				t.Errorf("expected connect-src single observation to be medium confidence, got %q", detail.Confidence)
			}
			if !detail.Included {
				t.Error("expected single-observation connect-src origin to be included")
			}
			break
		}
	}
	if !found {
		t.Error("expected origin detail for https://api.example.com")
	}
}

func TestCSPSingleInjectedRequestNotInCSP(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	// Simulate legitimate traffic
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/settings")

	// Single injected request from evil.com
	gen.RecordOrigin("https://evil.com", "script", "https://myapp.com/")

	resp := gen.Generate(Params{Mode: "moderate"})

	// evil.com should NOT be in the generated CSP
	if scripts, ok := resp.Directives["script-src"]; ok {
		assertNotContains(t, scripts, "https://evil.com")
	}
}

func TestCSPOriginOnThreePlusPages(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	// Origin seen on 3+ pages
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/settings")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/profile")

	resp := gen.Generate(Params{Mode: "moderate"})

	found := false
	for _, detail := range resp.OriginDetails {
		if detail.Origin == "https://cdn.example.com" && detail.Directive == "script-src" {
			found = true
			if detail.Confidence != "high" {
				t.Errorf("expected high confidence for origin on 3+ pages, got %q", detail.Confidence)
			}
			if len(detail.PagesSeenOn) < 3 {
				t.Errorf("expected 3+ pages, got %d", len(detail.PagesSeenOn))
			}
			break
		}
	}
	if !found {
		t.Error("expected origin detail for https://cdn.example.com")
	}
}

// --- Development Pollution Filtering Tests ---

func TestCSPFiltersChromeExtension(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("chrome-extension://abcdef123456", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/settings")

	resp := gen.Generate(Params{Mode: "moderate"})

	// Extension origin should be filtered
	if scripts, ok := resp.Directives["script-src"]; ok {
		assertNotContains(t, scripts, "chrome-extension://")
	}

	// Should appear in filtered_origins
	foundFiltered := false
	for _, f := range resp.FilteredOrigins {
		if strings.HasPrefix(f.Origin, "chrome-extension://") {
			foundFiltered = true
			break
		}
	}
	if !foundFiltered {
		t.Error("expected chrome-extension origin in filtered_origins")
	}
}

func TestCSPFiltersMozExtension(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("moz-extension://abcdef123456", "script", "https://myapp.com/")

	resp := gen.Generate(Params{Mode: "moderate"})

	// Should appear in filtered_origins
	foundFiltered := false
	for _, f := range resp.FilteredOrigins {
		if strings.HasPrefix(f.Origin, "moz-extension://") {
			foundFiltered = true
			break
		}
	}
	if !foundFiltered {
		t.Error("expected moz-extension origin in filtered_origins")
	}
}

func TestCSPFiltersLocalhostDevServer(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	// ws://localhost:3001 on a different port from the page (page is on :3000)
	gen.RecordOrigin("ws://localhost:3001", "connect", "https://myapp.com/")
	gen.RecordOrigin("http://localhost:3001", "connect", "https://myapp.com/")

	resp := gen.Generate(Params{Mode: "moderate"})

	// Should be filtered
	if connects, ok := resp.Directives["connect-src"]; ok {
		assertNotContains(t, connects, "ws://localhost:3001")
		assertNotContains(t, connects, "http://localhost:3001")
	}

	// Should appear in filtered_origins
	if len(resp.FilteredOrigins) == 0 {
		t.Error("expected localhost dev server origins in filtered_origins")
	}
}

func TestCSPFiltersWebpackHMR(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://myapp.com", "connect", "https://myapp.com/")
	// HMR requests are filtered by URL pattern, but since we record origins,
	// the webpack HMR origin is typically localhost
	gen.RecordOrigin("http://localhost:8080", "connect", "https://myapp.com/")

	resp := gen.Generate(Params{Mode: "moderate"})

	if connects, ok := resp.Directives["connect-src"]; ok {
		assertNotContains(t, connects, "http://localhost:8080")
	}
}

func TestCSPFiltersViteDevServer(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("http://localhost:5173", "connect", "https://myapp.com/")

	resp := gen.Generate(Params{Mode: "moderate"})

	if connects, ok := resp.Directives["connect-src"]; ok {
		assertNotContains(t, connects, "http://localhost:5173")
	}
}

func TestCSPFilteredOriginsListedInResponse(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("chrome-extension://abc123", "script", "https://myapp.com/")
	gen.RecordOrigin("ws://localhost:3001", "connect", "https://myapp.com/")

	resp := gen.Generate(Params{Mode: "moderate"})

	if len(resp.FilteredOrigins) < 2 {
		t.Errorf("expected at least 2 filtered origins, got %d", len(resp.FilteredOrigins))
	}

	// Each should have a reason
	for _, f := range resp.FilteredOrigins {
		if f.Reason == "" {
			t.Errorf("expected reason for filtered origin %q", f.Origin)
		}
	}
}

func TestCSPFirstPartyLocalhostNotFiltered(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	// If the page IS on localhost:3000, same-port localhost should not be filtered
	gen.RecordOrigin("http://localhost:3000", "script", "http://localhost:3000/")
	gen.RecordOrigin("http://localhost:3000", "script", "http://localhost:3000/dashboard")
	gen.RecordOrigin("http://localhost:3000", "script", "http://localhost:3000/settings")

	resp := gen.Generate(Params{Mode: "moderate"})

	// First-party localhost is the app itself, should become 'self' or be included
	// It should NOT appear in filtered_origins
	for _, f := range resp.FilteredOrigins {
		if f.Origin == "http://localhost:3000" {
			t.Error("first-party localhost:3000 should not be filtered")
		}
	}
}

// --- Resource Type Mapping Tests ---

func TestCSPResourceTypeMapping(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	testCases := []struct {
		origin       string
		resourceType string
		directive    string
	}{
		{"https://cdn.example.com", "script", "script-src"},
		{"https://styles.example.com", "style", "style-src"},
		{"https://fonts.example.com", "font", "font-src"},
		{"https://images.example.com", "img", "img-src"},
		{"https://api.example.com", "connect", "connect-src"},
		{"https://embed.example.com", "frame", "frame-src"},
		{"https://media.example.com", "media", "media-src"},
	}

	for _, tc := range testCases {
		// Record enough to reach medium confidence (2+ times on 2+ pages)
		gen.RecordOrigin(tc.origin, tc.resourceType, "https://myapp.com/")
		gen.RecordOrigin(tc.origin, tc.resourceType, "https://myapp.com/page2")
		gen.RecordOrigin(tc.origin, tc.resourceType, "https://myapp.com/page3")
	}

	resp := gen.Generate(Params{Mode: "moderate"})

	for _, tc := range testCases {
		t.Run(tc.directive, func(t *testing.T) {
			directives, ok := resp.Directives[tc.directive]
			if !ok {
				t.Fatalf("missing directive %s", tc.directive)
			}
			assertContains(t, directives, tc.origin)
		})
	}
}

// --- Observations / Reporting Tests ---

func TestCSPPagesVisitedCount(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/settings")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/profile")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/about")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/contact")

	resp := gen.Generate(Params{Mode: "moderate"})

	if resp.Observations.PagesVisited != 6 {
		t.Errorf("expected pages_visited=6, got %d", resp.Observations.PagesVisited)
	}
}

func TestObservationsIncludeTotalResources(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://fonts.example.com", "font", "https://myapp.com/")
	gen.RecordOrigin("https://api.example.com", "connect", "https://myapp.com/")

	resp := gen.Generate(Params{Mode: "moderate"})

	if resp.Observations.TotalResources != 3 {
		t.Errorf("expected total_resources=3, got %d", resp.Observations.TotalResources)
	}
}

func TestObservationsUniqueOrigins(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/page2")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/page3")
	gen.RecordOrigin("https://fonts.example.com", "font", "https://myapp.com/")
	gen.RecordOrigin("https://fonts.example.com", "font", "https://myapp.com/page2")
	gen.RecordOrigin("https://fonts.example.com", "font", "https://myapp.com/page3")

	resp := gen.Generate(Params{Mode: "moderate"})

	if resp.Observations.UniqueOrigins != 2 {
		t.Errorf("expected unique_origins=2, got %d", resp.Observations.UniqueOrigins)
	}
}

// --- Security Hardening Safety Tests ---

func TestCSPDefaultPolicyIncludesSecurityDirectives(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/settings")

	resp := gen.Generate(Params{Mode: "moderate"})

	// Should always include base security directives
	assertContains(t, resp.Directives["default-src"], "'self'")
}

func TestCSPWarningsGeneratedForLowCoverage(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	// Only 2 pages visited
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")

	resp := gen.Generate(Params{Mode: "moderate"})

	// Should warn about low coverage
	if len(resp.Warnings) == 0 {
		t.Error("expected warnings for low page coverage")
	}
}

func TestCSPRecommendedNextStep(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/settings")

	resp := gen.Generate(Params{Mode: "moderate"})

	if resp.RecommendedNextStep == "" {
		t.Error("expected recommended_next_step in response")
	}
}

// --- MCP Tool Handler Tests ---

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

func TestCSPConcurrentAccess(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	done := make(chan bool, 10)

	// Write from multiple goroutines
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/page")
			}
			done <- true
		}(i)
	}

	// Read concurrently
	for i := 0; i < 5; i++ {
		go func() {
			gen.Generate(Params{Mode: "moderate"})
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// --- Inline Scripts NOT Hashed Tests ---

func TestCSPInlineScriptHashesNotComputed(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	// Extension-injected inline scripts should not be hashed
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/settings")

	resp := gen.Generate(Params{Mode: "moderate"})

	// No sha256 hashes should appear in script-src (we don't compute them)
	if scripts, ok := resp.Directives["script-src"]; ok {
		for _, src := range scripts {
			if strings.HasPrefix(src, "'sha256-") {
				t.Errorf("unexpected inline script hash in CSP: %s", src)
			}
		}
	}
}

// --- Page URL Tracking Tests ---

func TestCSPPageURLTrackingIncreasesConfidence(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	// Same origin on ONE page: low confidence
	gen.RecordOrigin("https://single-page.example.com", "script", "https://myapp.com/")

	resp := gen.Generate(Params{Mode: "moderate"})

	for _, detail := range resp.OriginDetails {
		if detail.Origin == "https://single-page.example.com" {
			if detail.Confidence != "low" {
				t.Errorf("expected low confidence for single-page origin, got %q", detail.Confidence)
			}
		}
	}

	// Now see it on a second page: medium confidence
	gen.RecordOrigin("https://single-page.example.com", "script", "https://myapp.com/dashboard")

	resp = gen.Generate(Params{Mode: "moderate"})

	for _, detail := range resp.OriginDetails {
		if detail.Origin == "https://single-page.example.com" && detail.Directive == "script-src" {
			if detail.Confidence != "medium" {
				t.Errorf("expected medium confidence after 2 pages, got %q", detail.Confidence)
			}
		}
	}
}

// --- Timestamp Tests ---

func TestCSPFirstSeenTimestamp(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	before := time.Now()
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	after := time.Now()

	gen.mu.RLock()
	entry := gen.origins["https://cdn.example.com|script"]
	gen.mu.RUnlock()

	if entry.FirstSeen.Before(before) || entry.FirstSeen.After(after) {
		t.Error("FirstSeen timestamp out of expected range")
	}
}

func TestCSPLastSeenUpdates(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	time.Sleep(time.Millisecond)
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")

	gen.mu.RLock()
	entry := gen.origins["https://cdn.example.com|script"]
	gen.mu.RUnlock()

	if !entry.LastSeen.After(entry.FirstSeen) {
		t.Error("expected LastSeen to be after FirstSeen")
	}
}

// --- Helper Functions ---

func assertContains(t *testing.T, slice []string, value string) {
	t.Helper()
	for _, s := range slice {
		if s == value {
			return
		}
	}
	t.Errorf("expected slice to contain %q, got %v", value, slice)
}

func assertNotContains(t *testing.T, slice []string, value string) {
	t.Helper()
	for _, s := range slice {
		if s == value {
			t.Errorf("expected slice to NOT contain %q", value)
			return
		}
	}
}

// --- RecordOriginFromBody Tests ---

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
