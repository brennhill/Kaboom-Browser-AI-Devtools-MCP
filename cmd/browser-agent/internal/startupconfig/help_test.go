// help_test.go — Verifies the canonical process help contract.

package startupconfig

import (
	"strings"
	"testing"
)

func TestHelpTextDocumentsCurrentModesWithoutCompatibilityAliases(t *testing.T) {
	for _, required := range []string{
		"Usage: kaboom [options]", "--state-dir <path>", "--parallel", "--doctor", "CLI Mode (direct tool access):",
	} {
		if !strings.Contains(HelpText, required) {
			t.Errorf("help text missing %q", required)
		}
	}
	if strings.Contains(HelpText, "--check") {
		t.Fatal("help text retains removed --check compatibility facade")
	}
}
