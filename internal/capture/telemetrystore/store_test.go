// store_test.go — Verifies canonical cross-stream telemetry ingestion.

package telemetrystore

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestStoreEnrichesAndDetachesIngestedTelemetry(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	store := New(Dependencies{
		ActiveTestIDs: func() []string { return []string{"test-1"} },
		Now:           func() time.Time { return now },
		Dispatch:      func(callback func()) { callback() },
	})

	body := types.NetworkBody{RequestBody: "\x82\xa1a\x01\xa1b\x02"}
	store.AddNetworkBodies([]types.NetworkBody{body})
	event := types.WebSocketEvent{Event: "message", Data: "\x82\xa1a\x01\xa1b\x02"}
	store.AddWebSocketEvents([]types.WebSocketEvent{event})

	bodies := store.NetworkBodies().Snapshot().Bodies
	events := store.WebSockets().Snapshot().Events
	if len(bodies) != 1 || len(events) != 1 {
		t.Fatalf("retained bodies/events = %d/%d, want 1/1", len(bodies), len(events))
	}
	if bodies[0].BinaryFormat == "" || events[0].BinaryFormat == "" {
		t.Fatalf("detected formats = %q/%q, want both classified", bodies[0].BinaryFormat, events[0].BinaryFormat)
	}
	if len(bodies[0].TestIDs) != 1 || bodies[0].TestIDs[0] != "test-1" || len(events[0].TestIDs) != 1 || events[0].TestIDs[0] != "test-1" {
		t.Fatalf("active test IDs were not applied: bodies=%v events=%v", bodies[0].TestIDs, events[0].TestIDs)
	}
}

func TestStoreDispatchesNavigationCallbackOnlyAfterRetainedAction(t *testing.T) {
	t.Parallel()

	dispatched := 0
	store := New(Dependencies{
		ActiveTestIDs: func() []string { return nil },
		Now:           time.Now,
		Dispatch:      func(callback func()) { dispatched++; callback() },
	})
	callbackRuns := 0
	store.SetNavigationCallback(func() { callbackRuns++ })
	store.AddEnhancedActions([]types.EnhancedAction{{Type: "navigation"}})

	if dispatched != 1 || callbackRuns != 1 {
		t.Fatalf("navigation dispatch/callback = %d/%d, want 1/1", dispatched, callbackRuns)
	}
}
