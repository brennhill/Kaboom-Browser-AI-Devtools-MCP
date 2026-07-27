// handler.go — Saved-sequence persistence and CRUD operations.
// Docs: docs/features/feature/batch-sequences/index.md

package sequencehandler

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

type Store interface {
	Save(namespace, key string, data []byte) error
	Load(namespace, key string) ([]byte, error)
	List(namespace string) ([]string, error)
	Delete(namespace, key string) error
}

type Deps struct {
	Store          Store
	ReplayMu       *sync.Mutex
	Interact       func(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
	WaitForCommand func(string, time.Duration) (*queries.CommandResult, bool)
	RecordAction   func(string, string, map[string]any)
}

type Handler struct {
	deps Deps
}

func New(deps Deps) *Handler {
	return &Handler{deps: deps}
}

func (h *Handler) Save(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Tags        []string          `json:"tags"`
		Steps       []json.RawMessage `json:"steps"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}
	if resp, stop := toolresp.RequireString(req, params.Name, "name", "Add the 'name' parameter"); stop {
		return resp
	}
	if len(params.Name) > toolconfigure.MaxSequenceNameLen {
		return mcp.Fail(req, mcp.ErrInvalidParam, fmt.Sprintf("Name exceeds maximum length of %d characters", toolconfigure.MaxSequenceNameLen), "Use a shorter name", mcp.WithParam("name"))
	}
	if !toolconfigure.SequenceNamePattern.MatchString(params.Name) {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Name must match ^[a-zA-Z0-9_-]+$", "Use only alphanumeric characters, hyphens, and underscores", mcp.WithParam("name"))
	}
	if len(params.Steps) == 0 {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Steps must be a non-empty array", "Add at least one step", mcp.WithParam("steps"))
	}
	if len(params.Steps) > toolconfigure.MaxSequenceSteps {
		return mcp.Fail(req, mcp.ErrInvalidParam, fmt.Sprintf("Steps exceeds maximum of %d", toolconfigure.MaxSequenceSteps), "Split into smaller sequences", mcp.WithParam("steps"))
	}
	for i, step := range params.Steps {
		var action struct {
			What   string `json:"what"`
			Action string `json:"action"`
		}
		if err := json.Unmarshal(step, &action); err != nil || (action.What == "" && action.Action == "") {
			return mcp.Fail(req, mcp.ErrInvalidParam, fmt.Sprintf("Step[%d] missing required 'what' field", i), "Add a 'what' field to each step", mcp.WithParam("steps"))
		}
	}
	if resp, stop := h.requireStore(req); stop {
		return resp
	}
	sequence := toolconfigure.Sequence{
		Name: params.Name, Description: params.Description, Tags: params.Tags,
		SavedAt: time.Now().UTC().Format(time.RFC3339), StepCount: len(params.Steps), Steps: params.Steps,
	}
	data, err := json.Marshal(sequence)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Failed to serialize sequence: "+err.Error(), "Check step format")
	}
	if err := h.deps.Store.Save(toolconfigure.SequenceNamespace, params.Name, data); err != nil {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Failed to save sequence: "+err.Error(), "Check disk space")
	}
	return mcp.Succeed(req, "Sequence saved", map[string]any{
		"status": "saved", "name": sequence.Name, "step_count": sequence.StepCount,
		"saved_at": sequence.SavedAt, "message": fmt.Sprintf("Sequence saved: %s (%d steps)", sequence.Name, sequence.StepCount),
	})
}

func (h *Handler) Get(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Name string `json:"name"`
	}
	mcp.LenientUnmarshal(args, &params)
	if resp, stop := toolresp.RequireString(req, params.Name, "name", "Add the 'name' parameter"); stop {
		return resp
	}
	sequence, failure := h.load(req, params.Name)
	if failure != nil {
		return *failure
	}
	return mcp.Succeed(req, "Sequence details", map[string]any{
		"status": "ok", "name": sequence.Name, "description": sequence.Description,
		"tags": sequence.Tags, "saved_at": sequence.SavedAt, "step_count": sequence.StepCount, "steps": sequence.Steps,
	})
}

func (h *Handler) List(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Tags []string `json:"tags"`
	}
	mcp.LenientUnmarshal(args, &params)
	if resp, stop := h.requireStore(req); stop {
		return resp
	}
	keys, err := h.deps.Store.List(toolconfigure.SequenceNamespace)
	if err != nil {
		return mcp.Succeed(req, "Sequences", map[string]any{"status": "ok", "sequences": []any{}, "count": 0})
	}
	summaries := make([]toolconfigure.SequenceSummary, 0, len(keys))
	for _, key := range keys {
		data, loadErr := h.deps.Store.Load(toolconfigure.SequenceNamespace, key)
		var sequence toolconfigure.Sequence
		if loadErr != nil || json.Unmarshal(data, &sequence) != nil || !hasAllTags(sequence.Tags, params.Tags) {
			continue
		}
		summaries = append(summaries, toolconfigure.SequenceSummary{
			Name: sequence.Name, Description: sequence.Description, Tags: sequence.Tags,
			SavedAt: sequence.SavedAt, StepCount: sequence.StepCount,
		})
	}
	return mcp.Succeed(req, "Sequences", map[string]any{"status": "ok", "sequences": summaries, "count": len(summaries)})
}

func (h *Handler) Delete(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Name string `json:"name"`
	}
	mcp.LenientUnmarshal(args, &params)
	if resp, stop := toolresp.RequireString(req, params.Name, "name", "Add the 'name' parameter"); stop {
		return resp
	}
	if resp, stop := h.requireStore(req); stop {
		return resp
	}
	if _, err := h.deps.Store.Load(toolconfigure.SequenceNamespace, params.Name); err != nil {
		return mcp.Fail(req, mcp.ErrNoData, "Sequence not found: "+params.Name, "Use list_sequences to see available sequences")
	}
	if err := h.deps.Store.Delete(toolconfigure.SequenceNamespace, params.Name); err != nil {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Failed to delete sequence: "+err.Error(), "Try again")
	}
	return mcp.Succeed(req, "Sequence deleted", map[string]any{"status": "deleted", "name": params.Name, "message": "Sequence deleted: " + params.Name})
}

func (h *Handler) requireStore(req mcp.JSONRPCRequest) (mcp.JSONRPCResponse, bool) {
	if h.deps.Store != nil {
		return mcp.JSONRPCResponse{}, false
	}
	return mcp.Fail(req, mcp.ErrNotInitialized, "Session store not initialized", "Internal error — do not retry"), true
}

func (h *Handler) load(req mcp.JSONRPCRequest, name string) (*toolconfigure.Sequence, *mcp.JSONRPCResponse) {
	if resp, stop := h.requireStore(req); stop {
		return nil, &resp
	}
	data, err := h.deps.Store.Load(toolconfigure.SequenceNamespace, name)
	if err != nil {
		resp := mcp.Fail(req, mcp.ErrNoData, "Sequence not found: "+name, "Use list_sequences to see available sequences")
		return nil, &resp
	}
	var sequence toolconfigure.Sequence
	if err := json.Unmarshal(data, &sequence); err != nil {
		resp := mcp.Fail(req, mcp.ErrInvalidJSON, "Corrupted sequence data: "+err.Error(), "Delete and re-save the sequence")
		return nil, &resp
	}
	return &sequence, nil
}

func hasAllTags(sequenceTags, requiredTags []string) bool {
	tags := make(map[string]bool, len(sequenceTags))
	for _, tag := range sequenceTags {
		tags[tag] = true
	}
	for _, required := range requiredTags {
		if !tags[required] {
			return false
		}
	}
	return true
}
