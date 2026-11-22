// Purpose: Implements screenshot ingestion, decoding, naming, and persistence helpers.
// Why: Keeps media-specific HTTP handling separate from core log-store lifecycle behavior.
// Docs: docs/features/feature/tab-recording/index.md

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/state/paths"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/util"
)

const maxPostBodySize = 10 * 1024 * 1024 // 10MB

// screenshotRequest holds the parsed screenshot request body.
type screenshotRequest struct {
	DataURL       string `json:"data_url"`
	URL           string `json:"url"`
	CorrelationID string `json:"correlation_id"`
}

// saveScreenshotFile writes image data to the screenshots directory and returns the full path.
func saveScreenshotFile(filename string, imageData []byte) (string, error) {
	dir, err := state.ScreenshotsDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve screenshots directory: %w", err)
	}
	// #nosec G301 -- 0o755 is appropriate for screenshots directory
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create screenshots directory: %w", err)
	}
	savePath := filepath.Join(dir, filename)
	// #nosec G306 -- screenshots are intentionally world-readable
	if err := os.WriteFile(savePath, imageData, 0o644); err != nil {
		return "", fmt.Errorf("failed to save screenshot: %w", err)
	}
	return savePath, nil
}

// handleScreenshot saves a screenshot JPEG to disk and returns the filename.
func (s *Server) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	var body screenshotRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if body.DataURL == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing dataUrl"})
		return
	}

	imageData, err := util.DecodeDataURL(body.DataURL)
	if err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid base64 data"})
		return
	}

	filename := util.BuildScreenshotFilename(body.URL, body.CorrelationID)
	savePath, err := saveScreenshotFile(filename, imageData)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"filename":       filename,
		"path":           savePath,
		"correlation_id": body.CorrelationID,
	})
}
