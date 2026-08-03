//go:build !race

// start_timeout_norace_test.go — Server-startup poll budget without the race detector.
// Keeps failure feedback fast in normal runs; the race build uses a larger ceiling.

package main

import (
	"os"
	"testing"
	"time"
)

var coverageInstrumentedTest = testUsesCoverageInstrumentation(
	testing.CoverMode(),
	os.Getenv("GOCOVERDIR"),
	os.Getenv("KABOOM_GO_COVERDIR"),
)

var serverStartTimeout = testServerStartTimeout(coverageInstrumentedTest)

func testUsesCoverageInstrumentation(coverageMode, coverageDir, subprocessCoverageDir string) bool {
	return coverageMode != "" || coverageDir != "" || subprocessCoverageDir != ""
}

func testServerStartTimeout(instrumented bool) time.Duration {
	if instrumented {
		return 30 * time.Second
	}
	return 5 * time.Second
}

func integrationResponseTimeout(ordinary time.Duration) time.Duration {
	if coverageInstrumentedTest {
		return 30 * time.Second
	}
	return ordinary
}

func TestServerStartTimeoutAccountsForInstrumentation(t *testing.T) {
	if got := testServerStartTimeout(false); got != 5*time.Second {
		t.Fatalf("ordinary timeout = %v, want 5s", got)
	}
	if got := testServerStartTimeout(true); got != 30*time.Second {
		t.Fatalf("coverage timeout = %v, want 30s", got)
	}
	for _, tc := range []struct {
		mode          string
		dir           string
		subprocessDir string
	}{
		{mode: "set"},
		{dir: "/tmp/go-build-cover"},
		{subprocessDir: "/tmp/kaboom-subprocess-cover"},
	} {
		if !testUsesCoverageInstrumentation(tc.mode, tc.dir, tc.subprocessDir) {
			t.Fatalf("coverage environment %+v was not detected", tc)
		}
	}
	if testUsesCoverageInstrumentation("", "", "") {
		t.Fatal("ordinary test environment detected as instrumented")
	}
}
