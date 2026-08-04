// store.go — Bounded, generation-aware operational incident lifecycle storage.
package incident

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrUnknownCode = errors.New("unknown operational incident code")

const maxHistory = 20

type State string

const (
	StateDetected  State = "detected"
	StateRetrying  State = "retrying"
	StateRecovered State = "recovered"
	StateExhausted State = "exhausted"
)

type Outcome string

const (
	OutcomePending   Outcome = "pending"
	OutcomeRecovered Outcome = "recovered"
	OutcomeExhausted Outcome = "exhausted"
)

type AttemptBucket string

const (
	AttemptZero     AttemptBucket = "0"
	AttemptOne      AttemptBucket = "1"
	AttemptTwoThree AttemptBucket = "2_3"
	AttemptFourPlus AttemptBucket = "4_plus"
)

type LatencyBucket string

const (
	LatencyUnderSecond  LatencyBucket = "under_1s"
	LatencyOneToFive    LatencyBucket = "1s_5s"
	LatencyFiveToThirty LatencyBucket = "5s_30s"
	LatencyOverThirty   LatencyBucket = "over_30s"
)

// LocalEvidence never crosses the analytics projection boundary.
type LocalEvidence struct {
	Detail string `json:"detail,omitempty"`
	Fix    string `json:"fix,omitempty"`
}

type Report struct {
	Code          Code
	CorrelationID string
	Generation    uint64
	Evidence      LocalEvidence
}

type Transition struct {
	State    State     `json:"state"`
	At       time.Time `json:"at"`
	Attempts uint      `json:"attempts,omitempty"`
}

type Incident struct {
	Code          Code          `json:"code"`
	CorrelationID string        `json:"correlation_id,omitempty"`
	Generation    uint64        `json:"generation"`
	State         State         `json:"state"`
	Attempts      uint          `json:"attempts"`
	DetectedAt    time.Time     `json:"detected_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	ResolvedAt    time.Time     `json:"resolved_at,omitempty"`
	LocalEvidence LocalEvidence `json:"local_evidence,omitempty"`
	History       []Transition  `json:"history"`
}

// ReliabilityEvent is deliberately incapable of carrying local evidence,
// correlation identifiers, generations, paths, URLs, or arbitrary dimensions.
type ReliabilityEvent struct {
	Code          Code          `json:"code"`
	Subsystem     Subsystem     `json:"subsystem"`
	Stage         Stage         `json:"stage"`
	Severity      Severity      `json:"severity"`
	Retryable     bool          `json:"retryable"`
	Outcome       Outcome       `json:"outcome"`
	AttemptBucket AttemptBucket `json:"attempt_bucket"`
	LatencyBucket LatencyBucket `json:"latency_bucket"`
}

type Stats struct {
	Active   int
	Terminal int
	Capacity int
	Dropped  uint64
}

type Store struct {
	mu       sync.RWMutex
	items    map[string]Incident
	capacity int
	dropped  uint64
	now      func() time.Time
}

func NewStore(capacity int) *Store {
	if capacity < 1 {
		capacity = 1
	}
	return &Store{items: make(map[string]Incident), capacity: capacity, now: time.Now}
}

func (s *Store) Detect(report Report) (string, error) {
	if _, ok := Lookup(report.Code); !ok {
		return "", ErrUnknownCode
	}
	key := incidentKey(report.Code, report.CorrelationID)
	now := s.now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, ok := s.items[key]; ok && report.Generation <= current.Generation {
		return key, nil
	}
	incident := Incident{
		Code: report.Code, CorrelationID: compactLocal(report.CorrelationID), Generation: report.Generation,
		State: StateDetected, DetectedAt: now, UpdatedAt: now,
		LocalEvidence: LocalEvidence{Detail: compactLocal(report.Evidence.Detail), Fix: compactLocal(report.Evidence.Fix)},
		History:       []Transition{{State: StateDetected, At: now}},
	}
	s.items[key] = incident
	s.evictOneLocked(key)
	return key, nil
}

func (s *Store) Retry(key string, generation uint64, attempt uint) bool {
	return s.transition(key, generation, StateRetrying, attempt)
}

func (s *Store) Recover(key string, generation uint64) bool {
	return s.transition(key, generation, StateRecovered, 0)
}

func (s *Store) Exhaust(key string, generation uint64) bool {
	return s.transition(key, generation, StateExhausted, 0)
}

func (s *Store) transition(key string, generation uint64, next State, attempt uint) bool {
	now := s.now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.items[key]
	if !ok || generation != current.Generation || terminal(current.State) {
		return false
	}
	if next == StateRetrying {
		if attempt == 0 || attempt <= current.Attempts {
			return false
		}
		current.Attempts = attempt
	} else if current.State == next {
		return false
	}
	current.State = next
	current.UpdatedAt = now
	if terminal(next) {
		current.ResolvedAt = now
	}
	current.History = appendBounded(current.History, Transition{State: next, At: now, Attempts: current.Attempts})
	s.items[key] = current
	return true
}

func (s *Store) Analytics(key string) (ReliabilityEvent, bool) {
	s.mu.RLock()
	current, ok := s.items[key]
	s.mu.RUnlock()
	if !ok {
		return ReliabilityEvent{}, false
	}
	definition, ok := Lookup(current.Code)
	if !ok {
		return ReliabilityEvent{}, false
	}
	return ReliabilityEvent{
		Code: current.Code, Subsystem: definition.Subsystem, Stage: definition.Stage,
		Severity: definition.Severity, Retryable: definition.Retryable,
		Outcome: outcome(current.State), AttemptBucket: bucketAttempts(current.Attempts),
		LatencyBucket: bucketLatency(current.UpdatedAt.Sub(current.DetectedAt)),
	}, true
}

func (s *Store) Snapshot() []Incident {
	s.mu.RLock()
	result := make([]Incident, 0, len(s.items))
	for _, current := range s.items {
		current.History = append([]Transition(nil), current.History...)
		result = append(result, current)
	}
	s.mu.RUnlock()
	sort.Slice(result, func(i, j int) bool {
		if result[i].DetectedAt.Equal(result[j].DetectedAt) {
			return incidentKey(result[i].Code, result[i].CorrelationID) < incidentKey(result[j].Code, result[j].CorrelationID)
		}
		return result[i].DetectedAt.Before(result[j].DetectedAt)
	})
	return result
}

func (s *Store) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats := Stats{Capacity: s.capacity, Dropped: s.dropped}
	for _, current := range s.items {
		if terminal(current.State) {
			stats.Terminal++
		} else {
			stats.Active++
		}
	}
	return stats
}

func (s *Store) evictOneLocked(keep string) {
	if len(s.items) <= s.capacity {
		return
	}
	candidate := ""
	var candidateIncident Incident
	for key, current := range s.items {
		if key == keep {
			continue
		}
		if candidate == "" || (terminal(current.State) && !terminal(candidateIncident.State)) ||
			(terminal(current.State) == terminal(candidateIncident.State) && current.UpdatedAt.Before(candidateIncident.UpdatedAt)) {
			candidate, candidateIncident = key, current
		}
	}
	if candidate != "" {
		delete(s.items, candidate)
		s.dropped++
	}
}

func incidentKey(code Code, correlationID string) string {
	return string(code) + "\x00" + compactLocal(correlationID)
}
func terminal(state State) bool { return state == StateRecovered || state == StateExhausted }

func outcome(state State) Outcome {
	if state == StateRecovered {
		return OutcomeRecovered
	}
	if state == StateExhausted {
		return OutcomeExhausted
	}
	return OutcomePending
}

func bucketAttempts(attempts uint) AttemptBucket {
	if attempts == 0 {
		return AttemptZero
	}
	if attempts == 1 {
		return AttemptOne
	}
	if attempts <= 3 {
		return AttemptTwoThree
	}
	return AttemptFourPlus
}

func bucketLatency(duration time.Duration) LatencyBucket {
	if duration < time.Second {
		return LatencyUnderSecond
	}
	if duration < 5*time.Second {
		return LatencyOneToFive
	}
	if duration < 30*time.Second {
		return LatencyFiveToThirty
	}
	return LatencyOverThirty
}

func appendBounded(history []Transition, transition Transition) []Transition {
	if len(history) >= maxHistory {
		copy(history, history[len(history)-maxHistory+1:])
		history = history[:maxHistory-1]
	}
	return append(history, transition)
}

var localSecret = regexp.MustCompile(`(?i)(token|secret|password|api[_-]?key)=([^\s&#,;]+)`)

func compactLocal(value string) string {
	value = localSecret.ReplaceAllString(strings.TrimSpace(value), "$1=[REDACTED]")
	if len(value) > 500 {
		return value[:500]
	}
	return value
}
