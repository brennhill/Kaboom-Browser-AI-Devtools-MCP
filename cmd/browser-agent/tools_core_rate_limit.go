// Purpose: Shared correlation-id and tool-call rate-limiting utilities for ToolHandler.
// Why: Keeps reusable concurrency/time primitives separate from core handler wiring.

package main

import (
	"crypto/rand"
	"encoding/binary"
	"sync"
	"time"
)

// randomInt63 generates a random int64 for correlation IDs using crypto/rand.
func randomInt63() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback to time-based if rand fails (should never happen).
		return time.Now().UnixNano()
	}
	// #nosec G115 -- masked to the positive int63 range (top bit cleared); the uint64->int64 conversion cannot overflow.
	return int64(binary.BigEndian.Uint64(b[:]) & 0x7FFFFFFFFFFFFFFF)
}

// ToolCallLimiter implements a sliding window rate limiter for MCP tool calls.
// Thread-safe: uses its own mutex independent of other locks.
type ToolCallLimiter struct {
	mu         sync.Mutex
	timestamps []time.Time
	maxCalls   int
	window     time.Duration
}

// NewToolCallLimiter creates a rate limiter allowing maxCalls within the given window.
func NewToolCallLimiter(maxCalls int, window time.Duration) *ToolCallLimiter {
	return &ToolCallLimiter{
		timestamps: make([]time.Time, 0, maxCalls),
		maxCalls:   maxCalls,
		window:     window,
	}
}

// Allow checks if a new call is permitted. If allowed, records it and returns true.
func (l *ToolCallLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	// Compact: remove expired timestamps.
	valid := 0
	for _, ts := range l.timestamps {
		if ts.After(cutoff) {
			l.timestamps[valid] = ts
			valid++
		}
	}
	l.timestamps = l.timestamps[:valid]

	if len(l.timestamps) >= l.maxCalls {
		return false
	}

	l.timestamps = append(l.timestamps, now)
	return true
}
