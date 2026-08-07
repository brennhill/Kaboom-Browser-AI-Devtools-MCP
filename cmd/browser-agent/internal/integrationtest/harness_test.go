// harness_test.go — Verifies deterministic browser-agent integration harness policy.

package integrationtest

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildArgsCoverAllProductionPackagesOnlyWhenRequested(t *testing.T) {
	ordinary := strings.Join(buildArgs("/tmp/kaboom", false), " ")
	if ordinary != "build -cover -o /tmp/kaboom ." {
		t.Fatalf("ordinary args = %q", ordinary)
	}
	covered := strings.Join(buildArgs("/tmp/kaboom", true), " ")
	if covered != "build -cover -coverpkg=./... -o /tmp/kaboom ." {
		t.Fatalf("covered args = %q", covered)
	}
}

func TestSourceDirectoryTargetsBrowserAgentRegardlessOfWorkingDirectory(t *testing.T) {
	directory := sourceDirectory()
	if filepath.Base(directory) != "browser-agent" || filepath.Base(filepath.Dir(directory)) != "cmd" {
		t.Fatalf("source directory = %q", directory)
	}
}

func TestTimeoutPolicyAccountsForInstrumentation(t *testing.T) {
	if got := startTimeout(false); got != 5*time.Second {
		t.Fatalf("ordinary timeout = %v", got)
	}
	if got := startTimeout(true); got != 30*time.Second {
		t.Fatalf("instrumented timeout = %v", got)
	}
	if !usesCoverage("set", "", "") || !usesCoverage("", "/tmp/cover", "") || !usesCoverage("", "", "/tmp/subprocess") {
		t.Fatal("coverage environment was not detected")
	}
	if usesCoverage("", "", "") {
		t.Fatal("ordinary environment detected as instrumented")
	}
}
