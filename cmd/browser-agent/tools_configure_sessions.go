// tools_configure_sessions.go — Owns configure store, load, and session-diff flows.
// Docs: docs/features/feature/noise-filtering/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session"
	cfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/configure"
)

type configureSessionDeps interface {
	requireSessionStore(req JSONRPCRequest) (JSONRPCResponse, bool)
	invalidateSummaryPref()
}

type configureSessionHandler struct {
	deps             configureSessionDeps
	sessionStoreImpl *persistence.SessionStore
	sessionManager   *session.SessionManager
	server           *Server
}

func newConfigureSessionHandler(deps configureSessionDeps, store *persistence.SessionStore, manager *session.SessionManager, server *Server) *configureSessionHandler {
	return &configureSessionHandler{deps: deps, sessionStoreImpl: store, sessionManager: manager, server: server}
}

func (h *ToolHandler) configureSession() *configureSessionHandler {
	return h.configureSessionHandler
}

func (h *configureSessionHandler) handleConfigureStore(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		StoreAction string          `json:"store_action"`
		Action      string          `json:"action"`
		Namespace   string          `json:"namespace"`
		Key         string          `json:"key"`
		Data        json.RawMessage `json:"data"`
		Value       json.RawMessage `json:"value"`
	}
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &params); stop {
			return resp
		}
	}
	action := params.StoreAction
	if action == "" && isStoreAction(params.Action) {
		action = params.Action
	}
	if action == "" {
		action = "list"
	}
	namespace := params.Namespace
	if namespace == "" {
		namespace = defaultStoreNamespace
	}
	data := params.Data
	if len(data) == 0 && len(params.Value) > 0 {
		data = params.Value
	}
	if resp, blocked := h.deps.requireSessionStore(req); blocked {
		return resp
	}
	result, err := h.sessionStoreImpl.HandleSessionStore(persistence.SessionStoreArgs{
		Action: action, Namespace: namespace, Key: params.Key, Data: data,
	})
	if err != nil {
		return mcp.Fail(req, ErrInvalidParam, err.Error(), "Fix the request parameters and try again")
	}
	if namespace == "session" && params.Key == "response_mode" {
		h.deps.invalidateSummaryPref()
	}
	if params.Key == "active_codebase" && action == "save" && h.server != nil {
		var path string
		if json.Unmarshal(data, &path) == nil {
			h.server.SetActiveCodebase(path)
		}
	}
	var responseData map[string]any
	if json.Unmarshal(result, &responseData) != nil {
		responseData = map[string]any{"raw": string(result)}
	}
	return mcp.Succeed(req, "Store operation complete", responseData)
}

func (h *configureSessionHandler) handleLoadSessionContext(req JSONRPCRequest, _ json.RawMessage) JSONRPCResponse {
	if h.sessionStoreImpl == nil {
		return mcp.Fail(req, ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")
	}
	ctx := h.sessionStoreImpl.LoadSessionContext()
	responseData := map[string]any{
		"status": "ok", "project_id": ctx.ProjectID, "session_count": ctx.SessionCount,
		"baselines": ctx.Baselines, "error_history": ctx.ErrorHistory,
	}
	if ctx.NoiseConfig != nil {
		responseData["noise_config"] = ctx.NoiseConfig
	}
	if ctx.APISchema != nil {
		responseData["api_schema"] = ctx.APISchema
	}
	if ctx.Performance != nil {
		responseData["performance"] = ctx.Performance
	}
	return mcp.Succeed(req, "Session context loaded", responseData)
}

// handleDiffSessionsWrapper repackages verif_session_action -> action for handleDiffSessions.
func (h *configureSessionHandler) handleDiffSessionsWrapper(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	rewritten, err := cfg.RewriteDiffSessionsArgs(args)
	if err != nil {
		return mcp.Fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}
	return h.handleDiffSessions(req, rewritten)
}

func (h *configureSessionHandler) handleDiffSessions(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if h.sessionManager == nil {
		return mcp.Fail(req, ErrNotInitialized, "Session manager not initialized", "Internal error — do not retry")
	}

	result, err := h.sessionManager.HandleTool(args)
	if err != nil {
		return mcp.Fail(req, ErrInvalidParam, err.Error(), "Fix request parameters and retry")
	}

	responseData := map[string]any{"status": "ok"}
	if m, ok := result.(map[string]any); ok {
		for k, v := range m {
			responseData[k] = v
		}
	} else {
		responseData["result"] = result
	}

	return mcp.Succeed(req, "Session diff", responseData)
}
