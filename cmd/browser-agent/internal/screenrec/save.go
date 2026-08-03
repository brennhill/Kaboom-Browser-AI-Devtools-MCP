// Purpose: Handles recording upload parsing and persistence for /recordings/save.
// Why: Isolates write-path behavior from read/reveal paths for clearer tests and maintenance.
// Docs: docs/features/feature/tab-recording/index.md

package screenrec

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefile"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

type videoDiskOperations struct {
	createTemp    func(string, string) (*os.File, error)
	moveFile      func(string, string) error
	writeMetadata func(string, []byte, os.FileMode) error
	remove        func(string) error
}

func defaultVideoDiskOperations() videoDiskOperations {
	return videoDiskOperations{
		createTemp: os.CreateTemp, moveFile: statefile.Move,
		writeMetadata: statefile.Write, remove: os.Remove,
	}
}

// videoUpload holds the parsed multipart upload data for a recording save.
type videoUpload struct {
	videoFile io.ReadCloser
	meta      Metadata
	queryID   string
}

// parseVideoUpload extracts and validates the multipart fields from a recording save request.
// Returns nil and writes an HTTP error if validation fails.
func parseVideoUpload(w http.ResponseWriter, r *http.Request, diagnostics statediag.Reporter) *videoUpload {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		parseErr := strings.ToLower(err.Error())
		if strings.Contains(parseErr, "request body too large") {
			util.JSONResponse(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "RECORDING_SAVE: Upload exceeds 1GB limit"})
			return nil
		}
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "RECORDING_SAVE: Failed to parse multipart form"})
		return nil
	}

	videoFile, _, err := r.FormFile("video")
	if err != nil {
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "RECORDING_SAVE: Missing 'video' field"})
		return nil
	}

	meta, metaErr := parseVideoMetadata(r.FormValue("metadata"))
	if metaErr != "" {
		if closeErr := videoFile.Close(); closeErr != nil {
			reportSavedVideoRecovery(diagnostics, "A rejected screen-recording upload could not close its temporary input cleanly.")
		}
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": metaErr})
		return nil
	}

	return &videoUpload{videoFile: videoFile, meta: meta, queryID: r.FormValue("query_id")}
}

// parseVideoMetadata validates and parses the metadata JSON string.
// Returns the parsed metadata and an empty error string on success,
// or a zero metadata and an error message string on failure.
func parseVideoMetadata(metadataStr string) (Metadata, string) {
	if metadataStr == "" {
		return Metadata{}, "RECORDING_SAVE: Missing 'metadata' field. Include metadata JSON in the form."
	}

	var meta Metadata
	if err := json.Unmarshal([]byte(metadataStr), &meta); err != nil {
		return Metadata{}, "RECORDING_SAVE: Invalid metadata JSON"
	}

	if meta.Name == "" {
		return Metadata{}, "RECORDING_SAVE: Metadata missing 'name' field. Include a name in the metadata."
	}

	if strings.ContainsAny(meta.Name, "/\\") || strings.Contains(meta.Name, "..") {
		return Metadata{}, "RECORDING_SAVE: Invalid recording name — contains path separators. Use alphanumeric characters and hyphens."
	}
	meta.Name = sanitizeVideoSlug(meta.Name)

	return meta, ""
}

// writeVideoToDisk writes the video blob and metadata sidecar to dir.
// Returns the video path and final byte count, or an error string for the HTTP response.
func writeVideoToDisk(dir string, meta *Metadata, videoFile io.Reader) (string, error) {
	return writeVideoToDiskWithOperations(dir, meta, videoFile, defaultVideoDiskOperations())
}

func writeVideoToDiskWithOperations(dir string, meta *Metadata, videoFile io.Reader, ops videoDiskOperations) (string, error) {
	safeName := sanitizeVideoSlug(meta.Name)
	if safeName == "" {
		return "", fmt.Errorf("RECORDING_SAVE: Invalid recording name")
	}

	meta.Name = safeName
	outFile, err := ops.createTemp(dir, ".recording-*.webm")
	if err != nil {
		return "", errors.New("RECORDING_SAVE: video_create_failed")
	}

	temporaryPath := outFile.Name()
	videoPath := filepath.Join(dir, strings.TrimPrefix(filepath.Base(temporaryPath), "."))
	metaPath := strings.TrimSuffix(videoPath, ".webm") + "_meta.json"
	if !pathWithinDir(temporaryPath, dir) || !pathWithinDir(videoPath, dir) || !pathWithinDir(metaPath, dir) {
		closeErr := outFile.Close()
		cleanupErr := cleanupVideoPair(ops, temporaryPath, videoPath, metaPath)
		return "", errors.Join(errors.New("RECORDING_SAVE: invalid_recording_path"), stableVideoCleanupError(closeErr, cleanupErr))
	}

	written, err := io.Copy(outFile, videoFile)
	if err != nil {
		closeErr := outFile.Close()
		cleanupErr := cleanupVideoPair(ops, temporaryPath, videoPath, metaPath)
		return "", errors.Join(errors.New("RECORDING_SAVE: video_write_failed"), stableVideoCleanupError(closeErr, cleanupErr))
	}
	if err := outFile.Sync(); err != nil {
		closeErr := outFile.Close()
		cleanupErr := cleanupVideoPair(ops, temporaryPath, videoPath, metaPath)
		return "", errors.Join(errors.New("RECORDING_SAVE: video_sync_failed"), stableVideoCleanupError(closeErr, cleanupErr))
	}
	if err := outFile.Close(); err != nil {
		cleanupErr := cleanupVideoPair(ops, temporaryPath, videoPath, metaPath)
		return "", errors.Join(errors.New("RECORDING_SAVE: video_close_failed"), stableVideoCleanupError(nil, cleanupErr))
	}
	if err := ops.moveFile(temporaryPath, videoPath); err != nil {
		cleanupErr := cleanupVideoPair(ops, temporaryPath, videoPath, metaPath)
		return "", errors.Join(errors.New("RECORDING_SAVE: video_commit_failed"), stableVideoCleanupError(nil, cleanupErr))
	}

	meta.SizeBytes = written
	metaJSON, marshalErr := json.MarshalIndent(*meta, "", "  ")
	if marshalErr != nil {
		cleanupErr := cleanupVideoPair(ops, videoPath, metaPath)
		return "", errors.Join(errors.New("RECORDING_SAVE: metadata_encode_failed"), stableVideoCleanupError(nil, cleanupErr))
	}
	if err := ops.writeMetadata(metaPath, metaJSON, 0o600); err != nil {
		cleanupErr := cleanupVideoPair(ops, videoPath, metaPath)
		return "", errors.Join(errors.New("RECORDING_SAVE: metadata_write_failed"), stableVideoCleanupError(nil, cleanupErr))
	}

	return videoPath, nil
}

func cleanupVideoPair(ops videoDiskOperations, paths ...string) error {
	var cleanupErr error
	for _, path := range paths {
		if err := ops.remove(path); err != nil && !os.IsNotExist(err) {
			cleanupErr = errors.Join(cleanupErr, errors.New("recording_pair_cleanup_failed"))
		}
	}
	return cleanupErr
}

func stableVideoCleanupError(closeErr, cleanupErr error) error {
	if closeErr != nil {
		cleanupErr = errors.Join(cleanupErr, errors.New("recording_file_close_failed"))
	}
	return cleanupErr
}

// HandleSave handles POST /recordings/save from the extension.
// Accepts multipart form with "video" (binary) and "metadata" (JSON string) fields.
func HandleSave(
	w http.ResponseWriter,
	r *http.Request,
	cap interface{ SetQueryResult(string, json.RawMessage) },
	diagnostics statediag.Reporter,
) {
	handleSaveWithOperations(w, r, cap, diagnostics, defaultVideoDiskOperations())
}

func handleSaveWithOperations(
	w http.ResponseWriter,
	r *http.Request,
	cap interface{ SetQueryResult(string, json.RawMessage) },
	diagnostics statediag.Reporter,
	ops videoDiskOperations,
) {
	if r.Method != "POST" {
		util.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadSizeBytes)

	upload := parseVideoUpload(w, r, diagnostics)
	if upload == nil {
		return
	}
	defer func() {
		if closeErr := upload.videoFile.Close(); closeErr != nil {
			reportSavedVideoRecovery(diagnostics, "A screen-recording upload could not close its temporary input cleanly.")
		}
		if r.MultipartForm != nil {
			if cleanupErr := r.MultipartForm.RemoveAll(); cleanupErr != nil {
				reportSavedVideoRecovery(diagnostics, "Temporary screen-recording upload files could not be fully cleaned up.")
			}
		}
	}()

	dir, err := Dir()
	if err != nil {
		util.JSONResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	videoPath, writeErr := writeVideoToDiskWithOperations(dir, &upload.meta, upload.videoFile, ops)
	if writeErr != nil {
		reportSavedVideoRecovery(diagnostics, "A screen recording could not be committed as a complete video and metadata pair; incomplete files were removed.")
		util.JSONResponse(w, http.StatusInternalServerError, map[string]string{"error": writeErr.Error()})
		return
	}
	statediag.Resolve(diagnostics, "saved_video_state")

	if upload.queryID != "" && cap != nil {
		// Error impossible: map contains only primitive types from input
		result := mcp.SafeMarshal(map[string]any{
			"status":           "saved",
			"name":             upload.meta.Name,
			"path":             videoPath,
			"duration_seconds": upload.meta.DurationSeconds,
			"size_bytes":       upload.meta.SizeBytes,
			"truncated":        upload.meta.Truncated,
		}, "{}")
		cap.SetQueryResult(upload.queryID, result)
	}

	util.JSONResponse(w, http.StatusOK, map[string]any{
		"status": "saved",
		"name":   upload.meta.Name,
		"path":   videoPath,
		"size":   upload.meta.SizeBytes,
	})
}
