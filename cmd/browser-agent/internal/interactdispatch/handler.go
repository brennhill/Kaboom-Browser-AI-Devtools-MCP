// handler.go — Canonical interact routing and composable response orchestration.
// Docs: docs/features/feature/interact-explore/index.md

package interactdispatch

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolrouting"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

// ComposableSideEffectDelay allows queued side effects to begin before a
// screenshot is requested. Delay is injected so owner tests use no wall clock.
const composableSideEffectDelay = 300 * time.Millisecond

// Action is one canonical interact action implementation.
type Action func(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse

// Deps contains only the action and composable capabilities owned elsewhere.
type Deps struct {
	Actions            map[string]Action
	ApplyJitter        func(string) int
	QueueSubtitle      func(mcp.JSONRPCRequest, string)
	QueueAutoDismiss   func(mcp.JSONRPCRequest)
	QueueWaitForStable func(mcp.JSONRPCRequest, int)
	QueueActionDiff    func(mcp.JSONRPCRequest)
	AppendScreenshot   func(mcp.JSONRPCResponse, mcp.JSONRPCRequest) mcp.JSONRPCResponse
	AppendInteractive  func(mcp.JSONRPCResponse, mcp.JSONRPCRequest) mcp.JSONRPCResponse
	Delay              func(time.Duration)
}

// Handler owns immutable interact dispatch state for one ToolHandler.
type Handler struct {
	deps     Deps
	registry toolrouting.Registry[*Handler]
	actions  []string
}

// New constructs an interact dispatcher and defensively copies its action map.
func New(deps Deps) *Handler {
	actions := make(map[string]Action, len(deps.Actions))
	for name, action := range deps.Actions {
		actions[name] = action
	}
	deps.Actions = actions
	if deps.Delay == nil {
		deps.Delay = time.Sleep
	}

	h := &Handler{deps: deps}
	handlers := make(map[string]toolrouting.Handler[*Handler], len(actions))
	validModes := make([]string, 0, len(actions))
	for name, action := range actions {
		action := action
		handlers[name] = func(_ *Handler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return action(req, args)
		}
		validModes = append(validModes, name)
	}
	sort.Strings(validModes)
	h.actions = append([]string(nil), validModes...)
	h.registry = toolrouting.Registry[*Handler]{
		Handlers: handlers,
		Resolution: toolrouting.Resolution{
			ToolName:   "interact",
			ValidModes: strings.Join(validModes, ", "),
		},
		PreDispatch: preDispatch,
	}
	return h
}

// ActionNames returns the sorted immutable runtime action surface.
func (h *Handler) ActionNames() []string {
	if h == nil {
		return nil
	}
	return append([]string(nil), h.actions...)
}

func preDispatch(h *Handler, req mcp.JSONRPCRequest, args json.RawMessage, what string) (json.RawMessage, *mcp.JSONRPCResponse) {
	if h.deps.ApplyJitter != nil {
		h.deps.ApplyJitter(what)
	}
	if _, err := toolinteract.ParseEvidenceMode(args); err != nil {
		resp := mcp.Fail(req, mcp.ErrInvalidParam, "Invalid 'evidence' value",
			"Use evidence='off' (default), 'on_mutation', or 'always'", mcp.WithParam("evidence"))
		return args, &resp
	}
	return args, nil
}

// Handle dispatches one action and applies compatible composable effects.
func (h *Handler) Handle(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	params := parseComposableArgs(args)
	what := resolveWhat(args)
	response := toolrouting.Dispatch(h, req, args, h.registry)
	failed := act.IsErrorResponse(response)

	h.queueSubtitleEffect(req, params, what, failed)
	hasSideEffects := h.queueStabilizingEffects(req, params, what, failed)
	return h.applyResponseEnrichments(response, req, params, failed, hasSideEffects)
}

func (h *Handler) queueSubtitleEffect(req mcp.JSONRPCRequest, params composableArgs, what string, failed bool) {
	if params.Subtitle == nil || what == "subtitle" || failed || h.deps.QueueSubtitle == nil {
		return
	}
	h.deps.QueueSubtitle(req, *params.Subtitle)
}

func (h *Handler) queueStabilizingEffects(req mcp.JSONRPCRequest, params composableArgs, what string, failed bool) bool {
	hasSideEffects := false
	if params.AutoDismiss && what == "navigate" && !failed && h.deps.QueueAutoDismiss != nil {
		h.deps.QueueAutoDismiss(req)
		hasSideEffects = true
	}
	if params.WaitForStable && (what == "navigate" || what == "click") && !failed && h.deps.QueueWaitForStable != nil {
		h.deps.QueueWaitForStable(req, params.StabilityMs)
		hasSideEffects = true
	}
	if params.ActionDiff && !failed && h.deps.QueueActionDiff != nil {
		h.deps.QueueActionDiff(req)
		hasSideEffects = true
	}
	return hasSideEffects
}

func (h *Handler) applyResponseEnrichments(response mcp.JSONRPCResponse, req mcp.JSONRPCRequest, params composableArgs, failed bool, hasSideEffects bool) mcp.JSONRPCResponse {
	if hasSideEffects && params.IncludeScreenshot {
		h.deps.Delay(composableSideEffectDelay)
	}
	if params.IncludeScreenshot && !failed && h.deps.AppendScreenshot != nil {
		response = h.deps.AppendScreenshot(response, req)
	}
	if params.IncludeInteractive && !failed && h.deps.AppendInteractive != nil {
		response = h.deps.AppendInteractive(response, req)
	}
	return response
}

type composableArgs struct {
	Subtitle           *string `json:"subtitle"`
	IncludeScreenshot  bool    `json:"include_screenshot"`
	IncludeInteractive bool    `json:"include_interactive"`
	AutoDismiss        bool    `json:"auto_dismiss"`
	WaitForStable      bool    `json:"wait_for_stable"`
	StabilityMs        int     `json:"stability_ms,omitempty"`
	ActionDiff         bool    `json:"action_diff"`
}

func parseComposableArgs(args json.RawMessage) composableArgs {
	var params composableArgs
	if len(args) == 0 {
		return params
	}
	if err := json.Unmarshal(args, &params); err != nil {
		// EXPECTED_ABSENCE: canonical toolrouting.Dispatch reports the same malformed
		// JSON. Composable parsing must remain side-effect free and avoid a duplicate log.
		return composableArgs{}
	}
	return params
}

func resolveWhat(args json.RawMessage) string {
	if len(args) == 0 {
		return ""
	}
	var raw struct {
		What string `json:"what"`
	}
	if err := json.Unmarshal(args, &raw); err != nil {
		// EXPECTED_ABSENCE: malformed JSON is reported by canonical dispatch.
		return ""
	}
	return raw.What
}
