// collector.go — Collects redacted persisted-state recovery diagnostics.
// Why: Gives every state loader one explicit path to System Doctor without global state.
package statediag

import (
	"errors"
	"sort"
	"sync"
	"time"
)

// ErrAbsent identifies normal first-run state absence rather than recovery.
var ErrAbsent = errors.New("persisted state absent")

const maxHistoryTransitions = 20

type Lifecycle string

const (
	LifecycleActive    Lifecycle = "active"
	LifecycleRecovered Lifecycle = "recovered"
)

type Transition struct {
	Lifecycle Lifecycle
	At        time.Time
}

// Diagnostic describes a safe fallback taken while reading persisted user state.
type Diagnostic struct {
	Name          string
	CorrelationID string
	Detail        string
	Fix           string
	Lifecycle     Lifecycle
	FirstSeenAt   time.Time
	LastSeenAt    time.Time
	RecoveredAt   time.Time
	Occurrences   int
	History       []Transition
}

// Reporter accepts recovery diagnostics from state-owning modules.
type Reporter interface {
	Report(Diagnostic)
}

type Resolver interface {
	Resolve(name string)
}

// Collector owns the current recovery diagnostic for each stable state name.
type Collector struct {
	mu    sync.RWMutex
	items map[string]Diagnostic
	now   func() time.Time
}

// NewCollector creates an empty recovery diagnostic collector.
func NewCollector() *Collector {
	return &Collector{items: make(map[string]Diagnostic), now: time.Now}
}

// Report records or replaces a diagnostic without retaining raw persisted data.
func (c *Collector) Report(diagnostic Diagnostic) {
	if c == nil || diagnostic.Name == "" {
		return
	}
	now := c.now().UTC()
	c.mu.Lock()
	defer c.mu.Unlock()
	current, exists := c.items[diagnostic.Name]
	if !exists {
		diagnostic.FirstSeenAt = now
		diagnostic.History = appendTransition(nil, Transition{Lifecycle: LifecycleActive, At: now})
	} else {
		diagnostic.FirstSeenAt = current.FirstSeenAt
		diagnostic.History = current.History
		if current.Lifecycle == LifecycleRecovered {
			diagnostic.History = appendTransition(diagnostic.History, Transition{Lifecycle: LifecycleActive, At: now})
		}
		diagnostic.Occurrences = current.Occurrences
	}
	diagnostic.Lifecycle = LifecycleActive
	diagnostic.LastSeenAt = now
	diagnostic.RecoveredAt = time.Time{}
	diagnostic.Occurrences++
	c.items[diagnostic.Name] = diagnostic
}

func (c *Collector) Resolve(name string) {
	if c == nil || name == "" {
		return
	}
	now := c.now().UTC()
	c.mu.Lock()
	defer c.mu.Unlock()
	diagnostic, exists := c.items[name]
	if !exists || diagnostic.Lifecycle == LifecycleRecovered {
		return
	}
	diagnostic.Lifecycle = LifecycleRecovered
	diagnostic.RecoveredAt = now
	diagnostic.LastSeenAt = now
	diagnostic.History = appendTransition(diagnostic.History, Transition{Lifecycle: LifecycleRecovered, At: now})
	c.items[name] = diagnostic
}

func Resolve(reporter Reporter, name string) {
	if resolver, ok := reporter.(Resolver); ok {
		resolver.Resolve(name)
	}
}

// Snapshot returns a stable independent view for System Doctor.
func (c *Collector) Snapshot() []Diagnostic {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	result := make([]Diagnostic, 0, len(c.items))
	for _, diagnostic := range c.items {
		diagnostic.History = append([]Transition(nil), diagnostic.History...)
		result = append(result, diagnostic)
	}
	c.mu.RUnlock()
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func appendTransition(history []Transition, transition Transition) []Transition {
	if len(history) >= maxHistoryTransitions {
		copy(history, history[len(history)-maxHistoryTransitions+1:])
		history = history[:maxHistoryTransitions-1]
	}
	return append(history, transition)
}
