// netflag_test.go — Tests for suspicious-origin detection and severity classification.
// Purpose: Tests for security issue flagging and severity classification.
// Docs: docs/features/feature/security-hardening/index.md
//
// These were previously gated behind `//go:build integration` with a note saying
// NetworkWaterfallEntry/SecurityFlag "aren't available". That note is stale — the
// types are exported from internal/capture and the tests compile and pass in a
// normal run — so the tag is gone and the detectors are covered by default.
package netflag

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// ============================================
// Security Flagging Tests (TDD Phase 3)
// ============================================
// These tests verify the security threat detection algorithms
// for suspicious origins in network waterfall data.

// ============================================
// Suspicious TLD Tests
// ============================================

func TestCheckSuspiciousTLD_FlagsHighRiskTLD(t *testing.T) {
	t.Parallel()
	flag := checkSuspiciousTLD("https://cdn-analytics.xyz")

	if flag == nil {
		t.Fatal("Should flag .xyz TLD")
	}

	if flag.Type != "suspicious_tld" {
		t.Errorf("Expected type 'suspicious_tld', got '%s'", flag.Type)
	}

	if flag.Severity == "low" {
		t.Error("High-risk TLD should not have 'low' severity")
	}
}

func TestCheckSuspiciousTLD_AllowsLegitimateOrigin(t *testing.T) {
	t.Parallel()
	// Even if TLD is suspicious, known legitimate origins should pass
	flag := checkSuspiciousTLD("https://pages.dev")

	if flag != nil {
		t.Error("Known legitimate origin should not be flagged")
	}
}

func TestCheckSuspiciousTLD_AllowsCommonTLDs(t *testing.T) {
	t.Parallel()
	// Common TLDs should not be flagged
	commonOrigins := []string{
		"https://example.com",
		"https://cdn.example.org",
		"https://api.example.net",
	}

	for _, origin := range commonOrigins {
		flag := checkSuspiciousTLD(origin)
		if flag != nil {
			t.Errorf("Common TLD should not be flagged: %s", origin)
		}
	}
}

// ============================================
// Non-Standard Port Tests
// ============================================

func TestCheckNonStandardPort_FlagsUnusualPorts(t *testing.T) {
	t.Parallel()
	flag := checkNonStandardPort("https://example.com:8443")

	if flag == nil {
		t.Fatal("Should flag non-standard port 8443")
	}

	if flag.Type != "non_standard_port" {
		t.Errorf("Expected type 'non_standard_port', got '%s'", flag.Type)
	}
}

func TestCheckNonStandardPort_AllowsStandardPorts(t *testing.T) {
	t.Parallel()
	standardOrigins := []string{
		"https://example.com",     // Implicit 443
		"https://example.com:443", // Explicit 443
		"http://example.com",      // Implicit 80
		"http://example.com:80",   // Explicit 80
	}

	for _, origin := range standardOrigins {
		flag := checkNonStandardPort(origin)
		if flag != nil {
			t.Errorf("Standard port should not be flagged: %s", origin)
		}
	}
}

func TestCheckNonStandardPort_AllowsDevPorts(t *testing.T) {
	t.Parallel()
	// Development ports should not be flagged
	devOrigins := []string{
		"http://localhost:3000",
		"http://localhost:8080",
		"http://127.0.0.1:5173",
	}

	for _, origin := range devOrigins {
		flag := checkNonStandardPort(origin)
		if flag != nil {
			t.Errorf("Dev port should not be flagged: %s", origin)
		}
	}
}

// ============================================
// Mixed Content Tests
// ============================================

func TestCheckMixedContent_FlagsHTTPOnHTTPS(t *testing.T) {
	t.Parallel()
	entry := capture.NetworkWaterfallEntry{
		URL:           "http://cdn.example.com/script.js",
		InitiatorType: "script",
	}
	pageURL := "https://myapp.com"

	flag := checkMixedContent(entry, pageURL)

	if flag == nil {
		t.Fatal("Should flag HTTP resource on HTTPS page")
	}

	if flag.Type != "mixed_content" {
		t.Errorf("Expected type 'mixed_content', got '%s'", flag.Type)
	}

	if flag.Severity == "low" {
		t.Error("Mixed content should have at least medium severity")
	}
}

func TestCheckMixedContent_AllowsHTTPSOnHTTPS(t *testing.T) {
	t.Parallel()
	entry := capture.NetworkWaterfallEntry{
		URL:           "https://cdn.example.com/script.js",
		InitiatorType: "script",
	}
	pageURL := "https://myapp.com"

	flag := checkMixedContent(entry, pageURL)

	if flag != nil {
		t.Error("HTTPS resource on HTTPS page should not be flagged")
	}
}

func TestCheckMixedContent_AllowsHTTPOnHTTP(t *testing.T) {
	t.Parallel()
	entry := capture.NetworkWaterfallEntry{
		URL:           "http://cdn.example.com/script.js",
		InitiatorType: "script",
	}
	pageURL := "http://myapp.com"

	flag := checkMixedContent(entry, pageURL)

	if flag != nil {
		t.Error("HTTP resource on HTTP page should not be flagged")
	}
}

// ============================================
// IP Address Origin Tests
// ============================================

func TestCheckIPAddressOrigin_FlagsIPv4(t *testing.T) {
	t.Parallel()
	flag := checkIPAddressOrigin("http://192.168.1.100:8080")

	if flag == nil {
		t.Fatal("Should flag IPv4 address")
	}

	if flag.Type != "ip_address_origin" {
		t.Errorf("Expected type 'ip_address_origin', got '%s'", flag.Type)
	}
}

func TestCheckIPAddressOrigin_FlagsIPv6(t *testing.T) {
	t.Parallel()
	flag := checkIPAddressOrigin("http://[2001:db8::1]")

	if flag == nil {
		t.Fatal("Should flag IPv6 address")
	}

	if flag.Type != "ip_address_origin" {
		t.Errorf("Expected type 'ip_address_origin', got '%s'", flag.Type)
	}
}

func TestCheckIPAddressOrigin_AllowsHostnames(t *testing.T) {
	t.Parallel()
	flag := checkIPAddressOrigin("https://cdn.example.com")

	if flag != nil {
		t.Error("Hostname should not be flagged")
	}
}

func TestCheckIPAddressOrigin_AllowsLocalhost(t *testing.T) {
	t.Parallel()
	// localhost should not be flagged (development)
	flag := checkIPAddressOrigin("http://localhost:3000")

	if flag != nil {
		t.Error("localhost should not be flagged")
	}
}

// ============================================
// Typosquatting Tests
// ============================================

func TestCheckTyposquatting_FlagsSimilarDomains(t *testing.T) {
	t.Parallel()
	// "unpkg" vs "unpkg" typo
	flag := checkTyposquatting("https://unpkg.cm/library.js") // .cm instead of .com

	if flag == nil {
		t.Fatal("Should flag typosquatting attempt")
	}

	if flag.Type != "potential_typosquatting" {
		t.Errorf("Expected type 'potential_typosquatting', got '%s'", flag.Type)
	}

	if flag.Severity != "high" {
		t.Errorf("Typosquatting should be high severity, got '%s'", flag.Severity)
	}
}

func TestCheckTyposquatting_AllowsLegitDomains(t *testing.T) {
	t.Parallel()
	legitDomains := []string{
		"https://unpkg.com/library.js",
		"https://cdn.jsdelivr.net/npm/package",
		"https://cdnjs.cloudflare.com/ajax/libs/jquery",
	}

	for _, origin := range legitDomains {
		flag := checkTyposquatting(origin)
		if flag != nil {
			t.Errorf("Legitimate domain should not be flagged: %s", origin)
		}
	}
}

// ============================================
// Integration Tests
// ============================================

func TestAnalyzeNetworkSecurity_RunsAllChecks(t *testing.T) {
	t.Parallel()
	entry := capture.NetworkWaterfallEntry{
		URL:           "http://cdn-malicious.xyz:8443/script.js",
		InitiatorType: "script",
		Timestamp:     time.Now(),
	}
	pageURL := "https://myapp.com"

	flags := Analyze(entry, pageURL)

	// Should detect multiple issues:
	// 1. Suspicious TLD (.xyz)
	// 2. Non-standard port (8443)
	// 3. Mixed content (HTTP on HTTPS page)

	if len(flags) == 0 {
		t.Fatal("Should detect security issues")
	}

	// Check that multiple flag types were detected
	flagTypes := make(map[string]bool)
	for _, flag := range flags {
		flagTypes[flag.Type] = true
	}

	if len(flagTypes) < 2 {
		t.Errorf("Should detect multiple types of issues, got %d", len(flagTypes))
	}
}

func TestAnalyzeNetworkSecurity_ReturnsEmptyForSafeOrigins(t *testing.T) {
	t.Parallel()
	entry := capture.NetworkWaterfallEntry{
		URL:           "https://cdn.example.com/library.js",
		InitiatorType: "script",
		Timestamp:     time.Now(),
	}
	pageURL := "https://myapp.com"

	flags := Analyze(entry, pageURL)

	if len(flags) > 0 {
		t.Error("Safe origin should not trigger any flags")
	}
}

// ============================================
// Storage Tests
// ============================================
// NOTE: SecurityFlags storage tests removed — securityFlags field was dead code
// (never written by production code). The SecurityFlag type is retained for
// future use by security analysis functions.
