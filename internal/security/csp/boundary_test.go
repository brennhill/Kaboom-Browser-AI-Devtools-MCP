// boundary_test.go — Session-only whitelist overrides must not persist and must be audited.
// Purpose: Tests for security boundary enforcement and isolation.
// Docs: docs/features/feature/security-hardening/index.md

package csp

import (
	"strings"
	"testing"

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

	gen := NewGenerator()

	// Generate CSP with session-only override
	csp := gen.Generate(Params{
		WhitelistOverride: []string{"https://temp.xyz"},
	})

	// Verify override applied in this session
	if !strings.Contains(csp.CSPHeader, "temp.xyz") {
		t.Error("Session override should be applied to CSP header")
	}

	// Generate CSP again without override
	csp2 := gen.Generate(Params{})

	// Verify override NOT persisted
	if strings.Contains(csp2.CSPHeader, "temp.xyz") {
		t.Error("Session override should NOT persist across invocations")
	}
}

func TestSessionOverride_WarningIncluded(t *testing.T) {
	// Clear audit log before test
	policy.ClearAuditEvents()

	gen := NewGenerator()

	// Generate CSP with session-only override
	csp := gen.Generate(Params{
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

	gen := NewGenerator()

	// Generate CSP with session-only override
	csp := gen.Generate(Params{
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

	gen := NewGenerator()

	// Generate CSP with override
	_ = gen.Generate(Params{
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
