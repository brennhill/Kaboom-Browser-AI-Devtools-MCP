// fanout.go — Multi-subscriber fan-out for PTY output.
// Why: Allows multiple WebSocket connections to watch the same terminal session.

package fanout

import (
	"errors"
	"fmt"
	"sync"
)

// maxSubscribers is the default maximum number of concurrent subscribers.
const maxSubscribers = 32

// subscriberBufSize is the channel buffer size per subscriber.
const subscriberBufSize = 64

// ErrFanoutFull is returned when the maximum subscriber count is reached.
var ErrFanoutFull = errors.New("pty: subscriber limit reached")

// ErrFanoutClosed is returned when operating on a closed fanout.
var ErrFanoutClosed = errors.New("pty: fanout closed")

// ErrDuplicateSubscriber is returned when an id is already subscribed. The
// incumbent keeps its channel — a duplicate id is a caller bug, and silently
// replacing the incumbent orphans its goroutine on a channel that will never
// close or receive again.
var ErrDuplicateSubscriber = errors.New("pty: subscriber id already in use")

// Fanout broadcasts data to multiple subscribers. Slow subscribers are dropped
// rather than blocking the producer.
type Fanout struct {
	mu          sync.Mutex
	subscribers map[string]chan []byte
	maxSubs     int
	closed      bool
}

// NewFanout creates a new fan-out broadcaster.
func NewFanout() *Fanout {
	return &Fanout{
		subscribers: make(map[string]chan []byte),
		maxSubs:     maxSubscribers,
	}
}

// Subscribe adds a subscriber and returns a channel for receiving data.
//
// A duplicate id is rejected with ErrDuplicateSubscriber rather than replacing the
// incumbent. Overwriting the map entry left the incumbent's channel open but
// unreachable: its pump goroutine blocked forever on a channel nothing would write
// to or close, and the incumbent's Unsubscribe then closed the NEW subscriber's
// channel instead. Callers used to have to guarantee unique ids themselves; per
// rule 19 the invariant belongs here, where no caller can forget it (finding S7).
func (f *Fanout) Subscribe(id string) (<-chan []byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil, ErrFanoutClosed
	}
	if _, exists := f.subscribers[id]; exists {
		return nil, fmt.Errorf("%w: %s", ErrDuplicateSubscriber, id)
	}
	if len(f.subscribers) >= f.maxSubs {
		return nil, ErrFanoutFull
	}
	ch := make(chan []byte, subscriberBufSize)
	f.subscribers[id] = ch
	return ch, nil
}

// Unsubscribe removes a subscriber and closes its channel.
func (f *Fanout) Unsubscribe(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if ch, ok := f.subscribers[id]; ok {
		close(ch)
		delete(f.subscribers, id)
	}
}

// Broadcast sends data to all subscribers. Slow subscribers whose channel
// buffer is full are dropped to prevent blocking the producer.
func (f *Fanout) Broadcast(data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}
	msg := make([]byte, len(data))
	copy(msg, data)
	var dropped []string
	for id, ch := range f.subscribers {
		select {
		case ch <- msg:
		default:
			close(ch)
			dropped = append(dropped, id)
		}
	}
	for _, id := range dropped {
		delete(f.subscribers, id)
	}
}

// Count returns the number of active subscribers.
func (f *Fanout) Count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.subscribers)
}

// Close closes all subscriber channels and prevents new subscriptions.
func (f *Fanout) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}
	f.closed = true
	for id, ch := range f.subscribers {
		close(ch)
		delete(f.subscribers, id)
	}
}
