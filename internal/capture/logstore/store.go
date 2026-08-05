// store.go — Owns redacted extension and daemon diagnostic log storage.
// Docs: docs/features/feature/backend-log-streaming/index.md
// Docs: docs/features/feature/redaction-patterns/index.md

package logstore

import (
	"bytes"
	"encoding/json"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/pressure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/debuglog"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ExtensionCapacity bounds locally retained extension diagnostics.
const ExtensionCapacity = 500

// Extension owns bounded extension logs, redaction, and synchronization.
type Extension struct {
	mu       sync.RWMutex
	logs     []types.ExtensionLog
	dropped  int64
	redactFn func(string) string
}

// NewExtension constructs an empty extension log store.
func NewExtension(redactFn func(string) string) *Extension {
	return &Extension{logs: make([]types.ExtensionLog, 0, ExtensionCapacity), redactFn: redactFn}
}

// Add ingests extension logs and fills missing timestamps at receive time.
func (s *Extension) Add(logs []types.ExtensionLog) { s.AddAt(logs, time.Now()) }

// AddAt ingests logs using one explicit receive timestamp.
func (s *Extension) AddAt(logs []types.ExtensionLog, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prepared := make([]types.ExtensionLog, len(logs))
	for index, log := range logs {
		log.Data = bytes.Clone(log.Data)
		if log.Timestamp.IsZero() {
			log.Timestamp = now
		}
		prepared[index] = s.redactLog(log)
	}
	s.logs = append(s.logs, prepared...)
	s.enforceCapacityLocked()
}

// Pressure returns machine-readable capacity, drop, and age metrics.
func (s *Extension) Pressure() pressure.Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats := pressure.Stats{Size: len(s.logs), Capacity: ExtensionCapacity, Dropped: s.dropped}
	if len(s.logs) > 0 && !s.logs[0].Timestamp.IsZero() {
		stats.OldestAge = time.Since(s.logs[0].Timestamp)
		if stats.OldestAge < 0 {
			stats.OldestAge = 0
		}
	}
	return stats
}

// Entries returns a detached copy of the buffered extension logs.
func (s *Extension) Entries() []types.ExtensionLog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]types.ExtensionLog, len(s.logs))
	for index, log := range s.logs {
		log.Data = bytes.Clone(log.Data)
		out[index] = log
	}
	return out
}

// Clear removes all buffered extension logs and returns the removed count.
func (s *Extension) Clear() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := len(s.logs)
	s.logs = make([]types.ExtensionLog, 0, ExtensionCapacity)
	return count
}

func (s *Extension) redactLog(log types.ExtensionLog) types.ExtensionLog {
	if s.redactFn == nil {
		return log
	}
	log.Message = s.redactFn(log.Message)
	log.Source = s.redactFn(log.Source)
	log.Category = s.redactFn(log.Category)
	if len(log.Data) > 0 {
		log.Data = redactData(log.Data, s.redactFn)
	}
	return log
}

func redactData(data json.RawMessage, redactFn func(string) string) json.RawMessage {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return json.RawMessage(redactFn(string(data)))
	}
	output, err := json.Marshal(redactJSONValue(value, redactFn))
	if err != nil {
		return json.RawMessage(redactFn(string(data)))
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
		for index, child := range typed {
			typed[index] = redactJSONValue(child, redactFn)
		}
		return typed
	case string:
		return redactFn(typed)
	default:
		return value
	}
}

func (s *Extension) enforceCapacityLocked() {
	overflow := len(s.logs) - ExtensionCapacity
	if overflow <= 0 {
		return
	}
	s.dropped += int64(overflow)
	kept := make([]types.ExtensionLog, ExtensionCapacity)
	copy(kept, s.logs[len(s.logs)-ExtensionCapacity:])
	s.logs = kept
}

// Diagnostic owns bounded daemon diagnostics and redaction.
type Diagnostic struct {
	logger   debuglog.Logger
	redactFn func(string) string
}

// NewDiagnostic constructs a daemon diagnostic store.
func NewDiagnostic(redactFn func(string) string) *Diagnostic {
	return &Diagnostic{logger: debuglog.NewLogger(), redactFn: redactFn}
}

func (s *Diagnostic) AddPolling(entry types.PollingLogEntry) { s.logger.LogPollingActivity(entry) }
func (s *Diagnostic) AddHTTP(entry types.HTTPDebugEntry) {
	s.logger.LogHTTPDebugEntry(s.redactHTTP(entry))
}
func (s *Diagnostic) HTTPEntries() []types.HTTPDebugEntry { return s.logger.GetHTTPDebugLog() }

func (s *Diagnostic) redactHTTP(entry types.HTTPDebugEntry) types.HTTPDebugEntry {
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
	entry.RequestBody = redactNonEmpty(entry.RequestBody, s.redactFn)
	entry.ResponseBody = redactNonEmpty(entry.ResponseBody, s.redactFn)
	entry.Error = redactNonEmpty(entry.Error, s.redactFn)
	return entry
}

func redactNonEmpty(value string, redactFn func(string) string) string {
	if value == "" {
		return ""
	}
	return redactFn(value)
}
