// Purpose: Defines annotation store state, lifecycle, cleanup, reset, and wait coordination.
// Why: Keeps the shared in-memory owner and its bounded lifecycle behavior atomic.
// Docs: docs/features/feature/annotated-screenshots/index.md

// Package annotation stores draw-mode sessions and their detailed DOM/style context.
package annotation

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// Rect represents a viewport-relative rectangle.
type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Annotation is a lightweight annotation returned by default.
type Annotation struct {
	ID             string `json:"id"`
	Rect           Rect   `json:"rect"`
	Text           string `json:"text"`
	Timestamp      int64  `json:"timestamp"`
	PageURL        string `json:"page_url"`
	ElementSummary string `json:"element_summary"`
	CorrelationID  string `json:"correlation_id"`
}

// Detail contains full DOM/style detail for lazy retrieval.
type Detail struct {
	CorrelationID      string            `json:"correlation_id"`
	Selector           string            `json:"selector"`
	SelectorCandidates []string          `json:"selector_candidates,omitempty"`
	Tag                string            `json:"tag"`
	TextContent        string            `json:"text_content"`
	OuterHTML          string            `json:"outer_html,omitempty"`
	Classes            []string          `json:"classes"`
	ID                 string            `json:"id"`
	ComputedStyles     map[string]string `json:"computed_styles"`
	ParentSelector     string            `json:"parent_selector"`
	BoundingRect       Rect              `json:"bounding_rect"`
	A11yFlags          []string          `json:"a11y_flags,omitempty"`
	ShadowDOM          json.RawMessage   `json:"shadow_dom,omitempty"`
	AllElements        json.RawMessage   `json:"all_elements,omitempty"`
	ElementCount       int               `json:"element_count,omitempty"`
	IframeContent      json.RawMessage   `json:"iframe_content,omitempty"`
	ParentContext      json.RawMessage   `json:"parent_context,omitempty"`
	Siblings           json.RawMessage   `json:"siblings,omitempty"`
	CSSFramework       string            `json:"css_framework,omitempty"`
	JSFramework        string            `json:"js_framework,omitempty"`
	Component          json.RawMessage   `json:"component,omitempty"`
}

// Session represents a completed draw mode session.
type Session struct {
	Annotations    []Annotation `json:"annotations"`
	ScreenshotPath string       `json:"screenshot"`
	PageURL        string       `json:"page_url"`
	TabID          int          `json:"tab_id"`
	Timestamp      int64        `json:"timestamp"`
}

// detailEntry wraps detail with expiration time.
type detailEntry struct {
	Detail    Detail
	ExpiresAt time.Time
}

// sessionEntry wraps session with expiration time.
type sessionEntry struct {
	Session   *Session
	ExpiresAt time.Time
}

const MaxSessions = 100
const MaxNamedSessions = 50
const MaxDetails = 500

// NamedSession accumulates annotations across multiple pages.
type NamedSession struct {
	Name      string     `json:"name"`
	Pages     []*Session `json:"pages"`
	UpdatedAt int64      `json:"updated_at"`
}

// namedSessionEntry wraps a named session with TTL.
type namedSessionEntry struct {
	Session   *NamedSession
	ExpiresAt time.Time
}

// waiter is a pending correlation_id waiting for annotations to arrive.
// When annotations are stored, all matching waiters are completed via the callback.
type waiter struct {
	CorrelationID    string // command tracker correlation_id
	AnnotSessionName string // "" for anonymous, non-empty for named session
	URLFilter        string // optional URL scope filter applied at completion time
}

// Store manages annotation sessions and details in memory.
type Store struct {
	mu       sync.RWMutex
	sessions map[int]*sessionEntry         // tabID → session with TTL
	details  map[string]*detailEntry       // correlationID → detail with TTL
	named    map[string]*namedSessionEntry // session name → multi-page session

	detailTTL  time.Duration
	sessionTTL time.Duration
	now        func() time.Time
	done       chan struct{} // signals cleanup goroutine to stop
	closeOnce  sync.Once     // ensures Close() is safe to call concurrently

	// Blocking wait support
	sessionNotify     chan struct{} // closed on StoreSession, then recreated
	lastDrawStartedAt int64         // millis; set by MarkDrawStarted

	// Async wait support — LLM polls via observe({what: "command_result"})
	waiters         []waiter
	completeCommand func(correlationID string, result json.RawMessage) // callback to complete CommandTracker
}

// NewStore creates a new store with the given detail TTL.
func NewStore(detailTTL time.Duration) *Store {
	s := &Store{
		sessions:      make(map[int]*sessionEntry),
		details:       make(map[string]*detailEntry),
		named:         make(map[string]*namedSessionEntry),
		detailTTL:     detailTTL,
		sessionTTL:    2 * time.Hour,
		now:           time.Now,
		done:          make(chan struct{}),
		sessionNotify: make(chan struct{}),
	}
	// Start background cleanup goroutine.
	util.SafeGo(func() { s.cleanupLoop() })
	return s
}

// Close stops the background cleanup goroutine. Safe to call concurrently.
func (s *Store) Close() {
	s.closeOnce.Do(func() {
		close(s.done)
	})
}

// SetCommandCompleter sets the callback used to complete async annotation waiters.
// Must be called before any waiters are registered (typically at server startup).
func (s *Store) SetCommandCompleter(fn func(correlationID string, result json.RawMessage)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completeCommand = fn
}

// RegisterWaiter registers a correlation_id to be completed when annotations arrive.
// sessionName is "" for anonymous sessions, or a name for named sessions.
// urlFilter is optional and scopes async completion payloads to a specific project/page URL.
func (s *Store) RegisterWaiter(correlationID string, sessionName string, urlFilter string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.waiters = append(s.waiters, waiter{
		CorrelationID:    correlationID,
		AnnotSessionName: sessionName,
		URLFilter:        urlFilter,
	})
}

// cleanupLoop periodically removes expired sessions and detail entries.
func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.evictExpiredEntries()
		}
	}
}

// evictExpiredEntries removes all expired details, sessions, and named sessions.
func (s *Store) evictExpiredEntries() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for id, entry := range s.details {
		if now.After(entry.ExpiresAt) {
			delete(s.details, id)
		}
	}
	for tabID, entry := range s.sessions {
		if now.After(entry.ExpiresAt) {
			delete(s.sessions, tabID)
		}
	}
	for name, entry := range s.named {
		if now.After(entry.ExpiresAt) {
			delete(s.named, name)
		}
	}
}

// ClearCounts reports how many annotation-store entries were removed by ClearAll.
type ClearCounts struct {
	Sessions      int
	Details       int
	NamedSessions int
	Waiters       int
}

// ClearAll removes all anonymous sessions, named sessions, detail entries, and waiters.
// It also resets draw-start time so future waits evaluate against fresh state.
func (s *Store) ClearAll() ClearCounts {
	type clearPlan struct {
		counts ClearCounts
		ch     chan struct{}
	}
	plan := func() clearPlan {
		s.mu.Lock()
		defer s.mu.Unlock()

		counts := ClearCounts{
			Sessions:      len(s.sessions),
			Details:       len(s.details),
			NamedSessions: len(s.named),
			Waiters:       len(s.waiters),
		}

		s.sessions = make(map[int]*sessionEntry)
		s.details = make(map[string]*detailEntry)
		s.named = make(map[string]*namedSessionEntry)
		s.waiters = nil
		s.lastDrawStartedAt = 0

		ch := s.sessionNotify
		s.sessionNotify = make(chan struct{})

		return clearPlan{counts: counts, ch: ch}
	}()

	close(plan.ch)
	return plan.counts
}

// TakeWaiter removes and returns a pending async annotation waiter by correlation_id.
// Returns (sessionName, urlFilter, true) when found, or ("", "", false) when missing.
func (s *Store) TakeWaiter(correlationID string) (string, string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, w := range s.waiters {
		if w.CorrelationID != correlationID {
			continue
		}
		s.waiters = append(s.waiters[:i], s.waiters[i+1:]...)
		return w.AnnotSessionName, w.URLFilter, true
	}
	return "", "", false
}

// waitForCondition blocks until checker returns non-nil, timeout, or store close.
// Returns (result, timedOut). result is nil if timed out or store closed.
func (s *Store) waitForCondition(timeout time.Duration, checker func() any) (any, bool) {
	if result := checker(); result != nil {
		return result, false
	}

	deadline := time.Now().Add(timeout)
	for {
		s.mu.RLock()
		ch := s.sessionNotify
		s.mu.RUnlock()

		remaining := time.Until(deadline)
		if remaining <= 0 {
			if result := checker(); result != nil {
				return result, false
			}
			return nil, true
		}

		timer := time.NewTimer(remaining)
		select {
		case <-ch:
			timer.Stop()
			if result := checker(); result != nil {
				return result, false
			}
			continue
		case <-timer.C:
			if result := checker(); result != nil {
				return result, false
			}
			return nil, true
		case <-s.done:
			timer.Stop()
			return nil, false
		}
	}
}
