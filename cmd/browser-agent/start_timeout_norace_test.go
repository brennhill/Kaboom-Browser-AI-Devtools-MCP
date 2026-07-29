//go:build !race

// start_timeout_norace_test.go — Server-startup poll budget without the race detector.
// Keeps failure feedback fast in normal runs; the race build uses a larger ceiling.

package main

import (
	"testing"
	"time"
)

var serverStartTimeout = testServerStartTimeout(testing.CoverMode())

func testServerStartTimeout(coverageMode string) time.Duration {
	if coverageMode != "" {
		return 30 * time.Second
	}
	return 5 * time.Second
}

func TestServerStartTimeoutAccountsForInstrumentation(t *testing.T) {
	if got := testServerStartTimeout(""); got != 5*time.Second {
		t.Fatalf("ordinary timeout = %v, want 5s", got)
	}
	if got := testServerStartTimeout("set"); got != 30*time.Second {
		t.Fatalf("coverage timeout = %v, want 30s", got)
	}
}
