// collector.go — Collects redacted persisted-state recovery diagnostics.
// Why: Gives every state loader one explicit path to System Doctor without global state.
package statediag

import (
	"errors"
	"regexp"
	"sort"
	"sync"
	"time"
)

// ErrAbsent identifies normal first-run state absence rather than recovery.
var ErrAbsent = errors.New("persisted state absent")

const (
	maxHistoryTransitions   = 20
	maxRecoveredDiagnostics = 100
)

type Lifecycle string

const (
	LifecycleActive    Lifecycle = "active"
	LifecycleRecovered Lifecycle = "recovered"
)

type Transition struct {
	Lifecycle     Lifecycle `json:"lifecycle"`
	At            time.Time `json:"at"`
	Event         string    `json:"event,omitempty"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	Outcome       string    `json:"outcome,omitempty"`
}

// Diagnostic describes a safe fallback taken while reading persisted user state.
type Diagnostic struct {
	Name                     string       `json:"name"`
	CorrelationID            string       `json:"correlation_id,omitempty"`
	Detail                   string       `json:"detail"`
	Fix                      string       `json:"fix,omitempty"`
	Lifecycle                Lifecycle    `json:"lifecycle"`
	FirstSeenAt              time.Time    `json:"first_seen_at"`
	LastSeenAt               time.Time    `json:"last_seen_at"`
	RecoveredAt              time.Time    `json:"recovered_at,omitempty"`
	Occurrences              int          `json:"occurrences"`
	History                  []Transition `json:"history"`
	LastSuccessfulTransition string       `json:"last_successful_transition,omitempty"`
	ExpectedNextTransition   string       `json:"expected_next_transition,omitempty"`
	Deadline                 time.Time    `json:"deadline,omitempty"`
	RecoveryAttempt          int          `json:"recovery_attempt,omitempty"`
	RecoveryOutcome          string       `json:"recovery_outcome,omitempty"`
}

var (
	diagnosticBearer = regexp.MustCompile(`(?i)bearer\s+[^\s,;]+`)
	diagnosticSecret = regexp.MustCompile(`(?i)(token|secret|password|api[_-]?key)=([^\s&#,;]+)`)
)

// Reporter accepts recovery diagnostics from state-owning modules.
type Reporter interface {
	Report(Diagnostic)
}

type Resolver interface {
	Resolve(name string)
}

// CollectorStats describes bounded incident retention without exposing diagnostic contents.
type CollectorStats struct {
	Active           int
	Recovered        int
	RecoveredLimit   int
	DroppedRecovered int
}

// Collector owns the current recovery diagnostic for each stable state name.
type Collector struct {
	mu               sync.RWMutex
	items            map[string]Diagnostic
	now              func() time.Time
	recoveredLimit   int
	droppedRecovered int
}

// NewCollector creates an empty recovery diagnostic collector.
func NewCollector() *Collector {
	return &Collector{items: make(map[string]Diagnostic), now: time.Now, recoveredLimit: maxRecoveredDiagnostics}
}

// Report records or replaces a diagnostic without retaining raw persisted data.
func (c *Collector) Report(diagnostic Diagnostic) {
	if c == nil || diagnostic.Name == "" {
		return
	}
	now := c.now().UTC()
	diagnostic.Name = compactSafe(diagnostic.Name)
	diagnostic.CorrelationID = compactSafe(diagnostic.CorrelationID)
	diagnostic.Detail = compactSafe(diagnostic.Detail)
	diagnostic.Fix = compactSafe(diagnostic.Fix)
	diagnostic.ExpectedNextTransition = compactSafe(diagnostic.ExpectedNextTransition)
	c.mu.Lock()
	defer c.mu.Unlock()
	current, exists := c.items[diagnostic.Name]
	event := "failure_detected"
	if !exists {
		diagnostic.FirstSeenAt = now
	} else {
		event = "failure_recurred"
		diagnostic.FirstSeenAt = current.FirstSeenAt
		diagnostic.History = current.History
		if diagnostic.CorrelationID == "" {
			diagnostic.CorrelationID = current.CorrelationID
		}
		if diagnostic.ExpectedNextTransition == "" {
			diagnostic.ExpectedNextTransition = current.ExpectedNextTransition
		}
		if diagnostic.Deadline.IsZero() {
			diagnostic.Deadline = current.Deadline
		}
		diagnostic.Occurrences = current.Occurrences
		diagnostic.LastSuccessfulTransition = current.LastSuccessfulTransition
	}
	if diagnostic.ExpectedNextTransition == "" {
		diagnostic.ExpectedNextTransition = "state_verified"
	}
	diagnostic.Lifecycle = LifecycleActive
	diagnostic.LastSeenAt = now
	diagnostic.RecoveredAt = time.Time{}
	diagnostic.Occurrences++
	diagnostic.RecoveryAttempt = diagnostic.Occurrences
	diagnostic.RecoveryOutcome = "pending"
	diagnostic.History = appendTransition(diagnostic.History, Transition{
		Lifecycle: LifecycleActive, At: now, Event: event,
		CorrelationID: diagnostic.CorrelationID, Outcome: "active",
	})
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
	diagnostic.LastSuccessfulTransition = "state_verified"
	diagnostic.ExpectedNextTransition = ""
	diagnostic.Deadline = time.Time{}
	diagnostic.RecoveryOutcome = "recovered"
	diagnostic.History[len(diagnostic.History)-1].Event = "recovery_completed"
	diagnostic.History[len(diagnostic.History)-1].CorrelationID = diagnostic.CorrelationID
	diagnostic.History[len(diagnostic.History)-1].Outcome = "recovered"
	c.items[name] = diagnostic
	c.evictOldestRecoveredLocked()
}

func compactSafe(value string) string {
	value = diagnosticBearer.ReplaceAllString(value, "Bearer [REDACTED]")
	value = diagnosticSecret.ReplaceAllString(value, "$1=[REDACTED]")
	if len(value) > 500 {
		value = value[:500]
	}
	return value
}

// Stats returns content-free retention counters for System Doctor.
func (c *Collector) Stats() CollectorStats {
	if c == nil {
		return CollectorStats{}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	stats := CollectorStats{RecoveredLimit: c.recoveredLimit, DroppedRecovered: c.droppedRecovered}
	for _, diagnostic := range c.items {
		if diagnostic.Lifecycle == LifecycleRecovered {
			stats.Recovered++
		} else {
			stats.Active++
		}
	}
	return stats
}

// evictOldestRecoveredLocked performs one deterministic scan. Resolve adds at
// most one recovered incident, so no repeated remove-and-recheck loop is needed.
func (c *Collector) evictOldestRecoveredLocked() {
	recovered := 0
	oldestName := ""
	var oldestAt time.Time
	for name, diagnostic := range c.items {
		if diagnostic.Lifecycle != LifecycleRecovered {
			continue
		}
		recovered++
		if oldestName == "" || diagnostic.RecoveredAt.Before(oldestAt) ||
			(diagnostic.RecoveredAt.Equal(oldestAt) && name < oldestName) {
			oldestName = name
			oldestAt = diagnostic.RecoveredAt
		}
	}
	if recovered <= c.recoveredLimit {
		return
	}
	delete(c.items, oldestName)
	c.droppedRecovered++
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
