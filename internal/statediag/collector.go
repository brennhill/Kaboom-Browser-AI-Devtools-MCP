// collector.go — Collects redacted persisted-state recovery diagnostics.
// Why: Gives every state loader one explicit path to System Doctor without global state.
package statediag

import (
	"errors"
	"sort"
	"sync"
)

// ErrAbsent identifies normal first-run state absence rather than recovery.
var ErrAbsent = errors.New("persisted state absent")

// Diagnostic describes a safe fallback taken while reading persisted user state.
type Diagnostic struct {
	Name   string
	Detail string
	Fix    string
}

// Reporter accepts recovery diagnostics from state-owning modules.
type Reporter interface {
	Report(Diagnostic)
}

// Collector owns the current recovery diagnostic for each stable state name.
type Collector struct {
	mu    sync.RWMutex
	items map[string]Diagnostic
}

// NewCollector creates an empty recovery diagnostic collector.
func NewCollector() *Collector {
	return &Collector{items: make(map[string]Diagnostic)}
}

// Report records or replaces a diagnostic without retaining raw persisted data.
func (c *Collector) Report(diagnostic Diagnostic) {
	if c == nil || diagnostic.Name == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[diagnostic.Name] = diagnostic
}

// Snapshot returns a stable independent view for System Doctor.
func (c *Collector) Snapshot() []Diagnostic {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	result := make([]Diagnostic, 0, len(c.items))
	for _, diagnostic := range c.items {
		result = append(result, diagnostic)
	}
	c.mu.RUnlock()
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
