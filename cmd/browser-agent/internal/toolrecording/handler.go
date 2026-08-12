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
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording/logdiff"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording/playback"
	core "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/core"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// Store is the recording lifecycle boundary used by Handler.
type Store interface {
	StartRecording(name, pageURL string, sensitiveDataEnabled bool) (string, error)
	StopRecording(recordingID string) (int, int64, error)
	ActiveRecordingID() string
	ListRecordings(limit int) ([]recording.Recording, error)
	GetRecording(recordingID string) (*recording.Recording, error)
	LookupRecording(recordingID string) (*recording.Recording, error)
}

// Handler owns the full recording MCP lifecycle and its playback-session state.
type Handler struct {
	recordings Store
	appendLog  func(types.LogEntry)

	playbackMu       sync.RWMutex
	playbackSessions map[string]*playback.Session
}

// NewHandler constructs a recording handler around its explicit dependencies.
func NewHandler(recordings Store, appendLog func(types.LogEntry)) *Handler {
	return &Handler{
		recordings:       recordings,
		appendLog:        appendLog,
		playbackSessions: make(map[string]*playback.Session),
	}
}

func (h *Handler) log(entry types.LogEntry) {
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

	recordingID, err := h.recordings.StartRecording(params.Name, params.URL, params.SensitiveDataEnabled)
	if err != nil {
		// An already-running recording is an expected condition with an obvious
		// remedy, not an internal failure. It used to return ErrInternal with
		// "Check storage quota and try again", so a caller acting on the code and
		// the playbook went looking for disk space while the real fix — stopping
		// the active recording — went unmentioned, and the id it needed appeared
		// nowhere but this message.
		if recording.IsAlreadyRecording(err) {
			return mcp.Fail(req, mcp.ErrAlreadyRecording,
				fmt.Sprintf("Failed to start recording: %v", err),
				"Stop the active recording first: configure({what: 'event_recording_stop'}). "+
					"Its id is also reported by observe({what: 'recordings'}) as active_recording_id.")
		}
		return mcp.Fail(req, mcp.ErrInternal,
			fmt.Sprintf("Failed to start recording: %v", err),
			"Check storage quota and try again")
	}

	h.log(types.LogEntry{
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
	// recording_id is optional: omitting it stops whichever recording is active.
	// Requiring it made a lost id unrecoverable, because the active recording is
	// not listed by observe(recordings) and start refuses while one is running.
	var params recordingIDParams
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &params); stop {
			return resp
		}
	}
	recordingID := params.RecordingID

	actionCount, duration, err := h.recordings.StopRecording(recordingID)
	if err != nil {
		if recording.IsNoActiveRecording(err) {
			return mcp.Fail(req, mcp.ErrNoData,
				fmt.Sprintf("Failed to stop recording: %v", err),
				"Nothing is recording. Start one first: configure({what: 'event_recording_start', name: 'my-recording'})")
		}
		return mcp.Fail(req, mcp.ErrInternal,
			fmt.Sprintf("Failed to stop recording: %v", err),
			"No active recording with this ID. Start one first: configure({what: 'event_recording_start', name: 'my-recording'})")
	}
	if recordingID == "" {
		recordingID = "(active)"
	}

	h.log(types.LogEntry{
		"timestamp":    time.Now().Format(time.RFC3339Nano),
		"level":        "info",
		"message":      fmt.Sprintf("[RECORDING_STOP] Recording stopped: %s (%d actions, %dms)", recordingID, actionCount, duration),
		"category":     "RECORDING",
		"recording_id": recordingID,
		"action_count": actionCount,
		"duration_ms":  duration,
	})
	return mcp.Succeed(req, "Recording stopped", map[string]any{
		"status":       "ok",
		"recording_id": recordingID,
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
	// core.ClampLimit applies the ceiling the observe schema has always
	// documented ("max 1000"). Without it, limit=100000 built a 2.9MB response
	// from 4761 recordings that the size clamp then cut back — dropping the
	// recordings array entirely, since it is the last key, and answering a
	// request for more data with none.
	params.Limit = core.ClampLimit(params.Limit, 10)

	recordings, err := h.recordings.ListRecordings(params.Limit)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInternal,
			fmt.Sprintf("Failed to list recordings: %v", err),
			"Check that recordings directory exists")
	}
	// ListRecordings returns completed sessions only, so without this the running
	// recording is invisible everywhere — and stopping it needs an id that only
	// start ever returned.
	active := h.recordings.ActiveRecordingID()
	summary := fmt.Sprintf("%d recording(s) found", len(recordings))
	if active != "" {
		summary += fmt.Sprintf("; recording in progress: %s", active)
	}
	// Project to listing entries: an entry identifies a recording, it does not
	// replay it. See recording.RecordingSummary for the contract.
	entries, sizeTruncated := boundSummariesToBudget(recordings)
	payload := map[string]any{
		"recordings": entries,
		// count describes what is in THIS response; total is what exists. They
		// used to be the same number even after the backstop cut the payload,
		// so a caller was told it had 1000 entries while holding about 100.
		"count":               len(entries),
		"total":               len(recordings),
		"limit":               params.Limit,
		"active_recording_id": active,
	}
	if sizeTruncated {
		payload["truncated"] = true
		payload["hint"] = "Showing the newest recordings that fit the response budget. Narrow with limit, or fetch one with recording_actions."
	}
	return mcp.Succeed(req, summary, payload)
}

// recordingsByteBudget leaves room under the MCP response backstop for the
// envelope, the summary line and the surrounding fields.
//
// The mode bounds its own response because the backstop cannot: it keeps
// whichever bytes came first, which cuts an entry in half and, when the
// recordings array is the last key, drops it entirely.
const recordingsByteBudget = mcp.MaxResponseBytes / 2

// boundSummariesToBudget returns as many newest-first summaries as fit the
// byte budget, and whether any were withheld.
func boundSummariesToBudget(recordings []recording.Recording) ([]recording.RecordingSummary, bool) {
	entries := make([]recording.RecordingSummary, 0, len(recordings))
	used := 0
	for _, rec := range recordings {
		summary := rec.Summary()
		encoded, err := json.Marshal(summary)
		if err != nil {
			// An entry that cannot be encoded is skipped rather than failing the
			// listing; the caller still gets every other recording.
			continue
		}
		if used+len(encoded) > recordingsByteBudget && len(entries) > 0 {
			return entries, true
		}
		used += len(encoded)
		entries = append(entries, summary)
	}
	return entries, false
}

// RecordingActions returns the actions and metadata for one recording.
func (h *Handler) RecordingActions(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	recordingID, resp := parseRecordingID(req, args, "Provide the recording_id from a previous event_recording_start call")
	if resp != nil {
		return *resp
	}

	recording, err := h.recordings.GetRecording(recordingID)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInternal,
			fmt.Sprintf("Failed to load recording: %v", err),
			"Ensure the recording_id is correct")
	}
	return mcp.Succeed(req, fmt.Sprintf("%d action(s) from recording %s", len(recording.Actions), recordingID), map[string]any{
		"recording_id": recordingID,
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
	recordingID, resp := parseRecordingID(req, args, "Provide a recording_id from a previous recording")
	if resp != nil {
		return *resp
	}

	session, err := playback.Execute(h.recordings, recordingID)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInternal,
			fmt.Sprintf("Failed to execute playback: %v", err),
			"Ensure the recording_id is valid")
	}
	h.playbackMu.Lock()
	h.playbackSessions[recordingID] = session
	h.playbackMu.Unlock()

	total := session.ActionsExecuted + session.ActionsFailed
	h.log(types.LogEntry{
		"timestamp":        time.Now().Format(time.RFC3339Nano),
		"level":            "info",
		"message":          fmt.Sprintf("[PLAYBACK_COMPLETE] Recording replayed: %d/%d actions succeeded", session.ActionsExecuted, total),
		"category":         "PLAYBACK",
		"recording_id":     recordingID,
		"actions_executed": session.ActionsExecuted,
		"actions_failed":   session.ActionsFailed,
	})
	return BuildPlaybackResult(req, recordingID, session)
}

// PlaybackResults returns the stored execution snapshot for one playback.
func (h *Handler) PlaybackResults(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	recordingID, resp := parseRecordingID(req, args, "Provide the recording_id from playback")
	if resp != nil {
		return *resp
	}

	h.playbackMu.RLock()
	session, found := h.playbackSessions[recordingID]
	h.playbackMu.RUnlock()
	if !found {
		return mcp.Fail(req, mcp.ErrNoData,
			fmt.Sprintf("No playback results for recording_id %s", recordingID),
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
		"recording_id":      recordingID,
		"status":            "ok",
		"actions_executed":  session.ActionsExecuted,
		"actions_failed":    session.ActionsFailed,
		"actions_total":     total,
		"duration_ms":       time.Since(session.StartedAt).Milliseconds(),
		"results":           actions,
		"selector_failures": session.SelectorFailures,
	})
}

type recordingIDParams struct {
	RecordingID string `json:"recording_id"`
}

func parseRecordingID(req mcp.JSONRPCRequest, args json.RawMessage, hint string) (string, *mcp.JSONRPCResponse) {
	var params recordingIDParams
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &params); stop {
			return "", &resp
		}
	}
	if resp, blocked := toolresp.RequireString(req, params.RecordingID, "recording_id", hint); blocked {
		return "", &resp
	}
	return params.RecordingID, nil
}

// LogDiff compares two recordings and returns summary delta counts.
func (h *Handler) LogDiff(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	params, result, resp := h.compareRecordings(req, args, "Failed to diff recordings")
	if resp != nil {
		return *resp
	}

	h.log(types.LogEntry{
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
	_, result, resp := h.compareRecordings(req, args, "Failed to generate report")
	if resp != nil {
		return *resp
	}

	return mcp.Succeed(req, "Log diff report", map[string]any{
		"status":  result.Status,
		"report":  result.GetRegressionReport(),
		"summary": result.Summary,
		"stats":   result.ActionStats,
	})
}

type diffParams struct {
	OriginalID string `json:"original_id"`
	ReplayID   string `json:"replay_id"`
}

func (h *Handler) compareRecordings(
	req mcp.JSONRPCRequest,
	args json.RawMessage,
	failureSummary string,
) (diffParams, *logdiff.Result, *mcp.JSONRPCResponse) {
	params, resp := parseDiffParams(req, args)
	if resp != nil {
		return diffParams{}, nil, resp
	}
	result, err := logdiff.Compare(h.recordings, params.OriginalID, params.ReplayID)
	if err != nil {
		failure := mcp.Fail(req, mcp.ErrInternal,
			fmt.Sprintf("%s: %v", failureSummary, err),
			"Ensure both recording IDs are valid")
		return diffParams{}, nil, &failure
	}
	return params, result, nil
}

func parseDiffParams(req mcp.JSONRPCRequest, args json.RawMessage) (diffParams, *mcp.JSONRPCResponse) {
	var params diffParams
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &params); stop {
			return diffParams{}, &resp
		}
	}
	if resp, blocked := toolresp.RequireString(req, params.OriginalID, "original_id", "Provide the original recording ID"); blocked {
		return diffParams{}, &resp
	}
	if resp, blocked := toolresp.RequireString(req, params.ReplayID, "replay_id", "Provide the replay recording ID"); blocked {
		return diffParams{}, &resp
	}
	return params, nil
}
