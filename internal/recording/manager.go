// Purpose: Manages recording lifecycle: start/stop and in-memory action capture state.
// Docs: docs/features/feature/playback-engine/index.md

package recording

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

// ============================================================================
// Constants
// ============================================================================

const (
	RecordingStorageMax   = 1024 * 1024 * 1024 // 1GB max storage
	RecordingWarningLevel = 800 * 1024 * 1024  // 800MB warning threshold (80%)
	recordingMetadataFile = "metadata.json"
)

// ============================================================================
// Storage Info Types
// ============================================================================

// StorageInfo provides information about recording storage usage.
type StorageInfo struct {
	UsedBytes      int64   `json:"used_bytes"`      // Current storage usage in bytes
	MaxBytes       int64   `json:"max_bytes"`       // Maximum storage limit in bytes
	WarningBytes   int64   `json:"warning_bytes"`   // Warning threshold in bytes
	UsedPercent    float64 `json:"used_percent"`    // Storage usage as percentage
	WarningLevel   bool    `json:"warning_level"`   // True if at or above warning threshold
	RecordingCount int     `json:"recording_count"` // Number of recordings stored
}

// ============================================================================
// RecordingManager
// ============================================================================

// RecordingManager manages recording lifecycle, persistence, and storage tracking.
// Owns its own sync.Mutex — independent of Capture.mu.
type RecordingManager struct {
	mu                   sync.Mutex
	activeRecordingID    string
	recordings           map[string]*Recording
	recordingStorageUsed int64
	diagnosticsMu        sync.RWMutex
	diagnostics          statediag.Reporter
}

// SetDiagnostics connects recording recovery to the owning server's Doctor collector.
func (r *RecordingManager) SetDiagnostics(diagnostics statediag.Reporter) {
	if r == nil {
		return
	}
	r.diagnosticsMu.Lock()
	defer r.diagnosticsMu.Unlock()
	r.diagnostics = diagnostics
}

// NewRecordingManager creates a RecordingManager with initialized state.
func NewRecordingManager() *RecordingManager {
	return &RecordingManager{
		recordings: make(map[string]*Recording),
	}
}

// ============================================================================
// Validation
// ============================================================================

// ValidateRecordingID rejects IDs containing path traversal sequences.
func ValidateRecordingID(id string) error {
	if id == "" {
		return fmt.Errorf("recording_id_empty: Recording ID must not be empty")
	}
	if strings.Contains(id, "..") || strings.ContainsAny(id, `/\\`) {
		return fmt.Errorf("recording_id_invalid: Recording ID contains illegal characters")
	}
	// After cleaning, the ID must be a single path component.
	if filepath.Base(id) != id {
		return fmt.Errorf("recording_id_invalid: Recording ID must be a single directory name")
	}
	return nil
}

// ============================================================================
// Recording Lifecycle Methods
// ============================================================================

// StartRecording starts a new recording session.
// Returns recording_id and error status.
func (r *RecordingManager) StartRecording(name string, pageURL string, sensitiveDataEnabled bool) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if already recording.
	if r.activeRecordingID != "" {
		return "", fmt.Errorf("already_recording: A recording is already active (id: %s)", r.activeRecordingID)
	}

	// Check storage quota.
	if r.recordingStorageUsed >= RecordingStorageMax {
		return "", fmt.Errorf("recording_storage_full: Recording storage at capacity (1GB). Please delete old recordings")
	}

	// Warn if approaching limit (80%) - goes to stderr, not stdout (MCP stdio silence).
	if r.recordingStorageUsed >= RecordingWarningLevel {
		fmt.Fprintf(os.Stderr, "[WARNING] recording_storage_warning: Recording storage at 80%% (%d bytes / %d bytes)\n",
			r.recordingStorageUsed, RecordingStorageMax)
	}

	// Generate recording ID: name-YYYYMMDDTHHMMSS-nnnnnnnnnZ (nanosecond precision prevents collisions).
	now := time.Now()
	timestamp := fmt.Sprintf("%s-%09dZ", now.Format("20060102T150405"), now.Nanosecond())
	var recordingID string
	if name != "" {
		recordingID = fmt.Sprintf("%s-%s", name, timestamp)
	} else {
		// Auto-name from page title or URL.
		recordingID = fmt.Sprintf("recording-%s", timestamp)
	}

	// Create recording in memory.
	recording := &Recording{
		ID:                   recordingID,
		Name:                 name,
		CreatedAt:            now.Format(time.RFC3339),
		StartURL:             pageURL,
		Actions:              make([]RecordingAction, 0),
		SensitiveDataEnabled: sensitiveDataEnabled,
		TestID:               "", // Can be set later.
	}

	// Try to get viewport from the last EnhancedAction (hack but works for now).
	// In reality, this would come from the extension.
	recording.Viewport = ViewportInfo{Width: 1920, Height: 1080}

	// Store in memory.
	r.recordings[recordingID] = recording
	r.activeRecordingID = recordingID

	return recordingID, nil
}

// StopRecording stops the current recording and persists it to disk.
// Returns action count and duration.
func (r *RecordingManager) StopRecording(recordingID string) (int, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate recording exists.
	recording, exists := r.recordings[recordingID]
	if !exists {
		return 0, 0, fmt.Errorf("recording_not_found: No active recording with id: %s", recordingID)
	}

	// Calculate duration.
	startTime, _ := time.Parse(time.RFC3339, recording.CreatedAt)
	duration := time.Since(startTime).Milliseconds()
	recording.Duration = duration

	// Count actions.
	actionCount := len(recording.Actions)
	recording.ActionCount = actionCount

	// Persist to disk.
	err := r.persistRecordingToDisk(recording)
	if err != nil {
		return 0, 0, fmt.Errorf("recording_save_failed: Failed to save recording: %w", err)
	}

	// Update storage used.
	r.recordingStorageUsed += CalculateRecordingSize(recording)

	// Clear active recording.
	if r.activeRecordingID == recordingID {
		r.activeRecordingID = ""
	}

	return actionCount, duration, nil
}

// AddRecordingAction adds an action to the current recording.
func (r *RecordingManager) AddRecordingAction(action RecordingAction) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.activeRecordingID == "" {
		return fmt.Errorf("not_recording: No active recording")
	}

	recording := r.recordings[r.activeRecordingID]
	if recording == nil {
		return fmt.Errorf("recording_missing: Active recording not found")
	}

	// Redact sensitive data if needed.
	if !recording.SensitiveDataEnabled {
		// Redact text on type actions.
		if action.Type == "type" && action.Text != "" {
			action.Text = "[redacted]"
		}
	}

	// Set timestamp if not provided.
	if action.TimestampMs == 0 {
		action.TimestampMs = time.Now().UnixMilli()
	}

	recording.Actions = append(recording.Actions, action)
	return nil
}
