// Purpose: Provides test fixture constructors shared by capture package tests.
// Why: Reduces duplicated test bootstrapping code and keeps test setup behavior consistent.
// Docs: docs/features/feature/self-testing/index.md

package capture

import (
	"os"
	"path/filepath"
	"testing"

	recordingmodel "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/server"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestMain(m *testing.M) {
	stateRoot, err := os.MkdirTemp("", "kaboom-capture-tests-*")
	if err != nil {
		panic("create capture test state root: " + err.Error())
	}
	if err := os.Setenv(state.StateDirEnv, stateRoot); err != nil {
		panic("set capture test state root: " + err.Error())
	}
	code := m.Run()
	_ = os.RemoveAll(stateRoot)
	os.Exit(code)
}

// setupTestCapture creates a new Capture instance for testing.
func setupTestCapture(t *testing.T) *Capture {
	t.Helper()
	return NewCapture()
}

func mustStartRecording(t *testing.T, capture *Capture, name, pageURL string, sensitive bool) string {
	t.Helper()
	recordingID, err := capture.Recordings().StartRecording(name, pageURL, sensitive)
	if err != nil {
		t.Fatalf("StartRecording() error = %v", err)
	}
	return recordingID
}

func mustGetRecording(t *testing.T, capture *Capture, recordingID string) *recordingmodel.Recording {
	t.Helper()
	recording, err := capture.Recordings().GetRecording(recordingID)
	if err != nil {
		t.Fatalf("GetRecording(%q) error = %v", recordingID, err)
	}
	return recording
}

// setupTestServer creates a test instance of Server with a temporary log file.
func setupTestServer(t *testing.T) (*server.Server, string) {
	t.Helper()

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test-logs.jsonl")

	srv, err := server.NewServer(logFile, 10)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	return srv, logFile
}

// setupToolHandler creates a placeholder tool handler for integration tests.
// Returns nil — integration tests that need real handler behavior should
// construct their own via the cmd layer.
func setupToolHandler(t *testing.T, server *server.Server, capture *Capture) any {
	t.Helper()
	return nil
}
