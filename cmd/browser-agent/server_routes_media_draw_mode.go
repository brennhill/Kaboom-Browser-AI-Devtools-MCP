// Purpose: Draw-mode completion ingest and annotation session persistence handlers.
// Why: Isolates annotation/draw session parsing and persistence from screenshot upload logic.
// Docs: docs/features/feature/tab-recording/index.md

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// saveDrawScreenshot decodes a data URL and writes the screenshot to disk.
// Returns the saved path (empty string on any failure, with a non-nil error
// only for directory resolution failures that should abort the request).
func saveDrawScreenshot(dataURL string, tabID int) (string, error) {
	imageData, err := util.DecodeDataURL(dataURL)
	if err != nil {
		return "", nil
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("draw_%s_tab%d_%d.png", util.SanitizeForFilename(timestamp), tabID, toolresp.RandomInt63()%10000)

	dir, dirErr := screenshotsDir()
	if dirErr != nil {
		return "", dirErr
	}
	path := filepath.Join(dir, filename)
	if !uploadsec.IsWithinDir(path, dir) {
		return "", nil
	}
	// #nosec G306 -- path is validated to remain within screenshots dir
	if err := os.WriteFile(path, imageData, 0o600); err != nil {
		return "", nil
	}
	return path, nil
}

// parseAnnotations unmarshals raw annotation JSON, collecting warnings for
// entries that fail to parse.
func parseAnnotations(rawAnnotations []json.RawMessage) ([]annotation.Annotation, []string) {
	parsed := make([]annotation.Annotation, 0, len(rawAnnotations))
	var warnings []string
	for i, raw := range rawAnnotations {
		var ann annotation.Annotation
		if err := json.Unmarshal(raw, &ann); err != nil {
			warnings = append(warnings, fmt.Sprintf("annotation[%d]: %v", i, err))
		} else {
			parsed = append(parsed, ann)
		}
	}
	return parsed, warnings
}

// storeElementDetails persists annotation element details into the provided store.
func storeElementDetails(store *annotation.Store, details map[string]json.RawMessage) {
	if store == nil {
		return
	}
	for correlationID, rawDetail := range details {
		var detail annotation.Detail
		if err := json.Unmarshal(rawDetail, &detail); err == nil {
			if detail.Selector == "" && detail.Tag == "" {
				rawStr := string(rawDetail)
				if len(rawStr) > 200 {
					rawStr = rawStr[:200] + "..."
				}
				diag.Printf("[Kaboom] draw detail %s: empty (raw=%s)\n", correlationID, rawStr)
			}
			detail.CorrelationID = correlationID
			store.StoreDetail(correlationID, detail)
		} else {
			rawStr := string(rawDetail)
			if len(rawStr) > 200 {
				rawStr = rawStr[:200] + "..."
			}
			diag.Printf("[Kaboom] draw detail %s: unmarshal error: %v (raw=%s)\n", correlationID, err, rawStr)
		}
	}
}

// drawModeRequest holds the parsed and validated fields from a draw mode POST body.
type drawModeRequest struct {
	ScreenshotDataURL string                     `json:"screenshot_data_url"`
	Annotations       []json.RawMessage          `json:"annotations"`
	ElementDetails    map[string]json.RawMessage `json:"element_details"`
	PageURL           string                     `json:"page_url"`
	TabID             int                        `json:"tab_id"`
	AnnotSessionName  string                     `json:"annot_session_name"`
	CorrelationID     string                     `json:"correlation_id"`
}

// persistDrawSession writes the full draw session (annotations + element details) to disk
// as a JSON file alongside the screenshot. Files are retained until manually cleared.
func persistDrawSession(body *drawModeRequest, screenshotPath string, annotations []annotation.Annotation) {
	dir, err := screenshotsDir()
	if err != nil {
		return
	}
	ts := time.Now().UnixMilli()
	filename := fmt.Sprintf("draw-session-%d-%d.json", body.TabID, ts)
	path := filepath.Join(dir, filename)

	session := map[string]any{
		"annotations":     annotations,
		"element_details": body.ElementDetails,
		"page_url":        body.PageURL,
		"tab_id":          body.TabID,
		"screenshot":      screenshotPath,
		"timestamp":       ts,
		"correlation_id":  body.CorrelationID,
	}
	if body.AnnotSessionName != "" {
		session["annot_session_name"] = body.AnnotSessionName
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return
	}
	// #nosec G306 -- path is validated to remain within screenshots dir
	_ = os.WriteFile(path, data, 0o600)
}

func storeAnnotationSessionInStore(store *annotation.Store, body *drawModeRequest, screenshotPath string, annotations []annotation.Annotation) {
	if store == nil || body == nil {
		return
	}
	session := &annotation.Session{
		Annotations:    annotations,
		ScreenshotPath: screenshotPath,
		PageURL:        body.PageURL,
		TabID:          body.TabID,
		Timestamp:      time.Now().UnixMilli(),
	}
	store.StoreSession(body.TabID, session)
	if body.AnnotSessionName != "" {
		store.AppendToNamedSession(body.AnnotSessionName, session)
	}
}

// handleDrawModeComplete receives annotation data and screenshot from the extension
// when the user finishes a draw mode session.
func (s *Server) handleDrawModeComplete(w http.ResponseWriter, r *http.Request, cap *capture.Store) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	var body drawModeRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if body.TabID <= 0 {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "tab_id is required and must be > 0"})
		return
	}

	var screenshotPath string
	if body.ScreenshotDataURL != "" {
		path, err := saveDrawScreenshot(body.ScreenshotDataURL, body.TabID)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to resolve screenshots directory"})
			return
		}
		screenshotPath = path
	}

	parsedAnnotations, parseWarnings := parseAnnotations(body.Annotations)
	store := s.getAnnotationStore()
	// T6 fix: store element details BEFORE the session. StoreSession triggers
	// waiter completion which signals the AI agent; if the agent immediately
	// calls annotation_detail the details must already be present.
	storeElementDetails(store, body.ElementDetails)
	storeAnnotationSessionInStore(store, &body, screenshotPath, parsedAnnotations)

	// Persist full session to disk so the LLM can compare/contrast across restarts
	persistDrawSession(&body, screenshotPath, parsedAnnotations)

	result := map[string]any{
		"status":           "stored",
		"annotation_count": len(parsedAnnotations),
		"screenshot":       screenshotPath,
	}
	if len(parseWarnings) > 0 {
		result["warnings"] = parseWarnings
	}

	// Complete the pending command — unblocks WaitForCommand in tools_async_completion.go.
	// so the LLM can retrieve results via correlation_id.
	if body.CorrelationID != "" && cap != nil {
		resultJSON, _ := json.Marshal(result)
		cap.CompleteCommand(body.CorrelationID, resultJSON, "")
	}

	// Auto-push annotations to AI client via push pipeline
	s.pushDrawModeCompletion(&body, screenshotPath, parsedAnnotations)

	jsonResponse(w, http.StatusOK, result)
}
