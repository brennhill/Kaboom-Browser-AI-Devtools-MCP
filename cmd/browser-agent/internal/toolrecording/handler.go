// handler.go — Recording control, query, playback, and diff MCP handlers.
// Why: These operations share one capture boundary, playback state, and response contract.
// Docs: docs/features/feature/flow-recording/index.md

package toolrecording

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// Capture is the recording-specific subset of capture.Store used by Handler.
type Capture interface {
	StartRecording(name, pageURL string, sensitiveDataEnabled bool) (string, error)
	StopRecording(recordingID string) (int, int64, error)
	ListRecordings(limit int) ([]capture.Recording, error)
	GetRecording(recordingID string) (*capture.Recording, error)
	ExecutePlayback(recordingID string) (*capture.PlaybackSession, error)
	DiffRecordings(originalID, replayID string) (*capture.LogDiffResult, error)
}

// Handler owns the full recording MCP lifecycle and its playback-session state.
type Handler struct {
	capture   Capture
	appendLog func(mcp.LogEntry)

	playbackMu       sync.RWMutex
	playbackSessions map[string]*capture.PlaybackSession
}

// NewHandler constructs a recording handler around its explicit dependencies.
func NewHandler(recordingCapture Capture, appendLog func(mcp.LogEntry)) *Handler {
	return &Handler{
		capture:          recordingCapture,
		appendLog:        appendLog,
		playbackSessions: make(map[string]*capture.PlaybackSession),
	}
}

func (h *Handler) log(entry mcp.LogEntry) {
	if h.appendLog != nil {
		h.appendLog(entry)
	}
}

// EventRecordingStart starts capture and records the lifecycle event.
func (h *Handler) EventRecordingStart(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Name                 string `json:"name"`
		URL                  string `json:"url"`
		SensitiveDataEnabled bool   `json:"sensitive_data_enabled"`
	}
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &params); stop {
			return resp
		}
	}
	if params.URL == "" {
		params.URL = "about:blank"
	}

	recordingID, err := h.capture.StartRecording(params.Name, params.URL, params.SensitiveDataEnabled)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInternal,
			fmt.Sprintf("Failed to start recording: %v", err),
			"Check storage quota and try again")
	}

	h.log(mcp.LogEntry{
		"timestamp":    time.Now().Format(time.RFC3339Nano),
		"level":        "info",
		"message":      fmt.Sprintf("[RECORDING_START] Recording started: %s", recordingID),
		"category":     "RECORDING",
		"recording_id": recordingID,
		"url":          params.URL,
	})
	return mcp.Succeed(req, "Recording started", map[string]any{
		"status":       "ok",
		"recording_id": recordingID,
		"name":         params.Name,
		"url":          params.URL,
		"message":      fmt.Sprintf("Recording started: %s", recordingID),
	})
}

// EventRecordingStop stops capture and records the lifecycle event.
func (h *Handler) EventRecordingStop(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		RecordingID string `json:"recording_id"`
	}
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &params); stop {
			return resp
		}
	}
	if resp, blocked := toolresp.RequireString(req, params.RecordingID, "recording_id", "Provide the recording_id from event_recording_start"); blocked {
		return resp
	}

	actionCount, duration, err := h.capture.StopRecording(params.RecordingID)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInternal,
			fmt.Sprintf("Failed to stop recording: %v", err),
			"No active recording with this ID. Start one first: configure({what: 'event_recording_start', name: 'my-recording'})")
	}

	h.log(mcp.LogEntry{
		"timestamp":    time.Now().Format(time.RFC3339Nano),
		"level":        "info",
		"message":      fmt.Sprintf("[RECORDING_STOP] Recording stopped: %s (%d actions, %dms)", params.RecordingID, actionCount, duration),
		"category":     "RECORDING",
		"recording_id": params.RecordingID,
		"action_count": actionCount,
		"duration_ms":  duration,
	})
	return mcp.Succeed(req, "Recording stopped", map[string]any{
		"status":       "ok",
		"recording_id": params.RecordingID,
		"action_count": actionCount,
		"duration_ms":  duration,
		"message":      fmt.Sprintf("Recording stopped: %d actions captured in %dms", actionCount, duration),
	})
}

// Recordings lists saved recordings.
func (h *Handler) Recordings(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit int `json:"limit"`
	}
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &params); stop {
			return resp
		}
	}
	if params.Limit <= 0 {
		params.Limit = 10
	}

	recordings, err := h.capture.ListRecordings(params.Limit)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInternal,
			fmt.Sprintf("Failed to list recordings: %v", err),
			"Check that recordings directory exists")
	}
	return mcp.Succeed(req, fmt.Sprintf("%d recording(s) found", len(recordings)), map[string]any{
		"recordings": recordings,
		"count":      len(recordings),
		"limit":      params.Limit,
	})
}

// RecordingActions returns the actions and metadata for one recording.
func (h *Handler) RecordingActions(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		RecordingID string `json:"recording_id"`
	}
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &params); stop {
			return resp
		}
	}
	if resp, blocked := toolresp.RequireString(req, params.RecordingID, "recording_id", "Provide the recording_id from a previous event_recording_start call"); blocked {
		return resp
	}

	recording, err := h.capture.GetRecording(params.RecordingID)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInternal,
			fmt.Sprintf("Failed to load recording: %v", err),
			"Ensure the recording_id is correct")
	}
	return mcp.Succeed(req, fmt.Sprintf("%d action(s) from recording %s", len(recording.Actions), params.RecordingID), map[string]any{
		"recording_id": params.RecordingID,
		"name":         recording.Name,
		"created_at":   recording.CreatedAt,
		"start_url":    recording.StartURL,
		"duration_ms":  recording.Duration,
		"action_count": recording.ActionCount,
		"actions":      recording.Actions,
	})
}

// Playback executes a saved recording and stores its result for later observation.
func (h *Handler) Playback(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		RecordingID string `json:"recording_id"`
	}
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &params); stop {
			return resp
		}
	}
	if resp, blocked := toolresp.RequireString(req, params.RecordingID, "recording_id", "Provide a recording_id from a previous recording"); blocked {
		return resp
	}

	session, err := h.capture.ExecutePlayback(params.RecordingID)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInternal,
			fmt.Sprintf("Failed to execute playback: %v", err),
			"Ensure the recording_id is valid")
	}
	h.playbackMu.Lock()
	h.playbackSessions[params.RecordingID] = session
	h.playbackMu.Unlock()

	total := session.ActionsExecuted + session.ActionsFailed
	h.log(mcp.LogEntry{
		"timestamp":        time.Now().Format(time.RFC3339Nano),
		"level":            "info",
		"message":          fmt.Sprintf("[PLAYBACK_COMPLETE] Recording replayed: %d/%d actions succeeded", session.ActionsExecuted, total),
		"category":         "PLAYBACK",
		"recording_id":     params.RecordingID,
		"actions_executed": session.ActionsExecuted,
		"actions_failed":   session.ActionsFailed,
	})
	return BuildPlaybackResult(req, params.RecordingID, session)
}

// PlaybackResults returns the stored execution snapshot for one playback.
func (h *Handler) PlaybackResults(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		RecordingID string `json:"recording_id"`
	}
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &params); stop {
			return resp
		}
	}
	if resp, blocked := toolresp.RequireString(req, params.RecordingID, "recording_id", "Provide the recording_id from playback"); blocked {
		return resp
	}

	h.playbackMu.RLock()
	session, found := h.playbackSessions[params.RecordingID]
	h.playbackMu.RUnlock()
	if !found {
		return mcp.Fail(req, mcp.ErrNoData,
			fmt.Sprintf("No playback results for recording_id %s", params.RecordingID),
			"Run configure(action:'playback', recording_id:'...') first")
	}

	actions := make([]map[string]any, 0, len(session.Results))
	for _, result := range session.Results {
		action := map[string]any{
			"status":           result.Status,
			"action_index":     result.ActionIndex,
			"action_type":      result.ActionType,
			"selector_used":    result.SelectorUsed,
			"duration_ms":      result.DurationMs,
			"error":            result.Error,
			"selector_fragile": result.SelectorFragile,
		}
		if result.Coordinates != nil {
			action["coordinates"] = map[string]any{"x": result.Coordinates.X, "y": result.Coordinates.Y}
		}
		actions = append(actions, action)
	}

	total := session.ActionsExecuted + session.ActionsFailed
	return mcp.Succeed(req, fmt.Sprintf("Playback results: %d/%d actions executed", session.ActionsExecuted, total), map[string]any{
		"recording_id":      params.RecordingID,
		"status":            "ok",
		"actions_executed":  session.ActionsExecuted,
		"actions_failed":    session.ActionsFailed,
		"actions_total":     total,
		"duration_ms":       time.Since(session.StartedAt).Milliseconds(),
		"results":           actions,
		"selector_failures": session.SelectorFailures,
	})
}

// LogDiff compares two recordings and returns summary delta counts.
func (h *Handler) LogDiff(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		OriginalID string `json:"original_id"`
		ReplayID   string `json:"replay_id"`
	}
	if resp := parseDiffParams(req, args, &params.OriginalID, &params.ReplayID); resp != nil {
		return *resp
	}

	result, err := h.capture.DiffRecordings(params.OriginalID, params.ReplayID)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInternal,
			fmt.Sprintf("Failed to diff recordings: %v", err),
			"Ensure both recording IDs are valid")
	}
	h.log(mcp.LogEntry{
		"timestamp":   time.Now().Format(time.RFC3339Nano),
		"level":       "info",
		"message":     fmt.Sprintf("[LOG_DIFF] Comparison complete: %s", result.Summary),
		"category":    "LOG_DIFF",
		"original_id": params.OriginalID,
		"replay_id":   params.ReplayID,
		"status":      result.Status,
	})
	return mcp.Succeed(req, result.Summary, map[string]any{
		"status":         result.Status,
		"summary":        result.Summary,
		"original_id":    params.OriginalID,
		"replay_id":      params.ReplayID,
		"new_errors":     len(result.NewErrors),
		"missing_events": len(result.MissingEvents),
		"changed_values": len(result.ChangedValues),
		"action_stats":   result.ActionStats,
	})
}

// LogDiffReport returns a human-readable regression report for two recordings.
func (h *Handler) LogDiffReport(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		OriginalID string `json:"original_id"`
		ReplayID   string `json:"replay_id"`
	}
	if resp := parseDiffParams(req, args, &params.OriginalID, &params.ReplayID); resp != nil {
		return *resp
	}

	result, err := h.capture.DiffRecordings(params.OriginalID, params.ReplayID)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInternal,
			fmt.Sprintf("Failed to generate report: %v", err),
			"Ensure both recording IDs are valid")
	}
	return mcp.Succeed(req, "Log diff report", map[string]any{
		"status":  result.Status,
		"report":  result.GetRegressionReport(),
		"summary": result.Summary,
		"stats":   result.ActionStats,
	})
}

func parseDiffParams(req mcp.JSONRPCRequest, args json.RawMessage, originalID, replayID *string) *mcp.JSONRPCResponse {
	params := struct {
		OriginalID string `json:"original_id"`
		ReplayID   string `json:"replay_id"`
	}{}
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &params); stop {
			return &resp
		}
	}
	if resp, blocked := toolresp.RequireString(req, params.OriginalID, "original_id", "Provide the original recording ID"); blocked {
		return &resp
	}
	if resp, blocked := toolresp.RequireString(req, params.ReplayID, "replay_id", "Provide the replay recording ID"); blocked {
		return &resp
	}
	*originalID = params.OriginalID
	*replayID = params.ReplayID
	return nil
}
