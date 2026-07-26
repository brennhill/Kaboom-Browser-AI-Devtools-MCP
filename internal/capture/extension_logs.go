// Purpose: Implements extension-internal log ingestion, redaction, ring-buffer storage and retrieval.
// Why: Redaction is part of the ingest path — every append already went through it.
// Docs: docs/features/feature/backend-log-streaming/index.md
// Docs: docs/features/feature/redaction-patterns/index.md

package capture

import (
	"encoding/json"
	"time"
)

// ============================================
// Extension Logs Handler
// ============================================
// Receives log entries from the browser extension's background script,
// content script, and other extension contexts.
//
// This enables AI debugging of extension-internal behavior that isn't
// visible through page-level console capture.

// AddExtensionLogs ingests extension runtime logs into bounded in-memory buffer.
//
// Invariants:
// - Logs are redacted before storage.
// - Buffer compaction keeps the newest MaxExtensionLogs entries.
//
// Failure semantics:
// - Missing timestamps are filled with server receive time.
func (c *Capture) AddExtensionLogs(logs []ExtensionLog) {
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, log := range logs {
		if log.Timestamp.IsZero() {
			log.Timestamp = now
		}
		log = c.redactExtensionLog(log)
		c.extensionLogs.append(log)
	}
}

// GetExtensionLogs returns a detached copy of extension logs.
//
// Failure semantics:
// - Returns empty slice when buffer is empty.
func (c *Capture) GetExtensionLogs() []ExtensionLog {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.extensionLogs.snapshot()
}

// redactExtensionLog scrubs sensitive data from extension log fields before storage.
func (c *Capture) redactExtensionLog(log ExtensionLog) ExtensionLog {
	if c.logRedactor == nil {
		return log
	}

	log.Message = c.logRedactor.Redact(log.Message)
	log.Source = c.logRedactor.Redact(log.Source)
	log.Category = c.logRedactor.Redact(log.Category)
	if len(log.Data) > 0 {
		log.Data = c.redactExtensionLogData(log.Data)
	}
	return log
}

func (c *Capture) redactExtensionLogData(data json.RawMessage) json.RawMessage {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return json.RawMessage(c.logRedactor.Redact(string(data)))
	}

	redacted := redactJSONValue(value, c.logRedactor.Redact)
	output, err := json.Marshal(redacted)
	if err != nil {
		return json.RawMessage(c.logRedactor.Redact(string(data)))
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

// append adds one extension log entry and applies amortized eviction.
func (b *ExtensionLogBuffer) append(log ExtensionLog) {
	b.logs = append(b.logs, log)
	evictionThreshold := MaxExtensionLogs + MaxExtensionLogs/2
	if len(b.logs) <= evictionThreshold {
		return
	}

	kept := make([]ExtensionLog, MaxExtensionLogs)
	copy(kept, b.logs[len(b.logs)-MaxExtensionLogs:])
	b.logs = kept
}

// snapshot returns a detached copy of the buffer contents.
func (b *ExtensionLogBuffer) snapshot() []ExtensionLog {
	out := make([]ExtensionLog, len(b.logs))
	copy(out, b.logs)
	return out
}

// clear removes all buffered logs and returns removed count.
func (b *ExtensionLogBuffer) clear() int {
	count := len(b.logs)
	b.logs = make([]ExtensionLog, 0)
	return count
}
