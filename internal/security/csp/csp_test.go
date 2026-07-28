// csp_test.go — Tests CSP policy generation and directive validation.
// Docs: docs/features/feature/security-hardening/index.md

package csp

import (
	"strings"
	"testing"
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
