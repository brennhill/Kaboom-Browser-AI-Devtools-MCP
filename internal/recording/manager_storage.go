// Purpose: Handles recording storage-size estimation, quota tracking, and delete/recount operations.
// Docs: docs/features/feature/playback-engine/index.md

package recording

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ============================================================================
// Helpers
// ============================================================================

// CalculateRecordingSize estimates the size of a recording in bytes.
func CalculateRecordingSize(recording *Recording) int64 {
	// Rough estimate: metadata (JSON overhead) + actions.
	// Each action: ~200 bytes (type, timestamps, selectors, text).
	size := int64(len(recording.Name) + len(recording.StartURL) + len(recording.TestID) + 500)
	size += int64(len(recording.Actions)) * 200
	return size
}

// ============================================================================
// Storage Management
// ============================================================================

// GetStorageInfo returns information about recording storage usage.
func (r *RecordingManager) GetStorageInfo() (StorageInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Recalculate storage from disk for accuracy.
	usedBytes, recordingCount, err := r.calculateStorageFromDisk()
	if err != nil {
		r.reportRecovery("Recording storage usage could not be measured; quota-sensitive operations are unavailable.")
		return StorageInfo{}, errors.New("recording_storage_measurement_failed")
	}

	// Update in-memory tracking.
	r.recordingStorageUsed = usedBytes
	r.resolveRecovery()

	usedPercent := float64(usedBytes) / float64(RecordingStorageMax) * 100

	return StorageInfo{
		UsedBytes:      usedBytes,
		MaxBytes:       RecordingStorageMax,
		WarningBytes:   RecordingWarningLevel,
		UsedPercent:    usedPercent,
		WarningLevel:   usedBytes >= RecordingWarningLevel,
		RecordingCount: recordingCount,
	}, nil
}

// deleteRecordingFromRoot attempts to delete a recording from a single root directory.
// Returns bytes removed and whether the recording was found.
func (r *RecordingManager) deleteRecordingFromRoot(root, recordingID string) (int64, bool, error) {
	recordingDir := filepath.Join(root, recordingID)
	if _, statErr := r.filesystem().Stat(recordingDir); os.IsNotExist(statErr) {
		// EXPECTED_ABSENCE: the requested recording has already been removed or never existed.
		return 0, false, nil
	} else if statErr != nil {
		r.reportRecovery("Saved recording state could not be inspected for deletion.")
		return 0, false, errors.New("recording_delete_stat_failed")
	}

	sizeBefore, sizeErr := r.getDirectorySize(recordingDir)
	if sizeErr != nil {
		r.reportRecovery("Saved recording size could not be measured before deletion.")
		return 0, false, errors.New("recording_delete_measurement_failed")
	}
	if removeErr := r.filesystem().RemoveAll(recordingDir); removeErr != nil {
		r.reportRecovery("Saved recording files could not be removed; the recording remains available for retry.")
		return 0, false, errors.New("recording_delete_failed")
	}
	return sizeBefore, true, nil
}

// DeleteRecording deletes a recording from disk and updates storage tracking.
func (r *RecordingManager) DeleteRecording(recordingID string) error {
	if err := ValidateRecordingID(recordingID); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	roots, err := r.recordingReadRoots()
	if err != nil {
		return err
	}

	var totalRemoved int64
	found := false
	for _, root := range roots {
		removed, ok, delErr := r.deleteRecordingFromRoot(root, recordingID)
		if delErr != nil {
			return delErr
		}
		if ok {
			totalRemoved += removed
			found = true
		}
	}

	if !found {
		return fmt.Errorf("recording_not_found: No recording with id: %s", recordingID)
	}

	r.recordingStorageUsed -= totalRemoved
	if r.recordingStorageUsed < 0 {
		r.recordingStorageUsed = 0
	}
	delete(r.recordings, recordingID)
	r.resolveRecovery()
	return nil
}

// RecalculateStorageUsed recalculates storage usage from disk.
// Useful for recovering from inconsistencies or on startup.
func (r *RecordingManager) RecalculateStorageUsed() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	usedBytes, _, err := r.calculateStorageFromDisk()
	if err != nil {
		r.reportRecovery("Recording storage usage could not be recalculated; the previous in-memory quota value remains active.")
		return errors.New("recording_storage_measurement_failed")
	}

	r.recordingStorageUsed = usedBytes
	r.resolveRecovery()
	return nil
}

// calculateStorageFromDisk calculates total storage usage by scanning disk.
// Returns (bytes used, recording count, error).
func (r *RecordingManager) calculateStorageFromDisk() (int64, int, error) {
	roots, err := r.recordingReadRoots()
	if err != nil {
		return 0, 0, err
	}

	var totalSize int64
	recordingCount := 0
	seen := make(map[string]bool)

	for _, recordingsDir := range roots {
		size, count, scanErr := r.scanRootForStorage(recordingsDir, seen)
		if scanErr != nil {
			return 0, 0, scanErr
		}
		totalSize += size
		recordingCount += count
	}

	return totalSize, recordingCount, nil
}

// scanRootForStorage scans a single root directory for recording sizes.
func (r *RecordingManager) scanRootForStorage(dir string, seen map[string]bool) (int64, int, error) {
	entries, err := r.filesystem().ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// EXPECTED_ABSENCE: no recording root exists before the first saved recording.
			return 0, 0, nil
		}
		return 0, 0, errors.New("recording_storage_list_failed")
	}

	var totalSize int64
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() || seen[entry.Name()] {
			continue
		}
		seen[entry.Name()] = true
		size, sizeErr := r.getDirectorySize(filepath.Join(dir, entry.Name()))
		if sizeErr != nil {
			return 0, 0, errors.New("recording_storage_measurement_failed")
		}
		totalSize += size
		count++
	}
	return totalSize, count, nil
}

// getDirectorySize calculates the total size of a directory recursively.
func (r *RecordingManager) getDirectorySize(dirPath string) (int64, error) {
	var size int64

	err := r.filesystem().Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}
