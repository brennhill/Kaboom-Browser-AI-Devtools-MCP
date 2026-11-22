// store.go — Owns generic bounded FIFO retention without synchronization.
// Docs: docs/features/feature/backend-log-streaming/index.md

// Package ringstore provides the canonical fixed-capacity FIFO used by capture
// owners. Callers own synchronization so one lock can protect related counters.
package ringstore

// Store retains at most Capacity values in FIFO order.
type Store[T any] struct {
	storage []T
	head    int
	size    int
}

// New creates an empty bounded store. Negative capacities are normalized to zero.
func New[T any](capacity int) Store[T] {
	if capacity < 0 {
		capacity = 0
	}
	return Store[T]{storage: make([]T, capacity)}
}

// Capacity returns the fixed maximum entry count.
func (s *Store[T]) Capacity() int { return len(s.storage) }

// Len returns the retained entry count.
func (s *Store[T]) Len() int { return s.size }

// Push appends value and reports the oldest overwritten value when full.
func (s *Store[T]) Push(value T) (evicted T, overwritten bool) {
	if len(s.storage) == 0 {
		return value, true
	}
	if s.size < len(s.storage) {
		s.storage[(s.head+s.size)%len(s.storage)] = value
		s.size++
		return evicted, false
	}
	evicted = s.storage[s.head]
	s.storage[s.head] = value
	s.head = (s.head + 1) % len(s.storage)
	return evicted, true
}

// At returns the retained value at a zero-based FIFO index.
func (s *Store[T]) At(index int) *T {
	return &s.storage[(s.head+index)%len(s.storage)]
}

// DropOldest discards up to count values in one bounded pass.
func (s *Store[T]) DropOldest(count int) {
	if count <= 0 {
		return
	}
	if count > s.size {
		count = s.size
	}
	var zero T
	for i := 0; i < count; i++ {
		s.storage[(s.head+i)%len(s.storage)] = zero
	}
	if len(s.storage) > 0 {
		s.head = (s.head + count) % len(s.storage)
	}
	s.size -= count
	if s.size == 0 {
		s.head = 0
	}
}

// Clear discards every retained value.
func (s *Store[T]) Clear() { s.DropOldest(s.size) }

// Snapshot returns a detached FIFO-ordered copy.
func (s *Store[T]) Snapshot() []T {
	out := make([]T, s.size)
	for i := range out {
		out[i] = *s.At(i)
	}
	return out
}
