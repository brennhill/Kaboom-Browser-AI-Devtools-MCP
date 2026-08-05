// helpers_test.go — Shared composition fixtures for recording integration tests.
package recordingtest

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
)

func setupTestCapture(t *testing.T) *capture.Capture {
	t.Helper()
	state := capture.NewCapture()
	t.Cleanup(state.Close)
	return state
}

func mustStartRecording(t *testing.T, state *capture.Capture, name, pageURL string, sensitive bool) string {
	t.Helper()
	id, err := state.Recordings().StartRecording(name, pageURL, sensitive)
	if err != nil {
		t.Fatalf("StartRecording() error = %v", err)
	}
	return id
}

func mustGetRecording(t *testing.T, state *capture.Capture, id string) *recording.Recording {
	t.Helper()
	result, err := state.Recordings().GetRecording(id)
	if err != nil {
		t.Fatalf("GetRecording(%q) error = %v", id, err)
	}
	return result
}
