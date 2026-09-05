// Purpose: Screenshot ingest/rate-limit/file-save handlers for media routes.
// Why: Isolates screenshot upload flow from draw-mode annotation session logic.
// Docs: docs/features/feature/tab-recording/index.md

package mediaapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/pushapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/screenshotframe"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const screenshotMinInterval = time.Second

func ScreenshotsDir() (string, error) {
	dir, err := state.ScreenshotsDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine screenshots directory: %w", err)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("cannot create screenshots directory: %w", err)
	}
	return dir, nil
}

// checkScreenshotRateLimit enforces per-client screenshot rate limiting.
// Returns an HTTP status code (0 means allowed) and an error message.
func (h *Handler) checkScreenshotRateLimit(clientID, queryID string) (int, string) {
	if queryID != "" {
		return 0, ""
	}
	if clientID == "" {
		return 0, ""
	}
	h.rateMu.Lock()
	defer h.rateMu.Unlock()

	lastUpload, exists := h.rateByClient[clientID]
	if exists && time.Since(lastUpload) < screenshotMinInterval {
		return http.StatusTooManyRequests, "Rate limit exceeded: max 1 screenshot per second"
	}
	if len(h.rateByClient) >= 10000 && !exists {
		// Inline eviction: purge stale entries before rejecting.
		for id, ts := range h.rateByClient {
			if time.Since(ts) > screenshotMinInterval {
				delete(h.rateByClient, id)
			}
		}
		if len(h.rateByClient) >= 10000 {
			return http.StatusServiceUnavailable, "Rate limiter capacity exceeded"
		}
	}
	h.rateByClient[clientID] = time.Now()
	return 0, ""
}

// saveImageToScreenshotsDir writes image data to the screenshots directory.
// Returns the full path on success, or an HTTP status and error message on failure.
func saveImageToScreenshotsDir(filename string, imageData []byte) (string, int, string) {
	dir, dirErr := ScreenshotsDir()
	if dirErr != nil {
		return "", http.StatusInternalServerError, "Failed to resolve screenshots directory"
	}
	savePath := filepath.Join(dir, filename)
	if !uploadsec.IsWithinDir(savePath, dir) {
		return "", http.StatusBadRequest, "Invalid screenshot path"
	}
	// #nosec G306 -- path is validated to remain within screenshots dir
	if err := os.WriteFile(savePath, imageData, 0o600); err != nil {
		return "", http.StatusInternalServerError, "Failed to save screenshot"
	}
	return savePath, 0, ""
}

// handleScreenshot saves a screenshot JPEG to disk and returns the filename.
// If query_id is provided, resolves the pending query directly (on-demand screenshot flow).
func (h *Handler) HandleScreenshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		util.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	var body struct {
		DataURL       string `json:"data_url"`
		URL           string `json:"url"`
		CorrelationID string `json:"correlation_id"`
		QueryID       string `json:"query_id"`
		// CoordinateFrame ties the image's pixels to the coordinates click,
		// hover_at and scroll_at accept. Absent when the extension could not
		// measure one, in which case CoordinateFrameError says why.
		CoordinateFrame      *screenshotframe.WireCoordinateFrame `json:"coordinate_frame"`
		CoordinateFrameError string                               `json:"coordinate_frame_error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if status, msg := h.checkScreenshotRateLimit(r.Header.Get("X-Kaboom-Client"), body.QueryID); status != 0 {
		util.JSONResponse(w, status, map[string]string{"error": msg})
		return
	}

	imageData, err := util.DecodeDataURL(body.DataURL)
	if err != nil {
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	filename := util.BuildScreenshotFilename(body.URL, body.CorrelationID)
	savePath, status, saveErr := saveImageToScreenshotsDir(filename, imageData)
	if status != 0 {
		util.JSONResponse(w, status, map[string]string{"error": saveErr})
		return
	}

	result := map[string]string{
		"filename":       filename,
		"path":           savePath,
		"correlation_id": body.CorrelationID,
	}
	if body.QueryID != "" && h.capture != nil {
		// Include data_url in query result so observe(what="screenshot") can return inline image.
		// The HTTP response intentionally omits it to keep the /screenshots response lean.
		queryResult := map[string]any{
			"filename":       filename,
			"path":           savePath,
			"correlation_id": body.CorrelationID,
			"data_url":       body.DataURL,
		}
		addCoordinateFrame(queryResult, body.CoordinateFrame, body.CoordinateFrameError)
		// Error impossible: map contains only primitive types from input
		resultJSON, _ := json.Marshal(queryResult)
		h.capture.Queries().SetQueryResult(body.QueryID, resultJSON)
	}

	// Push screenshot notification to MCP inbox for non-query screenshots
	// (hover launcher, error-triggered). Query screenshots are already delivered via query result.
	if body.QueryID == "" && h.pushRouter != nil {
		b64 := body.DataURL
		if idx := strings.Index(b64, ","); idx >= 0 {
			b64 = b64[idx+1:]
		}
		ev := push.PushEvent{
			ID:            pushapi.EventID("push-ss"),
			Type:          "screenshot",
			Timestamp:     time.Now(),
			PageURL:       body.URL,
			ScreenshotB64: b64,
		}
		_, _ = h.pushRouter.DeliverPush(ev)
	}

	util.JSONResponse(w, http.StatusOK, result)
}

// addCoordinateFrame attaches the frame that makes a screenshot addressable, or the
// reason it is absent.
//
// A frame that fails validation is DROPPED and reported rather than forwarded. It
// would otherwise look exactly like a good one to the caller — a scale of zero is
// still a number — and every coordinate read off the image would land on the same
// point with nothing to say why. Absent-and-explained is a state a caller can
// handle; present-and-wrong is a misclick.
func addCoordinateFrame(result map[string]any, frame *screenshotframe.WireCoordinateFrame, reason string) {
	if frame == nil {
		if reason != "" {
			result["coordinate_frame_error"] = reason
		}
		return
	}
	if err := frame.Validate(); err != nil {
		result["coordinate_frame_error"] = "unusable frame from the extension: " + err.Error()
		return
	}
	result["coordinate_frame"] = frame.WithNote()
}
