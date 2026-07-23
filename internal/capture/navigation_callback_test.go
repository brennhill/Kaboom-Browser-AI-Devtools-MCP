// Purpose: Tests for capture navigation callback handling.
// Docs: docs/features/feature/backend-log-streaming/index.md

// navigation_callback_test.go — Tests for navigation action callback.
package capture

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testsync"
)

// ============================================
// SetNavigationCallback Tests
// ============================================

func TestNavigationCallback_FiredOnNavigationAction(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	var called atomic.Int32
	c.SetNavigationCallback(func() {
		called.Add(1)
	})

	c.AddEnhancedActions([]EnhancedAction{
		{Type: "navigation", Timestamp: time.Now().UnixMilli()},
	})

	testsync.Eventually(t, testsync.DefaultTimeout, "the navigation callback to fire", func() bool {
		return called.Load() == 1
	})
}

func TestNavigationCallback_NotFiredOnNonNavigationAction(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	var called atomic.Int32
	c.SetNavigationCallback(func() {
		called.Add(1)
	})

	c.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: time.Now().UnixMilli()},
		{Type: "type", Timestamp: time.Now().UnixMilli()},
		{Type: "scroll", Timestamp: time.Now().UnixMilli()},
	})

	// Negative assertion: there is no event to wait for, so a settle window is
	// the only way to observe "did not fire".
	time.Sleep(50 * time.Millisecond)

	if got := called.Load(); got != 0 {
		t.Errorf("navigation callback called %d times for non-navigation actions, want 0", got)
	}
}

func TestNavigationCallback_FiredOnceForMultipleNavigationsInBatch(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	var called atomic.Int32
	c.SetNavigationCallback(func() {
		called.Add(1)
	})

	// Two navigation actions in the same batch should fire callback only once
	c.AddEnhancedActions([]EnhancedAction{
		{Type: "navigation", Timestamp: time.Now().UnixMilli()},
		{Type: "click", Timestamp: time.Now().UnixMilli()},
		{Type: "navigation", Timestamp: time.Now().UnixMilli()},
	})

	testsync.Eventually(t, testsync.DefaultTimeout, "the batched navigation callback to fire", func() bool {
		return called.Load() == 1
	})
	// Coalescing is the point: give a second firing a chance to show up.
	time.Sleep(50 * time.Millisecond)
	if got := called.Load(); got != 1 {
		t.Errorf("navigation callback called %d times for batch with 2 navigations, want 1", got)
	}
}

func TestNavigationCallback_NotSetDoesNotPanic(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	// No callback set — should not panic
	c.AddEnhancedActions([]EnhancedAction{
		{Type: "navigation", Timestamp: time.Now().UnixMilli()},
	})
}

func TestNavigationCallback_NilCallbackDoesNotPanic(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	c.SetNavigationCallback(nil)

	c.AddEnhancedActions([]EnhancedAction{
		{Type: "navigation", Timestamp: time.Now().UnixMilli()},
	})
}

func TestNavigationCallback_FiredOutsideLock(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	// Verify the callback is invoked outside the Capture.mu lock by attempting
	// to acquire the lock inside the callback (would deadlock if still held).
	var wg sync.WaitGroup
	wg.Add(1)
	c.SetNavigationCallback(func() {
		defer wg.Done()
		// This would deadlock if callback is called inside c.mu.Lock
		count := c.GetEnhancedActionCount()
		if count == 0 {
			t.Error("expected actions to be stored before callback fires")
		}
	})

	c.AddEnhancedActions([]EnhancedAction{
		{Type: "navigation", Timestamp: time.Now().UnixMilli()},
	})

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success — callback completed without deadlock
	case <-time.After(2 * time.Second):
		t.Fatal("navigation callback appears to deadlock (possibly called inside lock)")
	}
}
