// helpers_test.go — Shared composition fixtures for WebSocket capture tests.
package websockettest

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/httpingest"
)

func setupTestCapture(t *testing.T) *capture.Capture {
	t.Helper()
	state := capture.NewCapture()
	t.Cleanup(state.Close)
	return state
}

func httpIngestForTest(state *capture.Capture) *httpingest.Handlers {
	return httpingest.New(httpingest.Dependencies{
		Telemetry: state.Telemetry(), Queries: state.Queries(), Recordings: state.Recordings(),
		Performance: state.Performance(), Circuit: state.Circuit(),
	})
}
