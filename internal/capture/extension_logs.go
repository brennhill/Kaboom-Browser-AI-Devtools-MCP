// extension_logs.go — Extension and daemon diagnostic log ingestion and retrieval.
// Purpose: Owns capture-level logging, redaction, and bounded extension log storage.
// Why: Both log paths share the same redactor and diagnostic boundary.
// Docs: docs/features/feature/backend-log-streaming/index.md
// Docs: docs/features/feature/redaction-patterns/index.md

package capture

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/debuglog"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// Extension Logs Handler
// ============================================
// Receives log entries from the browser extension's background script,
// content script, and other extension contexts.
//
// This enables AI debugging of extension-internal behavior that isn't
// visible through page-level console capture.

// ExtensionLogStore owns bounded extension logs, redaction, and synchronization.
type ExtensionLogStore struct {
	mu       sync.RWMutex
	logs     []types.ExtensionLog
	dropped  int64
	redactFn func(string) string
}

func newExtensionLogStore(redactFn func(string) string) *ExtensionLogStore {
	return &ExtensionLogStore{
		logs:     make([]types.ExtensionLog, 0, MaxExtensionLogs),
		redactFn: redactFn,
	}
}

// ExtensionLogs returns the independently synchronized extension-log owner.
func (c *Capture) ExtensionLogs() *ExtensionLogStore {
	return c.extensionLogs
}

// Add ingests extension runtime logs and fills missing timestamps at receive time.
func (s *ExtensionLogStore) Add(logs []types.ExtensionLog) {
	s.addAt(logs, time.Now())
}

func (s *ExtensionLogStore) addAt(logs []types.ExtensionLog, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prepared := make([]types.ExtensionLog, len(logs))
	for index, log := range logs {
		if log.Timestamp.IsZero() {
			log.Timestamp = now
		}
		prepared[index] = s.redactLog(log)
	}
	s.logs = append(s.logs, prepared...)
	s.enforceCapacityLocked()
}

// Pressure returns machine-readable capacity, drop, and age metrics.
func (s *ExtensionLogStore) Pressure() PressureStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats := PressureStats{Size: len(s.logs), Capacity: MaxExtensionLogs, Dropped: s.dropped}
	if len(s.logs) > 0 && !s.logs[0].Timestamp.IsZero() {
		stats.OldestAge = time.Since(s.logs[0].Timestamp)
		if stats.OldestAge < 0 {
			stats.OldestAge = 0
		}
	}
	return stats
}

// Entries returns a detached copy of the buffered extension logs.
func (s *ExtensionLogStore) Entries() []types.ExtensionLog {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]types.ExtensionLog, len(s.logs))
	copy(out, s.logs)
	return out
}

// Clear removes all buffered extension logs and returns the removed count.
func (s *ExtensionLogStore) Clear() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := len(s.logs)
	s.logs = make([]types.ExtensionLog, 0, MaxExtensionLogs)
	return count
}

func (s *ExtensionLogStore) redactLog(log types.ExtensionLog) types.ExtensionLog {
	if s.redactFn == nil {
		return log
	}

	log.Message = s.redactFn(log.Message)
	log.Source = s.redactFn(log.Source)
	log.Category = s.redactFn(log.Category)
	if len(log.Data) > 0 {
		log.Data = s.redactData(log.Data)
	}
	return log
}

func (s *ExtensionLogStore) redactData(data json.RawMessage) json.RawMessage {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return json.RawMessage(s.redactFn(string(data)))
	}

	redacted := redactJSONValue(value, s.redactFn)
	output, err := json.Marshal(redacted)
	if err != nil {
		return json.RawMessage(s.redactFn(string(data)))
	}
	return output
}

func redactJSONValue(value any, redactFn func(string) string) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			typed[key] = redactJSONValue(child, redactFn)
		}
		return typed
	case []any:
		for i, child := range typed {
			typed[i] = redactJSONValue(child, redactFn)
		}
		return typed
	case string:
		return redactFn(typed)
	default:
		return value
	}
}

func (s *ExtensionLogStore) enforceCapacityLocked() {
	overflow := len(s.logs) - MaxExtensionLogs
	if overflow <= 0 {
		return
	}
	s.dropped += int64(overflow)
	kept := make([]types.ExtensionLog, MaxExtensionLogs)
	copy(kept, s.logs[len(s.logs)-MaxExtensionLogs:])
	s.logs = kept
}

// DiagnosticLogStore owns bounded daemon diagnostics and redaction.
type DiagnosticLogStore struct {
	logger   debuglog.Logger
	redactFn func(string) string
}

func newDiagnosticLogStore(redactFn func(string) string) *DiagnosticLogStore {
	return &DiagnosticLogStore{
		logger:   debuglog.NewLogger(),
		redactFn: redactFn,
	}
}

// DiagnosticLogs returns the canonical redacted diagnostic-log owner.
func (c *Capture) DiagnosticLogs() *DiagnosticLogStore {
	return c.diagnosticLogs
}

func (s *DiagnosticLogStore) AddPolling(entry types.PollingLogEntry) {
	s.logger.LogPollingActivity(entry)
}

func (s *DiagnosticLogStore) AddHTTP(entry types.HTTPDebugEntry) {
	s.logger.LogHTTPDebugEntry(s.redactHTTP(entry))
}

func (s *DiagnosticLogStore) HTTPEntries() []types.HTTPDebugEntry {
	return s.logger.GetHTTPDebugLog()
}

func (s *DiagnosticLogStore) redactHTTP(entry types.HTTPDebugEntry) types.HTTPDebugEntry {
	if s.redactFn == nil {
		return entry
	}
	if len(entry.Headers) > 0 {
		redactedHeaders := make(map[string]string, len(entry.Headers))
		for key, value := range entry.Headers {
			redactedHeaders[key] = s.redactFn(value)
		}
		entry.Headers = redactedHeaders
	}
	if entry.RequestBody != "" {
		entry.RequestBody = s.redactFn(entry.RequestBody)
	}
	if entry.ResponseBody != "" {
		entry.ResponseBody = s.redactFn(entry.ResponseBody)
	}
	if entry.Error != "" {
		entry.Error = s.redactFn(entry.Error)
	}
	return entry
}
