// logger.go -- Implements bounded debug-log ring buffers for polling and HTTP diagnostic entries.
// Why: Provides lightweight operational diagnostics without unbounded memory growth.

package debuglog

import (
	"sync"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/types"
)

const LogSize = 50

// Logger manages two circular buffers for operator debugging:
// polling activity (sync/settings calls) and HTTP request/response logging.
type Logger struct {
	mu                sync.Mutex
	pollingLog        []types.PollingLogEntry
	pollingLogIndex   int
	httpDebugLog      []types.HTTPDebugEntry
	httpDebugLogIndex int
}

// NewLogger creates a Logger with pre-allocated circular buffers.
func NewLogger() Logger {
	return Logger{
		pollingLog:   make([]types.PollingLogEntry, LogSize),
		httpDebugLog: make([]types.HTTPDebugEntry, LogSize),
	}
}

// LogPollingActivity adds an entry to the circular polling log buffer.
func (dl *Logger) LogPollingActivity(entry types.PollingLogEntry) {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.pollingLog[dl.pollingLogIndex] = entry
	dl.pollingLogIndex = (dl.pollingLogIndex + 1) % LogSize
}

// LogHTTPDebugEntry adds an entry to the circular HTTP debug log buffer.
func (dl *Logger) LogHTTPDebugEntry(entry types.HTTPDebugEntry) {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.httpDebugLog[dl.httpDebugLogIndex] = entry
	dl.httpDebugLogIndex = (dl.httpDebugLogIndex + 1) % LogSize
}

// GetPollingLog returns a copy of the polling activity log.
func (dl *Logger) GetPollingLog() []types.PollingLogEntry {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	result := make([]types.PollingLogEntry, len(dl.pollingLog))
	copy(result, dl.pollingLog)
	return result
}

// GetHTTPDebugLog returns an independent copy of the HTTP debug log.
func (dl *Logger) GetHTTPDebugLog() []types.HTTPDebugEntry {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	result := make([]types.HTTPDebugEntry, len(dl.httpDebugLog))
	copy(result, dl.httpDebugLog)
	return result
}
