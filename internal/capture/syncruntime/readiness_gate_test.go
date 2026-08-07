// readiness_gate_test.go — Tests for cold-start readiness gate (WaitForExtensionConnected).
// Why: Validates that commands hold for up to ColdStartTimeout instead of failing instantly.
// Docs: docs/features/feature/cold-start-queuing/index.md

package syncruntime

import (
	"context"
	"testing"
	"time"
)

func requireReadinessResult(t *testing.T, result <-chan bool, want bool) {
	t.Helper()
	select {
	case connected := <-result:
		if connected != want {
			t.Fatalf("readiness result = %t, want %t", connected, want)
		}
	case <-time.After(time.Second):
		t.Fatal("readiness wait did not complete")
	}
}

func TestWaitForExtensionConnected_ConnectionTransitionClosesGeneration(t *testing.T) {
	t.Parallel()
	c := newTestState()
	t.Cleanup(c.Close)
	connected, notify := c.Extension().connectionReadinessSnapshot()
	if connected {
		t.Fatal("fresh extension runtime started connected")
	}

	connectForTest(c)
	select {
	case <-notify:
	default:
		t.Fatal("connection transition did not close readiness generation")
	}
	if !c.Extension().WaitForExtensionConnected(context.Background(), time.Second) {
		t.Fatal("connected runtime did not satisfy readiness wait")
	}
}

func TestWaitForExtensionConnected_NeverConnects(t *testing.T) {
	t.Parallel()
	c := newTestState()
	// Extension never connects — should timeout
	t.Cleanup(c.Close)

	ok := c.Extension().WaitForExtensionConnected(context.Background(), time.Millisecond)

	if ok {
		t.Fatal("expected WaitForExtensionConnected to return false when extension never connects")
	}
}

func TestWaitForExtensionConnected_ConnectsPartway(t *testing.T) {
	t.Parallel()
	c := newTestState()
	t.Cleanup(c.Close)
	result := make(chan bool, 1)
	go func() { result <- c.Extension().WaitForExtensionConnected(context.Background(), 2*time.Second) }()
	connectForTest(c)
	requireReadinessResult(t, result, true)
}

func TestWaitForExtensionConnected_ZeroTimeout(t *testing.T) {
	t.Parallel()
	c := newTestState()
	t.Cleanup(c.Close)

	// Zero timeout should behave like a single check
	ok := c.Extension().WaitForExtensionConnected(context.Background(), 0)
	if ok {
		t.Fatal("expected false with zero timeout and no connection")
	}
}

func TestWaitForExtensionConnected_ZeroTimeout_AlreadyConnected(t *testing.T) {
	t.Parallel()
	c := newTestState()
	t.Cleanup(c.Close)
	connectForTest(c)

	ok := c.Extension().WaitForExtensionConnected(context.Background(), 0)
	if !ok {
		t.Fatal("expected true with zero timeout when already connected")
	}
}

// P1-1: Verify context cancellation stops the wait and prevents goroutine leaks.
func TestWaitForExtensionConnected_ContextCancelled(t *testing.T) {
	t.Parallel()
	c := newTestState()
	t.Cleanup(c.Close)

	ctx, cancel := context.WithCancel(context.Background())

	result := make(chan bool, 1)
	go func() { result <- c.Extension().WaitForExtensionConnected(ctx, 5*time.Second) }()
	cancel()
	requireReadinessResult(t, result, false)
}

// P2-3: Connect-then-disconnect during wait returns after observing connection.
func TestWaitForExtensionConnected_ConnectsThenDisconnects(t *testing.T) {
	t.Parallel()
	c := newTestState()
	t.Cleanup(c.Close)
	result := make(chan bool, 1)
	go func() { result <- c.Extension().WaitForExtensionConnected(context.Background(), 2*time.Second) }()
	connectForTest(c)
	requireReadinessResult(t, result, true)
	c.Extension().MarkDisconnected()
	if c.Extension().IsExtensionConnected() {
		t.Fatal("extension remained connected after authoritative disconnect")
	}
}

// P2-4: Negative timeout should behave same as zero (single check, no wait).
func TestWaitForExtensionConnected_NegativeTimeout(t *testing.T) {
	t.Parallel()
	c := newTestState()
	t.Cleanup(c.Close)

	// Not connected — negative timeout should return false instantly
	ok := c.Extension().WaitForExtensionConnected(context.Background(), -1*time.Second)

	if ok {
		t.Fatal("expected false with negative timeout and no connection")
	}
}

func TestWaitForExtensionConnected_NegativeTimeout_AlreadyConnected(t *testing.T) {
	t.Parallel()
	c := newTestState()
	t.Cleanup(c.Close)
	connectForTest(c)

	ok := c.Extension().WaitForExtensionConnected(context.Background(), -1*time.Second)
	if !ok {
		t.Fatal("expected true with negative timeout when already connected")
	}
}

func TestWaitForTrackedURLChange_DirectTransitionClosesGeneration(t *testing.T) {
	t.Parallel()
	runtime := New()
	_, notify := runtime.trackingReadinessSnapshot()

	runtime.UpdateTrackedTab(42, "https://example.test/after", "After")

	select {
	case <-notify:
	default:
		t.Fatal("tracking transition did not close readiness generation")
	}
	url, changed := runtime.WaitForTrackedURLChange(context.Background(), "https://example.test/before", 0)
	if !changed || url != "https://example.test/after" {
		t.Fatalf("tracked URL result = %q, %t; want after, true", url, changed)
	}
}

func TestWaitForTrackedURLChange_SyncTransitionWakesWaiter(t *testing.T) {
	t.Parallel()
	state := newTestState()
	result := make(chan struct {
		url     string
		changed bool
	}, 1)
	go func() {
		url, changed := state.Extension().WaitForTrackedURLChange(
			context.Background(), "https://example.test/before", time.Second,
		)
		result <- struct {
			url     string
			changed bool
		}{url: url, changed: changed}
	}()

	runSyncRequest(t, state, SyncRequest{
		ExtSessionID: "tracking-transition",
		Settings: &SyncSettings{
			TrackingEnabled: true,
			TrackedTabID:    42,
			TrackedTabURL:   "https://example.test/after",
		},
	})

	select {
	case got := <-result:
		if !got.changed || got.url != "https://example.test/after" {
			t.Fatalf("tracked URL result = %q, %t; want after, true", got.url, got.changed)
		}
	case <-time.After(time.Second):
		t.Fatal("tracking wait did not wake after sync transition")
	}
}

func TestWaitForTrackedURLChange_TimeoutAndCancellation(t *testing.T) {
	t.Parallel()
	runtime := New()
	runtime.UpdateTrackedTab(42, "https://example.test/before", "Before")

	if url, changed := runtime.WaitForTrackedURLChange(context.Background(), "https://example.test/before", 0); changed || url != "https://example.test/before" {
		t.Fatalf("zero-timeout result = %q, %t; want before, false", url, changed)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if url, changed := runtime.WaitForTrackedURLChange(ctx, "https://example.test/before", time.Second); changed || url != "https://example.test/before" {
		t.Fatalf("cancelled result = %q, %t; want before, false", url, changed)
	}
}
