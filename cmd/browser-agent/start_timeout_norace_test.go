//go:build !race

// start_timeout_norace_test.go — Server-startup poll budget without the race detector.
// Keeps failure feedback fast in normal runs; the race build uses a larger ceiling.

package main

import (
	"os"
	"testing"
	"time"
)

var serverStartTimeout = testServerStartTimeout(testing.CoverMode(), os.Getenv("GOCOVERDIR"))

func testServerStartTimeout(coverageMode, coverageDir string) time.Duration {
	if coverageMode != "" || coverageDir != "" {
		return 30 * time.Second
	}
	return 5 * time.Second
}

func TestServerStartTimeoutAccountsForInstrumentation(t *testing.T) {
	if got := testServerStartTimeout("", ""); got != 5*time.Second {
		t.Fatalf("ordinary timeout = %v, want 5s", got)
	}
	for _, tc := range []struct {
		mode string
		dir  string
	}{
		{mode: "set"},
		{dir: "/tmp/go-build-cover"},
	} {
		if got := testServerStartTimeout(tc.mode, tc.dir); got != 30*time.Second {
			t.Fatalf("coverage timeout = %v, want 30s", got)
		}
	}
}
