// Purpose: Stress tests for capture pipeline under concurrent load.
// Docs: docs/features/feature/backend-log-streaming/index.md

// capture_stress_test.go — Concurrent stress tests for capture system.
package capture

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

type stressStartGate struct {
	ready sync.WaitGroup
	start chan struct{}
}

func newStressStartGate(participants int) *stressStartGate {
	gate := &stressStartGate{start: make(chan struct{})}
	gate.ready.Add(participants)
	return gate
}

func (gate *stressStartGate) awaitRelease() {
	gate.ready.Done()
	<-gate.start
}

func (gate *stressStartGate) release() {
	gate.ready.Wait()
	close(gate.start)
}

// TestStressCaptureSystemConcurrent verifies thread-safety of the Capture system
// under heavy concurrent load. Launches multiple goroutines adding WebSocket events,
// network bodies, enhanced actions, and reading from all buffers simultaneously.
// Designed to be run with -race to detect data races.
func TestStressCaptureSystemConcurrent(t *testing.T) {
	t.Run("concurrent_stress", func(t *testing.T) {
		const (
			numWSWriters     = 10
			numNetWriters    = 10
			numActionWriters = 10
			numReaders       = 10
			eventsPerWriter  = 50
			bodiesPerWriter  = 50
			actionsPerWriter = 50
			readsPerReader   = 20
		)

		c := NewCapture()
		defer c.Close()

		var wg sync.WaitGroup
		gate := newStressStartGate(numWSWriters + numNetWriters + numActionWriters + numReaders)

		// Launch WebSocket event writers
		for writerID := 0; writerID < numWSWriters; writerID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				gate.awaitRelease()
				for i := 0; i < eventsPerWriter; i++ {
					events := []types.WebSocketEvent{
						{
							ID:        fmt.Sprintf("ws-%d-%d", id, i),
							Event:     "message",
							URL:       fmt.Sprintf("wss://example.com/ws-%d", id),
							Direction: "incoming",
							Data:      fmt.Sprintf("data-%d-%d", id, i),
							Size:      100,
							Timestamp: time.Now().Format(time.RFC3339Nano),
						},
					}
					c.Telemetry().AddWebSocketEvents(events)
				}
			}(writerID)
		}

		// Launch network body writers
		for writerID := 0; writerID < numNetWriters; writerID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				gate.awaitRelease()
				for i := 0; i < bodiesPerWriter; i++ {
					bodies := []types.NetworkBody{
						{
							URL:          fmt.Sprintf("https://example.com/api-%d/%d", id, i),
							Method:       "POST",
							Status:       200,
							RequestBody:  fmt.Sprintf(`{"request":%d}`, i),
							ResponseBody: fmt.Sprintf(`{"response":%d}`, i),
							ContentType:  "application/json",
							Duration:     50,
							Timestamp:    time.Now().Format(time.RFC3339Nano),
						},
					}
					c.Telemetry().AddNetworkBodies(bodies)
				}
			}(writerID)
		}

		// Launch enhanced action writers
		for writerID := 0; writerID < numActionWriters; writerID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				gate.awaitRelease()
				for i := 0; i < actionsPerWriter; i++ {
					actions := []types.EnhancedAction{
						{
							Type:      "click",
							Timestamp: time.Now().UnixNano(),
							URL:       fmt.Sprintf("https://example.com/page-%d", id),
							Selectors: map[string]any{
								"css": fmt.Sprintf("button-%d", i),
							},
						},
					}
					c.Telemetry().AddEnhancedActions(actions)
				}
			}(writerID)
		}

		// Launch concurrent readers
		for readerID := 0; readerID < numReaders; readerID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				gate.awaitRelease()
				for i := 0; i < readsPerReader; i++ {
					// Read from all three buffers
					_ = c.Telemetry().GetAllWebSocketEvents()
					_ = c.Telemetry().GetNetworkBodies()
					_ = c.Telemetry().GetAllEnhancedActions()

					// Yield to allow writers to interleave
					if i%5 == 0 {
						runtime.Gosched()
					}
				}
			}(readerID)
		}

		gate.release()
		// Wait for all goroutines to complete
		wg.Wait()

		// Verify final state invariants
		wsEvents := c.Telemetry().GetAllWebSocketEvents()
		networkBodies := c.Telemetry().GetNetworkBodies()
		actions := c.Telemetry().GetAllEnhancedActions()

		// Buffer capacity bounds must hold
		if len(wsEvents) > maxWSEvents {
			t.Errorf("WS events %d exceeds maxWSEvents %d", len(wsEvents), maxWSEvents)
		}
		if len(networkBodies) > maxNetworkBodies {
			t.Errorf("Network bodies %d exceeds maxNetworkBodies %d", len(networkBodies), maxNetworkBodies)
		}
		if len(actions) > maxEnhancedActions {
			t.Errorf("Enhanced actions %d exceeds maxEnhancedActions %d", len(actions), maxEnhancedActions)
		}

		// With no clears, buffers must not be empty (we wrote plenty of data)
		totalWSWritten := numWSWriters * eventsPerWriter
		totalNetWritten := numNetWriters * bodiesPerWriter
		totalActionsWritten := numActionWriters * actionsPerWriter

		if len(wsEvents) == 0 {
			t.Errorf("Expected WS events > 0 after writing %d", totalWSWritten)
		}
		if len(networkBodies) == 0 {
			t.Errorf("Expected network bodies > 0 after writing %d", totalNetWritten)
		}
		if len(actions) == 0 {
			t.Errorf("Expected actions > 0 after writing %d", totalActionsWritten)
		}

		// Snapshot counts must match actual buffer lengths
		snap := c.Telemetry().GetSnapshot()
		if snap.WebSocketCount != len(wsEvents) {
			t.Errorf("Snapshot.WebSocketCount %d != len(wsEvents) %d", snap.WebSocketCount, len(wsEvents))
		}
		if snap.NetworkCount != len(networkBodies) {
			t.Errorf("Snapshot.NetworkCount %d != len(networkBodies) %d", snap.NetworkCount, len(networkBodies))
		}
		if snap.ActionCount != len(actions) {
			t.Errorf("Snapshot.ActionCount %d != len(actions) %d", snap.ActionCount, len(actions))
		}

		t.Logf("Stress test completed: wrote %d/%d/%d (ws/net/act), final %d/%d/%d",
			totalWSWritten, totalNetWritten, totalActionsWritten,
			len(wsEvents), len(networkBodies), len(actions))
	})
}

// TestStressCaptureWithClears verifies concurrent reads/writes with buffer clears.
func TestStressCaptureWithClears(t *testing.T) {
	t.Run("concurrent_with_clears", func(t *testing.T) {
		const (
			numWriters       = 5
			numReaders       = 5
			numClearers      = 2
			writesPerWriter  = 30
			readsPerReader   = 20
			clearsPerClearer = 5
		)

		c := NewCapture()
		defer c.Close()

		var wg sync.WaitGroup
		gate := newStressStartGate(numWriters + numReaders + numClearers)

		// Launch writers
		for writerID := 0; writerID < numWriters; writerID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				gate.awaitRelease()
				for i := 0; i < writesPerWriter; i++ {
					// Alternate between different buffer types
					switch i % 3 {
					case 0:
						c.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
							{ID: fmt.Sprintf("ws-%d-%d", id, i), Event: "message"},
						})
					case 1:
						c.Telemetry().AddNetworkBodies([]types.NetworkBody{
							{URL: fmt.Sprintf("https://api.com/%d", i), Method: "GET", Status: 200},
						})
					case 2:
						c.Telemetry().AddEnhancedActions([]types.EnhancedAction{
							{Type: "click", Timestamp: time.Now().UnixNano()},
						})
					}
				}
			}(writerID)
		}

		// Launch readers
		for readerID := 0; readerID < numReaders; readerID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				gate.awaitRelease()
				for i := 0; i < readsPerReader; i++ {
					switch i % 3 {
					case 0:
						_ = c.Telemetry().GetAllWebSocketEvents()
					case 1:
						_ = c.Telemetry().GetNetworkBodies()
					case 2:
						_ = c.Telemetry().GetAllEnhancedActions()
					}
					runtime.Gosched()
				}
			}(readerID)
		}

		// Launch clearers
		for clearID := 0; clearID < numClearers; clearID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				gate.awaitRelease()
				for i := 0; i < clearsPerClearer; i++ {
					runtime.Gosched()
					NewStateResetter(c).ClearAll()
				}
			}(clearID)
		}

		gate.release()
		wg.Wait()

		// Verify buffers are in valid state after concurrent clears
		wsEvents := c.Telemetry().GetAllWebSocketEvents()
		networkBodies := c.Telemetry().GetNetworkBodies()
		actions := c.Telemetry().GetAllEnhancedActions()

		// Capacity bounds must hold even with concurrent clears
		if len(wsEvents) > maxWSEvents {
			t.Errorf("WS events %d exceeds maxWSEvents %d after clears", len(wsEvents), maxWSEvents)
		}
		if len(networkBodies) > maxNetworkBodies {
			t.Errorf("Network bodies %d exceeds maxNetworkBodies %d after clears", len(networkBodies), maxNetworkBodies)
		}
		if len(actions) > maxEnhancedActions {
			t.Errorf("Actions %d exceeds maxEnhancedActions %d after clears", len(actions), maxEnhancedActions)
		}

		// Snapshot must be consistent with buffer contents
		snap := c.Telemetry().GetSnapshot()
		if snap.WebSocketCount < 0 || snap.NetworkCount < 0 || snap.ActionCount < 0 {
			t.Errorf("Snapshot has negative counts: ws=%d net=%d act=%d",
				snap.WebSocketCount, snap.NetworkCount, snap.ActionCount)
		}

		t.Logf("Stress test with clears completed: final %d/%d/%d (ws/net/act)",
			len(wsEvents), len(networkBodies), len(actions))
	})
}

// TestStressCaptureSnapshot verifies concurrent snapshot operations.
func TestStressCaptureSnapshot(t *testing.T) {
	t.Run("snapshot_concurrent_stress", func(t *testing.T) {
		const (
			numWriters      = 5
			numSnappers     = 10
			writesPerWriter = 20
			snapsPerSnapper = 30
		)

		c := NewCapture()
		defer c.Close()

		var wg sync.WaitGroup
		gate := newStressStartGate(numWriters + numSnappers)

		// Launch writers
		for writerID := 0; writerID < numWriters; writerID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				gate.awaitRelease()
				for i := 0; i < writesPerWriter; i++ {
					c.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
						{ID: fmt.Sprintf("ws-%d-%d", id, i), Event: "message"},
					})
					c.Telemetry().AddNetworkBodies([]types.NetworkBody{
						{URL: fmt.Sprintf("https://api.com/%d", i), Method: "GET", Status: 200},
					})
					c.Telemetry().AddEnhancedActions([]types.EnhancedAction{
						{Type: "click", Timestamp: time.Now().UnixNano()},
					})
				}
			}(writerID)
		}

		// Launch snapshot readers
		for snapperID := 0; snapperID < numSnappers; snapperID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				gate.awaitRelease()
				for i := 0; i < snapsPerSnapper; i++ {
					_ = c.Telemetry().GetSnapshot()
					runtime.Gosched()
				}
			}(snapperID)
		}

		gate.release()
		wg.Wait()

		// Final snapshot check
		snapshot := c.Telemetry().GetSnapshot()
		if snapshot.NetworkCount < 0 || snapshot.WebSocketCount < 0 || snapshot.ActionCount < 0 {
			t.Errorf("Snapshot has negative counts: %+v", snapshot)
		}

		t.Logf("Snapshot stress test completed: %+v", snapshot)
	})
}
