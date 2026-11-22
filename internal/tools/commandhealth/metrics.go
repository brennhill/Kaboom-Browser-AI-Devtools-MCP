// metrics.go — Tracks per-tool request/error counters and uptime snapshots for health reporting.
// Why: Isolates mutable health state and synchronization from response assembly logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package health

import (
	"sync"
	"time"
)

// Metrics tracks server operational metrics for the get_health tool.
// All counters are thread-safe for concurrent access.
type Metrics struct {
	mu            sync.RWMutex
	startTime     time.Time
	requestCounts map[string]int64 // per-tool request counts
	errorCounts   map[string]int64 // per-tool error counts
}

// NewMetrics creates a new Metrics instance with initialized counters.
func NewMetrics() *Metrics {
	return &Metrics{
		startTime:     time.Now(),
		requestCounts: make(map[string]int64),
		errorCounts:   make(map[string]int64),
	}
}

// IncrementRequest increments the request count for the given tool.
func (hm *Metrics) IncrementRequest(toolName string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.requestCounts[toolName]++
}

// IncrementError increments the error count for the given tool.
func (hm *Metrics) IncrementError(toolName string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.errorCounts[toolName]++
}

// GetRequestCount returns the request count for the given tool.
func (hm *Metrics) GetRequestCount(toolName string) int64 {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.requestCounts[toolName]
}

// GetErrorCount returns the error count for the given tool.
func (hm *Metrics) GetErrorCount(toolName string) int64 {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.errorCounts[toolName]
}

// GetTotalRequests returns the total request count across all tools.
func (hm *Metrics) GetTotalRequests() int64 {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	var total int64
	for _, count := range hm.requestCounts {
		total += count
	}
	return total
}

// GetTotalErrors returns the total error count across all tools.
func (hm *Metrics) GetTotalErrors() int64 {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	var total int64
	for _, count := range hm.errorCounts {
		total += count
	}
	return total
}

// GetUptime returns the server uptime duration.
func (hm *Metrics) GetUptime() time.Duration {
	return time.Since(hm.startTime)
}
