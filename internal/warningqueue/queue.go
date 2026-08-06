// queue.go — Owns ordered, deduplicated one-shot warnings.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package warningqueue

import "sync"

// Queue stores each warning once and delivers pending warnings atomically.
type Queue struct {
	mu      sync.Mutex
	pending []string
	seen    map[string]struct{}
}

// New returns an empty warning queue.
func New() *Queue { return &Queue{seen: make(map[string]struct{})} }

// Add stores a non-empty warning once for the process lifetime.
func (q *Queue) Add(message string) {
	if message == "" {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, exists := q.seen[message]; exists {
		return
	}
	q.seen[message] = struct{}{}
	q.pending = append(q.pending, message)
}

// Drain atomically returns and clears pending warnings.
func (q *Queue) Drain() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) == 0 {
		return nil
	}
	warnings := append([]string(nil), q.pending...)
	q.pending = nil
	return warnings
}
