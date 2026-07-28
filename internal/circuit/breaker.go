// Purpose: Implements the capture ingest circuit breaker and rate-limiting state machine.
// Why: Protects daemon stability by throttling abusive event rates and exposing health state.
// Docs: docs/features/feature/rate-limiting/index.md

package circuit

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// Constants for circuit breaker behavior.
const (
	// RateLimitThreshold is the maximum events/second before rate limiting kicks in.
	RateLimitThreshold = 1000

	// CircuitOpenStreakCount is consecutive seconds over threshold to open circuit.
	CircuitOpenStreakCount = 5

	// CircuitCloseSeconds is seconds below threshold to close circuit.
	CircuitCloseSeconds = 10
)

// HealthResponse is returned by GET /health endpoint.
type HealthResponse struct {
	CircuitOpen bool   `json:"circuit_open"`
	OpenedAt    string `json:"opened_at,omitempty"`
	CurrentRate int    `json:"current_rate"`
	Reason      string `json:"reason,omitempty"`
}

// RateLimitResponse is the 429 response body.
type RateLimitResponse struct {
	Error        string `json:"error"`
	Message      string `json:"message"`
	RetryAfterMs int    `json:"retry_after_ms"`
	CircuitOpen  bool   `json:"circuit_open"`
	CurrentRate  int    `json:"current_rate"`
	Threshold    int    `json:"threshold"`
}

// CircuitBreaker implements a rate limiter with circuit breaker pattern.
// Uses a 1-second sliding window for event counting and a streak-based
// state machine for circuit open/close transitions.
type CircuitBreaker struct {
	mu                   sync.RWMutex
	windowEventCount     int
	rateWindowStart      time.Time
	rateLimitStreak      int
	lastBelowThresholdAt time.Time
	circuitOpen          bool
	circuitOpenedAt      time.Time
	circuitReason        string

	// Injected: emits lifecycle events (circuit_opened, circuit_closed)
	emitEvent lifecycle.Listener

	// Injected clock for deterministic tests; defaults to time.Now.
	now func() time.Time
}

// NewCircuitBreaker creates a CircuitBreaker with injected dependencies.
func NewCircuitBreaker(emitEvent lifecycle.Listener) *CircuitBreaker {
	now := time.Now()
	return &CircuitBreaker{
		rateWindowStart:      now,
		lastBelowThresholdAt: now,
		emitEvent:            emitEvent,
		now:                  time.Now,
	}
}

// IsOpen returns whether the circuit breaker is currently open (rejecting all requests).
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.circuitOpen
}

// ForceOpen opens the circuit breaker for testing purposes.
func (cb *CircuitBreaker) ForceOpen(reason string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.circuitOpen = true
	cb.circuitOpenedAt = cb.now()
	cb.circuitReason = reason
}

// SetWindowState sets the rate window state for testing.
func (cb *CircuitBreaker) SetWindowState(start time.Time, count int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.rateWindowStart = start
	cb.windowEventCount = count
}

// RecordEvents records N events received in the current 1-second window.
// Called by ingest handlers with batch sizes.
func (cb *CircuitBreaker) RecordEvents(count int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.advanceWindowLocked()
	cb.windowEventCount += count
}

// advanceWindowLocked ticks the rate window state machine when the current
// 1-second window has expired, then starts a fresh window. Caller must hold lock.
func (cb *CircuitBreaker) advanceWindowLocked() {
	now := cb.now()
	if now.Sub(cb.rateWindowStart) > time.Second {
		cb.tickRateWindow()
		cb.windowEventCount = 0
		cb.rateWindowStart = now
	}
}

// CheckRateLimit returns true if the request should be rejected (429).
// Checks: 1) circuit open, 2) window rate.
//
// When the 1-second window has expired this also advances the window state
// machine (streak counting + circuit evaluation). This matters because ingest
// handlers call CheckRateLimit before RecordEvents and short-circuit with a
// 429 while the circuit is open — without ticking here, the OPEN->CLOSED
// transition would be unreachable until restart.
func (cb *CircuitBreaker) CheckRateLimit() bool {
	cb.mu.RLock()
	expired := cb.now().Sub(cb.rateWindowStart) > time.Second
	if !expired {
		rejected := cb.circuitOpen || cb.windowEventCount > RateLimitThreshold
		cb.mu.RUnlock()
		return rejected
	}
	cb.mu.RUnlock()

	// Window expired: upgrade to the write lock and tick the state machine so
	// the circuit is re-evaluated (and can close) on the rejection path too.
	// advanceWindowLocked re-checks expiry, so a concurrent tick is harmless.
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.advanceWindowLocked()
	return cb.circuitOpen || cb.windowEventCount > RateLimitThreshold
}

// tickRateWindow is called when a 1-second window expires.
// Updates streak counter and evaluates circuit state. Caller must hold lock.
func (cb *CircuitBreaker) tickRateWindow() {
	if cb.windowEventCount > RateLimitThreshold {
		cb.rateLimitStreak++
		cb.lastBelowThresholdAt = time.Time{}
	} else {
		cb.rateLimitStreak = 0
		if cb.lastBelowThresholdAt.IsZero() {
			cb.lastBelowThresholdAt = cb.now()
		}
	}
	cb.evaluateCircuit()
}

// evaluateCircuit implements the circuit breaker FSM.
// CLOSED->OPEN: streak>=5. OPEN->CLOSED: streak=0 AND below for 10s.
// Caller must hold lock.
func (cb *CircuitBreaker) evaluateCircuit() {
	if !cb.circuitOpen {
		// Rate-based opening
		if cb.rateLimitStreak >= CircuitOpenStreakCount {
			cb.circuitOpen = true
			cb.circuitOpenedAt = cb.now()
			cb.circuitReason = "rate_exceeded"
			// Capture values before goroutine to avoid data race on struct fields
			streak := cb.rateLimitStreak
			rate := cb.windowEventCount
			emitFn := cb.emitEvent
			util.SafeGo(func() {
				emitFn(lifecycle.EventCircuitOpened, map[string]any{
					"reason":    "rate_exceeded",
					"streak":    streak,
					"rate":      rate,
					"threshold": RateLimitThreshold,
				})
			})
			return
		}
		return
	}

	// Check if circuit should close
	if cb.rateLimitStreak > 0 {
		return
	}
	if cb.lastBelowThresholdAt.IsZero() {
		return
	}
	if cb.now().Sub(cb.lastBelowThresholdAt) < time.Duration(CircuitCloseSeconds)*time.Second {
		return
	}

	// All conditions met -- close
	openDuration := cb.now().Sub(cb.circuitOpenedAt)
	prevReason := cb.circuitReason
	cb.circuitOpen = false
	cb.circuitReason = ""
	cb.rateLimitStreak = 0
	// Capture values before goroutine to avoid data race on struct fields
	rate := cb.windowEventCount
	emitFn := cb.emitEvent

	util.SafeGo(func() {
		emitFn(lifecycle.EventCircuitClosed, map[string]any{
			"previous_reason":    prevReason,
			"open_duration_secs": openDuration.Seconds(),
			"rate":               rate,
		})
	})
}

// GetHealthStatus returns the current health/circuit state.
func (cb *CircuitBreaker) GetHealthStatus() HealthResponse {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	resp := HealthResponse{
		CircuitOpen: cb.circuitOpen,
		CurrentRate: cb.windowEventCount,
		Reason:      cb.circuitReason,
	}
	if cb.circuitOpen {
		resp.OpenedAt = cb.circuitOpenedAt.Format(time.RFC3339)
	}
	return resp
}

// GetState returns circuit breaker state fields for external snapshot consumers.
// Used by Capture.GetHealthSnapshot() to avoid reentrant locking.
func (cb *CircuitBreaker) GetState() (open bool, reason string, openedAt time.Time, eventCount int) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.circuitOpen, cb.circuitReason, cb.circuitOpenedAt, cb.windowEventCount
}

// WriteRateLimitResponse writes a 429 response with JSON body.
func (cb *CircuitBreaker) WriteRateLimitResponse(w http.ResponseWriter) {
	cb.mu.RLock()
	currentRate := cb.windowEventCount
	isOpen := cb.circuitOpen
	cb.mu.RUnlock()

	resp := RateLimitResponse{
		Error:        "rate_limited",
		Message:      "Server receiving >1000 events/sec. Retry after backoff.",
		RetryAfterMs: 1000,
		CircuitOpen:  isOpen,
		CurrentRate:  currentRate,
		Threshold:    RateLimitThreshold,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", "1")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(resp)
}
