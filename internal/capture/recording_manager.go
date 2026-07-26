// Purpose: Re-exports recording-manager constructors and delegates recording, replay, and log-diff calls.
// Why: Preserves capture package API compatibility while recording logic lives in internal/recording{,/playback,/logdiff}.
// Docs: docs/features/feature/playback-engine/index.md
// Docs: docs/features/feature/tab-recording/index.md

package capture

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording/logdiff"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording/playback"
)

// NewRecordingManager creates a RecordingManager with initialized state.
// Re-exported for backward compatibility with tests that call it directly.
var NewRecordingManager = recording.NewRecordingManager

// ============================================================================
// Capture delegation methods — preserve external API.
// ============================================================================

// StartRecording delegates to RecordingManager.
func (c *Capture) StartRecording(name, pageURL string, sensitiveDataEnabled bool) (string, error) {
	return c.recordingManager.StartRecording(name, pageURL, sensitiveDataEnabled)
}

// StopRecording delegates to RecordingManager.
func (c *Capture) StopRecording(recordingID string) (int, int64, error) {
	return c.recordingManager.StopRecording(recordingID)
}

// AddRecordingAction delegates to RecordingManager.
func (c *Capture) AddRecordingAction(action RecordingAction) error {
	return c.recordingManager.AddRecordingAction(action)
}

// ListRecordings delegates to RecordingManager.
func (c *Capture) ListRecordings(limit int) ([]Recording, error) {
	return c.recordingManager.ListRecordings(limit)
}

// GetRecording delegates to RecordingManager.
func (c *Capture) GetRecording(recordingID string) (*Recording, error) {
	return c.recordingManager.GetRecording(recordingID)
}

// StartPlayback delegates to the replay engine, reading recordings from RecordingManager.
func (c *Capture) StartPlayback(recordingID string) (*PlaybackSession, error) {
	return playback.Start(c.recordingManager, recordingID)
}

// ExecutePlayback delegates to the replay engine, reading recordings from RecordingManager.
func (c *Capture) ExecutePlayback(recordingID string) (*PlaybackSession, error) {
	return playback.Execute(c.recordingManager, recordingID)
}

// DetectFragileSelectors delegates to the replay engine.
func (c *Capture) DetectFragileSelectors(sessions []*PlaybackSession) map[string]bool {
	return playback.DetectFragileSelectors(sessions)
}

// GetPlaybackStatus delegates to the replay engine.
func (c *Capture) GetPlaybackStatus(session *PlaybackSession) map[string]any {
	return playback.Status(session)
}

// DiffRecordings delegates to the log-diff engine, reading recordings from RecordingManager.
func (c *Capture) DiffRecordings(originalID, replayID string) (*LogDiffResult, error) {
	return logdiff.Compare(c.recordingManager, originalID, replayID)
}

// CategorizeActionTypes delegates to the log-diff engine.
func (c *Capture) CategorizeActionTypes(rec *Recording) map[string]int {
	return logdiff.CategorizeActionTypes(rec)
}

// GetStorageInfo delegates to RecordingManager.
func (c *Capture) GetStorageInfo() (StorageInfo, error) {
	return c.recordingManager.GetStorageInfo()
}

// DeleteRecording delegates to RecordingManager.
func (c *Capture) DeleteRecording(recordingID string) error {
	return c.recordingManager.DeleteRecording(recordingID)
}

// RecalculateStorageUsed delegates to RecordingManager.
func (c *Capture) RecalculateStorageUsed() error {
	return c.recordingManager.RecalculateStorageUsed()
}
