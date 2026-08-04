// Purpose: Tests for capture navigation callback handling.
// Docs: docs/features/feature/backend-log-streaming/index.md

// navigation_callback_test.go — Tests for navigation action callback.
package capture

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"sync/atomic"
	"testing"
	"time"
)

func runNavigationCallbacksSynchronously(c *Capture) {
	c.Telemetry().dispatchCallback = func(callback func()) { callback() }
}

// ============================================
// SetNavigationCallback Tests
// ============================================

func TestNavigationCallback_FiredOnNavigationAction(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)
	runNavigationCallbacksSynchronously(c)

	var called atomic.Int32
	c.Telemetry().SetNavigationCallback(func() {
		called.Add(1)
	})

	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{
		{Type: "navigation", Timestamp: time.Now().UnixMilli()},
	})

	if got := called.Load(); got != 1 {
		t.Errorf("navigation callback called %d times, want 1", got)
	}
}

func TestNavigationCallback_NotFiredOnNonNavigationAction(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)
	runNavigationCallbacksSynchronously(c)

	var called atomic.Int32
	c.Telemetry().SetNavigationCallback(func() {
		called.Add(1)
	})

	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{
		{Type: "click", Timestamp: time.Now().UnixMilli()},
		{Type: "type", Timestamp: time.Now().UnixMilli()},
		{Type: "scroll", Timestamp: time.Now().UnixMilli()},
	})

	if got := called.Load(); got != 0 {
		t.Errorf("navigation callback called %d times for non-navigation actions, want 0", got)
	}
}

func TestNavigationCallback_FiredOnceForMultipleNavigationsInBatch(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)
	runNavigationCallbacksSynchronously(c)

	var called atomic.Int32
	c.Telemetry().SetNavigationCallback(func() {
		called.Add(1)
	})

	// Two navigation actions in the same batch should fire callback only once
	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{
		{Type: "navigation", Timestamp: time.Now().UnixMilli()},
		{Type: "click", Timestamp: time.Now().UnixMilli()},
		{Type: "navigation", Timestamp: time.Now().UnixMilli()},
	})

	if got := called.Load(); got != 1 {
		t.Errorf("navigation callback called %d times for batch with 2 navigations, want 1", got)
	}
}

func TestNavigationCallback_NotSetDoesNotPanic(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)
	runNavigationCallbacksSynchronously(c)

	// No callback set — should not panic
	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{
		{Type: "navigation", Timestamp: time.Now().UnixMilli()},
	})
}

func TestNavigationCallback_NilCallbackDoesNotPanic(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	c.Telemetry().SetNavigationCallback(nil)

	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{
		{Type: "navigation", Timestamp: time.Now().UnixMilli()},
	})
}

func TestNavigationCallback_FiredOutsideLock(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)
	runNavigationCallbacksSynchronously(c)

	// Verify the callback is invoked outside the telemetry lock by attempting
	// to acquire the lock inside the callback (would deadlock if still held).
	called := false
	c.Telemetry().SetNavigationCallback(func() {
		// This would deadlock if callback ran while the telemetry owner held its lock.
		count := len(c.Telemetry().GetAllEnhancedActions())
		if count == 0 {
			t.Error("expected actions to be stored before callback fires")
		}
		called = true
	})

	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{
		{Type: "navigation", Timestamp: time.Now().UnixMilli()},
	})

	if !called {
		t.Fatal("navigation callback did not complete outside the telemetry lock")
	}
}
