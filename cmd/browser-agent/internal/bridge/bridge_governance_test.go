// bridge_governance_test.go — Proves a bridge registers itself in the machine
// census and stands itself down when its MCP client disappears.

package bridge

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancereg"
)

func TestGovernBridgeRegistersAndDeregisters(t *testing.T) {
	t.Setenv(instancereg.DirEnv, t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	release := governBridge(ctx, bridgeGovernance{Version: "0.9.0", Port: 7890}, func(string) {})

	records, err := instancereg.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("registry has %d records, want 1 bridge", len(records))
	}
	if records[0].Role != instancereg.RoleBridge {
		t.Fatalf("registered role = %q, want bridge", records[0].Role)
	}
	if len(records[0].Ports) != 1 || records[0].Ports[0] != 7890 {
		t.Errorf("registered ports = %v, want [7890]", records[0].Ports)
	}

	cancel()
	release()
	records, _ = instancereg.List()
	if len(records) != 0 {
		t.Fatalf("release left %d records behind", len(records))
	}
}

// A bridge serves exactly one stdio client. stdin EOF covers the clean exit; this
// covers the client that was SIGKILLed and never closed the pipe, which is how two
// bridges came to be alive after 31 hours holding ~24MB each.
func TestGovernBridgeStandsDownWhenTheClientDies(t *testing.T) {
	t.Setenv(instancereg.DirEnv, t.TempDir())

	parent := 4321
	// Atomic: the watcher reads this from its own goroutine while the test writes
	// it, which is a data race under -race if it is a plain int.
	var current atomic.Int64
	current.Store(int64(parent))
	stood := make(chan string, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	release := governBridge(ctx, bridgeGovernance{
		Version: "0.9.0", Port: 7890,
		OriginalPPID: parent,
		ParentPoll:   5 * time.Millisecond,
		CurrentPPID:  func() int { return int(current.Load()) },
	}, func(reason string) { stood <- reason })
	defer release()

	select {
	case reason := <-stood:
		t.Fatalf("bridge stood down while its client was alive: %s", reason)
	case <-time.After(50 * time.Millisecond):
	}

	current.Store(1) // the MCP client exits
	select {
	case reason := <-stood:
		if reason == "" {
			t.Error("bridge stood down with no reason")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("bridge never stood down after its client disappeared")
	}
}

// A registry failure must not prevent the bridge from serving: the census is
// bookkeeping, and losing it is not a reason to break a working MCP session.
func TestGovernBridgeSurvivesAnUnavailableRegistry(t *testing.T) {
	t.Setenv(instancereg.DirEnv, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	release := governBridge(ctx, bridgeGovernance{Version: "0.9.0", Port: 7890}, func(string) {})
	if release == nil {
		t.Fatal("governBridge() returned no release function when the registry was unavailable")
	}
	release()
}
