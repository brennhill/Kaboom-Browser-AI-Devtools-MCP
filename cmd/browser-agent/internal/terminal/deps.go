// deps.go -- Dependency interfaces for the terminal package.
// Why: Defines narrow interfaces so the terminal subsystem depends on abstractions, not the god Server type.

package terminal

import (
	"bufio"
	"io"
	"net/http"

	terminalintent "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/intent"
)

// ServerDeps provides the subset of Server behavior needed by terminal handlers.
type ServerDeps interface {
	GetActiveCodebase() string
	SetActiveCodebase(path string)
}

// IntentDeps provides access to the intent store and PTY relay injection
// from the main Server. Used by intent handlers.
type IntentDeps interface {
	GetPtyRelays() RelayMap
	GetIntentStore() *terminalintent.Store
}

// RelayMap is the interface for terminal relay map operations used by intent handlers.
type RelayMap interface {
	WriteToFirst(data []byte) bool
	CloseAll()
}

// Deps bundles all dependencies needed to register terminal routes.
type Deps struct {
	JSONResponse   func(w http.ResponseWriter, status int, data any)
	CORSMiddleware func(next http.HandlerFunc) http.HandlerFunc
	Stderrf        func(format string, args ...any)
	// LogEvent emits a structured terminal lifecycle event (session spawn/exit,
	// WS connect/disconnect) to the daemon log, so a terminal outage is
	// diagnosable after the fact. May be nil; use deps.logEvent to call safely.
	LogEvent    func(event string, fields map[string]any)
	MaxPostBody int64

	// WebSocket codec functions injected from the main package.
	WSReadFrame  func(r io.Reader) (fin bool, opcode byte, payload []byte, err error)
	WSWriteFrame func(w *bufio.ReadWriter, opcode byte, payload []byte) error
	WSAcceptKey  func(key string) string
}

// logEvent emits a structured lifecycle event if a sink is wired (nil-safe).
func (d Deps) logEvent(event string, fields map[string]any) {
	if d.LogEvent != nil {
		d.LogEvent(event, fields)
	}
}
