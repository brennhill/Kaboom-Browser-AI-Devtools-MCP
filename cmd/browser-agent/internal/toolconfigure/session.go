// session.go — Configure store, load, and session-diff flows.

package toolconfigure

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session"
	cfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/configure"
)

const defaultStoreNamespace = "session"

// SessionDeps are the host operations required by configure session flows.
type SessionDeps struct {
	RequireStore      func(req mcp.JSONRPCRequest) (mcp.JSONRPCResponse, bool)
	InvalidateSummary func()
	SetActiveCodebase func(path string)
}

// SessionHandler owns configure session persistence and diff behavior.
type SessionHandler struct {
	deps    SessionDeps
	store   *persistence.SessionStore
	manager *session.SessionManager
}

// NewSessionHandler creates a configure session handler.
func NewSessionHandler(deps SessionDeps, store *persistence.SessionStore, manager *session.SessionManager) *SessionHandler {
	return &SessionHandler{deps: deps, store: store, manager: manager}
}

// Store executes a namespaced persistence operation.
func (handler *SessionHandler) Store(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		StoreAction string          `json:"store_action"`
		Namespace   string          `json:"namespace"`
		Key         string          `json:"key"`
		Data        json.RawMessage `json:"data"`
	}
	if len(args) > 0 {
		if response, stop := mcp.ParseArgs(req, args, &params); stop {
			return response
		}
	}
	action := params.StoreAction
	if action == "" {
		action = "list"
	}
	namespace := params.Namespace
	if namespace == "" {
		namespace = defaultStoreNamespace
	}
	data := params.Data
	if response, blocked := handler.deps.RequireStore(req); blocked {
		return response
	}
	result, err := handler.store.HandleSessionStore(persistence.SessionStoreArgs{
		Action: action, Namespace: namespace, Key: params.Key, Data: data,
	})
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidParam, err.Error(), "Fix the request parameters and try again")
	}
	if namespace == "session" && params.Key == "response_mode" {
		handler.deps.InvalidateSummary()
	}
	if params.Key == "active_codebase" && action == "save" && handler.deps.SetActiveCodebase != nil {
		var path string
		if json.Unmarshal(data, &path) == nil {
			handler.deps.SetActiveCodebase(path)
		}
	}
	var responseData map[string]any
	if json.Unmarshal(result, &responseData) != nil {
		responseData = map[string]any{"raw": string(result)}
	}
	return mcp.Succeed(req, "Store operation complete", responseData)
}

// Load returns the persisted session context.
func (handler *SessionHandler) Load(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	if handler.store == nil {
		return mcp.Fail(req, mcp.ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")
	}
	ctx := handler.store.LoadSessionContext()
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

// Diff rewrites the public arguments and executes session comparison.
func (handler *SessionHandler) Diff(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	rewritten, err := cfg.RewriteDiffSessionsArgs(args)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}
	if handler.manager == nil {
		return mcp.Fail(req, mcp.ErrNotInitialized, "Session manager not initialized", "Internal error — do not retry")
	}
	result, err := handler.manager.HandleTool(rewritten)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidParam, err.Error(), "Fix request parameters and retry")
	}
	responseData := map[string]any{"status": "ok"}
	if fields, ok := result.(map[string]any); ok {
		for key, value := range fields {
			responseData[key] = value
		}
	} else {
		responseData["result"] = result
	}
	return mcp.Succeed(req, "Session diff", responseData)
}
