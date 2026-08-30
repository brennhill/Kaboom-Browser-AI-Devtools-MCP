// procwatch_test.go — Proves a bridge notices its MCP client is gone, and never
// mistakes a healthy session for a dead one.

package procwatch_test

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/procwatch"
)

func TestParentGoneDetectsReparenting(t *testing.T) {
	cases := []struct {
		name              string
		original, current int
		want              bool
	}{
		{"same parent is alive", 4321, 4321, false},
		{"reparented to init means the client exited", 4321, 1, true},
		{"reparented to a subreaper also means the client exited", 4321, 999, true},
		{"already orphaned at start cannot be watched", 1, 1, false},
		{"unknown original parent cannot be watched", 0, 1, false},
		{"negative original parent cannot be watched", -1, 1, false},
	}
	for _, tc := range cases {
		if got := procwatch.ParentGoneForTest(tc.original, tc.current); got != tc.want {
			t.Errorf("%s: ParentGone(%d, %d) = %v, want %v", tc.name, tc.original, tc.current, got, tc.want)
		}
	}
}

// A bridge exists to serve exactly one stdio client. When that client dies the
// bridge has nothing left to do, and every one that lingers holds ~24MB forever:
// two such bridges were alive for 31 hours on one developer machine.
func TestWatchFiresWhenTheParentDisappears(t *testing.T) {
	ppid := os.Getppid()
	// Atomic: the watcher goroutine reads this while the test writes it.
	var current atomic.Int64
	current.Store(int64(ppid))
	fired := make(chan string, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go procwatch.Watch(ctx, procwatch.Config{
		OriginalPPID: ppid,
		Poll:         5 * time.Millisecond,
		CurrentPPID:  func() int { return int(current.Load()) },
	}, func(reason string) { fired <- reason })

	select {
	case reason := <-fired:
		t.Fatalf("Watch() fired while the parent was alive: %s", reason)
	case <-time.After(50 * time.Millisecond):
	}

	current.Store(1) // the client exits; we are adopted by init
	select {
	case reason := <-fired:
		if reason == "" {
			t.Error("Watch() fired with no reason")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Watch() never fired after the parent disappeared")
	}
}

func TestWatchStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		procwatch.Watch(ctx, procwatch.Config{
			OriginalPPID: 4321,
			Poll:         5 * time.Millisecond,
			CurrentPPID:  func() int { return 4321 },
		}, func(string) { t.Error("Watch() fired after cancel") })
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Watch() did not return when its context was cancelled")
	}
}

// A daemon is deliberately orphaned at birth. Watching its parent would make it
// exit immediately, so an unwatchable parent must be a no-op, not a shutdown.
func TestWatchIsANoOpWhenAlreadyOrphaned(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() {
		procwatch.Watch(ctx, procwatch.Config{
			OriginalPPID: 1,
			Poll:         5 * time.Millisecond,
			CurrentPPID:  func() int { return 1 },
		}, func(string) { t.Error("Watch() fired for an already-orphaned process") })
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Watch() did not return for an unwatchable parent")
	}
}
