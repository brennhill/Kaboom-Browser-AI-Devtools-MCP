// observer_test.go -- Tests for Observer event bus.
package lifecycle

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestObserver_SubscribeAndEmit(t *testing.T) {
	t.Parallel()
	obs := NewObserver()

	var received Event
	var receivedData map[string]any
	obs.Subscribe(func(event Event, data map[string]any) {
		received = event
		receivedData = data
	})

	obs.Emit(EventExtensionConnected, map[string]any{"key": "value"})

	if received != EventExtensionConnected {
		t.Errorf("received event = %d, want %d", received, EventExtensionConnected)
	}
	if receivedData["key"] != "value" {
		t.Errorf("received data = %v, want key=value", receivedData)
	}
}

func TestObserver_MultipleListeners(t *testing.T) {
	t.Parallel()
	obs := NewObserver()

	var count atomic.Int32
	obs.Subscribe(func(Event, map[string]any) { count.Add(1) })
	obs.Subscribe(func(Event, map[string]any) { count.Add(1) })
	obs.Subscribe(func(Event, map[string]any) { count.Add(1) })

	obs.Emit(EventCircuitOpened, nil)

	if got := count.Load(); got != 3 {
		t.Errorf("listener count = %d, want 3", got)
	}
}

func TestObserver_Unsubscribe(t *testing.T) {
	t.Parallel()
	obs := NewObserver()

	var count atomic.Int32
	id := obs.Subscribe(func(Event, map[string]any) { count.Add(1) })
	obs.Unsubscribe(id)

	obs.Emit(EventCircuitClosed, nil)

	if got := count.Load(); got != 0 {
		t.Errorf("listener count = %d, want 0 after unsubscribe", got)
	}
}

func TestObserver_UnsubscribeNonexistent(t *testing.T) {
	t.Parallel()
	obs := NewObserver()

	// Should not panic
	obs.Unsubscribe(999)
}

func TestObserver_PanicIsolation(t *testing.T) {
	t.Parallel()
	obs := NewObserver()

	var secondCalled atomic.Bool
	obs.Subscribe(func(Event, map[string]any) {
		panic("listener panic")
	})
	obs.Subscribe(func(Event, map[string]any) {
		secondCalled.Store(true)
	})

	// Should not panic; second listener should still run
	obs.Emit(EventExtensionDisconnected, nil)

	if !secondCalled.Load() {
		t.Error("second listener should run despite first listener panic")
	}
}

func TestObserver_EmitNoListeners(t *testing.T) {
	t.Parallel()
	obs := NewObserver()

	// Should not panic
	obs.Emit(EventCircuitOpened, nil)
}

func TestObserver_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	obs := NewObserver()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := obs.Subscribe(func(Event, map[string]any) {})
			obs.Emit(EventSyncSnapshot, nil)
			obs.Unsubscribe(id)
		}()
	}
	wg.Wait()
}

func TestEventString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		event Event
		want  string
	}{
		{EventCircuitOpened, "circuit_opened"},
		{EventCircuitClosed, "circuit_closed"},
		{EventExtensionConnected, "extension_connected"},
		{EventExtensionDisconnected, "extension_disconnected"},
		{EventBufferEviction, "buffer_eviction"},
		{EventRateLimitTriggered, "rate_limit_triggered"},
		{EventCommandStateDesync, "command_state_desync"},
		{EventSyncSnapshot, "sync_snapshot"},
		{EventUnknown, "unknown"},
		{Event(999), "unknown"},
	}

	for _, tc := range cases {
		if got := tc.event.String(); got != tc.want {
			t.Errorf("Event(%d).String() = %q, want %q", tc.event, got, tc.want)
		}
	}
}
