// readiness_gate_test.go — Tests for cold-start readiness gate (WaitForExtensionConnected).
// Why: Validates that commands hold for up to ColdStartTimeout instead of failing instantly.
// Docs: docs/features/feature/cold-start-queuing/index.md

package capture

import (
	"context"
	"testing"
	"time"
)

func TestWaitForExtensionConnected_NeverConnects(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	// Extension never connects — should timeout

	timeout := 200 * time.Millisecond
	start := time.Now()
	ok := c.Extension().WaitForExtensionConnected(context.Background(), timeout)
	elapsed := time.Since(start)

	if ok {
		t.Fatal("expected WaitForExtensionConnected to return false when extension never connects")
	}
	if elapsed < timeout-20*time.Millisecond {
		t.Fatalf("expected to wait at least %v, only waited %v", timeout, elapsed)
	}
}

func TestWaitForExtensionConnected_ConnectsPartway(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Simulate connection after 150ms
	go func() {
		time.Sleep(150 * time.Millisecond)
		c.Extension().SimulateExtensionConnectForTest()
	}()

	start := time.Now()
	ok := c.Extension().WaitForExtensionConnected(context.Background(), 2*time.Second)
	elapsed := time.Since(start)

	if !ok {
		t.Fatal("expected WaitForExtensionConnected to return true after mid-wait connection")
	}
	// Should have detected connection between 150ms and 350ms (150ms + poll interval slack)
	if elapsed < 100*time.Millisecond {
		t.Fatalf("detected connection too fast (%v), connection fires at 150ms", elapsed)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("took too long to detect connection (%v), expected within 500ms", elapsed)
	}
}

func TestWaitForExtensionConnected_ZeroTimeout(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Zero timeout should behave like a single check
	ok := c.Extension().WaitForExtensionConnected(context.Background(), 0)
	if ok {
		t.Fatal("expected false with zero timeout and no connection")
	}
}

func TestWaitForExtensionConnected_ZeroTimeout_AlreadyConnected(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	c.Extension().SimulateExtensionConnectForTest()

	ok := c.Extension().WaitForExtensionConnected(context.Background(), 0)
	if !ok {
		t.Fatal("expected true with zero timeout when already connected")
	}
}

// P1-1: Verify context cancellation stops the wait and prevents goroutine leaks.
func TestWaitForExtensionConnected_ContextCancelled(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	ok := c.Extension().WaitForExtensionConnected(ctx, 5*time.Second)
	elapsed := time.Since(start)

	if ok {
		t.Fatal("expected false when context is cancelled and extension never connected")
	}
	// Should return within ~150ms (100ms cancel + poll interval slack), not 5s
	if elapsed > 500*time.Millisecond {
		t.Fatalf("context cancellation should stop wait promptly, took %v", elapsed)
	}
	if elapsed < 50*time.Millisecond {
		t.Fatalf("should have waited for context cancel, only waited %v", elapsed)
	}
}

// P2-3: Connect-then-disconnect during wait — lastSyncSeen is still recent so returns true.
// NOTE: This test mutates a package-level var (extensionDisconnectThreshold)
// via SetExtensionDisconnectThresholdForTesting, so it must NOT use t.Parallel().
func TestWaitForExtensionConnected_ConnectsThenDisconnects(t *testing.T) {
	c := NewCapture()
	// Use a short disconnect threshold so the test can verify disconnect detection.
	restore := SetExtensionDisconnectThresholdForTesting(500 * time.Millisecond)
	defer restore()

	// Simulate connect at 50ms — early enough that the first poll tick (100ms)
	// reliably sees the connected state.
	go func() {
		time.Sleep(50 * time.Millisecond)
		c.Extension().SimulateExtensionConnectForTest()
	}()

	// Simulate disconnect at 400ms — gives at least 3 poll ticks (100ms each)
	// to detect the connection before it goes away. The previous 200ms delay
	// left only a ~100ms window which caused flaky failures when the poll tick
	// aligned with the disconnect.
	go func() {
		time.Sleep(400 * time.Millisecond)
		c.Extension().SimulateExtensionDisconnectForTest()
	}()

	// Start waiting — extension connects at 50ms. The poll at ~100ms should catch
	// the connected state. Even though disconnect fires at 400ms, the poll at
	// ~100ms should see the connection well before the disconnect.
	start := time.Now()
	ok := c.Extension().WaitForExtensionConnected(context.Background(), 2*time.Second)
	elapsed := time.Since(start)

	if !ok {
		t.Fatal("expected WaitForExtensionConnected to return true — lastSyncSeen was recent when polled")
	}
	if elapsed < 30*time.Millisecond {
		t.Fatalf("detected connection too fast (%v), connection fires at 50ms", elapsed)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("took too long to detect connection (%v)", elapsed)
	}
}

// P2-4: Negative timeout should behave same as zero (single check, no wait).
func TestWaitForExtensionConnected_NegativeTimeout(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Not connected — negative timeout should return false instantly
	start := time.Now()
	ok := c.Extension().WaitForExtensionConnected(context.Background(), -1*time.Second)
	elapsed := time.Since(start)

	if ok {
		t.Fatal("expected false with negative timeout and no connection")
	}
	if elapsed > 50*time.Millisecond {
		t.Fatalf("negative timeout should be instant, took %v", elapsed)
	}
}

func TestWaitForExtensionConnected_NegativeTimeout_AlreadyConnected(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	c.Extension().SimulateExtensionConnectForTest()

	ok := c.Extension().WaitForExtensionConnected(context.Background(), -1*time.Second)
	if !ok {
		t.Fatal("expected true with negative timeout when already connected")
	}
}
