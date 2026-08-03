// Purpose: Handles recording disk persistence, directory resolution, and listing/load queries.
// Docs: docs/features/feature/playback-engine/index.md

package recording

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefile"
)

type recordingFilesystem interface {
	MkdirAll(string, fs.FileMode) error
	ReadDir(string) ([]os.DirEntry, error)
	ReadFile(string) ([]byte, error)
	WriteFile(string, []byte, fs.FileMode) error
}

type localRecordingFilesystem struct{}

func (localRecordingFilesystem) MkdirAll(path string, permissions fs.FileMode) error {
	return os.MkdirAll(path, permissions)
}
func (localRecordingFilesystem) ReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}
func (localRecordingFilesystem) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
func (localRecordingFilesystem) WriteFile(path string, data []byte, permissions fs.FileMode) error {
	return statefile.Write(path, data, permissions)
}

func (r *RecordingManager) filesystem() recordingFilesystem {
	if r != nil && r.files != nil {
		return r.files
	}
	// EXPECTED_ABSENCE: partial zero-value managers in package tests use the
	// canonical local boundary; production construction always initializes it.
	return localRecordingFilesystem{}
}

// ============================================================================
// Persistence
// ============================================================================

func primaryRecordingsDir() (string, error) {
	return state.RecordingsDir()
}

// recordingReadRoots returns the canonical recording directory.
func (r *RecordingManager) recordingReadRoots() ([]string, error) {
	primaryDir, err := primaryRecordingsDir()
	if err != nil {
		return nil, fmt.Errorf("cannot_determine_recordings_dir: %w", err)
	}

	return []string{primaryDir}, nil
}

// persistRecordingToDisk writes recording metadata.json to state/recordings/{id}/
func (r *RecordingManager) persistRecordingToDisk(recording *Recording) error {
	// Determine storage directory.
	recordingsDir, err := primaryRecordingsDir()
	if err != nil {
		return fmt.Errorf("cannot_find_recordings_dir: %w", err)
	}

	recordingDir := filepath.Join(recordingsDir, recording.ID)

	// Create directory.
	err = r.filesystem().MkdirAll(recordingDir, 0o700)
	if err != nil {
		r.reportRecovery("Event recording storage could not be prepared; the active recording remains available for retry.")
		return errors.New("recording_directory_create_failed")
	}

	// Create metadata.
	metadata := &RecordingMetadata{
		ID:                   recording.ID,
		Name:                 recording.Name,
		CreatedAt:            recording.CreatedAt,
		StartURL:             recording.StartURL,
		Viewport:             recording.Viewport,
		Duration:             recording.Duration,
		ActionCount:          recording.ActionCount,
		Actions:              recording.Actions,
		SensitiveDataEnabled: recording.SensitiveDataEnabled,
		TestID:               recording.TestID,
	}

	// Marshal to JSON.
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("json_marshal_failed: %w", err)
	}

	// Write file.
	metadataPath := filepath.Join(recordingDir, recordingMetadataFile)
	err = r.filesystem().WriteFile(metadataPath, data, 0o600)
	if err != nil {
		r.reportRecovery("Event recording metadata could not be persisted; the active recording remains available for retry.")
		return errors.New("recording_metadata_write_failed")
	}

	r.resolveRecovery()
	return nil
}

// ============================================================================
// Querying
// ============================================================================

// collectRecordingsFromRoots loads recordings from disk directories, deduplicating by name.
func (r *RecordingManager) collectRecordingsFromRoots(roots []string, limit int) ([]Recording, bool, error) {
	recordings := make([]Recording, 0)
	seen := make(map[string]bool)
	hadRecovery := false

	for _, recordingsDir := range roots {
		entries, err := r.filesystem().ReadDir(recordingsDir)
		if err != nil {
			if os.IsNotExist(err) {
				// EXPECTED_ABSENCE: no recordings directory exists before the first capture.
				continue
			}
			r.reportRecovery("Saved event recordings could not be listed; no incomplete result was returned.")
			return nil, true, errors.New("recording_list_read_failed")
		}
		for _, entry := range entries {
			if !entry.IsDir() || seen[entry.Name()] {
				continue
			}
			seen[entry.Name()] = true
			recording, err := r.loadRecordingFromDisk(entry.Name())
			if err != nil {
				hadRecovery = true
				continue
			}
			recordings = append(recordings, *recording)
			if limit > 0 && len(recordings) >= limit {
				return recordings, hadRecovery, nil
			}
		}
	}
	return recordings, hadRecovery, nil
}

// ListRecordings returns all saved recordings from disk.
func (r *RecordingManager) ListRecordings(limit int) ([]Recording, error) {
	roots, err := r.recordingReadRoots()
	if err != nil {
		return nil, err
	}

	recordings, hadRecovery, err := r.collectRecordingsFromRoots(roots, limit)
	if err != nil {
		return nil, err
	}
	if !hadRecovery {
		r.resolveRecovery()
	}

	// Sort by created_at (newest first).
	sort.Slice(recordings, func(i, j int) bool {
		t1, _ := time.Parse(time.RFC3339, recordings[i].CreatedAt)
		t2, _ := time.Parse(time.RFC3339, recordings[j].CreatedAt)
		return t2.Before(t1)
	})

	return recordings, nil
}

// GetRecording loads a specific recording by ID.
func (r *RecordingManager) GetRecording(recordingID string) (*Recording, error) {
	if err := ValidateRecordingID(recordingID); err != nil {
		return nil, err
	}
	recording, err := r.loadRecordingFromDisk(recordingID)
	if err == nil {
		r.resolveRecovery()
	}
	return recording, err
}

// LookupRecording returns a recording by ID, preferring the in-memory copy (an
// active or just-stopped recording) and falling back to disk.
//
// Unlike GetRecording this does not run ValidateRecordingID: an in-memory
// recording is addressed by the ID the manager itself minted, never by a
// caller-supplied path. It exists for the replay engine in
// internal/recording/playback, which must see unpersisted in-memory state.
func (r *RecordingManager) LookupRecording(recordingID string) (*Recording, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if rec, ok := r.recordings[recordingID]; ok {
		return rec, nil
	}
	return r.loadRecordingFromDisk(recordingID)
}

// loadRecordingFromDisk reads metadata.json and returns the Recording.
func (r *RecordingManager) loadRecordingFromDisk(recordingID string) (*Recording, error) {
	roots, err := r.recordingReadRoots()
	if err != nil {
		return nil, err
	}

	var lastErr error
	for _, root := range roots {
		metadataPath := filepath.Join(root, recordingID, recordingMetadataFile)

		// Read file.
		data, err := r.filesystem().ReadFile(metadataPath) // nosemgrep: go_filesystem_rule-fileread -- CLI tool reads local recording metadata
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			lastErr = errors.New("recording_metadata_read_failed")
			r.reportRecovery("Saved event recording metadata could not be read; the affected recording was omitted.")
			continue
		}

		// Unmarshal metadata.
		metadata := &RecordingMetadata{}
		err = json.Unmarshal(data, metadata)
		if err != nil {
			lastErr = errors.New("recording_metadata_corrupt")
			r.reportRecovery("Saved event recording metadata was malformed; the affected recording was omitted.")
			continue
		}

		// Convert to Recording.
		recording := &Recording{
			ID:                   metadata.ID,
			Name:                 metadata.Name,
			CreatedAt:            metadata.CreatedAt,
			StartURL:             metadata.StartURL,
			Viewport:             metadata.Viewport,
			Duration:             metadata.Duration,
			ActionCount:          metadata.ActionCount,
			Actions:              metadata.Actions,
			SensitiveDataEnabled: metadata.SensitiveDataEnabled,
			TestID:               metadata.TestID,
		}

		return recording, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("read_failed: recording not found: %s", recordingID)
}

func (r *RecordingManager) reportRecovery(detail string) {
	if r == nil {
		return
	}
	r.diagnosticsMu.RLock()
	diagnostics := r.diagnostics
	r.diagnosticsMu.RUnlock()
	if diagnostics == nil {
		return
	}
	diagnostics.Report(statediag.Diagnostic{
		Name:   "event_recording_state",
		Detail: detail,
		Fix:    "Delete the affected recording metadata or capture the recording again.",
	})
}

func (r *RecordingManager) resolveRecovery() {
	if r == nil {
		return
	}
	r.diagnosticsMu.RLock()
	diagnostics := r.diagnostics
	r.diagnosticsMu.RUnlock()
	statediag.Resolve(diagnostics, "event_recording_state")
}
