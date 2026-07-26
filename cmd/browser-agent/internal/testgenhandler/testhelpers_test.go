package testgenhandler

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// fakeDeps satisfies Deps over a real capture.Store and an in-memory log buffer.
// It replaces the old harness, which built a whole *ToolHandler (plus a *Server
// and a *logstore.Store) just to reach these three methods.
type fakeDeps struct {
	cap     *capture.Store
	entries []mcp.LogEntry
	stamps  []time.Time
}

func (f *fakeDeps) GetCapture() *capture.Store                   { return f.cap }
func (f *fakeDeps) GetLogEntries() ([]mcp.LogEntry, []time.Time) { return f.entries, f.stamps }
func (f *fakeDeps) GetLogTotalAdded() int64                      { return int64(len(f.entries)) }

// testEnv pairs a Handler with the capture store its Deps read from, so tests can
// seed actions/bodies and then assert on generated output.
type testEnv struct {
	h    *Handler
	cap  *capture.Store
	deps *fakeDeps
}

func newTestEnv() *testEnv {
	cap := capture.NewCapture()
	deps := &fakeDeps{cap: cap}
	return &testEnv{h: New(deps), cap: cap, deps: deps}
}

// newPureHandler builds a Handler for the classification/healing paths, which are
// pure functions of their arguments and never touch Deps. The old tests expressed
// this as `(&ToolHandler{}).testGen()`, which was a nil sub-handler.
func newPureHandler() *Handler { return New(nil) }
