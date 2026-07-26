// Purpose: Tests for security boundary enforcement and isolation.
// Docs: docs/features/feature/security-hardening/index.md

package security

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/policy"
)

// ============================================
// Security Boundary: LLM Trust Model Tests
// ============================================
// These tests enforce the security boundary between LLM tool calls
// (untrusted) and persistent security configuration (trusted).
// See: docs/specs/security-boundary-llm-trust.md

// ============================================
// Session-Only Override Tests
// ============================================

func TestSessionOverride_NotPersisted(t *testing.T) {
	// Clear audit log before test
	policy.ClearAuditEvents()

	// Create CSP generator
	gen := NewCSPGenerator()

	// Generate CSP with session-only override
	csp := gen.GenerateCSP(CSPParams{
		WhitelistOverride: []string{"https://temp.xyz"},
	})

	// Verify override applied in this session
	if !strings.Contains(csp.CSPHeader, "temp.xyz") {
		t.Error("Session override should be applied to CSP header")
	}

	// Generate CSP again without override
	csp2 := gen.GenerateCSP(CSPParams{})

	// Verify override NOT persisted
	if strings.Contains(csp2.CSPHeader, "temp.xyz") {
		t.Error("Session override should NOT persist across invocations")
	}
}

func TestSessionOverride_WarningIncluded(t *testing.T) {
	// Clear audit log before test
	policy.ClearAuditEvents()

	// Create CSP generator
	gen := NewCSPGenerator()

	// Generate CSP with session-only override
	csp := gen.GenerateCSP(CSPParams{
		WhitelistOverride: []string{"https://temp.xyz"},
	})

	// Verify warning about session-only override
	foundWarning := false
	for _, warning := range csp.Warnings {
		if strings.Contains(warning, "SESSION-ONLY") && strings.Contains(warning, "temp.xyz") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Error("CSP should include SESSION-ONLY warning for override")
	}
}

func TestSessionOverride_AuditInfo(t *testing.T) {
	// Clear audit log before test
	policy.ClearAuditEvents()

	// Create CSP generator
	gen := NewCSPGenerator()

	// Generate CSP with session-only override
	csp := gen.GenerateCSP(CSPParams{
		WhitelistOverride: []string{"https://temp.xyz"},
	})

	// Verify audit info included
	if csp.Audit == nil {
		t.Fatal("CSP should include audit information")
	}

	if len(csp.Audit.SessionOverrides) == 0 {
		t.Error("Audit should list session overrides")
	}

	if csp.Audit.OverrideSource != "mcp_tool_parameter" {
		t.Errorf("Audit should track override source, got: %s", csp.Audit.OverrideSource)
	}
}

// ============================================
// Audit Logging Tests
// ============================================

func TestAuditLog_RecordsSecurityDecisions(t *testing.T) {
	// Clear audit log before test
	policy.ClearAuditEvents()

	// Create CSP generator with audit logging
	gen := NewCSPGenerator()

	// Generate CSP with override
	_ = gen.GenerateCSP(CSPParams{
		WhitelistOverride: []string{"https://temp.xyz"},
	})

	// Verify audit log entry created
	events := policy.AuditEvents()

	if len(events) == 0 {
		t.Fatal("Expected audit log entry for CSP generation with override")
	}

	// Find the CSP generation event
	foundEvent := false
	for _, event := range events {
		if event.Action == "whitelist_override" && event.Origin == "https://temp.xyz" {
			foundEvent = true

			// Verify required fields
			if event.Persistent {
				t.Error("Session override should be marked as non-persistent")
			}
			if event.Source != "mcp" {
				t.Errorf("Source should be 'mcp', got: %s", event.Source)
			}
			if event.Timestamp.IsZero() {
				t.Error("Timestamp should be set")
			}

			break
		}
	}

	if !foundEvent {
		t.Error("Audit log should contain whitelist_override event")
	}
}

// ============================================
// Network Security Check Wiring Tests
// ============================================

func TestScan_NetworkCheckDetectsSuspiciousTLD(t *testing.T) {
	scanner := NewSecurityScanner()

	input := SecurityScanInput{
		WaterfallEntries: []capture.NetworkWaterfallEntry{
			{URL: "https://cdn-analytics.xyz/tracker.js", InitiatorType: "script"},
		},
		PageURLs: []string{"https://myapp.com"},
		Checks:   []string{"network"},
	}

	result := scanner.Scan(input)

	if len(result.Findings) == 0 {
		t.Fatal("Expected network findings for suspicious TLD .xyz")
	}

	found := false
	for _, f := range result.Findings {
		if f.Check == "network" && strings.Contains(f.Title, ".xyz") {
			found = true
			if f.Severity != "medium" {
				t.Errorf("Expected medium severity for .xyz TLD, got %s", f.Severity)
			}
		}
	}
	if !found {
		t.Error("Expected a 'network' finding mentioning .xyz TLD")
	}
}

func TestScan_NetworkCheckDetectsTyposquatting(t *testing.T) {
	scanner := NewSecurityScanner()

	input := SecurityScanInput{
		WaterfallEntries: []capture.NetworkWaterfallEntry{
			{URL: "https://unpkg.cm/library.js", InitiatorType: "script"},
		},
		PageURLs: []string{"https://myapp.com"},
		Checks:   []string{"network"},
	}

	result := scanner.Scan(input)

	found := false
	for _, f := range result.Findings {
		if f.Check == "network" && f.Severity == "high" {
			found = true
		}
	}
	if !found {
		t.Error("Expected high-severity network finding for typosquatting domain unpkg.cm")
	}
}

func TestScan_NetworkCheckDetectsMixedContent(t *testing.T) {
	scanner := NewSecurityScanner()

	input := SecurityScanInput{
		WaterfallEntries: []capture.NetworkWaterfallEntry{
			{URL: "http://cdn.example.com/script.js", InitiatorType: "script"},
		},
		PageURLs: []string{"https://myapp.com"},
		Checks:   []string{"network"},
	}

	result := scanner.Scan(input)

	found := false
	for _, f := range result.Findings {
		if f.Check == "network" && strings.Contains(f.Title, "mixed content") {
			found = true
			if f.Severity != "high" {
				t.Errorf("Expected high severity for script mixed content, got %s", f.Severity)
			}
		}
	}
	if !found {
		t.Error("Expected mixed content finding for HTTP script on HTTPS page")
	}
}

func TestScan_NetworkCheckSafeOriginNoFindings(t *testing.T) {
	scanner := NewSecurityScanner()

	input := SecurityScanInput{
		WaterfallEntries: []capture.NetworkWaterfallEntry{
			{URL: "https://cdn.example.com/library.js", InitiatorType: "script"},
		},
		PageURLs: []string{"https://myapp.com"},
		Checks:   []string{"network"},
	}

	result := scanner.Scan(input)

	for _, f := range result.Findings {
		if f.Check == "network" {
			t.Errorf("Safe origin should not produce network findings, got: %s", f.Title)
		}
	}
}

func TestScan_NetworkCheckIncludedByDefault(t *testing.T) {
	scanner := NewSecurityScanner()

	// No explicit Checks — should run all including "network"
	input := SecurityScanInput{
		WaterfallEntries: []capture.NetworkWaterfallEntry{
			{URL: "https://cdn-analytics.xyz/tracker.js", InitiatorType: "script"},
		},
		PageURLs: []string{"https://myapp.com"},
	}

	result := scanner.Scan(input)

	found := false
	for _, f := range result.Findings {
		if f.Check == "network" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Network check should run by default when no checks specified")
	}
}
