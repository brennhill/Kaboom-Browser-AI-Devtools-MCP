// Purpose: Tests for capture query lifecycle management.
// Docs: docs/features/feature/backend-log-streaming/index.md

// queries_lifecycle_test.go — Tests for query goroutine lifecycle fixes.
// Covers: startResultCleanup stop mechanism, WaitForResult goroutine control,
// and Close() method for Capture cleanup.
package capture

import (
	"encoding/json"
	"runtime"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testsync"
)

// ============================================
// queryResultTTL: must be long enough for multi-step agents
// ============================================

func TestQueryResultTTL_FiveMinutes(t *testing.T) {
	t.Parallel()
	if queryResultTTL != 5*time.Minute {
		t.Fatalf("queryResultTTL = %v, want 5m", queryResultTTL)
	}
}

// ============================================
// startResultCleanup: stop function
// ============================================

func TestStartResultCleanup_ReturnsStopFunction(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()

	// Close calls stopCleanup internally; verify it returns promptly
	done := make(chan struct{})
	go func() {
		c.Close()
		close(done)
	}()

	select {
	case <-done:
		// Good
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return within 2 seconds")
	}
}

func TestStartResultCleanup_GoroutineStopsOnClose(t *testing.T) {
	// NOT parallel: relies on runtime.NumGoroutine() counts
	before := testsync.SettledGoroutines()

	c := NewCapture()

	// Goroutine count should have increased (cleanup goroutine started)
	time.Sleep(20 * time.Millisecond)
	during := runtime.NumGoroutine()
	if during <= before {
		t.Logf("Warning: goroutine count did not visibly increase: before=%d, during=%d", before, during)
	}

	// Teardown is not synchronous with Close(); poll instead of guessing how long
	// the scheduler needs.
	c.Close()
	testsync.EventuallyGoroutines(t, before+1, "capture goroutines to exit after Close")
}

func TestClose_Idempotent(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Multiple Close calls should not panic
	c.Close()
	c.Close()
	c.Close()
}

// ============================================
// WaitForResult: goroutine control
// ============================================

func TestWaitForResult_NoGoroutineLeakOnTimeout(t *testing.T) {
	// NOT parallel: relies on runtime.NumGoroutine() counts
	c := NewCapture()
	defer c.Close()

	id, _ := c.CreatePendingQuery(queries.PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"#leak-test"}`),
	})

	before := testsync.SettledGoroutines()

	// This will timeout — the key assertion is no goroutine leak after
	_, err := c.WaitForResult(id, 80*time.Millisecond)
	if err == nil {
		t.Fatal("Expected timeout error")
	}

	testsync.EventuallyGoroutines(t, before+1, "WaitForResult goroutines to exit after timeout")
}

func TestWaitForResult_MultipleTimeoutsNoLeak(t *testing.T) {
	// NOT parallel: relies on runtime.NumGoroutine() counts
	c := NewCapture()
	defer c.Close()

	// Short query timeout so CreatePendingQuery cleanup goroutines exit quickly
	c.SetQueryTimeout(40 * time.Millisecond)

	before := testsync.SettledGoroutines()

	for i := 0; i < 6; i++ {
		id, _ := c.CreatePendingQuery(queries.PendingQuery{
			Type:   "dom",
			Params: json.RawMessage(`{"selector":"#leak-test"}`),
		})
		_, _ = c.WaitForResult(id, 40*time.Millisecond)
	}

	// Old behavior: ~100 leaked goroutines (per-iteration spawns in WaitForResult loop).
	// Fixed: 1 wakeup goroutine per WaitForResult call, cleaned up on return.
	testsync.EventuallyGoroutines(t, before+3, "per-query cleanup goroutines to exit after 6 timeouts")
}

func TestWaitForResult_ReturnsResultWhenAvailable(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()

	id, _ := c.CreatePendingQuery(queries.PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"#test"}`),
	})

	// Post result after a short delay
	go func() {
		time.Sleep(20 * time.Millisecond)
		c.SetQueryResult(id, json.RawMessage(`{"found": true}`))
	}()

	result, err := c.WaitForResult(id, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if string(result) != `{"found": true}` {
		t.Errorf("Unexpected result: %s", result)
	}
}
