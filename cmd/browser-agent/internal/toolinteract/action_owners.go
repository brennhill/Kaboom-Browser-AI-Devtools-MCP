// action_owners.go — Shared action owners, batch execution, and evidence lifecycle.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/replay"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract/elemindex"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RuntimeDeps contains only the command lifecycle capabilities shared by all families.
type RuntimeDeps struct {
	RequireCSPClear        func(mcp.JSONRPCRequest, string) (mcp.JSONRPCResponse, bool)
	EnqueuePendingQuery    func(mcp.JSONRPCRequest, queries.PendingQuery, time.Duration) (mcp.JSONRPCResponse, bool)
	MaybeWaitForCommand    func(mcp.JSONRPCRequest, string, json.RawMessage, string) mcp.JSONRPCResponse
	RecordAIAction         func(string, string, map[string]any)
	DefaultEvidenceCapture func(string) EvidenceShot
}

type DOMDeps struct {
	RequirePilot, RequireExtension, RequireTabTracking toolguard.Check
	RecordDOMPrimitiveAction                           func(string, string, string, string)
}

type BrowserDeps struct {
	RequirePilot, RequireExtension, RequireTabTracking toolguard.Check
	Capture                                            func() *capture.Capture
	InjectCSPBlockedActions                            func(mcp.JSONRPCResponse) mcp.JSONRPCResponse
	GetListenPort                                      func() int
}

type PageDeps struct {
	RequirePilot, RequireExtension, RequireTabTracking toolguard.Check
	Capture                                            func() *capture.Capture
	EnqueuePendingQuery                                func(mcp.JSONRPCRequest, queries.PendingQuery, time.Duration) (mcp.JSONRPCResponse, bool)
	RecordAIAction                                     func(string, string, map[string]any)
	MarkDrawStarted                                    func()
	GetScreenshot                                      func(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
	GetPageInfo                                        func(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
}

type WorkflowDeps struct {
	Capture                      func() *capture.Capture
	ToolAnalyze, ToolExportSARIF func(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
	Now                          func() time.Time
}

type StorageDeps struct {
	RequirePilot, RequireExtension, RequireTabTracking toolguard.Check
}

type BatchDeps struct {
	RequirePilot, RequireExtension toolguard.Check
	Capture                        func() *capture.Capture
	RecordAIAction                 func(string, string, map[string]any)
	ToolInteract                   func(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
	ReplayMu                       *sync.Mutex
}

type DOMActions struct {
	runtime              *ActionRuntime
	deps                 DOMDeps
	elementIndexRegistry *elemindex.Registry
}

func NewDOMActions(runtime *ActionRuntime, deps DOMDeps) *DOMActions {
	return &DOMActions{runtime: runtime, deps: deps, elementIndexRegistry: elemindex.New()}
}

type BrowserActions struct {
	runtime *ActionRuntime
	page    *PageActions
	deps    BrowserDeps
}

func NewBrowserActions(runtime *ActionRuntime, page *PageActions, deps BrowserDeps) *BrowserActions {
	return &BrowserActions{runtime: runtime, page: page, deps: deps}
}

type PageActions struct {
	runtime *ActionRuntime
	dom     *DOMActions
	storage *StorageActions
	deps    PageDeps
}

func NewPageActions(runtime *ActionRuntime, dom *DOMActions, storage *StorageActions, deps PageDeps) *PageActions {
	return &PageActions{runtime: runtime, dom: dom, storage: storage, deps: deps}
}

type WorkflowActions struct {
	runtime *ActionRuntime
	dom     *DOMActions
	browser *BrowserActions
	page    *PageActions
	deps    WorkflowDeps
}

func NewWorkflowActions(runtime *ActionRuntime, dom *DOMActions, browser *BrowserActions, page *PageActions, deps WorkflowDeps) *WorkflowActions {
	return &WorkflowActions{runtime: runtime, dom: dom, browser: browser, page: page, deps: deps}
}

type StorageActions struct {
	runtime *ActionRuntime
	deps    StorageDeps
}

func NewStorageActions(runtime *ActionRuntime, deps StorageDeps) *StorageActions {
	return &StorageActions{runtime: runtime, deps: deps}
}

type BatchActions struct {
	runtime *ActionRuntime
	deps    BatchDeps
}

func NewBatchActions(runtime *ActionRuntime, deps BatchDeps) *BatchActions {
	return &BatchActions{runtime: runtime, deps: deps}
}

// Deps holds all external dependencies interact handlers need from the caller.
func marshalQueryParams(fields map[string]any) json.RawMessage {
	return mcp.SafeMarshal(fields, "{}")
}

func checkGuards(req mcp.JSONRPCRequest, guards ...toolguard.Check) (mcp.JSONRPCResponse, bool) {
	for _, guard := range guards {
		if resp, blocked := guard(req); blocked {
			return resp, true
		}
	}
	return mcp.JSONRPCResponse{}, false
}

func checkGuardsWithOpts(req mcp.JSONRPCRequest, opts []func(*mcp.StructuredError), guards ...toolguard.Check) (mcp.JSONRPCResponse, bool) {
	for _, guard := range guards {
		if resp, blocked := guard(req, opts...); blocked {
			return resp, true
		}
	}
	return mcp.JSONRPCResponse{}, false
}

const (
	maxSequenceSteps   = replay.MaxSteps
	defaultStepTimeout = replay.DefaultStepTimeout
)

// ReplayMu prevents concurrent batch/replay execution.
var ReplayMu sync.Mutex

// handleBatch executes a sequence of interact steps provided inline.
func (h *BatchActions) HandleBatch(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	// Fail fast if pilot/extension are not available — avoids acquiring replayMu
	// and iterating steps that would all fail individually (#9.R3.9).
	if resp, blocked := checkGuards(req, h.deps.RequirePilot, h.deps.RequireExtension); blocked {
		return resp
	}

	var params struct {
		Steps         []json.RawMessage `json:"steps"`
		StepTimeoutMs int               `json:"step_timeout_ms"`
		ContinueOnErr *bool             `json:"continue_on_error"`
		StopAfterStep int               `json:"stop_after_step"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	// Validate steps
	if len(params.Steps) == 0 {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Steps must be a non-empty array", "Add at least one step", mcp.WithParam("steps"))
	}
	if len(params.Steps) > maxSequenceSteps {
		return mcp.Fail(req, mcp.ErrInvalidParam, fmt.Sprintf("Steps exceeds maximum of %d", maxSequenceSteps), "Split into smaller batches", mcp.WithParam("steps"))
	}

	// Validate each step has a what (or action) field
	for i, step := range params.Steps {
		var s struct {
			What   string `json:"what"`
			Action string `json:"action"`
		}
		if err := json.Unmarshal(step, &s); err != nil || (s.What == "" && s.Action == "") {
			return mcp.Fail(req, mcp.ErrInvalidParam, fmt.Sprintf("Step[%d] missing required 'what' field", i), "Add a 'what' field to each step", mcp.WithParam("steps"))
		}
	}

	// Default continue_on_error to true
	continueOnError := true
	if params.ContinueOnErr != nil {
		continueOnError = *params.ContinueOnErr
	}

	if params.StepTimeoutMs <= 0 {
		params.StepTimeoutMs = defaultStepTimeout
	}

	// Acquire replay mutex (prevent concurrent batch/replay)
	mu := h.deps.ReplayMu
	if mu == nil {
		mu = &ReplayMu
	}
	if !mu.TryLock() {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Another batch or sequence is currently executing", "Wait for it to complete")
	}
	defer mu.Unlock()

	// Record audit trail
	h.deps.RecordAIAction("batch", "", map[string]any{"steps": len(params.Steps)})

	start := time.Now()
	results := make([]replay.StepResult, 0, len(params.Steps))
	stepsExecuted := 0
	stepsFailed := 0
	stepsQueued := 0
	maxSteps := len(params.Steps)
	if params.StopAfterStep > 0 && params.StopAfterStep < maxSteps {
		maxSteps = params.StopAfterStep
	}

	for i := 0; i < maxSteps; i++ {
		stepArgs := params.Steps[i]

		// Extract action name for result
		var stepAction struct {
			What   string `json:"what"`
			Action string `json:"action"`
		}
		json.Unmarshal(stepArgs, &stepAction) //nolint:errcheck // best-effort extraction
		actionName := stepAction.What
		if actionName == "" {
			actionName = stepAction.Action
		}

		// Strip include_screenshot from batch steps — screenshots are captured per-step
		// but then discarded in the aggregate response, wasting CPU on base64 encoding (#9.2.2).
		stepArgs = StripComposableScreenshotFromStep(stepArgs)

		replayStepArgs := replay.ForceAsync(stepArgs)
		stepStart := time.Now()
		stepResp := h.deps.ToolInteract(req, replayStepArgs)
		stepDuration := time.Since(stepStart).Milliseconds()

		stepResult := replay.StepResult{
			StepIndex:  i,
			Action:     actionName,
			DurationMs: stepDuration,
		}

		if corrID := replay.CorrelationID(stepResp); corrID != "" {
			stepResult.CorrelationID = corrID
			timeout := time.Duration(params.StepTimeoutMs) * time.Millisecond
			if timeout > 0 {
				cmd, found := h.deps.Capture().Queries().WaitForCommand(corrID, timeout)
				if found {
					switch cmd.Status {
					case "pending":
						stepResult.Status = "queued"
						stepsQueued++
					case "complete":
						stepResult.Status = "ok"
					default:
						stepResult.Status = "error"
						if cmd.Error != "" {
							stepResult.Error = cmd.Error
						} else {
							stepResult.Error = "command failed with status " + cmd.Status
						}
						stepsFailed++
					}
				} else {
					stepResult.Status = "queued"
					stepsQueued++
				}
			}
		}

		if act.IsErrorResponse(stepResp) {
			// Only count as failed if not already counted by the correlation path above (#9.R1).
			// In the contradictory case where correlation resolved to "ok" but isErrorResponse
			// is true, we trust the correlation result since it reflects the actual extension-side
			// outcome. This cannot happen in practice because replay.ForceAsync generates
			// a fresh correlation ID per step.
			if stepResult.Status == "" {
				stepResult.Status = "error"
				stepResult.Error = replay.ErrorMessage(stepResp)
				stepsFailed++
			}
			results = append(results, stepResult)
			stepsExecuted++
			if !continueOnError {
				break
			}
			continue
		}

		if stepResult.Status == "" {
			stepResult.Status = "ok"
		}
		stepsExecuted++
		results = append(results, stepResult)
	}

	totalDuration := time.Since(start).Milliseconds()

	status := "ok"
	var message string
	stepsTotal := len(params.Steps)
	if stepsFailed > 0 && stepsExecuted < stepsTotal {
		// Stopped early (continue_on_error=false)
		status = "error"
		message = fmt.Sprintf("Batch failed at step %d/%d", stepsExecuted, stepsTotal)
	} else if stepsFailed > 0 && stepsFailed == stepsExecuted {
		// All executed steps failed
		status = "error"
		message = fmt.Sprintf("Batch failed: all %d executed steps had errors", stepsFailed)
	} else if stepsQueued > 0 && stepsFailed > 0 {
		status = "partial"
		message = fmt.Sprintf("Batch executed with failures: %d queued, %d failed", stepsQueued, stepsFailed)
	} else if stepsQueued > 0 {
		status = "queued"
		message = fmt.Sprintf("Batch queued: %d/%d steps still running", stepsQueued, stepsTotal)
	} else if stepsFailed > 0 {
		status = "partial"
		message = fmt.Sprintf("Batch partially executed: %d/%d steps succeeded, %d failed", stepsExecuted-stepsFailed, stepsTotal, stepsFailed)
	} else {
		message = fmt.Sprintf("Batch executed: %d/%d steps in %dms", stepsExecuted, stepsTotal, totalDuration)
	}

	responseData := map[string]any{
		"status":         status,
		"steps_executed": stepsExecuted,
		"steps_failed":   stepsFailed,
		"steps_queued":   stepsQueued,
		"steps_total":    stepsTotal,
		"duration_ms":    totalDuration,
		"results":        results,
		"message":        message,
	}

	return mcp.Succeed(req, "Batch execution", responseData)
}

// StripComposableScreenshotFromStep removes include_screenshot from batch step args
// to prevent wasted screenshot captures that are discarded in the aggregate response.
func StripComposableScreenshotFromStep(stepArgs json.RawMessage) json.RawMessage {
	var raw map[string]any
	if err := json.Unmarshal(stepArgs, &raw); err != nil {
		return stepArgs
	}
	if _, has := raw["include_screenshot"]; has {
		delete(raw, "include_screenshot")
		patched, err := json.Marshal(raw)
		if err != nil {
			return stepArgs
		}
		return patched
	}
	return stepArgs
}

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
var evidenceCaptureFn func(clientID string) EvidenceShot

// CaptureEvidence captures one screenshot through the canonical query lifecycle.
// It lives with evidence state because its error vocabulary is part of that contract.
func CaptureEvidence(store *capture.Capture, clientID string) EvidenceShot {
	if store == nil {
		return EvidenceShot{Error: "capture_not_initialized"}
	}
	enabled, _, _ := store.Extension().GetTrackingStatus()
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

func (h *ActionRuntime) captureEvidenceWithRetry(clientID string) EvidenceShot {
	retries := evidenceRetryCount()
	attempts := retries + 1
	last := EvidenceShot{Error: "evidence_capture_not_attempted"}

	captureFn := evidenceCaptureFn
	if captureFn == nil && h.deps.DefaultEvidenceCapture != nil {
		captureFn = func(cid string) EvidenceShot {
			return h.deps.DefaultEvidenceCapture(cid)
		}
	}
	if captureFn == nil {
		return EvidenceShot{Error: "evidence_capture_not_configured"}
	}

	for i := 0; i < attempts; i++ {
		shot := captureFn(clientID)
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
func SetEvidenceCaptureFn(fn func(clientID string) EvidenceShot) {
	evidenceCaptureFn = fn
}

// ResetEvidenceCaptureFn restores the default evidence capture function.
func ResetEvidenceCaptureFn() {
	evidenceCaptureFn = nil
}

func (h *ActionRuntime) clearEvidenceState(correlationID string) {
	h.evidenceMu.Lock()
	defer h.evidenceMu.Unlock()
	delete(h.evidenceByCommand, correlationID)
}

func (h *ActionRuntime) storeEvidenceState(correlationID string, state *commandEvidenceState) {
	h.evidenceMu.Lock()
	defer h.evidenceMu.Unlock()
	if h.evidenceByCommand == nil {
		h.evidenceByCommand = make(map[string]*commandEvidenceState)
	}
	h.evidenceByCommand[correlationID] = state
}

func (h *ActionRuntime) loadEvidenceAttachContext(correlationID string) (cached map[string]any, needsAfter bool, clientID string, done bool) {
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

func (h *ActionRuntime) finalizeEvidencePayload(correlationID string, needsAfter bool, after EvidenceShot) (map[string]any, bool) {
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

func (h *ActionRuntime) ArmEvidenceForCommand(correlationID, action string, args json.RawMessage, clientID string) {
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

func (h *ActionRuntime) AttachEvidencePayload(correlationID string, responseData map[string]any) {
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
