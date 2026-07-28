// observer.go -- Typed lifecycle event bus with panic isolation and unsubscribe support.
// Why: Replaces closure-chaining pattern with a proper observer.
// Improvements: typed events, slice-based listeners, panic recovery in Emit, Unsubscribe support.

package lifecycle

import (
	"fmt"
	"os"
	"sync"
)

// Event is a typed enum for capture lifecycle events.
type Event int

const (
	EventUnknown               Event = iota
	EventCircuitOpened               // Circuit breaker opened (rate exceeded)
	EventCircuitClosed               // Circuit breaker closed (recovered)
	EventExtensionConnected          // Extension connected or reconnected
	EventExtensionDisconnected       // Extension disconnected (poll timeout)
	EventBufferEviction              // Ring buffer evicted old entries
	EventRateLimitTriggered          // Rate limit threshold hit
	EventCommandStateDesync          // Command state mismatch with extension
	EventSyncSnapshot                // Periodic sync state snapshot
)

// eventNames maps typed events to their wire-format string names.
var eventNames = map[Event]string{
	EventUnknown:               "unknown",
	EventCircuitOpened:         "circuit_opened",
	EventCircuitClosed:         "circuit_closed",
	EventExtensionConnected:    "extension_connected",
	EventExtensionDisconnected: "extension_disconnected",
	EventBufferEviction:        "buffer_eviction",
	EventRateLimitTriggered:    "rate_limit_triggered",
	EventCommandStateDesync:    "command_state_desync",
	EventSyncSnapshot:          "sync_snapshot",
}

// String returns the wire-format name for a lifecycle event.
func (e Event) String() string {
	if name, ok := eventNames[e]; ok {
		return name
	}
	return "unknown"
}

// Listener is the callback signature for lifecycle event subscribers.
type Listener func(event Event, data map[string]any)

// listenerEntry pairs a listener with a stable subscription ID.
type listenerEntry struct {
	id int
	fn Listener
}

// Observer is a concurrency-safe event bus for capture lifecycle events.
// Supports multiple listeners, unsubscribe by ID, and panic isolation per listener.
type Observer struct {
	mu        sync.RWMutex
	listeners []listenerEntry
	nextID    int
}

// NewObserver creates an empty observer ready for subscriptions.
func NewObserver() *Observer {
	return &Observer{}
}

// Subscribe registers a listener and returns a subscription ID for later removal.
func (o *Observer) Subscribe(fn Listener) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	id := o.nextID
	o.nextID++
	o.listeners = append(o.listeners, listenerEntry{id: id, fn: fn})
	return id
}

// Unsubscribe removes a listener by its subscription ID. No-op if ID not found.
func (o *Observer) Unsubscribe(id int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for i, entry := range o.listeners {
		if entry.id == id {
			o.listeners = append(o.listeners[:i], o.listeners[i+1:]...)
			return
		}
	}
}

// Emit dispatches an event to all listeners. Each listener is called with panic
// recovery so one misbehaving listener cannot break others. Listeners are called
// synchronously in subscription order; callers should wrap in util.SafeGo if needed.
func (o *Observer) Emit(event Event, data map[string]any) {
	o.mu.RLock()
	snapshot := make([]listenerEntry, len(o.listeners))
	copy(snapshot, o.listeners)
	o.mu.RUnlock()

	for _, entry := range snapshot {
		func(fn Listener) {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "[Kaboom] lifecycle observer: listener panic on %s: %v\n", event.String(), r)
				}
			}()
			fn(event, data)
		}(entry.fn)
	}
}
