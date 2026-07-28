// main_flags_test.go — Tests the canonical daemon CLI flag surface.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"flag"
	"io"
	"os"
	"testing"
)

func withTestFlagSet(t *testing.T, args []string, run func()) {
	t.Helper()
	originalArgs := os.Args
	originalCommandLine := flag.CommandLine
	defer func() {
		os.Args = originalArgs
		flag.CommandLine = originalCommandLine
	}()

	flag.CommandLine = flag.NewFlagSet(originalArgs[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = append([]string{originalArgs[0]}, args...)
	run()
}

func TestRegisterFlagsAcceptsParallel(t *testing.T) {
	withTestFlagSet(t, []string{"--parallel"}, func() {
		parsed := registerFlags()
		if parsed == nil || parsed.parallelMode == nil || !*parsed.parallelMode {
			t.Fatalf("registerFlags() parallelMode = %v, want true", parsed)
		}
	})
}

func TestRegisterFlagsOmitsDeprecatedCompatibilityFlags(t *testing.T) {
	withTestFlagSet(t, nil, func() {
		registerFlags()
		for _, name := range []string{"mcp", "persist", "check"} {
			if flag.Lookup(name) != nil {
				t.Errorf("deprecated compatibility flag --%s is still registered", name)
			}
		}
	})
}
