package testgenhandler

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// fakeDeps satisfies Deps over a real capture.Capture and an in-memory log buffer.
// It replaces the old harness, which built a whole *ToolHandler (plus a *Server
// and a *logstore.Store) just to reach these three methods.
type fakeDeps struct {
	cap     *capture.Capture
	entries []types.LogEntry
}

func (f *fakeDeps) handlerDeps() Deps {
	return Deps{
		LogEntries:      func() []types.LogEntry { return f.entries },
		EnhancedActions: f.cap.Telemetry().GetAllEnhancedActions,
		NetworkBodies: func() []types.NetworkBody {
			return f.cap.Telemetry().NetworkBodies().Snapshot().Bodies
		},
	}
}

// testEnv pairs a Handler with the capture store its Deps read from, so tests can
// seed actions/bodies and then assert on generated output.
type testEnv struct {
	h    *Handler
	cap  *capture.Capture
	deps *fakeDeps
}

func newTestEnv() *testEnv {
	cap := capture.NewCapture()
	deps := &fakeDeps{cap: cap}
	return &testEnv{h: New(deps.handlerDeps()), cap: cap, deps: deps}
}

// newPureHandler builds a Handler for the classification/healing paths, which are
// pure functions of their arguments and never touch Deps. The old tests expressed
// this without an initialized test-generation handler.
func newPureHandler() *Handler { return New(Deps{}) }
