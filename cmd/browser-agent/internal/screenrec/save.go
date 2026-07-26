// Purpose: Handles recording upload parsing and persistence for /recordings/save.
// Why: Isolates write-path behavior from read/reveal paths for clearer tests and maintenance.
// Docs: docs/features/feature/tab-recording/index.md

package screenrec

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// videoUpload holds the parsed multipart upload data for a recording save.
type videoUpload struct {
	videoFile io.ReadCloser
	meta      Metadata
	queryID   string
}

// parseVideoUpload extracts and validates the multipart fields from a recording save request.
// Returns nil and writes an HTTP error if validation fails.
func parseVideoUpload(w http.ResponseWriter, r *http.Request) *videoUpload {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		parseErr := strings.ToLower(err.Error())
		if strings.Contains(parseErr, "request body too large") {
			util.JSONResponse(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "RECORDING_SAVE: Upload exceeds 1GB limit"})
			return nil
		}
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "RECORDING_SAVE: Failed to parse multipart form. " + err.Error()})
		return nil
	}

	videoFile, _, err := r.FormFile("video")
	if err != nil {
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "RECORDING_SAVE: Missing 'video' field. " + err.Error()})
		return nil
	}

	meta, metaErr := parseVideoMetadata(r.FormValue("metadata"))
	if metaErr != "" {
		_ = videoFile.Close()
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
		return Metadata{}, "RECORDING_SAVE: Invalid metadata JSON. " + err.Error()
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
	safeName := sanitizeVideoSlug(meta.Name)
	if safeName == "" {
		return "", fmt.Errorf("RECORDING_SAVE: Invalid recording name")
	}

	meta.Name = safeName
	outFile, err := os.CreateTemp(dir, "recording-*.webm")
	if err != nil {
		return "", fmt.Errorf("RECORDING_SAVE: Failed to create file. %w", err)
	}

	videoPath := outFile.Name()
	if !pathWithinDir(videoPath, dir) {
		_ = outFile.Close()
		// #nosec G703 -- path came from os.CreateTemp(dir, ...) and is constrained by pathWithinDir
		_ = os.Remove(videoPath)
		return "", fmt.Errorf("RECORDING_SAVE: Invalid recording path")
	}

	written, err := io.Copy(outFile, videoFile)
	closeErr := outFile.Close()
	if err != nil {
		// #nosec G703 -- path came from os.CreateTemp(dir, ...) and is constrained by pathWithinDir
		_ = os.Remove(videoPath)
		return "", fmt.Errorf("RECORDING_SAVE: Failed to write video. %w", err)
	}
	if closeErr != nil {
		// #nosec G703 -- path came from os.CreateTemp(dir, ...) and is constrained by pathWithinDir
		_ = os.Remove(videoPath)
		return "", fmt.Errorf("RECORDING_SAVE: Failed to finalize video file. %w", closeErr)
	}

	meta.SizeBytes = written

	base := strings.TrimSuffix(filepath.Base(videoPath), ".webm")
	metaPath := filepath.Join(dir, base+"_meta.json")
	if !pathWithinDir(metaPath, dir) {
		return "", fmt.Errorf("RECORDING_SAVE: Invalid metadata path")
	}
	// Error impossible: simple struct with no circular refs or unsupported types
	metaJSON, _ := json.MarshalIndent(*meta, "", "  ")
	// #nosec G306,G703 -- recording metadata path is derived from trusted temp filename in recordings dir
	if err := os.WriteFile(metaPath, metaJSON, 0o600); err != nil {
		return "", fmt.Errorf("RECORDING_SAVE: Failed to write metadata. %w", err)
	}

	return videoPath, nil
}

// HandleSave handles POST /recordings/save from the extension.
// Accepts multipart form with "video" (binary) and "metadata" (JSON string) fields.
func HandleSave(w http.ResponseWriter, r *http.Request, cap interface{ SetQueryResult(string, json.RawMessage) }) {
	if r.Method != "POST" {
		util.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadSizeBytes)

	upload := parseVideoUpload(w, r)
	if upload == nil {
		return
	}
	defer func() {
		_ = upload.videoFile.Close()
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	dir, err := Dir()
	if err != nil {
		util.JSONResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	videoPath, writeErr := writeVideoToDisk(dir, &upload.meta, upload.videoFile)
	if writeErr != nil {
		util.JSONResponse(w, http.StatusInternalServerError, map[string]string{"error": writeErr.Error()})
		return
	}

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
