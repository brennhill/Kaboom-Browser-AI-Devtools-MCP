// harness_test.go — Verifies deterministic browser-agent integration harness policy.

package integrationtest

import (
	"fmt"
	"net"
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

func TestConfiguredBinaryUsesRunnerArtifact(t *testing.T) {
	t.Setenv("KABOOM_INTEGRATION_BINARY", "/tmp/prebuilt-kaboom")
	if got := configuredBinary(); got != "/tmp/prebuilt-kaboom" {
		t.Fatalf("configured binary = %q", got)
	}
}

func TestRunnerInstrumentationActivatesExpandedBudget(t *testing.T) {
	t.Setenv("KABOOM_INTEGRATION_INSTRUMENTED", "1")
	if !instrumented() {
		t.Fatal("runner-built covered binary was not classified as instrumented")
	}
}

// FreePort used to bind 127.0.0.1:0, so every integration daemon landed on an
// OS-assigned port. That is safely away from the user's 7890, but a daemon that
// outlives its test then sits on an unknown number: the hunt for twelve stale
// daemons had to match on process name because nothing could be found by port.
//
// The band makes a leak findable. Binding to claim is what makes it safe —
// the OS arbitrates, so two parallel packages cannot be handed the same port
// however their scans overlap.
func TestFreePortPrefersTheReservedBand(t *testing.T) {
	port := FreePort(t)
	if port < ReservedPortBase || port > ReservedPortEnd {
		t.Fatalf("FreePort returned %d, outside the reserved band %d-%d", port, ReservedPortBase, ReservedPortEnd)
	}
}

// The band must never include the user's daemon or its terminal server.
func TestReservedBandExcludesTheUserPorts(t *testing.T) {
	for _, userPort := range []int{7890, 7891} {
		if userPort >= ReservedPortBase && userPort <= ReservedPortEnd {
			t.Fatalf("reserved band %d-%d contains the user port %d", ReservedPortBase, ReservedPortEnd, userPort)
		}
	}
	// 7899 belongs to the CLI integration suite, which pins it by name.
	if ReservedPortBase <= 7899 {
		t.Fatalf("band base %d collides with the CLI suite's reserved 7899", ReservedPortBase)
	}
}

// Ports must not be handed out twice while still held.
func TestFreePortDoesNotRepeatWhilePortsAreHeld(t *testing.T) {
	seen := map[int]bool{}
	held := []net.Listener{}
	t.Cleanup(func() {
		for _, l := range held {
			_ = l.Close()
		}
	})
	for i := 0; i < 12; i++ {
		port := FreePort(t)
		if seen[port] {
			t.Fatalf("FreePort returned %d twice while it was still held", port)
		}
		seen[port] = true
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			t.Fatalf("FreePort returned %d but it could not be bound: %v", port, err)
		}
		held = append(held, listener)
	}
}

// A full band must degrade to an ephemeral port rather than failing a test run.
func TestFreePortFallsBackWhenTheBandIsExhausted(t *testing.T) {
	port := freePortInBand(ReservedPortEnd, ReservedPortBase) // empty range: base > end
	if port != 0 {
		t.Fatalf("an empty band should yield no port, got %d", port)
	}
	if got := FreePort(t); got <= 0 {
		t.Fatalf("FreePort must always return a usable port, got %d", got)
	}
}
