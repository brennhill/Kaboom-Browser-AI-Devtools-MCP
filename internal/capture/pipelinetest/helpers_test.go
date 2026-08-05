// helpers_test.go — Shared composition fixtures for telemetry pipeline tests.
package pipelinetest

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/httpingest"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/resetter"
)

const (
	maxWSEvents         = 500
	wsBufferMemoryLimit = 4 * 1024 * 1024
)

func setupTestCapture(t *testing.T) *capture.Capture {
	t.Helper()
	state := capture.NewCapture()
	t.Cleanup(state.Close)
	return state
}

func httpIngestForTest(state *capture.Capture) *httpingest.Handlers {
	return httpingest.New(httpingest.Dependencies{Telemetry: state.Telemetry(), Queries: state.Queries(),
		Recordings: state.Recordings(), Performance: state.Performance(), Circuit: state.Circuit()})
}

func resetterForTest(state *capture.Capture) *resetter.Resetter {
	return resetter.New(resetter.Dependencies{Extension: state.Extension(), Telemetry: state.Telemetry(),
		Performance: state.Performance(), ExtensionLogs: state.ExtensionLogs()})
}
