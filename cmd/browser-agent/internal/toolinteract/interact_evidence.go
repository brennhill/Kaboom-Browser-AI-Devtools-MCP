// interact_evidence.go — The evidence side-channel: before/after screenshot pairs
// captured around DOM-mutating interact actions, keyed by correlation_id.
// Why one file: this was five files (types, env config, capture retry, state store,
// entry points) around a single mutex and a single map. Keeping the mutex, the map,
// and every function that touches them in one file is what makes the locking
// discipline reviewable — the same reason internal/queries was folded back rather
// than exporting its mutexes.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

type evidenceMode string

const (
	evidenceModeOff        evidenceMode = "off"
	evidenceModeOnMutation evidenceMode = "on_mutation"
	evidenceModeAlways     evidenceMode = "always"
)

const (
	evidenceRetryEnv       = "KABOOM_EVIDENCE_RETRY_COUNT"
	evidenceMaxCapturesEnv = "KABOOM_EVIDENCE_MAX_CAPTURES_PER_COMMAND"
)

// EvidenceShot holds a single evidence screenshot result.
type EvidenceShot struct {
	Path     string
	Filename string
	Error    string
	Attempts int
}

type commandEvidenceState struct {
	mode          evidenceMode
	action        string
	shouldCapture bool
	maxCaptures   int
	clientID      string
	skipped       string

	before EvidenceShot
	after  EvidenceShot

	finalized bool
	cached    map[string]any
}

func ParseEvidenceMode(args json.RawMessage) (evidenceMode, error) {
	var params struct {
		Evidence string `json:"evidence"`
	}
	mcp.LenientUnmarshal(args, &params)
	raw := strings.TrimSpace(params.Evidence)
	if raw == "" {
		return evidenceModeOff, nil
	}

	mode := evidenceMode(strings.ToLower(raw))
	switch mode {
	case evidenceModeOff, evidenceModeOnMutation, evidenceModeAlways:
		return mode, nil
	default:
		return evidenceModeOff, fmt.Errorf("interact_evidence: invalid evidence mode %q. Valid modes: off, on_mutation, always", raw)
	}
}

func evidenceMaxCapturesPerCommand() int {
	return parseBoundedEnvInt(evidenceMaxCapturesEnv, 2, 0, 2)
}

func evidenceRetryCount() int {
	return parseBoundedEnvInt(evidenceRetryEnv, 1, 0, 3)
}

func parseBoundedEnvInt(name string, def, min, max int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func canonicalActionFromInteractArgs(args json.RawMessage) string {
	var params struct {
		What   string `json:"what"`
		Action string `json:"action"`
	}
	mcp.LenientUnmarshal(args, &params)
	action := strings.TrimSpace(params.What)
	if action == "" {
		action = strings.TrimSpace(params.Action)
	}
	return strings.ToLower(action)
}

func isMutationAction(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case
		"highlight",
		"execute_js",
		"navigate", "refresh", "back", "forward", "new_tab", "switch_tab", "close_tab", "activate_tab",
		"click", "type", "select", "check", "paste", "key_press",
		"set_attribute", "scroll_to", "focus", "hover",
		"open_composer", "submit_active_composer", "confirm_top_dialog", "dismiss_top_overlay",
		"set_storage", "delete_storage", "clear_storage",
		"set_cookie", "delete_cookie",
		"fill_form", "fill_form_and_submit",
		"upload":
		return true
	default:
		return false
	}
}

const (
	// evidenceScreenshotTimeout is the timeout for creating and waiting for
	// screenshot evidence capture queries.
	evidenceScreenshotTimeout = 12 * time.Second

	// evidenceRetryDelay is the pause between evidence capture retry attempts.
	evidenceRetryDelay = 150 * time.Millisecond
)

// EvidenceCaptureFn is the pluggable evidence capture function.
// Tests can replace it to avoid real screenshot I/O.
var evidenceCaptureFn func(deps *Deps, clientID string) EvidenceShot

// CaptureEvidence captures one screenshot through the canonical query lifecycle.
// It lives with evidence state because its error vocabulary is part of that contract.
func CaptureEvidence(store *capture.Capture, clientID string) EvidenceShot {
	if store == nil {
		return EvidenceShot{Error: "capture_not_initialized"}
	}
	enabled, _, _ := store.GetTrackingStatus()
	if !enabled {
		return EvidenceShot{Error: "no_tracked_tab"}
	}

	queryID, err := store.Queries().CreatePendingQueryWithTimeout(
		queries.PendingQuery{Type: "screenshot", Params: json.RawMessage(`{}`)},
		evidenceScreenshotTimeout,
		clientID,
	)
	if err != nil {
		return EvidenceShot{Error: "queue_full: " + err.Error()}
	}

	raw, err := store.Queries().WaitForResult(queryID, evidenceScreenshotTimeout)
	if err != nil {
		return EvidenceShot{Error: "screenshot_timeout: " + err.Error()}
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return EvidenceShot{Error: "screenshot_parse_error: " + err.Error()}
	}
	if message, ok := payload["error"].(string); ok && strings.TrimSpace(message) != "" {
		return EvidenceShot{Error: strings.TrimSpace(message)}
	}

	path, _ := payload["path"].(string)
	filename, _ := payload["filename"].(string)
	path = strings.TrimSpace(path)
	filename = strings.TrimSpace(filename)
	if path == "" {
		return EvidenceShot{Filename: filename, Error: "screenshot_missing_path"}
	}
	return EvidenceShot{Path: path, Filename: filename}
}

func (h *InteractActionHandler) captureEvidenceWithRetry(clientID string) EvidenceShot {
	retries := evidenceRetryCount()
	attempts := retries + 1
	last := EvidenceShot{Error: "evidence_capture_not_attempted"}

	captureFn := evidenceCaptureFn
	if captureFn == nil && h.deps.DefaultEvidenceCapture != nil {
		captureFn = func(_ *Deps, cid string) EvidenceShot {
			return h.deps.DefaultEvidenceCapture(cid)
		}
	}
	if captureFn == nil {
		return EvidenceShot{Error: "evidence_capture_not_configured"}
	}

	for i := 0; i < attempts; i++ {
		shot := captureFn(h.deps, clientID)
		shot.Attempts = i + 1
		if strings.TrimSpace(shot.Path) != "" {
			return shot
		}
		if strings.TrimSpace(shot.Error) == "" {
			shot.Error = "evidence_capture_failed"
		}
		last = shot
		if i < attempts-1 {
			time.Sleep(evidenceRetryDelay)
		}
	}

	return last
}

// SetEvidenceCaptureFn overrides the evidence capture function (for testing).
func SetEvidenceCaptureFn(fn func(deps *Deps, clientID string) EvidenceShot) {
	evidenceCaptureFn = fn
}

// ResetEvidenceCaptureFn restores the default evidence capture function.
func ResetEvidenceCaptureFn() {
	evidenceCaptureFn = nil
}

func (h *InteractActionHandler) clearEvidenceState(correlationID string) {
	h.evidenceMu.Lock()
	defer h.evidenceMu.Unlock()
	delete(h.evidenceByCommand, correlationID)
}

func (h *InteractActionHandler) storeEvidenceState(correlationID string, state *commandEvidenceState) {
	h.evidenceMu.Lock()
	defer h.evidenceMu.Unlock()
	if h.evidenceByCommand == nil {
		h.evidenceByCommand = make(map[string]*commandEvidenceState)
	}
	h.evidenceByCommand[correlationID] = state
}

func (h *InteractActionHandler) loadEvidenceAttachContext(correlationID string) (cached map[string]any, needsAfter bool, clientID string, done bool) {
	h.evidenceMu.Lock()
	defer h.evidenceMu.Unlock()

	state, ok := h.evidenceByCommand[correlationID]
	if !ok {
		return nil, false, "", true
	}
	if state.finalized {
		return cloneAnyMap(state.cached), false, "", true
	}

	return nil, state.shouldCapture && state.maxCaptures > 1, state.clientID, false
}

func (h *InteractActionHandler) finalizeEvidencePayload(correlationID string, needsAfter bool, after EvidenceShot) (map[string]any, bool) {
	h.evidenceMu.Lock()
	defer h.evidenceMu.Unlock()

	state, ok := h.evidenceByCommand[correlationID]
	if !ok {
		return nil, false
	}
	if !state.finalized {
		if needsAfter {
			state.after = after
		}
		state.cached = buildEvidencePayload(state)
		state.finalized = true
	}

	return cloneAnyMap(state.cached), true
}

func buildEvidencePayload(state *commandEvidenceState) map[string]any {
	if state == nil {
		return map[string]any{}
	}

	payload := map[string]any{
		"mode":   string(state.mode),
		"action": state.action,
	}

	if state.before.Path != "" {
		payload["before"] = state.before.Path
	}
	if state.after.Path != "" {
		payload["after"] = state.after.Path
	}

	files := map[string]any{}
	if state.before.Filename != "" {
		files["before"] = state.before.Filename
	}
	if state.after.Filename != "" {
		files["after"] = state.after.Filename
	}
	if len(files) > 0 {
		payload["filenames"] = files
	}

	errors := map[string]any{}
	if state.before.Error != "" {
		errors["before"] = state.before.Error
	}
	if state.after.Error != "" {
		errors["after"] = state.after.Error
	}
	if len(errors) > 0 {
		payload["errors"] = errors
	}

	if state.skipped != "" {
		payload["skipped"] = state.skipped
	}

	if len(errors) > 0 && (state.before.Path != "" || state.after.Path != "") {
		payload["partial"] = true
	}

	return payload
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if nested, ok := v.(map[string]any); ok {
			out[k] = cloneAnyMap(nested)
			continue
		}
		out[k] = v
	}
	return out
}

func (h *InteractActionHandler) ArmEvidenceForCommand(correlationID, action string, args json.RawMessage, clientID string) {
	if h == nil || correlationID == "" {
		return
	}

	h.armRetryContract(correlationID, action, args)

	mode, err := ParseEvidenceMode(args)
	if err != nil {
		return
	}

	if mode == evidenceModeOff {
		h.clearEvidenceState(correlationID)
		return
	}

	if action == "" {
		action = canonicalActionFromInteractArgs(args)
	}

	maxCaptures := evidenceMaxCapturesPerCommand()
	state := &commandEvidenceState{
		mode:        mode,
		action:      strings.ToLower(strings.TrimSpace(action)),
		maxCaptures: maxCaptures,
		clientID:    clientID,
	}

	switch mode {
	case evidenceModeAlways:
		state.shouldCapture = true
	case evidenceModeOnMutation:
		state.shouldCapture = isMutationAction(state.action)
		if !state.shouldCapture {
			state.skipped = "non_mutating_action"
		}
	}

	if state.shouldCapture && state.maxCaptures <= 0 {
		state.shouldCapture = false
		state.skipped = "capture_budget_zero"
	}

	if state.shouldCapture {
		state.before = h.captureEvidenceWithRetry(clientID)
	}

	h.storeEvidenceState(correlationID, state)
}

func (h *InteractActionHandler) AttachEvidencePayload(correlationID string, responseData map[string]any) {
	if h == nil || correlationID == "" || responseData == nil {
		return
	}

	cached, needsAfter, clientID, done := h.loadEvidenceAttachContext(correlationID)
	if done {
		if cached != nil {
			responseData["evidence"] = cached
		}
		return
	}

	var after EvidenceShot
	if needsAfter {
		after = h.captureEvidenceWithRetry(clientID)
	}

	payload, ok := h.finalizeEvidencePayload(correlationID, needsAfter, after)
	if !ok {
		return
	}

	responseData["evidence"] = payload
}
