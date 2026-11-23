// boundary_test.go — Enforces the LLM-trust boundary: MCP mode detection and blocked config mutations.
// Purpose: Tests for security boundary enforcement and isolation.
// Docs: docs/features/feature/security-hardening/index.md

package policy

import (
	"os"
	"strings"
	"testing"
)

// ============================================
// Security Boundary: LLM Trust Model Tests
// ============================================
// These tests enforce the security boundary between LLM tool calls
// (untrusted) and persistent security configuration (trusted).
// See: docs/specs/security-boundary-llm-trust.md

// ============================================
// MCP Mode Detection Tests
// ============================================

func TestIsMCPMode_DetectsEnvironmentVariable(t *testing.T) {
	// Test: MCP_MODE=1 should enable MCP mode
	t.Setenv("MCP_MODE", "1")
	InitMode()

	if !IsMCPMode() {
		t.Error("MCP_MODE=1 should enable MCP mode")
	}

	if IsInteractiveTerminal() {
		t.Error("MCP mode should never be interactive")
	}
}

func TestIsMCPMode_DefaultsToFalse(t *testing.T) {
	// Test: No MCP_MODE env var should default to false
	t.Setenv("MCP_MODE", "")
	InitMode()

	if IsMCPMode() {
		t.Error("Without MCP_MODE env var, should not be in MCP mode")
	}
}

// ============================================
// Security Config Modification Guard Tests
// ============================================

func TestAddToWhitelist_BlockedInMCPMode(t *testing.T) {
	t.Setenv("MCP_MODE", "1")
	InitMode()

	// Test: MCP mode should block whitelist additions
	err := AddToWhitelist("https://evil.xyz")

	if err == nil {
		t.Fatal("AddToWhitelist should return error in MCP mode")
	}

	if !strings.Contains(err.Error(), "human review") {
		t.Errorf("Error should mention human review, got: %s", err.Error())
	}
}

func TestSetMinSeverity_BlockedInMCPMode(t *testing.T) {
	t.Setenv("MCP_MODE", "1")
	InitMode()

	// Test: MCP mode should block severity threshold changes
	err := SetMinSeverity("critical")

	if err == nil {
		t.Fatal("SetMinSeverity should return error in MCP mode")
	}

	if !strings.Contains(err.Error(), "human review") {
		t.Errorf("Error should mention human review, got: %s", err.Error())
	}
}

func TestClearWhitelist_BlockedInMCPMode(t *testing.T) {
	t.Setenv("MCP_MODE", "1")
	InitMode()

	// Test: MCP mode should block whitelist clearing
	err := ClearWhitelist()

	if err == nil {
		t.Fatal("ClearWhitelist should return error in MCP mode")
	}

	if !strings.Contains(err.Error(), "human review") {
		t.Errorf("Error should mention human review, got: %s", err.Error())
	}
}

// ============================================
// Config File Immutability Tests
// ============================================

func TestMCPCalls_DoNotModifyConfigFile(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	path := tmpDir + "/security.json"
	initialConfig := `{"version": "1.0", "whitelisted_origins": []}`

	if err := os.WriteFile(path, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Setup MCP mode
	t.Setenv("MCP_MODE", "1")
	InitMode()

	// Override config path
	original := configPath()
	setConfigPath(path)
	defer setConfigPath(original)

	// Attempt to modify config via MCP tool (should fail)
	_ = AddToWhitelist("https://evil.xyz")

	// Read config file
	configData, err := os.ReadFile(path) // nosemgrep: go_filesystem_rule-fileread -- test helper reads fixture/output file
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	// Verify config unchanged
	if string(configData) != initialConfig {
		t.Error("MCP calls should not modify security config file")
	}
}

// ============================================
// EditInstruction — path resolution branches
// ============================================

// NOTE: configPathOverride tests cannot use t.Parallel() because they mutate package-level state.

func TestEditInstruction_WithPath(t *testing.T) {
	// Not parallel — mutates package-level configPathOverride
	old := configPathOverride
	setConfigPath("/tmp/test-security.json")
	defer setConfigPath(old)

	got := EditInstruction()
	if got != "Edit /tmp/test-security.json manually" {
		t.Errorf("EditInstruction() = %q", got)
	}
}

func TestEditInstruction_EmptyPath(t *testing.T) {
	// Not parallel — mutates package-level configPathOverride
	old := configPathOverride
	setConfigPath("")
	defer setConfigPath(old)

	got := EditInstruction()
	if got == "" {
		t.Error("expected non-empty instruction")
	}
}
