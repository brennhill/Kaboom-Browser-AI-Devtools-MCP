// Purpose: Provides capture-level polling and HTTP debug instrumentation with field redaction.
// Why: Preserves existing capture API call sites while storage lives in internal/debuglog.
// Docs: docs/features/feature/backend-log-streaming/index.md
// Docs: docs/features/feature/redaction-patterns/index.md

package capture

// DebugLogger, NewDebugLogger and debugLogSize are re-exported from internal/debuglog
// in aliases.go.

// logPollingActivity delegates to the DebugLogger sub-struct.
// Safe to call with or without c.mu held — DebugLogger has its own lock.
func (c *Capture) logPollingActivity(entry PollingLogEntry) {
	c.debug.LogPollingActivity(entry)
}

// LogHTTPDebugEntry logs an HTTP debug entry. Delegates to DebugLogger (own lock).
func (c *Capture) LogHTTPDebugEntry(entry HTTPDebugEntry) {
	c.debug.LogHTTPDebugEntry(c.redactHTTPDebugEntry(entry))
}

// GetHTTPDebugLog returns a copy of the HTTP debug log. Delegates to DebugLogger (own lock).
func (c *Capture) GetHTTPDebugLog() []HTTPDebugEntry {
	return c.debug.GetHTTPDebugLog()
}

// redactHTTPDebugEntry scrubs sensitive data from HTTP debug entry fields before storage.
func (c *Capture) redactHTTPDebugEntry(entry HTTPDebugEntry) HTTPDebugEntry {
	if c.logRedactor == nil {
		return entry
	}

	if len(entry.Headers) > 0 {
		redactedHeaders := make(map[string]string, len(entry.Headers))
		for key, value := range entry.Headers {
			redactedHeaders[key] = c.logRedactor.Redact(value)
		}
		entry.Headers = redactedHeaders
	}

	if entry.RequestBody != "" {
		entry.RequestBody = c.logRedactor.Redact(entry.RequestBody)
	}
	if entry.ResponseBody != "" {
		entry.ResponseBody = c.logRedactor.Redact(entry.ResponseBody)
	}
	if entry.Error != "" {
		entry.Error = c.logRedactor.Redact(entry.Error)
	}

	return entry
}
