// main_flags_test.go — Tests the canonical daemon CLI flag surface.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package runtimeflags

import (
	"testing"
)

func TestParseAcceptsParallel(t *testing.T) {
	parsed, err := Parse([]string{"--parallel"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.ParallelMode {
		t.Fatal("ParallelMode = false, want true")
	}
}

func TestParseRejectsDeprecatedCompatibilityFlags(t *testing.T) {
	for _, name := range []string{"mcp", "persist", "check"} {
		if _, err := Parse([]string{"--" + name}, ""); err == nil {
			t.Errorf("deprecated compatibility flag --%s was accepted", name)
		}
	}
}

func TestParseUsesExplicitAPIKeyDefault(t *testing.T) {
	parsed, err := Parse(nil, "environment-key")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.APIKey != "environment-key" {
		t.Fatalf("APIKey = %q", parsed.APIKey)
	}
}
